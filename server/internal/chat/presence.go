package chat

import (
	"time"

	"driftbottle/internal/model"
)

// 在线状态。
//
// ⚠️ hub 是**单实例内存态**：多实例部署时它只知道连在自己身上的那些连接。
// V1 按单机部署，暂不改 Redis Pub/Sub（已知取舍，见开发进度文档）。
//
// 为了让状态在任何部署形态下都不至于「全员离线」，这里做两级判定：
//  1. hub 里有活跃 WS 连接 → 确定在线
//  2. 否则看 LastActiveAt，5 分钟内有动作 → 视为在线（降级方案）
//
// 客户端据此显示绿点或「N 分钟前活跃」。

// PresenceWindow 最近活跃多久内算在线。
const PresenceWindow = 5 * time.Minute

// PresenceItem 单个用户的在线状态。
type PresenceItem struct {
	UserID     int64     `json:"user_id,string"`
	Online     bool      `json:"online"`
	LastActive time.Time `json:"last_active_at"`
}

// Presence 批量查在线状态。一次问一批，不要每个头像发一个请求。
func (s *Service) Presence(tenantID int64, userIDs []int64) ([]PresenceItem, error) {
	if len(userIDs) == 0 {
		return []PresenceItem{}, nil
	}
	if len(userIDs) > 100 {
		userIDs = userIDs[:100]
	}

	var rows []model.User
	if err := s.db.Select("user_id, last_active_at").
		Where("tenant_id = ? AND user_id IN ?", tenantID, userIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]PresenceItem, 0, len(rows))
	for _, u := range rows {
		online := s.hub != nil && s.hub.IsOnline(u.UserID)
		if !online {
			online = time.Since(u.LastActiveAt) < PresenceWindow
		}
		out = append(out, PresenceItem{
			UserID:     u.UserID,
			Online:     online,
			LastActive: u.LastActiveAt,
		})
	}
	return out, nil
}
