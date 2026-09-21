package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Config contains settings validated by LoadConfig.
type Config struct {
	Port        int
	APIKey      string
	FailureMode string
	FailureRate float64
}

// LoadConfig reads the selected profile once. Amount mode ignores failureRate.
func LoadConfig(path string) (Config, error) {
	var profile struct {
		Port        int             `json:"port"`
		APIKeyFile  string          `json:"apiKeyFile"`
		FailureMode string          `json:"failureMode"`
		FailureRate json.RawMessage `json:"failureRate"`
	}
	var rate *float64

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, configError("Unable to read mock configuration.")
	}

	if err := json.Unmarshal(data, &profile); err != nil {
		return Config{}, configError("Mock configuration must be a valid JSON object.")
	}

	if profile.Port < 1 || profile.Port > 65535 {
		return Config{}, configError("port must be an integer from 1 through 65535.")
	}

	config := Config{Port: profile.Port, FailureMode: profile.FailureMode}
	switch profile.FailureMode {
	case "loanAmount":
		// This mode deliberately ignores failureRate.
	case "random":
		if err := json.Unmarshal(profile.FailureRate, &rate); err != nil || rate == nil || *rate < 0 || *rate > 1 {
			return Config{}, configError("Random mode requires a numeric failureRate from 0 through 1.")
		}
		config.FailureRate = *rate
	default:
		return Config{}, configError("failureMode must be random or loanAmount.")
	}

	if profile.APIKeyFile == "" {
		return Config{}, configError("apiKeyFile is required.")
	}

	keyPath := profile.APIKeyFile
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(filepath.Dir(path), keyPath)
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return Config{}, configError("Unable to read the API key file.")
	}

	config.APIKey = strings.TrimSpace(string(key))
	if config.APIKey == "" {
		return Config{}, configError("API key file must not be empty.")
	}

	return config, nil
}

func configError(message string) error {
	slog.Error(message, "service", "commission-quote", "operation", "load_config", "error_code", "INVALID_CONFIG")
	return errors.New(message)
}
