// Package redis реализует кэш-репозиторий корзины на go-redis/v9.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/max25/smart-cart-service/internal/domain"
)

// CartCache — кэш активных корзин в Redis/Valkey.
type CartCache struct {
	client *goredis.Client
}

// NewCartCache создаёт кэш корзин.
func NewCartCache(client *goredis.Client) *CartCache {
	return &CartCache{client: client}
}

func cacheKey(userID int64) string {
	return fmt.Sprintf("cart:user:%d", userID)
}

// Get возвращает корзину из кэша. Промах кэша — (nil, nil).
func (c *CartCache) Get(ctx context.Context, userID int64) (*domain.Cart, error) {
	data, err := c.client.Get(ctx, cacheKey(userID)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get cart: %w", err)
	}

	var cart domain.Cart
	if err := json.Unmarshal(data, &cart); err != nil {
		return nil, fmt.Errorf("redis: unmarshal cart: %w", err)
	}

	return &cart, nil
}

// Set сохраняет корзину в кэш с заданным TTL.
func (c *CartCache) Set(ctx context.Context, cart *domain.Cart, ttl time.Duration) error {
	data, err := json.Marshal(cart)
	if err != nil {
		return fmt.Errorf("redis: marshal cart: %w", err)
	}

	if err := c.client.Set(ctx, cacheKey(cart.UserID), data, ttl).Err(); err != nil {
		return fmt.Errorf("redis: set cart: %w", err)
	}
	return nil
}

// Delete инвалидирует кэш корзины.
func (c *CartCache) Delete(ctx context.Context, userID int64) error {
	if err := c.client.Del(ctx, cacheKey(userID)).Err(); err != nil {
		return fmt.Errorf("redis: delete cart: %w", err)
	}
	return nil
}
