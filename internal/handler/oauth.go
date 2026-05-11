package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/ivantung/todo-backend/internal/httpx"
	"github.com/ivantung/todo-backend/internal/oauth"
	"github.com/ivantung/todo-backend/internal/service"
)

const stateCookieName = "oauth_state"

type OAuthHandler struct {
	google      *oauth.Service
	auth        *service.AuthService
	frontendURL string
}

func NewOAuthHandler(google *oauth.Service, authSvc *service.AuthService, frontendURL string) *OAuthHandler {
	return &OAuthHandler{google: google, auth: authSvc, frontendURL: frontendURL}
}

func (h *OAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not start oauth")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  time.Now().Add(10 * time.Minute),
	})
	http.Redirect(w, r, h.google.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if errParam := q.Get("error"); errParam != "" {
		h.fail(w, r, "google sign-in cancelled")
		return
	}
	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		h.fail(w, r, "missing state or code")
		return
	}
	cookie, err := r.Cookie(stateCookieName)
	if err != nil || cookie.Value == "" || cookie.Value != state {
		h.fail(w, r, "invalid oauth state")
		return
	}
	// clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name: stateCookieName, Value: "", Path: "/", MaxAge: -1,
	})

	profile, err := h.google.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("oauth exchange: %v", err)
		h.fail(w, r, "could not verify google sign-in")
		return
	}

	res, err := h.auth.LoginOrRegisterFromGoogle(r.Context(), *profile)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailNotVerified):
			h.fail(w, r, "google email is not verified")
		default:
			log.Printf("oauth login: %v", err)
			h.fail(w, r, "could not complete sign-in")
		}
		return
	}

	target := h.frontendURL + "/auth/callback#token=" + url.QueryEscape(res.Token) +
		"&expires=" + url.QueryEscape(res.ExpiresAt.Format(time.RFC3339))
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *OAuthHandler) fail(w http.ResponseWriter, r *http.Request, msg string) {
	target := h.frontendURL + "/login?oauth_error=" + url.QueryEscape(msg)
	http.Redirect(w, r, target, http.StatusFound)
}

func randomState() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}
