package app

import (
	"github.com/go-chi/chi/v5"

	"commissionquote/internal/httpapi"
	"commissionquote/internal/integration/commissionquote"
)

func NewRouter(quoteClient *commissionquote.QuoteClient) *chi.Mux {
	router := chi.NewRouter()
	router.Use(logRequest)
	router.Get("/health", httpapi.Health)
	router.Post("/api/quotes", httpapi.NewQuoteHandler(quoteClient).GenerateQuote)
	return router
}
