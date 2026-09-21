package httpapi_test

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

	"commissionquote/internal/app"
	"commissionquote/internal/integration/commissionquote"
)

const _validQuoteBody = `{"loanAmount":10000,"loanTermInMonths":36,"riskBand":"medium"}`

func TestQuoteInvalidRequest(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()
	router := app.NewRouter(commissionquote.NewQuoteClient(quoteHTTPClient(), server.URL, "test-only-key"))
	cases := []struct {
		name        string
		body        string
		key         string
		contentType string
	}{
		{"missing key", _validQuoteBody, "", "application/json"},
		{"long key", _validQuoteBody, strings.Repeat("a", 256), "application/json"},
		{"space in key", _validQuoteBody, "a b", "application/json"},
		{"control in key", _validQuoteBody, "a\tb", "application/json"},
		{"non ASCII key", _validQuoteBody, "clé", "application/json"},
		{"DEL in key", _validQuoteBody, "a\x7f", "application/json"},
		{"missing content type", _validQuoteBody, "key", ""},
		{"wrong content type", _validQuoteBody, "key", "text/plain"},
		{"invalid content type", _validQuoteBody, "key", "application/json;="},
		{"empty body", "", "key", "application/json"},
		{"malformed body", "{", "key", "application/json"},
		{"null body", "null", "key", "application/json"},
		{"array body", "[]", "key", "application/json"},
		{"string body", `"text"`, "key", "application/json"},
		{"number body", "10", "key", "application/json"},
		{"trailing object", _validQuoteBody + " {}", "key", "application/json"},
		{"trailing null", _validQuoteBody + " null", "key", "application/json"},
		{"trailing garbage", _validQuoteBody + " x", "key", "application/json"},
	}
	for _, field := range []struct {
		name   string
		values []string
	}{
		{"loanAmount", []string{"", "null", "true", "[]", "{}", `"10000"`, "3999", "10000001", "4000.5", "-1", "1e100"}},
		{"loanTermInMonths", []string{"", "null", "true", "[]", "{}", `"36"`, "11", "361", "12.5", "-1"}},
		{"riskBand", []string{"", "null", "true", "[]", "{}", "1", `""`, `"Medium"`, `"medium "`, `"unknown"`}},
	} {
		for _, value := range field.values {
			var fields map[string]json.RawMessage

			if err := json.Unmarshal([]byte(_validQuoteBody), &fields); err != nil {
				t.Fatal(err)
			}

			if value == "" {
				delete(fields, field.name)
			} else {
				fields[field.name] = json.RawMessage(value)
			}
			body, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			cases = append(cases, struct {
				name, body, key, contentType string
			}{field.name + "/" + value, string(body), "key", "application/json"})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := quoteRequest(tc.body, tc.key)
			request.Header.Set("Content-Type", tc.contentType)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assertQuoteError(t, response, http.StatusBadRequest, "INVALID_REQUEST")
		})
	}

	if calls.Load() != 0 {
		t.Fatalf("invalid requests made %d downstream calls", calls.Load())
	}
}

