package bdui

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/max25/smart-cart-service/internal/usecase/rules"
	"github.com/stretchr/testify/require"
)

func money(v int64) domain.Money { return domain.Money(v) }

func moneyPtr(v int64) *domain.Money {
	m := domain.Money(v)
	return &m
}

func availableItem(sku int64, qty int, selected bool, priceAdded, current, old int64, stock int, title string, badges []domain.Badge) rules.CalculatedItem {
	item := rules.CalculatedItem{
		Item: domain.CartItem{
			UserID:          1,
			SKUID:           sku,
			Quantity:        qty,
			IsSelected:      selected,
			PriceAtAddition: money(priceAdded),
			AddedAt:         time.Unix(0, 0).UTC(),
		},
		Product: domain.Product{
			SKUID:         sku,
			Title:         title,
			CurrentPrice:  money(current),
			OldPrice:      moneyPtr(old),
			StockQuantity: stock,
		},
		Available:         true,
		EffectiveQuantity: qty,
		Badges:            badges,
	}
	return item
}

// TestBuilder_MatchesSpecExample проверяет полное соответствие эталонному ответу.
func TestBuilder_MatchesSpecExample(t *testing.T) {
	cc := &rules.CalculatedCart{
		UserID: 1,
		Available: []rules.CalculatedItem{
			availableItem(
				101, 1, true, 135000, 115000, 135000, 3,
				"Механическая клавиатура Keychron K2",
				[]domain.Badge{
					{Text: "Цена снизилась на 200 ₽", Color: domain.BadgeColorGreen},
					{Text: "Осталось 3 шт.", Color: domain.BadgeColorOrange},
				},
			),
		},
		Unavailable: []rules.CalculatedItem{
			{
				Item: domain.CartItem{UserID: 1, SKUID: 205, Quantity: 1, IsSelected: true, PriceAtAddition: money(99000)},
				Product: domain.Product{
					SKUID:         205,
					Title:         "Беспроводная мышь Logitech MX Master 3S",
					CurrentPrice:  money(99000),
					StockQuantity: 0,
				},
				Available: false,
				Similar: []domain.Product{
					{SKUID: 206, Title: "Беспроводная мышь Razer Pro Click", CurrentPrice: money(89000), StockQuantity: 50},
				},
			},
		},
		TotalItemsCount: 1,
		Subtotal:        money(135000),
		Discount:        money(20000),
		Final:           money(115000),
		FreeShipping: rules.FreeShippingInfo{
			Threshold: money(150000),
			Current:   money(115000),
			Remaining: money(35000),
			Achieved:  false,
			Percent:   76,
		},
	}

	screen := NewBuilder().Build(cc)

	got, err := json.Marshal(screen)
	require.NoError(t, err)

	const want = `{
	  "screen": "cart_v1",
	  "widgets": [
	    {
	      "id": "shipping_bar_1",
	      "type": "progress_bar",
	      "payload": {
	        "title": "Добавьте товаров ещё на 350 ₽ для бесплатной доставки",
	        "current_value": 115000,
	        "target_value": 150000,
	        "progress_percent": 76,
	        "style": "info"
	      }
	    },
	    {
	      "id": "item_sku_101",
	      "type": "cart_item",
	      "payload": {
	        "sku_id": 101,
	        "title": "Механическая клавиатура Keychron K2",
	        "quantity": 1,
	        "is_selected": true,
	        "unit_price": 115000,
	        "old_price": 135000,
	        "badges": [
	          { "text": "Цена снизилась на 200 ₽", "color": "green" },
	          { "text": "Осталось 3 шт.", "color": "orange" }
	        ]
	      },
	      "actions": {
	        "increment": { "method": "PATCH", "url": "/api/v1/cart/items/101", "body": { "quantity": 2 } },
	        "decrement": { "method": "PATCH", "url": "/api/v1/cart/items/101", "body": { "quantity": 0 } },
	        "toggle": { "method": "POST", "url": "/api/v1/cart/items/101/toggle" }
	      }
	    },
	    {
	      "id": "unavailable_section",
	      "type": "out_of_stock_group",
	      "payload": {
	        "title": "Закончились, но есть похожие",
	        "items": [
	          {
	            "sku_id": 205,
	            "title": "Беспроводная мышь Logitech MX Master 3S",
	            "similar_products": [
	              { "sku_id": 206, "title": "Беспроводная мышь Razer Pro Click", "price": 89000 }
	            ]
	          }
	        ]
	      }
	    },
	    {
	      "id": "checkout_footer",
	      "type": "summary_footer",
	      "payload": {
	        "total_items_count": 1,
	        "subtotal_amount": 135000,
	        "discount_amount": 20000,
	        "final_amount": 115000,
	        "checkout_button": {
	          "text": "Перейти к оформлению (1 150 ₽)",
	          "is_enabled": true
	        }
	      }
	    }
	  ]
	}`

	require.JSONEq(t, want, string(got))
}

