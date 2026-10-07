package robot

import (
	"encoding/json"
	"log"
	"math/rand"
	"strings"
	"sync"

	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// KeywordMatcher 加载 robot_keyword_rules，内存匹配，支持热重载。
type KeywordMatcher struct {
	mu       sync.RWMutex
	db       *gorm.DB
	tenantID int64
	rules    []keywordRule
}

type keywordRule struct {
	ruleID    int64
	priority  int
	matchType string
	keywords  []string
	responses map[string][]string // persona_role → []variant；"default" 为通用
}

func newKeywordMatcher(db *gorm.DB, tenantID int64) *KeywordMatcher {
	km := &KeywordMatcher{db: db, tenantID: tenantID}
	_ = km.Reload()
	return km
}

// Reload 从 DB 重新加载当前租户的活跃规则。
func (km *KeywordMatcher) Reload() error {
	var rows []model.RobotKeywordRule
	if err := km.db.Where("tenant_id = ? AND status = ?", km.tenantID, "active").
		Order("priority DESC, rule_id ASC").Find(&rows).Error; err != nil {
		return err
	}

	rules := make([]keywordRule, 0, len(rows))
	for _, r := range rows {
		var kws []string
		if err := json.Unmarshal([]byte(r.KeywordsJSON), &kws); err != nil {
			log.Printf("[robot/keymatcher] parse keywords rule_id=%d: %v", r.RuleID, err)
			continue
		}
		var resps map[string][]string
		if err := json.Unmarshal([]byte(r.ResponsesJSON), &resps); err != nil {
			log.Printf("[robot/keymatcher] parse responses rule_id=%d: %v", r.RuleID, err)
			continue
		}
		rules = append(rules, keywordRule{
			ruleID: r.RuleID, priority: r.Priority, matchType: r.MatchType,
			keywords: kws, responses: resps,
		})
	}

	km.mu.Lock()
	km.rules = rules
	km.mu.Unlock()
	return nil
}

// Match 对 normalized 文本按规则匹配，返回回复变体（命中则非空；未命中返回 ""）。
// personaRole 用于取对应角色的变体，无对应角色时取 "default"。
func (km *KeywordMatcher) Match(text, personaRole string) (string, int64) {
	norm := normalizeText(text)

	km.mu.RLock()
	rules := km.rules
	km.mu.RUnlock()

	for _, r := range rules {
		if !matchKeywords(norm, r.matchType, r.keywords) {
			continue
		}
		variants := r.responses[personaRole]
		if len(variants) == 0 {
			variants = r.responses["default"]
		}
		if len(variants) == 0 {
			continue
		}
		return variants[rand.Intn(len(variants))], r.ruleID
	}
	return "", 0
}

func matchKeywords(norm, matchType string, keywords []string) bool {
	for _, kw := range keywords {
		kw = normalizeText(kw)
		switch matchType {
		case "exact":
			if norm == kw {
				return true
			}
		case "prefix":
			if strings.HasPrefix(norm, kw) {
				return true
			}
		default: // contains
			if strings.Contains(norm, kw) {
				return true
			}
		}
	}
	return false
}

// IncrHit 异步更新命中计数。
func (km *KeywordMatcher) IncrHit(ruleID int64) {
	go func() {
		km.db.Model(&model.RobotKeywordRule{}).Where("rule_id = ?", ruleID).
			UpdateColumn("hit_count", gorm.Expr("hit_count + 1"))
	}()
}
