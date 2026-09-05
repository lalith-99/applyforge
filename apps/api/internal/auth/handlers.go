package auth

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

const oauthStateCookieName = "af_oauth_state"

// Handlers wires the auth Service to HTTP routes.
type Handlers struct {
	svc          *Service
	webBaseURL   string
	secureCookie bool
}

// NewHandlers builds auth Handlers. secureCookie should be true in production
// (HTTPS) deployments and false for local HTTP development.
func NewHandlers(svc *Service, webBaseURL string, secureCookie bool) *Handlers {
	return &Handlers{svc: svc, webBaseURL: webBaseURL, secureCookie: secureCookie}
}

// Mount registers auth routes onto r.
func (h *Handlers) Mount(r chi.Router) {
	r.Post("/signup", h.handleSignup)
	r.Post("/login", h.handleLogin)
	r.Post("/logout", h.handleLogout)
	r.Get("/google/start", h.handleGoogleStart)
	r.Get("/google/callback", h.handleGoogleCallback)

	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(h.svc))
		r.Get("/session", h.handleSession)
	})
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c credentialsRequest) validate() error {
	if _, err := mail.ParseAddress(c.Email); err != nil {
		return errors.New("a valid email is required")
	}
	if len(c.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

func (h *Handlers) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	issued, err := h.svc.SignUp(r.Context(), strings.TrimSpace(req.Email), req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		if errors.Is(err, ErrEmailInUse) {
			httpx.WriteError(w, http.StatusConflict, "email already in use")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	h.setSessionCookie(w, issued.Token)
	httpx.WriteJSON(w, http.StatusCreated, userResponse(issued.User))
}

func (h *Handlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	issued, err := h.svc.Login(r.Context(), strings.TrimSpace(req.Email), req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not log in")
		return
	}

	h.setSessionCookie(w, issued.Token)
	httpx.WriteJSON(w, http.StatusOK, userResponse(issued.User))
}

func (h *Handlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		_ = h.svc.Logout(r.Context(), cookie.Value)
	}
	h.clearSessionCookie(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h *Handlers) handleSession(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, userResponse(u))
}

func (h *Handlers) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	state, err := newSessionToken()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	authURL, err := h.svc.GoogleAuthURL(state)
	if err != nil {
		if errors.Is(err, ErrGoogleNotConfigured) {
			httpx.WriteError(w, http.StatusServiceUnavailable, "google sign-in is not configured")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *Handlers) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || r.URL.Query().Get("state") != stateCookie.Value {
		httpx.WriteError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	issued, err := h.svc.CompleteGoogleLogin(r.Context(), code, r.UserAgent(), clientIP(r))
	if err != nil {
		if errors.Is(err, ErrGoogleNotConfigured) {
			httpx.WriteError(w, http.StatusServiceUnavailable, "google sign-in is not configured")
			return
		}
		httpx.WriteError(w, http.StatusUnauthorized, "google sign-in failed")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	h.setSessionCookie(w, issued.Token)
	http.Redirect(w, r, h.webBaseURL+"/onboarding", http.StatusFound)
}

func (h *Handlers) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(SessionTTL),
	})
}

func (h *Handlers) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func userResponse(u users.User) map[string]any {
	return map[string]any{
		"id":                u.ID,
		"email":             u.Email,
		"email_verified_at": u.EmailVerifiedAt,
		"has_password":      u.PasswordHash != nil,
		"has_google":        u.GoogleID != nil,
		"created_at":        u.CreatedAt,
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return r.RemoteAddr
}
