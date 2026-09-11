package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SmartRecruitersTenantVerification is the evidence collected from the public
// SmartRecruiters Posting API before a registry-only tenant is promoted to
// direct polling. We deliberately require an employer identity returned by the
// provider rather than trusting an ATS-directory company-name match alone.
type SmartRecruitersTenantVerification struct {
	Verified           bool
	ObservedName       string
	ObservedIdentifier string
}

type smartRecruitersVerificationResponse struct {
	Content []struct {
		Company struct {
			Identifier string `json:"identifier"`
			Name       string `json:"name"`
		} `json:"company"`
	} `json:"content"`
}

// VerifyCompanyOwnership verifies that this SmartRecruiters tenant belongs to
// expectedCompanyName. It uses only the public postings list and does not fetch
// per-posting details. A tenant with no active postings cannot be proven and is
// therefore left registry-only for a later retry.
func (s *SmartRecruitersSource) VerifyCompanyOwnership(
	ctx context.Context,
	expectedCompanyName string,
) (SmartRecruitersTenantVerification, error) {
	expectedCompanyName = strings.TrimSpace(expectedCompanyName)
	if expectedCompanyName == "" {
		return SmartRecruitersTenantVerification{}, errors.New("expected company name is required")
	}
	if strings.TrimSpace(s.CompanyIdentifier) == "" {
		return SmartRecruitersTenantVerification{}, errors.New("SmartRecruiters company identifier is required")
	}

	u, err := url.Parse(fmt.Sprintf("%s/%s/postings", strings.TrimRight(s.BaseURL, "/"), s.CompanyIdentifier))
	if err != nil {
		return SmartRecruitersTenantVerification{}, err
	}
	q := u.Query()
	q.Set("offset", "0")
	q.Set("limit", "20")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return SmartRecruitersTenantVerification{}, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return SmartRecruitersTenantVerification{}, fmt.Errorf("verify SmartRecruiters company %s: %w", s.CompanyIdentifier, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SmartRecruitersTenantVerification{}, fmt.Errorf("verify SmartRecruiters company %s returned %s", s.CompanyIdentifier, resp.Status)
	}

	var parsed smartRecruitersVerificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return SmartRecruitersTenantVerification{}, fmt.Errorf("decode SmartRecruiters verification response: %w", err)
	}
	if len(parsed.Content) == 0 {
		return SmartRecruitersTenantVerification{}, fmt.Errorf("SmartRecruiters company %s has no active postings to verify", s.CompanyIdentifier)
	}

	expectedKeys := make(map[string]struct{})
	for _, key := range sourceCompanyExactKeys(expectedCompanyName) {
		expectedKeys[key] = struct{}{}
	}
	if len(expectedKeys) == 0 {
		return SmartRecruitersTenantVerification{}, errors.New("expected company name could not be normalized")
	}

	var firstObserved SmartRecruitersTenantVerification
	for _, posting := range parsed.Content {
		observedName := strings.TrimSpace(posting.Company.Name)
		observedIdentifier := strings.TrimSpace(posting.Company.Identifier)
		if observedName == "" {
			continue
		}
		if firstObserved.ObservedName == "" {
			firstObserved.ObservedName = observedName
			firstObserved.ObservedIdentifier = observedIdentifier
		}
		for _, observedKey := range sourceCompanyExactKeys(observedName) {
			if _, ok := expectedKeys[observedKey]; ok {
				return SmartRecruitersTenantVerification{
					Verified:           true,
					ObservedName:       observedName,
					ObservedIdentifier: observedIdentifier,
				}, nil
			}
		}
	}

	if firstObserved.ObservedName == "" {
		return SmartRecruitersTenantVerification{}, fmt.Errorf("SmartRecruiters company %s postings did not expose provider company identity", s.CompanyIdentifier)
	}
	return firstObserved, nil
}
