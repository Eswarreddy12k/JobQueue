package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Connect creates a Redis client and verifies the connection.
// addr example: "localhost:6379"
func Connect(ctx context.Context, addr string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return rdb, nil
}
