// JNexus Ops Platform — By JJ Zhang, Version 1.0
package buildinfo

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Dynamic version resolution, by priority: ldflags injection > JNEXUS_VERSION env var >
// git commit count (minor version auto +1 per commit, e.g. 1.88+ccab716) > VERSION file > 1.0

var (
	once   sync.Once
	cached string
	// Version can be overridden at build time via -ldflags "-X jnexus/internal/buildinfo.Version=..."
	Version string
)

// Get returns the current version (result cached)
func Get() string {
	once.Do(func() { cached = compute() })
	return cached
}

func compute() string {
	if v := sanitize(Version); v != "" {
		return v
	}
	if v := sanitize(os.Getenv("JNEXUS_VERSION")); v != "" {
		return v
	}
	if cnt, err := git("rev-list", "--count", "HEAD"); err == nil && cnt != "" {
		return fmt.Sprintf("1.%s", cnt)
	}
	if b, err := os.ReadFile("VERSION"); err == nil {
		if v := sanitize(strings.TrimSpace(string(b))); v != "" {
			return v
		}
	}
	return "1.0"
}

// sanitize cleans the version value: must be a complete 1.<number> (rejects broken "1." / "1" exported on machines without git)
func sanitize(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "1.") {
		return ""
	}
	digits := strings.TrimPrefix(v, "1.")
	if digits == "" {
		return ""
	}
	if _, err := strconv.Atoi(digits); err != nil {
		return ""
	}
	return v
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
