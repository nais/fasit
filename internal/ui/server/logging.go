package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type metricsMiddleware struct {
	requestsTotal   metric.Int64Counter
	requestDuration metric.Float64Histogram
}

func NewMetricsMiddleware(meter metric.Meter) (*metricsMiddleware, error) {
	requestsTotal, err := meter.Int64Counter("http_requests_total",
		metric.WithDescription("Total HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram("http_request_duration_ms",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000),
	)
	if err != nil {
		return nil, err
	}

	return &metricsMiddleware{
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
	}, nil
}

var skipMetricsPaths = map[string]bool{
	"/site.css":       true,
	"/site.js":        true,
	"/favicon.ico":    true,
	"/deployments.js": true,
	"/features.js":    true,
	"/reconciler.js":  true,
}

func (m *metricsMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		if skipMetricsPaths[r.URL.Path] {
			return
		}

		pattern := r.URL.Path
		if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
			pattern = rctx.RoutePattern()
		}

		attrs := metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("path", pattern),
			attribute.String("status", strconv.Itoa(rw.status)),
		)

		m.requestsTotal.Add(r.Context(), 1, attrs)
		m.requestDuration.Record(r.Context(), float64(time.Since(start).Milliseconds()),
			metric.WithAttributes(
				attribute.String("method", r.Method),
				attribute.String("path", pattern),
			),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
