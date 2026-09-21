package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func writeQuoteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeQuoteJSON(w, r, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func writeQuoteJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.ErrorContext(r.Context(), "Quote response could not be written",
			"service", "app", "request_id", middleware.GetReqID(r.Context()),
			"operation", "write_quote_response", "cause", "response write failed",
		)
	}
}
