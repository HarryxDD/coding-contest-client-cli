package main

import (
	"coding-contest-client-cli/internal/api"
	"coding-contest-client-cli/internal/config"
	"coding-contest-client-cli/internal/ui"
	"coding-contest-client-cli/internal/watch"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.FromFlagsAndEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	client := api.New(cfg.BaseURL, cfg.APIPrefix, cfg.Token, cfg.Timeout)
	renderer := ui.NewTerminalRenderer()
	watcher := watch.New(client, renderer, cfg.ContestID, cfg.Interval, cfg.TopN)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := watcher.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
		os.Exit(1)
	}
}
