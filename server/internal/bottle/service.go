package bottle

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/common/appdto"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/moderation"
	"driftbottle/internal/quota"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// Verifier 抽象 user 模块的认证校验,避免 bottle 直接依赖 user 包(解耦)。
type Verifier interface {
	EnsureVerifiedIfRequired(tenantID, userID int64) error
}

type Service struct {
	db   *gorm.DB
	mod  *moderation.Service
	wlt  *wallet.Service
	verf Verifier
	feed *FeedService
	// OnReplied 回信成功后回调(#4 通知瓶主);可选,由 main 注入,nil 时忽略。
	OnReplied func(tenantID, ownerID, bottleID, replierID int64)
	// OnScooped 瓶子当天首次被捞回调(漂流轨迹);可选,由 main 注入,nil 时忽略。
	OnScooped func(tenantID, ownerID, bottleID, viewerID int64)
}

func New(db *gorm.DB, mod *moderation.Service, wlt *wallet.Service, verf Verifier) *Service {
	return &Service{db: db, mod: mod, wlt: wlt, verf: verf, feed: NewFeedService(db)}
}

type CreateInput struct {
	Content     string
	ContentType string
	MediaURL    string
	Tags        []string
	IsAnonymous bool
	Scope       string // local/national
	City        string
	Visibility  string // 24h/7d/forever
	Night       bool   // 深夜瓶(仅夜场时段生效,打 night 标签)
	// 位置可选。指针区分「没传」与「传了 0」:(0,0) 是几内亚湾的真实坐标,
	// 不能用零值表示「没有位置」。
	Lat       *float64
	Lng       *float64
	PlaceName string
}

// InNightWindow 当前是否处于该租户的深夜场时段(跨零点区间,如 22→2)。
func InNightWindow(tenantID int64) bool {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyNightBottleEnabled) {
		return false
	}
	start := sysconfig.GetInt(tenantID, sysconfig.KeyNightStart)
	end := sysconfig.GetInt(tenantID, sysconfig.KeyNightEnd)
	h := time.Now().Hour()
	if start <= end {
		return h >= start && h < end
	}
	return h >= start || h < end
}

