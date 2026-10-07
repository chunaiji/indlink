package robot

import (
	"log"
	"time"

	"driftbottle/internal/model"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MemoryStore 操作 robot_memory 表：用户×机器人关系记忆。
type MemoryStore struct {
	db *gorm.DB
}

func newMemoryStore(db *gorm.DB) *MemoryStore { return &MemoryStore{db: db} }

// GetOrCreate 返回用户对某机器人的记忆；不存在则创建默认值。
func (m *MemoryStore) GetOrCreate(tenantID, userID, botUserID int64) model.RobotMemory {
	var mem model.RobotMemory
	err := m.db.Where("user_id = ? AND bot_user_id = ?", userID, botUserID).First(&mem).Error
	if err == nil {
		return mem
	}
	mem = model.RobotMemory{
		ID:          idgen.Next(),
		TenantID:    tenantID,
		UserID:      userID,
		BotUserID:   botUserID,
		Familiarity: 0.3,
		UpdatedAt:   time.Now(),
	}
	_ = m.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&mem).Error
	return mem
}

// UpdateAsync 异步更新记忆（familiarity 递增 +0.01，上限 0.9；追加 sessionSummary）。
func (m *MemoryStore) UpdateAsync(tenantID, userID, botUserID int64, sessionSummary string) {
	go func() {
		updates := map[string]interface{}{
			"familiarity":     gorm.Expr("LEAST(familiarity + 0.01, 0.9)"),
			"updated_at":      time.Now(),
		}
		if sessionSummary != "" {
			updates["session_summary"] = sessionSummary
		}
		if err := m.db.Model(&model.RobotMemory{}).
			Where("user_id = ? AND bot_user_id = ?", userID, botUserID).
			Updates(updates).Error; err != nil {
			log.Printf("[robot/memory] update err: %v", err)
		}
	}()
}
