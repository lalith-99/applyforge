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

func TestValidReusableProfileURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		value        *string
		requiredHost string
		want         bool
	}{
		{name: "missing stays manual", value: nil, requiredHost: "linkedin.com", want: true},
		{name: "blank stays manual", value: stringPtr("  "), requiredHost: "linkedin.com", want: true},
		{name: "linkedin profile", value: stringPtr("https://www.linkedin.com/in/example"), requiredHost: "linkedin.com", want: true},
		{name: "linkedin subdomain", value: stringPtr("https://uk.linkedin.com/in/example"), requiredHost: "linkedin.com", want: true},
		{name: "linkedin lookalike", value: stringPtr("https://linkedin.com.evil.example/in/example"), requiredHost: "linkedin.com", want: false},
		{name: "github profile", value: stringPtr("https://github.com/example"), requiredHost: "github.com", want: true},
		{name: "github lookalike", value: stringPtr("https://github.com.evil.example/example"), requiredHost: "github.com", want: false},
		{name: "wrong profile host", value: stringPtr("https://example.com/profile"), requiredHost: "linkedin.com", want: false},
		{name: "portfolio https", value: stringPtr("https://example.dev"), requiredHost: "", want: true},
		{name: "portfolio http", value: stringPtr("http://example.dev/work"), requiredHost: "", want: true},
		{name: "javascript rejected", value: stringPtr("javascript:alert(1)"), requiredHost: "", want: false},
		{name: "relative rejected", value: stringPtr("/profile"), requiredHost: "", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validReusableProfileURL(tc.value, tc.requiredHost); got != tc.want {
				t.Fatalf("validReusableProfileURL(%v, %q) = %v, want %v", tc.value, tc.requiredHost, got, tc.want)
			}
		})
	}
}

func stringPtr(value string) *string { return &value }
