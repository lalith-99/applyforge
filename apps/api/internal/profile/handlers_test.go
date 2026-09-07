package profile

import "testing"

func TestToResponse_NormalizesNilSlicesToEmptyArrays(t *testing.T) {
	response := toResponse(Profile{})

	for _, field := range []string{
		"primary_target_titles",
		"alternative_target_titles",
		"preferred_industries",
		"preferred_technologies",
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
