package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"purser/internal/adapters/acoustid"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/musicbrainz"
	"purser/internal/adapters/musicbrainz/fixtureserver"
	"purser/internal/config"
	"purser/internal/domain"
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
	imagestorelocal "purser/internal/adapters/imagestore/local"
	adapterjobqueue "purser/internal/adapters/jobqueue"

	adapterpipeline "purser/internal/adapters/pipeline"
	pipelinemusic "purser/internal/adapters/pipeline/music"
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

	checkMediaToolchain(logger)

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

	mux, watcherCloser, err := newServeMux(sigCtx, logger, ds, cfg.Pipeline, cfg.MusicBrainz, cfg.AcoustID, cfg.Media)
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

// requiredMediaBinaries are the external binaries the media/pipeline
// toolchain shells out to — checkMediaToolchain verifies each is resolvable
// on PATH at startup: ffprobe (Music's FileFingerprinter, M4) and fpcalc
// (Chromaprint, AcoustID's Fingerprint, M5) — see
// docs/technical/pipeline-music-fingerprinter.md and
// docs/technical/pipeline-music-acoustid-adapter.md.
var requiredMediaBinaries = []string{"ffprobe", "fpcalc"}

// checkMediaToolchain verifies every binary in requiredMediaBinaries
// resolves on PATH and logs an error for each one that doesn't. This is
// deliberately non-fatal: a missing binary only degrades the specific
// pipeline capability that shells out to it (e.g. Music fingerprinting
// falls back to producing no tag/duration signal for every file, the same
// "no cost, no crash" treatment an unregistered content type already gets)
// — it never prevents the rest of the server, which has no dependency on
// it, from starting.
func checkMediaToolchain(logger *slog.Logger) {
	checkRequiredBinaries(logger, requiredMediaBinaries)
}

