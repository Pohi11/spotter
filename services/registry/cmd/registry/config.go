package main

import (
	"fmt"
	"time"
)

// config is everything the registry reads from the environment (design D10).
type config struct {
	DatabaseURL     string
	HTTPAddr        string
	ShutdownTimeout time.Duration
}

// loadConfig reads the configuration through getenv (os.Getenv in production) and
// names the variable in every error.
func loadConfig(getenv func(string) string) (config, error) {
	c := config{
		DatabaseURL:     getenv("DATABASE_URL"),
		HTTPAddr:        getenv("HTTP_ADDR"),
		ShutdownTimeout: 10 * time.Second,
	}
	if c.DatabaseURL == "" {
		return config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if c.HTTPAddr == "" {
		c.HTTPAddr = ":8080"
	}
	if s := getenv("SHUTDOWN_TIMEOUT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration like 10s, got %q", s)
		}
		c.ShutdownTimeout = d
	}
	return c, nil
}
