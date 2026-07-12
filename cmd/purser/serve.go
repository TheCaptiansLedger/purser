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

	afterdarkv1connect "purser/gen/go/purser/afterdark/v1/afterdarkv1connect"
	domainv1connect "purser/gen/go/purser/domain/v1/domainv1connect"
	preentryperson "purser/internal/adapters/memory/entryperson"
	memexternalid "purser/internal/adapters/memory/externalid"
	memgroup "purser/internal/adapters/memory/group"
	memimage "purser/internal/adapters/memory/image"
	memitem "purser/internal/adapters/memory/item"
	memitemperson "purser/internal/adapters/memory/itemperson"
	memlibraryentry "purser/internal/adapters/memory/libraryentry"
	memmediafile "purser/internal/adapters/memory/mediafile"
	memperformerprofile "purser/internal/adapters/memory/performerprofile"
	memperson "purser/internal/adapters/memory/person"
	memtag "purser/internal/adapters/memory/tag"
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
// ADR-0011). Every shared-kernel entity in this pass follows the exact
// same four-line shape; that repetition is intentional (SRP per entity)
// rather than a signal to collapse it into a generic helper.
func newServeMux(logger *slog.Logger) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	interceptors := connect.WithInterceptors(apiconnect.NewLoggingInterceptor(logger))

	personRepo, err := memperson.New("person", memperson.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing person repository: %w", err)
	}
	personHandler := apiconnect.NewPersonHandler(service.NewPersonService(personRepo), logger)
	personPath, personConnectHandler := domainv1connect.NewPersonServiceHandler(personHandler, interceptors)
	mux.Handle(personPath, personConnectHandler)

	libraryEntryRepo, err := memlibraryentry.New("library_entry", memlibraryentry.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing library entry repository: %w", err)
	}
	libraryEntryHandler := apiconnect.NewLibraryEntryHandler(service.NewLibraryEntryService(libraryEntryRepo), logger)
	libraryEntryPath, libraryEntryConnectHandler := domainv1connect.NewLibraryEntryServiceHandler(libraryEntryHandler, interceptors)
	mux.Handle(libraryEntryPath, libraryEntryConnectHandler)

	groupRepo, err := memgroup.New("group", memgroup.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing group repository: %w", err)
	}
	groupHandler := apiconnect.NewGroupHandler(service.NewGroupService(groupRepo), logger)
	groupPath, groupConnectHandler := domainv1connect.NewGroupServiceHandler(groupHandler, interceptors)
	mux.Handle(groupPath, groupConnectHandler)

	itemRepo, err := memitem.New("item", memitem.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing item repository: %w", err)
	}
	itemHandler := apiconnect.NewItemHandler(service.NewItemService(itemRepo), logger)
	itemPath, itemConnectHandler := domainv1connect.NewItemServiceHandler(itemHandler, interceptors)
	mux.Handle(itemPath, itemConnectHandler)

	entryPersonRepo, err := preentryperson.New("entry_person", preentryperson.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing entry person repository: %w", err)
	}
	entryPersonHandler := apiconnect.NewEntryPersonHandler(service.NewEntryPersonService(entryPersonRepo), logger)
	entryPersonPath, entryPersonConnectHandler := domainv1connect.NewEntryPersonServiceHandler(entryPersonHandler, interceptors)
	mux.Handle(entryPersonPath, entryPersonConnectHandler)

	itemPersonRepo, err := memitemperson.New("item_person", memitemperson.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing item person repository: %w", err)
	}
	itemPersonHandler := apiconnect.NewItemPersonHandler(service.NewItemPersonService(itemPersonRepo), logger)
	itemPersonPath, itemPersonConnectHandler := domainv1connect.NewItemPersonServiceHandler(itemPersonHandler, interceptors)
	mux.Handle(itemPersonPath, itemPersonConnectHandler)

	tagRepo, err := memtag.New("tag", memtag.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing tag repository: %w", err)
	}
	tagHandler := apiconnect.NewTagHandler(service.NewTagService(tagRepo), logger)
	tagPath, tagConnectHandler := domainv1connect.NewTagServiceHandler(tagHandler, interceptors)
	mux.Handle(tagPath, tagConnectHandler)

	externalIDRepo, err := memexternalid.New("external_id", memexternalid.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing external id repository: %w", err)
	}
	externalIDHandler := apiconnect.NewExternalIDHandler(service.NewExternalIDService(externalIDRepo), logger)
	externalIDPath, externalIDConnectHandler := domainv1connect.NewExternalIDServiceHandler(externalIDHandler, interceptors)
	mux.Handle(externalIDPath, externalIDConnectHandler)

	imageRepo, err := memimage.New("image", memimage.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing image repository: %w", err)
	}
	imageHandler := apiconnect.NewImageHandler(service.NewImageService(imageRepo), logger)
	imagePath, imageConnectHandler := domainv1connect.NewImageServiceHandler(imageHandler, interceptors)
	mux.Handle(imagePath, imageConnectHandler)

	mediaFileRepo, err := memmediafile.New("media_file", memmediafile.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing media file repository: %w", err)
	}
	mediaFileHandler := apiconnect.NewMediaFileHandler(service.NewMediaFileService(mediaFileRepo), logger)
	mediaFilePath, mediaFileConnectHandler := domainv1connect.NewMediaFileServiceHandler(mediaFileHandler, interceptors)
	mux.Handle(mediaFilePath, mediaFileConnectHandler)

	// AfterDark: the first module built on the shared kernel above — every
	// line here is additive, nothing in the kernel wiring changed to add it.
	performerProfileRepo, err := memperformerprofile.New("performer_profile", memperformerprofile.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing performer profile repository: %w", err)
	}
	performerProfileHandler := apiconnect.NewPerformerProfileHandler(service.NewPerformerProfileService(performerProfileRepo), logger)
	performerProfilePath, performerProfileConnectHandler := afterdarkv1connect.NewPerformerProfileServiceHandler(performerProfileHandler, interceptors)
	mux.Handle(performerProfilePath, performerProfileConnectHandler)

	reflector := grpcreflect.NewStaticReflector(
		domainv1connect.PersonServiceName,
		domainv1connect.LibraryEntryServiceName,
		domainv1connect.GroupServiceName,
		domainv1connect.ItemServiceName,
		domainv1connect.EntryPersonServiceName,
		domainv1connect.ItemPersonServiceName,
		domainv1connect.TagServiceName,
		domainv1connect.ExternalIDServiceName,
		domainv1connect.ImageServiceName,
		domainv1connect.MediaFileServiceName,
		afterdarkv1connect.PerformerProfileServiceName,
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	return mux, nil
}
