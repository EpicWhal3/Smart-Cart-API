package postgres

import (
	"context"
	"database/sql"
	"errors"

	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/max25/smart-cart-service/internal/domain"
)

// querier абстрагирует пул соединений и транзакцию pgx.
type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CartRepository — репозиторий корзин на pgx/v5.
type CartRepository struct {
	pool *pgxpool.Pool
}

// NewCartRepository создаёт репозиторий корзин.
func NewCartRepository(pool *pgxpool.Pool) *CartRepository {
	return &CartRepository{pool: pool}
}

// GetCart читает корзину пользователя.
func (r *CartRepository) GetCart(ctx context.Context, userID int64) (*domain.Cart, error) {
	return loadCart(ctx, r.pool, userID)
}

// AddItem добавляет товар (или увеличивает количество существующего) в транзакции.
func (r *CartRepository) AddItem(ctx context.Context, userID, skuID int64, quantity int, priceAtAddition domain.Money) (*domain.Cart, error) {
	if quantity <= 0 {
		return nil, domain.ErrInvalidQuantity
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: begin add item: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO carts (user_id, updated_at) VALUES ($1, now())
		 ON CONFLICT (user_id) DO UPDATE SET updated_at = now()`,
		userID,
	); err != nil {
		return nil, fmt.Errorf("postgres: ensure cart: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO cart_items (user_id, sku_id, quantity, is_selected, price_at_addition, added_at)
		 VALUES ($1, $2, $3, TRUE, $4, now())
		 ON CONFLICT (user_id, sku_id) DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity`,
		userID, skuID, quantity, int64(priceAtAddition),
	); err != nil {
		return nil, fmt.Errorf("postgres: add item: %w", err)
	}

	cart, err := loadCart(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("postgres: commit add item: %w", err)
	}
	return cart, nil
}

// UpdateItemQuantity изменяет количество; при quantity <= 0 позиция удаляется.
func (r *CartRepository) UpdateItemQuantity(ctx context.Context, userID, skuID int64, quantity int) (*domain.Cart, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: begin update quantity: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tag pgconn.CommandTag
	if quantity <= 0 {
		tag, err = tx.Exec(ctx, `DELETE FROM cart_items WHERE user_id = $1 AND sku_id = $2`, userID, skuID)
	} else {
		tag, err = tx.Exec(ctx, `UPDATE cart_items SET quantity = $3 WHERE user_id = $1 AND sku_id = $2`, userID, skuID, quantity)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: update item quantity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrItemNotFound
	}

	if err := touchCart(ctx, tx, userID); err != nil {
		return nil, err
	}

	cart, err := loadCart(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("postgres: commit update quantity: %w", err)
	}
	return cart, nil
}

// RemoveItem удаляет позицию корзины.
func (r *CartRepository) RemoveItem(ctx context.Context, userID, skuID int64) (*domain.Cart, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: begin remove item: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `DELETE FROM cart_items WHERE user_id = $1 AND sku_id = $2`, userID, skuID)
	if err != nil {
		return nil, fmt.Errorf("postgres: remove item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrItemNotFound
	}

	if err := touchCart(ctx, tx, userID); err != nil {
		return nil, err
	}

	cart, err := loadCart(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("postgres: commit remove item: %w", err)
	}
	return cart, nil
}

// ToggleSelection инвертирует флаг выбора товара.
func (r *CartRepository) ToggleSelection(ctx context.Context, userID, skuID int64) (*domain.Cart, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: begin toggle: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE cart_items SET is_selected = NOT is_selected WHERE user_id = $1 AND sku_id = $2`,
		userID, skuID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: toggle selection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrItemNotFound
	}

	if err := touchCart(ctx, tx, userID); err != nil {
		return nil, err
	}

	cart, err := loadCart(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("postgres: commit toggle: %w", err)
	}
	return cart, nil
}

// SetPromoCode устанавливает (или снимает при nil) промокод корзины.
func (r *CartRepository) SetPromoCode(ctx context.Context, userID int64, code *string) (*domain.Cart, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: begin set promo: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO carts (user_id, promo_code, updated_at) VALUES ($1, $2, now())
		 ON CONFLICT (user_id) DO UPDATE SET promo_code = $2, updated_at = now()`,
		userID, code,
	); err != nil {
		return nil, fmt.Errorf("postgres: set promo code: %w", err)
	}

	cart, err := loadCart(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("postgres: commit set promo: %w", err)
	}
	return cart, nil
}

func touchCart(ctx context.Context, q querier, userID int64) error {
	if _, err := q.Exec(ctx, `UPDATE carts SET updated_at = now() WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("postgres: touch cart: %w", err)
	}
	return nil
}

func loadCart(ctx context.Context, q querier, userID int64) (*domain.Cart, error) {
	var (
		promoCode sql.NullString
		updatedAt sql.NullTime
	)

	err := q.QueryRow(ctx, `SELECT promo_code, updated_at FROM carts WHERE user_id = $1`, userID).
		Scan(&promoCode, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrCartNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: load cart header: %w", err)
	}

	cart := &domain.Cart{UserID: userID}
	if promoCode.Valid {
		code := promoCode.String
		cart.PromoCode = &code
	}
	if updatedAt.Valid {
		cart.UpdatedAt = updatedAt.Time
	}

	rows, err := q.Query(ctx,
		`SELECT sku_id, quantity, is_selected, price_at_addition, added_at
		 FROM cart_items WHERE user_id = $1 ORDER BY added_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: load cart items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			item  domain.CartItem
			price int64
		)
		item.UserID = userID
		if err := rows.Scan(&item.SKUID, &item.Quantity, &item.IsSelected, &price, &item.AddedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan cart item: %w", err)
		}
		item.PriceAtAddition = domain.Money(price)
		cart.Items = append(cart.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate cart items: %w", err)
	}

	return cart, nil
}
