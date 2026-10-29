package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"prod-go/internal/app"
	"prod-go/internal/config"
)

func main() {
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

	log.Printf("starting API on %s", cfg.HTTPAddr)
	if err := instance.Run(ctx); err != nil {
		return err
	}
	return nil
}
