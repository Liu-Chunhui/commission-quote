package main

import (
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"commissionquote/internal/app"
	"commissionquote/internal/config"
	"commissionquote/internal/integration/commissionquote"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Value.Kind() == slog.KindTime {
				attr.Value = slog.TimeValue(attr.Value.Time().UTC())
			}
			return attr
		},
	})))

	configPath := flag.String("config", "confg/dev.json", "path to application JSON configuration")
	flag.Parse()

	appConfig, err := config.LoadConfig(*configPath)
	if err != nil {
		os.Exit(1)
	}

	httpClient := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	quoteClient := commissionquote.NewQuoteClient(httpClient, appConfig.Dependencies.CommissionQuote.BaseURL, appConfig.APIKey)
	server := &http.Server{
		Addr:              net.JoinHostPort(appConfig.Host, strconv.Itoa(appConfig.Port)),
		Handler:           app.NewRouter(quoteClient),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("HTTP server starting", "service", "app", "operation", "serve", "port", appConfig.Port)

	if err := server.ListenAndServe(); err != nil {
		slog.Error("HTTP server stopped", "service", "app", "operation", "serve",
			"cause", "unable to listen or serve HTTP")
		os.Exit(1)
	}
}
