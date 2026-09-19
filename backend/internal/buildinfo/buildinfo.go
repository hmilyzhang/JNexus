// JNexus Ops Platform — By JJ Zhang, Version 1.0
package buildinfo

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Dynamic version resolution, by priority: ldflags injection > JNEXUS_VERSION env var >
// VERSION file (release milestone, e.g. "2.0"; bumped per release) > git commit count
// (legacy auto counter, "2.<count>") > 2.0

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
	// VERSION next to the binary (deploy layout) or one level up (dev: backend/)
	for _, p := range []string{"VERSION", "../VERSION"} {
		if b, err := os.ReadFile(p); err == nil {
			if v := sanitize(strings.TrimSpace(string(b))); v != "" {
				return v
			}
		}
	}
	// legacy fallback for checkouts without a VERSION file: the old commit counter
	if cnt, err := git("rev-list", "--count", "HEAD"); err == nil && cnt != "" {
		return "2." + cnt
	}
	return "2.0"
}

// sanitize cleans the version value: accepts "major.minor" (e.g. 2.0, 1.303);
// rejects broken values like "2." / "2" exported on machines without git
func sanitize(v string) string {
	v = strings.TrimSpace(v)
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return ""
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
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
