package api

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/http"
	"purser/internal/app/library"
	"purser/internal/app/metadata"
	"purser/internal/app/people"
	"purser/internal/config"
	"purser/internal/ports"
	"purser/pkg/cache"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server is the HTTP server. It owns the Chi router and all handler registration.
type Server struct {
	httpServer *http.Server
	router     *chi.Mux
}

// New wires up all routes and returns a ready-to-start Server.
func New(
	port int,
	mediaPath string,
	cfg *config.Config,
	db *sql.DB,
	libSvc *library.Service,
	peopleSvc *people.Service,
	metaSvc *metadata.Service,
	tagRepo ports.TagRepository,
	jobQueue ports.JobQueue,
	cfgSvc ports.ConfigService,
	sources []ports.MetadataSource,
	uiFS fs.FS,
	imgDownloader ports.ImageDownloader,
	gh ports.GitHubProxy,
	caches []*cache.Cache,
	shutdownFn func(),
) *Server {
	s := &Server{
		router: chi.NewRouter(),
	}
	s.mount(mediaPath, cfg, db, libSvc, peopleSvc, metaSvc, tagRepo, jobQueue, cfgSvc, sources, uiFS, imgDownloader, gh, caches, shutdownFn)
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return s
}

func (s *Server) mount(
	mediaPath string,
	cfg *config.Config,
	db *sql.DB,
	libSvc *library.Service,
	peopleSvc *people.Service,
	metaSvc *metadata.Service,
	tagRepo ports.TagRepository,
	jobQueue ports.JobQueue,
	cfgSvc ports.ConfigService,
	sources []ports.MetadataSource,
	uiFS fs.FS,
	imgDownloader ports.ImageDownloader,
	gh ports.GitHubProxy,
	caches []*cache.Cache,
	shutdownFn func(),
) {
	r := s.router

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger)

	r.Route("/api/v1", func(r chi.Router) {
		cfgH := &configHandler{cfg: cfg, cfgSvc: cfgSvc}
		r.Get("/config", cfgH.get)
		r.Patch("/config", cfgH.patch)
		r.Get("/config/content-types", cfgH.contentTypes)
		r.Get("/config/kinds", cfgH.kinds)

		providerImagesH := &providerImagesHandler{svc: metaSvc}

		imgSetH := newEntityImageSetHandler(libSvc, peopleSvc, mediaPath, imgDownloader)

		entryH := &libraryEntryHandler{svc: libSvc}
		r.Route("/library-entries", func(r chi.Router) {
			entryH.routes(r)
			r.Get("/{id}/provider-images", providerImagesH.forEntry)
			r.Post("/{id}/image", imgSetH.setEntryImage)
			r.Delete("/{id}/image", imgSetH.clearEntryImage)
			r.Post("/{id}/banner", imgSetH.setEntryBanner)
			r.Delete("/{id}/banner", imgSetH.clearEntryBanner)
		})

		groupH := &groupHandler{svc: libSvc}
		r.Route("/groups", func(r chi.Router) {
			groupH.routes(r)
			r.Get("/{id}/provider-images", providerImagesH.forGroup)
			r.Post("/{id}/image", imgSetH.setGroupImage)
			r.Delete("/{id}/image", imgSetH.clearGroupImage)
		})

		itemH := &itemHandler{svc: libSvc}
		r.Route("/items", func(r chi.Router) {
			itemH.routes(r)
			r.Get("/{id}/provider-images", providerImagesH.forItem)
			r.Post("/{id}/image", imgSetH.setItemImage)
			r.Delete("/{id}/image", imgSetH.clearItemImage)
		})

		peopleH := &peopleHandler{svc: peopleSvc}
		r.Route("/people", func(r chi.Router) {
			peopleH.routes(r)
			r.Get("/{id}/provider-images", providerImagesH.forPerson)
			r.Post("/{id}/image", imgSetH.setPersonImage)
			r.Delete("/{id}/image", imgSetH.clearPersonImage)
		})

		tagH := &tagHandler{repo: tagRepo}
		r.Route("/tags", tagH.routes)

		imgH := &imageHandler{basePath: mediaPath}
		r.Route("/images", imgH.routes)

		dbH := &databaseHandler{db: db, dsn: cfg.Database.DSN, shutdownFn: shutdownFn}
		r.Route("/database", dbH.routes)

		metaH := &metadataHandler{svc: metaSvc}
		r.Route("/metadata", metaH.routes)

		jobH := &jobHandler{queue: jobQueue}
		r.Route("/jobs", jobH.routes)

		cmdH := &commandsHandler{metaSvc: metaSvc}
		r.Route("/commands", cmdH.routes)

		setupH := &setupHandler{config: cfgSvc}
		r.Route("/setup", func(r chi.Router) {
			r.Get("/status", setupH.status)
			r.Post("/complete", setupH.complete)
		})

		verifyH := &verifyHandler{sources: sources}
		r.Route("/verify", func(r chi.Router) {
			r.Post("/source", verifyH.source)
		})

		roadmapH := &roadmapHandler{gh: gh}
		r.Route("/roadmap", roadmapH.routes)

		cacheH := &cacheHandler{caches: caches}
		r.Get("/cache/stats", cacheH.stats)
	})

	// Serve the embedded web UI for all non-API paths.
	// Unknown paths fall back to index.html to support SPA client-side routing.
	fileServer := http.FileServer(http.FS(uiFS))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		_, err := fs.Stat(uiFS, r.URL.Path[1:])
		if err != nil {
			// Path not found in dist — serve index.html for SPA routing.
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// Shutdown gracefully drains in-flight requests before stopping the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Start begins listening on the configured port. It blocks until the server exits.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Handler returns the underlying http.Handler, useful for testing.
func (s *Server) Handler() http.Handler {
	return s.router
}