// Create 扔瓶子:认证校验 -> 内容审核 -> 落库(带 tenant_id)。
func (s *Service) Create(tenantID, userID int64, in CreateInput) (*model.Bottle, error) {
	if err := s.verf.EnsureVerifiedIfRequired(tenantID, userID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Content) == "" && in.MediaURL == "" {
		return nil, errs.New(errs.CodeBadRequest, "内容不能为空")
	}
	if in.ContentType == "text" || in.ContentType == "" {
		if err := s.mod.CheckUGC(tenantID, userID, moderation.SceneSocial, in.Content); err != nil {
			return nil, err
		}
	}
	ct := in.ContentType
	if ct == "" {
		ct = "text"
	}
	scope := in.Scope
	if scope == "" {
		scope = "national"
	}
	// 深夜瓶:夜场时段内扔且勾选 → 打 night 标签(只在夜场时段可被捞到)
	if in.Night && InNightWindow(tenantID) && !slices.Contains(in.Tags, "night") {
		in.Tags = append(in.Tags, "night")
	}
	b := model.Bottle{
		BottleID:    idgen.Next(),
		TenantID:    tenantID,
		UserID:      userID,
		Content:     in.Content,
		ContentType: ct,
		MediaURL:    in.MediaURL,
		Tags:        strings.Join(in.Tags, ","),
		IsAnonymous: in.IsAnonymous,
		Scope:       scope,
		City:        in.City,
		Status:      "active",
		CreatedAt:   time.Now(),
		ExpireAt:    expireFrom(in.Visibility),
		PlaceName:   in.PlaceName,
	}
	// 成对写入:只有一半的坐标算不出距离,不如不写。
	if in.Lat != nil && in.Lng != nil {
		b.Lat, b.Lng = *in.Lat, *in.Lng
	}
	// 每日扔瓶次数限制(#5):校验通过后再扣额度(免费→次数包),都没有则拦截
	if !quota.TryConsume(tenantID, userID, quota.ActionThrow, sysconfig.GetInt(tenantID, sysconfig.KeyQuotaThrowDaily)) {
		return nil, errs.New(errs.CodeQuotaExceeded, "今日扔瓶次数已用完,可购买次数包或明天再来")
	}
	if err := s.db.Create(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// ScoopOne 捞一个(计入每日次数 + 去重已看过):免费额度→次数包,都没有则拦截。
func (s *Service) ScoopOne(tenantID, userID int64, city string, tags []string, ft Filter) (*model.Bottle, error) {
	if !quota.TryConsume(tenantID, userID, quota.ActionScoop, sysconfig.GetInt(tenantID, sysconfig.KeyQuotaScoopDaily)) {
		return nil, errs.New(errs.CodeQuotaExceeded, "今日捞瓶次数已用完,可购买次数包或明天再来")
	}
	list, err := s.feed.Next(tenantID, userID, city, tags, ft)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		// 海里没有未看过的瓶子:退回刚消耗的次数,避免空捞扣额度
		quota.Refund(tenantID, userID, quota.ActionScoop, sysconfig.GetInt(tenantID, sysconfig.KeyQuotaScoopDaily))
		return nil, nil
	}
	b := list[0]
	s.fillAuthor(&b)
	s.logAction(tenantID, userID, b.BottleID, "view") // 记录已看过 → 下次不再捞到(#3)
	s.notifyFirstScoopToday(tenantID, b.UserID, b.BottleID, userID)
	return &b, nil
}

// notifyFirstScoopToday 当天首次被捞时异步通知瓶主(轨迹回访钩子,当天聚合一条防骚扰)。
func (s *Service) notifyFirstScoopToday(tenantID, ownerID, bottleID, viewerID int64) {
	if s.OnScooped == nil || !sysconfig.GetBool(tenantID, sysconfig.KeyBottleTraceEnabled) || ownerID == viewerID {
		return
	}
	go func() {
		// 本地(服务器 CST)当日零点;Truncate(24h) 按 UTC 截断会偏 8 小时,勿用
		now := time.Now()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		var n int64
		s.db.Model(&model.MatchLog{}).
			Where("bottle_id = ? AND action = ? AND created_at >= ?", bottleID, "view", dayStart).Count(&n)
		if n == 1 { // 刚落库的这条就是今天第一条
			s.OnScooped(tenantID, ownerID, bottleID, viewerID)
		}
	}()
}

// ViewerFix 取浏览者最后上报的经纬度，用于算「这个瓶子离你多远」。
//
// 单独一次主键查询。没有定位(或查不到用户)时返回 0,0——
// 调用方交给 appdto.DistanceFrom 判断，那里会因为 HasFix 不成立而返回 nil，
// 前端据此只显示城市、不显示距离。
func (s *Service) ViewerFix(tenantID, userID int64) (lat, lng float64) {
	var u model.User
	if err := s.db.Select("lat, lng").
		First(&u, "tenant_id = ? AND user_id = ?", tenantID, userID).Error; err != nil {
		return 0, 0
	}
	return u.Lat, u.Lng
}

// fillAuthor 填充作者性别/年龄(#6:不管匿名都显示)。
func (s *Service) fillAuthor(b *model.Bottle) {
	if b == nil {
		return
	}
	var u model.User
	if err := s.db.Select("gender, age").First(&u, "user_id = ?", b.UserID).Error; err == nil {
		b.AuthorGender = u.Gender
		b.AuthorAge = u.Age
	}
}

// QuotaStatus 返回今日剩余 扔/捞 次数(免费剩余 + 次数包)。
func (s *Service) QuotaStatus(tenantID, userID int64) (throwLeft, scoopLeft int) {
	tf, tp := quota.Remaining(tenantID, userID, quota.ActionThrow, sysconfig.GetInt(tenantID, sysconfig.KeyQuotaThrowDaily))
	sf, sp := quota.Remaining(tenantID, userID, quota.ActionScoop, sysconfig.GetInt(tenantID, sysconfig.KeyQuotaScoopDaily))
	return tf + tp, sf + sp
}

func expireFrom(v string) time.Time {
	now := time.Now()
	switch v {
	case "24h":
		return now.Add(24 * time.Hour)
	case "forever":
		return now.AddDate(100, 0, 0)
	default: // 7d
		return now.Add(7 * 24 * time.Hour)
	}
}

// Scoop 捞一批瓶子(走 Redis 预生成 feed,按租户隔离 + 去重已看过)。
func (s *Service) Scoop(tenantID, userID int64, city string, tags []string, ft Filter) ([]model.Bottle, error) {
	return s.feed.Next(tenantID, userID, city, tags, ft)
}

// ViewerFlags 某个查看者对一批瓶子的「赞过 / 收藏过」状态。
//
// 两条 IN 查询拿完,**不要在渲染循环里逐条查**——捞瓶 feed 一页 20 条,
// 逐条查就是 40 次往返。写法照抄 Mine 里的批量聚合。
type ViewerFlags struct {
	Liked     map[int64]bool
	Collected map[int64]bool
}

func (f ViewerFlags) Of(bottleID int64) (liked, collected bool) {
	return f.Liked[bottleID], f.Collected[bottleID]
}

// ViewerFlagsFor 批量查 viewer 对这批瓶子的赞/藏状态。bottleIDs 为空时返回空表。
func (s *Service) ViewerFlagsFor(tenantID, viewerID int64, bottleIDs []int64) ViewerFlags {
	out := ViewerFlags{Liked: map[int64]bool{}, Collected: map[int64]bool{}}
	if len(bottleIDs) == 0 || viewerID == 0 {
		return out
	}

	var liked []int64
	s.db.Model(&model.MatchLog{}).
		Where("viewer_id = ? AND action = ? AND bottle_id IN ?", viewerID, "like", bottleIDs).
		Pluck("bottle_id", &liked)
	for _, id := range liked {
		out.Liked[id] = true
	}

	var collected []int64
	s.db.Model(&model.Collection{}).
		Where("tenant_id = ? AND user_id = ? AND target_type = ? AND target_id IN ?",
			tenantID, viewerID, "bottle", bottleIDs).
		Pluck("target_id", &collected)
	for _, id := range collected {
		out.Collected[id] = true
	}
	return out
}

// MineBottle 我的瓶子行(含漂流轨迹聚合,开关关闭时聚合恒为 0 且前端不显示入口)。
type MineBottle struct {
	model.Bottle
	ScoopCount int64              `gorm:"-" json:"scoop_count"` // 被捞次数(MatchLog action=view)
	CityCount  int64              `gorm:"-" json:"city_count"`  // 漂过城市数(捞瓶人城市去重,不含空)
	Trace      []appdto.TraceNode `gorm:"-" json:"trace"`       // App 用的轨迹节点
}

// TraceAgg 一只瓶子的轨迹聚合:头部数字 + 按城市聚合的节点。
type TraceAgg struct {
	ScoopCount int64
	CityCount  int64
	Nodes      []appdto.TraceNode
}

// TraceAggFor 批量聚合多只瓶子的漂流轨迹——**一条 group 查询**,不是 N+1。
//
// 节点口径对齐原型 B4:「Mumbai · 你扔出」→「Pune · 被 18 人看到」→「Delhi · 收到 3 条回信」。
// 同城多次被看到合成一条、次数累加、时间取最近一次;没填城市的看瓶人计入总数但不单独成节点
// (「 · 被 3 人看到」这种空城市节点没法读)。开关关闭返回空 map,调用方拿不到就不显示。
func (s *Service) TraceAggFor(tenantID int64, bottles []model.Bottle) map[int64]*TraceAgg {
	out := map[int64]*TraceAgg{}
	if len(bottles) == 0 || !sysconfig.GetBool(tenantID, sysconfig.KeyBottleTraceEnabled) {
		return out
	}
	ids := make([]int64, len(bottles))
	for i, b := range bottles {
		ids[i] = b.BottleID
		out[b.BottleID] = &TraceAgg{Nodes: []appdto.TraceNode{{Kind: "thrown", City: b.City, At: b.CreatedAt}}}
	}
	var rows []struct {
		BottleID int64
		Action   string
		City     string
		N        int64
		At       time.Time
	}
	s.db.Raw(`SELECT m.bottle_id, m.action, COALESCE(u.city,'') AS city, COUNT(*) AS n, MAX(m.created_at) AS at
		FROM match_logs m LEFT JOIN users u ON u.user_id = m.viewer_id
		WHERE m.bottle_id IN ? AND m.action IN ('view','reply')
		GROUP BY m.bottle_id, m.action, city`, ids).Scan(&rows)
	seenCities := map[int64]map[string]bool{}
	for _, r := range rows {
		agg := out[r.BottleID]
		if agg == nil {
			continue
		}
		if r.Action == "view" {
			agg.ScoopCount += r.N
			if r.City != "" {
				if seenCities[r.BottleID] == nil {
					seenCities[r.BottleID] = map[string]bool{}
				}
				seenCities[r.BottleID][r.City] = true
			}
		}
		if r.City == "" {
			continue
		}
		kind := "seen"
		if r.Action == "reply" {
			kind = "replied"
		}
		agg.Nodes = append(agg.Nodes, appdto.TraceNode{Kind: kind, City: r.City, At: r.At, Count: r.N})
	}
	for id, agg := range out {
		agg.CityCount = int64(len(seenCities[id]))
		// 扔出永远排第一,其余按时间升序——这是一条路,不是一张表。
		sort.SliceStable(agg.Nodes[1:], func(i, j int) bool {
			return agg.Nodes[1+i].At.Before(agg.Nodes[1+j].At)
		})
	}
	return out
}

// CanSeeTrace 瓶主、或捞过/回过/赞过这只瓶子的人,可以看它的轨迹。
// 轨迹只有城市与次数,不含身份;捞到的人看「它从哪漂来」是这个产品的情绪核心,不该只给瓶主。
func (s *Service) CanSeeTrace(b *model.Bottle, viewerID int64) bool {
	if b.UserID == viewerID {
		return true
	}
	var n int64
	s.db.Model(&model.MatchLog{}).Where("bottle_id = ? AND viewer_id = ?", b.BottleID, viewerID).Count(&n)
	return n > 0
}

// Mine 我的瓶子(轨迹开关开启时批量补充被捞/城市聚合,一页一条 group 查询避免 N+1)。
func (s *Service) Mine(tenantID, userID int64, page, size int) ([]MineBottle, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	var bottles []model.Bottle
	err := s.db.Where("tenant_id = ? AND user_id = ? AND status != ?", tenantID, userID, "deleted").
		Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&bottles).Error
	if err != nil {
		return nil, err
	}
	list := make([]MineBottle, len(bottles))
	for i, b := range bottles {
		list[i] = MineBottle{Bottle: b}
	}
	aggs := s.TraceAggFor(tenantID, bottles)
	for i := range list {
		if a := aggs[list[i].BottleID]; a != nil {
			list[i].ScoopCount, list[i].CityCount, list[i].Trace = a.ScoopCount, a.CityCount, a.Nodes
		}
	}
	return list, nil
}

