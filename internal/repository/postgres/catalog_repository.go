package postgres

import (
	"context"
	"database/sql"
	"errors"

	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/max25/smart-cart-service/internal/domain"
)

// CatalogRepository — встроенный каталог товаров (Catalog Provider) на pgx/v5.
type CatalogRepository struct {
	pool *pgxpool.Pool
}

// NewCatalogRepository создаёт репозиторий каталога.
func NewCatalogRepository(pool *pgxpool.Pool) *CatalogRepository {
	return &CatalogRepository{pool: pool}
}

// GetProducts возвращает товары по списку SKU (отсутствующие просто не попадут в map).
func (r *CatalogRepository) GetProducts(ctx context.Context, skuIDs []int64) (map[int64]domain.Product, error) {
	products := make(map[int64]domain.Product, len(skuIDs))
	if len(skuIDs) == 0 {
		return products, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT sku_id, title, category_id, current_price, old_price, stock_quantity
		 FROM catalog_items WHERE sku_id = ANY($1)`,
		skuIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get products: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products[product.SKUID] = product
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate products: %w", err)
	}

	return products, nil
}

// GetSimilar подбирает до limit товаров-аналогов из той же категории, что и skuID,
// среди доступных (stock_quantity > 0), отсортированных по возрастанию цены.
func (r *CatalogRepository) GetSimilar(ctx context.Context, skuID int64, limit int) ([]domain.Product, error) {
	if limit <= 0 {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT c.sku_id, c.title, c.category_id, c.current_price, c.old_price, c.stock_quantity
		 FROM catalog_items c
		 JOIN catalog_items src ON src.sku_id = $1
		 WHERE c.category_id = src.category_id
		   AND c.sku_id <> src.sku_id
		   AND c.stock_quantity > 0
		 ORDER BY c.current_price ASC
		 LIMIT $2`,
		skuID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get similar: %w", err)
	}
	defer rows.Close()

	var products []domain.Product
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate similar: %w", err)
	}

	return products, nil
}

func scanProduct(row pgx.Row) (domain.Product, error) {
	var (
		product  domain.Product
		current  int64
		oldPrice sql.NullInt64
	)
	if err := row.Scan(
		&product.SKUID,
		&product.Title,
		&product.CategoryID,
		&current,
		&oldPrice,
		&product.StockQuantity,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Product{}, domain.ErrProductNotFound
		}
		return domain.Product{}, fmt.Errorf("postgres: scan product: %w", err)
	}

	product.CurrentPrice = domain.Money(current)
	if oldPrice.Valid {
		op := domain.Money(oldPrice.Int64)
		product.OldPrice = &op
	}

	return product, nil
}
