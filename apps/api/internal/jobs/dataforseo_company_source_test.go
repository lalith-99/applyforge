package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataForSEOCompanySourceResolverFindsDirectATS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "login" || pass != "password" {
			t.Fatalf("unexpected basic auth")
		}

		var payload []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(payload) != 1 || payload[0]["keyword"] != "Acme Corporation careers jobs" {
			t.Fatalf("unexpected payload: %+v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status_code": 20000,
			"status_message": "Ok.",
			"tasks": [{
				"id": "task-123",
				"status_code": 20000,
				"status_message": "Ok.",
				"result": [{
					"items": [
						{
							"type": "organic",
							"title": "Acme Careers",
							"url": "https://jobs.lever.co/acme/123",
							"description": "Open roles"
						},
						{
							"type": "organic",
							"title": "LinkedIn",
							"url": "https://www.linkedin.com/company/acme/jobs",
							"description": "Jobs"
						}
					]
				}]
			}]
		}`))
	}))
	defer server.Close()

	resolver := NewDataForSEOCompanySourceResolver(DataForSEOCompanySourceConfig{
		Login:        "login",
		Password:     "password",
		Endpoint:     server.URL,
		LocationCode: 2840,
		LanguageCode: "en",
		Depth:        10,
	})

	result, err := resolver.Resolve(context.Background(), "Acme Corporation")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.ProviderRequestID != "task-123" {
		t.Fatalf("unexpected provider request id: %s", result.ProviderRequestID)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected one direct source, got %+v", result.Sources)
	}
	if result.Sources[0].SourceType != "LEVER" || result.Sources[0].BoardToken != "acme" {
		t.Fatalf("unexpected source: %+v", result.Sources[0])
	}
	if !result.Sources[0].Monitorable {
		t.Fatal("expected Lever source to be directly monitorable")
	}
}

func TestDataForSEOCompanySourceResolverFallsBackToCareerPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status_code": 20000,
			"tasks": [{
				"id": "task-456",
				"status_code": 20000,
				"result": [{
					"items": [
						{
							"type": "organic",
							"title": "Careers at Acme",
							"url": "https://www.acme.example/careers",
							"description": "Explore open positions"
						}
					]
				}]
			}]
		}`))
	}))
	defer server.Close()

	resolver := NewDataForSEOCompanySourceResolver(DataForSEOCompanySourceConfig{
		Login:        "login",
		Password:     "password",
		Endpoint:     server.URL,
		LocationCode: 2840,
		LanguageCode: "en",
		Depth:        10,
	})

	result, err := resolver.Resolve(context.Background(), "Acme")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(result.Sources) != 1 || result.Sources[0].SourceType != "CUSTOM" {
		t.Fatalf("expected custom careers source, got %+v", result.Sources)
	}
	if result.Sources[0].Monitorable {
		t.Fatal("custom career page must not be marked directly monitorable")
	}
}

func TestLooksLikeCompanyCareerResultRejectsAggregators(t *testing.T) {
	if looksLikeCompanyCareerResult(
		"https://www.linkedin.com/jobs/search",
		"Acme jobs",
		"Careers",
	) {
		t.Fatal("expected LinkedIn result to be rejected")
	}
	if !looksLikeCompanyCareerResult(
		"https://acme.example/company/careers",
		"Careers at Acme",
		"Open jobs",
	) {
		t.Fatal("expected company career page to be accepted")
	}
}