// Scooped 我捞过的瓶子。
//
// 数据源是 MatchLog(viewer_id=我)：捞瓶记 view、回信记 reply、点赞记 like——
// 与捞瓶 feed 的「已看过」去重同源(feed.go 用 viewer_id 判 seen)，所以这里也只按
// viewer_id 过滤，保证「我的瓶子·我捞的」与「捞瓶不再重复推」看到的是同一批。
// likedOnly=true 时只返回我点过赞的(供「点赞的瓶子」用)。按最近一次交互时间倒序。
func (s *Service) Scooped(tenantID, userID int64, likedOnly bool, page, size int) ([]model.Bottle, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	actions := []string{"view", "reply", "like"}
	if likedOnly {
		actions = []string{"like"}
	}
	var rows []struct {
		BottleID int64
		LastAt   time.Time
	}
	err := s.db.Model(&model.MatchLog{}).
		Select("bottle_id, MAX(created_at) AS last_at").
		Where("viewer_id = ? AND action IN ?", userID, actions).
		Group("bottle_id").
		Order("last_at DESC").
		Offset((page - 1) * size).Limit(size).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []model.Bottle{}, nil
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.BottleID
	}
	var bottles []model.Bottle
	if err := s.db.Where("tenant_id = ? AND bottle_id IN ? AND status != ?", tenantID, ids, "deleted").
		Find(&bottles).Error; err != nil {
		return nil, err
	}
	byID := make(map[int64]model.Bottle, len(bottles))
	for _, b := range bottles {
		byID[b.BottleID] = b
	}
	// 按 MatchLog 的最近交互顺序还原(SQL IN 的返回序不保证)。
	out := make([]model.Bottle, 0, len(ids))
	for _, id := range ids {
		if b, ok := byID[id]; ok {
			out = append(out, b)
		}
	}
	return out, nil
}

