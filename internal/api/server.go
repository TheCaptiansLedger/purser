package api

import (
	"database/sql"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"purser/internal/app/library"
	"purser/internal/app/metadata"
	"purser/internal/app/people"
	"purser/internal/config"
	"purser/internal/ports"
)

// Server is the HTTP server. It owns the Chi router and all handler registration.
type Server struct {
	router *chi.Mux
	port   int
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
	uiFS fs.FS,
) *Server {
	s := &Server{
		router: chi.NewRouter(),
		port:   port,
	}
	s.mount(mediaPath, cfg, db, libSvc, peopleSvc, metaSvc, tagRepo, uiFS)
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
	uiFS fs.FS,
) {
	r := s.router

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger)

	r.Route("/api/v1", func(r chi.Router) {
		cfgH := &configHandler{cfg: cfg}
		r.Get("/config", cfgH.get)

		entryH := &libraryEntryHandler{svc: libSvc}
		r.Route("/library-entries", entryH.routes)

		groupH := &groupHandler{svc: libSvc}
		r.Route("/groups", groupH.routes)

		itemH := &itemHandler{svc: libSvc}
		r.Route("/items", itemH.routes)

		peopleH := &peopleHandler{svc: peopleSvc}
		r.Route("/people", peopleH.routes)

		tagH := &tagHandler{repo: tagRepo}
		r.Route("/tags", tagH.routes)

		imgH := &imageHandler{basePath: mediaPath}
		r.Route("/images", imgH.routes)

		dbH := &databaseHandler{db: db, dsn: cfg.Database.DSN}
		r.Route("/database", dbH.routes)

		metaH := &metadataHandler{svc: metaSvc}
		r.Route("/metadata", metaH.routes)
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

// Start begins listening on the configured port. It blocks until the server exits.
func (s *Server) Start() error {
	return http.ListenAndServe(fmt.Sprintf(":%d", s.port), s.router)
}

// Handler returns the underlying http.Handler, useful for testing.
func (s *Server) Handler() http.Handler {
	return s.router
}
