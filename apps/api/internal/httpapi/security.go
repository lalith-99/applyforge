package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

const companionTokenHeader = "X-ApplyForge-Companion-Token"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}

func protectBrowserMutations(webBaseURL string) func(http.Handler) http.Handler {
	expected := originOf(webBaseURL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			// Browser-companion requests carry a short-lived, intent-scoped bearer
			// capability. Only the self-authenticated /companion surface may bypass
			// normal web Origin checks; regular session endpoints remain protected
			// even if a caller adds the same header.
			if strings.HasPrefix(r.URL.Path, "/api/v1/companion/") && strings.TrimSpace(r.Header.Get(companionTokenHeader)) != "" {
				next.ServeHTTP(w, r)
				return
			}

			if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-site mutation rejected"})
				return
			}

			if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
				if expected != "" && originOf(origin) != expected {
					writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid request origin"})
					return
				}
			} else if referer := strings.TrimSpace(r.Referer()); referer != "" {
				if expected != "" && originOf(referer) != expected {
					writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid request origin"})
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func originOf(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}
