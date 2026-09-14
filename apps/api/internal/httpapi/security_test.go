package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectBrowserMutationsCompanionBypassIsRouteScoped(t *testing.T) {
	handler := protectBrowserMutations("http://localhost:3000")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	companion := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/companion/submissions/test/claim", nil)
	companion.Header.Set("Sec-Fetch-Site", "cross-site")
	companion.Header.Set(companionTokenHeader, "scoped-token")
	companionRecorder := httptest.NewRecorder()
	handler.ServeHTTP(companionRecorder, companion)
	if companionRecorder.Code != http.StatusNoContent {
		t.Fatalf("companion token should bypass Origin checks only on companion routes, got %d", companionRecorder.Code)
	}

	regular := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/applications", nil)
	regular.Header.Set("Sec-Fetch-Site", "cross-site")
	regular.Header.Set(companionTokenHeader, "scoped-token")
	regularRecorder := httptest.NewRecorder()
	handler.ServeHTTP(regularRecorder, regular)
	if regularRecorder.Code != http.StatusForbidden {
		t.Fatalf("companion header must not bypass CSRF protection on normal routes, got %d", regularRecorder.Code)
	}
}
