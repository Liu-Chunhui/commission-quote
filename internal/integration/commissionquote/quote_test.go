package commissionquote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/shopspring/decimal"
)

func TestGenerateQuote(t *testing.T) {
	var calls atomic.Int32

	input := QuoteRequest{LoanAmount: decimal.NewFromInt(4001), LoanTermInMonths: 36, RiskBand: "low"}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var received QuoteRequest

		calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/quotes" {
			t.Error("outbound method or path does not match the contract")
		}

		if r.Header.Get("api-key") != "test-only-key" || r.Header.Get("idempotency-key") != "same-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("outbound headers do not match the contract")
		}

		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}

		if !received.LoanAmount.Equal(input.LoanAmount) || received.LoanTermInMonths != input.LoanTermInMonths || received.RiskBand != input.RiskBand {
			t.Errorf("request = %+v, want %+v", received, input)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"quoteId":"opaque-id","commissionRate":"0.01","totalCommission":"40.01"}`)
	}))
	defer server.Close()
	httpClient := server.Client()
	httpClient.Timeout = 3 * time.Second
	client := NewQuoteClient(httpClient, server.URL+"/", "test-only-key")
	for range 2 {
		quote, err := client.GenerateQuote(context.Background(), "same-key", input)
		if err != nil {
			t.Fatal(err)
		}

		if quote.QuoteID != "opaque-id" || !quote.CommissionRate.Equal(decimal.RequireFromString("0.01")) || !quote.TotalCommission.Equal(decimal.RequireFromString("40.01")) {
			t.Errorf("unexpected quote: %+v", quote)
		}
	}

	if calls.Load() != 2 {
		t.Errorf("outbound calls = %d, want 2 (one per attempt, no local cache)", calls.Load())
	}
}

func TestGenerateQuoteAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") == "test-only-key" {
			fmt.Fprint(w, `{"quoteId":"id","commissionRate":"0.02","totalCommission":"200"}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"code":"UNAUTHORIZED","message":"Test key rejected"}}`)
	}))
	defer server.Close()
	for _, key := range []string{"", "wrong-test-key"} {
		client := NewQuoteClient(newTestHTTPClient(), server.URL, key)
		if _, err := client.GenerateQuote(context.Background(), "test-key", QuoteRequest{decimal.NewFromInt(10000), 36, "medium"}); err == nil {
			t.Fatal("authentication rejection must propagate as a failure")
		}
	}
}

func TestGenerateQuoteDoesNotRetryConnectionFailure(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		if calls.Add(1) == 2 {
			connection, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			connection.Close()
			return
		}
		fmt.Fprint(w, `{"quoteId":"id","commissionRate":"0.02","totalCommission":"200"}`)
	}))
	defer server.Close()
	client := NewQuoteClient(newTestHTTPClient(), server.URL, "test-only-key")
	input := QuoteRequest{decimal.NewFromInt(10000), 36, "medium"}
	if _, err := client.GenerateQuote(context.Background(), "test-key", input); err != nil {
		t.Fatal(err)
	}

	if _, err := client.GenerateQuote(context.Background(), "test-key", input); err == nil {
		t.Fatal("connection failure must not be hidden by a transport retry")
	}

	if calls.Load() != 2 {
		t.Errorf("requests = %d, want 2", calls.Load())
	}
}

func TestGenerateQuoteFailures(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"bad request", 400, `{"error":{"code":"INVALID_REQUEST","message":"test detail"}}`},
		{"unauthorized", 401, `{"error":{"code":"UNAUTHORIZED","message":"test detail"}}`},
		{"conflict", 409, `{"error":{"code":"IDEMPOTENCY_CONFLICT","message":"test detail"}}`},
		{"rate limited", 429, `{"error":{"code":"TOO_MANY_REQUESTS","message":"test detail"}}`},
		{"internal", 500, `{"error":{"code":"INTERNAL_ERROR","message":"test detail"}}`},
		{"unavailable", 503, `{"error":{"code":"VENDOR_UNAVAILABLE","message":"test detail"}}`},
		{"unknown status", 502, `{"error":{"code":"INTERNAL_ERROR","message":"test detail"}}`},
		{"unknown code", 500, `{"error":{"code":"UNKNOWN","message":"test detail"}}`},
		{"mismatched code", 401, `{"error":{"code":"INVALID_REQUEST","message":"test detail"}}`},
		{"malformed error", 500, `<html>test detail</html>`},
		{"missing error message", 500, `{"error":{"code":"INTERNAL_ERROR"}}`},
		{"malformed success", 200, `{`},
		{"trailing success", 200, `{"quoteId":"id","commissionRate":"0.02","totalCommission":"200"} {}`},
		{"missing ID", 200, `{"commissionRate":"0.02","totalCommission":"200"}`},
		{"missing rate", 200, `{"quoteId":"id","totalCommission":"200"}`},
		{"wrong rate", 200, `{"quoteId":"id","commissionRate":"2","totalCommission":"200"}`},
		{"missing commission", 200, `{"quoteId":"id","commissionRate":"0.02"}`},
		{"null commission", 200, `{"quoteId":"id","commissionRate":"0.02","totalCommission":null}`},
		{"numeric commission", 200, `{"quoteId":"id","commissionRate":"0.02","totalCommission":200}`},
		{"fractional cents", 200, `{"quoteId":"id","commissionRate":"0.02","totalCommission":"200.001"}`},
		{"sub-float fractional cents", 200, `{"quoteId":"id","commissionRate":"0.02","totalCommission":"40.0100000000000000001"}`},
		{"numeric rate", 200, `{"quoteId":"id","commissionRate":0.02,"totalCommission":"200"}`},
		{"NaN commission", 200, `{"quoteId":"id","commissionRate":"0.02","totalCommission":"NaN"}`},
		{"infinite commission", 200, `{"quoteId":"id","commissionRate":"0.02","totalCommission":"Infinity"}`},
		{"exponent commission", 200, `{"quoteId":"id","commissionRate":"0.02","totalCommission":"2e2"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client := NewQuoteClient(newTestHTTPClient(), server.URL, "test-only-key")
			quote, err := client.GenerateQuote(context.Background(), "test-key", QuoteRequest{decimal.NewFromInt(10000), 36, "medium"})
			if err == nil || err.Error() != "unable to generate a quote" {
				t.Fatal("expected the generic quote failure")
			}

			if quote.QuoteID != "" || !quote.CommissionRate.IsZero() || !quote.TotalCommission.IsZero() || calls.Load() != 1 {
				t.Errorf("failed request must return no quote and make exactly one attempt; quote=%+v calls=%d", quote, calls.Load())
			}
		})
	}
}

