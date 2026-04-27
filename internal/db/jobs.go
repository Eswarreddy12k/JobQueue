package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mini-job-queue/internal/models"
)

// InsertJob inserts a new job with status=pending.
func InsertJob(ctx context.Context, pool *pgxpool.Pool, payload []byte) (*models.Job, error) {
	row := pool.QueryRow(ctx, `
		INSERT INTO jobs (payload)
		VALUES ($1)
		RETURNING id, payload, status, retry_count, COALESCE(error,''), created_at, updated_at
	`, payload)

	var j models.Job
	if err := row.Scan(&j.ID, &j.Payload, &j.Status, &j.RetryCount, &j.Error, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert job: %w", err)
	}
	return &j, nil
}

// GetJob fetches a single job by ID.
func GetJob(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (*models.Job, error) {
	row := pool.QueryRow(ctx, `
		SELECT id, payload, status, retry_count, COALESCE(error,''), created_at, updated_at
		FROM jobs WHERE id = $1
	`, id)

	var j models.Job
	if err := row.Scan(&j.ID, &j.Payload, &j.Status, &j.RetryCount, &j.Error, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return &j, nil
}

// ClaimJob atomically picks one pending job and marks it running.
// Uses FOR UPDATE SKIP LOCKED so multiple workers never grab the same job.
func ClaimJob(ctx context.Context, pool *pgxpool.Pool) (*models.Job, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT id, payload, status, retry_count, COALESCE(error,''), created_at, updated_at
		FROM jobs
		WHERE status = 'pending'
		ORDER BY created_at
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`)

	var j models.Job
	if err := row.Scan(&j.ID, &j.Payload, &j.Status, &j.RetryCount, &j.Error, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return nil, err // includes pgx.ErrNoRows when queue is empty
	}

	_, err = tx.Exec(ctx, `
		UPDATE jobs SET status = 'running', updated_at = NOW() WHERE id = $1
	`, j.ID)
	if err != nil {
		return nil, fmt.Errorf("mark running: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	j.Status = models.StatusRunning
	return &j, nil
}

// MarkRunning sets a job's status to running. Used after dequeuing from Redis.
func MarkRunning(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	_, err := pool.Exec(ctx, `
		UPDATE jobs SET status = 'running', updated_at = NOW()
		WHERE id = $1 AND status = 'pending'
	`, id)
	return err
}

const maxRetries = 3

// MarkDone marks a job as done.
func MarkDone(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	_, err := pool.Exec(ctx, `
		UPDATE jobs SET status = 'done', updated_at = NOW() WHERE id = $1
	`, id)
	return err
}

// MarkFailed either retries a job (if under the retry limit) or marks it dead.
// Returns dead=true when retries are exhausted (job should go to DLQ).
func MarkFailed(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, jobErr string) (dead bool, err error) {
	var newStatus string
	row := pool.QueryRow(ctx, `
		UPDATE jobs
		SET
			retry_count = retry_count + 1,
			status      = CASE WHEN retry_count + 1 < $2 THEN 'pending' ELSE 'dead' END,
			error       = $3,
			updated_at  = NOW()
		WHERE id = $1
		RETURNING status
	`, id, maxRetries, jobErr)

	if err := row.Scan(&newStatus); err != nil {
		return false, fmt.Errorf("mark failed: %w", err)
	}
	return newStatus == string(models.StatusDead), nil
}

// GetStaleRunningJobs finds jobs stuck in "running" for longer than staleDuration.
// These are likely from crashed workers and need to be re-enqueued.
func GetStaleRunningJobs(ctx context.Context, pool *pgxpool.Pool, staleSeconds int) ([]uuid.UUID, error) {
	rows, err := pool.Query(ctx, `
		UPDATE jobs
		SET status = 'pending', updated_at = NOW()
		WHERE status = 'running' AND updated_at < NOW() - make_interval(secs => $1)
		RETURNING id
	`, staleSeconds)
	if err != nil {
		return nil, fmt.Errorf("get stale jobs: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
