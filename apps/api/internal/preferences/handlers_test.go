package preferences

import "testing"

func TestToResponse_NormalizesNilSlicesToEmptyArrays(t *testing.T) {
	response := toResponse(Preferences{})

	for _, field := range []string{
		"preferred_locations",
		"employment_types",
		"excluded_companies",
		"excluded_locations",
		"excluded_industries",
	} {
		values, ok := response[field].([]string)
		if !ok {
			t.Fatalf("%s must be encoded as []string, got %T", field, response[field])
		}
		if values == nil {
			t.Fatalf("%s must be an empty slice, not nil", field)
		}
		if len(values) != 0 {
			t.Fatalf("%s expected empty slice, got %v", field, values)
		}
	}
}
