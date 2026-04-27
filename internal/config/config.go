package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	BaseURL   string
	APIPrefix string
	Token     string
	ContestID string
	Interval  time.Duration
	Timeout   time.Duration
	TopN      int
}

func FromFlagsAndEnv() (Config, error) {
	baseURLDefault := envOr("CCS_BASE_URL", "http://localhost:3000")
	apiPrefixDefault := envOr("CCS_API_PREFIX", "/api")
	tokenDefault := envOr("CCS_TOKEN", "")
	contestDefault := envOr("CCS_CONTEST_ID", "")
	intervalDefault := envOr("CCS_INTERVAL", "10s")
	timeoutDefault := envOr("CCS_TIMEOUT", "20s")
	topDefault := envOr("CCS_TOP", "10")

	baseURL := flag.String("base-url", baseURLDefault, "API base URL (example: http://localhost:3000)")
	apiPrefix := flag.String("api-prefix", apiPrefixDefault, "API prefix (example: /api)")
	token := flag.String("token", tokenDefault, "JWT or PAT token")
	contestID := flag.String("contest-id", contestDefault, "Contest UUID to watch")
	intervalRaw := flag.String("interval", intervalDefault, "Polling interval (example: 10s)")
	timeoutRaw := flag.String("timeout", timeoutDefault, "HTTP timeout (example: 20s)")
	topN := flag.Int("top", atoiSafe(topDefault, 10), "Top N rows shown in leaderboard (0 = all)")

	flag.Parse()

	interval, err := time.ParseDuration(*intervalRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid --interval: %w", err)
	}

	timeout, err := time.ParseDuration(*timeoutRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid --timeout: %w", err)
	}

	cfg := Config{
		BaseURL:   *baseURL,
		APIPrefix: *apiPrefix,
		Token:     *token,
		ContestID: *contestID,
		Interval:  interval,
		Timeout:   timeout,
		TopN:      *topN,
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.BaseURL == "" {
		return errors.New("base URL is required")
	}
	if c.APIPrefix == "" {
		return errors.New("api prefix is required")
	}
	if c.Token == "" {
		return errors.New("token is required (use --token or CCS_TOKEN)")
	}
	if c.ContestID == "" {
		return errors.New("contest ID is required (use --contest-id or CCS_CONTEST_ID)")
	}
	if c.Interval < time.Second {
		return errors.New("interval must be >= 1s")
	}
	if c.Timeout < time.Second {
		return errors.New("timeout must be >= 1s")
	}
	if c.TopN < 0 {
		return errors.New("top must be >= 0")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func atoiSafe(v string, fallback int) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
