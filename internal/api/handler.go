package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"mini-job-queue/internal/db"
	"mini-job-queue/internal/metrics"
	"mini-job-queue/internal/models"
	"mini-job-queue/internal/queue"
)

type Handler struct {
	Pool *pgxpool.Pool
	RDB  *redis.Client
}

// POST /jobs  — submit a new job
// Body: any valid JSON object, e.g. {"task":"send_email","to":"x@y.com"}
func (h *Handler) SubmitJob(w http.ResponseWriter, r *http.Request) {
	var payload json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	job, err := db.InsertJob(r.Context(), h.Pool, payload)
	if err != nil {
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}

	metrics.JobsSubmittedTotal.Inc()

	// Enqueue to Redis for fast dispatch. If this fails the recovery sweep
	// will pick it up later — the job is safe in Postgres.
	if err := queue.Enqueue(r.Context(), h.RDB, job.ID); err != nil {
		log.Printf("warning: redis enqueue failed for job %s: %v", job.ID, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

// GET /jobs/{id}  — check job status
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id") // requires Go 1.22+
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}

	job, err := db.GetJob(r.Context(), h.Pool, id)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// GET /jobs/dead  — list jobs in the dead-letter queue
func (h *Handler) GetDeadJobs(w http.ResponseWriter, r *http.Request) {
	ids, err := queue.ListDLQ(r.Context(), h.RDB, 100)
	if err != nil {
		http.Error(w, "failed to list dead jobs", http.StatusInternalServerError)
		return
	}

	jobs := make([]*models.Job, 0, len(ids))
	for _, id := range ids {
		job, err := db.GetJob(r.Context(), h.Pool, id)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}
