// Package notify 站内通知(#4):别人回信/互动后落库 + 在线 WS 实时推 + 红点未读数。
// 微信「订阅消息」(应用关闭也能收到)留有下发钩子,需生产环境模板 ID + access_token,见 SendWxSubscribe。
package notify

import (
	"log"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"

	"gorm.io/gorm"
)

// Pusher 由 chat.Hub 实现,用于在线实时推送(松耦合,避免反向依赖 chat)。
type Pusher interface {
	IsOnline(userID int64) bool
	PushTo(userID int64, payload interface{})
}

type Service struct {
	db  *gorm.DB
	hub Pusher
}

func New(db *gorm.DB, hub Pusher) *Service { return &Service{db: db, hub: hub} }

// Create 落库一条通知,并在用户在线时 WS 实时推送。
func (s *Service) Create(tenantID, userID int64, typ string, refID int64, title, body string) {
	n := model.Notification{
		TenantID: tenantID, UserID: userID, Type: typ, RefID: refID,
		Title: title, Body: body, Read: false, CreatedAt: time.Now(),
	}
	if err := s.db.Create(&n).Error; err != nil {
		log.Printf("[notify] 落库失败: %v", err)
		return
	}
	if s.hub != nil && s.hub.IsOnline(userID) {
		s.hub.PushTo(userID, map[string]interface{}{
			"type": "notify", "notify_type": typ, "ref_id": n.RefID, "title": title, "body": body,
		})
	}
	// 微信订阅消息(离线触达)——配置了模板才尝试下发
	s.SendWxSubscribe(tenantID, userID, title, body)
}

// OnBottleReplied 供 bottle 模块回调:有人回信 -> 通知瓶主(回信者自己不通知)。
func (s *Service) OnBottleReplied(tenantID, ownerID, bottleID, replierID int64) {
	if ownerID == replierID {
		return
	}
	s.Create(tenantID, ownerID, "reply", bottleID, "收到新回信 💌", "有人在你的漂流瓶里留言了,去看看吧")
}

// OnBottleScooped 供 bottle 模块回调:瓶子当天首次被捞 -> 通知瓶主(捞瓶人不暴露身份)。
func (s *Service) OnBottleScooped(tenantID, ownerID, bottleID, viewerID int64) {
	if ownerID == viewerID {
		return
	}
	s.Create(tenantID, ownerID, "scoop", bottleID, "瓶子被捞起了 🌊", "你的漂流瓶今天被人捞起,看看它漂到哪了")
}

// SendWxSubscribe 微信订阅消息下发(钩子)。生产需:模板 ID(sysconfig)+ 用户一次性授权 + access_token。
// 当前 dev 无 access_token 基建,配置了模板也仅记录,待 #4 后续接 token 服务。
func (s *Service) SendWxSubscribe(tenantID, userID int64, title, body string) {
	tmpl := sysconfig.GetString(tenantID, sysconfig.KeyNotifyWxTemplate)
	if tmpl == "" {
		return
	}
	log.Printf("[notify] (stub) 微信订阅消息待下发 user=%d tmpl=%s title=%s", userID, tmpl, title)
	// TODO: 取 access_token -> POST cgi-bin/message/subscribe/send(校验用户授权额度)
}

// ---- 查询 ----

func (s *Service) List(userID int64, page, size int) []model.Notification {
	if size <= 0 || size > 50 {
		size = 20
	}
	var list []model.Notification
	s.db.Where("user_id = ?", userID).Order("created_at desc").
		Offset((page - 1) * size).Limit(size).Find(&list)
	return list
}

func (s *Service) UnreadCount(userID int64) int64 {
	var n int64
	s.db.Model(&model.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&n)
	return n
}

// MarkRead id<=0 表示全部已读。
func (s *Service) MarkRead(userID, id int64) {
	q := s.db.Model(&model.Notification{}).Where("user_id = ?", userID)
	if id > 0 {
		q = q.Where("id = ?", id)
	}
	q.Update("is_read", true)
}
