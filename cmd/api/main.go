package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"mini-job-queue/internal/api"
	"mini-job-queue/internal/db"
	"mini-job-queue/internal/metrics"
	redisconn "mini-job-queue/internal/redis"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/jobqueue?sslmode=disable"
	}

	pool, err := db.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb, err := redisconn.Connect(context.Background(), redisAddr)
	if err != nil {
		log.Fatalf("redis connect: %v", err)
	}
	defer rdb.Close()

	h := &api.Handler{Pool: pool, RDB: rdb}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /jobs", h.SubmitJob)
	mux.HandleFunc("GET /jobs/dead", h.GetDeadJobs)
	mux.HandleFunc("GET /jobs/{id}", h.GetJob)
	mux.Handle("GET /metrics", promhttp.Handler())

	addr := ":8080"
	log.Printf("API server listening on %s", addr)
	if err := http.ListenAndServe(addr, metrics.HTTPMetrics(mux)); err != nil {
		log.Fatal(err)
	}
}
