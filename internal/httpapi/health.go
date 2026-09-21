package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte("ok")); err != nil {
		slog.ErrorContext(r.Context(), "Health response could not be written",
			"service", "app",
			"request_id", middleware.GetReqID(r.Context()),
			"operation", "health",
			"cause", "response write failed",
		)
	}
}
