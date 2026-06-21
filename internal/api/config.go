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
	Sources  sourcesCfgResponse  `json:"sources"`
	Log      logCfgResponse      `json:"log"`
}

type serverCfgResponse struct {
	Port    int `json:"port"`
	Workers int `json:"workers"`
}

type sourcesCfgResponse struct {
	StashDB     sourceCfgResponse `json:"stashdb"`
	TPDB        sourceCfgResponse `json:"tpdb"`
	Stash       sourceCfgResponse `json:"stash"`
	TMDB        sourceCfgResponse `json:"tmdb"`
	TVDB        sourceCfgResponse `json:"tvdb"`
	MusicBrainz sourceCfgResponse `json:"musicbrainz"`
	Fanart      sourceCfgResponse `json:"fanart"`
	LastFM      sourceCfgResponse `json:"lastfm"`
	TheAudioDB  sourceCfgResponse `json:"theaudiodb"`
	OpenLibrary sourceCfgResponse `json:"openlibrary"`
}

type sourceCfgResponse struct {
	Enabled   bool   `json:"enabled"`
	URL       string `json:"url"`
	APIKey    string `json:"api_key"`
	UserAgent string `json:"user_agent"`
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
		Server:   serverCfgResponse{Port: c.Server.Port, Workers: c.Server.Workers},
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
		Sources: sourcesCfgResponse{
			StashDB:     maskSource(c.Sources.StashDB),
			TPDB:        maskSource(c.Sources.TPDB),
			Stash:       maskSource(c.Sources.Stash),
			TMDB:        maskSource(c.Sources.TMDB),
			TVDB:        maskSource(c.Sources.TVDB),
			MusicBrainz: maskSource(c.Sources.MusicBrainz),
			Fanart:      maskSource(c.Sources.Fanart),
			LastFM:      maskSource(c.Sources.LastFM),
			TheAudioDB:  maskSource(c.Sources.TheAudioDB),
			OpenLibrary: maskSource(c.Sources.OpenLibrary),
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

func maskSource(s config.MetadataSourceConfig) sourceCfgResponse {
	return sourceCfgResponse{
		Enabled:   s.Enabled,
		URL:       s.URL,
		APIKey:    maskSecret(s.APIKey),
		UserAgent: s.UserAgent,
	}
}

func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	return "***"
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
