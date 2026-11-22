// Package svc holds the process plumbing every Spotter service shares: reading
// configuration from environment variables and serving HTTP and gRPC listeners until a
// stop signal, then shutting them down within a deadline.
package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
)

// Env reads configuration through getenv (os.Getenv in production) and names the
// variable in every error.
type Env func(string) string

// Required returns the variable's value or an error naming it.
func (e Env) Required(key string) (string, error) {
	if v := e(key); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s is required", key)
}

// Or returns the variable's value, or def if it is unset.
func (e Env) Or(key, def string) string {
	if v := e(key); v != "" {
		return v
	}
	return def
}

// Duration parses a positive duration like "10s", or returns def if the variable is unset.
func (e Env) Duration(key string, def time.Duration) (time.Duration, error) {
	s := e(key)
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration like 10s, got %q", key, s)
	}
	return d, nil
}

// Server is anything that serves on a listener and can be shut down within a deadline.
type Server interface {
	Serve(net.Listener) error
	Shutdown(ctx context.Context) error
}

// HTTP wraps an http.Handler as a Server.
func HTTP(h http.Handler) Server {
	return &httpServer{srv: &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}}
}

type httpServer struct{ srv *http.Server }

func (s *httpServer) Serve(ln net.Listener) error {
	if err := s.srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
func (s *httpServer) Shutdown(ctx context.Context) error { return s.srv.Shutdown(ctx) }

// GRPC wraps a grpc.Server as a Server. Shutdown waits for in-flight calls
// (GracefulStop) and forces them closed (Stop) if the deadline passes.
func GRPC(s *grpc.Server) Server { return &grpcServer{srv: s} }

type grpcServer struct{ srv *grpc.Server }

func (s *grpcServer) Serve(ln net.Listener) error {
	if err := s.srv.Serve(ln); !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

func (s *grpcServer) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() { s.srv.GracefulStop(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.srv.Stop()
		<-done
		return fmt.Errorf("grpc: forced stop: %w", ctx.Err())
	}
}

// Listener is one server bound to one address.
type Listener struct {
	Name   string // "http", "grpc"; used in logs and errors
	Addr   string // e.g. ":8080" or "127.0.0.1:0"
	Server Server
}

// Options controls Run.
type Options struct {
	Logger          *slog.Logger
	ShutdownTimeout time.Duration
	// OnListening, if set, is called with each bound address once all listeners are up.
	OnListening func(name, addr string)
	// BeforeShutdown, if set, runs first when ctx is cancelled (e.g. mark health NOT_SERVING).
	BeforeShutdown func()
}

// Run binds every listener, serves until ctx is cancelled or a server fails, then shuts
// all servers down in parallel within ShutdownTimeout.
func Run(ctx context.Context, opts Options, listeners ...Listener) error {
	bound := make([]net.Listener, 0, len(listeners))
	for _, l := range listeners {
		ln, err := net.Listen("tcp", l.Addr)
		if err != nil {
			for _, b := range bound {
				b.Close()
			}
			return fmt.Errorf("%s listen on %s: %w", l.Name, l.Addr, err)
		}
		bound = append(bound, ln)
	}

	serveErr := make(chan error, len(listeners))
	for i, l := range listeners {
		go func() {
			if err := l.Server.Serve(bound[i]); err != nil {
				serveErr <- fmt.Errorf("%s server: %w", l.Name, err)
			}
		}()
		opts.Logger.Info("listening", "server", l.Name, "addr", bound[i].Addr().String())
	}
	if opts.OnListening != nil {
		for i, l := range listeners {
			opts.OnListening(l.Name, bound[i].Addr().String())
		}
	}

	var runErr error
	select {
	case runErr = <-serveErr:
	case <-ctx.Done():
	}

	opts.Logger.Info("shutting down", "timeout", opts.ShutdownTimeout.String())
	if opts.BeforeShutdown != nil {
		opts.BeforeShutdown()
	}
	// A fresh context: ctx is already cancelled, but in-flight work gets the timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), opts.ShutdownTimeout)
	defer cancel()
	var wg sync.WaitGroup
	errs := make([]error, len(listeners))
	for i, l := range listeners {
		wg.Go(func() {
			if err := l.Server.Shutdown(shutdownCtx); err != nil {
				errs[i] = fmt.Errorf("%s shutdown: %w", l.Name, err)
			}
		})
	}
	wg.Wait()
	return errors.Join(append([]error{runErr}, errs...)...)
}
