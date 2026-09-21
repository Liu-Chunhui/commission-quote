package app

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// LogRequests adds request context and logs the HTTP lifecycle.
func LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		requestID := rand.Text()
		r = r.WithContext(context.WithValue(r.Context(), middleware.RequestIDKey, requestID))
		logger := slog.Default().With("service", "quotevendor", "request_id", requestID)
		response := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		logger.Info("Request started", "operation", "http_request", "method", r.Method, "path", r.URL.Path)

		next.ServeHTTP(response, r)

		logger.Info("Request completed", "operation", "http_request", "method", r.Method, "path", r.URL.Path,
			"status", response.Status(), "duration_ms", time.Since(start).Milliseconds())
	})
}
