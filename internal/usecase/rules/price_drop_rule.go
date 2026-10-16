package rules

import (
	"context"
	"fmt"

	"github.com/max25/smart-cart-service/internal/domain"
)

// PriceDropRule добавляет бейдж снижения цены, если текущая цена ниже цены
// на момент добавления товара в корзину. Разница выводится в рублях.
type PriceDropRule struct{}

// NewPriceDropRule создаёт правило снижения цены.
func NewPriceDropRule() PriceDropRule { return PriceDropRule{} }

// Name возвращает имя правила.
func (PriceDropRule) Name() string { return "price_drop" }

// Apply применяет правило к доступным позициям корзины.
func (PriceDropRule) Apply(_ context.Context, cc *CalculatedCart, _ *Deps) error {
	for i := range cc.Available {
		item := &cc.Available[i]
		if item.Product.CurrentPrice >= item.Item.PriceAtAddition {
			continue
		}

		diff := item.Item.PriceAtAddition - item.Product.CurrentPrice
		badge := domain.Badge{
			Text:  fmt.Sprintf("Цена снизилась на %s", diff.FormatRub()),
			Color: domain.BadgeColorGreen,
		}
		// Бейдж снижения цены отображается первым (как в эталонном BDUI-ответе).
		item.Badges = append([]domain.Badge{badge}, item.Badges...)
	}

	return nil
}
