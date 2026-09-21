package app

import (
	"github.com/go-chi/chi/v5"

	"commissionquote/internal/httpapi"
)

func NewRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Use(logRequest)
	router.Get("/health", httpapi.Health)
	return router
}
