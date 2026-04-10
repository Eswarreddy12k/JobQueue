package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mini-job-queue/internal/db"
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	worker.Run(ctx, pool)
}
