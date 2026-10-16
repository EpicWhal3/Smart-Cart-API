package rules

import (
	"time"

	"github.com/max25/smart-cart-service/internal/domain"
)

func money(v int64) domain.Money { return domain.Money(v) }

func moneyPtr(v int64) *domain.Money {
	m := domain.Money(v)
	return &m
}

func strPtr(v string) *string { return &v }

func cartItem(sku int64, qty int, selected bool, priceAtAddition int64) domain.CartItem {
	return domain.CartItem{
		UserID:          1,
		SKUID:           sku,
		Quantity:        qty,
		IsSelected:      selected,
		PriceAtAddition: money(priceAtAddition),
		AddedAt:         time.Unix(0, 0).UTC(),
	}
}
