package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ivantung/todo-backend/internal/auth"
	"github.com/ivantung/todo-backend/internal/httpx"
)

type ctxKey int

const userCtxKey ctxKey = 1

type Identity struct {
	UserID   int64
	Username string
}

func RequireAuth(signer *auth.Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r)
			if raw == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			claims, err := signer.Parse(raw)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ident := Identity{UserID: claims.UserID, Username: claims.Username}
			ctx := context.WithValue(r.Context(), userCtxKey, ident)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func FromContext(ctx context.Context) (Identity, bool) {
	v, ok := ctx.Value(userCtxKey).(Identity)
	return v, ok
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
