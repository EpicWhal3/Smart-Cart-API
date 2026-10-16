package rules

import (
	"context"

	"github.com/max25/smart-cart-service/internal/domain"
)

// PromoRule считает итоговые суммы по выбранным доступным товарам и применяет
// промокод (с валидацией минимальной суммы заказа).
type PromoRule struct{}

// NewPromoRule создаёт правило промокода.
func NewPromoRule() PromoRule { return PromoRule{} }

// Name возвращает имя правила.
func (PromoRule) Name() string { return "promo" }

// Apply применяет правило к корзине.
func (PromoRule) Apply(_ context.Context, cc *CalculatedCart, deps *Deps) error {
	var (
		subtotalOriginal domain.Money
		subtotalCurrent  domain.Money
		totalCount       int
	)

	for i := range cc.Available {
		item := &cc.Available[i]
		if !item.Item.IsSelected {
			continue
		}

		qty := domain.Money(item.EffectiveQuantity)
		subtotalOriginal += item.Item.PriceAtAddition * qty
		subtotalCurrent += item.Product.CurrentPrice * qty
		totalCount += item.EffectiveQuantity
	}

	cc.Subtotal = subtotalOriginal
	cc.TotalItemsCount = totalCount
	cc.PriceDropDiscount = subtotalOriginal - subtotalCurrent

	cc.Promo = applyPromo(cc.PromoCode, deps.Promo, subtotalCurrent)
	if cc.Promo != nil {
		cc.PromoDiscount = cc.Promo.Discount
	}

	cc.Discount = cc.PriceDropDiscount + cc.PromoDiscount
	cc.Final = cc.Subtotal - cc.Discount

	return nil
}

func applyPromo(promoCode *string, rule *domain.PromoRule, base domain.Money) *PromoResult {
	if promoCode == nil || *promoCode == "" {
		return nil
	}

	result := &PromoResult{Code: *promoCode}

	switch {
	case rule == nil:
		result.Reason = "промокод не найден"
	case !rule.IsActive:
		result.Reason = "промокод неактивен"
	case base < rule.MinOrderAmount:
		result.Reason = "сумма корзины меньше минимальной для промокода"
	default:
		result.Applied = true
		result.Discount = base * domain.Money(rule.DiscountPercent) / 100
	}

	return result
}
