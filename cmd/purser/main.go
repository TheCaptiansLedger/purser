package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"purser/internal/adapters/db"
	"purser/internal/adapters/fanart"
	"purser/internal/adapters/mbz"
	"purser/internal/adapters/stashdb"
	"purser/internal/api"
	"purser/internal/app/library"
	"purser/internal/app/metadata"
	"purser/internal/app/people"
	"purser/internal/config"
	"purser/internal/ports"
	"purser/internal/version"
	"purser/web"
	"syscall"

	fsadapter "purser/internal/adapters/fs"

	"github.com/spf13/cobra"

	jobsadapter "purser/internal/adapters/jobs"
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
	cfg, err := config.Load(cfgPath)
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

	database, err := db.Open(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	entryRepo := db.NewLibraryEntryRepo(database)
	groupRepo := db.NewGroupRepo(database)
	itemRepo := db.NewItemRepo(database)
	personRepo := db.NewPersonRepo(database)
	tagRepo := db.NewTagRepo(database)
	extIDRepo := db.NewExternalIDRepo(database)

	jobQueue := jobsadapter.New(cfg.Server.Workers)
	defer jobQueue.Close()

	libSvc := library.New(entryRepo, groupRepo, itemRepo, personRepo)
	peopleSvc := people.New(personRepo)
	metaSvc := metadata.New(buildSources(cfg), jobQueue, entryRepo, groupRepo, itemRepo, personRepo, tagRepo, extIDRepo, fsadapter.NewImageDownloader(cfg.Media.Path))

	uiFS, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return fmt.Errorf("load embedded UI: %w", err)
	}

	srv := api.New(cfg.Server.Port, cfg.Media.Path, cfg, database, libSvc, peopleSvc, metaSvc, tagRepo, jobQueue, uiFS)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		slog.Info("listening", "port", cfg.Server.Port)
		if err := srv.Start(); err != nil {
			slog.Error("server stopped", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	return nil
}

// buildSources constructs and returns all enabled MetadataSource adapters.
func buildSources(cfg *config.Config) []ports.MetadataSource {
	var sources []ports.MetadataSource
	if cfg.Sources.StashDB.Enabled {
		sources = append(sources, stashdb.New(cfg.Sources.StashDB))
	}
	if cfg.Sources.MusicBrainz.Enabled {
		sources = append(sources, mbz.New(cfg.Sources.MusicBrainz))
	}
	if cfg.Sources.Fanart.Enabled {
		slog.Info("source enabled", "name", "fanart")
		sources = append(sources, fanart.New(cfg.Sources.Fanart))
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