// checkRequiredBinaries is checkMediaToolchain's logic, taking the binary
// list as a parameter so it's testable without mutating the package-level
// requiredMediaBinaries.
func checkRequiredBinaries(logger *slog.Logger, binaries []string) {
	for _, name := range binaries {
		if _, err := exec.LookPath(name); err != nil {
			logger.Error("required media toolchain binary not found on PATH",
				"binary", name,
				"impact", "pipeline capabilities depending on this binary will fail until it is installed",
			)
		}
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
func newServeMux(ctx context.Context, logger *slog.Logger, ds datastore.Datastore, pipelineCfg config.Pipeline, mbCfg config.MusicBrainz, acoustIDCfg config.AcoustID, mediaCfg config.Media) (*http.ServeMux, io.Closer, error) {
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
	watcherCloser, err := wireScanPipeline(ctx, mux, ds, logger, interceptors, jobEngine, jobAdapter, itemRepo, mediaFileRepo, libraryEntryRepo, groupRepo, externalIDRepo, musicReleaseRepo, imageRepo, pipelineCfg, mbCfg, acoustIDCfg, mediaCfg)
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
		pipelinev1connect.OrganizerServiceName,
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
// neither service imports pkg/jobqueue directly. itemRepo/mediaFileRepo/
// libraryEntryRepo/groupRepo/externalIDRepo/musicReleaseRepo are the
// already-constructed repositories built alongside their own entity
// services above — ScanExecutor needs mediaFileRepo for the "already
// known" short-circuit's MediaFile-side lookup, UnmatchedFileService needs
// itemRepo/mediaFileRepo for Resolve's match outcome (docs/adr/0015's
// composing-service exception, see internal/service/unmatched_file.go),
// and the Music Persister needs the rest for its Artist/Release Group/
// MusicRelease/Item/MediaFile cascade (docs/technical/pipeline-music-persist.md).
// See docs/adr/0023-job-queue.md, docs/adr/0024-pipeline-core.md. The
// returned io.Closer stops the filesystem watcher started for
// pipelineCfg.ScanRoots (see startScanWatcher); it is nil when no roots
// are configured.
func wireScanPipeline(
	ctx context.Context,
	mux *http.ServeMux,
	ds datastore.Datastore,
	logger *slog.Logger,
	interceptors connect.HandlerOption,
	jobEngine *pkgjobqueue.Engine,
	jobAdapter *adapterjobqueue.Adapter,
	itemRepo ports.ItemRepository,
	mediaFileRepo ports.MediaFileRepository,
	libraryEntryRepo ports.LibraryEntryRepository,
	groupRepo ports.GroupRepository,
	externalIDRepo ports.ExternalIDRepository,
	musicReleaseRepo ports.MusicReleaseRepository,
	imageRepo ports.ImageRepository,
	pipelineCfg config.Pipeline,
	mbCfg config.MusicBrainz,
	acoustIDCfg config.AcoustID,
	mediaCfg config.Media,
) (io.Closer, error) {
	unmatchedFileRepo, err := storeunmatchedfile.New("unmatched_file", ds, storeunmatchedfile.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing unmatched file repository: %w", err)
	}

	// Content types with no registered ports.Grouping implementation fall
	// back to service.IdentityGrouping.
	groupingRegistry := service.NewGroupingRegistry(pipelinemusic.Grouping{})
	// Content types with no registered ports.SidecarClassifier
	// implementation fall back to service.NoopClassifier.
	sidecarClassifierRegistry := service.NewSidecarClassifierRegistry(pipelinemusic.SidecarClassifier{})
	// Content types with no registered ports.FileFingerprinter
	// implementation fall back to service.NoopFingerprinter.
	fingerprinterRegistry := service.NewFileFingerprinterRegistry(pipelinemusic.New(pipelinemusic.WithLogger(logger)))

	mbClient, acoustIDClient, err := newMusicIdentificationClients(mbCfg, acoustIDCfg, logger)
	if err != nil {
		return nil, err
	}
	// Content types with no registered ports.Identifier/ports.ConfidenceScorer
	// implementation fall back to service.NoopIdentifier/NoopConfidenceScore.
	identifierRegistry := service.NewIdentifierRegistry(pipelinemusic.NewIdentifier(mbClient, acoustIDClient, pipelinemusic.FilenameParser{}, pipelinemusic.WithLogger(logger)))
	confidenceScoreRegistry := service.NewConfidenceScoreRegistry(pipelinemusic.NewConfidenceScorer(pipelinemusic.WithLogger(logger)))

	// The local imagestore adapter — see docs/adr/0013-image-blob-storage.md.
	// The Music Persister's cover-art step (M10b) is its first real caller;
	// no other code path writes image bytes yet.
	imageStore, err := imagestorelocal.New("image", mediaCfg.Path, imagestorelocal.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing image store: %w", err)
	}

	// Content types with no registered ports.TemplateDataBuilder
	// implementation fall back to service.NoopTemplateDataBuilder.
	musicTemplateDataBuilder := pipelinemusic.NewTemplateDataBuilder(groupRepo, musicReleaseRepo, libraryEntryRepo, pipelinemusic.WithLogger(logger))
	templateDataRegistry := service.NewTemplateDataBuilderRegistry(musicTemplateDataBuilder)

	organizeConfigs := make(map[domain.ContentType]service.OrganizeConfig, len(pipelineCfg.Organize))
	for ct, oc := range pipelineCfg.Organize {
		organizeConfigs[ct] = service.OrganizeConfig{Root: oc.Root, Template: oc.Template}
	}
	// organizerSvc is always constructed — needed for the manual
	// OrganizerService RPC regardless of AutoOrganize's setting — but only
	// wired into musicPersister's auto-trigger below when AutoOrganize is
	// on; otherwise musicPersister gets noopOrganizer{}. See
	// docs/technical/pipeline-music-organizer.md.
	organizerSvc := service.NewOrganizer(mediaFileRepo, itemRepo, templateDataRegistry, organizeConfigs, logger)

	var musicAutoOrganizer ports.Organizer = noopOrganizer{}
	if pipelineCfg.AutoOrganize {
		musicAutoOrganizer = organizerSvc
	}

	musicPersister := pipelinemusic.NewPersister(mbClient, externalIDRepo, libraryEntryRepo, groupRepo, musicReleaseRepo, itemRepo, mediaFileRepo, imageRepo, imageStore, musicAutoOrganizer, pipelinemusic.WithLogger(logger))
	persisterRegistry := service.NewPersisterRegistry(musicPersister)
	decisionSvc := service.NewDecisionService(pipelineCfg.ConfidenceThreshold, persisterRegistry)

	jobEngine.Register("scan", adapterpipeline.NewScanExecutor(unmatchedFileRepo, mediaFileRepo, groupingRegistry, fingerprinterRegistry, identifierRegistry, confidenceScoreRegistry, decisionSvc))

	fileWalker, err := filewalkerlocal.New(filewalkerlocal.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("cmd/purser: constructing file walker: %w", err)
	}

	roots := make([]service.RootContentType, len(pipelineCfg.ScanRoots))
	for i, r := range pipelineCfg.ScanRoots {
		roots[i] = service.RootContentType{Path: r.Path, ContentType: r.ContentType}
	}

	scanSvc := service.NewScanService(jobAdapter, fileWalker, pipelineCfg.EnableMD5, pipelineCfg.EnableSHA512, roots, sidecarClassifierRegistry)
	scanHandler := apiconnect.NewScanHandler(scanSvc, logger)
	scanPath, scanConnectHandler := pipelinev1connect.NewScanServiceHandler(scanHandler, interceptors)
	mux.Handle(scanPath, scanConnectHandler)

	unmatchedFileSvc := service.NewUnmatchedFileService(unmatchedFileRepo, itemRepo, mediaFileRepo, persisterRegistry)
	unmatchedFileHandler := apiconnect.NewUnmatchedFileHandler(unmatchedFileSvc, logger)
	unmatchedFilePath, unmatchedFileConnectHandler := pipelinev1connect.NewUnmatchedFileServiceHandler(unmatchedFileHandler, interceptors)
	mux.Handle(unmatchedFilePath, unmatchedFileConnectHandler)

	organizerHandler := apiconnect.NewOrganizerHandler(organizerSvc, logger)
	organizerPath, organizerConnectHandler := pipelinev1connect.NewOrganizerServiceHandler(organizerHandler, interceptors)
	mux.Handle(organizerPath, organizerConnectHandler)

	watcherCloser, err := startScanWatcher(ctx, pipelineCfg.ScanRoots, scanSvc, logger)
	if err != nil {
		return nil, err
	}
	return watcherCloser, nil
}

// newMusicIdentificationClients constructs the real MusicBrainz client
// (always — mbCfg.BaseURL has a safe adapter-level default per
// musicbrainz.DefaultConfig) and the AcoustID client only when an API key
// is configured (acoustIDCfg.APIKey has no safe default — acoustid.New
// errors on an empty one). AcoustID is optional corroboration, gated on
// tag-derived signals not already resolving a group
// (docs/adr/0025-music-identification-confidence-scoring.md), so an
// unconfigured deployment gets noopAcoustIDClient instead of failing
// startup.
//
// When PURSER_MUSICBRAINZ_MOCK is set (any non-empty value), the
// MusicBrainz client is built with musicbrainz.WithBaseTransport pointed at
// internal/adapters/musicbrainz/fixtureserver's canned route table instead
// of a real transport — no real socket, not even loopback, ever opens.
// This is CI-only wiring (k6's flow suite, per Makefile's k6-ci target),
// never something an operator sets in a real deployment config, which is
// why it's a bare env var read here rather than a config.MusicBrainz field.
func newMusicIdentificationClients(mbCfg config.MusicBrainz, acoustIDCfg config.AcoustID, logger *slog.Logger) (ports.MusicBrainzClient, ports.AcoustIDClient, error) {
	mbAdapterCfg := musicbrainz.DefaultConfig()
	if mbCfg.BaseURL != "" {
		mbAdapterCfg.BaseURL = mbCfg.BaseURL
	}
	if mbCfg.ResponseHeaderTimeout > 0 {
		mbAdapterCfg.HTTPClient.ResponseHeaderTimeout = mbCfg.ResponseHeaderTimeout
	}
	mbOpts := []musicbrainz.Option{musicbrainz.WithLogger(logger)}
	if os.Getenv("PURSER_MUSICBRAINZ_MOCK") != "" {
		logger.Warn("PURSER_MUSICBRAINZ_MOCK is set: MusicBrainz calls are answered from fixtureserver's canned data, never a live network call")
		mbOpts = append(mbOpts, musicbrainz.WithBaseTransport(fixtureserver.Transport()))
	}
	mbClient, err := musicbrainz.New(mbAdapterCfg, mbOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing musicbrainz client: %w", err)
	}

	if acoustIDCfg.APIKey == "" {
		return mbClient, noopAcoustIDClient{}, nil
	}
	acoustIDAdapterCfg := acoustid.DefaultConfig()
	acoustIDAdapterCfg.APIKey = acoustIDCfg.APIKey
	if acoustIDCfg.BaseURL != "" {
		acoustIDAdapterCfg.BaseURL = acoustIDCfg.BaseURL
	}
	acoustIDClient, err := acoustid.New(acoustIDAdapterCfg, acoustid.WithLogger(logger))
	if err != nil {
		return nil, nil, fmt.Errorf("cmd/purser: constructing acoustid client: %w", err)
	}
	return mbClient, acoustIDClient, nil
}

// noopAcoustIDClient is the ports.AcoustIDClient used when no AcoustID API
// key is configured — Fingerprint always errors, which
// music.Identifier.collectAcoustID already treats as non-fatal (logs and
// skips the file, per-file, never per-group), so Lookup is never reached.
type noopAcoustIDClient struct{}

func (noopAcoustIDClient) Fingerprint(context.Context, string) (string, float64, error) {
	return "", 0, errors.New("acoustid: no api key configured")
}

func (noopAcoustIDClient) Lookup(context.Context, string, float64) ([]ports.AcoustIDMatch, error) {
	return nil, ports.ErrNotFound
}

// noopOrganizer is the ports.Organizer a content type's Persister is wired
// with when config.Pipeline.AutoOrganize is off — Organize is a no-op,
// never an error, so a Persister that always calls it unconditionally
// never has anything to log. Auto-organize is decided once, here at the
// composition root, never as a nil-check inside a Persister — see
// pipelinemusic.NewPersister's own doc comment.
type noopOrganizer struct{}

func (noopOrganizer) Organize(context.Context, string) (*domain.MediaFile, error) {
	return nil, nil //nolint:nilnil // deliberate: AutoOrganize off means nothing to do, not an error
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
