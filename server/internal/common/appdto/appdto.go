// Package appdto 把库表模型转成 App 端 UI 需要的形状。
//
// 为什么要这一层：后端模型是小程序时代长出来的——主键叫 user_id / bottle_id、
// tags 是逗号分隔串、媒体是单个 media_url、作者信息平铺成 author_gender/author_age。
// 而 App 的界面按 V1 原型做，要的是 id、tags 数组、images 数组、author 嵌套对象。
//
// 两边都没错，只是长在不同年代。与其让客户端每个页面各拼一遍（必然到处漏），
// 不如在出口统一转一次。**小程序路径不经过这里**，按 JWT 的 platform 分派，零影响。
package appdto

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/model"
	"driftbottle/pkg/geodist"
)

// itoa ID 一律以字符串下发——int64 超出 JS/Dart 的安全整数范围，直接下发会丢精度。
func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// splitComma 拆逗号分隔串并丢掉空项。
//
// 返回非 nil 空切片：nil 会被序列化成 JSON null，而客户端拿数组字段。
func splitComma(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// User App 端的用户结构。
//
// gender 仍下发 int8（0未知/1男/2女）、language 与 interests 仍是逗号分隔串——
// 客户端的 Gender.parse 与 stringList 本来就认这两种形状，没必要在这里多翻译一道。
// 真正要改的只有主键名：客户端一律用 id。
type User struct {
	ID             string    `json:"id"`
	Nickname       string    `json:"nickname"`
	Avatar         string    `json:"avatar"`
	Bio            string    `json:"bio"`
	Gender         int8      `json:"gender"`
	Age            int       `json:"age"`
	City           string    `json:"city"`
	Charm          int64     `json:"charm"`
	FollowingCount int64     `json:"following_count"` // 我关注的人数(Relation user_a=我,type=like)
	FollowerCount  int64     `json:"follower_count"`  // 关注我的人数(Relation user_b=我,type=like)
	BottleCount    int64     `json:"bottle_count"`    // 我扔过的瓶子数(「我的瓶子」入口显示)
	MomentCount    int64     `json:"moment_count"`    // 我发过的动态数
	Language       string    `json:"language"`
	Interests      string    `json:"interests"`
	Status         string    `json:"status"`
	AnonymousLevel int8      `json:"anonymous_level"`
	IsVerified     bool      `json:"is_verified"`
	CreatedAt      time.Time `json:"created_at"`
	LastActiveAt   time.Time `json:"last_active_at"`

	// GoogleBound / AppleBound 供「账号与安全」页显示绑定状态。
	// **只下发布尔值，绝不下发 sub 本身**——那是第三方的用户标识,
	// 前端没有任何用途,泄露出去只会扩大攻击面。
	GoogleBound bool `json:"google_bound"`
	AppleBound  bool `json:"apple_bound"`
	WechatBound bool `json:"wechat_bound"`
	AlipayBound bool `json:"alipay_bound"`

	// MaskedPhone / MaskedEmail 供「账号与安全」页展示已绑定的登录标识。
	// **一律脱敏下发**——这一屏虽是登录态才能进,但截屏外泄是常态,
	// 明文手机号/邮箱泄出去等于把账号找回入口一并交出去。空=未绑定。
	MaskedPhone string `json:"masked_phone,omitempty"`
	MaskedEmail string `json:"masked_email,omitempty"`

	// Birthday 只在看自己的资料时下发(FromUserSelf);看别人只给 Age。
	Birthday string `json:"birthday,omitempty"`
}

// FromUserSelf 自己的资料:在 FromUser 之上补生日(别人的资料不下发生日)。
func FromUserSelf(u *model.User) User {
	d := FromUser(u)
	if u != nil {
		d.Birthday = u.Birthday
	}
	return d
}

func FromUser(u *model.User) User {
	if u == nil {
		return User{}
	}
	return User{
		ID:             itoa(u.UserID),
		Nickname:       u.Nickname,
		Avatar:         u.Avatar,
		Bio:            u.Bio,
		Gender:         u.Gender,
		Age:            u.Age,
		City:           u.City,
		Charm:          u.Charm,
		Language:       u.Language,
		Interests:      u.Interests,
		Status:         u.Status,
		AnonymousLevel: u.AnonymousLevel,
		IsVerified:     u.IsVerified,
		CreatedAt:      u.CreatedAt,
		LastActiveAt:   u.LastActiveAt,
		GoogleBound:    u.GoogleSub != nil,
		AppleBound:     u.AppleSub != nil,
		WechatBound:    u.WxOpenID != "",
		AlipayBound:    u.AlipayUID != "",
		MaskedPhone:    maskTail(u.Phone),
		MaskedEmail:    maskEmail(u.Email),
	}
}

// maskTail 脱敏手机号(E.164)：保留头 3 尾 3，中间打点。空串原样返回。
func maskTail(s string) string {
	r := []rune(s)
	if len(r) <= 6 {
		return s
	}
	return string(r[:3]) + "••••" + string(r[len(r)-3:])
}

// maskEmail 脱敏邮箱：本地部分保留头 2 尾 1，域名照旧（me•••a@gmail.com）。
func maskEmail(e string) string {
	at := strings.IndexByte(e, '@')
	if at <= 0 {
		return e
	}
	local := []rune(e[:at])
	domain := e[at:]
	if len(local) <= 2 {
		return string(local[:1]) + "•••" + domain
	}
	return string(local[:2]) + "•••" + string(local[len(local)-1:]) + domain
}

// Author 瓶子 / 动态 / 评论的作者摘要。三处共用，但**填充程度不同**：
//
//   - 瓶子：只给性别年龄，**不给昵称头像**。这是产品硬约束不是字段遗漏——
//     捞到的瓶子必须保持匿名，能认出人就毁了整个玩法
//   - 动态、评论：实名，给昵称头像
//
// 客户端一律用 UserBrief 接，缺的字段它自己兜底（空头像渲染成 emoji）。
type Author struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	Gender   int8   `json:"gender"`
	Age      int    `json:"age,omitempty"`
}

