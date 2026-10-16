// Package http содержит HTTP-слой (REST и BDUI): маршрутизацию, хендлеры и DTO.
package http

import (
	"time"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/max25/smart-cart-service/internal/usecase"
	"github.com/max25/smart-cart-service/internal/usecase/rules"
)

// --- Запросы ---

// AddItemRequest — тело запроса на добавление товара в корзину.
type AddItemRequest struct {
	SKUID    int64 `json:"sku_id"`
	Quantity int   `json:"quantity"`
}

// UpdateQuantityRequest — тело запроса на изменение количества товара.
type UpdateQuantityRequest struct {
	Quantity int `json:"quantity"`
}

// ApplyPromoRequest — тело запроса на применение промокода.
type ApplyPromoRequest struct {
	Code string `json:"code"`
}

// --- Ответы ---

// CartResponse — классический REST-ответ с обогащённой и рассчитанной корзиной.
type CartResponse struct {
	UserID           int64           `json:"user_id"`
	PromoCode        *string         `json:"promo_code,omitempty"`
	Items            []CartItemDTO   `json:"items"`
	UnavailableItems []CartItemDTO   `json:"unavailable_items"`
	Totals           CartTotalsDTO   `json:"totals"`
	FreeShipping     FreeShippingDTO `json:"free_shipping"`
	Promo            *PromoResultDTO `json:"promo,omitempty"`
	UpdatedAt        *time.Time      `json:"updated_at,omitempty"`
}

// CartItemDTO — позиция корзины с обогащением каталога, бейджами и аналогами.
type CartItemDTO struct {
	SKUID             int64            `json:"sku_id"`
	Title             string           `json:"title"`
	Quantity          int              `json:"quantity"`
	EffectiveQuantity int              `json:"effective_quantity"`
	IsSelected        bool             `json:"is_selected"`
	Available         bool             `json:"available"`
	UnitPrice         domain.Money     `json:"unit_price"`
	PriceAtAddition   domain.Money     `json:"price_at_addition"`
	OldPrice          *domain.Money    `json:"old_price,omitempty"`
	StockQuantity     int              `json:"stock_quantity"`
	LineTotal         domain.Money     `json:"line_total"`
	Badges            []domain.Badge   `json:"badges"`
	SimilarProducts   []SimilarItemDTO `json:"similar_products,omitempty"`
	AddedAt           time.Time        `json:"added_at"`
}

// SimilarItemDTO — товар-аналог для недоступной позиции.
type SimilarItemDTO struct {
	SKUID int64        `json:"sku_id"`
	Title string       `json:"title"`
	Price domain.Money `json:"price"`
}

// CartTotalsDTO — итоговые суммы корзины.
type CartTotalsDTO struct {
	TotalItemsCount   int          `json:"total_items_count"`
	SubtotalAmount    domain.Money `json:"subtotal_amount"`
	PriceDropDiscount domain.Money `json:"price_drop_discount"`
	PromoDiscount     domain.Money `json:"promo_discount"`
	DiscountAmount    domain.Money `json:"discount_amount"`
	FinalAmount       domain.Money `json:"final_amount"`
}

// FreeShippingDTO — прогресс до бесплатной доставки.
type FreeShippingDTO struct {
	Threshold  domain.Money `json:"threshold"`
	Current    domain.Money `json:"current"`
	Remaining  domain.Money `json:"remaining"`
	Achieved   bool         `json:"achieved"`
	Percentage int          `json:"progress_percent"`
}

// PromoResultDTO — результат применения промокода.
type PromoResultDTO struct {
	Code     string       `json:"code"`
	Applied  bool         `json:"applied"`
	Discount domain.Money `json:"discount"`
	Reason   string       `json:"reason,omitempty"`
}

// MutationResponse — ответ на мутирующие операции (возвращает рассчитанную корзину).
type MutationResponse struct {
	UserID    int64         `json:"user_id"`
	PromoCode *string       `json:"promo_code,omitempty"`
	Items     []CartItemDTO `json:"items"`
	Totals    CartTotalsDTO `json:"totals"`
}

// ErrorResponse — унифицированный ответ об ошибке.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// --- Маппинг ---

// newCartResponse преобразует обогащённую корзину в REST-ответ.
func newCartResponse(view *usecase.CartView) CartResponse {
	cc := view.Calculated

	items := make([]CartItemDTO, 0, len(cc.Available))
	for _, item := range cc.Available {
		items = append(items, newCartItemDTO(item))
	}

	unavailable := make([]CartItemDTO, 0, len(cc.Unavailable))
	for _, item := range cc.Unavailable {
		unavailable = append(unavailable, newCartItemDTO(item))
	}

	resp := CartResponse{
		UserID:           cc.UserID,
		PromoCode:        view.Cart.PromoCode,
		Items:            items,
		UnavailableItems: unavailable,
		Totals: CartTotalsDTO{
			TotalItemsCount:   cc.TotalItemsCount,
			SubtotalAmount:    cc.Subtotal,
			PriceDropDiscount: cc.PriceDropDiscount,
			PromoDiscount:     cc.PromoDiscount,
			DiscountAmount:    cc.Discount,
			FinalAmount:       cc.Final,
		},
		FreeShipping: FreeShippingDTO{
			Threshold:  cc.FreeShipping.Threshold,
			Current:    cc.FreeShipping.Current,
			Remaining:  cc.FreeShipping.Remaining,
			Achieved:   cc.FreeShipping.Achieved,
			Percentage: cc.FreeShipping.Percent,
		},
	}

	if !view.Cart.UpdatedAt.IsZero() {
		updated := view.Cart.UpdatedAt
		resp.UpdatedAt = &updated
	}

	if cc.Promo != nil {
		resp.Promo = &PromoResultDTO{
			Code:     cc.Promo.Code,
			Applied:  cc.Promo.Applied,
			Discount: cc.Promo.Discount,
			Reason:   cc.Promo.Reason,
		}
	}

	return resp
}

// newCartItemDTO преобразует рассчитанную позицию в DTO.
func newCartItemDTO(item rules.CalculatedItem) CartItemDTO {
	badges := item.Badges
	if badges == nil {
		badges = []domain.Badge{}
	}

	dto := CartItemDTO{
		SKUID:             item.Item.SKUID,
		Title:             item.Product.Title,
		Quantity:          item.Item.Quantity,
		EffectiveQuantity: item.EffectiveQuantity,
		IsSelected:        item.Item.IsSelected,
		Available:         item.Available,
		UnitPrice:         item.Product.CurrentPrice,
		PriceAtAddition:   item.Item.PriceAtAddition,
		OldPrice:          item.Product.OldPrice,
		StockQuantity:     item.Product.StockQuantity,
		LineTotal:         domain.Money(int64(item.Product.CurrentPrice) * int64(item.EffectiveQuantity)),
		Badges:            badges,
		AddedAt:           item.Item.AddedAt,
	}

	if len(item.Similar) > 0 {
		similar := make([]SimilarItemDTO, 0, len(item.Similar))
		for _, product := range item.Similar {
			similar = append(similar, SimilarItemDTO{
				SKUID: product.SKUID,
				Title: product.Title,
				Price: product.CurrentPrice,
			})
		}
		dto.SimilarProducts = similar
	}

	return dto
}

// newMutationResponse строит краткий ответ на мутацию корзины.
func newMutationResponse(view *usecase.CartView) MutationResponse {
	resp := newCartResponse(view)
	return MutationResponse{
		UserID:    resp.UserID,
		PromoCode: resp.PromoCode,
		Items:     resp.Items,
		Totals:    resp.Totals,
	}
}
