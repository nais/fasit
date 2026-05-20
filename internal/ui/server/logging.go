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

	requestDuration, err := meter.Float64Histogram("http_request_duration_seconds",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &metricsMiddleware{
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
	}, nil
}

func (m *metricsMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

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
		m.requestDuration.Record(r.Context(), time.Since(start).Seconds(),
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
