package app

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type failedResponseWriter struct {
	*httptest.ResponseRecorder
}

func TestHealth(t *testing.T) {
	t.Setenv("VENDOR_API_KEY", "")
	t.Setenv("VENDOR_BASE_URL", "http://127.0.0.1:1")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	NewRouter().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}

	if got := response.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}

	if got := response.Body.String(); got != "ok" {
		t.Errorf("body = %q, want ok", got)
	}
}

func TestRequestLogs(t *testing.T) {
	var logs bytes.Buffer

	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	router := NewRouter()
	requestIDs := make(map[string]bool)
	cases := []struct {
		name      string
		path      string
		failWrite bool
		status    int
		levels    []string
	}{
		{"success", "/health", false, 200, []string{"INFO", "INFO"}},
		{"write failure", "/health", true, 200, []string{"INFO", "ERROR", "INFO"}},
		{"not found", "/missing", false, 404, []string{"INFO", "INFO"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requestID string
			var writer http.ResponseWriter = httptest.NewRecorder()

			logs.Reset()
			if tc.failWrite {
				writer = failedResponseWriter{httptest.NewRecorder()}
			}

			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			request.Header.Set("X-Request-ID", "caller-reused-id")
			router.ServeHTTP(writer, request)
			lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
			if len(lines) != len(tc.levels) {
				t.Fatalf("log records = %d, want %d: %s", len(lines), len(tc.levels), logs.String())
			}

			for i, line := range lines {
				var record map[string]any

				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal(err)
				}

				if i == 0 {
					requestID, _ = record["request_id"].(string)
					if requestID == "" || requestID == "caller-reused-id" || requestIDs[requestID] {
						t.Fatalf("expected a fresh request ID, got %q", requestID)
					}
					requestIDs[requestID] = true
				}

				if record["request_id"] != requestID || record["service"] != "app" {
					t.Errorf("missing or inconsistent log context: %s", line)
				}

				if record["level"] == "INFO" && record["operation"] != "http_request" {
					t.Errorf("incorrect request operation: %s", line)
				}

				if record["level"] != tc.levels[i] {
					t.Errorf("level = %v, want %s", record["level"], tc.levels[i])
				}

				if record["level"] == "ERROR" && (record["operation"] != "health" || record["cause"] != "response write failed") {
					t.Errorf("missing safe failure explanation: %s", line)
				}

				if i == len(lines)-1 {
					duration, ok := record["duration_ms"].(float64)
					if !ok || duration < 0 || record["status"] != float64(tc.status) || record["method"] != "GET" || record["path"] != tc.path {
						t.Errorf("incorrect completion log: %s", line)
					}
				}
			}
			t.Log(logs.String())
		})
	}
}

func (w failedResponseWriter) Write(body []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return 0, io.ErrClosedPipe
}
