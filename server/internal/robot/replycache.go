package robot

import (
	"encoding/json"
	"log"
	"math/rand"
	"slices"
	"strings"
	"time"

	"driftbottle/internal/model"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxVariants = 5
	// cacheRegenProb 变体未满时穿透缓存走 LLM 的百分比概率,用于让变体池逐步积累。
	cacheRegenProb = 40
)

// ReplyCache 操作 robot_reply_cache（Tier 2 LLM 自积累缓存）。
type ReplyCache struct {
	db *gorm.DB
}

func newReplyCache(db *gorm.DB) *ReplyCache { return &ReplyCache{db: db} }

// GetVariants 返回缓存条目的全部可用变体(过滤 AI 自白);未命中或解析失败返回 nil。
func (rc *ReplyCache) GetVariants(tenantID int64, hash, personaRole string) []string {
	var row model.RobotReplyCache
	err := rc.db.Where("tenant_id = ? AND question_hash = ? AND persona_role = ? AND status = ?",
		tenantID, hash, personaRole, "active").First(&row).Error
	if err != nil {
		return nil
	}
	var variants []string
	if e := json.Unmarshal([]byte(row.ResponsesJSON), &variants); e != nil {
		return nil
	}
	out := make([]string, 0, len(variants))
	for _, v := range variants {
		if containsAIConfession(v) || containsAssistantTone(v) {
			continue
		}
		v = sanitizeOutgoingReply(v, "")
		if v != "" && !containsAIConfession(v) {
			out = append(out, v)
		}
	}
	return out
}

// pickVariant 从 variants 随机取一条不在 exclude 里的;全被排除或为空返回 ""。
func pickVariant(variants, exclude []string) string {
	pool := make([]string, 0, len(variants))
	for _, v := range variants {
		if !slices.Contains(exclude, v) {
			pool = append(pool, v)
		}
	}
	if len(pool) == 0 {
		return ""
	}
	return pool[rand.Intn(len(pool))]
}

// AddVariant 异步将 LLM 生成的回复写入缓存（最多 maxVariants 条变体）。
func (rc *ReplyCache) AddVariant(tenantID int64, hash, personaRole, questionSample, text string) {
	text = strings.TrimSpace(text)
	if text == "" || containsAIConfession(text) || containsAssistantTone(text) {
		return // 不缓存含 AI 自白/助手腔的回复，防止缓存污染
	}
	text = sanitizeOutgoingReply(text, questionSample)
	go func() {
		var row model.RobotReplyCache
		err := rc.db.Where("tenant_id = ? AND question_hash = ? AND persona_role = ?",
			tenantID, hash, personaRole).First(&row).Error

		if err != nil {
			// 新建缓存条目
			variants, _ := json.Marshal([]string{text})
			entry := model.RobotReplyCache{
				CacheID:        idgen.Next(),
				TenantID:       tenantID,
				PersonaRole:    personaRole,
				QuestionHash:   hash,
				QuestionSample: questionSample,
				ResponsesJSON:  string(variants),
				Source:         "ai_generated",
				Status:         "active",
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			if e := rc.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; e != nil {
				log.Printf("[robot/cache] create err: %v", e)
			}
			return
		}

		// 已有条目：变体未满则追加
		var variants []string
		if e := json.Unmarshal([]byte(row.ResponsesJSON), &variants); e != nil {
			variants = []string{}
		}
		if len(variants) >= maxVariants {
			return
		}
		variants = append(variants, text)
		b, _ := json.Marshal(variants)
		if e := rc.db.Model(&row).Updates(map[string]interface{}{
			"responses_json": string(b),
			"updated_at":     time.Now(),
		}).Error; e != nil {
			log.Printf("[robot/cache] update variant err: %v", e)
		}
	}()
}

// IncrHit 异步递增命中计数。
func (rc *ReplyCache) IncrHit(tenantID int64, hash, personaRole string) {
	go func() {
		rc.db.Model(&model.RobotReplyCache{}).
			Where("tenant_id = ? AND question_hash = ? AND persona_role = ?", tenantID, hash, personaRole).
			UpdateColumn("hit_count", gorm.Expr("hit_count + 1"))
	}()
}
