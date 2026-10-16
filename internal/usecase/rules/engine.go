// Package rules реализует движок продуктовых правил корзины.
package rules

import (
	"context"
	"fmt"
	"time"

	"github.com/max25/smart-cart-service/internal/domain"
)

// Пороговые константы продуктовых правил.
const (
	// FreeShippingThreshold — порог бесплатной доставки: 1 500 ₽ (в копейках).
	FreeShippingThreshold domain.Money = 150000
	// LowStockThreshold — остаток, при котором показываем бейдж дефицита.
	LowStockThreshold = 5
)

// Deps — внешние данные, необходимые правилам. Правила не выполняют I/O:
// все данные (каталог, промокод, аналоги) заранее загружаются в usecase-слое.
type Deps struct {
	Cart                  *domain.Cart
	Products              map[int64]domain.Product
	Promo                 *domain.PromoRule
	Similar               map[int64][]domain.Product
	FreeShippingThreshold domain.Money
	Now                   time.Time
}

// CalculatedItem — рассчитанная позиция корзины.
type CalculatedItem struct {
	Item      domain.CartItem
	Product   domain.Product
	Badges    []domain.Badge
	Available bool
	// EffectiveQuantity — количество, участвующее в финансовом расчёте
	// (ограничено доступным остатком на складе).
	EffectiveQuantity int
	Similar           []domain.Product
}

// PromoResult — результат применения промокода.
type PromoResult struct {
	Code     string
	Applied  bool
	Discount domain.Money
	Reason   string
}

// FreeShippingInfo — состояние порога бесплатной доставки.
type FreeShippingInfo struct {
	Threshold domain.Money
	Current   domain.Money
	Remaining domain.Money
	Achieved  bool
	Percent   int
}

// CalculatedCart — результат работы пайплайна правил.
type CalculatedCart struct {
	UserID      int64
	PromoCode   *string
	Available   []CalculatedItem
	Unavailable []CalculatedItem

	TotalItemsCount int

	Subtotal          domain.Money // Сумма выбранных доступных позиций по цене добавления (до скидок)
	PriceDropDiscount domain.Money // Скидка за счёт снижения цены
	PromoDiscount     domain.Money // Скидка по промокоду
	Discount          domain.Money // Общая скидка
	Final             domain.Money // Итоговая сумма

	FreeShipping FreeShippingInfo
	Promo        *PromoResult
}

// Rule — одно звено цепочки правил.
type Rule interface {
	Name() string
	Apply(ctx context.Context, cc *CalculatedCart, deps *Deps) error
}

// Engine последовательно применяет правила к корзине.
type Engine struct {
	rules []Rule
}

// NewEngine создаёт движок с заданным набором правил.
func NewEngine(rules ...Rule) *Engine {
	return &Engine{rules: rules}
}

// NewDefaultEngine собирает стандартный пайплайн правил в продуктовом порядке.
func NewDefaultEngine() *Engine {
	return NewEngine(
		NewScarcityRule(),  // 1. доступность и дефицит
		NewPriceDropRule(), // 2. снижение цены
		NewPromoRule(),     // 3. промокод
		NewDeliveryRule(),  // 4. бесплатная доставка
	)
}

// Calculate прогоняет корзину через цепочку правил.
func (e *Engine) Calculate(ctx context.Context, deps *Deps) (*CalculatedCart, error) {
	if deps == nil || deps.Cart == nil {
		return nil, fmt.Errorf("rules: cart is required")
	}

	cc := &CalculatedCart{
		UserID:    deps.Cart.UserID,
		PromoCode: deps.Cart.PromoCode,
	}

	for _, r := range e.rules {
		if err := r.Apply(ctx, cc, deps); err != nil {
			return nil, fmt.Errorf("rules: apply %q: %w", r.Name(), err)
		}
	}

	return cc, nil
}
