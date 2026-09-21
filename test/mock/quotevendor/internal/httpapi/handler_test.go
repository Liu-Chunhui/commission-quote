package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"quotevendor/internal/app"
	"quotevendor/internal/config"
)

const _validBody = `{"loanAmount":10000,"loanTermInMonths":36,"riskBand":"medium"}`
const _testKey = "test-only-placeholder"

type quoteResult struct {
	QuoteID         string  `json:"quoteId"`
	CommissionRate  float64 `json:"commissionRate"`
	TotalCommission float64 `json:"totalCommission"`
}

func TestAuthentication(t *testing.T) {
	handler := app.NewRouter(config.Config{APIKey: _testKey, FailureMode: "loanAmount"})
	request(t, handler, _validBody, "replay", _testKey, 200, "")
	for _, key := range []string{"", "wrong-placeholder"} {
		request(t, handler, "not json", "", key, 401, "UNAUTHORIZED")
		request(t, handler, _validBody, "replay", key, 401, "UNAUTHORIZED")
		request(t, handler, `{"loanAmount":100429,"loanTermInMonths":36,"riskBand":"medium"}`, "trigger", key, 401, "UNAUTHORIZED")
	}
	// Failed authentication must not reserve a key.
	request(t, handler, _validBody, "trigger", _testKey, 200, "")
}

func TestCalculation(t *testing.T) {
	handler := app.NewRouter(config.Config{APIKey: _testKey, FailureMode: "random", FailureRate: 0})
	for _, tc := range []struct {
		amount, term int
		risk         string
		rate, total  float64
	}{
		{10000, 36, "low", 0.01, 100},
		{10000, 36, "medium", 0.02, 200},
		{10000, 36, "high", 0.03, 300},
		{750000, 360, "medium", 0.02, 15000},
		{4001, 36, "low", 0.01, 40.01},
		{4000, 12, "low", 0.01, 40},
		{10000000, 360, "high", 0.03, 300000},
	} {
		body := fmt.Sprintf(`{"loanAmount":%d,"loanTermInMonths":%d,"riskBand":%q}`, tc.amount, tc.term, tc.risk)
		response := request(t, handler, body, bodyKey(tc.amount, tc.term, tc.risk), _testKey, 200, "")
		quote := decodeQuote(t, response)
		if quote.QuoteID == "" || quote.CommissionRate != tc.rate || quote.TotalCommission != tc.total {
			t.Fatalf("quote = %+v, want rate %v total %v", quote, tc.rate, tc.total)
		}
	}
}

func TestConcurrentDuplicates(t *testing.T) {
	var wg sync.WaitGroup
	handler := app.NewRouter(config.Config{APIKey: _testKey, FailureMode: "loanAmount"})
	responses := make([]*httptest.ResponseRecorder, 32)
	for i := range responses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/quotes", strings.NewReader(_validBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("api-key", _testKey)
			req.Header.Set("idempotency-key", "concurrent")
			responses[i] = httptest.NewRecorder()
			handler.ServeHTTP(responses[i], req)
		}()
	}
	wg.Wait()
	for _, response := range responses {
		if response.Code != 200 {
			t.Fatalf("concurrent status = %d, want 200", response.Code)
		}
	}
	first := decodeQuote(t, responses[0])
	for _, response := range responses[1:] {
		if got := decodeQuote(t, response); got != first {
			t.Fatalf("concurrent quote = %+v, want %+v", got, first)
		}
	}
}

func TestFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name           string
		config         config.Config
		amount, status int
		code           string
	}{
		{"bad request", config.Config{FailureMode: "loanAmount"}, 100400, 400, "INVALID_REQUEST"},
		{"rate limited", config.Config{FailureMode: "loanAmount"}, 100429, 429, "TOO_MANY_REQUESTS"},
		{"unlisted 430", config.Config{FailureMode: "loanAmount"}, 100430, 200, ""},
		{"unlisted 500", config.Config{FailureMode: "loanAmount"}, 100500, 200, ""},
		{"random zero ignores 400", config.Config{FailureMode: "random", FailureRate: 0}, 100400, 200, ""},
		{"random zero ignores 429", config.Config{FailureMode: "random", FailureRate: 0}, 100429, 200, ""},
		{"random one", config.Config{FailureMode: "random", FailureRate: 1}, 10000, 503, "VENDOR_UNAVAILABLE"},
		{"random one ignores 400", config.Config{FailureMode: "random", FailureRate: 1}, 100400, 503, "VENDOR_UNAVAILABLE"},
		{"random one ignores 429", config.Config{FailureMode: "random", FailureRate: 1}, 100429, 503, "VENDOR_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.APIKey = _testKey
			handler := app.NewRouter(tc.config)
			body := fmt.Sprintf(`{"loanAmount":%d,"loanTermInMonths":36,"riskBand":"medium"}`, tc.amount)
			request(t, handler, body, "failure", _testKey, tc.status, tc.code)
			request(t, handler, body, "failure", _testKey, tc.status, tc.code)
			if tc.status != 200 {
				request(t, handler, body, "", _testKey, 400, "INVALID_REQUEST")
				request(t, handler, strings.Replace(body, `:36`, `:12`, 1), "failure", _testKey, 409, "IDEMPOTENCY_CONFLICT")
			}
			request(t, handler, `{}`, "invalid", _testKey, 400, "INVALID_REQUEST")
		})
	}
}

func TestHealth(t *testing.T) {
	handler := app.NewRouter(config.Config{APIKey: _testKey, FailureMode: "random", FailureRate: 1})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != 200 || response.Body.String() != "ok" || response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("health: status %d, headers %v, body %q", response.Code, response.Header(), response.Body.String())
	}
}

func TestIdempotency(t *testing.T) {
	handler := app.NewRouter(config.Config{APIKey: _testKey, FailureMode: "loanAmount"})
	first := decodeQuote(t, request(t, handler, _validBody, "same", _testKey, 200, ""))
	for _, body := range []string{_validBody, `{"riskBand":"medium","extra":true,"loanTermInMonths":36,"loanAmount":10000}`} {
		if got := decodeQuote(t, request(t, handler, body, "same", _testKey, 200, "")); got != first {
			t.Fatalf("replay = %+v, want %+v", got, first)
		}
	}
	for _, body := range []string{
		strings.Replace(_validBody, "10000", "100429", 1),
		strings.Replace(_validBody, ":36", ":12", 1),
		strings.Replace(_validBody, "medium", "low", 1),
	} {
		request(t, handler, body, "same", _testKey, 409, "IDEMPOTENCY_CONFLICT")
	}
	request(t, handler, `{}`, "same", _testKey, 400, "INVALID_REQUEST")
	for _, key := range []string{"different", "Same"} {
		if got := decodeQuote(t, request(t, handler, _validBody, key, _testKey, 200, "")); got.QuoteID == first.QuoteID {
			t.Fatal("different key reused quote")
		}
	}
	restarted := app.NewRouter(config.Config{APIKey: _testKey, FailureMode: "loanAmount"})
	if got := decodeQuote(t, request(t, restarted, _validBody, "same", _testKey, 200, "")); got.QuoteID == first.QuoteID {
		t.Fatal("fresh service reused quote")
	}
}

func TestIdempotencyHeaders(t *testing.T) {
	handler := app.NewRouter(config.Config{APIKey: _testKey, FailureMode: "loanAmount"})
	for _, key := range []string{"", "with space", "\t", "\n", "\x1f", "\x7f", "é", strings.Repeat("a", 256)} {
		request(t, handler, _validBody, key, _testKey, 400, "INVALID_REQUEST")
	}
	for _, key := range []string{"!", "~", strings.Repeat("a", 255)} {
		request(t, handler, _validBody, key, _testKey, 200, "")
	}
}

