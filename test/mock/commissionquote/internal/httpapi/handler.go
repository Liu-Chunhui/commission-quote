package httpapi

import (
	"sync"

	"commissionquote/mock/internal/config"
)

type storedQuote struct {
	input quoteRequest
	quote quoteResponse
}

type responseError struct {
	status  int
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Handler owns the state used by the quote endpoint.
type Handler struct {
	config config.Config
	mu     sync.Mutex
	quotes map[string]storedQuote
}

// NewHandler creates quote handlers using a validated startup config.
func NewHandler(cfg config.Config) *Handler {
	return &Handler{
		config: cfg,
		quotes: make(map[string]storedQuote),
	}
}
