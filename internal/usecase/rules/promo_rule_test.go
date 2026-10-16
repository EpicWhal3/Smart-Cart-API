package rules

import (
	"context"
	"testing"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/stretchr/testify/require"
)

func availableItem(sku int64, qty int, selected bool, priceAdded, current int64) CalculatedItem {
	return CalculatedItem{
		Item:              cartItem(sku, qty, selected, priceAdded),
		Product:           domain.Product{SKUID: sku, CurrentPrice: money(current), StockQuantity: 10},
		Available:         true,
		EffectiveQuantity: qty,
	}
}

func TestPromoRule(t *testing.T) {
	ozon10 := &domain.PromoRule{Code: "OZON10", DiscountPercent: 10, MinOrderAmount: money(200000), IsActive: true}

	tests := []struct {
		name           string
		promoCode      *string
		rule           *domain.PromoRule
		items          []CalculatedItem
		wantSubtotal   domain.Money
		wantDiscount   domain.Money
		wantFinal      domain.Money
		wantCount      int
		wantPromoNil   bool
		wantApplied    bool
		wantReasonSet  bool
		wantPromoValue domain.Money
	}{
		{
			name:           "промокод применён",
			promoCode:      strPtr("OZON10"),
			rule:           ozon10,
			items:          []CalculatedItem{availableItem(101, 1, true, 250000, 250000)},
			wantSubtotal:   money(250000),
			wantDiscount:   money(25000),
			wantFinal:      money(225000),
			wantCount:      1,
			wantApplied:    true,
			wantPromoValue: money(25000),
		},
		{
			name:          "сумма меньше минимальной — промокод не применён",
			promoCode:     strPtr("OZON10"),
			rule:          ozon10,
			items:         []CalculatedItem{availableItem(101, 1, true, 100000, 100000)},
			wantSubtotal:  money(100000),
			wantDiscount:  money(0),
			wantFinal:     money(100000),
			wantCount:     1,
			wantApplied:   false,
			wantReasonSet: true,
		},
		{
			name:          "неактивный промокод не применён",
			promoCode:     strPtr("OLD20"),
			rule:          &domain.PromoRule{Code: "OLD20", DiscountPercent: 20, MinOrderAmount: money(0), IsActive: false},
			items:         []CalculatedItem{availableItem(101, 1, true, 300000, 300000)},
			wantSubtotal:  money(300000),
			wantDiscount:  money(0),
			wantFinal:     money(300000),
			wantCount:     1,
			wantApplied:   false,
			wantReasonSet: true,
		},
		{
			name:          "промокод не найден",
			promoCode:     strPtr("UNKNOWN"),
			rule:          nil,
			items:         []CalculatedItem{availableItem(101, 1, true, 300000, 300000)},
			wantSubtotal:  money(300000),
			wantDiscount:  money(0),
			wantFinal:     money(300000),
			wantCount:     1,
			wantApplied:   false,
			wantReasonSet: true,
		},
		{
			name:         "промокод не задан",
			promoCode:    nil,
			rule:         nil,
			items:        []CalculatedItem{availableItem(101, 1, true, 300000, 300000)},
			wantSubtotal: money(300000),
			wantDiscount: money(0),
			wantFinal:    money(300000),
			wantCount:    1,
			wantPromoNil: true,
		},
		{
			name:         "снижение цены учитывается в скидке",
			promoCode:    nil,
			rule:         nil,
			items:        []CalculatedItem{availableItem(101, 1, true, 300000, 250000)},
			wantSubtotal: money(300000),
			wantDiscount: money(50000),
			wantFinal:    money(250000),
			wantCount:    1,
			wantPromoNil: true,
		},
		{
			name:         "невыбранные товары не учитываются",
			promoCode:    nil,
			rule:         nil,
			items:        []CalculatedItem{availableItem(101, 1, false, 300000, 250000)},
			wantSubtotal: money(0),
			wantDiscount: money(0),
			wantFinal:    money(0),
			wantCount:    0,
			wantPromoNil: true,
		},
		{
			name:         "пустая корзина",
			promoCode:    nil,
			rule:         nil,
			items:        nil,
			wantSubtotal: money(0),
			wantDiscount: money(0),
			wantFinal:    money(0),
			wantCount:    0,
			wantPromoNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := &CalculatedCart{UserID: 1, PromoCode: tt.promoCode, Available: tt.items}
			deps := &Deps{Promo: tt.rule}

			require.NoError(t, NewPromoRule().Apply(context.Background(), cc, deps))

			require.Equal(t, tt.wantSubtotal, cc.Subtotal)
			require.Equal(t, tt.wantDiscount, cc.Discount)
			require.Equal(t, tt.wantFinal, cc.Final)
			require.Equal(t, tt.wantCount, cc.TotalItemsCount)

			if tt.wantPromoNil {
				require.Nil(t, cc.Promo)
				return
			}

			require.NotNil(t, cc.Promo)
			require.Equal(t, tt.wantApplied, cc.Promo.Applied)
			require.Equal(t, tt.wantPromoValue, cc.Promo.Discount)
			if tt.wantReasonSet {
				require.NotEmpty(t, cc.Promo.Reason)
			}
		})
	}
}