// Bottle App 端的瓶子结构。
type Bottle struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Author     Author    `json:"author"`
	CreatedAt  time.Time `json:"created_at"`
	ExpireAt   time.Time `json:"expire_at"`
	Images     []string  `json:"images"`
	Tags       []string  `json:"tags"`
	City       string    `json:"city"`
	Status     string    `json:"status"`
	ReplyCount int       `json:"reply_count"`
	LikeCount  int       `json:"like_count"`

	// 以下四个库表里没有：ViewCount/CityCount 来自 MatchLog 聚合，
	// Liked 来自 MatchLog(action=like)，Collected 来自 Collection 表。
	//
	// ⚠️ 现状：只有 /bottle/mine 传了真实的 ViewCount/CityCount（它本来就批量聚合过）。
	// 其余路径这四个字段**恒为零值**——捞瓶场景下 liked/collected 天然是 false 不影响，
	// 但详情页的收藏按钮会一直显示未收藏。补齐要在 service 层加批量查询，见 APP_V1_PROGRESS。
	ViewCount int64 `json:"view_count"`
	CityCount int64 `json:"city_count"`
	Liked     bool  `json:"liked"`
	Collected bool  `json:"collected"`

	// PlaceName 发瓶时带的短地名("Bandra West")。空=没带地点。
	PlaceName string `json:"place_name,omitempty"`
	// DistanceKM 捞到这个瓶子时的距离。**双方都有定位才有值**，
	// 任一方没有则为 nil，前端据此隐藏距离、只显示城市——
	// 不要显示成 0 km(是错的)或「未知」(像故障)。
	//
	// ⚠️ **只给距离,绝不给经纬度**：原始坐标配合多次采样可以三角定位。
	// model.Bottle 的 Lat/Lng 标了 json:"-"，这里也不要加回来。
	DistanceKM *float64 `json:"distance_km,omitempty"`

	// Trace 漂流轨迹节点(按城市聚合,见 bottle.Service.TraceAggFor)。
	// 开关关闭、或浏览者既不是瓶主也没捞过这只瓶子时为空数组;App 据此决定显不显示「漂过哪些地方」。
	Trace []TraceNode `json:"trace"`
}

