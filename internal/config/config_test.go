package config

import (
	"fmt"
	"os"
	"path/filepath"
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
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
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
			body := fmt.Sprintf(`{"port":8080,"dependencies":{"commissionquote":{"baseUrl":%s}}}`, tc.value)
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
