package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/supporttools/prometheus-tunnel/pkg/config"
	"github.com/supporttools/prometheus-tunnel/pkg/health"
	"github.com/supporttools/prometheus-tunnel/pkg/logging"
)

var logger = logging.SetupLogging(config.CFG.Debug)

var (
	// TotalRequests tracks the total number of requests received
	TotalRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "proxy_total_requests",
			Help: "Total number of requests received",
		},
	)
	// RequestDuration tracks the duration of requests
	RequestDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "proxy_request_duration_seconds",
			Help:    "Histogram of response latency (seconds) of requests.",
			Buckets: prometheus.DefBuckets,
		},
	)
	// ResponseStatus tracks the total number of responses sent, partitioned by status code
	ResponseStatus = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxy_response_status_total",
			Help: "Total number of responses sent, partitioned by status code",
		},
		[]string{"status"},
	)
)

func init() {
	logger.Printf("Initializing Prometheus metrics\n")
	// Register custom metrics with Prometheus's DefaultRegisterer.
	prometheus.MustRegister(
		TotalRequests,
		RequestDuration,
		ResponseStatus,
	)
}

// StartMetricsServer starts the metrics server
func StartMetricsServer() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", health.HealthzHandler())
	mux.HandleFunc("/readyz", health.ReadyzHandler())
	mux.HandleFunc("/version", health.VersionHandler())

	serverPortStr := strconv.Itoa(config.CFG.MetricsPort)
	logger.Printf("Metrics server starting on port %s\n", serverPortStr)

	srv := &http.Server{
		Addr:         ":" + serverPortStr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		logger.Fatalf("Metrics server failed to start: %v", err)
	}
}

// RecordMetrics wraps the HTTP handler to record Prometheus metrics
func RecordMetrics(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures the status code
		ww := &statusCapturingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		RequestDuration.Observe(duration)
		TotalRequests.Inc()
		ResponseStatus.WithLabelValues(strconv.Itoa(ww.statusCode)).Inc()
	}
}

// statusCapturingResponseWriter is a wrapper around http.ResponseWriter that captures the status code
type statusCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (w *statusCapturingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
