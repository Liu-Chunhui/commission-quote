package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"commissionquote/internal/integration/commissionquote"

	"github.com/go-chi/chi/v5/middleware"
)

type QuoteHandler struct {
	client *commissionquote.QuoteClient
}

func NewQuoteHandler(client *commissionquote.QuoteClient) *QuoteHandler {
	return &QuoteHandler{client: client}
}

func (h *QuoteHandler) GenerateQuote(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("idempotency-key")

	input, errMessage := h.validateQuoteRequest(r, key)
	if errMessage != "" {
		slog.ErrorContext(r.Context(), "Quote request rejected",
			"service", "app", "request_id", middleware.GetReqID(r.Context()),
			"operation", "validate_quote", "error_code", "INVALID_REQUEST", "cause", errMessage,
		)
		writeQuoteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", errMessage)
		return
	}

	quote, err := h.client.GenerateQuote(r.Context(), key, input)
	if err != nil {
		writeQuoteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to generate a quote. Please try again later.")
		return
	}

	writeQuoteJSON(w, r, http.StatusOK, quote)
}

func (h *QuoteHandler) validateQuoteRequest(r *http.Request, key string) (commissionquote.QuoteRequest, string) {
	var fields map[string]json.RawMessage
	var extra json.RawMessage
	var input commissionquote.QuoteRequest

	if len(key) < 1 || len(key) > 255 {
		return input, "idempotency-key must contain 1 to 255 printable ASCII characters without spaces."
	}

	for _, char := range key {
		if char < '!' || char > '~' {
			return input, "idempotency-key must contain 1 to 255 printable ASCII characters without spaces."
		}
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return input, "Content-Type must be application/json."
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return input, "Request body must contain one JSON object."
	}

	if err := decoder.Decode(&extra); err != io.EOF {
		return input, "Request body must contain one JSON object."
	}

	if err := json.Unmarshal(fields["loanAmount"], &input.LoanAmount); err != nil || input.LoanAmount < 4000 || input.LoanAmount > 10000000 {
		return input, "loanAmount must be an integer between AUD 4000 and AUD 10000000."
	}

	if err := json.Unmarshal(fields["loanTermInMonths"], &input.LoanTermInMonths); err != nil || input.LoanTermInMonths < 12 || input.LoanTermInMonths > 360 {
		return input, "loanTermInMonths must be an integer between 12 and 360."
	}

	if err := json.Unmarshal(fields["riskBand"], &input.RiskBand); err != nil || (input.RiskBand != "low" && input.RiskBand != "medium" && input.RiskBand != "high") {
		return input, "riskBand must be low, medium, or high."
	}

	return input, ""
}