// TraceEvent 漂流轨迹事件(捞瓶人只给城市,不暴露身份)。
type TraceEvent struct {
	Action    string    `json:"action"` // view/reply/like
	City      string    `json:"city"`
	CreatedAt time.Time `json:"created_at"`
}

// TraceResp 漂流轨迹:头部聚合 + 升序时间线。
type TraceResp struct {
	ScoopCount int64        `json:"scoop_count"`
	CityCount  int64        `json:"city_count"`
	Events     []TraceEvent `json:"events"`
}

// Trace 瓶子漂流轨迹(瓶主或捞过它的人可查;开关关闭时拒绝;最多 100 条)。
func (s *Service) Trace(tenantID, viewerID, bottleID int64) (*TraceResp, error) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyBottleTraceEnabled) {
		return nil, errs.New(errs.CodeBadRequest, "功能未开启")
	}
	var b model.Bottle
	if err := s.db.Select("bottle_id, user_id").First(&b, "tenant_id = ? AND bottle_id = ?", tenantID, bottleID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "瓶子不存在")
	}
	if !s.CanSeeTrace(&b, viewerID) {
		return nil, errs.New(errs.CodeForbidden, "只能查看自己的瓶子")
	}
	resp := &TraceResp{Events: []TraceEvent{}}
	s.db.Raw(`SELECT COUNT(*) AS scoop_count, COUNT(DISTINCT NULLIF(u.city,'')) AS city_count
		FROM match_logs m LEFT JOIN users u ON u.user_id = m.viewer_id
		WHERE m.bottle_id = ? AND m.action = 'view'`, bottleID).Scan(resp)
	s.db.Raw(`SELECT m.action, COALESCE(u.city,'') AS city, m.created_at
		FROM match_logs m LEFT JOIN users u ON u.user_id = m.viewer_id
		WHERE m.bottle_id = ? AND m.action IN ('view','reply','like')
		ORDER BY m.created_at ASC LIMIT 100`, bottleID).Scan(&resp.Events)
	return resp, nil
}

