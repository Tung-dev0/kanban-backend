package handler

import (
	"net/http"

	"github.com/ivantung/todo-backend/internal/httpx"
	"github.com/ivantung/todo-backend/internal/middleware"
	"github.com/ivantung/todo-backend/internal/service"
)

// BoardHandler handles GET /api/board.
type BoardHandler struct {
	svc *service.BoardService
}

func NewBoardHandler(svc *service.BoardService) *BoardHandler {
	return &BoardHandler{svc: svc}
}

// Get returns the user's board, auto-creating 3 default columns on first call.
func (h *BoardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ident, ok := middleware.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	board, err := h.svc.GetOrInit(r.Context(), ident.UserID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load board")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, board)
}
