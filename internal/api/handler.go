package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mini-job-queue/internal/db"
)

type Handler struct {
	Pool *pgxpool.Pool
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
