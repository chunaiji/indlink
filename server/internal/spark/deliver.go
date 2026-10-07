package spark

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"
)

type PeerInfo struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// Payload WS 推给客户端的火花事件。
//
// 中英文各推一份:WS 连接上没有 Accept-Language,服务端不知道这个客户端是什么语种,
// 让客户端按自己的 locale 挑。空串表示后台没配,客户端用内置 ARB 文案。
type Payload struct {
	Type    string   `json:"type"`
	ChatID  string   `json:"chat_id,omitempty"` // 机器人配对才有;真人配对等 /spark/accept
	PeerID  string   `json:"peer_id"`
	Peer    PeerInfo `json:"peer"`
	Title   string   `json:"title"`
	Text    string   `json:"text"`
	TitleEn string   `json:"title_en"`
	TextEn  string   `json:"text_en"`
}

// renderCopy 后台文案为空时用兜底,并把 {nickname} 换成对方昵称。
func renderCopy(tpl, fallback, nickname string) string {
	if strings.TrimSpace(tpl) == "" {
		tpl = fallback
	}
	return strings.ReplaceAll(tpl, "{nickname}", nickname)
}

func buildPayload(chatID int64, peer Candidate, title, text, titleEn, textEn string) Payload {
	p := Payload{
		Type:   "spark",
		PeerID: strconv.FormatInt(peer.UserID, 10),
		Peer: PeerInfo{
			ID: strconv.FormatInt(peer.UserID, 10), Nickname: peer.Nickname, Avatar: peer.Avatar,
		},
		Title: title, Text: text, TitleEn: titleEn, TextEn: textEn,
	}
	if chatID != 0 {
		p.ChatID = strconv.FormatInt(chatID, 10)
	}
	return p
}

const (
	fallbackTitleZh = "有人和你对上眼了"
	fallbackTextZh  = "{nickname} 与你碰撞出了火花"
	fallbackTitleEn = "Someone caught your eye"
	fallbackTextEn  = "You and {nickname} just sparked"
)

// copyFor 组一对(中/英)标题与正文。
func copyFor(tenantID int64, nickname string) (title, text, titleEn, textEn string) {
	title = renderCopy(sysconfig.GetString(tenantID, sysconfig.KeySparkTitle), fallbackTitleZh, nickname)
	text = renderCopy(sysconfig.GetString(tenantID, sysconfig.KeySparkText), fallbackTextZh, nickname)
	titleEn = renderCopy(sysconfig.GetString(tenantID, sysconfig.KeySparkTitleEN), fallbackTitleEn, nickname)
	textEn = renderCopy(sysconfig.GetString(tenantID, sysconfig.KeySparkTextEN), fallbackTextEn, nickname)
	return
}

// markPaired 记下这一对今天配过了,TTL 到当天结束。
// 它同时是 /spark/accept 的授权依据:没有这条记录就不能免费建会话。
func (s *Service) markPaired(ctx context.Context, tenantID, a, b int64, now time.Time) {
	cache.RDB.Set(ctx, pairKey(tenantID, a, b), 1, endOfDay(now))
}

// deliverReal 真人配对:**不建会话**,只给双方推弹窗。
// 预建会话会让两人的消息列表各多出一个空会话——没人说话的那种。
func (s *Service) deliverReal(ctx context.Context, tenantID int64, a, b Candidate, now time.Time) {
	s.markPaired(ctx, tenantID, a.UserID, b.UserID, now)
	ta, xa, tae, xae := copyFor(tenantID, b.Nickname)
	s.d.Push(a.UserID, buildPayload(0, b, ta, xa, tae, xae))
	tb, xb, tbe, xbe := copyFor(tenantID, a.Nickname)
	s.d.Push(b.UserID, buildPayload(0, a, tb, xb, tbe, xbe))
	log.Printf("[spark] tenant=%d 真人配对 %d <-> %d", tenantID, a.UserID, b.UserID)
}

// deliverRobot 机器人配对:先建会话、让机器人说第一句,再弹窗。
// 顺序不能反:用户点进去要看到内容,空会话会让人直接退出去。
func (s *Service) deliverRobot(ctx context.Context, tenantID int64, self Candidate,
	paired func(int64) bool, now time.Time) {
	var bots []model.User
	s.d.DB.Select("user_id, nickname, avatar").
		Where("tenant_id = ? AND is_robot = ? AND status = ?", tenantID, true, "active").
		Limit(500).Find(&bots)
	var bot *model.User
	for i := range bots {
		if !paired(bots[i].UserID) {
			bot = &bots[i]
			break
		}
	}
	if bot == nil {
		return
	}
	chatID, err := s.d.EnsureChat(tenantID, bot.UserID, self.UserID)
	if err != nil {
		log.Printf("[spark] 建会话失败 tenant=%d bot=%d user=%d: %v", tenantID, bot.UserID, self.UserID, err)
		return
	}
	if text := s.d.Opening(tenantID, bot.UserID); text != "" {
		if _, err := s.d.SendAs(tenantID, bot.UserID, chatID, text, "text"); err != nil {
			log.Printf("[spark] 开场白发送失败 chat=%d: %v", chatID, err)
		}
	}
	s.markPaired(ctx, tenantID, bot.UserID, self.UserID, now)
	peer := Candidate{UserID: bot.UserID, Nickname: bot.Nickname, Avatar: bot.Avatar}
	t, x, te, xe := copyFor(tenantID, bot.Nickname)
	s.d.Push(self.UserID, buildPayload(chatID, peer, t, x, te, xe))
	log.Printf("[spark] tenant=%d 机器人配对 bot=%d -> user=%d chat=%d", tenantID, bot.UserID, self.UserID, chatID)
}

// Accept 真人配对点击后换一个免费会话。
//
// ⚠️ **必须校验配对记录存在**:这是唯一拦住「任何人调这个接口就能和任意人免费开聊」的闸门。
// 没有它,price_chat 形同虚设。
func (s *Service) Accept(tenantID, userID, peerID int64) (int64, error) {
	if userID == peerID {
		return 0, errs.New(errs.CodeBadRequest, "不能和自己开聊")
	}
	ctx := context.Background()
	if cache.RDB.Exists(ctx, pairKey(tenantID, userID, peerID)).Val() == 0 {
		return 0, errs.New(errs.CodeForbidden, "匹配已过期")
	}
	var peer model.User
	if err := s.d.DB.Select("user_id, status").
		First(&peer, "tenant_id = ? AND user_id = ?", tenantID, peerID).Error; err != nil {
		return 0, errs.New(errs.CodeNotFound, "对方不存在")
	}
	if peer.Status != "active" {
		return 0, errs.New(errs.CodeForbidden, "对方账号不可用")
	}
	return s.d.EnsureChat(tenantID, userID, peerID)
}
