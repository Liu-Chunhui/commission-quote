package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
)

type Config struct {
	Port int `json:"port"`
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
		err = errors.New("configuration must be valid JSON with an integer port")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	if config.Port < 1 || config.Port > 65535 {
		err := errors.New("port must be between 1 and 65535")
		logger.Error("Configuration load failed", "cause", err.Error())
		return Config{}, err
	}

	return config, nil
}
