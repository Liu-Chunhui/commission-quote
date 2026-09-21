package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const _testKey = "test-only-placeholder"

func TestLoadConfig(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		wantError bool
	}{
		{"amount mode", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"loanAmount"}`, false},
		{"ignored null", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"loanAmount","failureRate":null}`, false},
		{"ignored string", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"loanAmount","failureRate":"ignored"}`, false},
		{"ignored range", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"loanAmount","failureRate":20}`, false},
		{"ignored object", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"loanAmount","failureRate":{}}`, false},
		{"zero", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"random","failureRate":0}`, false},
		{"one", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"random","failureRate":1}`, false},
		{"fraction", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"random","failureRate":0.1}`, false},
		{"missing rate", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"random"}`, true},
		{"null rate", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"random","failureRate":null}`, true},
		{"string rate", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"random","failureRate":"0"}`, true},
		{"negative rate", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"random","failureRate":-0.1}`, true},
		{"large rate", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"random","failureRate":1.1}`, true},
		{"missing mode", `{"port":8090,"apiKeyFile":"API_KEY"}`, true},
		{"unknown mode", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"other"}`, true},
		{"malformed", `{`, true},
		{"trailing value", `{"port":8090,"apiKeyFile":"API_KEY","failureMode":"loanAmount"} {}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "config.json")
			if err := os.WriteFile(filepath.Join(directory, "API_KEY"), []byte(_testKey+"\n"), 0600); err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadConfig(path)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error = %v", err, tc.wantError)
			}
		})
	}

	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("missing config accepted")
	}
}

func TestMountedAPIKey(t *testing.T) {
	for _, tc := range []struct {
		name       string
		keyFile    string
		contents   string
		createFile bool
		absolute   bool
		wantError  bool
	}{
		{name: "relative path", keyFile: "../data/API_KEY", contents: _testKey + "\n", createFile: true},
		{name: "absolute path", keyFile: "../data/API_KEY", contents: _testKey + "\r\n", createFile: true, absolute: true},
		{name: "missing setting", wantError: true},
		{name: "missing file", keyFile: "../data/API_KEY", wantError: true},
		{name: "empty file", keyFile: "../data/API_KEY", createFile: true, wantError: true},
		{name: "blank file", keyFile: "../data/API_KEY", contents: " \n", createFile: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			configDirectory := filepath.Join(directory, "config")
			dataDirectory := filepath.Join(directory, "data")
			if err := os.MkdirAll(configDirectory, 0700); err != nil {
				t.Fatal(err)
			}

			if err := os.MkdirAll(dataDirectory, 0700); err != nil {
				t.Fatal(err)
			}
			keyPath := filepath.Join(dataDirectory, "API_KEY")
			if tc.createFile {
				if err := os.WriteFile(keyPath, []byte(tc.contents), 0600); err != nil {
					t.Fatal(err)
				}
			}
			reference := tc.keyFile
			if tc.absolute {
				reference = keyPath
			}
			body := fmt.Sprintf(`{"port":8090,"failureMode":"loanAmount","apiKeyFile":%q}`, reference)
			configPath := filepath.Join(configDirectory, "dev.json")
			if err := os.WriteFile(configPath, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			config, err := LoadConfig(configPath)
			if (err != nil) != tc.wantError {
				t.Fatalf("load error presence = %v, want %v", err != nil, tc.wantError)
			}

			if tc.wantError {
				return
			}

			if config.APIKey != _testKey {
				t.Fatal("mounted API key was not loaded correctly")
			}
		})
	}
}

func TestPort(t *testing.T) {
	for _, tc := range []struct {
		name      string
		settings  string
		wantPort  int
		wantError bool
	}{
		{"configured settings", `"port":9012`, 9012, false},
		{"minimum port", `"port":1`, 1, false},
		{"maximum port", `"port":65535`, 65535, false},
		{"missing port", `"unused":true`, 0, true},
		{"zero port", `"port":0`, 0, true},
		{"negative port", `"port":-1`, 0, true},
		{"large port", `"port":65536`, 0, true},
		{"string port", `"port":"8090"`, 0, true},
		{"fractional port", `"port":8090.5`, 0, true},
		{"null port", `"port":null`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "config.json")
			if err := os.WriteFile(filepath.Join(directory, "API_KEY"), []byte(_testKey), 0600); err != nil {
				t.Fatal(err)
			}

			body := fmt.Sprintf(`{"apiKeyFile":"API_KEY","failureMode":"loanAmount",%s}`, tc.settings)
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}

			cfg, err := LoadConfig(path)
			if (err != nil) != tc.wantError {
				t.Fatalf("load error presence = %v, want %v", err != nil, tc.wantError)
			}

			if tc.wantError {
				return
			}

			if cfg.Port != tc.wantPort {
				t.Fatal("port was not loaded from the configuration file")
			}
		})
	}
}
