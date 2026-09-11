package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (w *CompanySourceInspectionWorker) verifyICIMSSource(
	ctx context.Context,
	registryID uuid.UUID,
	companyID uuid.UUID,
	payload InspectCompanySourcePayload,
) error {
	var candidate *DiscoveredCompanySource
	for _, discovery := range DetectCompanySources(payload.SourceURL) {
		if discovery.SourceType != "ICIMS" || strings.TrimSpace(discovery.BoardToken) == "" {
			continue
		}
		copy := discovery
		candidate = &copy
		break
	}
	if candidate == nil {
		invalidErr := errors.New("iCIMS registry URL did not contain a verifiable tenant host")
		return w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			30*24*time.Hour,
			invalidErr,
		)
	}

	source, err := NewICIMSSource(candidate.BoardToken)
	if err != nil {
		return w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			30*24*time.Hour,
			err,
		)
	}
	verification, verifyErr := source.VerifyCompanyOwnership(ctx, payload.CompanyName)
	if verifyErr != nil {
		if markErr := w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			7*24*time.Hour,
			verifyErr,
		); markErr != nil {
			return markErr
		}
		slog.Info("iCIMS source verification deferred",
			"company_id", companyID,
			"company_name", payload.CompanyName,
			"board_token", candidate.BoardToken,
			"error", verifyErr,
		)
		return nil
	}
	if !verification.Verified {
		mismatchErr := fmt.Errorf(
			"iCIMS tenant identity mismatch: expected %q, provider returned %q",
			payload.CompanyName,
			verification.ObservedName,
		)
		if markErr := w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			30*24*time.Hour,
			mismatchErr,
		); markErr != nil {
			return markErr
		}
		slog.Warn("iCIMS source identity mismatch",
			"company_id", companyID,
			"company_name", payload.CompanyName,
			"board_token", candidate.BoardToken,
			"observed_company_name", verification.ObservedName,
		)
		return nil
	}

	candidate.DiscoveryMethod = "CAREER_PAGE"
	candidate.Confidence = 0.99
	candidate.Monitorable = true
	if err := w.repo.RecordDiscoveredCompanySources(ctx, companyID, []DiscoveredCompanySource{*candidate}); err != nil {
		if markErr := w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			7*24*time.Hour,
			err,
		); markErr != nil {
			return markErr
		}
		return nil
	}

	slog.Info("verified iCIMS source ownership",
		"company_id", companyID,
		"company_name", payload.CompanyName,
		"board_token", candidate.BoardToken,
		"observed_company_name", verification.ObservedName,
	)
	return w.repo.MarkCompanySourceInspectionOutcome(ctx, registryID, "RESOLVED", 0, nil)
}
