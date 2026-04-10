# Mini Job Queue

A simple job queue in Go + PostgreSQL to learn the fundamentals before adding Redis / Kubernetes.

## Project Layout

```
mini-job-queue/
├── cmd/
│   ├── api/        # HTTP server (submit & query jobs)
│   └── worker/     # Background worker (polls & processes jobs)
├── internal/
│   ├── api/        # HTTP handlers
│   ├── db/         # All SQL queries
│   ├── models/     # Job struct
│   └── worker/     # Processing loop
├── migrations/     # SQL schema
└── docker-compose.yml
```

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/)

## Run it

### 1. Start PostgreSQL
```bash
docker compose up -d
```

### 2. Install dependencies
```bash
go mod tidy
```

### 3. Start the API server
```bash
go run ./cmd/api
```

### 4. Start a worker (new terminal)
```bash
go run ./cmd/worker
```

### 5. Submit a job
```bash
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{"task":"send_email","to":"hello@example.com"}'
```

### 6. Check job status
```bash
curl http://localhost:8080/jobs/<id>
```

## Key concepts in this project

| Concept | Where to look |
|---|---|
| Job struct | `internal/models/job.go` |
| All SQL queries | `internal/db/jobs.go` |
| `FOR UPDATE SKIP LOCKED` | `db.ClaimJob()` — prevents two workers grabbing the same job |
| Retry logic | `db.MarkFailed()` — resets to pending if under retry limit |
| HTTP handlers | `internal/api/handler.go` |
| Worker poll loop | `internal/worker/worker.go` |
