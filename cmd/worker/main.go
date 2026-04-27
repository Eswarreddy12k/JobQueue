package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mini-job-queue/internal/db"
	"mini-job-queue/internal/metrics"
	redisconn "mini-job-queue/internal/redis"
	"mini-job-queue/internal/worker"
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

	go metrics.StartMetricsServer(":9090")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	worker.Run(ctx, pool, rdb)
}
