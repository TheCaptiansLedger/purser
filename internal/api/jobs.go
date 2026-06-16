package api

import (
	"context"
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
	r.Post("/_simulate", h.simulate) // dev/test only — submit a fake sleep job
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

type simulateRequest struct {
	Name       string `json:"name"`
	DurationMs int    `json:"durationMs"`
	Steps      int    `json:"steps"` // how many progress ticks; 0 means indeterminate
}

// simulate submits a fake job that sleeps for DurationMs, reporting progress
// every tick. Intended for manual UI testing only.
func (h *jobHandler) simulate(w http.ResponseWriter, r *http.Request) {
	var req simulateRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.Name == "" {
		req.Name = "simulated-job"
	}
	if req.DurationMs <= 0 {
		req.DurationMs = 1000
	}
	if req.Steps < 0 {
		req.Steps = 0
	}

	duration := time.Duration(req.DurationMs) * time.Millisecond
	steps := req.Steps

	job, err := h.queue.Submit(r.Context(), req.Name, nil, func(ctx context.Context, p ports.ProgressReporter) error {
		if steps == 0 {
			select {
			case <-time.After(duration):
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		}

		tick := duration / time.Duration(steps)
		for i := 1; i <= steps; i++ {
			select {
			case <-time.After(tick):
			case <-ctx.Done():
				return ctx.Err()
			}
			p.Report(i, steps, "")
		}
		return nil
	})
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusAccepted, jobToResponse(job))
}
