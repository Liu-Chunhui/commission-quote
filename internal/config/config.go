package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port         int    `json:"port"`
	APIKey       string `json:"-"`
	Dependencies struct {
		CommissionQuote struct {
			BaseURL string `json:"baseUrl"`
		} `json:"commissionquote"`
	} `json:"dependencies"`
}

func LoadConfig(path string) (Config, error) {
	var profile struct {
		Config
		APIKeyFile string `json:"apiKeyFile"`
	}

	logger := slog.With("service", "app", "operation", "load_config")
	data, err := os.ReadFile(path)
	if err != nil {
		err = errors.New("unable to read configuration file")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	if err := json.Unmarshal(data, &profile); err != nil {
		err = errors.New("configuration must be valid JSON with correctly typed fields")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	config := profile.Config
	if config.Port < 1 || config.Port > 65535 {
		err := errors.New("port must be between 1 and 65535")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	baseURL, err := url.Parse(config.Dependencies.CommissionQuote.BaseURL)
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Hostname() == "" {
		err := errors.New("dependencies.commissionquote.baseUrl must be an absolute HTTP(S) URL")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	if baseURL.User != nil || strings.ContainsAny(config.Dependencies.CommissionQuote.BaseURL, "?#") {
		err := errors.New("dependencies.commissionquote.baseUrl must not contain credentials, a query, or a fragment")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	if profile.APIKeyFile == "" {
		err := errors.New("apiKeyFile is required")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	keyPath := profile.APIKeyFile
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(filepath.Dir(path), keyPath)
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		err = errors.New("unable to read the API key file")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	config.APIKey = strings.TrimSpace(string(key))
	if config.APIKey == "" {
		err := errors.New("API key file must not be empty")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	return config, nil
}
