package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func requestLogger(ctx context.Context) *slog.Logger {
	return slog.Default().With("service", "quotevendor", "request_id", middleware.GetReqID(ctx))
}

func validKey(key string) bool {
	if len(key) < 1 || len(key) > 255 {
		return false
	}

	for i := range len(key) {
		if key[i] < '!' || key[i] > '~' {
			return false
		}
	}
	return true
}

func writeError(w http.ResponseWriter, r *http.Request, operation string, err *responseError) {
	requestLogger(r.Context()).Error(err.Message, "operation", operation, "error_code", err.Code, "status", err.status)
	writeJSON(w, r, err.status, struct {
		Error *responseError `json:"error"`
	}{Error: err})
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		requestLogger(r.Context()).Error("Unable to write JSON response", "operation", "write_response")
	}
}
