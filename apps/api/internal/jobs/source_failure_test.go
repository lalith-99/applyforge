package jobs

import (
	"errors"
	"testing"
)

func TestIsPermanentSourcePollFailure(t *testing.T) {
	tests := []struct {
		name       string
		sourceType string
		err        error
		want       bool
	}{
		{name: "greenhouse 404", sourceType: "GREENHOUSE", err: errors.New("greenhouse board acme returned 404 Not Found"), want: true},
		{name: "ashby 404", sourceType: "ASHBY", err: errors.New("ashby board acme returned 404 Not Found"), want: true},
		{name: "workable 410", sourceType: "WORKABLE", err: errors.New("workable account acme returned 410 Gone"), want: true},
		{name: "workday 422", sourceType: "WORKDAY", err: errors.New("Workday board acme/careers returned 422 Unprocessable Entity"), want: true},
		{name: "icims 404", sourceType: "ICIMS", err: errors.New("iCIMS source returned 404 Not Found"), want: true},
		{name: "successfactors 404", sourceType: "SUCCESSFACTORS", err: errors.New("SuccessFactors feed returned 404 Not Found"), want: true},
		{name: "successfactors 410", sourceType: "SUCCESSFACTORS", err: errors.New("SuccessFactors feed returned 410 Gone"), want: true},
		{name: "broad provider 404 stays retryable", sourceType: "BRIGHTDATA", err: errors.New("Bright Data returned 404 Not Found"), want: false},
		{name: "broad provider 422 stays retryable", sourceType: "BRIGHTDATA", err: errors.New("Bright Data returned 422 Unprocessable Entity"), want: false},
		{name: "rate limit", sourceType: "GREENHOUSE", err: errors.New("greenhouse board acme returned 429 Too Many Requests"), want: false},
		{name: "server error", sourceType: "ASHBY", err: errors.New("ashby board acme returned 500 Internal Server Error"), want: false},
		{name: "workday parser defect stays retryable", sourceType: "WORKDAY", err: errors.New("Workday posting \"\" has no detail slug"), want: false},
		{name: "transport failure", sourceType: "LEVER", err: errors.New("fetch lever board acme: context deadline exceeded"), want: false},
		{name: "nil", sourceType: "GREENHOUSE", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPermanentSourcePollFailure(tt.sourceType, tt.err); got != tt.want {
				t.Fatalf("isPermanentSourcePollFailure(%q, %v) = %v, want %v", tt.sourceType, tt.err, got, tt.want)
			}
		})
	}
}

func TestIsPermanentSourceErrorText(t *testing.T) {
	if !isPermanentSourceErrorText("PERMANENT_SOURCE: greenhouse board acme returned 404 Not Found") {
		t.Fatal("expected permanent marker to be detected")
	}
	if isPermanentSourceErrorText("greenhouse board acme returned 404 Not Found") {
		t.Fatal("unmarked historical error must not be treated as quarantined")
	}
}