func TestBuilder_EmptyCart(t *testing.T) {
	cc := &rules.CalculatedCart{
		UserID: 1,
		FreeShipping: rules.FreeShippingInfo{
			Threshold: money(150000),
			Current:   money(0),
			Remaining: money(150000),
			Achieved:  false,
			Percent:   0,
		},
	}

	screen := NewBuilder().Build(cc)

	require.Equal(t, ScreenCartV1, screen.Screen)
	require.Len(t, screen.Widgets, 2, "ожидаем прогресс-бар и итоговую плашку")

	// Прогресс-бар.
	require.Equal(t, "shipping_bar_1", screen.Widgets[0].ID)
	bar, ok := screen.Widgets[0].Payload.(domain.ProgressBarPayload)
	require.True(t, ok)
	require.Contains(t, bar.Title, "1 500 ₽")

	// Итоговая плашка: кнопка выключена при пустой корзине.
	footer, ok := screen.Widgets[1].Payload.(domain.SummaryFooterPayload)
	require.True(t, ok)
	require.False(t, footer.CheckoutButton.IsEnabled)
	require.Equal(t, int64(0), footer.FinalAmount)
	require.Contains(t, footer.CheckoutButton.Text, "0 ₽")
}

func TestBuilder_FreeShippingAchieved(t *testing.T) {
	cc := &rules.CalculatedCart{
		UserID:          1,
		Available:       []rules.CalculatedItem{availableItem(101, 1, true, 150000, 150000, 0, 50, "Товар", nil)},
		TotalItemsCount: 1,
		Subtotal:        money(150000),
		Final:           money(150000),
		FreeShipping: rules.FreeShippingInfo{
			Threshold: money(150000),
			Current:   money(150000),
			Remaining: money(0),
			Achieved:  true,
			Percent:   100,
		},
	}

	screen := NewBuilder().Build(cc)
	bar := screen.Widgets[0].Payload.(domain.ProgressBarPayload)

	require.Equal(t, "Бесплатная доставка доступна", bar.Title)
	require.Equal(t, "success", bar.Style)
	require.Equal(t, 100, bar.ProgressPercent)
}

func TestBuilder_NoOutOfStockSectionWhenEmpty(t *testing.T) {
	cc := &rules.CalculatedCart{
		UserID:          1,
		Available:       []rules.CalculatedItem{availableItem(101, 1, true, 115000, 115000, 0, 50, "Товар", nil)},
		TotalItemsCount: 1,
		Subtotal:        money(115000),
		Final:           money(115000),
		FreeShipping:    rules.FreeShippingInfo{Threshold: money(150000), Current: money(115000), Remaining: money(35000), Percent: 76},
	}

	screen := NewBuilder().Build(cc)

	for _, w := range screen.Widgets {
		require.NotEqual(t, domain.WidgetTypeOutOfStock, w.Type)
	}
	require.Len(t, screen.Widgets, 3)
}

func TestBuilder_CartItemActions(t *testing.T) {
	cc := &rules.CalculatedCart{
		UserID:          1,
		Available:       []rules.CalculatedItem{availableItem(101, 3, true, 115000, 115000, 0, 50, "Товар", nil)},
		TotalItemsCount: 3,
		Subtotal:        money(345000),
		Final:           money(345000),
		FreeShipping:    rules.FreeShippingInfo{Threshold: money(150000), Current: money(345000), Percent: 100, Achieved: true},
	}

	screen := NewBuilder().Build(cc)
	itemWidget := screen.Widgets[1]

	require.Equal(t, "item_sku_101", itemWidget.ID)

	inc := itemWidget.Actions["increment"].Body.(map[string]int)["quantity"]
	dec := itemWidget.Actions["decrement"].Body.(map[string]int)["quantity"]
	require.Equal(t, 4, inc)
	require.Equal(t, 2, dec)
	require.Equal(t, "PATCH", itemWidget.Actions["increment"].Method)
	require.Equal(t, "/api/v1/cart/items/101/toggle", itemWidget.Actions["toggle"].URL)
}
