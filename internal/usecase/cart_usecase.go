// Package usecase содержит оркестрацию чтения и мутации корзины.
package usecase

import (
	"context"
	"errors"
	"log/slog"

	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/max25/smart-cart-service/internal/usecase/bdui"
	"github.com/max25/smart-cart-service/internal/usecase/rules"
)

// CartRepository — персистентное хранилище корзин (PostgreSQL).
type CartRepository interface {
	GetCart(ctx context.Context, userID int64) (*domain.Cart, error)
	AddItem(ctx context.Context, userID, skuID int64, quantity int, priceAtAddition domain.Money) (*domain.Cart, error)
	UpdateItemQuantity(ctx context.Context, userID, skuID int64, quantity int) (*domain.Cart, error)
	RemoveItem(ctx context.Context, userID, skuID int64) (*domain.Cart, error)
	ToggleSelection(ctx context.Context, userID, skuID int64) (*domain.Cart, error)
	SetPromoCode(ctx context.Context, userID int64, code *string) (*domain.Cart, error)
}

// CatalogProvider — источник данных каталога (Catalog Provider).
type CatalogProvider interface {
	GetProducts(ctx context.Context, skuIDs []int64) (map[int64]domain.Product, error)
	GetSimilar(ctx context.Context, skuID int64, limit int) ([]domain.Product, error)
}

// PromoRepository — источник промокодов.
type PromoRepository interface {
	GetPromoCode(ctx context.Context, code string) (*domain.PromoRule, error)
}

// CartCache — кэш активных корзин (Redis/Valkey).
type CartCache interface {
	Get(ctx context.Context, userID int64) (*domain.Cart, error)
	Set(ctx context.Context, cart *domain.Cart, ttl time.Duration) error
	Delete(ctx context.Context, userID int64) error
}

// CartView — обогащённая и рассчитанная корзина.
type CartView struct {
	Cart       *domain.Cart
	Calculated *rules.CalculatedCart
}

// Deps — зависимости usecase-слоя.
type Deps struct {
	Carts         CartRepository
	Catalog       CatalogProvider
	Promos        PromoRepository
	Cache         CartCache
	Engine        *rules.Engine
	Builder       *bdui.Builder
	Logger        *slog.Logger
	CacheTTL      time.Duration
	EnrichTimeout time.Duration
	SimilarLimit  int
}

// CartUseCase — сценарии работы с корзиной.
type CartUseCase struct {
	carts         CartRepository
	catalog       CatalogProvider
	promos        PromoRepository
	cache         CartCache
	engine        *rules.Engine
	builder       *bdui.Builder
	logger        *slog.Logger
	cacheTTL      time.Duration
	enrichTimeout time.Duration
	similarLimit  int
}

// New создаёт usecase корзины.
func New(deps Deps) *CartUseCase {
	similarLimit := deps.SimilarLimit
	if similarLimit <= 0 {
		similarLimit = 2
	}
	return &CartUseCase{
		carts:         deps.Carts,
		catalog:       deps.Catalog,
		promos:        deps.Promos,
		cache:         deps.Cache,
		engine:        deps.Engine,
		builder:       deps.Builder,
		logger:        deps.Logger,
		cacheTTL:      deps.CacheTTL,
		enrichTimeout: deps.EnrichTimeout,
		similarLimit:  similarLimit,
	}
}

// GetCart возвращает обогащённую и рассчитанную корзину.
func (u *CartUseCase) GetCart(ctx context.Context, userID int64) (*CartView, error) {
	cart, err := u.loadCartForRead(ctx, userID)
	if err != nil {
		return nil, err
	}
	return u.enrich(ctx, cart)
}

// GetBDUI возвращает BDUI-экран корзины.
func (u *CartUseCase) GetBDUI(ctx context.Context, userID int64) (domain.Screen, error) {
	view, err := u.GetCart(ctx, userID)
	if err != nil {
		return domain.Screen{}, err
	}
	return u.builder.Build(view.Calculated), nil
}

// AddItem добавляет товар в корзину.
func (u *CartUseCase) AddItem(ctx context.Context, userID, skuID int64, quantity int) (*domain.Cart, error) {
	if quantity <= 0 {
		return nil, domain.ErrInvalidQuantity
	}

	products, err := u.catalog.GetProducts(ctx, []int64{skuID})
	if err != nil {
		return nil, err
	}
	product, ok := products[skuID]
	if !ok {
		return nil, domain.ErrProductNotFound
	}

	cart, err := u.carts.AddItem(ctx, userID, skuID, quantity, product.CurrentPrice)
	if err != nil {
		return nil, err
	}

	u.writeThroughCache(ctx, cart)
	return cart, nil
}

// UpdateItemQuantity изменяет количество товара (0 — удаление).
func (u *CartUseCase) UpdateItemQuantity(ctx context.Context, userID, skuID int64, quantity int) (*domain.Cart, error) {
	cart, err := u.carts.UpdateItemQuantity(ctx, userID, skuID, quantity)
	if err != nil {
		return nil, err
	}

	u.writeThroughCache(ctx, cart)
	return cart, nil
}

// RemoveItem удаляет товар из корзины.
func (u *CartUseCase) RemoveItem(ctx context.Context, userID, skuID int64) (*domain.Cart, error) {
	cart, err := u.carts.RemoveItem(ctx, userID, skuID)
	if err != nil {
		return nil, err
	}

	u.writeThroughCache(ctx, cart)
	return cart, nil
}

