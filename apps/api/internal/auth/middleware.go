package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

type contextKey string

const userContextKey contextKey = "auth.user"

// SessionCookieName is the name of the cookie carrying the raw session token.
const SessionCookieName = "af_session"

// UserFromContext returns the authenticated user stored by RequireAuth.
func UserFromContext(ctx context.Context) (users.User, bool) {
	u, ok := ctx.Value(userContextKey).(users.User)
	return u, ok
}

// RequireAuth returns middleware that resolves the session cookie into an
// authenticated user, rejecting the request with 401 if absent/invalid.
func RequireAuth(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			u, err := svc.CurrentUser(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, ErrSessionNotFound) || errors.Is(err, users.ErrNotFound) {
					httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
					return
				}
				httpx.WriteError(w, http.StatusInternalServerError, "internal error")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
