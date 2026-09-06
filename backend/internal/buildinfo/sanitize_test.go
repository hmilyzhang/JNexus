package buildinfo

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"1.130": "1.130", // 正常
		"1.":    "",      // 部署机无 git 导出的残缺值
		"1":     "",      // 缺小数部分
		"":      "",
		" 1.121 ": "1.121",
		"1.abc": "",
		"2.5":   "",
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
