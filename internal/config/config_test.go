package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	cases := []struct {
		name string
		body string
		port int
	}{
		{"valid", `{"port":8080,"dependencies":{"commissionquote":{"baseUrl":"http://localhost:8090"}}}`, 8080},
		{"minimum", `{"port":1,"dependencies":{"commissionquote":{"baseUrl":"http://localhost:8090"}}}`, 1},
		{"maximum", `{"port":65535,"dependencies":{"commissionquote":{"baseUrl":"http://localhost:8090"}}}`, 65535},
		{"missing port", `{}`, 0},
		{"null port", `{"port":null}`, 0},
		{"zero", `{"port":0}`, 0},
		{"negative", `{"port":-1}`, 0},
		{"too large", `{"port":65536}`, 0},
		{"fractional", `{"port":8080.5}`, 0},
		{"string", `{"port":"8080"}`, 0},
		{"malformed", `{"port":`, 0},
		{"trailing JSON", `{"port":8080} {}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(filepath.Join(filepath.Dir(path), "test-key"), []byte("test-only"), 0600); err != nil {
				t.Fatal(err)
			}

			body := strings.Replace(tc.body, "{", `{"apiKeyFile":"test-key",`, 1)
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}

			config, err := LoadConfig(path)
			if tc.port == 0 {
				if err == nil {
					t.Fatal("expected invalid configuration to be rejected")
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if config.Port != tc.port {
				t.Errorf("port = %d, want %d", config.Port, tc.port)
			}
		})
	}
}

func TestLoadConfigAPIKey(t *testing.T) {
	cases := []struct {
		name     string
		key      string
		absolute bool
		wantErr  string
	}{
		{name: "relative", key: "  test-only\n"},
		{name: "absolute", key: "test-only", absolute: true},
		{name: "missing setting", wantErr: "apiKeyFile is required"},
		{name: "missing file", wantErr: "unable to read the API key file"},
		{name: "empty file", wantErr: "API key file must not be empty"},
		{name: "whitespace", key: " \n\t", wantErr: "API key file must not be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer

			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })
			dir := t.TempDir()
			keyPath := filepath.Join(dir, "test-key")
			if tc.name != "missing file" {
				if err := os.WriteFile(keyPath, []byte(tc.key), 0600); err != nil {
					t.Fatal(err)
				}
			}

			setting := "test-key"
			if tc.absolute {
				setting = keyPath
			}

			if tc.name == "missing setting" {
				setting = ""
			}

			body := fmt.Sprintf(`{"port":8080,"apiKeyFile":%q,"apiKey":"ignored-inline-value","dependencies":{"commissionquote":{"baseUrl":"http://localhost:8090"}}}`, setting)
			path := filepath.Join(dir, "config.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}

			config, err := LoadConfig(path)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatal("expected safe configuration error")
				}

				if strings.Count(logs.String(), `"level":"ERROR"`) != 1 {
					t.Fatal("expected one error log")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}

				if config.APIKey != "test-only" {
					t.Fatal("expected trimmed API key from file")
				}
			}

			encoded, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}

			if strings.Contains(string(encoded)+logs.String(), "test-only") || strings.Contains(logs.String(), keyPath) {
				t.Fatal("API key or private path exposed")
			}
		})
	}
}

func TestLoadConfigCommissionQuoteURL(t *testing.T) {
	cases := []struct {
		name  string
		value string
		valid bool
	}{
		{"local", `"http://localhost:8090"`, true},
		{"https", `"https://example.com/vendor/"`, true},
		{"empty", `""`, false},
		{"null", `null`, false},
		{"number", `8090`, false},
		{"relative", `"localhost:8090"`, false},
		{"missing host", `"http://"`, false},
		{"wrong scheme", `"ftp://example.com"`, false},
		{"invalid escape", `"http://example.com/%zz"`, false},
		{"userinfo", `"http://test-user@example.com"`, false},
		{"query", `"http://example.com?mode=dev"`, false},
		{"empty query", `"http://example.com?"`, false},
		{"fragment", `"http://example.com#quotes"`, false},
		{"empty fragment", `"http://example.com#"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(filepath.Join(filepath.Dir(path), "test-key"), []byte("test-only"), 0600); err != nil {
				t.Fatal(err)
			}

			body := fmt.Sprintf(`{"port":8080,"apiKeyFile":"test-key","dependencies":{"commissionquote":{"baseUrl":%s}}}`, tc.value)
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}

			config, err := LoadConfig(path)
			if (err == nil) != tc.valid {
				t.Fatalf("valid = %v, expected %v", err == nil, tc.valid)
			}

			if tc.valid && fmt.Sprintf("%q", config.Dependencies.CommissionQuote.BaseURL) != tc.value {
				t.Error("base URL was not preserved")
			}
		})
	}

	path := filepath.Join(t.TempDir(), "missing-dependency.json")
	if err := os.WriteFile(path, []byte(`{"port":8080}`), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfig(path); err == nil {
		t.Fatal("missing commissionquote configuration must be rejected")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected a missing configuration file to be rejected")
	}
}
