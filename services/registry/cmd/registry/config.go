package main

import (
	"time"

	"github.com/Pohi11/spotter/internal/svc"
)

// config is everything the registry reads from the environment.
type config struct {
	DatabaseURL     string
	NATSURL         string
	HTTPAddr        string
	GRPCAddr        string
	ShutdownTimeout time.Duration
}

func loadConfig(getenv func(string) string) (config, error) {
	env := svc.Env(getenv)
	dbURL, err := env.Required("DATABASE_URL")
	if err != nil {
		return config{}, err
	}
	natsURL, err := env.Required("NATS_URL")
	if err != nil {
		return config{}, err
	}
	timeout, err := env.Duration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return config{}, err
	}
	return config{
		DatabaseURL:     dbURL,
		NATSURL:         natsURL,
		HTTPAddr:        env.Or("HTTP_ADDR", ":8080"),
		GRPCAddr:        env.Or("GRPC_ADDR", ":9090"),
		ShutdownTimeout: timeout,
	}, nil
}