// ToggleSelection инвертирует флаг выбора товара.
func (u *CartUseCase) ToggleSelection(ctx context.Context, userID, skuID int64) (*domain.Cart, error) {
	cart, err := u.carts.ToggleSelection(ctx, userID, skuID)
	if err != nil {
		return nil, err
	}

	u.writeThroughCache(ctx, cart)
	return cart, nil
}

// ApplyPromoCode применяет промокод к корзине.
func (u *CartUseCase) ApplyPromoCode(ctx context.Context, userID int64, code string) (*domain.Cart, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, domain.ErrPromoCodeNotFound
	}

	rule, err := u.promos.GetPromoCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, domain.ErrPromoCodeNotFound
	}
	if !rule.IsActive {
		return nil, domain.ErrPromoCodeInactive
	}

	cart, err := u.carts.SetPromoCode(ctx, userID, &rule.Code)
	if err != nil {
		return nil, err
	}

	u.writeThroughCache(ctx, cart)
	return cart, nil
}

// loadCartForRead читает корзину (Redis → PostgreSQL с fallback). Для отсутствующей
// корзины возвращает пустую корзину, чтобы клиент отрисовал пустое состояние.
func (u *CartUseCase) loadCartForRead(ctx context.Context, userID int64) (*domain.Cart, error) {
	cart, err := u.loadCart(ctx, userID)
	if errors.Is(err, domain.ErrCartNotFound) {
		return &domain.Cart{UserID: userID}, nil
	}
	return cart, err
}

// loadCart реализует Cache-Aside с Fallback на PostgreSQL при отказе Redis.
func (u *CartUseCase) loadCart(ctx context.Context, userID int64) (*domain.Cart, error) {
	if u.cache != nil {
		cached, err := u.cache.Get(ctx, userID)
		switch {
		case err != nil:
			u.logger.WarnContext(ctx, "cart cache unavailable, fallback to postgres",
				slog.Int64("user_id", userID), slog.String("error", err.Error()))
		case cached != nil:
			// TTL продлевается при каждом чтении
			u.writeThroughCache(ctx, cached)
			return cached, nil
		}
	}

	cart, err := u.carts.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	u.writeThroughCache(ctx, cart)
	return cart, nil
}

func (u *CartUseCase) writeThroughCache(ctx context.Context, cart *domain.Cart) {
	if u.cache == nil {
		return
	}
	if err := u.cache.Set(ctx, cart, u.cacheTTL); err != nil {
		u.logger.WarnContext(ctx, "failed to update cart cache",
			slog.Int64("user_id", cart.UserID), slog.String("error", err.Error()))
	}
}

// enrich параллельно обогащает корзину данными каталога и промокода (errgroup),
// с graceful degradation для второстепенного источника аналогов.
func (u *CartUseCase) enrich(ctx context.Context, cart *domain.Cart) (*CartView, error) {
	enrichCtx := ctx
	if u.enrichTimeout > 0 {
		var cancel context.CancelFunc
		enrichCtx, cancel = context.WithTimeout(ctx, u.enrichTimeout)
		defer cancel()
	}

	skuIDs := collectSKUs(cart.Items)

	var (
		products map[int64]domain.Product
		promo    *domain.PromoRule
	)

	group, groupCtx := errgroup.WithContext(enrichCtx)

	group.Go(func() error {
		result, err := u.catalog.GetProducts(groupCtx, skuIDs)
		if err != nil {
			return err
		}
		products = result
		return nil
	})

	if cart.PromoCode != nil && *cart.PromoCode != "" {
		group.Go(func() error {
			rule, err := u.promos.GetPromoCode(groupCtx, *cart.PromoCode)
			if err != nil {
				return err
			}
			promo = rule
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}
	if products == nil {
		products = make(map[int64]domain.Product)
	}

	similar := u.loadSimilar(enrichCtx, cart, products)

	calculated, err := u.engine.Calculate(enrichCtx, &rules.Deps{
		Cart:     cart,
		Products: products,
		Promo:    promo,
		Similar:  similar,
		Now:      time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}

	return &CartView{Cart: cart, Calculated: calculated}, nil
}

// loadSimilar параллельно подбирает аналоги для недоступных товаров.
// Любая ошибка источника аналогов не роняет корзину (Graceful Degradation).
func (u *CartUseCase) loadSimilar(ctx context.Context, cart *domain.Cart, products map[int64]domain.Product) map[int64][]domain.Product {
	similar := make(map[int64][]domain.Product)

	var (
		mu    sync.Mutex
		group errgroup.Group
	)

	for _, item := range cart.Items {
		product, ok := products[item.SKUID]
		if ok && product.StockQuantity > 0 {
			continue
		}

		skuID := item.SKUID
		group.Go(func() error {
			result, err := u.catalog.GetSimilar(ctx, skuID, u.similarLimit)
			if err != nil {
				u.logger.WarnContext(ctx, "similar products unavailable, degrade gracefully",
					slog.Int64("sku_id", skuID), slog.String("error", err.Error()))
				return nil
			}
			mu.Lock()
			similar[skuID] = result
			mu.Unlock()
			return nil
		})
	}

	_ = group.Wait()
	return similar
}

func collectSKUs(items []domain.CartItem) []int64 {
	seen := make(map[int64]struct{}, len(items))
	skus := make([]int64, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item.SKUID]; ok {
			continue
		}
		seen[item.SKUID] = struct{}{}
		skus = append(skus, item.SKUID)
	}
	return skus
}
