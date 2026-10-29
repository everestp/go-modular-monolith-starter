// Package main starts the HTTP API server.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"prod-go/internal/app"
	"prod-go/internal/config"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "startup failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	instance, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer instance.Close()

	if err := instance.Run(ctx); err != nil {
		return err
	}
	return nil
}

func healthcheck() error {
	client := http.Client{Timeout: 2 * time.Second}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:8080/healthz", nil)
	if err != nil {
		return fmt.Errorf("create health check request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("health check request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", response.StatusCode)
	}
	return nil
}
