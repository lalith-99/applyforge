package jobs

import (
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
)

func TestSanitizeAIClassification(t *testing.T) {
	tests := []struct {
		name string
		in   aiclient.JobRoleClassification
		want RoleClassification
	}{
		{
			name: "accepted software family",
			in:   aiclient.JobRoleClassification{Family: "PLATFORM", Classification: "IC_SOFTWARE", Confidence: 0.91},
			want: RoleClassification{Family: "PLATFORM", Classification: "IC_SOFTWARE", Confidence: 0.91},
		},
		{
			name: "unsupported software family becomes unknown",
			in:   aiclient.JobRoleClassification{Family: "ROCKET_ENGINEERING", Classification: "IC_SOFTWARE", Confidence: 0.99},
			want: RoleClassification{Family: "UNKNOWN", Classification: "UNKNOWN", Confidence: 0.25},
		},
		{
			name: "non software collapses to excluded",
			in:   aiclient.JobRoleClassification{Family: "MECHANICAL", Classification: "NON_SOFTWARE", Confidence: 0.88},
			want: RoleClassification{Family: "EXCLUDED", Classification: "NON_SOFTWARE", Confidence: 0.88},
		},
		{
			name: "invalid classification becomes unknown",
			in:   aiclient.JobRoleClassification{Family: "BACKEND", Classification: "MAYBE", Confidence: 0.8},
			want: RoleClassification{Family: "UNKNOWN", Classification: "UNKNOWN", Confidence: 0.25},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeAIClassification(tt.in); got != tt.want {
				t.Fatalf("got %+v want %+v", got, tt.want)
			}
		})
	}
}
