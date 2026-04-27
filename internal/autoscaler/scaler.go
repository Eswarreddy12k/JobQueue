package autoscaler

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Config holds all tunable parameters for the autoscaler.
type Config struct {
	Namespace         string
	DeploymentName    string
	MinReplicas       int32
	MaxReplicas       int32
	JobsPerWorker     int32
	PollInterval      time.Duration
	ScaleUpCooldown   time.Duration
	ScaleDownCooldown time.Duration
}

// Scaler monitors queue depth and adjusts worker replica count.
type Scaler struct {
	cfg           Config
	kubeClient    kubernetes.Interface
	rdb           *redis.Client
	pool          *pgxpool.Pool
	lastScaleUp   time.Time
	lastScaleDown time.Time
}

// New creates a Scaler with the given dependencies.
func New(cfg Config, kubeClient kubernetes.Interface, rdb *redis.Client, pool *pgxpool.Pool) *Scaler {
	return &Scaler{
		cfg:        cfg,
		kubeClient: kubeClient,
		rdb:        rdb,
		pool:       pool,
	}
}

// Run starts the autoscaler control loop. Blocks until ctx is cancelled.
func (s *Scaler) Run(ctx context.Context) {
	log.Printf("autoscaler started (poll=%s, min=%d, max=%d, jobsPerWorker=%d)",
		s.cfg.PollInterval, s.cfg.MinReplicas, s.cfg.MaxReplicas, s.cfg.JobsPerWorker)

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("autoscaler stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick runs one iteration of the control loop: poll → decide → act.
func (s *Scaler) tick(ctx context.Context) {
	pending, running, err := s.getMetrics(ctx)
	if err != nil {
		log.Printf("autoscaler: metrics error: %v", err)
		return
	}

	desired := s.desiredReplicas(pending, running)

	// Get current replica count from K8s
	deploy, err := s.kubeClient.AppsV1().Deployments(s.cfg.Namespace).Get(
		ctx, s.cfg.DeploymentName, metav1.GetOptions{},
	)
	if err != nil {
		log.Printf("autoscaler: get deployment: %v", err)
		return
	}
	current := *deploy.Spec.Replicas

	if desired == current {
		log.Printf("autoscaler: pending=%d running=%d replicas=%d (no change)", pending, running, current)
		return
	}

	now := time.Now()

	if desired > current {
		// Scale UP
		if now.Sub(s.lastScaleUp) < s.cfg.ScaleUpCooldown {
			log.Printf("autoscaler: scale-up cooldown active, skipping (%d -> %d)", current, desired)
			return
		}
		log.Printf("autoscaler: scaling UP %d -> %d (pending=%d, running=%d)", current, desired, pending, running)
		s.lastScaleUp = now
	} else {
		// Scale DOWN
		if now.Sub(s.lastScaleDown) < s.cfg.ScaleDownCooldown {
			log.Printf("autoscaler: scale-down cooldown active, skipping (%d -> %d)", current, desired)
			return
		}
		log.Printf("autoscaler: scaling DOWN %d -> %d (pending=%d, running=%d)", current, desired, pending, running)
		s.lastScaleDown = now
	}

	if err := s.scale(ctx, desired); err != nil {
		log.Printf("autoscaler: scale error: %v", err)
	}
}

// getMetrics reads queue depth from Redis and in-flight count from Postgres.
func (s *Scaler) getMetrics(ctx context.Context) (pending, running int64, err error) {
	pending, err = s.rdb.LLen(ctx, "jobs:pending").Result()
	if err != nil {
		return 0, 0, fmt.Errorf("redis LLEN: %w", err)
	}

	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE status = 'running'`).Scan(&running)
	if err != nil {
		return 0, 0, fmt.Errorf("postgres count: %w", err)
	}

	return pending, running, nil
}

// desiredReplicas computes how many workers should be running.
func (s *Scaler) desiredReplicas(pending, running int64) int32 {
	// Workers needed for pending backlog
	neededForPending := int32(math.Ceil(float64(pending) / float64(s.cfg.JobsPerWorker)))

	// Workers needed for currently running jobs (1 job = 1 worker)
	neededForRunning := int32(running)

	// Use the larger of the two — don't scale down while workers are busy
	desired := neededForPending
	if neededForRunning > desired {
		desired = neededForRunning
	}

	// Clamp to [min, max]
	if desired < s.cfg.MinReplicas {
		desired = s.cfg.MinReplicas
	}
	if desired > s.cfg.MaxReplicas {
		desired = s.cfg.MaxReplicas
	}
	return desired
}

// scale updates the worker Deployment's replica count via the K8s Scale subresource.
func (s *Scaler) scale(ctx context.Context, replicas int32) error {
	sc, err := s.kubeClient.AppsV1().Deployments(s.cfg.Namespace).GetScale(
		ctx, s.cfg.DeploymentName, metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("get scale: %w", err)
	}
	sc.Spec.Replicas = replicas
	_, err = s.kubeClient.AppsV1().Deployments(s.cfg.Namespace).UpdateScale(
		ctx, s.cfg.DeploymentName, sc, metav1.UpdateOptions{},
	)
	if err != nil {
		return fmt.Errorf("update scale: %w", err)
	}
	return nil
}
