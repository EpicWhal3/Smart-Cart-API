package domain

import "errors"

// Доменные ошибки сервиса.
var (
	// ErrCartNotFound — корзина пользователя не найдена.
	ErrCartNotFound = errors.New("cart not found")
	// ErrItemNotFound — позиция корзины не найдена.
	ErrItemNotFound = errors.New("cart item not found")
	// ErrInvalidQuantity — некорректное количество товара.
	ErrInvalidQuantity = errors.New("quantity must be greater than zero")
	// ErrProductNotFound — товар отсутствует в каталоге.
	ErrProductNotFound = errors.New("product not found")
	// ErrPromoCodeNotFound — промокод не найден.
	ErrPromoCodeNotFound = errors.New("promo code not found")
	// ErrPromoCodeInactive — промокод неактивен.
	ErrPromoCodeInactive = errors.New("promo code is not active")
	// ErrPromoMinAmountNotReached — сумма корзины меньше минимальной для промокода.
	ErrPromoMinAmountNotReached = errors.New("cart total is below the promo code minimum amount")
	// ErrCartEmpty — корзина пуста.
	ErrCartEmpty = errors.New("cart is empty")
)
