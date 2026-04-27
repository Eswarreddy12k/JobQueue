package models

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
	StatusDead    Status = "dead"
)

type Job struct {
	ID         uuid.UUID  `json:"id"`
	Payload    []byte     `json:"payload"`  // raw JSON
	Status     Status     `json:"status"`
	RetryCount int        `json:"retry_count"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
