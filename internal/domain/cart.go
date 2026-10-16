package domain

import (
	"strconv"
	"strings"
	"time"
)

// Money — денежная сумма в копейках. Никаких float64 для денег.
type Money int64

// Kopecks возвращает величину в копейках.
func (m Money) Kopecks() int64 { return int64(m) }

// FormatRub возвращает человекочитаемую строку вида "1 150 ₽" (или "1 150,50 ₽").
func (m Money) FormatRub() string {
	v := int64(m)
	negative := v < 0
	if negative {
		v = -v
	}

	rub := v / 100
	kop := v % 100

	out := formatThousands(rub)
	if kop != 0 {
		out += "," + pad2(kop)
	}
	out += " ₽"

	if negative {
		return "-" + out
	}
	return out
}

func formatThousands(n int64) string {
	raw := strconv.FormatInt(n, 10)
	if len(raw) <= 3 {
		return raw
	}
	var b strings.Builder
	lead := len(raw) % 3
	if lead > 0 {
		b.WriteString(raw[:lead])
	}
	for i := lead; i < len(raw); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(raw[i : i+3])
	}
	return b.String()
}

func pad2(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

// Cart — корзина пользователя.
type Cart struct {
	UserID    int64      `json:"user_id"`
	PromoCode *string    `json:"promo_code,omitempty"`
	Items     []CartItem `json:"items"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// CartItem — позиция корзины. Хранит только данные, необходимые для мутации.
type CartItem struct {
	UserID          int64     `json:"user_id"`
	SKUID           int64     `json:"sku_id"`
	Quantity        int       `json:"quantity"`
	IsSelected      bool      `json:"is_selected"`
	PriceAtAddition Money     `json:"price_at_addition"`
	AddedAt         time.Time `json:"added_at"`
}
