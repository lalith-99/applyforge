package jobs

import (
	"testing"
	"time"
)

func TestSourceScheduleScorePrioritizesSponsorDirectSources(t *testing.T) {
	now := time.Date(2026, 9, 14, 20, 0, 0, 0, time.UTC)
	oneHourAgo := now.Add(-time.Hour)

	hotDirect := scheduledJobSource{
		config:              JobSourceConfig{SourceType: "GREENHOUSE"},
		sponsorTier:         "HOT",
		lastPolledAt:        &oneHourAgo,
		pollIntervalMinutes: 60,
	}
	coldBroad := scheduledJobSource{
		config:              JobSourceConfig{SourceType: "ARBEITNOW"},
		sponsorTier:         "COLD",
		lastPolledAt:        &oneHourAgo,
		pollIntervalMinutes: 60,
	}

	if sourceScheduleScore(hotDirect, now) <= sourceScheduleScore(coldBroad, now) {
		t.Fatalf("expected HOT direct ATS source to outrank equally due COLD broad source")
	}
}

func TestSourceScheduleScoreLetsOverdueSourcesCatchUp(t *testing.T) {
	now := time.Date(2026, 9, 14, 20, 0, 0, 0, time.UTC)
	oneHourAgo := now.Add(-time.Hour)
	eightHoursAgo := now.Add(-8 * time.Hour)

	hotDirect := scheduledJobSource{
		config:              JobSourceConfig{SourceType: "GREENHOUSE"},
		sponsorTier:         "HOT",
		lastPolledAt:        &oneHourAgo,
		pollIntervalMinutes: 60,
		usefulFreshJobs24h:  100,
	}
	coldDirectVeryOverdue := scheduledJobSource{
		config:              JobSourceConfig{SourceType: "LEVER"},
		sponsorTier:         "COLD",
		lastPolledAt:        &eightHoursAgo,
		pollIntervalMinutes: 60,
	}

	if sourceScheduleScore(coldDirectVeryOverdue, now) <= sourceScheduleScore(hotDirect, now) {
		t.Fatalf("expected sufficiently overdue COLD source to outrank productive newly due HOT source")
	}
}

func TestSourceScheduleScoreNeverPolledDoesNotBeatDueHotDirectByDefault(t *testing.T) {
	now := time.Date(2026, 9, 14, 20, 0, 0, 0, time.UTC)
	oneHourAgo := now.Add(-time.Hour)

	hotDirect := scheduledJobSource{
		config:              JobSourceConfig{SourceType: "WORKDAY"},
		sponsorTier:         "HOT",
		lastPolledAt:        &oneHourAgo,
		pollIntervalMinutes: 60,
	}
	newBroad := scheduledJobSource{
		config:              JobSourceConfig{SourceType: "SERPAPI_GOOGLE_JOBS"},
		pollIntervalMinutes: 360,
	}

	if sourceScheduleScore(newBroad, now) >= sourceScheduleScore(hotDirect, now) {
		t.Fatalf("expected newly discovered broad source to remain behind a due HOT direct ATS source")
	}
}

func TestSourceScheduleScoreRewardsUsefulFreshYield(t *testing.T) {
	now := time.Date(2026, 9, 14, 20, 0, 0, 0, time.UTC)
	oneHourAgo := now.Add(-time.Hour)

	productive := scheduledJobSource{
		config:              JobSourceConfig{SourceType: "GREENHOUSE"},
		sponsorTier:         "WARM",
		lastPolledAt:        &oneHourAgo,
		pollIntervalMinutes: 60,
		usefulFreshJobs24h:  7,
	}
	quiet := scheduledJobSource{
		config:              JobSourceConfig{SourceType: "GREENHOUSE"},
		sponsorTier:         "WARM",
		lastPolledAt:        &oneHourAgo,
		pollIntervalMinutes: 60,
	}

	if sourceScheduleScore(productive, now) <= sourceScheduleScore(quiet, now) {
		t.Fatalf("expected productive source to outrank otherwise equivalent quiet source")
	}
}

func TestSourceYieldScheduleBonusIsBounded(t *testing.T) {
	if got := sourceYieldScheduleBonus(0); got != 0 {
		t.Fatalf("expected zero yield bonus for no useful jobs, got %v", got)
	}
	if got := sourceYieldScheduleBonus(7); got != 1.5 {
		t.Fatalf("expected seven useful jobs to reach 1.5 cap, got %v", got)
	}
	if got := sourceYieldScheduleBonus(10000); got != 1.5 {
		t.Fatalf("expected huge source yield to remain capped at 1.5, got %v", got)
	}
}
