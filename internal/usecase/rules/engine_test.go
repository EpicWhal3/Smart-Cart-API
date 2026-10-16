package rules

import (
	"context"
	"testing"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestEngine_DefaultPipeline(t *testing.T) {
	engine := NewDefaultEngine()

	tests := []struct {
		name          string
		cart          *domain.Cart
		products      map[int64]domain.Product
		wantCount     int
		wantSubtotal  domain.Money
		wantDiscount  domain.Money
		wantFinal     domain.Money
		wantPercent   int
		wantAchieved  bool
		wantAvailable int
		wantUnavail   int
	}{
		{
			name:          "пустая корзина",
			cart:          &domain.Cart{UserID: 1},
			products:      map[int64]domain.Product{},
			wantCount:     0,
			wantSubtotal:  money(0),
			wantDiscount:  money(0),
			wantFinal:     money(0),
			wantPercent:   0,
			wantAchieved:  false,
			wantAvailable: 0,
			wantUnavail:   0,
		},
		{
			name: "все товары не выбраны чекбоксами",
			cart: &domain.Cart{UserID: 1, Items: []domain.CartItem{
				cartItem(101, 1, false, 115000),
			}},
			products: map[int64]domain.Product{
				101: {SKUID: 101, CurrentPrice: money(115000), StockQuantity: 10},
			},
			wantCount:     0,
			wantSubtotal:  money(0),
			wantDiscount:  money(0),
			wantFinal:     money(0),
			wantPercent:   0,
			wantAchieved:  false,
			wantAvailable: 1,
			wantUnavail:   0,
		},
		{
			name: "эталонный сценарий: падение цены + дефицит",
			cart: &domain.Cart{UserID: 1, Items: []domain.CartItem{
				cartItem(101, 1, true, 135000),
			}},
			products: map[int64]domain.Product{
				101: {SKUID: 101, CurrentPrice: money(115000), StockQuantity: 3},
			},
			wantCount:     1,
			wantSubtotal:  money(135000),
			wantDiscount:  money(20000),
			wantFinal:     money(115000),
			wantPercent:   76,
			wantAchieved:  false,
			wantAvailable: 1,
			wantUnavail:   0,
		},
		{
			name: "сумма ровно равна порогу бесплатной доставки",
			cart: &domain.Cart{UserID: 1, Items: []domain.CartItem{
				cartItem(101, 1, true, 150000),
			}},
			products: map[int64]domain.Product{
				101: {SKUID: 101, CurrentPrice: money(150000), StockQuantity: 10},
			},
			wantCount:     1,
			wantSubtotal:  money(150000),
			wantDiscount:  money(0),
			wantFinal:     money(150000),
			wantPercent:   100,
			wantAchieved:  true,
			wantAvailable: 1,
			wantUnavail:   0,
		},
		{
			name: "товар закончился — попадает в недоступные, не влияет на сумму",
			cart: &domain.Cart{UserID: 1, Items: []domain.CartItem{
				cartItem(205, 1, true, 99000),
			}},
			products: map[int64]domain.Product{
				205: {SKUID: 205, CurrentPrice: money(99000), StockQuantity: 0},
			},
			wantCount:     0,
			wantSubtotal:  money(0),
			wantDiscount:  money(0),
			wantFinal:     money(0),
			wantPercent:   0,
			wantAchieved:  false,
			wantAvailable: 0,
			wantUnavail:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &Deps{
				Cart:     tt.cart,
				Products: tt.products,
				Similar:  map[int64][]domain.Product{},
			}

			cc, err := engine.Calculate(context.Background(), deps)
			require.NoError(t, err)

			require.Equal(t, tt.wantCount, cc.TotalItemsCount)
			require.Equal(t, tt.wantSubtotal, cc.Subtotal)
			require.Equal(t, tt.wantDiscount, cc.Discount)
			require.Equal(t, tt.wantFinal, cc.Final)
			require.Equal(t, tt.wantPercent, cc.FreeShipping.Percent)
			require.Equal(t, tt.wantAchieved, cc.FreeShipping.Achieved)
			require.Len(t, cc.Available, tt.wantAvailable)
			require.Len(t, cc.Unavailable, tt.wantUnavail)
		})
	}
}

func TestEngine_ExactBadgesFromSpec(t *testing.T) {
	engine := NewDefaultEngine()
	deps := &Deps{
		Cart: &domain.Cart{UserID: 1, Items: []domain.CartItem{
			cartItem(101, 1, true, 135000),
		}},
		Products: map[int64]domain.Product{
			101: {SKUID: 101, CurrentPrice: money(115000), StockQuantity: 3},
		},
		Similar: map[int64][]domain.Product{},
	}

	cc, err := engine.Calculate(context.Background(), deps)
	require.NoError(t, err)
	require.Len(t, cc.Available, 1)
	require.Equal(t, []domain.Badge{
		{Text: "Цена снизилась на 200 ₽", Color: domain.BadgeColorGreen},
		{Text: "Осталось 3 шт.", Color: domain.BadgeColorOrange},
	}, cc.Available[0].Badges)
}
