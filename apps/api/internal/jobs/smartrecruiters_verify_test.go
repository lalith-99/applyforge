package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSmartRecruitersSource_VerifyCompanyOwnership_LegalEntityAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limit":20,"offset":0,"totalFound":1,"content":[{"company":{"identifier":"Deloitte","name":"Deloitte"}}]}`))
	}))
	defer server.Close()

	source := NewSmartRecruitersSource("Deloitte")
	source.BaseURL = server.URL
	source.http = server.Client()

	verification, err := source.VerifyCompanyOwnership(context.Background(), "Deloitte Consulting LLP")
	if err != nil {
		t.Fatalf("VerifyCompanyOwnership: %v", err)
	}
	if !verification.Verified {
		t.Fatalf("expected verified tenant, got %+v", verification)
	}
	if verification.ObservedName != "Deloitte" || verification.ObservedIdentifier != "Deloitte" {
		t.Fatalf("unexpected verification evidence: %+v", verification)
	}
}

func TestSmartRecruitersSource_VerifyCompanyOwnership_RejectsDifferentEmployer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limit":20,"offset":0,"totalFound":1,"content":[{"company":{"identifier":"AnotherCompany","name":"Another Company"}}]}`))
	}))
	defer server.Close()

	source := NewSmartRecruitersSource("candidate-tenant")
	source.BaseURL = server.URL
	source.http = server.Client()

	verification, err := source.VerifyCompanyOwnership(context.Background(), "Deloitte Consulting LLP")
	if err != nil {
		t.Fatalf("VerifyCompanyOwnership: %v", err)
	}
	if verification.Verified {
		t.Fatalf("different employer must not verify: %+v", verification)
	}
	if verification.ObservedName != "Another Company" {
		t.Fatalf("expected mismatch evidence to be returned: %+v", verification)
	}
}

func TestSmartRecruitersSource_VerifyCompanyOwnership_EmptyBoardIsUnverified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limit":20,"offset":0,"totalFound":0,"content":[]}`))
	}))
	defer server.Close()

	source := NewSmartRecruitersSource("empty")
	source.BaseURL = server.URL
	source.http = server.Client()

	if _, err := source.VerifyCompanyOwnership(context.Background(), "Acme"); err == nil {
		t.Fatal("expected an empty board to remain unverifiable")
	}
}

func TestSmartRecruitersSource_VerifyCompanyOwnership_MissingProviderIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limit":20,"offset":0,"totalFound":1,"content":[{"company":{}}]}`))
	}))
	defer server.Close()

	source := NewSmartRecruitersSource("missing-identity")
	source.BaseURL = server.URL
	source.http = server.Client()

	if _, err := source.VerifyCompanyOwnership(context.Background(), "Acme"); err == nil {
		t.Fatal("expected missing provider identity to remain unverifiable")
	}
}
