package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"purser/internal/app/errs"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"reflect"
	"strconv"
)

type configHandler struct {
	cfg    *config.Config
	cfgSvc ports.ConfigService
}

// Response types mirror the YAML config structure exactly.

type configResponse struct {
	Server   serverCfgResponse            `json:"server"`
	Database databaseCfgResponse          `json:"database"`
	Media    mediaCfgResponse             `json:"media"`
	Modules  map[string]moduleCfgResponse `json:"modules"`
	Sources  map[string]sourceCfgResponse `json:"sources"`
	Log      logCfgResponse               `json:"log"`
	Locked   map[string]bool              `json:"locked"`
}

type serverCfgResponse struct {
	Port    int `json:"port"`
	Workers int `json:"workers"`
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
	Modules map[string]*modulePatchRequest `json:"modules"`
	Sources map[string]*sourcePatchRequest `json:"sources"`
}

type modulePatchRequest struct {
	Enabled *bool    `json:"enabled"`
	Roots   []string `json:"roots"`
}

type sourcePatchRequest struct {
	Enabled   *bool   `json:"enabled"`
	URL       *string `json:"url"`
	APIKey    *string `json:"api_key"`
	UserAgent *string `json:"user_agent"`
}

type contentTypeConfigResponse struct {
	ContentType string   `json:"contentType"`
	PersonRoles []string `json:"personRoles"`
}

type kindConfigResponse struct {
	Kind        string   `json:"kind"`
	PersonRoles []string `json:"personRoles"`
	ShowDates   bool     `json:"showDates"`
}

type configKV struct{ key, value string }

func (h *configHandler) contentTypes(w http.ResponseWriter, _ *http.Request) {
	out := make([]contentTypeConfigResponse, 0, len(domain.ContentTypes()))
	for _, ct := range domain.ContentTypes() {
		out = append(out, contentTypeConfigResponse{
			ContentType: string(ct),
			PersonRoles: ct.ItemPersonRoles(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *configHandler) kinds(w http.ResponseWriter, _ *http.Request) {
	out := make([]kindConfigResponse, 0, len(domain.Kinds()))
	for _, k := range domain.Kinds() {
		out = append(out, kindConfigResponse{
			Kind:        string(k),
			PersonRoles: k.EntryPersonRoles(),
			ShowDates:   k.SupportsMemberRelationships(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

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
		Modules:  buildModulesMap(c.Modules),
		Sources:  buildSourcesMap(c.Sources),
		Log:      logCfgResponse{Level: c.Log.Level, Format: c.Log.Format},
		Locked:   locked,
	})
}

// buildSourcesMap derives the sources response map from mapstructure tags on
// MetadataSourcesConfig. Adding a new source to that struct automatically
// includes it here with no changes required in this file.
func buildSourcesMap(srcs config.MetadataSourcesConfig) map[string]sourceCfgResponse {
	m := make(map[string]sourceCfgResponse)
	v := reflect.ValueOf(srcs)
	t := reflect.TypeOf(srcs)
	for i := range t.NumField() {
		key := t.Field(i).Tag.Get("mapstructure")
		if key == "" {
			continue
		}
		m[key] = maskSource(v.Field(i).Interface().(config.MetadataSourceConfig))
	}
	return m
}

// buildModulesMap derives the modules response map from mapstructure tags on
// ModulesConfig. Adding a new module to that struct automatically includes it
// here with no changes required in this file.
func buildModulesMap(mods config.ModulesConfig) map[string]moduleCfgResponse {
	m := make(map[string]moduleCfgResponse)
	v := reflect.ValueOf(mods)
	t := reflect.TypeOf(mods)
	for i := range t.NumField() {
		key := t.Field(i).Tag.Get("mapstructure")
		if key == "" {
			continue
		}
		mod := v.Field(i).Interface().(config.ModuleConfig)
		m[key] = moduleCfgResponse{Enabled: mod.Enabled, Roots: nullableRoots(mod.Roots)}
	}
	return m
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
	out := make([]configKV, 0, len(req.Modules)+len(req.Sources))
	for name, m := range req.Modules {
		out = append(out, modulePatchKVs(name, m)...)
	}
	for name, s := range req.Sources {
		out = append(out, sourcePatchKVs(name, s)...)
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
