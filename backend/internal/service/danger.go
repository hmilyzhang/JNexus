package service

import (
	"regexp"
	"sync"

	"autoops/internal/model"
)

var (
	rulesMu   sync.RWMutex
	ruleCache []compiledRule
)

type compiledRule struct {
	ID   uint
	Desc string
	Re   *regexp.Regexp
}

// LoadDangerRules 重新加载规则缓存（规则增删改后调用）
func LoadDangerRules() error {
	var rules []model.DangerRule
	if err := model.DB.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return err
	}
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue // 跳过非法正则
		}
		compiled = append(compiled, compiledRule{ID: r.ID, Desc: r.Desc, Re: re})
	}
	rulesMu.Lock()
	ruleCache = compiled
	rulesMu.Unlock()
	return nil
}

// CheckDanger 检查命令是否命中危险规则，返回命中的规则描述列表
func CheckDanger(cmd string) []string {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	var hits []string
	for _, r := range ruleCache {
		if r.Re.MatchString(cmd) {
			hits = append(hits, r.Desc)
		}
	}
	return hits
}
