package rules

import (
	"context"
	"testing"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestScarcityRule(t *testing.T) {
	tests := []struct {
		name          string
		item          domain.CartItem
		product       domain.Product
		productExists bool
		wantAvailable bool
		wantEffective int
		wantBadges    []domain.Badge
	}{
		{
			name:          "товар закончился — переносится в недоступные",
			item:          cartItem(101, 1, true, 100000),
			product:       domain.Product{SKUID: 101, StockQuantity: 0, CurrentPrice: money(100000)},
			productExists: true,
			wantAvailable: false,
			wantEffective: 1,
			wantBadges:    nil,
		},
		{
			name:          "низкий остаток (3) — бейдж дефицита",
			item:          cartItem(101, 1, true, 115000),
			product:       domain.Product{SKUID: 101, StockQuantity: 3, CurrentPrice: money(115000)},
			productExists: true,
			wantAvailable: true,
			wantEffective: 1,
			wantBadges:    []domain.Badge{{Text: "Осталось 3 шт.", Color: domain.BadgeColorOrange}},
		},
		{
			name:          "остаток ровно 5 — бейдж дефицита",
			item:          cartItem(101, 1, true, 115000),
			product:       domain.Product{SKUID: 101, StockQuantity: 5, CurrentPrice: money(115000)},
			productExists: true,
			wantAvailable: true,
			wantEffective: 1,
			wantBadges:    []domain.Badge{{Text: "Осталось 5 шт.", Color: domain.BadgeColorOrange}},
		},
		{
			name:          "достаточный остаток (50) — без бейджей",
			item:          cartItem(101, 1, true, 115000),
			product:       domain.Product{SKUID: 101, StockQuantity: 50, CurrentPrice: money(115000)},
			productExists: true,
			wantAvailable: true,
			wantEffective: 1,
			wantBadges:    nil,
		},
		{
			name:          "количество больше остатка — корректировка и предупреждение",
			item:          cartItem(101, 5, true, 115000),
			product:       domain.Product{SKUID: 101, StockQuantity: 2, CurrentPrice: money(115000)},
			productExists: true,
			wantAvailable: true,
			wantEffective: 2,
			wantBadges: []domain.Badge{
				{Text: "Доступно только 2 шт.", Color: domain.BadgeColorRed},
				{Text: "Осталось 2 шт.", Color: domain.BadgeColorOrange},
			},
		},
		{
			name:          "товара нет в каталоге — недоступен",
			item:          cartItem(999, 1, true, 115000),
			productExists: false,
			wantAvailable: false,
			wantEffective: 1,
			wantBadges:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			products := map[int64]domain.Product{}
			if tt.productExists {
				products[tt.item.SKUID] = tt.product
			}
			similar := map[int64][]domain.Product{
				tt.item.SKUID: {{SKUID: 200, Title: "Аналог", CurrentPrice: money(90000), StockQuantity: 10}},
			}

			deps := &Deps{
				Cart:     &domain.Cart{UserID: 1, Items: []domain.CartItem{tt.item}},
				Products: products,
				Similar:  similar,
			}
			cc := &CalculatedCart{UserID: 1}

			require.NoError(t, NewScarcityRule().Apply(context.Background(), cc, deps))

			if tt.wantAvailable {
				require.Len(t, cc.Available, 1)
				require.Empty(t, cc.Unavailable)
				require.Equal(t, tt.wantEffective, cc.Available[0].EffectiveQuantity)
				require.True(t, cc.Available[0].Available)
				require.Equal(t, tt.wantBadges, cc.Available[0].Badges)
				return
			}

			require.Empty(t, cc.Available)
			require.Len(t, cc.Unavailable, 1)
			require.False(t, cc.Unavailable[0].Available)
			require.Equal(t, tt.wantEffective, cc.Unavailable[0].EffectiveQuantity)
			require.NotEmpty(t, cc.Unavailable[0].Similar, "для недоступного товара должны быть подобраны аналоги")
		})
	}
}