func TestGenerateQuoteLogs(t *testing.T) {
	var logs bytes.Buffer

	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	cases := []struct {
		name   string
		status int
		body   string
		code   string
	}{
		{"success", 200, `{"quoteId":"opaque-id","commissionRate":"0.02","totalCommission":"200"}`, ""},
		{"failure", 401, `{"error":{"code":"UNAUTHORIZED","message":"test-private-detail"}}`, "UNAUTHORIZED"},
		{"unknown code", 500, `{"error":{"code":"test-private-detail","message":"test-private-detail"}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client := NewQuoteClient(newTestHTTPClient(), server.URL, "test-only-key")
			ctx := context.WithValue(context.Background(), middleware.RequestIDKey, "request-42")
			_, err := client.GenerateQuote(ctx, "test-key", QuoteRequest{decimal.NewFromInt(10000), 36, "medium"})
			if (err != nil) != (tc.status != 200) {
				t.Fatal("unexpected call outcome")
			}

			levels := []string{"INFO", "INFO"}
			if tc.status != 200 {
				levels = []string{"INFO", "ERROR", "INFO"}
			}

			lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
			if len(lines) != len(levels) {
				t.Fatalf("log records = %d, want %d", len(lines), len(levels))
			}

			for i, line := range lines {
				var record map[string]any

				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal(err)
				}

				if record["level"] != levels[i] || record["service"] != "app" || record["request_id"] != "request-42" || record["operation"] != "generate_quote" || record["method"] != "POST" || record["path"] != "/quotes" {
					t.Error("incorrect call log context")
				}

				if record["level"] == "ERROR" {
					cause, _ := record["cause"].(string)
					if cause == "" {
						t.Error("error must explain the safe failure cause")
					}

					if tc.code != "" && record["error_code"] != tc.code {
						t.Error("documented error code was not logged")
					}
				}

				if i == len(lines)-1 {
					duration, ok := record["duration_ms"].(float64)
					if !ok || duration < 0 || record["status"] != float64(tc.status) {
						t.Error("completion must include response status and duration")
					}
				}
			}

			if strings.Contains(logs.String(), "test-private-detail") || strings.Contains(logs.String(), "test-only-key") || strings.Contains(logs.String(), server.URL) {
				t.Fatal("logs must not contain credentials, raw upstream details, or URLs")
			}
			t.Log(logs.String())
		})
	}
}

func TestGenerateQuoteNetworkFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	for _, endpoint := range []string{server.URL, "://invalid"} {
		client := NewQuoteClient(newTestHTTPClient(), endpoint, "test-only-key")
		_, err := client.GenerateQuote(context.Background(), "test-key", QuoteRequest{decimal.NewFromInt(10000), 36, "medium"})
		if err == nil || err.Error() != "unable to generate a quote" {
			t.Fatal("network and request construction failures must return a safe error")
		}
	}
}

func TestGenerateQuotePassThrough(t *testing.T) {
	cases := []struct {
		input    QuoteRequest
		response string
		want     QuoteResponse
	}{
		{QuoteRequest{decimal.NewFromInt(750000), 360, "medium"}, `{"quoteId":"large","commissionRate":"0.02","totalCommission":"15000"}`, QuoteResponse{"large", decimal.RequireFromString("0.02"), decimal.RequireFromString("15000")}},
		{QuoteRequest{decimal.NewFromInt(4000), 12, "high"}, `{"quoteId":"high","commissionRate":"0.03","totalCommission":"120"}`, QuoteResponse{"high", decimal.RequireFromString("0.03"), decimal.RequireFromString("120")}},
		{QuoteRequest{decimal.NewFromInt(100400), 36, "medium"}, `{"quoteId":"trigger-one","commissionRate":"0.02","totalCommission":"123.45"}`, QuoteResponse{"trigger-one", decimal.RequireFromString("0.02"), decimal.RequireFromString("123.45")}},
		{QuoteRequest{decimal.NewFromInt(100429), 36, "medium"}, `{"quoteId":"trigger-two","commissionRate":"0.02","totalCommission":"0"}`, QuoteResponse{"trigger-two", decimal.RequireFromString("0.02"), decimal.RequireFromString("0")}},
	}
	for _, tc := range cases {
		t.Run(tc.want.QuoteID, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var received QuoteRequest

				if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
					t.Error(err)
				}

				if !received.LoanAmount.Equal(tc.input.LoanAmount) || received.LoanTermInMonths != tc.input.LoanTermInMonths || received.RiskBand != tc.input.RiskBand {
					t.Error("client must forward input unchanged, including mock trigger amounts")
				}
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			client := NewQuoteClient(newTestHTTPClient(), server.URL, "test-only-key")
			quote, err := client.GenerateQuote(context.Background(), "test-key", tc.input)
			if err != nil || quote.QuoteID != tc.want.QuoteID || !quote.CommissionRate.Equal(tc.want.CommissionRate) || !quote.TotalCommission.Equal(tc.want.TotalCommission) {
				t.Fatalf("response must be returned without recalculation: quote=%+v err=%v", quote, err)
			}
		})
	}
}

func TestGenerateQuoteReadFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		fmt.Fprint(w, `{"quoteId":"truncated`)
	}))
	defer server.Close()
	client := NewQuoteClient(newTestHTTPClient(), server.URL, "test-only-key")
	if _, err := client.GenerateQuote(context.Background(), "test-key", QuoteRequest{decimal.NewFromInt(10000), 36, "medium"}); err == nil {
		t.Fatal("truncated response must fail")
	}
}

