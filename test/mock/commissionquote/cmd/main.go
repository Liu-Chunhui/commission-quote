package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"commissionquote/mock/internal/app"
	"commissionquote/mock/internal/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Value.Kind() == slog.KindTime {
				attr.Value = slog.TimeValue(attr.Value.Time().UTC())
			}
			return attr
		},
	})))
	configPath := flag.String("config", "config/dev.json", "Path to the service configuration")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		os.Exit(1)
	}

	server := &http.Server{
		Addr:    "localhost:" + strconv.Itoa(cfg.Port),
		Handler: app.NewRouter(cfg),
	}

	slog.Info("commission quote starting", "service", "commission-quote", "operation", "startup")

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Unable to serve HTTP requests", "service", "commission-quote", "operation", "serve", "error_code", "INTERNAL_ERROR", "cause", err)
		os.Exit(1)
	}
}
