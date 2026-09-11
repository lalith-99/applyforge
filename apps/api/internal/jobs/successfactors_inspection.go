package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

func isSuccessFactorsInspectionPayload(payload InspectCompanySourcePayload) bool {
	if strings.EqualFold(strings.TrimSpace(payload.SourceType), "SUCCESSFACTORS") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(payload.SourceType), "CUSTOM") &&
		strings.HasPrefix(strings.ToUpper(strings.TrimSpace(payload.BoardToken)), "SUCCESSFACTORS|")
}

func (w *CompanySourceInspectionWorker) verifySuccessFactorsSource(
	ctx context.Context,
	registryID uuid.UUID,
	companyID uuid.UUID,
	payload InspectCompanySourcePayload,
) error {
	boardToken := strings.TrimSpace(payload.SourceURL)
	if boardToken == "" {
		return w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			30*24*time.Hour,
			fmt.Errorf("SuccessFactors registry URL is empty"),
		)
	}

	source, err := NewSuccessFactorsSource(boardToken)
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
		slog.Info("SuccessFactors source verification deferred",
			"company_id", companyID,
			"company_name", payload.CompanyName,
			"board_token", boardToken,
			"error", verifyErr,
		)
		return nil
	}

	if !verification.Verified {
		mismatchErr := fmt.Errorf(
			"SuccessFactors tenant identity mismatch: expected %q, provider returned %q",
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
		slog.Warn("SuccessFactors source identity mismatch",
			"company_id", companyID,
			"company_name", payload.CompanyName,
			"board_token", boardToken,
			"observed_company_name", verification.ObservedName,
		)
		return nil
	}

	candidate := DiscoveredCompanySource{
		SourceType:      "SUCCESSFACTORS",
		BoardToken:      boardToken,
		SourceURL:       boardToken,
		DiscoveryMethod: "CAREER_PAGE",
		Confidence:      0.99,
		Monitorable:     true,
	}
	if err := w.repo.RecordDiscoveredCompanySources(ctx, companyID, []DiscoveredCompanySource{candidate}); err != nil {
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

	slog.Info("verified SuccessFactors source ownership",
		"company_id", companyID,
		"company_name", payload.CompanyName,
		"board_token", boardToken,
		"observed_company_name", verification.ObservedName,
	)
	return w.repo.MarkCompanySourceInspectionOutcome(ctx, registryID, "RESOLVED", 0, nil)
}