func TestGenerateQuoteRedirect(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusTemporaryRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var redirectedCalls atomic.Int32

			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				redirectedCalls.Add(1)
			}))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL, status)
			}))
			defer server.Close()
			client := NewQuoteClient(newTestHTTPClient(), server.URL, "test-only-key")
			if _, err := client.GenerateQuote(context.Background(), "test-key", QuoteRequest{decimal.NewFromInt(10000), 36, "medium"}); err == nil {
				t.Fatal("unexpected redirect must fail")
			}

			if redirectedCalls.Load() != 0 {
				t.Fatal("client must not forward credentials to a redirect destination")
			}
		})
	}
}

func TestGenerateQuoteTimeoutAndCancellation(t *testing.T) {
	for _, mode := range []string{"timeout", "body timeout", "cancellation"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{})
			canceled := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				if mode == "body timeout" {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
				close(canceled)
			}))
			defer server.Close()
			httpClient := newTestHTTPClient()
			client := NewQuoteClient(httpClient, server.URL, "test-only-key")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode != "cancellation" {
				httpClient.Timeout = 100 * time.Millisecond
			} else {
				go func() {
					<-started
					cancel()
				}()
			}

			_, err := client.GenerateQuote(ctx, "test-key", QuoteRequest{decimal.NewFromInt(10000), 36, "medium"})
			if err == nil || err.Error() != "unable to generate a quote" {
				t.Fatal("timeout or cancellation must return a safe error")
			}

			select {
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("outbound request was not canceled")
			}
		})
	}
}

func newTestHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
