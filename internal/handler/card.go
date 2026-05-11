package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ivantung/todo-backend/internal/httpx"
	"github.com/ivantung/todo-backend/internal/middleware"
	"github.com/ivantung/todo-backend/internal/service"
)

// CardHandler handles /api/cards routes.
type CardHandler struct {
	svc *service.CardService
}

func NewCardHandler(svc *service.CardService) *CardHandler {
	return &CardHandler{svc: svc}
}

// Create handles POST /api/cards
func (h *CardHandler) Create(w http.ResponseWriter, r *http.Request) {
	ident, _ := middleware.FromContext(r.Context())

	var req struct {
		ColumnID    int64      `json:"column_id"`
		Title       string     `json:"title"`
		Description string     `json:"description"`
		DueAt       *time.Time `json:"due_at"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	card, err := h.svc.Create(r.Context(), ident.UserID, req.ColumnID, req.Title, req.Description, req.DueAt)
	switch {
	case errors.Is(err, service.ErrColumnNotFound):
		httpx.WriteError(w, http.StatusNotFound, "column not found")
	case errors.Is(err, service.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "could not create card")
	default:
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{"card": card})
	}
}

// Get handles GET /api/cards/{id}
func (h *CardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ident, _ := middleware.FromContext(r.Context())
	id, ok := parseID(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid card id")
		return
	}

	card, err := h.svc.Get(r.Context(), ident.UserID, id)
	switch {
	case errors.Is(err, service.ErrCardNotFound):
		httpx.WriteError(w, http.StatusNotFound, "card not found")
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "could not get card")
	default:
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"card": card})
	}
}

// Update handles PUT /api/cards/{id} — partial update
func (h *CardHandler) Update(w http.ResponseWriter, r *http.Request) {
	ident, _ := middleware.FromContext(r.Context())
	id, ok := parseID(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid card id")
		return
	}

	// We do NOT use DecodeJSON (which calls DisallowUnknownFields) because
	// we want to be lenient on the update body. Use standard json.Decoder.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	patch := service.UpdatePatch{}

	if v, ok := raw["column_id"]; ok {
		var cid int64
		if err := json.Unmarshal(v, &cid); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid column_id")
			return
		}
		patch.ColumnID = &cid
	}
	if v, ok := raw["title"]; ok {
		var title string
		if err := json.Unmarshal(v, &title); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid title")
			return
		}
		patch.Title = &title
	}
	if v, ok := raw["description"]; ok {
		var desc string
		if err := json.Unmarshal(v, &desc); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid description")
			return
		}
		patch.Description = &desc
	}
	if v, ok := raw["due_at"]; ok {
		patch.DueAtSet = true
		if string(v) == "null" {
			patch.DueAt = nil // clear
		} else {
			var t time.Time
			if err := json.Unmarshal(v, &t); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid due_at: must be a valid RFC3339 timestamp or null")
				return
			}
			patch.DueAt = &t
		}
	}

	if patch.ColumnID == nil && patch.Title == nil && patch.Description == nil && !patch.DueAtSet {
		httpx.WriteError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	card, err := h.svc.Update(r.Context(), ident.UserID, id, patch)
	switch {
	case errors.Is(err, service.ErrCardNotFound):
		httpx.WriteError(w, http.StatusNotFound, "card not found")
	case errors.Is(err, service.ErrColumnNotFound):
		httpx.WriteError(w, http.StatusNotFound, "column not found")
	case errors.Is(err, service.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "could not update card")
	default:
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"card": card})
	}
}

// Delete handles DELETE /api/cards/{id}
func (h *CardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ident, _ := middleware.FromContext(r.Context())
	id, ok := parseID(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid card id")
		return
	}

	err := h.svc.Delete(r.Context(), ident.UserID, id)
	switch {
	case errors.Is(err, service.ErrCardNotFound):
		httpx.WriteError(w, http.StatusNotFound, "card not found")
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "could not delete card")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// SetLabels handles PUT /api/cards/{id}/labels
func (h *CardHandler) SetLabels(w http.ResponseWriter, r *http.Request) {
	ident, _ := middleware.FromContext(r.Context())
	id, ok := parseID(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid card id")
		return
	}

	var req struct {
		Colors []string `json:"colors"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Colors == nil {
		req.Colors = []string{}
	}

	labels, err := h.svc.SetLabels(r.Context(), ident.UserID, id, req.Colors)
	switch {
	case errors.Is(err, service.ErrCardNotFound):
		httpx.WriteError(w, http.StatusNotFound, "card not found")
	case errors.Is(err, service.ErrInvalidLabel):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "could not set labels")
	default:
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"labels": labels})
	}
}

