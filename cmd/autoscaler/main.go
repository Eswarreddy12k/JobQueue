package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"mini-job-queue/internal/autoscaler"
	"mini-job-queue/internal/db"
	redisconn "mini-job-queue/internal/redis"
)

func main() {
	// Database connection
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/jobqueue?sslmode=disable"
	}
	pool, err := db.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	// Redis connection
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb, err := redisconn.Connect(context.Background(), redisAddr)
	if err != nil {
		log.Fatalf("redis connect: %v", err)
	}
	defer rdb.Close()

	// Kubernetes client — try in-cluster first (running in a pod),
	// fall back to local kubeconfig (running on your Mac)
	kubeClient, err := buildKubeClient()
	if err != nil {
		log.Fatalf("k8s client: %v", err)
	}

	// Parse autoscaler config from environment
	cfg := autoscaler.Config{
		Namespace:         envOrDefault("AUTOSCALER_NAMESPACE", "jobqueue"),
		DeploymentName:    envOrDefault("AUTOSCALER_DEPLOYMENT", "worker"),
		MinReplicas:       int32(envOrDefaultInt("AUTOSCALER_MIN_REPLICAS", 1)),
		MaxReplicas:       int32(envOrDefaultInt("AUTOSCALER_MAX_REPLICAS", 10)),
		JobsPerWorker:     int32(envOrDefaultInt("AUTOSCALER_JOBS_PER_WORKER", 5)),
		PollInterval:      envOrDefaultDuration("AUTOSCALER_POLL_INTERVAL", 15*time.Second),
		ScaleUpCooldown:   envOrDefaultDuration("AUTOSCALER_SCALEUP_COOLDOWN", 30*time.Second),
		ScaleDownCooldown: envOrDefaultDuration("AUTOSCALER_SCALEDOWN_COOLDOWN", 120*time.Second),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	scaler := autoscaler.New(cfg, kubeClient, rdb, pool)
	scaler.Run(ctx)
}

// buildKubeClient tries in-cluster config first, then falls back to kubeconfig.
func buildKubeClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Not running in a pod — use local kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(config)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envOrDefaultDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
