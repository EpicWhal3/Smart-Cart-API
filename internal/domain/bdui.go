package domain

// Типы BDUI-виджетов.
const (
	WidgetTypeProgressBar   = "progress_bar"
	WidgetTypeCartItem      = "cart_item"
	WidgetTypeOutOfStock    = "out_of_stock_group"
	WidgetTypeSummaryFooter = "summary_footer"
)

// Цвета бейджей.
const (
	BadgeColorGreen  = "green"
	BadgeColorOrange = "orange"
	BadgeColorRed    = "red"
)

// Screen — корневой BDUI-экран.
type Screen struct {
	Screen  string   `json:"screen"`
	Widgets []Widget `json:"widgets"`
}

// Widget — UI-компонент, управляемый бэкендом.
type Widget struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Payload any               `json:"payload"`
	Actions map[string]Action `json:"actions,omitempty"`
}

// Action — доступное клиенту действие над виджетом.
type Action struct {
	Method string `json:"method"`
	URL    string `json:"url"`
	Body   any    `json:"body,omitempty"`
}

// Badge — текстовый бейдж на карточке товара.
type Badge struct {
	Text  string `json:"text"`
	Color string `json:"color"`
}

// ProgressBarPayload — данные прогресс-бара бесплатной доставки.
type ProgressBarPayload struct {
	Title           string `json:"title"`
	CurrentValue    int64  `json:"current_value"`
	TargetValue     int64  `json:"target_value"`
	ProgressPercent int    `json:"progress_percent"`
	Style           string `json:"style"`
}

// CartItemPayload — данные карточки товара в корзине.
type CartItemPayload struct {
	SKUID      int64   `json:"sku_id"`
	Title      string  `json:"title"`
	Quantity   int     `json:"quantity"`
	IsSelected bool    `json:"is_selected"`
	UnitPrice  int64   `json:"unit_price"`
	OldPrice   *int64  `json:"old_price,omitempty"`
	Badges     []Badge `json:"badges,omitempty"`
}

// OutOfStockGroupPayload — блок закончившихся товаров.
type OutOfStockGroupPayload struct {
	Title string            `json:"title"`
	Items []OutOfStockEntry `json:"items"`
}

// OutOfStockEntry — закончившийся товар с вложенной каруселью аналогов.
type OutOfStockEntry struct {
	SKUID           int64            `json:"sku_id"`
	Title           string           `json:"title"`
	SimilarProducts []SimilarProduct `json:"similar_products"`
}

// SimilarProduct — товар-аналог.
type SimilarProduct struct {
	SKUID int64  `json:"sku_id"`
	Title string `json:"title"`
	Price int64  `json:"price"`
}

// SummaryFooterPayload — итоговая плашка корзины.
type SummaryFooterPayload struct {
	TotalItemsCount int            `json:"total_items_count"`
	SubtotalAmount  int64          `json:"subtotal_amount"`
	DiscountAmount  int64          `json:"discount_amount"`
	FinalAmount     int64          `json:"final_amount"`
	CheckoutButton  CheckoutButton `json:"checkout_button"`
}

// CheckoutButton — кнопка перехода к оформлению.
type CheckoutButton struct {
	Text      string `json:"text"`
	IsEnabled bool   `json:"is_enabled"`
}
