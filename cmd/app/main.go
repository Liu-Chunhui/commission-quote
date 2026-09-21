package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"commissionquote/internal/app"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	configPath := flag.String("config", "confg/dev.json", "path to application JSON configuration")
	flag.Parse()
	config, err := app.LoadConfig(*configPath)
	if err != nil {
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              "localhost:" + strconv.Itoa(config.Port),
		Handler:           app.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("HTTP server starting", "service", "app", "operation", "serve", "port", config.Port)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("HTTP server stopped", "service", "app", "operation", "serve",
			"cause", "unable to listen or serve HTTP")
		os.Exit(1)
	}
}
