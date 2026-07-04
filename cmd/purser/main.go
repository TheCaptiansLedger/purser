package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	badgeradapter "purser/internal/adapters/badger"
	"purser/internal/adapters/db"
	"purser/internal/adapters/fanart"
	"purser/internal/adapters/fingerprint"
	fsadapter "purser/internal/adapters/fs"
	githubadapter "purser/internal/adapters/github"
	"purser/internal/adapters/identifier"
	jobsadapter "purser/internal/adapters/jobs"
	"purser/internal/adapters/mbz"
	"purser/internal/adapters/notify"
	"purser/internal/adapters/stashdb"
	"purser/internal/adapters/theaudiodb"
	"purser/internal/api"
	appconfig "purser/internal/app/config"
	"purser/internal/app/library"
	"purser/internal/app/metadata"
	"purser/internal/app/people"
	appscan "purser/internal/app/scan"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/version"
	"purser/pkg/cache"
	"purser/web"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	var cfgPath string

	root := &cobra.Command{
		Use:          "purser",
		Short:        "Self-hosted media metadata manager",
		Version:      version.Version,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(cfgPath)
		},
	}
	root.SetVersionTemplate("purser {{.Version}}\n")
	root.PersistentFlags().StringVar(&cfgPath, "config", "purser.yaml", "path to config file")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cfgPath string) error {
	cfg, v, locked, err := config.LoadFull(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	slog.SetDefault(newLogger(cfg.Log))
	slog.Info("purser starting", "port", cfg.Server.Port, "db_driver", cfg.Database.Driver)

	if err := fsadapter.MigrateFlat(cfg.Media.Path); err != nil {
		return fmt.Errorf("migrate media: %w", err)
	}
	if err := fsadapter.EnsureDirs(cfg.Media.Path); err != nil {
		return fmt.Errorf("ensure media dirs: %w", err)
	}

	entryRepo, groupRepo, itemRepo, personRepo, tagRepo, extIDRepo, settingsRepo, storageAdmin, mediaFileRepo, unmatchedRepo, closeStorage, err := openStorage(cfg)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer closeStorage()

	cfgSvc := appconfig.New(v, locked, settingsRepo)

	jobQueue := jobsadapter.New(cfg.Server.Workers)
	defer jobQueue.Close()

	githubCache, err := cache.New("github", 256)
	if err != nil {
		return fmt.Errorf("create github cache: %w", err)
	}
	audiodbCache, err := cache.New("audiodb", 1024)
	if err != nil {
		return fmt.Errorf("create audiodb cache: %w", err)
	}
	mbzCache, err := cache.New("mbz", 512)
	if err != nil {
		return fmt.Errorf("create mbz cache: %w", err)
	}
	fanartCache, err := cache.New("fanart", 256)
	if err != nil {
		return fmt.Errorf("create fanart cache: %w", err)
	}
	stashdbCache, err := cache.New("stashdb", 256)
	if err != nil {
		return fmt.Errorf("create stashdb cache: %w", err)
	}

	libSvc := library.New(entryRepo, groupRepo, itemRepo, personRepo, tagRepo)
	peopleSvc := people.New(personRepo)
	sources := buildSources(cfg, audiodbCache, mbzCache, fanartCache, stashdbCache)
	imgDownloader := fsadapter.NewImageDownloader(cfg.Media.Path)

	osFS := fsadapter.NewFileSystem()
	scanner := fsadapter.NewScanner(osFS, mediaFileRepo)
	watcher := fsadapter.NewWatcher(2 * time.Second)
	videoFP := fingerprint.NewVideoFingerprinter(osFS)
	musicFP := fingerprint.NewMusicFingerprinter()
	bookFP := fingerprint.NewBookFingerprinter()
	adultID := identifier.NewAdultIdentifier(mediaFileRepo, itemRepo, sources)
	musicID := identifier.NewMusicIdentifier(extIDRepo, itemRepo, sources, cfg.Sources.AcoustID.APIKey)
	videoID := identifier.NewVideoIdentifier(mediaFileRepo, itemRepo, sources)
	bookID := identifier.NewBookIdentifier(extIDRepo, itemRepo, sources)
	noopNotifier := &notify.NoopDispatcher{}
	thumbnailCache := fsadapter.NewThumbnailCache(cfg.Media.Path)
	scanSvc := appscan.New(
		scanner, watcher,
		[]ports.FileFingerprinter{videoFP, musicFP, bookFP},
		[]ports.FileIdentifier{adultID, musicID, videoID, bookID},
		itemRepo, mediaFileRepo, unmatchedRepo, noopNotifier, 0.85,
		jobQueue, entryRepo, groupRepo,
		thumbnailCache, buildUpgradeMode(cfg),
	)
	metaSvc := metadata.New(sources, jobQueue, entryRepo, groupRepo, itemRepo, personRepo, tagRepo, extIDRepo, imgDownloader)
	ghAdapter := githubadapter.New(githubadapter.Config{
		Repo:  cfg.GitHub.Repo,
		Token: cfg.GitHub.Token,
	}, githubCache)

	uiFS, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return fmt.Errorf("load embedded UI: %w", err)
	}

	signalCtx, signalStop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer signalStop()
	lifecycleCtx, shutdown := context.WithCancel(context.Background())
	go func() {
		<-signalCtx.Done()
		shutdown()
	}()

	srv := api.New(cfg.Server.Port, cfg.Media.Path, cfg, storageAdmin, libSvc, peopleSvc, metaSvc, scanSvc, tagRepo, jobQueue, cfgSvc, sources, uiFS, imgDownloader, ghAdapter, []*cache.Cache{githubCache, audiodbCache, mbzCache, fanartCache, stashdbCache}, shutdown)

	go func() { _ = scanSvc.StartWatching(lifecycleCtx, modulesFromConfig(cfg)) }()

	go func() {
		slog.Info("listening", "port", cfg.Server.Port)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			shutdown()
		}
	}()

	<-lifecycleCtx.Done()
	slog.Info("shutting down")
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer drainCancel()
	if err := srv.Shutdown(drainCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	return nil
}

