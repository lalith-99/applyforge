package applications

import "testing"

func TestValidReusableBinaryAnswer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value *string
		want  bool
	}{
		{name: "missing stays manual", value: nil, want: true},
		{name: "yes", value: stringPtr("yes"), want: true},
		{name: "no", value: stringPtr("no"), want: true},
		{name: "case and whitespace", value: stringPtr(" YES "), want: true},
		{name: "blank stays manual", value: stringPtr(""), want: true},
		{name: "h1b is ambiguous", value: stringPtr("H-1B"), want: false},
		{name: "sentence is not canonical", value: stringPtr("H-1B transfer required"), want: false},
		{name: "boolean-like text is not approved canonical value", value: stringPtr("true"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validReusableBinaryAnswer(tc.value); got != tc.want {
				t.Fatalf("validReusableBinaryAnswer(%v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func stringPtr(value string) *string { return &value }