// TraceNode App 端的轨迹节点。kind: thrown(扔出) / seen(被看到) / replied(收到回信)。
// 只有城市与次数,**没有任何身份信息**——轨迹会出现在可分享的海报上。
type TraceNode struct {
	Kind  string    `json:"kind"`
	City  string    `json:"city"`
	At    time.Time `json:"at"`
	Count int64     `json:"count"`
}

func FromBottle(b *model.Bottle) Bottle {
	return FromBottleWith(b, 0, 0, false, false)
}

// FromBottleWith 带上调用方已经查好的聚合值，避免在这里再查一次库造成 N+1。
func FromBottleWith(b *model.Bottle, viewCount, cityCount int64, liked, collected bool) Bottle {
	if b == nil {
		return Bottle{}
	}
	images := []string{}
	if b.MediaURL != "" {
		images = append(images, b.MediaURL)
	}
	return Bottle{
		ID:        itoa(b.BottleID),
		Content:   b.Content,
		Author:    Author{ID: itoa(b.UserID), Gender: b.AuthorGender, Age: b.AuthorAge},
		CreatedAt: b.CreatedAt,
		ExpireAt:  b.ExpireAt,
		Images:    images,
		Tags:      splitComma(b.Tags),
		City:      b.City,

		Status:     b.Status,
		ReplyCount: b.ReplyCount,
		LikeCount:  b.LikeCount,

		ViewCount: viewCount,
		CityCount: cityCount,
		Liked:     liked,
		Collected: collected,

		PlaceName: b.PlaceName,
		Trace:     []TraceNode{},
	}
}

// DistanceFrom 计算展示用距离：双方都有定位才返回值，否则 nil。
//
// 调用方拿到浏览者位置后自行填到 Bottle.DistanceKM 上——DTO 转换函数本身
// 不查库，也就拿不到浏览者，所以做成独立函数而不是塞进 FromBottleWith。
//
// 返回的是**粗化后**的距离(见 geodist.CoarseKM)：精确距离多点采样可反推坐标。
func DistanceFrom(viewerLat, viewerLng, targetLat, targetLng float64) *float64 {
	if !geodist.HasFix(viewerLat, viewerLng) || !geodist.HasFix(targetLat, targetLng) {
		return nil
	}
	d := geodist.CoarseKM(geodist.KM(viewerLat, viewerLng, targetLat, targetLng))
	return &d
}

func FromBottles(list []model.Bottle) []Bottle {
	out := make([]Bottle, len(list))
	for i := range list {
		out[i] = FromBottle(&list[i])
	}
	return out
}

// ---------------------------------------------------------------- 聊天

// Gift 礼物气泡。gift 类消息的 content 存的是这份 JSON。
type Gift struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Coins int64  `json:"coins"`
	Qty   int    `json:"qty"`
}

