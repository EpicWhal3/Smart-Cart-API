package domain

// Product — товар из каталога.
type Product struct {
	SKUID         int64
	Title         string
	CategoryID    int
	CurrentPrice  Money
	OldPrice      *Money
	StockQuantity int
}

// InStock сообщает, доступен ли товар к заказу.
func (p Product) InStock() bool {
	return p.StockQuantity > 0
}

// PromoRule — правило промокода.
type PromoRule struct {
	Code            string
	DiscountPercent int
	MinOrderAmount  Money
	IsActive        bool
}
