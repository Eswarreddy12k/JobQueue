package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	keyPending = "jobs:pending"
	keyDead    = "jobs:dead"
)

// Enqueue pushes a job ID onto the pending queue.
func Enqueue(ctx context.Context, rdb *redis.Client, jobID uuid.UUID) error {
	return rdb.LPush(ctx, keyPending, jobID.String()).Err()
}

// Dequeue blocks until a job ID is available (up to 5s) and returns it.
// Returns uuid.Nil and an error if the timeout elapses with no job.
func Dequeue(ctx context.Context, rdb *redis.Client) (uuid.UUID, error) {
	result, err := rdb.BRPop(ctx, 5*time.Second, keyPending).Result()
	if err != nil {
		return uuid.Nil, err
	}
	// BRPop returns [key, value]
	id, err := uuid.Parse(result[1])
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse job id from redis: %w", err)
	}
	return id, nil
}

// SendToDLQ pushes a job ID onto the dead-letter queue.
func SendToDLQ(ctx context.Context, rdb *redis.Client, jobID uuid.UUID) error {
	return rdb.LPush(ctx, keyDead, jobID.String()).Err()
}

// ListDLQ returns up to limit job IDs from the dead-letter queue.
func ListDLQ(ctx context.Context, rdb *redis.Client, limit int) ([]uuid.UUID, error) {
	vals, err := rdb.LRange(ctx, keyDead, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(vals))
	for _, v := range vals {
		id, err := uuid.Parse(v)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}