// Message App 端的消息结构。
type Message struct {
	ID        string    `json:"id"`
	FromID    string    `json:"from_id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Read      bool      `json:"read_status"`

	// Image/Gift/Qty 都是从 Content 里解出来的——库里只有 content 一列，
	// 图片存 URL、礼物存 JSON。让客户端再解一次 JSON 是把后端的存储细节漏出去。
	Image string `json:"image,omitempty"`
	Gift  *Gift  `json:"gift,omitempty"`
	Qty   int    `json:"qty,omitempty"`
}

func FromMessage(m *model.Message) Message {
	if m == nil {
		return Message{}
	}
	out := Message{
		ID:        itoa(m.MessageID),
		FromID:    itoa(m.SenderID),
		Type:      m.Type,
		Content:   m.Content,
		CreatedAt: m.CreatedAt,
		Read:      m.ReadStatus,
	}
	switch m.Type {
	case "image":
		out.Image = m.Content
	case "gift":
		var g struct {
			ItemID int64  `json:"item_id"`
			Name   string `json:"name"`
			Icon   string `json:"icon"`
			Coins  int64  `json:"coins"`
			Qty    int    `json:"qty"`
		}
		if json.Unmarshal([]byte(m.Content), &g) == nil {
			qty := g.Qty
			if qty <= 0 {
				qty = 1 // 加 qty 之前发出的老消息没有这个字段
			}
			out.Gift = &Gift{
				ID: itoa(g.ItemID), Name: g.Name, Icon: g.Icon, Coins: g.Coins, Qty: qty,
			}
			out.Qty = qty
		}
	}
	return out
}

func FromMessages(list []model.Message) []Message {
	out := make([]Message, len(list))
	for i := range list {
		out[i] = FromMessage(&list[i])
	}
	return out
}

// ---------------------------------------------------------- 钱包 / 道具

// Item 道具（礼物）列表项。
type Item struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Type  string `json:"type"`
	Coins int64  `json:"coins"`
	Charm int64  `json:"charm"`
}

// FromItems 道具列表。
//
// Charm 直接取售价——不是降级：`chat.SendGift` 给对方累加魅力值时用的就是
// `it.PriceCoin`，所以「送出后涨多少魅力」本来就等于售价。库里没有独立的 charm 列，
// 加一列反而会和实际发放逻辑对不上。
func FromItems(list []model.Item) []Item {
	out := make([]Item, len(list))
	for i := range list {
		it := &list[i]
		out[i] = Item{
			ID: itoa(it.ItemID), Name: it.Name, Icon: it.Icon, Type: it.Type,
			Coins: it.PriceCoin, Charm: it.PriceCoin,
		}
	}
	return out
}

// WalletTxn 钱包流水。
type WalletTxn struct {
	ID string `json:"id"`
	// Amount **带符号**：正数入账、负数出账。库里是 direction + 恒正的 coins，
	// 客户端靠正负判断收支，不合并的话所有支出都会显示成收入。
	Amount       int64     `json:"amount"`
	Scene        string    `json:"scene"`
	BalanceAfter int64     `json:"balance_after"`
	CreatedAt    time.Time `json:"created_at"`
}

func FromWalletTxns(list []model.WalletTxn) []WalletTxn {
	out := make([]WalletTxn, len(list))
	for i := range list {
		t := &list[i]
		amount := t.Coins
		if t.Direction == "debit" {
			amount = -amount
		}
		out[i] = WalletTxn{
			ID: itoa(t.TxnID), Amount: amount, Scene: t.Scene,
			BalanceAfter: t.BalanceAfter, CreatedAt: t.CreatedAt,
		}
	}
	return out
}

// 充值档位**没有** DTO：`GET /pay/packages` 不挂鉴权中间件（未登录也要能看价格），
// 拿不到 JWT 里的 platform，分派不了。那一组由客户端的 fromJson 自己兼容两种形状。

// ---------------------------------------------------------------- 动态

// Moment App 端的动态结构。
type Moment struct {
	ID           string    `json:"id"`
	Content      string    `json:"content"`
	Author       Author    `json:"author"`
	Images       []string  `json:"images"`
	City         string    `json:"city"`
	LikeCount    int       `json:"like_count"`
	CommentCount int       `json:"comment_count"`
	Liked        bool      `json:"liked"`
	Following    bool      `json:"following"`
	CreatedAt    time.Time `json:"created_at"`
}

// MomentInput 动态转换的输入。作者信息与 liked 由调用方(moment 包的 FeedItem)带来，
// 它已经批量补过，这里不再查库。
type MomentInput struct {
	Moment    *model.Moment
	Nickname  string
	Avatar    string
	Gender    int8
	Liked     bool
	Following bool
	City      string
}

func FromMoment(in MomentInput) Moment {
	m := in.Moment
	if m == nil {
		return Moment{}
	}
	return Moment{
		ID:      itoa(m.MomentID),
		Content: m.Content,
		Author: Author{
			ID: itoa(m.UserID), Gender: in.Gender,
			Nickname: in.Nickname, Avatar: in.Avatar,
		},
		Images:       parseJSONArray(m.Images),
		City:         in.City,
		LikeCount:    m.LikeCount,
		CommentCount: m.CommentCount,
		Liked:        in.Liked,
		Following:    in.Following,
		CreatedAt:    m.CreatedAt,
	}
}

// parseJSONArray 解 `Moment.Images` 那种「JSON 数组存成字符串」的列。
//
// ⚠️ 不能按逗号切：`["a","b"]` 会被切成 `["a"` 和 `"b"]`，带着方括号和引号，
// 客户端拿去当图片 URL 必然全挂。
func parseJSONArray(s string) []string {
	out := []string{}
	if s = strings.TrimSpace(s); s == "" {
		return out
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		// 老数据可能是逗号分隔的,兜一下,别让一条脏数据把整页 feed 打空。
		return splitComma(s)
	}
	if out == nil {
		return []string{}
	}
	return out
}

// MomentComment App 端的动态评论。
type MomentComment struct {
	ID          string    `json:"id"`
	Content     string    `json:"content"`
	Author      Author    `json:"author"`
	ReplyToNick string    `json:"reply_to_nick"`
	Gift        *Gift     `json:"gift,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CommentInput 评论转换的输入。同样收散装参数，避免 appdto 反向依赖 moment 包。
type CommentInput struct {
	ID          int64
	UserID      int64
	Nickname    string
	Avatar      string
	ReplyToNick string
	Type        string
	Content     string
	CreatedAt   time.Time
}

func FromComment(in CommentInput) MomentComment {
	out := MomentComment{
		ID:      itoa(in.ID),
		Content: in.Content,
		Author: Author{
			ID: itoa(in.UserID), Nickname: in.Nickname, Avatar: in.Avatar,
		},
		ReplyToNick: in.ReplyToNick,
		CreatedAt:   in.CreatedAt,
	}
	// type=gift 时 content 是 JSON{name,icon,coins}——同聊天里的礼物消息。
	if in.Type == "gift" {
		var g struct {
			ItemID int64  `json:"item_id"`
			Name   string `json:"name"`
			Icon   string `json:"icon"`
			Coins  int64  `json:"coins"`
		}
		if json.Unmarshal([]byte(in.Content), &g) == nil {
			out.Gift = &Gift{
				ID: itoa(g.ItemID), Name: g.Name, Icon: g.Icon, Coins: g.Coins, Qty: 1,
			}
		}
	}
	return out
}

// Peer 会话里的对方。
type Peer struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// Conversation App 端的会话结构。
type Conversation struct {
	ID          string    `json:"id"`
	Peer        Peer      `json:"peer"`
	LastMessage string    `json:"last_message"`
	LastType    string    `json:"last_type"`
	LastAt      time.Time `json:"last_at"`
	Unread      int64     `json:"unread"`
	FromBottle  bool      `json:"from_bottle"`
}

// ChatInput 会话转换的输入。
//
// 收散装参数而不是直接收 chat.ChatItem：appdto 被 chat 依赖，
// 反过来依赖它就成了循环 import。
type ChatInput struct {
	Chat            *model.Chat
	PartnerID       int64
	PartnerNickname string
	PartnerAvatar   string
	UnreadCount     int64
}

func FromChat(in ChatInput) Conversation {
	if in.Chat == nil {
		return Conversation{}
	}
	lastType := in.Chat.LastType
	if lastType == "" {
		lastType = "text" // 加 last_type 之前建的老会话
	}
	return Conversation{
		ID: itoa(in.Chat.ChatID),
		Peer: Peer{
			ID:       itoa(in.PartnerID),
			Nickname: in.PartnerNickname,
			Avatar:   in.PartnerAvatar,
		},
		LastMessage: in.Chat.LastMessage,
		LastType:    lastType,
		// 客户端要 last_at，库里叫 updated_at——它就是会话最后活动时间。
		LastAt:     in.Chat.UpdatedAt,
		Unread:     in.UnreadCount,
		FromBottle: in.Chat.SourceBottle != 0,
	}
}
