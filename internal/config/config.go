package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port         int `json:"port"`
	Dependencies struct {
		CommissionQuote struct {
			BaseURL string `json:"baseUrl"`
		} `json:"commissionquote"`
	} `json:"dependencies"`
}

func LoadConfig(path string) (Config, error) {
	var config Config

	logger := slog.With("service", "app", "operation", "load_config")
	data, err := os.ReadFile(path)
	if err != nil {
		err = errors.New("unable to read configuration file")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	if err := json.Unmarshal(data, &config); err != nil {
		err = errors.New("configuration must be valid JSON with correctly typed fields")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

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

	return config, nil
}
