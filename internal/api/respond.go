package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// page is the standard envelope for paginated list responses.
type page[T any] struct {
	Data   []T `json:"data"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type errResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errResponse{Error: msg, Code: code})
}

func paginate(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return
}

// boolPtr parses an optional boolean query parameter.
// Returns nil if the parameter is absent or unparseable.
func boolPtr(r *http.Request, key string) *bool {
	s := r.URL.Query().Get(key)
	if s == "" {
		return nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return nil
	}
	return &b
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
