// Package bdui трансформирует рассчитанную корзину в дерево BDUI-виджетов,
// готовое к рендерингу на Web/Mobile.
package bdui

import (
	"fmt"
	"net/http"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/max25/smart-cart-service/internal/usecase/rules"
)

const (
	// ScreenCartV1 — идентификатор BDUI-экрана корзины.
	ScreenCartV1 = "cart_v1"
	// itemsPathPrefix — базовый путь REST-действий над позициями корзины.
	itemsPathPrefix = "/api/v1/cart/items"
)

// Builder собирает BDUI-экран из рассчитанной корзины.
type Builder struct {
	ScreenID string
}

// NewBuilder создаёт билдер BDUI с экраном по умолчанию.
func NewBuilder() *Builder {
	return &Builder{ScreenID: ScreenCartV1}
}

// Build формирует дерево виджетов в продуктовом порядке:
// прогресс-бар доставки → карточки товаров → блок недоступных → итоговая плашка.
func (b *Builder) Build(cc *rules.CalculatedCart) domain.Screen {
	screen := domain.Screen{
		Screen:  b.screenID(),
		Widgets: make([]domain.Widget, 0, len(cc.Available)+3),
	}

	screen.Widgets = append(screen.Widgets, b.buildShippingWidget(cc))

	for i := range cc.Available {
		screen.Widgets = append(screen.Widgets, b.buildCartItemWidget(&cc.Available[i]))
	}

	if len(cc.Unavailable) > 0 {
		screen.Widgets = append(screen.Widgets, b.buildOutOfStockWidget(cc.Unavailable))
	}

	screen.Widgets = append(screen.Widgets, b.buildSummaryWidget(cc))

	return screen
}

func (b *Builder) screenID() string {
	if b.ScreenID == "" {
		return ScreenCartV1
	}
	return b.ScreenID
}

func (b *Builder) buildShippingWidget(cc *rules.CalculatedCart) domain.Widget {
	fs := cc.FreeShipping

	title := fmt.Sprintf("Добавьте товаров ещё на %s для бесплатной доставки", fs.Remaining.FormatRub())
	style := "info"
	if fs.Achieved {
		title = "Бесплатная доставка доступна"
		style = "success"
	}

	return domain.Widget{
		ID:   "shipping_bar_1",
		Type: domain.WidgetTypeProgressBar,
		Payload: domain.ProgressBarPayload{
			Title:           title,
			CurrentValue:    fs.Current.Kopecks(),
			TargetValue:     fs.Threshold.Kopecks(),
			ProgressPercent: fs.Percent,
			Style:           style,
		},
	}
}

func (b *Builder) buildCartItemWidget(item *rules.CalculatedItem) domain.Widget {
	sku := item.Item.SKUID
	url := fmt.Sprintf("%s/%d", itemsPathPrefix, sku)

	next := item.Item.Quantity + 1
	prev := max(item.Item.Quantity-1, 0)

	var oldPrice *int64
	if item.Product.OldPrice != nil {
		v := item.Product.OldPrice.Kopecks()
		oldPrice = &v
	}

	return domain.Widget{
		ID:   fmt.Sprintf("item_sku_%d", sku),
		Type: domain.WidgetTypeCartItem,
		Payload: domain.CartItemPayload{
			SKUID:      sku,
			Title:      item.Product.Title,
			Quantity:   item.Item.Quantity,
			IsSelected: item.Item.IsSelected,
			UnitPrice:  item.Product.CurrentPrice.Kopecks(),
			OldPrice:   oldPrice,
			Badges:     item.Badges,
		},
		Actions: map[string]domain.Action{
			"increment": {
				Method: http.MethodPatch,
				URL:    url,
				Body:   map[string]int{"quantity": next},
			},
			"decrement": {
				Method: http.MethodPatch,
				URL:    url,
				Body:   map[string]int{"quantity": prev},
			},
			"toggle": {
				Method: http.MethodPost,
				URL:    url + "/toggle",
			},
		},
	}
}

func (b *Builder) buildOutOfStockWidget(items []rules.CalculatedItem) domain.Widget {
	entries := make([]domain.OutOfStockEntry, 0, len(items))
	for i := range items {
		item := &items[i]

		similar := make([]domain.SimilarProduct, 0, len(item.Similar))
		for _, s := range item.Similar {
			similar = append(similar, domain.SimilarProduct{
				SKUID: s.SKUID,
				Title: s.Title,
				Price: s.CurrentPrice.Kopecks(),
			})
		}

		entries = append(entries, domain.OutOfStockEntry{
			SKUID:           item.Item.SKUID,
			Title:           item.Product.Title,
			SimilarProducts: similar,
		})
	}

	return domain.Widget{
		ID:   "unavailable_section",
		Type: domain.WidgetTypeOutOfStock,
		Payload: domain.OutOfStockGroupPayload{
			Title: "Закончились, но есть похожие",
			Items: entries,
		},
	}
}

func (b *Builder) buildSummaryWidget(cc *rules.CalculatedCart) domain.Widget {
	return domain.Widget{
		ID:   "checkout_footer",
		Type: domain.WidgetTypeSummaryFooter,
		Payload: domain.SummaryFooterPayload{
			TotalItemsCount: cc.TotalItemsCount,
			SubtotalAmount:  cc.Subtotal.Kopecks(),
			DiscountAmount:  cc.Discount.Kopecks(),
			FinalAmount:     cc.Final.Kopecks(),
			CheckoutButton: domain.CheckoutButton{
				Text:      fmt.Sprintf("Перейти к оформлению (%s)", cc.Final.FormatRub()),
				IsEnabled: cc.TotalItemsCount > 0,
			},
		},
	}
}
