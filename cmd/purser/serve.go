package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"purser/internal/config"
	"purser/internal/service"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	domainv1connect "purser/gen/go/purser/domain/v1/domainv1connect"
	memperson "purser/internal/adapters/memory/person"
	apiconnect "purser/internal/api/connect"
)

func newServeCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Purser Connect/gRPC API server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd.Context(), configPath)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "path to ops/purser.yaml (optional)")
	return cmd
}

// runServe is the composition root for the server process: it installs
// the process-wide slog handler, loads config, wires the in-memory
// adapters to their services and Connect handlers, and serves until an
// interrupt/TERM signal or a listener error.
func runServe(ctx context.Context, configPath string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(viper.New(), configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Telemetry: no SDK/exporter is wired yet (OTLP/Prometheus config is a
	// follow-up, tracked as a gap, not silently skipped — see ADR 0007).
	// The global TracerProvider/MeterProvider stay at OTel's own no-op
	// default, which every instrumented package already supports at zero
	// cost per ADR 0007's design.

	mux, err := newServeMux(logger)
	if err != nil {
		return err
	}

	// Native gRPC requires HTTP/2. There's no TLS in front of this dev/local
	// server, so unencrypted ("h2c") HTTP/2 must be explicitly enabled
	// alongside HTTP/1.1 (for Connect's HTTP/JSON transport) — the stdlib
	// http.Server.Protocols field replaces the older golang.org/x/net/http2/h2c
	// helper as of Go 1.24.
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:              cfg.Server.ListenAddr,
		Handler:           mux,
		Protocols:         protocols,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", cfg.Server.ListenAddr)
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-sigCtx.Done():
		logger.Info("shutting down")
		// sigCtx is already Done() at this point, so the shutdown timeout
		// derives from it via WithoutCancel (keeps it a child context,
		// satisfying contextcheck) rather than an already-expired context.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(sigCtx), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case serveErr := <-errCh:
		return serveErr
	}
}

// newServeMux wires every entity's adapter -> service -> Connect handler
// and mounts it, plus gRPC reflection for grpcurl/buf curl debugging (see
// ADR-0011). One entity today (Person); Phase 2 adds the rest the same
// way.
func newServeMux(logger *slog.Logger) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	interceptors := connect.WithInterceptors(apiconnect.NewLoggingInterceptor(logger))

	personRepo, err := memperson.New("person", memperson.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing person repository: %w", err)
	}
	personSvc := service.NewPersonService(personRepo)
	personHandler := apiconnect.NewPersonHandler(personSvc, logger)
	personPath, personConnectHandler := domainv1connect.NewPersonServiceHandler(personHandler, interceptors)
	mux.Handle(personPath, personConnectHandler)

	reflector := grpcreflect.NewStaticReflector(domainv1connect.PersonServiceName)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	return mux, nil
}
