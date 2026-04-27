package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// API metrics
	JobsSubmittedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "jobqueue_jobs_submitted_total",
		Help: "Total number of jobs submitted via the API.",
	})

	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "jobqueue_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status_code"})

	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "jobqueue_http_requests_total",
		Help: "Total HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "status_code"})

	// Worker metrics
	JobsProcessedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "jobqueue_jobs_processed_total",
		Help: "Total jobs processed, labeled by outcome.",
	}, []string{"status"})

	JobProcessingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "jobqueue_job_processing_duration_seconds",
		Help:    "Time spent processing a single job.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	})

	WorkerBusy = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobqueue_worker_busy",
		Help: "1 if this worker is currently processing a job, 0 if idle.",
	})

	// Queue / Autoscaler metrics
	QueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobqueue_queue_depth",
		Help: "Number of pending jobs in the Redis queue.",
	})

	RunningJobs = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobqueue_running_jobs",
		Help: "Number of jobs currently in running state.",
	})

	AutoscalerCurrentReplicas = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobqueue_autoscaler_current_replicas",
		Help: "Current number of worker replicas.",
	})

	AutoscalerDesiredReplicas = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobqueue_autoscaler_desired_replicas",
		Help: "Desired number of worker replicas as computed by autoscaler.",
	})
)

func init() {
	prometheus.MustRegister(
		JobsSubmittedTotal,
		HTTPRequestDuration, HTTPRequestsTotal,
		JobsProcessedTotal, JobProcessingDuration, WorkerBusy,
		QueueDepth, RunningJobs,
		AutoscalerCurrentReplicas, AutoscalerDesiredReplicas,
	)
}
