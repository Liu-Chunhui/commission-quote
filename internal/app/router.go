package app

import "github.com/go-chi/chi/v5"

func NewRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Use(logRequest)
	router.Get("/health", health)
	return router
}
