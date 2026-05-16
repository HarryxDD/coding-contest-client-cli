package config

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	valid := Config{
		BaseURL:   "http://localhost:3000",
		APIPrefix: "/api",
		Interval:  time.Second,
		Timeout:   2 * time.Second,
		TopN:      10,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := valid
	invalid.BaseURL = ""
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected error for empty base URL")
	}

	invalid = valid
	invalid.TopN = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected error for negative top")
	}
}
