// JNexus 运维平台 — By JJ Zhang, Version 1.0
package buildinfo

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// 动态版本号：优先级 ldflags 注入 > AUTOOPS_VERSION 环境变量 >
// git 提交次数（每次代码提交小版本自动 +1，如 1.88+ccab716）> VERSION 文件 > 1.0

var (
	once     sync.Once
	cached   string
	// Version 可在构建时通过 -ldflags "-X autoops/internal/buildinfo.Version=..." 覆盖
	Version string
)

// Get 返回当前版本号（结果缓存）
func Get() string {
	once.Do(func() { cached = compute() })
	return cached
}

func compute() string {
	if Version != "" {
		return Version
	}
	if v := os.Getenv("AUTOOPS_VERSION"); v != "" {
		return v
	}
	if cnt, err := git("rev-list", "--count", "HEAD"); err == nil && cnt != "" {
		return fmt.Sprintf("1.%s", cnt)
	}
	if b, err := os.ReadFile("VERSION"); err == nil && strings.TrimSpace(string(b)) != "" {
		return strings.TrimSpace(string(b))
	}
	return "1.0"
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