// Detail 瓶子详情(限本租户)。
func (s *Service) Detail(tenantID, bottleID int64) (*model.Bottle, error) {
	var b model.Bottle
	if err := s.db.First(&b, "tenant_id = ? AND bottle_id = ?", tenantID, bottleID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "瓶子不存在")
	}
	s.fillAuthor(&b)
	return &b, nil
}

// Reply 回信:审核 -> 落库 -> reply_count+1 -> 记录行为/热度。
func (s *Service) Reply(tenantID, userID, bottleID int64, content string) (*model.BottleReply, error) {
	if err := s.verf.EnsureVerifiedIfRequired(tenantID, userID); err != nil {
		return nil, err
	}
	if err := s.mod.CheckUGC(tenantID, userID, moderation.SceneComment, content); err != nil {
		return nil, err
	}
	var b model.Bottle
	if err := s.db.First(&b, "tenant_id = ? AND bottle_id = ?", tenantID, bottleID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "瓶子不存在")
	}
	if b.Status != "active" || b.ExpireAt.Before(time.Now()) {
		return nil, errs.New(errs.CodeBottleExpired, "瓶子已过期")
	}
	r := model.BottleReply{
		ReplyID: idgen.Next(), TenantID: tenantID, BottleID: bottleID, UserID: userID,
		Content: content, IsUnlocked: false, CreatedAt: time.Now(),
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&r).Error; err != nil {
			return err
		}
		return tx.Model(&model.Bottle{}).Where("bottle_id = ?", bottleID).
			Updates(map[string]interface{}{
				"reply_count": gorm.Expr("reply_count + 1"),
				"heat_score":  gorm.Expr("heat_score + 3"),
			}).Error
	})
	if err != nil {
		return nil, err
	}
	s.logAction(tenantID, userID, bottleID, "reply")
	if s.OnReplied != nil {
		s.OnReplied(tenantID, b.UserID, bottleID, userID) // 通知瓶主
	}
	return &r, nil
}

// ReplyView 回信展示:未解锁则内容脱敏(前 N 字 + ****),含回信者头像/昵称。
type ReplyView struct {
	ReplyID    int64     `json:"reply_id,string"`
	UserID     int64     `json:"user_id,string"`
	Nickname   string    `json:"nickname"`
	Avatar     string    `json:"avatar"`
	Content    string    `json:"content"`
	Locked     bool      `json:"locked"`
	UnlockCost int64     `json:"unlock_cost"`
	CreatedAt  time.Time `json:"created_at"`
}

