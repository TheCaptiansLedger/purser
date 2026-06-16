package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"purser/internal/domain"
	"purser/internal/ports"
)

type jobHandler struct {
	queue ports.JobQueue
}

func (h *jobHandler) routes(r chi.Router) {
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Delete("/{id}", h.cancel)
}

type jobResponse struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Payload     map[string]any `json:"payload,omitempty"`
	Status      string         `json:"status"`
	Current     int            `json:"current"`
	Total       int            `json:"total"`
	Message     string         `json:"message,omitempty"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	StartedAt   *time.Time     `json:"startedAt"`
	CompletedAt *time.Time     `json:"completedAt"`
}

func jobToResponse(j *domain.Job) jobResponse {
	return jobResponse{
		ID:          j.ID,
		Name:        j.Name,
		Payload:     j.Payload,
		Status:      string(j.Status),
		Current:     j.Current,
		Total:       j.Total,
		Message:     j.Message,
		Error:       j.Error,
		CreatedAt:   j.CreatedAt,
		StartedAt:   j.StartedAt,
		CompletedAt: j.CompletedAt,
	}
}

func (h *jobHandler) list(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.queue.List(r.Context())
	if handleErr(w, err) {
		return
	}
	resp := make([]jobResponse, len(jobs))
	for i, j := range jobs {
		resp[i] = jobToResponse(j)
	}
	writeJSON(w, http.StatusOK, page[jobResponse]{Data: resp, Total: len(resp), Limit: len(resp)})
}

func (h *jobHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	j, err := h.queue.Get(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, jobToResponse(j))
}

func (h *jobHandler) cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.queue.Cancel(r.Context(), id); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
