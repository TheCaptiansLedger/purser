package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"purser/internal/adapters/datastore"
	"purser/internal/config"
	"purser/internal/ports"
	"purser/internal/service"
	"purser/pkg/fswatch"
	"purser/pkg/fswatch/fsnotify"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	afterdarkv1connect "purser/gen/go/purser/afterdark/v1/afterdarkv1connect"
	domainv1connect "purser/gen/go/purser/domain/v1/domainv1connect"
	jobv1connect "purser/gen/go/purser/job/v1/jobv1connect"
	musicv1connect "purser/gen/go/purser/music/v1/musicv1connect"
	pipelinev1connect "purser/gen/go/purser/pipeline/v1/pipelinev1connect"
	dsbadger "purser/internal/adapters/datastore/badger"
	dssql "purser/internal/adapters/datastore/sql"
	filewalkerlocal "purser/internal/adapters/filewalker/local"
	adapterjobqueue "purser/internal/adapters/jobqueue"
	adapterpipeline "purser/internal/adapters/pipeline"
	storeentryperson "purser/internal/adapters/store/entryperson"
	storeexternalid "purser/internal/adapters/store/externalid"
	storegroup "purser/internal/adapters/store/group"
	storeimage "purser/internal/adapters/store/image"
	storeitem "purser/internal/adapters/store/item"
	storeitemperson "purser/internal/adapters/store/itemperson"
	storelibraryentry "purser/internal/adapters/store/libraryentry"
	storemediafile "purser/internal/adapters/store/mediafile"
	storemusicrelease "purser/internal/adapters/store/music"
	storeperformerprofile "purser/internal/adapters/store/performerprofile"
	storeperson "purser/internal/adapters/store/person"
	storetag "purser/internal/adapters/store/tag"
	storetagassignment "purser/internal/adapters/store/tagassignment"
	storeunmatchedfile "purser/internal/adapters/store/unmatchedfile"
	apiconnect "purser/internal/api/connect"

	pkgjobqueue "purser/pkg/jobqueue"
	jobqueuememory "purser/pkg/jobqueue/memory"
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

	shutdownTelemetry, err := setupTelemetry(ctx, cfg.Telemetry, logger)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if closeErr := shutdownTelemetry(shutdownCtx); closeErr != nil {
			logger.Error("shutting down telemetry", "error", closeErr)
		}
	}()

	// sigCtx is created here (rather than immediately before srv.ListenAndServe,
	// where it used to live) so it can also bound the filesystem watcher's
	// lifetime below — both the HTTP server and the watcher/scan-trigger
	// consumer goroutine shut down on the same signal.
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	ds, dsCloser, err := openDatastore(cfg.Database)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dsCloser.Close(); closeErr != nil {
			logger.Error("closing datastore", "error", closeErr)
		}
	}()

	mux, watcherCloser, err := newServeMux(sigCtx, logger, ds, cfg.Pipeline)
	if err != nil {
		return err
	}
	if watcherCloser != nil {
		defer func() {
			if closeErr := watcherCloser.Close(); closeErr != nil {
				logger.Error("closing filesystem watcher", "error", closeErr)
			}
		}()
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
		Handler:           otelhttp.NewHandler(mux, "purser"),
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

// openDatastore opens the datastore.Datastore backend selected by
// cfg.Driver and returns it alongside the underlying *badger.DB/*sql.DB so
// the caller can close it on shutdown — see
// docs/adr/0012-datastore-persistence.md. This is the only place in the
// codebase that knows both backends exist; everything above the returned
// datastore.Datastore is backend-agnostic.
func openDatastore(cfg config.Database) (datastore.Datastore, io.Closer, error) {
	switch cfg.Driver {
	case "badger":
		db, err := dsbadger.Open(dsbadger.Options{
			DataDir:     cfg.Badger.DataDir,
			ValueLogDir: cfg.Badger.ValueLogDir,
			SyncWrites:  cfg.Badger.SyncWrites,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("cmd/purser: opening badger datastore: %w", err)
		}
		store, err := dsbadger.New("kernel", db)
		if err != nil {
			return nil, nil, fmt.Errorf("cmd/purser: constructing badger datastore: %w", err)
		}
		return store, db, nil
	case "postgres", "mysql", "sqlite":
		dialect := dssql.Dialect(cfg.Driver)
		db, err := dssql.Open(dssql.Options{Dialect: dialect, DSN: cfg.SQL.DSN})
		if err != nil {
			return nil, nil, fmt.Errorf("cmd/purser: opening sql datastore (%s): %w", cfg.Driver, err)
		}
		store, err := dssql.New("kernel", db, dialect)
		if err != nil {
			return nil, nil, fmt.Errorf("cmd/purser: constructing sql datastore: %w", err)
		}
		return store, db, nil
	default:
		// config.Database.Validate already rejects this at Load time —
		// reachable only if a caller constructs a Database bypassing
		// Validate, so this is a defensive fallback, not the primary
		// error path.
		return nil, nil, fmt.Errorf("cmd/purser: unknown database driver %q", cfg.Driver)
	}
}

// newServeMux wires every entity's adapter -> service -> Connect handler
// and mounts it, plus gRPC reflection for grpcurl/buf curl debugging (see
// ADR-0011). Every shared-kernel entity in this pass follows the exact
// same four-line shape; that repetition is intentional (SRP per entity)
// rather than a signal to collapse it into a generic helper.
//
// Every entity is backed by the single shared ds — see
// docs/adr/0012-datastore-persistence.md. ctx bounds the lifetime of the
// Common Scan Pipeline's filesystem watcher/consumer goroutine, if one is
// started (see wireScanPipeline); the returned io.Closer stops it during
// shutdown and is nil when pipelineCfg.ScanRoots is empty.
func newServeMux(ctx context.Context, logger *slog.Logger, ds datastore.Datastore, pipelineCfg config.Pipeline) (*http.ServeMux, io.Closer, error) {
	mux := http.NewServeMux()
	interceptors := connect.WithInterceptors(apiconnect.NewLoggingInterceptor(logger))

	// personRepo's handler (personHandler) is constructed further down,
	// after PersonDeletionService's other referrer repos (entryPersonRepo,
	// itemPersonRepo, externalIDRepo, imageRepo, tagAssignmentRepo,
	// performerProfileRepo) exist — see
	// docs/adr/0015-deletion-impact-and-composing-services.md.
	personRepo, err := storeperson.New("person", ds, storeperson.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing person repository: %w", err)
	}

	// libraryEntryRepo's handler (libraryEntryHandler) is constructed
	// further down, after LibraryEntryDeletionService's dependencies
	// (groupRepo, itemRepo, entryPersonRepo, externalIDRepo, imageRepo,
	// tagAssignmentRepo, groupDeletionSvc, itemDeletionSvc) all exist —
	// see docs/adr/0015-deletion-impact-and-composing-services.md.
	libraryEntryRepo, err := storelibraryentry.New("library_entry", ds, storelibraryentry.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing library entry repository: %w", err)
	}

	// groupRepo's handler (groupHandler) is constructed further down, after
	// GroupDeletionService's other referrer repos (itemRepo, externalIDRepo,
	// imageRepo, tagAssignmentRepo) exist — see
	// docs/adr/0015-deletion-impact-and-composing-services.md.
	groupRepo, err := storegroup.New("group", ds, storegroup.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing group repository: %w", err)
	}

	// itemRepo's handler (itemHandler) is constructed further down, after
	// ItemDeletionService's other referrer repos (itemPersonRepo,
	// mediaFileRepo, externalIDRepo, imageRepo, tagAssignmentRepo) exist —
	// see docs/adr/0015-deletion-impact-and-composing-services.md.
	itemRepo, err := storeitem.New("item", ds, storeitem.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing item repository: %w", err)
	}

	entryPersonRepo, err := storeentryperson.New("entry_person", ds, storeentryperson.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing entry person repository: %w", err)
	}
	entryPersonHandler := apiconnect.NewEntryPersonHandler(service.NewEntryPersonService(entryPersonRepo), logger)
	entryPersonPath, entryPersonConnectHandler := domainv1connect.NewEntryPersonServiceHandler(entryPersonHandler, interceptors)
	mux.Handle(entryPersonPath, entryPersonConnectHandler)

	itemPersonRepo, err := storeitemperson.New("item_person", ds, storeitemperson.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing item person repository: %w", err)
	}
	itemPersonHandler := apiconnect.NewItemPersonHandler(service.NewItemPersonService(itemPersonRepo), logger)
	itemPersonPath, itemPersonConnectHandler := domainv1connect.NewItemPersonServiceHandler(itemPersonHandler, interceptors)
	mux.Handle(itemPersonPath, itemPersonConnectHandler)

	tagRepo, err := storetag.New("tag", ds, storetag.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing tag repository: %w", err)
	}

	tagAssignmentRepo, err := storetagassignment.New("tag_assignment", ds, storetagassignment.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing tag assignment repository: %w", err)
	}
	tagAssignmentHandler := apiconnect.NewTagAssignmentHandler(service.NewTagAssignmentService(tagAssignmentRepo), logger)
	tagAssignmentPath, tagAssignmentConnectHandler := domainv1connect.NewTagAssignmentServiceHandler(tagAssignmentHandler, interceptors)
	mux.Handle(tagAssignmentPath, tagAssignmentConnectHandler)

	// TagDeletionService is the composing-service exception per
	// docs/adr/0015-deletion-impact-and-composing-services.md — it reuses
	// the already-constructed tagRepo/tagAssignmentRepo.
	tagDeletionSvc := service.NewTagDeletionService(tagRepo, tagAssignmentRepo)
	tagHandler := apiconnect.NewTagHandler(service.NewTagService(tagRepo), tagDeletionSvc, logger)
	tagPath, tagConnectHandler := domainv1connect.NewTagServiceHandler(tagHandler, interceptors)
	mux.Handle(tagPath, tagConnectHandler)

	externalIDRepo, err := storeexternalid.New("external_id", ds, storeexternalid.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing external id repository: %w", err)
	}
	externalIDHandler := apiconnect.NewExternalIDHandler(service.NewExternalIDService(externalIDRepo), logger)
	externalIDPath, externalIDConnectHandler := domainv1connect.NewExternalIDServiceHandler(externalIDHandler, interceptors)
	mux.Handle(externalIDPath, externalIDConnectHandler)

	imageRepo, err := storeimage.New("image", ds, storeimage.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing image repository: %w", err)
	}
	imageHandler := apiconnect.NewImageHandler(service.NewImageService(imageRepo), logger)
	imagePath, imageConnectHandler := domainv1connect.NewImageServiceHandler(imageHandler, interceptors)
	mux.Handle(imagePath, imageConnectHandler)

	mediaFileRepo, err := storemediafile.New("media_file", ds, storemediafile.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing media file repository: %w", err)
	}
	mediaFileHandler := apiconnect.NewMediaFileHandler(service.NewMediaFileService(mediaFileRepo), logger)
	mediaFilePath, mediaFileConnectHandler := domainv1connect.NewMediaFileServiceHandler(mediaFileHandler, interceptors)
	mux.Handle(mediaFilePath, mediaFileConnectHandler)

	// ItemDeletionService is the composing-service exception per
	// docs/adr/0015-deletion-impact-and-composing-services.md — it reuses
	// the already-constructed itemRepo/itemPersonRepo/mediaFileRepo/
	// externalIDRepo/imageRepo/tagAssignmentRepo.
	itemDeletionSvc := service.NewItemDeletionService(itemRepo, itemPersonRepo, mediaFileRepo, externalIDRepo, imageRepo, tagAssignmentRepo)
	itemHandler := apiconnect.NewItemHandler(service.NewItemService(itemRepo), itemDeletionSvc, logger)
	itemPath, itemConnectHandler := domainv1connect.NewItemServiceHandler(itemHandler, interceptors)
	mux.Handle(itemPath, itemConnectHandler)

	// Music's release repository and deletion service must exist before
	// GroupDeletionService/LibraryEntryDeletionService below — per
	// docs/adr/0021-music-domain-model.md's "Ripple effects" section, both
	// need MusicRelease as a referrer. Music's own handler is registered
	// further down, alongside the rest of the Music module.
	musicReleaseRepo, err := storemusicrelease.New("music_release", ds, storemusicrelease.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing music release repository: %w", err)
	}
	// MusicReleaseDeletionService is the composing-service exception per
	// docs/adr/0015-deletion-impact-and-composing-services.md — it reuses
	// the already-constructed musicReleaseRepo/itemRepo.
	musicReleaseDeletionSvc := service.NewMusicReleaseDeletionService(musicReleaseRepo, itemRepo)

	// GroupDeletionService is the composing-service exception per
	// docs/adr/0015-deletion-impact-and-composing-services.md — it reuses
	// the already-constructed groupRepo/itemRepo/externalIDRepo/imageRepo/
	// tagAssignmentRepo, plus musicReleaseRepo/musicReleaseDeletionSvc per
	// docs/adr/0021-music-domain-model.md's "Ripple effects" section.
	groupDeletionSvc := service.NewGroupDeletionService(groupRepo, itemRepo, externalIDRepo, imageRepo, tagAssignmentRepo, musicReleaseRepo, musicReleaseDeletionSvc)
	groupHandler := apiconnect.NewGroupHandler(service.NewGroupService(groupRepo), groupDeletionSvc, logger)
	groupPath, groupConnectHandler := domainv1connect.NewGroupServiceHandler(groupHandler, interceptors)
	mux.Handle(groupPath, groupConnectHandler)

	// LibraryEntryDeletionService is the composing-service exception per
	// docs/adr/0015-deletion-impact-and-composing-services.md — it reuses
	// every already-constructed referrer repo plus the GroupDeletionService/
	// ItemDeletionService constructed just above, since a cascade delete
	// recurses into both. musicReleaseRepo is used for accurate
	// GetDeletionImpact counting only — its Delete flow already falls out
	// of the cascade into groupDeletionSvc — per
	// docs/adr/0021-music-domain-model.md's "Ripple effects" section.
	libraryEntryDeletionSvc := service.NewLibraryEntryDeletionService(
		libraryEntryRepo, groupRepo, itemRepo, entryPersonRepo, externalIDRepo, imageRepo, tagAssignmentRepo, musicReleaseRepo,
		groupDeletionSvc, itemDeletionSvc,
	)
	libraryEntryHandler := apiconnect.NewLibraryEntryHandler(service.NewLibraryEntryService(libraryEntryRepo), libraryEntryDeletionSvc, logger)
	libraryEntryPath, libraryEntryConnectHandler := domainv1connect.NewLibraryEntryServiceHandler(libraryEntryHandler, interceptors)
	mux.Handle(libraryEntryPath, libraryEntryConnectHandler)

	// AfterDark: the first module built on the shared kernel above — every
	// line here is additive, nothing in the kernel wiring changed to add it.
	performerProfileRepo, err := storeperformerprofile.New("performer_profile", ds, storeperformerprofile.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing performer profile repository: %w", err)
	}
	performerProfileHandler := apiconnect.NewPerformerProfileHandler(service.NewPerformerProfileService(performerProfileRepo), logger)
	performerProfilePath, performerProfileConnectHandler := afterdarkv1connect.NewPerformerProfileServiceHandler(performerProfileHandler, interceptors)
	mux.Handle(performerProfilePath, performerProfileConnectHandler)

	// PersonDeletionService is the composing-service exception per
	// docs/adr/0015-deletion-impact-and-composing-services.md — it reuses
	// the already-constructed personRepo/entryPersonRepo/itemPersonRepo/
	// externalIDRepo/imageRepo/tagAssignmentRepo/performerProfileRepo.
	personDeletionSvc := service.NewPersonDeletionService(personRepo, entryPersonRepo, itemPersonRepo, externalIDRepo, imageRepo, tagAssignmentRepo, performerProfileRepo)
	personHandler := apiconnect.NewPersonHandler(service.NewPersonService(personRepo), personDeletionSvc, logger)
	personPath, personConnectHandler := domainv1connect.NewPersonServiceHandler(personHandler, interceptors)
	mux.Handle(personPath, personConnectHandler)

	// BrowseService is the composing service exception per
	// docs/adr/0015-deletion-impact-and-composing-services.md — it reuses
	// the kernel repositories already constructed above, no new repository
	// construction needed.
	browseSvc := service.NewAfterDarkBrowseService(libraryEntryRepo, itemRepo, itemPersonRepo, personRepo, performerProfileRepo)
	browseHandler := apiconnect.NewBrowseHandler(browseSvc, logger)
	browsePath, browseConnectHandler := afterdarkv1connect.NewBrowseServiceHandler(browseHandler, interceptors)
	mux.Handle(browsePath, browseConnectHandler)

	// Music: the second module built on the shared kernel — every line here
	// is additive, nothing above changed to add it. See
	// docs/adr/0021-music-domain-model.md. musicReleaseRepo/
	// musicReleaseDeletionSvc are constructed earlier, alongside
	// GroupDeletionService/LibraryEntryDeletionService, since that ADR's
	// "Ripple effects" section requires both to depend on them.
	musicReleaseHandler := apiconnect.NewMusicReleaseHandler(service.NewMusicReleaseService(musicReleaseRepo), musicReleaseDeletionSvc, logger)
	musicReleasePath, musicReleaseConnectHandler := musicv1connect.NewMusicReleaseServiceHandler(musicReleaseHandler, interceptors)
	mux.Handle(musicReleasePath, musicReleaseConnectHandler)

	// Job Queue: ephemeral, in-process — not backed by ds like every
	// entity above. See docs/adr/0023-job-queue.md. jobAdapter satisfies
	// both ports.JobPublisher and ports.JobReader.
	jobEngine := pkgjobqueue.NewEngine(jobqueuememory.New(), pkgjobqueue.WithLogger(logger))
	jobAdapter := adapterjobqueue.New(jobEngine)
	jobHandler := apiconnect.NewJobHandler(service.NewJobService(jobAdapter, jobAdapter), logger)
	jobPath, jobConnectHandler := jobv1connect.NewJobServiceHandler(jobHandler, interceptors)
	mux.Handle(jobPath, jobConnectHandler)

	// Common Scan Pipeline: split into its own function purely to keep
	// newServeMux's cyclomatic complexity under budget — no behavior
	// difference from being inlined here. See docs/adr/0024-pipeline-core.md.
	watcherCloser, err := wireScanPipeline(ctx, mux, ds, logger, interceptors, jobEngine, jobAdapter, itemRepo, mediaFileRepo, pipelineCfg)
	if err != nil {
		return nil, nil, err
	}

	reflector := grpcreflect.NewStaticReflector(
		domainv1connect.PersonServiceName,
		domainv1connect.LibraryEntryServiceName,
		domainv1connect.GroupServiceName,
		domainv1connect.ItemServiceName,
		domainv1connect.EntryPersonServiceName,
		domainv1connect.ItemPersonServiceName,
		domainv1connect.TagServiceName,
		domainv1connect.TagAssignmentServiceName,
		domainv1connect.ExternalIDServiceName,
		domainv1connect.ImageServiceName,
		domainv1connect.MediaFileServiceName,
		afterdarkv1connect.PerformerProfileServiceName,
		afterdarkv1connect.BrowseServiceName,
		musicv1connect.MusicReleaseServiceName,
		jobv1connect.JobServiceName,
		pipelinev1connect.ScanServiceName,
		pipelinev1connect.UnmatchedFileServiceName,
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	return mux, watcherCloser, nil
}

// wireScanPipeline builds the Common Scan Pipeline's adapters, services,
// and Connect handlers, registers the "scan" Executor directly on
// jobEngine (exactly like "diagnostic" self-registers inside
// pkgjobqueue.NewEngine), and mounts ScanService and UnmatchedFileService
// on mux. jobAdapter is reused as ScanService's ports.JobPublisher —
// neither service imports pkg/jobqueue directly. itemRepo/mediaFileRepo
// are the already-constructed ports.ItemRepository/ports.MediaFileRepository
// (built alongside ItemService/MediaFileService above) — ScanExecutor
// needs mediaFileRepo for the "already known" short-circuit's MediaFile-
// side lookup, and UnmatchedFileService needs both for Resolve's match
// outcome (docs/adr/0015's composing-service exception, see
// internal/service/unmatched_file.go). See docs/adr/0023-job-queue.md,
// docs/adr/0024-pipeline-core.md. The returned io.Closer stops the
// filesystem watcher started for pipelineCfg.ScanRoots (see
// startScanWatcher); it is nil when no roots are configured.
func wireScanPipeline(ctx context.Context, mux *http.ServeMux, ds datastore.Datastore, logger *slog.Logger, interceptors connect.HandlerOption, jobEngine *pkgjobqueue.Engine, jobAdapter *adapterjobqueue.Adapter, itemRepo ports.ItemRepository, mediaFileRepo ports.MediaFileRepository, pipelineCfg config.Pipeline) (io.Closer, error) {
	unmatchedFileRepo, err := storeunmatchedfile.New("unmatched_file", ds, storeunmatchedfile.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing unmatched file repository: %w", err)
	}

	// No content-type-specific ports.Grouping implementations exist yet
	// (Music's folder/multi-disc grouping is a separate issue) — every
	// content type falls back to service.IdentityGrouping until one is
	// registered here.
	groupingRegistry := service.NewGroupingRegistry()
	jobEngine.Register("scan", adapterpipeline.NewScanExecutor(unmatchedFileRepo, mediaFileRepo, groupingRegistry))

	fileWalker, err := filewalkerlocal.New(filewalkerlocal.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing file walker: %w", err)
	}

	roots := make([]service.RootContentType, len(pipelineCfg.ScanRoots))
	for i, r := range pipelineCfg.ScanRoots {
		roots[i] = service.RootContentType{Path: r.Path, ContentType: r.ContentType}
	}

	scanSvc := service.NewScanService(jobAdapter, fileWalker, pipelineCfg.EnableMD5, pipelineCfg.EnableSHA512, roots)
	scanHandler := apiconnect.NewScanHandler(scanSvc, logger)
	scanPath, scanConnectHandler := pipelinev1connect.NewScanServiceHandler(scanHandler, interceptors)
	mux.Handle(scanPath, scanConnectHandler)

	unmatchedFileSvc := service.NewUnmatchedFileService(unmatchedFileRepo, itemRepo, mediaFileRepo)
	unmatchedFileHandler := apiconnect.NewUnmatchedFileHandler(unmatchedFileSvc, logger)
	unmatchedFilePath, unmatchedFileConnectHandler := pipelinev1connect.NewUnmatchedFileServiceHandler(unmatchedFileHandler, interceptors)
	mux.Handle(unmatchedFilePath, unmatchedFileConnectHandler)

	watcherCloser, err := startScanWatcher(ctx, pipelineCfg.ScanRoots, scanSvc, logger)
	if err != nil {
		return nil, err
	}
	return watcherCloser, nil
}

// startScanWatcher starts a live pkg/fswatch.Watcher over roots' paths and
// a ScanWatchConsumer goroutine driving scanSvc.Trigger from its settled
// events — the automatic half of docs/adr/0024-pipeline-core.md's
// "Discovery: both a watcher and an on-demand recursive scan, one code
// path." Returns a nil io.Closer and no error when roots is empty: no
// watched paths means no cost to opt out, the same convention
// docs/adr/0007-telemetry.md established for telemetry. ctx bounds the
// watcher's and the consumer's lifetime; the returned watcher should still
// be Close()d during shutdown for a clean fsnotify handle teardown.
func startScanWatcher(ctx context.Context, roots []config.ScanRoot, scanSvc *service.ScanService, logger *slog.Logger) (io.Closer, error) {
	if len(roots) == 0 {
		return nil, nil //nolint:nilnil // deliberate: no configured roots means no watcher, not an error
	}

	paths := make([]string, len(roots))
	for i, r := range roots {
		paths[i] = r.Path
	}

	src, err := fsnotify.New(fsnotify.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing fsnotify source: %w", err)
	}

	watcher, err := fswatch.New(src, paths, fswatch.DefaultConfig(), fswatch.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing filesystem watcher: %w", err)
	}
	if err := watcher.Start(ctx); err != nil {
		return nil, fmt.Errorf("cmd/purser: starting filesystem watcher: %w", err)
	}

	consumer := service.NewScanWatchConsumer(watcher, scanSvc, logger)
	go consumer.Run(ctx)

	return watcher, nil
}
