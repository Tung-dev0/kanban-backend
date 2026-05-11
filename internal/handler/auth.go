package handler

import (
	"errors"
	"net/http"

	"github.com/ivantung/todo-backend/internal/httpx"
	"github.com/ivantung/todo-backend/internal/middleware"
	"github.com/ivantung/todo-backend/internal/repository"
	"github.com/ivantung/todo-backend/internal/service"
)

type AuthHandler struct {
	svc   *service.AuthService
	users *repository.UserRepo
}

func NewAuthHandler(svc *service.AuthService, users *repository.UserRepo) *AuthHandler {
	return &AuthHandler{svc: svc, users: users}
}

type credentialsReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req credentialsReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	res, err := h.svc.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			httpx.WriteError(w, http.StatusBadRequest, "username must be 3-32 chars [a-zA-Z0-9_.-], password 6-128 chars")
		case errors.Is(err, repository.ErrUsernameTaken):
			httpx.WriteError(w, http.StatusConflict, "username already taken")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "could not register")
		}
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, res)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req credentialsReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	res, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			httpx.WriteError(w, http.StatusUnauthorized, "invalid username or password")
		case errors.Is(err, service.ErrUseGoogle):
			httpx.WriteError(w, http.StatusUnauthorized, "this account was created with Google — use the Google button to sign in")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "could not login")
		}
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// Me returns the authenticated user's profile.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	ident, ok := middleware.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	u, err := h.users.GetByID(r.Context(), ident.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			httpx.WriteError(w, http.StatusUnauthorized, "user no longer exists")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not load user")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}
