// JNexus Ops Platform — By JJ Zhang, Version 1.0

package service

import (
	"regexp"
	"sync"

	"jnexus/internal/model"
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

// LoadDangerRules reloads the rule cache (called after rules are added/removed/modified)
func LoadDangerRules() error {
	var rules []model.DangerRule
	if err := model.DB.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return err
	}
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue // skip invalid regex
		}
		compiled = append(compiled, compiledRule{ID: r.ID, Desc: r.Desc, Re: re})
	}
	rulesMu.Lock()
	ruleCache = compiled
	rulesMu.Unlock()
	return nil
}

// CheckDanger checks whether a command matches any danger rule and returns the matched rule descriptions
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
