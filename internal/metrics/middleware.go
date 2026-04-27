package metrics

import (
	"net/http"
	"strconv"
	"time"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

// HTTPMetrics returns middleware that records request duration and count.
func HTTPMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}

		next.ServeHTTP(sw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(sw.code)
		path := r.URL.Path

		HTTPRequestDuration.WithLabelValues(r.Method, path, status).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
	})
}
