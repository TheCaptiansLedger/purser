package api

import (
	"net/http"
	"net/url"
	"purser/internal/config"
)

type configHandler struct {
	cfg *config.Config
}

// Response types mirror the YAML config structure exactly.

type configResponse struct {
	Server   serverCfgResponse   `json:"server"`
	Database databaseCfgResponse `json:"database"`
	Media    mediaCfgResponse    `json:"media"`
	Modules  modulesCfgResponse  `json:"modules"`
	Log      logCfgResponse      `json:"log"`
}

type serverCfgResponse struct {
	Port int `json:"port"`
}

type databaseCfgResponse struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

type mediaCfgResponse struct {
	Path string `json:"path"`
}

type modulesCfgResponse struct {
	Movies    moduleCfgResponse `json:"movies"`
	TV        moduleCfgResponse `json:"tv"`
	Music     moduleCfgResponse `json:"music"`
	Books     moduleCfgResponse `json:"books"`
	AfterDark moduleCfgResponse `json:"afterdark"`
	JAV       moduleCfgResponse `json:"jav"`
}

type moduleCfgResponse struct {
	Enabled bool     `json:"enabled"`
	Roots   []string `json:"roots"`
}

type logCfgResponse struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

func (h *configHandler) get(w http.ResponseWriter, _ *http.Request) {
	c := h.cfg
	writeJSON(w, http.StatusOK, configResponse{
		Server:   serverCfgResponse{Port: c.Server.Port},
		Database: databaseCfgResponse{Driver: c.Database.Driver, DSN: maskDSN(c.Database.DSN)},
		Media:    mediaCfgResponse{Path: c.Media.Path},
		Modules: modulesCfgResponse{
			Movies:    moduleCfgResponse{Enabled: c.Modules.Movies.Enabled, Roots: nullableRoots(c.Modules.Movies.Roots)},
			TV:        moduleCfgResponse{Enabled: c.Modules.TV.Enabled, Roots: nullableRoots(c.Modules.TV.Roots)},
			Music:     moduleCfgResponse{Enabled: c.Modules.Music.Enabled, Roots: nullableRoots(c.Modules.Music.Roots)},
			Books:     moduleCfgResponse{Enabled: c.Modules.Books.Enabled, Roots: nullableRoots(c.Modules.Books.Roots)},
			AfterDark: moduleCfgResponse{Enabled: c.Modules.AfterDark.Enabled, Roots: nullableRoots(c.Modules.AfterDark.Roots)},
			JAV:       moduleCfgResponse{Enabled: c.Modules.JAV.Enabled, Roots: nullableRoots(c.Modules.JAV.Roots)},
		},
		Log: logCfgResponse{Level: c.Log.Level, Format: c.Log.Format},
	})
}

// nullableRoots returns an empty slice instead of nil so the JSON field is always [].
func nullableRoots(roots []string) []string {
	if roots == nil {
		return []string{}
	}
	return roots
}

// maskDSN replaces the password in a URL-format DSN with ***.
// File paths and key-value DSNs without passwords are returned unchanged.
func maskDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" || u.User == nil {
		return dsn
	}
	if _, hasPass := u.User.Password(); hasPass {
		u.User = url.UserPassword(u.User.Username(), "***")
		return u.String()
	}
	return dsn
}
