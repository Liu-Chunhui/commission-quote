package commissionquote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/shopspring/decimal"
)

var errQuoteFailed = errors.New("unable to generate a quote")
var _commissionPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]{1,2})?$`)

type QuoteClient struct {
	httpClient *http.Client
	quoteURL   string
	apiKey     string
}

// NewQuoteClient requires a non-nil HTTP client with a timeout and redirects disabled.
func NewQuoteClient(httpClient *http.Client, baseURL, apiKey string) *QuoteClient {
	return &QuoteClient{
		httpClient: httpClient,
		quoteURL:   strings.TrimRight(baseURL, "/") + "/quotes",
		apiKey:     apiKey,
	}
}

// GenerateQuote expects input and an idempotency key validated by the HTTP handler.
func (c *QuoteClient) GenerateQuote(ctx context.Context, key string, input QuoteRequest) (QuoteResponse, error) {
	var status int
	var quote struct {
		QuoteID         string  `json:"quoteId"`
		CommissionRate  string  `json:"commissionRate"`
		TotalCommission *string `json:"totalCommission"`
	}

	started := time.Now().UTC()
	logger := slog.With(
		"service", "app",
		"request_id", middleware.GetReqID(ctx),
		"operation", "generate_quote",
		"method", http.MethodPost,
		"path", "/quotes",
	)
	logger.InfoContext(ctx, "Quote service call started")
	defer func() {
		logger.InfoContext(ctx, "Quote service call completed",
			"status", status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	}()

	// Validated decimals, integers, and strings always marshal successfully.
	body, _ := json.Marshal(input)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.quoteURL, bytes.NewReader(body))
	if err != nil {
		logger.ErrorContext(ctx, "Quote service call failed", "cause", "request could not be created")
		return QuoteResponse{}, errQuoteFailed
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("api-key", c.apiKey)
	request.Header.Set("idempotency-key", key)
	// Prevent transport retries of this POST, even with an idempotency key.
	request.GetBody = nil

	response, err := c.httpClient.Do(request)
	if err != nil {
		cause := "connection failed"
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			cause = "request timed out"
		case errors.Is(err, context.Canceled):
			cause = "request canceled"
		}
		logger.ErrorContext(ctx, "Quote service call failed", "cause", cause)
		return QuoteResponse{}, errQuoteFailed
	}
	defer response.Body.Close()
	status = response.StatusCode

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		cause := "response body could not be read"
		if errors.Is(err, context.DeadlineExceeded) {
			cause = "response body timed out"
		}
		logger.ErrorContext(ctx, "Quote service call failed", "cause", cause, "status", status)
		return QuoteResponse{}, errQuoteFailed
	}

	if status != http.StatusOK {
		code, cause := quoteServiceError(status, responseBody)
		errorLogger := logger.With("status", status)
		if code != "" {
			errorLogger = errorLogger.With("error_code", code)
		}
		errorLogger.ErrorContext(ctx, "Quote service call failed", "cause", cause)
		return QuoteResponse{}, errQuoteFailed
	}

	if err := json.Unmarshal(responseBody, &quote); err != nil {
		logger.ErrorContext(ctx, "Quote service call failed", "cause", "invalid quote JSON", "status", status)
		return QuoteResponse{}, errQuoteFailed
	}

	if quote.QuoteID == "" || (quote.CommissionRate != "0.01" && quote.CommissionRate != "0.02" && quote.CommissionRate != "0.03") || quote.TotalCommission == nil {
		logger.ErrorContext(ctx, "Quote service call failed", "cause", "missing or invalid quote fields", "status", status)
		return QuoteResponse{}, errQuoteFailed
	}

	if !_commissionPattern.MatchString(*quote.TotalCommission) {
		logger.ErrorContext(ctx, "Quote service call failed", "cause", "commission must be a decimal string with at most two decimal places", "status", status)
		return QuoteResponse{}, errQuoteFailed
	}

	return QuoteResponse{
		QuoteID:         quote.QuoteID,
		CommissionRate:  decimal.RequireFromString(quote.CommissionRate),
		TotalCommission: decimal.RequireFromString(*quote.TotalCommission),
	}, nil
}

func quoteServiceError(status int, body []byte) (string, string) {
	var code, cause string
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	switch status {
	case http.StatusBadRequest:
		code, cause = "INVALID_REQUEST", "quote request rejected"
	case http.StatusUnauthorized:
		code, cause = "UNAUTHORIZED", "quote authentication failed"
	case http.StatusConflict:
		code, cause = "IDEMPOTENCY_CONFLICT", "idempotency key conflicts with earlier input"
	case http.StatusTooManyRequests:
		code, cause = "TOO_MANY_REQUESTS", "quote requests rate limited"
	case http.StatusInternalServerError:
		code, cause = "INTERNAL_ERROR", "quote service internal failure"
	case http.StatusServiceUnavailable:
		code, cause = "VENDOR_UNAVAILABLE", "quote service unavailable"
	default:
		return "", "unexpected response status"
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "invalid error response"
	}

	if payload.Error.Code != code || payload.Error.Message == "" {
		return "", "unrecognized error response"
	}

	return code, cause
}