func TestQuoteLogs(t *testing.T) {
	var logs bytes.Buffer

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	ids := make(map[string]bool)
	for _, tc := range []struct {
		name   string
		status int
		body   string
		key    string
		want   int
		levels []string
	}{
		{"success", 200, `{"quoteId":"opaque","commissionRate":0.02,"totalCommission":200}`, "key", 200, []string{"INFO", "INFO", "INFO", "INFO"}},
		{"dependency failure", 503, `{"error":{"code":"VENDOR_UNAVAILABLE","message":"private-detail"}}`, "key", 500, []string{"INFO", "INFO", "ERROR", "INFO", "INFO"}},
		{"invalid request", 200, "", "", 400, []string{"INFO", "ERROR", "INFO"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requestID string

			logs.Reset()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			router := app.NewRouter(commissionquote.NewQuoteClient(quoteHTTPClient(), server.URL, "test-only-key"))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, quoteRequest(_validQuoteBody, tc.key))
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}

			lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
			if len(lines) != len(tc.levels) {
				t.Fatalf("got %d logs, want %d", len(lines), len(tc.levels))
			}

			for i, line := range lines {
				var record map[string]any

				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal(err)
				}

				if i == 0 {
					requestID, _ = record["request_id"].(string)
					if requestID == "" || ids[requestID] {
						t.Fatal("each attempt must have a unique request ID")
					}
					ids[requestID] = true
				}

				if record["request_id"] != requestID || record["service"] != "app" || record["level"] != tc.levels[i] || record["operation"] == "" {
					t.Fatal("incorrect log context or sequence")
				}

				if record["level"] == "ERROR" && (record["cause"] == "" || record["error_code"] == "") {
					t.Fatal("error must include cause and code")
				}

				if i == len(lines)-1 {
					duration, ok := record["duration_ms"].(float64)
					if !ok || duration < 0 || record["status"] != float64(tc.want) {
						t.Fatal("completion must include final HTTP status and timing")
					}
				}
			}

			for _, private := range []string{"test-only-key", "private-detail", server.URL} {
				if strings.Contains(logs.String()+response.Body.String(), private) {
					t.Fatal("private downstream details exposed")
				}
			}
			t.Log(logs.String())
		})
	}
}

func TestQuoteSuccess(t *testing.T) {
	for _, tc := range []struct {
		amount int
		term   int
		risk   string
		key    string
	}{
		{4000, 12, "low", "!"},
		{10000000, 360, "high", strings.Repeat("~", 255)},
		{10000, 36, "medium", "32fafb2a-70b4-5114-9f2d-52be7286eaf5"},
		{4001, 36, "low", "opaque-key"},
		{100400, 36, "medium", "trigger-one"},
		{100429, 36, "medium", "trigger-two"},
	} {
		t.Run(fmt.Sprint(tc.amount), func(t *testing.T) {
			var calls atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var input map[string]any

				calls.Add(1)
				if r.Method != "POST" || r.URL.Path != "/quotes" || r.Header.Get("api-key") != "test-only-key" || r.Header.Get("idempotency-key") != tc.key || r.Header.Get("Content-Type") != "application/json" {
					t.Error("incorrect outgoing request")
				}

				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
				}

				if len(input) != 3 || input["loanAmount"] != float64(tc.amount) || input["loanTermInMonths"] != float64(tc.term) || input["riskBand"] != tc.risk {
					t.Error("validated fields must be forwarded unchanged; extras ignored")
				}
				fmt.Fprint(w, `{"quoteId":"opaque-result","commissionRate":0.01,"totalCommission":40.01}`)
			}))
			defer server.Close()
			router := app.NewRouter(commissionquote.NewQuoteClient(quoteHTTPClient(), server.URL, "test-only-key"))
			for range 2 {
				body := fmt.Sprintf(`{"loanAmount":%d,"loanTermInMonths":%d,"riskBand":%q,"ignored":true}`, tc.amount, tc.term, tc.risk)
				request := quoteRequest(body, tc.key)
				request.Header.Set("Content-Type", "application/json; charset=utf-8")
				request.Header.Set("api-key", "untrusted-browser-key")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if response.Code != 200 || response.Header().Get("Content-Type") != "application/json" || strings.TrimSpace(response.Body.String()) != `{"quoteId":"opaque-result","commissionRate":0.01,"totalCommission":40.01}` {
					t.Fatalf("quote was not returned unchanged: %d %s", response.Code, response.Body.String())
				}
			}

			if calls.Load() != 2 {
				t.Fatal("each attempt must make one downstream call, without local caching")
			}
		})
	}
}

