// Command api is the HTTP service entry point. It is the only place allowed to create a root context.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/accesslog"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/auth"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/config"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/db"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/logging"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/requestid"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	log := logging.New(os.Stdout, cfg.LogLevel)
	slog.SetDefault(log)

	shutdownTracing, err := telemetry.Setup(ctx, cfg.TracesExporter)
	if err != nil {
		return err
	}
	defer func() { _ = shutdownTracing(context.WithoutCancel(ctx)) }()

	authn, err := auth.New(cfg.AuthMode)
	if err != nil {
		return err
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler(log, authn, routes(pool)),
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.HTTPAddr)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// handler wraps the routes in the platform middleware, outermost first: the request ID (on every response
// and log line), the OTel server span, the access log line, authentication, and the route capture that the
// access log and the span name read. The order is an invariant (ADR 0018), pinned by main_test.go.
func handler(log *slog.Logger, authn func(http.Handler) http.Handler, mux http.Handler) http.Handler {
	return requestid.Middleware(telemetry.Middleware("api")(accesslog.Middleware(log)(authn(accesslog.Route(mux)))))
}
