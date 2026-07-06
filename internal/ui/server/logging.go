package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/database"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type metricsMiddleware struct {
	requestsTotal     metric.Int64Counter
	requestDuration   metric.Float64Histogram
	queriesPerRequest metric.Int64Histogram
	dbTimePerRequest  metric.Float64Histogram
}

func NewMetricsMiddleware(meter metric.Meter) (*metricsMiddleware, error) {
	requestsTotal, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"http_request_duration_ms",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000),
	)
	if err != nil {
		return nil, err
	}

	queriesPerRequest, err := meter.Int64Histogram(
		"db_queries_per_request",
		metric.WithDescription("Number of database queries executed while serving one HTTP request"),
		metric.WithExplicitBucketBoundaries(1, 2, 5, 10, 20, 50, 100, 200),
	)
	if err != nil {
		return nil, err
	}

	dbTimePerRequest, err := meter.Float64Histogram(
		"db_time_per_request_ms",
		metric.WithDescription("Total time spent in the database while serving one HTTP request"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000),
	)
	if err != nil {
		return nil, err
	}

	return &metricsMiddleware{
		requestsTotal:     requestsTotal,
		requestDuration:   requestDuration,
		queriesPerRequest: queriesPerRequest,
		dbTimePerRequest:  dbTimePerRequest,
	}, nil
}

var skipMetricsPaths = map[string]bool{
	"/site.css":       true,
	"/site.js":        true,
	"/favicon.ico":    true,
	"/assignments.js": true,
	"/reconciler.js":  true,
}

func (m *metricsMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		ctx, stats := database.WithRequestStats(r.Context())
		next.ServeHTTP(rw, r.WithContext(ctx))

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

		pathAttrs := metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("path", pattern),
		)

		m.requestsTotal.Add(r.Context(), 1, attrs)
		m.requestDuration.Record(r.Context(), float64(time.Since(start).Milliseconds()), pathAttrs)
		m.queriesPerRequest.Record(r.Context(), stats.Queries(), pathAttrs)
		m.dbTimePerRequest.Record(r.Context(), float64(stats.DBDuration().Milliseconds()), pathAttrs)
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
