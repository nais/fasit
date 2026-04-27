package server

import (
	"net/http"
	"time"

	"github.com/nais/fasit/internal/ui/server/stats"
	"github.com/sirupsen/logrus"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stats.IncrementRequests()
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		logrus.WithFields(logrus.Fields{"method": r.Method, "path": r.URL.Path, "status": rw.status, "duration": time.Since(start)}).Info("http request")
	})
}
