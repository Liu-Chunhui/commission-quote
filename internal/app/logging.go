package app

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := strconv.FormatUint(middleware.NextRequestID(), 10)
		ctx := context.WithValue(r.Context(), middleware.RequestIDKey, requestID)
		response := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		logger := slog.With(
			"service", "app",
			"request_id", requestID,
			"operation", "http_request",
			"method", r.Method,
			"path", r.URL.Path,
		)
		logger.InfoContext(ctx, "Request started")
		next.ServeHTTP(response, r.WithContext(ctx))
		logger.InfoContext(ctx, "Request completed",
			"status", response.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}
