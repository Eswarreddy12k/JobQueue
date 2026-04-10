package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mini-job-queue/internal/db"
)

// Run polls the DB for pending jobs and processes them.
// It blocks until ctx is cancelled.
func Run(ctx context.Context, pool *pgxpool.Pool) {
	log.Println("worker started")
	for {
		select {
		case <-ctx.Done():
			log.Println("worker stopped")
			return
		default:
			if err := processOne(ctx, pool); err != nil {
				if err != pgx.ErrNoRows {
					log.Printf("worker error: %v", err)
				}
				// No job available — wait before polling again
				time.Sleep(2 * time.Second)
			}
		}
	}
}

func processOne(ctx context.Context, pool *pgxpool.Pool) error {
	job, err := db.ClaimJob(ctx, pool)
	if err != nil {
		return err // pgx.ErrNoRows means empty queue
	}

	log.Printf("processing job %s  payload=%s", job.ID, job.Payload)

	if err := handle(job.Payload); err != nil {
		log.Printf("job %s failed: %v (retry_count=%d)", job.ID, err, job.RetryCount)
		return db.MarkFailed(ctx, pool, job.ID, err.Error())
	}

	log.Printf("job %s done", job.ID)
	return db.MarkDone(ctx, pool, job.ID)
}

// handle contains your actual business logic.
// Replace this with whatever the job should do.
func handle(payload []byte) error {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("bad payload: %w", err)
	}

	// Simulate work
	time.Sleep(100 * time.Millisecond)

	task, _ := data["task"].(string)
	log.Printf("  -> executed task: %q", task)
	return nil
}
