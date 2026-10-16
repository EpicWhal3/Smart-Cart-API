// Точка входа
package main

import (
	"context"
	"database/sql"
	"errors"

	"log/slog"
	"net/http"
	"os"

	"os/signal"
	"syscall"
	"time"

	"github.com/pressly/goose/v3"
	goredis "github.com/redis/go-redis/v9"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/max25/smart-cart-service/internal/config"
	httpcontroller "github.com/max25/smart-cart-service/internal/controller/http"
	pgrepo "github.com/max25/smart-cart-service/internal/repository/postgres"
	redisrepo "github.com/max25/smart-cart-service/internal/repository/redis"
	"github.com/max25/smart-cart-service/internal/usecase"
	"github.com/max25/smart-cart-service/internal/usecase/bdui"
	"github.com/max25/smart-cart-service/internal/usecase/rules"
)

const migrationsDir = "migrations"

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	err = pool.Ping(pingCtx)
	cancelPing()
	if err != nil {
		return err
	}
	logger.Info("connected to postgres")

	if err := runMigrations(cfg.PostgresDSN, logger); err != nil {
		return err
	}

	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer func() { _ = redisClient.Close() }()

	pingRedisCtx, cancelRedis := context.WithTimeout(ctx, 2*time.Second)
	if err := redisClient.Ping(pingRedisCtx).Err(); err != nil {
		logger.Warn("redis unavailable at startup, continuing with postgres fallback",
			slog.String("error", err.Error()))
	} else {
		logger.Info("connected to redis")
	}
	cancelRedis()

	cartRepo := pgrepo.NewCartRepository(pool)
	catalogRepo := pgrepo.NewCatalogRepository(pool)
	promoRepo := pgrepo.NewPromoRepository(pool)
	cartCache := redisrepo.NewCartCache(redisClient)

	cartUseCase := usecase.New(usecase.Deps{
		Carts:         cartRepo,
		Catalog:       catalogRepo,
		Promos:        promoRepo,
		Cache:         cartCache,
		Engine:        rules.NewDefaultEngine(),
		Builder:       bdui.NewBuilder(),
		Logger:        logger,
		CacheTTL:      cfg.CartCacheTTL,
		EnrichTimeout: cfg.EnrichmentTimeout,
		SimilarLimit:  2,
	})

	handler := httpcontroller.NewRouter(cartUseCase, logger, cfg.EnrichmentTimeout)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server started", slog.String("addr", cfg.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("service stopped gracefully")
	return nil
}

// runMigrations прогоняет SQL-миграции goose через database/sql.
func runMigrations(dsn string, logger *slog.Logger) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return err
	}

	logger.Info("migrations applied", slog.String("dir", migrationsDir))
	return nil
}

// newLogger создаёт JSON-логгер slog с заданным уровнем.
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}
