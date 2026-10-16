package rules

import (
	"context"
	"fmt"

	"github.com/max25/smart-cart-service/internal/domain"
)

// ScarcityRule — правило доступности и дефицита, выполняется первым,
// чтобы отсечь недоступные товары до подсчёта сумм.
//
//   - stock == 0 → товар недоступен (is_available = false),
//     исключается из финансового расчёта и помечается для подбора аналогов;
//   - 0 < stock <= 5 → товар доступен + бейдж «Осталось N шт.»;
//   - quantity > stock → в расчёт берётся stock, добавляется предупреждение.
type ScarcityRule struct{}

// NewScarcityRule создаёт правило доступности и дефицита.
func NewScarcityRule() ScarcityRule { return ScarcityRule{} }

// Name возвращает имя правила.
func (ScarcityRule) Name() string { return "scarcity" }

// Apply применяет правило к корзине.
func (ScarcityRule) Apply(_ context.Context, cc *CalculatedCart, deps *Deps) error {
	for _, item := range deps.Cart.Items {
		product, ok := deps.Products[item.SKUID]

		if !ok || product.StockQuantity <= 0 {
			// Товар отсутствует в каталоге или закончился.
			unavailable := CalculatedItem{
				Item:              item,
				Product:           product,
				Available:         false,
				EffectiveQuantity: item.Quantity,
				Similar:           deps.Similar[item.SKUID],
			}
			if !ok {
				unavailable.Product = domain.Product{SKUID: item.SKUID}
			}
			cc.Unavailable = append(cc.Unavailable, unavailable)
			continue
		}

		calculated := CalculatedItem{
			Item:              item,
			Product:           product,
			Available:         true,
			EffectiveQuantity: item.Quantity,
		}

		if item.Quantity > product.StockQuantity {
			calculated.EffectiveQuantity = product.StockQuantity
			calculated.Badges = append(calculated.Badges, domain.Badge{
				Text:  fmt.Sprintf("Доступно только %d шт.", product.StockQuantity),
				Color: domain.BadgeColorRed,
			})
		}

		if product.StockQuantity <= LowStockThreshold {
			calculated.Badges = append(calculated.Badges, domain.Badge{
				Text:  fmt.Sprintf("Осталось %d шт.", product.StockQuantity),
				Color: domain.BadgeColorOrange,
			})
		}

		cc.Available = append(cc.Available, calculated)
	}

	return nil
}
