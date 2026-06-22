package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"purser/internal/app/errs"
	"purser/internal/config"
	"purser/internal/ports"
	"strconv"
)

type configHandler struct {
	cfg    *config.Config
	cfgSvc ports.ConfigService
}

// Response types mirror the YAML config structure exactly.

type configResponse struct {
	Server   serverCfgResponse   `json:"server"`
	Database databaseCfgResponse `json:"database"`
	Media    mediaCfgResponse    `json:"media"`
	Modules  modulesCfgResponse  `json:"modules"`
	Sources  sourcesCfgResponse  `json:"sources"`
	Log      logCfgResponse      `json:"log"`
	Locked   map[string]bool     `json:"locked"`
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

type patchConfigRequest struct {
	// Bootstrap-only — presence returns 422.
	Server   json.RawMessage `json:"server"`
	Database json.RawMessage `json:"database"`
	Media    json.RawMessage `json:"media"`
	Log      json.RawMessage `json:"log"`
	// Runtime-configurable.
	Modules *modulesPatchRequest `json:"modules"`
	Sources *sourcesPatchRequest `json:"sources"`
}

type modulesPatchRequest struct {
	Movies    *modulePatchRequest `json:"movies"`
	TV        *modulePatchRequest `json:"tv"`
	Music     *modulePatchRequest `json:"music"`
	Books     *modulePatchRequest `json:"books"`
	AfterDark *modulePatchRequest `json:"afterdark"`
	JAV       *modulePatchRequest `json:"jav"`
}

type modulePatchRequest struct {
	Enabled *bool    `json:"enabled"`
	Roots   []string `json:"roots"`
}

type sourcesPatchRequest struct {
	StashDB     *sourcePatchRequest `json:"stashdb"`
	TPDB        *sourcePatchRequest `json:"tpdb"`
	Stash       *sourcePatchRequest `json:"stash"`
	TMDB        *sourcePatchRequest `json:"tmdb"`
	TVDB        *sourcePatchRequest `json:"tvdb"`
	MusicBrainz *sourcePatchRequest `json:"musicbrainz"`
	Fanart      *sourcePatchRequest `json:"fanart"`
	LastFM      *sourcePatchRequest `json:"lastfm"`
	TheAudioDB  *sourcePatchRequest `json:"theaudiodb"`
	OpenLibrary *sourcePatchRequest `json:"openlibrary"`
}

type sourcePatchRequest struct {
	Enabled   *bool   `json:"enabled"`
	URL       *string `json:"url"`
	APIKey    *string `json:"api_key"`
	UserAgent *string `json:"user_agent"`
}

type configKV struct{ key, value string }

func (h *configHandler) get(w http.ResponseWriter, _ *http.Request) {
	c := h.cfg
	locked := h.cfgSvc.LockedKeys()
	if locked == nil {
		locked = map[string]bool{}
	}
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
		Log:    logCfgResponse{Level: c.Log.Level, Format: c.Log.Format},
		Locked: locked,
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

func (h *configHandler) patch(w http.ResponseWriter, r *http.Request) {
	var req patchConfigRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON")
		return
	}

	if req.Server != nil || req.Database != nil || req.Log != nil || req.Media != nil {
		writeError(w, http.StatusUnprocessableEntity, "BOOTSTRAP_KEY", "key cannot be updated via the API")
		return
	}

	kvs := configPatchKVs(req)

	for _, kv := range kvs {
		if h.cfgSvc.IsLocked(kv.key) {
			writeError(w, http.StatusConflict, "LOCKED", "key is operator-managed")
			return
		}
	}

	for _, kv := range kvs {
		if err := h.cfgSvc.Set(r.Context(), kv.key, kv.value); err != nil {
			if errors.Is(err, errs.ErrLocked) {
				writeError(w, http.StatusConflict, "LOCKED", "key is operator-managed")
				return
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save config")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func configPatchKVs(req patchConfigRequest) []configKV {
	var out []configKV
	if m := req.Modules; m != nil {
		out = append(out, modulePatchKVs("movies", m.Movies)...)
		out = append(out, modulePatchKVs("tv", m.TV)...)
		out = append(out, modulePatchKVs("music", m.Music)...)
		out = append(out, modulePatchKVs("books", m.Books)...)
		out = append(out, modulePatchKVs("afterdark", m.AfterDark)...)
		out = append(out, modulePatchKVs("jav", m.JAV)...)
	}
	if s := req.Sources; s != nil {
		out = append(out, sourcePatchKVs("stashdb", s.StashDB)...)
		out = append(out, sourcePatchKVs("tpdb", s.TPDB)...)
		out = append(out, sourcePatchKVs("stash", s.Stash)...)
		out = append(out, sourcePatchKVs("tmdb", s.TMDB)...)
		out = append(out, sourcePatchKVs("tvdb", s.TVDB)...)
		out = append(out, sourcePatchKVs("musicbrainz", s.MusicBrainz)...)
		out = append(out, sourcePatchKVs("fanart", s.Fanart)...)
		out = append(out, sourcePatchKVs("lastfm", s.LastFM)...)
		out = append(out, sourcePatchKVs("theaudiodb", s.TheAudioDB)...)
		out = append(out, sourcePatchKVs("openlibrary", s.OpenLibrary)...)
	}
	return out
}

func modulePatchKVs(name string, m *modulePatchRequest) []configKV {
	if m == nil {
		return nil
	}
	prefix := "modules." + name
	var out []configKV
	if m.Enabled != nil {
		out = append(out, configKV{prefix + ".enabled", strconv.FormatBool(*m.Enabled)})
	}
	if m.Roots != nil {
		raw, _ := json.Marshal(m.Roots)
		out = append(out, configKV{prefix + ".roots", string(raw)})
	}
	return out
}

func sourcePatchKVs(name string, s *sourcePatchRequest) []configKV {
	if s == nil {
		return nil
	}
	prefix := "sources." + name
	var out []configKV
	if s.Enabled != nil {
		out = append(out, configKV{prefix + ".enabled", strconv.FormatBool(*s.Enabled)})
	}
	if s.URL != nil {
		out = append(out, configKV{prefix + ".url", *s.URL})
	}
	if s.APIKey != nil {
		out = append(out, configKV{prefix + ".api_key", *s.APIKey})
	}
	if s.UserAgent != nil {
		out = append(out, configKV{prefix + ".user_agent", *s.UserAgent})
	}
	return out
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
