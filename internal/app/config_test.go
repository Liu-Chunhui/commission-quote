package app

import (
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
		{"valid", `{"port":8080}`, 8080},
		{"minimum", `{"port":1}`, 1},
		{"maximum", `{"port":65535}`, 65535},
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

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected a missing configuration file to be rejected")
	}
}
