package config

import (
	"fmt"
	"os"
	"strconv"

	"strings"
	"time"
)

// Config содержит все настройки сервиса, считываемые из переменных окружения.
type Config struct {
	HTTPAddr          string
	PostgresDSN       string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	CartCacheTTL      time.Duration
	EnrichmentTimeout time.Duration
	ShutdownTimeout   time.Duration
	LogLevel          string
}

// Load считывает конфигурацию из переменных окружения и подставляет значения по умолчанию.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		PostgresDSN:       getEnv("POSTGRES_DSN", "postgres://cart:cart@localhost:5432/cart_db?sslmode=disable"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		CartCacheTTL:      24 * time.Hour,
		EnrichmentTimeout: 2 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		RedisDB:           0,
	}

	redisDB, err := getEnvInt("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	cfg.RedisDB = redisDB

	ttl, err := getEnvDuration("CART_CACHE_TTL", cfg.CartCacheTTL)
	if err != nil {
		return Config{}, err
	}
	cfg.CartCacheTTL = ttl

	enrichTimeout, err := getEnvDuration("ENRICHMENT_TIMEOUT", cfg.EnrichmentTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.EnrichmentTimeout = enrichTimeout

	shutdownTimeout, err := getEnvDuration("SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.ShutdownTimeout = shutdownTimeout

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid int for %s: %w", key, err)
	}
	return n, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s: %w", key, err)
	}
	return d, nil
}
