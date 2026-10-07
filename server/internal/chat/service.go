package chat

import (
	gocontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/moderation"
	"driftbottle/internal/rank"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/cache"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// Verifier 解耦 user 模块的认证校验。
type Verifier interface {
	EnsureVerifiedIfRequired(tenantID, userID int64) error
}

// BotEnqueueFn 机器人回复投队列的函数签名，由 robot 包注入，避免循环依赖。
type BotEnqueueFn func(tenantID, chatID, botUserID, userID int64, userMsg string)

// MsgPushFn 新消息推送回调，由 push 包注入，避免循环依赖。
type MsgPushFn func(tenantID, recipientID int64)

type Service struct {
	db         *gorm.DB
	wlt        *wallet.Service
	mod        *moderation.Service
	verf       Verifier
	hub        *Hub
	botEnqueue BotEnqueueFn
	onMsgPush  MsgPushFn
}

func New(db *gorm.DB, wlt *wallet.Service, mod *moderation.Service, verf Verifier, hub *Hub) *Service {
	return &Service{db: db, wlt: wlt, mod: mod, verf: verf, hub: hub}
}

// SetBotEnqueue 注入机器人回复投队列函数（启动后由 robot 包调用一次）。
func (s *Service) SetBotEnqueue(fn BotEnqueueFn) { s.botEnqueue = fn }

// SetOnMsgPush 注入新消息推送回调（启动后由 push 包调用一次）。
func (s *Service) SetOnMsgPush(fn MsgPushFn) { s.onMsgPush = fn }

func (s *Service) Hub() *Hub { return s.hub }

// StartChat 开聊:认证校验 -> 拉黑校验 -> 已存在则直接返回 -> 扣金币并建会话。
// sourceBottle 可为 0(来自同城/扩列)或瓶子 ID(来自回信)。
func (s *Service) StartChat(tenantID, userID, targetID, sourceBottle int64) (*model.Chat, error) {
	if userID == targetID {
		return nil, errs.New(errs.CodeBadRequest, "不能和自己开聊")
	}
	if err := s.verf.EnsureVerifiedIfRequired(tenantID, userID); err != nil {
		return nil, err
	}
	if s.mod.IsBlocked(userID, targetID) {
		return nil, errs.ErrChatBlocked
	}

	// 已有会话直接返回,不重复扣费
	if existing, err := s.findChat(tenantID, userID, targetID); err == nil && existing != nil {
		return existing, nil
	}

	a, b := order(userID, targetID)
	price := sysconfig.GetInt64(tenantID, sysconfig.KeyPriceChat)
	bizNo := "chat:" + strconv.FormatInt(idgen.Next(), 10)

	chat := &model.Chat{
		ChatID: idgen.Next(), TenantID: tenantID, UserA: a, UserB: b, SourceBottle: sourceBottle,
		RelationStage: "stranger", UpdatedAt: time.Now(), CreatedAt: time.Now(),
	}
	err := s.wlt.Debit(tenantID, userID, price, wallet.SceneChat, bizNo, func(tx *gorm.DB) error {
		return tx.Create(chat).Error
	})
	if err != nil {
		return nil, err
	}
	return chat, nil
}

func (s *Service) findChat(tenantID, u1, u2 int64) (*model.Chat, error) {
	a, b := order(u1, u2)
	var chat model.Chat
	err := s.db.First(&chat, "tenant_id = ? AND user_a = ? AND user_b = ?", tenantID, a, b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

// SendMessage 发消息:审核 -> 落库(只写一次)-> 更新会话 -> WS 推送对方。
// SendGift 聊天内送礼:库存先抵扣、差额扣币 → 每件记一条 item_order(target=对方)
// → 收礼方魅力值+ → 插入 gift 消息 → 推送对方。
// maxGiftQty 单次连送上限。客户端把连送合并成一条消息展示，
// 不设上限的话一次请求就能清空库存，且礼物气泡的数字会失控。
const maxGiftQty = 99

// SendGift 赠送礼物。qty 为连送数量，<=0 按 1 处理。
func (s *Service) SendGift(tenantID, senderID, chatID, itemID int64, qty int) (*model.Message, error) {
	if qty <= 0 {
		qty = 1
	}
	if qty > maxGiftQty {
		return nil, errs.New(errs.CodeBadRequest, "一次最多赠送 99 个")
	}
	var chat model.Chat
	if err := s.db.First(&chat, "tenant_id = ? AND chat_id = ?", tenantID, chatID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "会话不存在")
	}
	if chat.UserA != senderID && chat.UserB != senderID {
		return nil, errs.New(errs.CodeForbidden, "无权赠送")
	}
	other := chat.UserA
	if other == senderID {
		other = chat.UserB
	}
	if s.mod.IsBlocked(senderID, other) {
		return nil, errs.ErrChatBlocked
	}

	var it model.Item
	if err := s.db.First(&it, "item_id = ? AND status = ?", itemID, "active").Error; err != nil {
		return nil, errs.New(errs.CodeBadRequest, "礼物不存在")
	}

	// gift 消息内容:JSON(名称/图标/金币数/数量),前端据此渲染礼物气泡
	body, _ := json.Marshal(map[string]interface{}{
		"item_id": it.ItemID, "name": it.Name, "icon": it.Icon,
		"coins": it.PriceCoin, "qty": qty,
	})
	msg := &model.Message{
		MessageID: idgen.Next(), TenantID: tenantID, ChatID: chatID, SenderID: senderID,
		Content: string(body), Type: "gift", ReadStatus: false, CreatedAt: time.Now(),
	}
	lastMsg := "[礼物] " + it.Name
	if qty > 1 {
		lastMsg += " ×" + strconv.Itoa(qty)
	}
	// 库存先抵扣，不够的部分再扣币:手里有 ×1 就不该再为这一个付一次钱(真机反馈)。
	// 库存数在事务外先数一遍算出差额和要扣的币;事务内再按这个数删库存行,
	// 并发把库存花掉了就 RowsAffected 不够 → 整单回滚,不会出现"扣了币又吞了库存"。
	var owned int64
	s.db.Model(&model.ItemOrder{}).
		Where("user_id = ? AND item_id = ? AND target_id = 0", senderID, itemID).Count(&owned)
	useStock := int64(qty)
	if owned < useStock {
		useStock = owned
	}
	payQty := int64(qty) - useStock
	bizNo := "gift:" + strconv.FormatInt(msg.MessageID, 10)
	err := s.wlt.Debit(tenantID, senderID, it.PriceCoin*payQty, wallet.SceneGift, bizNo, func(tx *gorm.DB) error {
		if useStock > 0 {
			res := tx.Where("user_id = ? AND item_id = ? AND target_id = 0", senderID, itemID).
				Order("id asc").Limit(int(useStock)).Delete(&model.ItemOrder{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected < useStock {
				// 专属错误码:前端据此弹「去道具页购买」引导,而不是通用错误 toast。
				return errs.New(errs.CodeItemInsufficient, "礼物数量不足")
			}
		}
		// 每件送出都记一条 target=对方 的 ItemOrder(库存抵扣的也记):
		// 礼物墙 / 魅力周榜从 ItemOrder(target) 聚合,聊天送的礼不记就从墙上消失了。
		now := time.Now()
		for i := 0; i < qty; i++ {
			if err := tx.Create(&model.ItemOrder{
				ID: idgen.Next(), TenantID: tenantID, UserID: senderID, ItemID: itemID, TargetID: other,
				Coins: it.PriceCoin, CreatedAt: now,
			}).Error; err != nil {
				return err
			}
		}
		if it.PriceCoin > 0 {
			if err := tx.Model(&model.User{}).Where("user_id = ?", other).
				UpdateColumn("charm", gorm.Expr("charm + ?", it.PriceCoin*int64(qty))).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		return tx.Model(&model.Chat{}).Where("chat_id = ?", chatID).
			Updates(map[string]interface{}{
				"last_message": lastMsg,
				"last_type":    "gift",
				"updated_at":   time.Now(),
			}).Error
	})
	if err != nil {
		return nil, err
	}
	rank.InvalidateWeekCache(tenantID) // 周榜即时更新

	s.hub.PushTo(other, map[string]interface{}{
		"event": "message", "chat_id": strconv.FormatInt(chatID, 10), "message": msg,
	})
	if s.onMsgPush != nil {
		go s.onMsgPush(tenantID, other)
	}
	return msg, nil
}

func (s *Service) SendMessage(tenantID, senderID, chatID int64, content, msgType string) (*model.Message, error) {
	var chat model.Chat
	if err := s.db.First(&chat, "tenant_id = ? AND chat_id = ?", tenantID, chatID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "会话不存在")
	}
	if chat.UserA != senderID && chat.UserB != senderID {
		return nil, errs.New(errs.CodeForbidden, "无权发送")
	}
	other := chat.UserA
	if other == senderID {
		other = chat.UserB
	}
	if s.mod.IsBlocked(senderID, other) {
		return nil, errs.ErrChatBlocked
	}
	var sender model.User
	if err := s.db.Select("is_muted,is_robot").First(&sender, senderID).Error; err == nil {
		if sender.IsMuted && !sender.IsRobot {
			return nil, errs.New(errs.CodeForbidden, "已被禁言，无法发送消息")
		}
	}
	if msgType == "" || msgType == "text" {
		if err := s.mod.CheckUGC(tenantID, senderID, moderation.SceneComment, content); err != nil {
			return nil, err
		}
		msgType = "text"
	}

	msg := &model.Message{
		MessageID: idgen.Next(), TenantID: tenantID, ChatID: chatID, SenderID: senderID,
		Content: content, Type: msgType, ReadStatus: false, CreatedAt: time.Now(),
	}
	persist := func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		return tx.Model(&model.Chat{}).Where("chat_id = ?", chatID).
			Updates(map[string]interface{}{
				"last_message": truncate(content, 100),
				"last_type":    msgType,
				"updated_at":   time.Now(),
			}).Error
	}

	// 规则:每条扣 N(price_msg),发送方在这个会话里的前 L 条(chat_free_msgs)免费。
	price := sysconfig.GetInt64(tenantID, sysconfig.KeyPriceMsg)
	var err error
	if price > 0 && !sender.IsRobot && !s.withinFreeMsgs(tenantID, chatID, senderID) {
		// 真人发消息：扣费与落库同事务，余额不足透传 ErrInsufficient
		err = s.wlt.Debit(tenantID, senderID, price, wallet.SceneMsg, "msg:"+strconv.FormatInt(msg.MessageID, 10), persist)
	} else {
		err = s.db.Transaction(persist)
	}
	if err != nil {
		return nil, err
	}

	// 实时推送对方（chat_id 序列化为字符串，避免 JS int64 精度丢失）
	s.hub.PushTo(other, map[string]interface{}{
		"event": "message", "chat_id": strconv.FormatInt(chatID, 10), "message": msg,
	})

	// 真人发消息时触发微信订阅推送（机器人发的不推）
	if s.onMsgPush != nil && !sender.IsRobot {
		go s.onMsgPush(tenantID, other)
	}

	// 若对方是机器人且 AI 聊天已启用，异步触发路径 C 回复
	if s.botEnqueue != nil {
		var partner model.User
		if s.db.First(&partner, "user_id = ?", other).Error == nil && partner.IsRobot {
			s.botEnqueue(tenantID, chatID, other, senderID, content)
		}
	}

	// 真人发消息后:余额首次跌破阈值软提示
	if !sender.IsRobot {
		s.maybeLowBalanceNotice(tenantID, senderID, chatID)
	}

	return msg, nil
}

// withinFreeMsgs 这条消息是否还在「前 L 条免费」额度内:按会话、按发送方数已发的非系统消息。
// 数的是落库前的条数,所以第 L+1 条开始扣。L 为 0 直接 false,省一次查询。
func (s *Service) withinFreeMsgs(tenantID, chatID, senderID int64) bool {
	free := sysconfig.GetInt64(tenantID, sysconfig.KeyChatFreeMsgs)
	if free <= 0 {
		return false
	}
	var sent int64
	s.db.Model(&model.Message{}).
		Where("chat_id = ? AND sender_id = ? AND type <> ?", chatID, senderID, "system").
		Count(&sent)
	return sent < free
}

// maybeLowBalanceNotice 真人发消息后,若余额首次跌破阈值,插入并推送一条系统消息。
// 标记按「会话」维度 lowbal_notified:{tenant}:{user}:{chat},即每个聊天框各提示一次,
// 换个聊天框也会提示(更大概率触达);该会话余额回升到阈值以上时清本会话标记。
func (s *Service) maybeLowBalanceNotice(tenantID, userID, chatID int64) {
	// 聊天按条不收费(price_msg=0)时这条提示是误导:用户看到"余额不足"却发现消息照发、余额不动,
	// 会以为扣费坏了(真机反馈)。只有真的按条扣币时才提示。
	if sysconfig.GetInt64(tenantID, sysconfig.KeyPriceMsg) <= 0 {
		return
	}
	threshold := sysconfig.GetInt64(tenantID, sysconfig.KeyLowBalanceThreshold)
	if threshold <= 0 {
		return
	}
	bal, err := s.wlt.Balance(userID)
	if err != nil {
		return
	}
	ctx := gocontext.Background()
	key := fmt.Sprintf("lowbal_notified:%d:%d:%d", tenantID, userID, chatID)
	if bal >= threshold {
		cache.RDB.Del(ctx, key) // 余额充足:清标记,下次跌破可再提示
		return
	}
	// 余额不足:仅首次(SetNX 成功)提示
	ok, e := cache.RDB.SetNX(ctx, key, 1, 7*24*time.Hour).Result()
	if e != nil || !ok {
		return
	}
	msg := sysconfig.GetString(tenantID, sysconfig.KeyLowBalanceMsg)
	if msg == "" {
		return
	}
	s.pushSystemMessage(tenantID, chatID, userID, msg)
}

// pushSystemMessage 插入一条 system 消息(sender_id=0,不计费)并 WS 推给指定用户。
func (s *Service) pushSystemMessage(tenantID, chatID, toUser int64, content string) {
	msg := &model.Message{
		MessageID: idgen.Next(), TenantID: tenantID, ChatID: chatID, SenderID: 0,
		Content: content, Type: "system", ReadStatus: false, CreatedAt: time.Now(),
	}
	if err := s.db.Create(msg).Error; err != nil {
		return
	}
	s.hub.PushTo(toUser, map[string]interface{}{
		"event": "message", "chat_id": strconv.FormatInt(chatID, 10), "message": msg,
	})
}

// HistoryRecent 取最近 limit 条消息（正序），供机器人 prompt 构建用。
func (s *Service) HistoryRecent(chatID int64, limit int) ([]model.Message, error) {
	var list []model.Message
	err := s.db.Where("chat_id = ?", chatID).Order("created_at desc").Limit(limit).Find(&list).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// ChatItem 会话列表条目(含对方用户信息)。
type ChatItem struct {
	model.Chat
	PartnerID       int64  `json:"partner_id,string"`
	PartnerNickname string `json:"partner_nickname"`
	PartnerAvatar   string `json:"partner_avatar"`
	PartnerVerified bool   `json:"partner_verified"`
	UnreadCount     int64  `json:"unread_count"`
}

// PartnerBrief 取会话中"对方"的 ID 与昵称头像。
//
// 单条会话用(如刚建会话后要立刻渲染聊天页标题);列表场景走 ListChatsWithPartner 的批量版本。
func (s *Service) PartnerBrief(chat *model.Chat, myID int64) (int64, string, string) {
	pid := chat.UserA
	if pid == myID {
		pid = chat.UserB
	}
	var u model.User
	if err := s.db.Select("nickname, avatar").First(&u, "user_id = ?", pid).Error; err != nil {
		return pid, "", ""
	}
	return pid, u.Nickname, u.Avatar
}

// ListChatsWithPartner 返回含对方昵称/头像的会话列表。
func (s *Service) ListChatsWithPartner(tenantID, userID int64, page, size int) ([]ChatItem, error) {
	chats, err := s.ListChats(tenantID, userID, page, size)
	if err != nil || len(chats) == 0 {
		return nil, err
	}
	pids := make([]int64, 0, len(chats))
	chatIDs := make([]int64, 0, len(chats))
	for _, c := range chats {
		pid := c.UserB
		if c.UserB == userID {
			pid = c.UserA
		}
		pids = append(pids, pid)
		chatIDs = append(chatIDs, c.ChatID)
	}
	var users []model.User
	s.db.Where("user_id IN (?)", pids).Find(&users)
	um := make(map[int64]model.User, len(users))
	for _, u := range users {
		um[u.UserID] = u
	}
	// 每条会话的未读消息数
	var unreadRows []struct {
		ChatID      int64
		UnreadCount int64
	}
	s.db.Model(&model.Message{}).
		Select("chat_id, count(*) as unread_count").
		Where("chat_id IN ? AND sender_id != ? AND read_status = ?", chatIDs, userID, false).
		Group("chat_id").Scan(&unreadRows)
	unreadMap := make(map[int64]int64, len(unreadRows))
	for _, r := range unreadRows {
		unreadMap[r.ChatID] = r.UnreadCount
	}
	items := make([]ChatItem, len(chats))
	for i, c := range chats {
		pid := c.UserB
		if c.UserB == userID {
			pid = c.UserA
		}
		p := um[pid]
		items[i] = ChatItem{Chat: c, PartnerID: pid, PartnerNickname: p.Nickname, PartnerAvatar: p.Avatar, PartnerVerified: p.IsVerified, UnreadCount: unreadMap[c.ChatID]}
	}
	return items, nil
}

// ListChats 会话列表(按更新时间倒序)。
func (s *Service) ListChats(tenantID, userID int64, page, size int) ([]model.Chat, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	var list []model.Chat
	err := s.db.Where("tenant_id = ? AND (user_a = ? OR user_b = ?)", tenantID, userID, userID).
		Order("updated_at desc").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, err
}

// History 消息历史(分页,倒序取再翻转)。
func (s *Service) History(tenantID, userID, chatID int64, page, size int) ([]model.Message, error) {
	var chat model.Chat
	if err := s.db.First(&chat, "tenant_id = ? AND chat_id = ?", tenantID, chatID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "会话不存在")
	}
	if chat.UserA != userID && chat.UserB != userID {
		return nil, errs.New(errs.CodeForbidden, "无权查看")
	}
	if size <= 0 || size > 50 {
		size = 20
	}
	var list []model.Message
	err := s.db.Where("chat_id = ?", chatID).Order("created_at desc").
		Offset((page - 1) * size).Limit(size).Find(&list).Error
	if err != nil {
		return nil, err
	}
	// 翻转为正序
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	// 标记对方发来的消息为已读
	s.db.Model(&model.Message{}).Where("chat_id = ? AND sender_id <> ? AND read_status = ?", chatID, userID, false).
		Update("read_status", true)
	return list, nil
}

// UnreadCount 统计当前用户所有会话中,对方发来的未读消息数(#2)。
func (s *Service) UnreadCount(tenantID, userID int64) int64 {
	chatIDs := s.db.Model(&model.Chat{}).Select("chat_id").
		Where("tenant_id = ? AND (user_a = ? OR user_b = ?)", tenantID, userID, userID)
	var n int64
	s.db.Model(&model.Message{}).
		Where("chat_id IN (?) AND sender_id <> ? AND read_status = ?", chatIDs, userID, false).
		Count(&n)
	return n
}

// EnsureFreeChat 找到或创建两人的会话，**不走钱包**。
//
// 与 StartChat 的差别就是这一点：StartChat 按 price_chat 扣币，这里不扣。
// 两个系统发起的会话用它：机器人主动触达、火花匹配。都是系统把人推到用户面前，
// 让用户为此付费说不过去。free_chat_test.go 用 AST 盯着这个函数体里不出现钱包调用。
func (s *Service) EnsureFreeChat(tenantID, u1, u2 int64) (int64, error) {
	if existing, err := s.findChat(tenantID, u1, u2); err == nil && existing != nil {
		return existing.ChatID, nil
	}
	a, b := order(u1, u2)
	chat := &model.Chat{
		ChatID: idgen.Next(), TenantID: tenantID, UserA: a, UserB: b,
		RelationStage: "stranger", UpdatedAt: time.Now(), CreatedAt: time.Now(),
	}
	if err := s.db.Create(chat).Error; err != nil {
		return 0, err
	}
	return chat.ChatID, nil
}

// EnsureRobotChat 机器人主动触达的入口，保留原名和参数含义。
func (s *Service) EnsureRobotChat(tenantID, botUserID, userID int64) (int64, error) {
	return s.EnsureFreeChat(tenantID, botUserID, userID)
}

func order(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
