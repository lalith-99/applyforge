package matching

import "testing"

func TestEmploymentPreferenceRejectsUnknownType(t *testing.T) {
	result := CheckEligibility(Input{
		PreferredEmploymentTypes: []string{"full_time"},
		EmploymentType:           "",
	})
	if result.Eligible {
		t.Fatalf("expected unknown employment type to be ineligible for strict full-time preference: %+v", result)
	}
	if len(result.HardFailures) == 0 {
		t.Fatalf("expected a hard failure explaining unknown employment type: %+v", result)
	}
}

func TestEmploymentPreferenceAcceptsNormalizedFullTimeVariants(t *testing.T) {
	for _, employmentType := range []string{"full_time", "full-time", "full time", "fulltime"} {
		t.Run(employmentType, func(t *testing.T) {
			result := CheckEligibility(Input{
				PreferredEmploymentTypes: []string{"full time"},
				EmploymentType:           employmentType,
			})
			if !result.Eligible {
				t.Fatalf("expected %q to satisfy full-time preference: %+v", employmentType, result)
			}
		})
	}
}

func TestEmploymentPreferenceRejectsContractForFullTimeOnly(t *testing.T) {
	result := CheckEligibility(Input{
		PreferredEmploymentTypes: []string{"full_time"},
		EmploymentType:           "contract",
	})
	if result.Eligible {
		t.Fatalf("expected contract role to be ineligible for full-time-only preference: %+v", result)
	}
}

func TestUnknownEmploymentTypeRemainsEligibleWithoutPreference(t *testing.T) {
	result := CheckEligibility(Input{EmploymentType: ""})
	if !result.Eligible {
		t.Fatalf("unknown employment type should remain catalog-eligible when no preference is configured: %+v", result)
	}
}
