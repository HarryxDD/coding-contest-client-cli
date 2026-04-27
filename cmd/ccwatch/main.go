package main

import (
	"coding-contest-client-cli/internal/api"
	"coding-contest-client-cli/internal/config"
	"coding-contest-client-cli/internal/tui"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
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
	app := tui.NewApp(cfg, client)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "application error: %v\n", err)
		os.Exit(1)
	}
}
