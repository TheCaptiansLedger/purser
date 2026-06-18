package api

import (
	"net/http"
	"purser/internal/app/library"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type itemHandler struct {
	svc *library.Service
}

func (h *itemHandler) routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
}

// ── Response types ────────────────────────────────────────────────────────────

type itemResponse struct {
	ID             string               `json:"id"`
	ContentType    string               `json:"contentType"`
	LibraryEntryID string               `json:"libraryEntryId"`
	GroupID        string               `json:"groupId,omitempty"`
	Title          string               `json:"title"`
	Overview       string               `json:"overview"`
	Date           string               `json:"date,omitempty"` // YYYY-MM-DD
	Sequence       string               `json:"sequence,omitempty"`
	RuntimeSeconds int                  `json:"runtimeSeconds"`
	Monitored      bool                 `json:"monitored"`
	Status         string               `json:"status"`
	CoverURL       string               `json:"coverUrl,omitempty"`
	People         []itemPersonResponse `json:"people"`
	Tags           []tagResponse        `json:"tags"`
	ExternalIDs    []externalIDResponse `json:"externalIds"`
	MediaFile      *mediaFileResponse   `json:"mediaFile,omitempty"`
	Metadata       map[string]any       `json:"metadata,omitempty"`
	AddedAt        time.Time            `json:"addedAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

func toItemResponse(item *domain.Item) *itemResponse {
	r := &itemResponse{
		ID:             item.ID,
		ContentType:    string(item.ContentType),
		LibraryEntryID: item.LibraryEntryID,
		GroupID:        item.GroupID,
		Title:          item.Title,
		Overview:       item.Overview,
		Sequence:       item.Sequence,
		RuntimeSeconds: item.RuntimeSeconds,
		Monitored:      item.Monitored,
		Status:         string(item.Status),
		CoverURL:       imageURL("items", item.ID, item.CoverPath),
		Metadata:       item.Metadata,
		AddedAt:        item.AddedAt,
		UpdatedAt:      item.UpdatedAt,
		People:         []itemPersonResponse{},
		Tags:           []tagResponse{},
		ExternalIDs:    []externalIDResponse{},
	}
	if !item.Date.IsZero() {
		r.Date = item.Date.UTC().Format("2006-01-02")
	}
	for _, ip := range item.People {
		ipr := itemPersonResponse{
			PersonID: ip.PersonID,
			Role:     string(ip.Role),
		}
		if ip.Person != nil {
			ipr.Person = &personRefResponse{
				ID:       ip.Person.ID,
				Name:     ip.Person.Name,
				SortName: ip.Person.SortName,
				ImageURL: imageURL("people", ip.Person.ID, ip.Person.ImagePath),
			}
		}
		r.People = append(r.People, ipr)
	}
	for _, t := range item.Tags {
		r.Tags = append(r.Tags, tagResponse{ID: t.ID, Name: t.Name, Scope: string(t.Scope)})
	}
	for _, id := range item.ExternalIDs {
		r.ExternalIDs = append(r.ExternalIDs, externalIDResponse{
			Source: string(id.Source),
			Value:  id.Value,
		})
	}
	if item.MediaFile != nil {
		mf := item.MediaFile
		r.MediaFile = &mediaFileResponse{
			ID:         mf.ID,
			Path:       mf.Path,
			Size:       mf.Size,
			OSHash:     mf.OSHash,
			Quality:    string(mf.Quality),
			Resolution: mf.Resolution,
			Codec:      mf.Codec,
			Container:  mf.Container,
			AddedAt:    mf.AddedAt,
		}
	}
	return r
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *itemHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r)
	q := r.URL.Query()

	f := ports.ItemFilter{
		LibraryEntryID: q.Get("libraryEntryId"),
		GroupID:        q.Get("groupId"),
		ContentType:    domain.ContentType(q.Get("contentType")),
		Status:         domain.ItemStatus(q.Get("status")),
		Monitored:      boolPtr(r, "monitored"),
		PersonID:       q.Get("personId"),
		Search:         q.Get("search"),
		Limit:          limit,
		Offset:         offset,
	}
	if raw := q.Get("tagIds"); raw != "" {
		f.TagIDs = strings.Split(raw, ",")
	}

	items, total, err := h.svc.ListItems(r.Context(), f)
	if handleErr(w, err) {
		return
	}

	resp := make([]*itemResponse, len(items))
	for i, item := range items {
		resp[i] = toItemResponse(item)
	}
	writeJSON(w, http.StatusOK, page[*itemResponse]{Data: resp, Total: total, Limit: limit, Offset: offset})
}

func (h *itemHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.svc.GetItem(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toItemResponse(item))
}

type createItemRequest struct {
	ContentType    string `json:"contentType"`
	LibraryEntryID string `json:"libraryEntryId"`
	GroupID        string `json:"groupId"`
	Title          string `json:"title"`
	Overview       string `json:"overview"`
	Date           string `json:"date"` // YYYY-MM-DD
	Sequence       string `json:"sequence"`
	RuntimeSeconds int    `json:"runtimeSeconds"`
	Monitored      bool   `json:"monitored"`
	ExternalIDs    []struct {
		Source string `json:"source"`
		Value  string `json:"value"`
	} `json:"externalIds"`
	People []struct {
		PersonID string `json:"personId"`
		Role     string `json:"role"`
	} `json:"people"`
	Metadata map[string]any `json:"metadata"`
}

func (h *itemHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createItemRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	item := &domain.Item{
		ContentType:    domain.ContentType(req.ContentType),
		LibraryEntryID: req.LibraryEntryID,
		GroupID:        req.GroupID,
		Title:          req.Title,
		Overview:       req.Overview,
		Sequence:       req.Sequence,
		RuntimeSeconds: req.RuntimeSeconds,
		Monitored:      req.Monitored,
		Metadata:       req.Metadata,
	}
	if req.Date != "" {
		item.Date, _ = time.Parse("2006-01-02", req.Date)
	}
	for _, id := range req.ExternalIDs {
		item.ExternalIDs = append(item.ExternalIDs, domain.ExternalID{
			Source: domain.ExternalIDSource(id.Source),
			Value:  id.Value,
		})
	}
	for _, p := range req.People {
		item.People = append(item.People, domain.ItemPerson{
			PersonID: p.PersonID,
			Role:     domain.PersonRole(p.Role),
		})
	}

	if err := h.svc.CreateItem(r.Context(), item); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, toItemResponse(item))
}

type patchItemRequest struct {
	Title          *string `json:"title"`
	Overview       *string `json:"overview"`
	Date           *string `json:"date"`
	Sequence       *string `json:"sequence"`
	RuntimeSeconds *int    `json:"runtimeSeconds"`
	Monitored      *bool   `json:"monitored"`
	Status         *string `json:"status"`
}

func (h *itemHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.svc.GetItem(r.Context(), id)
	if handleErr(w, err) {
		return
	}

	var req patchItemRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.Title != nil {
		item.Title = *req.Title
	}
	if req.Overview != nil {
		item.Overview = *req.Overview
	}
	if req.Date != nil {
		item.Date, _ = time.Parse("2006-01-02", *req.Date)
	}
	if req.Sequence != nil {
		item.Sequence = *req.Sequence
	}
	if req.RuntimeSeconds != nil {
		item.RuntimeSeconds = *req.RuntimeSeconds
	}
	if req.Monitored != nil {
		item.Monitored = *req.Monitored
	}
	if req.Status != nil {
		newStatus := domain.ItemStatus(*req.Status)
		if newStatus != domain.StatusWanted && newStatus != domain.StatusSkipped {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "status must be 'wanted' or 'skipped'")
			return
		}
		if err := domain.ValidateTransition(item.Status, newStatus); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_TRANSITION", err.Error())
			return
		}
		item.Status = newStatus
	}

	if err := h.svc.SaveItem(r.Context(), item); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toItemResponse(item))
}

func (h *itemHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteItem(r.Context(), id); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
