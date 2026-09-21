package app

import (
	"net/http"

	"commissionquote/mock/internal/config"
	"commissionquote/mock/internal/httpapi"

	"github.com/go-chi/chi/v5"
)

// NewRouter registers the service endpoints and request middleware.
func NewRouter(cfg config.Config) http.Handler {
	handler := httpapi.NewHandler(cfg)
	router := chi.NewRouter()
	router.Use(LogRequests)
	router.Get("/health", httpapi.Health)
	router.Post("/quotes", handler.Quote)
	return router
}
