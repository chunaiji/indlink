package robot

import (
	"sync"

	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// defaultPersona 当机器人未绑定人格时使用的内置默认值。
var defaultPersona = model.PersonaConfig{
	RelationshipRole: "friend",
	AffectiveStyle:   "warm_soft",
	VoiceStyle:       "casual",
	Name:             "默认",
}

// BotRegistry 缓存 botUserID → PersonaConfig 的内存映射，支持热重载。
type BotRegistry struct {
	mu   sync.RWMutex
	db   *gorm.DB
	bots map[int64]model.PersonaConfig // botUserID → persona
}

func newBotRegistry(db *gorm.DB) *BotRegistry {
	r := &BotRegistry{db: db, bots: map[int64]model.PersonaConfig{}}
	_ = r.Reload()
	return r
}

// Reload 从 DB 重新加载所有机器人的人格绑定（保存后调用）。
func (r *BotRegistry) Reload() error {
	// 取所有 is_robot=true 且有 persona_id 的用户，左连接人格表
	type row struct {
		UserID           int64
		PersonaID        *int64
		RelationshipRole string
		AffectiveStyle   string
		VoiceStyle       string
		RulesJSON        string
		PersonaName      string
	}
	var rows []row
	r.db.Raw(`
		SELECT u.user_id, u.persona_id,
		       COALESCE(p.relationship_role,'') AS relationship_role,
		       COALESCE(p.affective_style,'')   AS affective_style,
		       COALESCE(p.voice_style,'')        AS voice_style,
		       COALESCE(p.rules_json,'')         AS rules_json,
		       COALESCE(p.name,'')               AS persona_name
		FROM users u
		LEFT JOIN persona_configs p ON p.persona_id = u.persona_id AND p.status = 'active'
		WHERE u.is_robot = 1
	`).Scan(&rows)

	m := make(map[int64]model.PersonaConfig, len(rows))
	for _, ro := range rows {
		if ro.PersonaID == nil {
			m[ro.UserID] = defaultPersona
		} else {
			m[ro.UserID] = model.PersonaConfig{
				PersonaID:        *ro.PersonaID,
				RelationshipRole: ro.RelationshipRole,
				AffectiveStyle:   ro.AffectiveStyle,
				VoiceStyle:       ro.VoiceStyle,
				RulesJSON:        ro.RulesJSON,
				Name:             ro.PersonaName,
			}
		}
	}

	r.mu.Lock()
	r.bots = m
	r.mu.Unlock()
	return nil
}

// ReloadGlobal triggers BotRegistry.Reload() on the global worker pool's registry.
// Called by admin after persona changes. No-op if pool not yet initialized.
func ReloadGlobal() {
	if globalPool != nil {
		_ = globalPool.registry.Reload()
	}
}

// Get 返回机器人绑定的人格；找不到时返回默认人格。
func (r *BotRegistry) Get(botUserID int64) model.PersonaConfig {
	r.mu.RLock()
	p, ok := r.bots[botUserID]
	r.mu.RUnlock()
	if ok {
		return p
	}
	return defaultPersona
}
