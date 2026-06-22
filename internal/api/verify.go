package api

import (
	"net/http"
	"purser/internal/ports"
)

type verifyHandler struct {
	sources []ports.MetadataSource
}

type verifySourceRequest struct {
	Source string `json:"source"`
}

type verifySourceResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (h *verifyHandler) source(w http.ResponseWriter, r *http.Request) {
	var req verifySourceRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	var target ports.Verifiable
	for _, s := range h.sources {
		if s.Name() == req.Source {
			v, ok := s.(ports.Verifiable)
			if !ok {
				writeError(w, http.StatusBadRequest, "NOT_VERIFIABLE", "source does not support verification")
				return
			}
			target = v
			break
		}
	}
	if target == nil {
		writeError(w, http.StatusBadRequest, "UNKNOWN_SOURCE", "unknown source: "+req.Source)
		return
	}

	if err := target.Verify(r.Context()); err != nil {
		writeJSON(w, http.StatusOK, verifySourceResponse{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, verifySourceResponse{OK: true})
}
