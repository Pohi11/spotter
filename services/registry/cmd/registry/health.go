package main

import (
	"context"
	"net/http"
	"time"
)

// pinger is the one database capability readiness needs. *pgxpool.Pool satisfies it.
type pinger interface {
	Ping(ctx context.Context) error
}

// healthHandler serves /livez, /readyz, and /metrics.
//
// /livez never touches dependencies: a slow database must not get the pod restarted.
// /readyz pings the database: while it fails, the pod receives no traffic.
func healthHandler(db pinger, metrics http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("GET /metrics", metrics)
	return mux
}
