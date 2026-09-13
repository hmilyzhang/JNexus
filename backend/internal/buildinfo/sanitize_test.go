package buildinfo

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"1.130":   "1.130", // valid
		"1.":      "",      // broken value exported on a machine without git
		"1":       "",      // missing fractional part
		"":        "",
		" 1.121 ": "1.121",
		"1.abc":   "",
		"2.5":     "",
	}
	for in, want := range cases {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestComputeFallbacks(t *testing.T) {
	t.Setenv("JNEXUS_VERSION", "1.")
	if got := compute(); !strings.HasPrefix(got, "1.") || got == "1" {
		t.Errorf("compute() with broken env = %q, want a valid 1.<n> fallback", got)
	}
	t.Setenv("JNEXUS_VERSION", "1.55")
	if got := compute(); got != "1.55" {
		t.Errorf("compute() with valid env = %q, want 1.55", got)
	}
}