// maskReply 取前 n 个字明文,其余用 **** 代替;兼容内容长度不足 n 的情况。
func maskReply(content string, n int) string {
	runes := []rune(content)
	if n < 0 {
		n = 0
	}
	if n > len(runes) {
		n = len(runes)
	}
	return string(runes[:n]) + "****"
}

func (s *Service) Replies(tenantID, viewerID, bottleID int64) ([]ReplyView, error) {
	var b model.Bottle
	if err := s.db.First(&b, "tenant_id = ? AND bottle_id = ?", tenantID, bottleID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "瓶子不存在")
	}
	isOwner := b.UserID == viewerID // 本人发布的瓶子:回信免解锁(#4)
	var rs []model.BottleReply
	if err := s.db.Where("tenant_id = ? AND bottle_id = ?", tenantID, bottleID).Order("created_at asc").Find(&rs).Error; err != nil {
		return nil, err
	}
	// 该 viewer 已解锁的回信集合
	unlocked := map[int64]bool{}
	if !isOwner && len(rs) > 0 {
		ids := make([]int64, len(rs))
		for i, r := range rs {
			ids[i] = r.ReplyID
		}
		var us []model.ReplyUnlock
		s.db.Where("viewer_id = ? AND reply_id IN ?", viewerID, ids).Find(&us)
		for _, u := range us {
			unlocked[u.ReplyID] = true
		}
	}
	// 批量取回信者头像/昵称
	authors := s.usersInfo(replyUserIDs(rs))
	cost := sysconfig.GetInt64(tenantID, sysconfig.KeyPriceUnlock)
	maskLen := sysconfig.GetInt(tenantID, sysconfig.KeyReplyMaskLen)

	out := make([]ReplyView, 0, len(rs))
	for _, r := range rs {
		v := ReplyView{ReplyID: r.ReplyID, UserID: r.UserID, CreatedAt: r.CreatedAt, UnlockCost: cost}
		if a, ok := authors[r.UserID]; ok {
			v.Nickname = a.nickname
			v.Avatar = a.avatar
		}
		canSee := isOwner || r.UserID == viewerID || unlocked[r.ReplyID]
		if canSee {
			v.Content = r.Content
			v.Locked = false
		} else {
			v.Content = maskReply(r.Content, maskLen)
			v.Locked = true
		}
		out = append(out, v)
	}
	return out, nil
}

type userBrief struct {
	nickname string
	avatar   string
}

