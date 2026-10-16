package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/max25/smart-cart-service/internal/domain"
)

// PromoRepository — репозиторий промокодов на pgx/v5.
type PromoRepository struct {
	pool *pgxpool.Pool
}

// NewPromoRepository создаёт репозиторий промокодов.
func NewPromoRepository(pool *pgxpool.Pool) *PromoRepository {
	return &PromoRepository{pool: pool}
}

// GetPromoCode возвращает промокод или (nil, nil), если он не найден.
func (r *PromoRepository) GetPromoCode(ctx context.Context, code string) (*domain.PromoRule, error) {
	var (
		rule   domain.PromoRule
		minAmt int64
	)

	err := r.pool.QueryRow(ctx,
		`SELECT code, discount_percent, min_order_amount, is_active
		 FROM promo_codes WHERE code = $1`,
		code,
	).Scan(&rule.Code, &rule.DiscountPercent, &minAmt, &rule.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get promo code: %w", err)
	}

	rule.MinOrderAmount = domain.Money(minAmt)
	return &rule, nil
}