func openStorage(cfg *config.Config) (
	ports.LibraryEntryRepository,
	ports.GroupRepository,
	ports.ItemRepository,
	ports.PersonRepository,
	ports.TagRepository,
	ports.ExternalIDRepository,
	ports.SettingsRepository,
	ports.StorageAdminPort,
	ports.MediaFileRepository,
	ports.UnmatchedFileRepository,
	func(),
	error,
) {
	switch cfg.Database.Driver {
	case "badger":
		bdb, err := badgeradapter.Open(cfg.Database.Badger)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("open badger: %w", err)
		}
		return badgeradapter.NewLibraryEntryRepo(bdb),
			badgeradapter.NewGroupRepo(bdb),
			badgeradapter.NewItemRepo(bdb),
			badgeradapter.NewPersonRepo(bdb),
			badgeradapter.NewTagRepo(bdb),
			badgeradapter.NewExternalIDRepo(bdb),
			badgeradapter.NewSettingsRepo(bdb),
			badgeradapter.NewStorageAdmin(bdb, cfg.Database.Badger.DataDir),
			badgeradapter.NewMediaFileRepo(bdb),
			badgeradapter.NewUnmatchedFileRepo(bdb),
			func() { _ = bdb.Close() },
			nil
	default: // "sqlite"
		sqldb, err := db.Open(cfg.Database.DSN)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("open sqlite: %w", err)
		}
		return db.NewLibraryEntryRepo(sqldb),
			db.NewGroupRepo(sqldb),
			db.NewItemRepo(sqldb),
			db.NewPersonRepo(sqldb),
			db.NewTagRepo(sqldb),
			db.NewExternalIDRepo(sqldb),
			db.NewSettingsRepo(sqldb),
			db.NewStorageAdmin(sqldb, cfg.Database.DSN),
			db.NewMediaFileRepo(sqldb),
			db.NewUnmatchedFileRepo(sqldb),
			func() { _ = sqldb.Close() },
			nil
	}
}

// modulesFromConfig builds per-content-type scan modules from the enabled module configs.
func modulesFromConfig(cfg *config.Config) []appscan.Module {
	type entry struct {
		mc config.ModuleConfig
		ct domain.ContentType
	}
	all := []entry{
		{cfg.Modules.Movies, domain.ContentTypeMovie},
		{cfg.Modules.TV, domain.ContentTypeTV},
		{cfg.Modules.Music, domain.ContentTypeMusic},
		{cfg.Modules.Books, domain.ContentTypeBook},
		{cfg.Modules.AfterDark, domain.ContentTypeAdult},
		{cfg.Modules.JAV, domain.ContentTypeJAV},
	}
	var modules []appscan.Module
	for _, e := range all {
		if e.mc.Enabled && len(e.mc.Roots) > 0 {
			modules = append(modules, appscan.Module{ContentType: e.ct, Roots: e.mc.Roots})
		}
	}
	return modules
}

// buildUpgradeMode maps each ContentType to its configured upgrade mode.
func buildUpgradeMode(cfg *config.Config) map[domain.ContentType]string {
	type entry struct {
		mc config.ModuleConfig
		ct domain.ContentType
	}
	all := []entry{
		{cfg.Modules.Movies, domain.ContentTypeMovie},
		{cfg.Modules.TV, domain.ContentTypeTV},
		{cfg.Modules.Music, domain.ContentTypeMusic},
		{cfg.Modules.Books, domain.ContentTypeBook},
		{cfg.Modules.AfterDark, domain.ContentTypeAdult},
		{cfg.Modules.JAV, domain.ContentTypeJAV},
	}
	m := make(map[domain.ContentType]string, len(all))
	for _, e := range all {
		if e.mc.UpgradeMode != "" {
			m[e.ct] = e.mc.UpgradeMode
		}
	}
	return m
}

// buildSources constructs and returns all enabled MetadataSource adapters.
func buildSources(cfg *config.Config, audiodbCache, mbzCache, fanartCache, stashdbCache *cache.Cache) []ports.MetadataSource {
	var sources []ports.MetadataSource
	if cfg.Sources.StashDB.Enabled {
		sources = append(sources, stashdb.New(cfg.Sources.StashDB, stashdbCache))
	}
	if cfg.Sources.MusicBrainz.Enabled {
		sources = append(sources, mbz.New(cfg.Sources.MusicBrainz, mbzCache))
	}
	if cfg.Sources.Fanart.Enabled {
		slog.Info("source enabled", "name", "fanart")
		sources = append(sources, fanart.New(cfg.Sources.Fanart, fanartCache))
	}
	if cfg.Sources.TheAudioDB.Enabled {
		slog.Info("source enabled", "name", "audiodb")
		sources = append(sources, theaudiodb.New(cfg.Sources.TheAudioDB, audiodbCache))
	}
	return sources
}

func newLogger(cfg config.LogConfig) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.Format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
