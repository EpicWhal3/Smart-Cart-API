package rules

import (
	"context"
	"testing"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestPriceDropRule(t *testing.T) {
	tests := []struct {
		name       string
		priceAdded int64
		current    int64
		wantBadges []domain.Badge
	}{
		{
			name:       "цена снизилась — бейдж в рублях",
			priceAdded: 135000,
			current:    115000,
			wantBadges: []domain.Badge{{Text: "Цена снизилась на 200 ₽", Color: domain.BadgeColorGreen}},
		},
		{
			name:       "цена не изменилась — нет бейджа",
			priceAdded: 115000,
			current:    115000,
			wantBadges: nil,
		},
		{
			name:       "цена выросла — нет бейджа",
			priceAdded: 100000,
			current:    115000,
			wantBadges: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := &CalculatedCart{
				Available: []CalculatedItem{
					{
						Item:    cartItem(101, 1, true, tt.priceAdded),
						Product: domain.Product{SKUID: 101, CurrentPrice: money(tt.current), StockQuantity: 10},
					},
				},
			}

			require.NoError(t, NewPriceDropRule().Apply(context.Background(), cc, &Deps{}))

			require.Len(t, cc.Available, 1)
			require.Equal(t, tt.wantBadges, cc.Available[0].Badges)
		})
	}
}

func TestPriceDropRule_BadgeOrderedFirst(t *testing.T) {
	// Бейдж снижения цены должен идти перед бейджем дефицита.
	cc := &CalculatedCart{
		Available: []CalculatedItem{
			{
				Item:    cartItem(101, 1, true, 135000),
				Product: domain.Product{SKUID: 101, CurrentPrice: money(115000), StockQuantity: 3},
				Badges:  []domain.Badge{{Text: "Осталось 3 шт.", Color: domain.BadgeColorOrange}},
			},
		},
	}

	require.NoError(t, NewPriceDropRule().Apply(context.Background(), cc, &Deps{}))

	require.Equal(t, []domain.Badge{
		{Text: "Цена снизилась на 200 ₽", Color: domain.BadgeColorGreen},
		{Text: "Осталось 3 шт.", Color: domain.BadgeColorOrange},
	}, cc.Available[0].Badges)
}