func TestQuoteTimeoutAndCancellation(t *testing.T) {
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
			httpClient := quoteHTTPClient()
			httpClient.Timeout = 100 * time.Millisecond
			router := app.NewRouter(commissionquote.NewQuoteClient(httpClient, server.URL, "test-only-key"))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if mode == "cancellation" {
				httpClient.Timeout = 3 * time.Second
				go func() {
					<-started
					cancel()
				}()
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, quoteRequest(_validQuoteBody, "key").WithContext(ctx))
			assertQuoteError(t, response, 500, "INTERNAL_ERROR")
			select {
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("downstream request did not receive cancellation")
			}
		})
	}
}

func TestQuoteUpstreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"bad request", 400, `{"error":{"code":"INVALID_REQUEST","message":"private-detail"}}`},
		{"unauthorized", 401, `{"error":{"code":"UNAUTHORIZED","message":"private-detail"}}`},
		{"conflict", 409, `{"error":{"code":"IDEMPOTENCY_CONFLICT","message":"private-detail"}}`},
		{"rate limit", 429, `{"error":{"code":"TOO_MANY_REQUESTS","message":"private-detail"}}`},
		{"internal error", 500, `{"error":{"code":"INTERNAL_ERROR","message":"private-detail"}}`},
		{"unavailable", 503, `{"error":{"code":"VENDOR_UNAVAILABLE","message":"private-detail"}}`},
		{"unknown status", 418, `private-detail`},
		{"unknown code", 500, `{"error":{"code":"UNKNOWN","message":"private-detail"}}`},
		{"malformed error", 500, `{`},
		{"malformed quote", 200, `{`},
		{"missing quote fields", 200, `{}`},
		{"invalid quote fields", 200, `{"quoteId":1,"commissionRate":0.02,"totalCommission":200}`},
		{"invalid commission", 200, `{"quoteId":"opaque","commissionRate":0.02,"totalCommission":1.234}`},
		{"truncated body", 200, `{"quoteId":`},
		{"redirect", 302, ""},
		{"connection failure", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.name == "truncated body" {
					w.Header().Set("Content-Length", "1000")
				}

				if tc.name == "redirect" {
					w.Header().Set("Location", "/redirected")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			if tc.status == 0 {
				server.Close()
			}
			router := app.NewRouter(commissionquote.NewQuoteClient(quoteHTTPClient(), server.URL, "test-only-key"))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, quoteRequest(_validQuoteBody, "key"))
			assertQuoteError(t, response, 500, "INTERNAL_ERROR")
			if tc.status != 0 && calls.Load() != 1 {
				t.Fatal("downstream errors must not trigger redirects or retries")
			}
		})
	}
}

func TestQuoteWriteFailure(t *testing.T) {
	var logs bytes.Buffer

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"quoteId":"opaque","commissionRate":0.02,"totalCommission":200}`)
	}))
	defer server.Close()
	router := app.NewRouter(commissionquote.NewQuoteClient(quoteHTTPClient(), server.URL, "test-only-key"))
	router.ServeHTTP(failedResponseWriter{httptest.NewRecorder()}, quoteRequest(_validQuoteBody, "key"))
	if strings.Count(logs.String(), `"level":"ERROR"`) != 1 || !strings.Contains(logs.String(), `"cause":"response write failed"`) {
		t.Fatal("response write failure must be logged once with a safe cause")
	}
}

func assertQuoteError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	t.Helper()
	if response.Code != status || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status = %d, want %d; content type = %s", response.Code, status, response.Header().Get("Content-Type"))
	}

	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.Error.Code != code || body.Error.Message == "" {
		t.Fatalf("unexpected error response: %s", response.Body.String())
	}

	if status == 500 && strings.TrimSpace(response.Body.String()) != `{"error":{"code":"INTERNAL_ERROR","message":"Unable to generate a quote. Please try again later."}}` {
		t.Fatal("all downstream failures must return the exact generic payload")
	}
}

func quoteHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func quoteRequest(body, key string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/quotes", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("idempotency-key", key)
	return request
}
