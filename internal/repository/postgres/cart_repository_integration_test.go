package postgres_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/max25/smart-cart-service/internal/domain"
	cartpg "github.com/max25/smart-cart-service/internal/repository/postgres"
	cartredis "github.com/max25/smart-cart-service/internal/repository/redis"
)

// TestCartRepositoryIntegration проверяет связку PostgreSQL + Redis (testcontainers).
func TestCartRepositoryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx := context.Background()

	// --- PostgreSQL ---
	pgContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("cart_db"),
		tcpostgres.WithUsername("cart"),
		tcpostgres.WithPassword("cart"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("docker unavailable, skipping integration test: %v", err)
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, runMigrations(dsn))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// --- Redis ---
	redisContainer, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Skipf("docker unavailable, skipping integration test: %v", err)
	}
	defer func() { _ = redisContainer.Terminate(ctx) }()

	endpoint, err := redisContainer.Endpoint(ctx, "")
	require.NoError(t, err)

	client := goredis.NewClient(&goredis.Options{Addr: endpoint})
	defer func() { _ = client.Close() }()

	repo := cartpg.NewCartRepository(pool)
	cache := cartredis.NewCartCache(client)

	const userID int64 = 1001

	// AddItem.
	cart, err := repo.AddItem(ctx, userID, 101, 1, domain.Money(135000))
	require.NoError(t, err)
	require.Len(t, cart.Items, 1)
	require.Equal(t, 1, cart.Items[0].Quantity)
	require.True(t, cart.Items[0].IsSelected)

	// Кэш: запись и чтение.
	require.NoError(t, cache.Set(ctx, cart, time.Hour))
	cached, err := cache.Get(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, cached)
	require.Equal(t, cart.UserID, cached.UserID)
	require.Len(t, cached.Items, 1)
	require.Equal(t, domain.Money(135000), cached.Items[0].PriceAtAddition)

	// UpdateItemQuantity.
	cart, err = repo.UpdateItemQuantity(ctx, userID, 101, 2)
	require.NoError(t, err)
	require.Equal(t, 2, cart.Items[0].Quantity)

	// ToggleSelection.
	cart, err = repo.ToggleSelection(ctx, userID, 101)
	require.NoError(t, err)
	require.False(t, cart.Items[0].IsSelected)

	// SetPromoCode.
	promo := "OZON10"
	cart, err = repo.SetPromoCode(ctx, userID, &promo)
	require.NoError(t, err)
	require.NotNil(t, cart.PromoCode)
	require.Equal(t, "OZON10", *cart.PromoCode)

	// RemoveItem.
	cart, err = repo.RemoveItem(ctx, userID, 101)
	require.NoError(t, err)
	require.Empty(t, cart.Items)

	// Отсутствующая корзина.
	_, err = repo.GetCart(ctx, 999999)
	require.ErrorIs(t, err, domain.ErrCartNotFound)
}

// TestCartCacheFallback проверяет, что при недоступности Redis кэш возвращает ошибку,
// которую usecase-слой трактует как сигнал к чтению из PostgreSQL (Graceful Fallback).
func TestCartCacheFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 200 * time.Millisecond,
		MaxRetries:  -1,
	})
	defer func() { _ = client.Close() }()

	cache := cartredis.NewCartCache(client)

	_, err := cache.Get(context.Background(), 1)
	require.Error(t, err, "при недоступном Redis должен вернуться error для fallback")
}

func runMigrations(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "../../../migrations")
}