func TestRequestLogs(t *testing.T) {
	var output bytes.Buffer
	var records []map[string]any
	var record map[string]any
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	handler := app.NewRouter(config.Config{APIKey: _testKey, FailureMode: "loanAmount"})
	request(t, handler, _validBody, "log-success", _testKey, 200, "")
	request(t, handler, strings.Replace(_validBody, "10000", "100429", 1), "log-failure", _testKey, 429, "TOO_MANY_REQUESTS")
	if strings.Contains(output.String(), _testKey) || strings.Contains(output.String(), "loanAmount") {
		t.Fatal("logs contain credentials or request body")
	}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		record = nil
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}

		if record["service"] != "quotevendor" || record["request_id"] == "" || record["request_id"] == nil || record["operation"] == nil {
			t.Fatalf("missing context: %v", record)
		}
		records = append(records, record)
	}

	if len(records) != 6 {
		t.Fatalf("got %d log records, want 6", len(records))
	}
	for _, offset := range []int{0, 3} {
		if records[offset]["msg"] != "Request started" || records[offset+2]["msg"] != "Request completed" {
			t.Fatal("incorrect lifecycle order")
		}
		for _, record := range records[offset : offset+3] {
			if record["request_id"] != records[offset]["request_id"] {
				t.Fatal("request ID lost")
			}
		}

		if records[offset+2]["duration_ms"] == nil {
			t.Fatal("missing duration")
		}
	}

	if records[0]["request_id"] == records[3]["request_id"] || records[2]["status"] != float64(200) || records[5]["status"] != float64(429) || records[4]["level"] != "ERROR" || records[4]["error_code"] != "TOO_MANY_REQUESTS" {
		t.Fatalf("unexpected request logs: %v", records)
	}
}

func TestValidation(t *testing.T) {
	var fields map[string]json.RawMessage
	handler := app.NewRouter(config.Config{APIKey: _testKey, FailureMode: "random", FailureRate: 0})
	bodies := []string{"", "{", "null", "[]", "true", "42", `"text"`, `{}`, _validBody + ` {}`, _validBody + ` trailing`}
	for _, field := range []string{"loanAmount", "loanTermInMonths", "riskBand"} {
		fields = nil
		if err := json.Unmarshal([]byte(_validBody), &fields); err != nil {
			t.Fatal(err)
		}
		delete(fields, field)
		missing, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, string(missing))
		for _, value := range []string{"null", "true", "[]", "{}"} {
			fields[field] = json.RawMessage(value)
			body, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			bodies = append(bodies, string(body))
		}
	}
	for _, amount := range []string{"3999", "10000001", "4000.5", `"10000"`, "1e100"} {
		bodies = append(bodies, strings.Replace(_validBody, "10000", amount, 1))
	}
	for _, term := range []string{"11", "361", "12.5", `"36"`} {
		bodies = append(bodies, strings.Replace(_validBody, ":36", ":"+term, 1))
	}
	for _, risk := range []string{`""`, `"LOW"`, `" medium"`, `"other"`, `1`} {
		bodies = append(bodies, strings.Replace(_validBody, `"medium"`, risk, 1))
	}
	bodies = append(bodies, strings.Replace(_validBody, "loanAmount", "LoanAmount", 1))
	for i, body := range bodies {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			key := fmt.Sprintf("validation-%d", i)
			request(t, handler, body, key, _testKey, 400, "INVALID_REQUEST")
			request(t, handler, _validBody, key, _testKey, 200, "")
		})
	}
}

func bodyKey(amount, term int, risk string) string {
	return fmt.Sprintf("%d|%d|%s", amount, term, risk)
}

func decodeQuote(t *testing.T, response *httptest.ResponseRecorder) quoteResult {
	var quote quoteResult
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), &quote); err != nil {
		t.Fatal(err)
	}
	return quote
}

func request(t *testing.T, handler http.Handler, body, key, apiKey string, status int, code string) *httptest.ResponseRecorder {
	var payload struct {
		Error struct{ Code, Message string }
	}
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/quotes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)
	req.Header.Set("idempotency-key", key)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}

	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}

	if code != "" {
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		if payload.Error.Code != code || payload.Error.Message == "" {
			t.Fatalf("error = %+v, want %s", payload, code)
		}
	}
	return response
}
