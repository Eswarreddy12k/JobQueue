# Mini Job Queue

Distributed task queue with autoscaling, observability, and CI/CD.

**Go, PostgreSQL, Redis, Kubernetes, Prometheus, Grafana, AWS EKS, GitHub Actions**

## Architecture

```
                         ┌──────────────┐
  curl POST /jobs ──────►│   API :8080  │──── Postgres (source of truth)
                         └──────┬───────┘
                                │ LPUSH
                         ┌──────▼───────┐
                         │ Redis Queue  │
                         └──────┬───────┘
                                │ BRPOP
                    ┌───────────┼───────────┐
                    ▼           ▼           ▼
               ┌────────┐ ┌────────┐ ┌────────┐
               │Worker 1│ │Worker 2│ │Worker N│  ◄── Autoscaler scales 1-10
               └────────┘ └────────┘ └────────┘
                                │
                    Prometheus scrapes /metrics
                                │
                         ┌──────▼───────┐
                         │   Grafana    │
                         └──────────────┘
```

## Project Layout

```
mini-job-queue/
├── cmd/
│   ├── api/              # HTTP server (submit & query jobs)
│   ├── worker/           # Background worker (processes jobs from Redis)
│   └── autoscaler/       # Custom K8s controller (scales workers)
├── internal/
│   ├── api/              # HTTP handlers
│   ├── autoscaler/       # Autoscaler control loop
│   ├── db/               # All SQL queries (pgx)
│   ├── metrics/          # Prometheus metrics, middleware, server
│   ├── models/           # Job struct
│   ├── queue/            # Redis queue operations (enqueue, dequeue, DLQ)
│   ├── redis/            # Redis connection
│   └── worker/           # Processing loop with retry + DLQ
├── k8s/                  # Kubernetes manifests
├── scripts/              # AWS setup/teardown scripts
├── migrations/           # SQL schema
├── .github/workflows/    # CI/CD pipeline
├── Dockerfile            # Multi-stage build (3 binaries)
├── Makefile              # Build, test, push, deploy commands
└── docker-compose.yml    # Local Postgres + Redis
```

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- Kubernetes enabled in Docker Desktop (Settings → Kubernetes → Enable)

## Quick Start (Local Development)

```bash
# Start Postgres + Redis
docker compose up -d

# Run API (terminal 1)
go run ./cmd/api

# Run worker (terminal 2)
go run ./cmd/worker

# Submit a job
curl -X POST localhost:8080/jobs -d '{"task":"send_email","to":"hello@example.com"}'

# Check job status
curl localhost:8080/jobs/<id>

# Poison job — retries 3x then goes to DLQ
curl -X POST localhost:8080/jobs -d '{"task":"poison"}'

# Check the dead-letter queue
curl localhost:8080/jobs/dead
```

## Deploy to Local Kubernetes

```bash
# Build the Docker image
docker build -t mini-job-queue:local .

# Load image into K8s (Docker Desktop with containerd)
docker save mini-job-queue:local | docker exec -i desktop-control-plane \
    ctr --namespace k8s.io images import -

# Deploy everything
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/

# Watch pods come up
kubectl get pods -n jobqueue -w

# Port-forward services to localhost
kubectl port-forward -n jobqueue svc/api 30080:8080 &
kubectl port-forward -n jobqueue svc/prometheus 30090:9090 &
kubectl port-forward -n jobqueue svc/grafana 30030:3000 &

# Test it
curl -X POST localhost:30080/jobs -d '{"task":"hello-k8s"}'

# Burst jobs and watch autoscaler
for i in $(seq 1 50); do
  curl -s -X POST localhost:30080/jobs -d "{\"task\":\"work-$i\"}"
done
kubectl -n jobqueue get deploy worker -w

# Check worker logs
kubectl logs -n jobqueue -l app=worker

# Check autoscaler logs
kubectl -n jobqueue logs -f deploy/autoscaler
```

**Dashboards:**
- Prometheus: http://localhost:30090
- Grafana: http://localhost:30030 (Dashboard: "Mini Job Queue")

## Deploy to AWS EKS

Requires: [AWS CLI](https://aws.amazon.com/cli/), [eksctl](https://eksctl.io/)

```bash
# 1. Set your GitHub username and run setup (~15 min for EKS)
export GITHUB_USER=<your-username>
./scripts/setup-aws.sh

# 2. Add AWS_ACCOUNT_ID as a GitHub repository secret
#    (Settings → Secrets and variables → Actions → New repository secret)

# 3. Push to main — CI/CD builds, pushes to ECR, and deploys
git push origin main

# 4. Get the API endpoint (AWS ELB hostname)
kubectl get svc api -n jobqueue -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'

# 5. IMPORTANT: Teardown when done to stop charges (~$4.30/day)
./scripts/teardown-aws.sh
```

## Makefile Commands

```bash
make build          # Build all Go binaries to bin/
make test           # Run tests
make docker-build   # Build Docker image tagged with git SHA
make docker-push    # Push to ECR
make deploy         # Rolling update all deployments on EKS
```

## Key Concepts

| Concept | Where to look |
|---|---|
| Job model + statuses | `internal/models/job.go` |
| SQL queries (pgx) | `internal/db/jobs.go` |
| `FOR UPDATE SKIP LOCKED` | `internal/db/jobs.go` — prevents two workers grabbing the same job |
| Redis queue (LPUSH/BRPOP) | `internal/queue/queue.go` |
| Dead-letter queue | `internal/queue/queue.go` + `internal/worker/worker.go` |
| Recovery sweep | `internal/worker/worker.go` — re-enqueues stale running jobs |
| Custom K8s autoscaler | `internal/autoscaler/scaler.go` — poll-decide-act control loop |
| Prometheus metrics | `internal/metrics/metrics.go` — counters, gauges, histograms |
| HTTP middleware | `internal/metrics/middleware.go` — request duration + count |
| OIDC auth (no static keys) | `.github/workflows/deploy.yml` + `scripts/setup-aws.sh` |
| Multi-stage Docker build | `Dockerfile` — golang:1.24-alpine → alpine:3.19 |
