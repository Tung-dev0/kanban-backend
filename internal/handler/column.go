package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ivantung/todo-backend/internal/httpx"
	"github.com/ivantung/todo-backend/internal/middleware"
	"github.com/ivantung/todo-backend/internal/service"
)

// ColumnHandler handles /api/columns routes.
type ColumnHandler struct {
	svc *service.ColumnService
}

func NewColumnHandler(svc *service.ColumnService) *ColumnHandler {
	return &ColumnHandler{svc: svc}
}

// Create handles POST /api/columns
func (h *ColumnHandler) Create(w http.ResponseWriter, r *http.Request) {
	ident, _ := middleware.FromContext(r.Context())

	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	col, err := h.svc.Create(r.Context(), ident.UserID, req.Name)
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "could not create column")
	default:
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{"column": col})
	}
}

// Update handles PUT /api/columns/{id} — rename
func (h *ColumnHandler) Update(w http.ResponseWriter, r *http.Request) {
	ident, _ := middleware.FromContext(r.Context())
	id, ok := parseID(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid column id")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	col, err := h.svc.Rename(r.Context(), ident.UserID, id, req.Name)
	switch {
	case errors.Is(err, service.ErrColumnNotFound):
		httpx.WriteError(w, http.StatusNotFound, "column not found")
	case errors.Is(err, service.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "could not update column")
	default:
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"column": col})
	}
}

// Delete handles DELETE /api/columns/{id}
func (h *ColumnHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ident, _ := middleware.FromContext(r.Context())
	id, ok := parseID(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid column id")
		return
	}

	err := h.svc.Delete(r.Context(), ident.UserID, id)
	switch {
	case errors.Is(err, service.ErrColumnNotFound):
		httpx.WriteError(w, http.StatusNotFound, "column not found")
	case errors.Is(err, service.ErrColumnNotEmpty):
		var notEmpty *service.ColumnNotEmptyError
		errors.As(err, &notEmpty)
		httpx.WriteError(w, http.StatusConflict, fmt.Sprintf("column has %d cards", notEmpty.Count))
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "could not delete column")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// Reorder handles PUT /api/columns/reorder
func (h *ColumnHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	ident, _ := middleware.FromContext(r.Context())

	var req struct {
		Order []int64 `json:"order"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Order) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "order must be a non-empty array of column ids")
		return
	}

	result, err := h.svc.Reorder(r.Context(), ident.UserID, req.Order)
	switch {
	case errors.Is(err, service.ErrReorderMismatch):
		httpx.WriteError(w, http.StatusBadRequest, "order must contain exactly the user's current column ids")
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "could not reorder columns")
	default:
		httpx.WriteJSON(w, http.StatusOK, result)
	}
}

