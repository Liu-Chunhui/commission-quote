package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"io"
	mathrand "math/rand/v2"
	"mime"
	"net/http"
)

type quoteRequest struct {
	LoanAmount       int
	LoanTermInMonths int
	RiskBand         string
}

type quoteResponse struct {
	QuoteID         string  `json:"quoteId"`
	CommissionRate  float64 `json:"commissionRate"`
	TotalCommission float64 `json:"totalCommission"`
}

func (s *Handler) Quote(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("api-key")), []byte(s.config.APIKey)) != 1 {
		writeError(w, r, "authenticate", &responseError{http.StatusUnauthorized, "UNAUTHORIZED", "A valid API key is required."})
		return
	}

	key := r.Header.Get("idempotency-key")
	if !validKey(key) {
		writeError(w, r, "validate_request", &responseError{http.StatusBadRequest, "INVALID_REQUEST", "idempotency-key must contain 1 to 255 printable ASCII characters without spaces."})
		return
	}

	input, message := s.readQuoteRequest(r)
	if message != "" {
		writeError(w, r, "validate_request", &responseError{http.StatusBadRequest, "INVALID_REQUEST", message})
		return
	}

	quote, err := s.generate(r.Context(), input, key)
	if err != nil {
		operation := "simulate_failure"
		if err.status == http.StatusConflict {
			operation = "check_idempotency"
		}
		writeError(w, r, operation, err)
		return
	}

	writeJSON(w, r, http.StatusOK, quote)
}

func (s *Handler) generate(ctx context.Context, input quoteRequest, key string) (quoteResponse, *responseError) {
	// ponytail: one lock serializes in-memory calculations; shard by key if throughput matters.
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, exists := s.quotes[key]
	if exists && stored.input != input {
		return quoteResponse{}, &responseError{http.StatusConflict, "IDEMPOTENCY_CONFLICT", "This request key was already used with different loan details. Submit a new quote."}
	}

	if stored.quote.QuoteID != "" {
		requestLogger(ctx).Info("Stored quote replayed", "operation", "replay_quote")
		return stored.quote, nil
	}

	s.quotes[key] = storedQuote{input: input}
	if s.config.FailureMode == "loanAmount" {
		switch input.LoanAmount {
		case 100400:
			return quoteResponse{}, &responseError{http.StatusBadRequest, "INVALID_REQUEST", "Simulated vendor bad request."}
		case 100429:
			return quoteResponse{}, &responseError{http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "Too many quote requests. Please try again later."}
		}
	} else if mathrand.Float64() < s.config.FailureRate {
		return quoteResponse{}, &responseError{http.StatusServiceUnavailable, "VENDOR_UNAVAILABLE", "The quote provider is temporarily unavailable."}
	}

	ratePercent := 1
	switch input.RiskBand {
	case "medium":
		ratePercent = 2
	case "high":
		ratePercent = 3
	}
	commissionCents := input.LoanAmount * ratePercent
	quote := quoteResponse{
		QuoteID:         rand.Text(),
		CommissionRate:  float64(ratePercent) / 100,
		TotalCommission: float64(commissionCents) / 100,
	}
	s.quotes[key] = storedQuote{input: input, quote: quote}
	requestLogger(ctx).Info("Quote generated", "operation", "generate_quote")
	return quote, nil
}

func (s *Handler) readQuoteRequest(r *http.Request) (quoteRequest, string) {
	var fields map[string]json.RawMessage
	var input quoteRequest
	var trailing any

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return input, "Content-Type must be application/json."
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return input, "Request body must contain one JSON object."
	}

	if err := decoder.Decode(&trailing); err != io.EOF {
		return input, "Request body must contain one JSON object without trailing values."
	}

	if err := json.Unmarshal(fields["loanAmount"], &input.LoanAmount); err != nil || input.LoanAmount < 4000 || input.LoanAmount > 10000000 {
		return input, "loanAmount must be an integer between AUD 4000 and AUD 10000000."
	}

	if err := json.Unmarshal(fields["loanTermInMonths"], &input.LoanTermInMonths); err != nil || input.LoanTermInMonths < 12 || input.LoanTermInMonths > 360 {
		return input, "loanTermInMonths must be an integer between 12 and 360."
	}

	if err := json.Unmarshal(fields["riskBand"], &input.RiskBand); err != nil {
		return input, "riskBand must be low, medium, or high."
	}

	switch input.RiskBand {
	case "low", "medium", "high":
		return input, ""
	default:
		return input, "riskBand must be low, medium, or high."
	}
}