func replyUserIDs(rs []model.BottleReply) []int64 {
	set := map[int64]bool{}
	for _, r := range rs {
		set[r.UserID] = true
	}
	ids := make([]int64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids
}

func (s *Service) usersInfo(ids []int64) map[int64]userBrief {
	out := map[int64]userBrief{}
	if len(ids) == 0 {
		return out
	}
	type row struct {
		UserID   int64
		Nickname string
		Avatar   string
	}
	var rows []row
	s.db.Model(&model.User{}).Select("user_id, nickname, avatar").Where("user_id IN ?", ids).Scan(&rows)
	for _, r := range rows {
		out[r.UserID] = userBrief{nickname: r.Nickname, avatar: r.Avatar}
	}
	return out
}

// UnlockReply 任何人花金币解锁一条回信查看(#4);瓶主/回信本人免费。按 viewer 记录,幂等。
func (s *Service) UnlockReply(tenantID, viewerID, replyID int64) (string, error) {
	var r model.BottleReply
	if err := s.db.First(&r, "tenant_id = ? AND reply_id = ?", tenantID, replyID).Error; err != nil {
		return "", errs.New(errs.CodeNotFound, "回信不存在")
	}
	var b model.Bottle
	if err := s.db.First(&b, "bottle_id = ?", r.BottleID).Error; err != nil {
		return "", errs.New(errs.CodeNotFound, "瓶子不存在")
	}
	// 瓶主 / 回信本人:免费可看
	if b.UserID == viewerID || r.UserID == viewerID {
		return r.Content, nil
	}
	// 已解锁过:直接返回
	var cnt int64
	s.db.Model(&model.ReplyUnlock{}).Where("viewer_id = ? AND reply_id = ?", viewerID, replyID).Count(&cnt)
	if cnt > 0 {
		return r.Content, nil
	}
	price := sysconfig.GetInt64(tenantID, sysconfig.KeyPriceUnlock)
	bizNo := "unlock:" + strconv.FormatInt(replyID, 10) + ":" + strconv.FormatInt(viewerID, 10)
	err := s.wlt.Debit(tenantID, viewerID, price, wallet.SceneUnlock, bizNo, func(tx *gorm.DB) error {
		return tx.Create(&model.ReplyUnlock{
			ID: idgen.Next(), TenantID: tenantID, ReplyID: replyID, ViewerID: viewerID, CreatedAt: time.Now(),
		}).Error
	})
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

// UnlockBottleReplies 整瓶解锁:把这个瓶子里 viewer 还看不到的回信一次性解开。
//
// App 的按钮写的是「解锁 N 条回信」,而 UnlockReply 是按单条收费的。
// 让用户连点 N 次、扣 N 笔流水不是一回事——这里按条累计价格,但**只扣一笔**。
// 单条入口保留:小程序详情页还在用它。
//
// 幂等:已解锁的不重复收费;一条都没锁着时不扣钱,直接返回列表。
func (s *Service) UnlockBottleReplies(tenantID, viewerID, bottleID int64) ([]ReplyView, error) {
	var b model.Bottle
	if err := s.db.First(&b, "tenant_id = ? AND bottle_id = ?", tenantID, bottleID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "瓶子不存在")
	}

	// 瓶主免解锁,跳过扣费直接给全文。
	if b.UserID != viewerID {
		locked, err := s.lockedReplyIDs(tenantID, viewerID, bottleID)
		if err != nil {
			return nil, err
		}
		if len(locked) > 0 {
			price := sysconfig.GetInt64(tenantID, sysconfig.KeyPriceUnlock) * int64(len(locked))
			bizNo := "unlock:bottle:" + strconv.FormatInt(bottleID, 10) + ":" + strconv.FormatInt(viewerID, 10)
			now := time.Now()
			rows := make([]model.ReplyUnlock, 0, len(locked))
			for _, id := range locked {
				rows = append(rows, model.ReplyUnlock{
					ID: idgen.Next(), TenantID: tenantID, ReplyID: id, ViewerID: viewerID, CreatedAt: now,
				})
			}
			err := s.wlt.Debit(tenantID, viewerID, price, wallet.SceneUnlock, bizNo,
				func(tx *gorm.DB) error { return tx.Create(&rows).Error })
			if err != nil {
				return nil, err
			}
		}
	}
	return s.Replies(tenantID, viewerID, bottleID)
}

// lockedReplyIDs 该 viewer 在这个瓶子里还没解锁的回信 ID(自己写的那条不算)。
func (s *Service) lockedReplyIDs(tenantID, viewerID, bottleID int64) ([]int64, error) {
	var rs []model.BottleReply
	if err := s.db.Select("reply_id").
		Where("tenant_id = ? AND bottle_id = ? AND user_id <> ?", tenantID, bottleID, viewerID).
		Find(&rs).Error; err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, nil
	}
	ids := make([]int64, len(rs))
	for i, r := range rs {
		ids[i] = r.ReplyID
	}
	var us []model.ReplyUnlock
	s.db.Where("viewer_id = ? AND reply_id IN ?", viewerID, ids).Find(&us)
	done := make(map[int64]bool, len(us))
	for _, u := range us {
		done[u.ReplyID] = true
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !done[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// Like / Skip 记录行为并更新热度。
func (s *Service) Like(tenantID, userID, bottleID int64) error {
	s.db.Model(&model.Bottle{}).Where("tenant_id = ? AND bottle_id = ?", tenantID, bottleID).
		Update("like_count", gorm.Expr("like_count + 1")).
		Update("heat_score", gorm.Expr("heat_score + 1"))
	s.logAction(tenantID, userID, bottleID, "like")
	return nil
}

func (s *Service) Skip(tenantID, userID, bottleID int64) error {
	s.logAction(tenantID, userID, bottleID, "skip")
	return nil
}

func (s *Service) logAction(tenantID, userID, bottleID int64, action string) {
	_ = s.db.Create(&model.MatchLog{
		MatchID: idgen.Next(), TenantID: tenantID, BottleID: bottleID, ViewerID: userID,
		Action: action, CreatedAt: time.Now(),
	}).Error
}
