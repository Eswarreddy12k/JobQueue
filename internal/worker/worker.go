package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"mini-job-queue/internal/db"
	"mini-job-queue/internal/queue"
)

// Run blocks until ctx is cancelled, processing jobs from the Redis queue.
// It also starts a recovery sweep goroutine that re-enqueues stale running jobs.
func Run(ctx context.Context, pool *pgxpool.Pool, rdb *redis.Client) {
	log.Println("worker started")

	// Start the recovery sweep in a background goroutine
	go recoverySweep(ctx, pool, rdb)

	for {
		select {
		case <-ctx.Done():
			log.Println("worker stopped")
			return
		default:
			if err := processOne(ctx, pool, rdb); err != nil {
				// redis.Nil means BRPOP timed out — no job available, just loop again
				if err != redis.Nil {
					log.Printf("worker error: %v", err)
				}
			}
		}
	}
}

// recoverySweep runs every 30s, finding jobs stuck in "running" for over 60s
// (likely from crashed workers) and re-enqueuing them.
func recoverySweep(ctx context.Context, pool *pgxpool.Pool, rdb *redis.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ids, err := db.GetStaleRunningJobs(ctx, pool, 60)
			if err != nil {
				log.Printf("recovery sweep error: %v", err)
				continue
			}
			for _, id := range ids {
				if err := queue.Enqueue(ctx, rdb, id); err != nil {
					log.Printf("recovery re-enqueue error for %s: %v", id, err)
					continue
				}
				log.Printf("recovery sweep: re-enqueued stale job %s", id)
			}
		}
	}
}

func processOne(ctx context.Context, pool *pgxpool.Pool, rdb *redis.Client) error {
	// Block up to 5s waiting for a job ID from Redis
	jobID, err := queue.Dequeue(ctx, rdb)
	if err != nil {
		return err
	}

	// Mark the job as running in Postgres
	if err := db.MarkRunning(ctx, pool, jobID); err != nil {
		return fmt.Errorf("mark running %s: %w", jobID, err)
	}

	// Fetch the full job to get the payload
	job, err := db.GetJob(ctx, pool, jobID)
	if err != nil {
		return fmt.Errorf("get job %s: %w", jobID, err)
	}

	log.Printf("processing job %s  payload=%s", job.ID, job.Payload)

	if err := handle(job.Payload); err != nil {
		log.Printf("job %s failed: %v (retry_count=%d)", job.ID, err, job.RetryCount)
		dead, markErr := db.MarkFailed(ctx, pool, job.ID, err.Error())
		if markErr != nil {
			return fmt.Errorf("mark failed %s: %w", job.ID, markErr)
		}
		if dead {
			log.Printf("job %s moved to dead-letter queue", job.ID)
			return queue.SendToDLQ(ctx, rdb, job.ID)
		}
		// Not dead yet — re-enqueue for retry
		return queue.Enqueue(ctx, rdb, job.ID)
	}

	log.Printf("job %s done", job.ID)
	return db.MarkDone(ctx, pool, job.ID)
}

// handle contains your actual business logic.
func handle(payload []byte) error {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("bad payload: %w", err)
	}

	task, _ := data["task"].(string)

	// Poison task — always fails, useful for testing the DLQ
	if task == "poison" {
		return fmt.Errorf("poison task: always fails")
	}

	// Simulate work
	time.Sleep(100 * time.Millisecond)

	log.Printf("  -> executed task: %q", task)
	return nil
}
