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

Phase 1 test

To test it

docker compose up -d                    # starts Postgres + Redis
go run ./cmd/api                        # terminal 1
go run ./cmd/worker                     # terminal 2

# Normal job — instant processing
curl -X POST localhost:8080/jobs -d '{"task":"send_email"}'

# Poison job — retries 3x then goes to DLQ
curl -X POST localhost:8080/jobs -d '{"task":"poison"}'

# Check the dead-letter queue
curl localhost:8080/jobs/dead









Phase 2 test
To deploy
Before running these, make sure you:

Enable Kubernetes in Docker Desktop (Settings → Kubernetes → Enable)
brew install kubectl
Then:


# Build the Docker image
docker build -t mini-job-queue:local .

# Deploy everything
kubectl apply -f k8s/

# Watch pods come up
kubectl get pods -n jobqueue -w

# Test it
curl -X POST http://localhost:30080/jobs \
  -H "Content-Type: application/json" \
  -d '{"task":"hello-k8s"}'

# Check worker logs
kubectl logs -n jobqueue -l app=worker

# Scale workers
kubectl scale deployment worker -n jobqueue --replicas=5








Phase3
To deploy

docker build -t mini-job-queue:local .
kubectl apply -f k8s/autoscaler.yaml

# Watch it work
kubectl -n jobqueue logs -f deploy/autoscaler

# Burst jobs and watch workers scale
for i in $(seq 1 50); do
  curl -s -X POST localhost:30080/jobs -d "{\"task\":\"work-$i\"}"
done
kubectl -n jobqueue get deploy worker -w