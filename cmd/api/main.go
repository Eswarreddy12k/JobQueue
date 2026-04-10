package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"mini-job-queue/internal/api"
	"mini-job-queue/internal/db"
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

	h := &api.Handler{Pool: pool}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /jobs", h.SubmitJob)
	mux.HandleFunc("GET /jobs/{id}", h.GetJob)

	addr := ":8080"
	log.Printf("API server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
