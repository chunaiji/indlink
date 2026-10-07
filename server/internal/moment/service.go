// Package moment 用户动态:发布(文字+图)、公开广场 feed、点赞、评论(单层)。
package moment

import (
	"encoding/json"
	"strconv"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/moderation"
	"driftbottle/internal/rank"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

type Service struct {
	db  *gorm.DB
	mod *moderation.Service
	wlt *wallet.Service
	// OnCommented 评论成功回调(通知动态作者);可选,由 main 注入,nil 时忽略。
	OnCommented func(tenantID, authorID, momentID, commenterID int64)
	// OnGifted 送礼成功回调(通知动态作者);可选。
	OnGifted func(tenantID, authorID, momentID, senderID int64, itemName string)
}

func New(db *gorm.DB, mod *moderation.Service, wlt *wallet.Service) *Service {
	return &Service{db: db, mod: mod, wlt: wlt}
}

// Create 发布动态:文字审核 → 图片≤9 存 JSON。
// CreateInput 发动态入参。
//
// 从位置参数改成结构体:加了位置之后就有 8 个参数了,调用处根本看不出哪个是哪个。
// 形状与 bottle.CreateInput 保持一致。
type CreateInput struct {
	Content string
	Visible string
	Images  []string
	// 位置可选。指针区分「没传」与「传了 0」:(0,0) 是几内亚湾的真实坐标。
	City      string
	Lat       *float64
	Lng       *float64
	PlaceName string
}

func (s *Service) Create(tenantID, userID int64, in CreateInput) (*model.Moment, error) {
	content, visible, images := in.Content, in.Visible, in.Images
	if visible != "public" && visible != "self" {
		visible = "public"
	}
	if content != "" {
		if err := s.mod.CheckUGC(tenantID, userID, moderation.SceneSocial, content); err != nil {
			return nil, err
		}
	}
	if len(images) > 9 {
		images = images[:9]
	}
	imgJSON := ""
	if len(images) > 0 {
		b, _ := json.Marshal(images)
		imgJSON = string(b)
	}
	if content == "" && imgJSON == "" {
		return nil, errs.New(errs.CodeBadRequest, "写点什么或配张图吧")
	}
	m := &model.Moment{
		MomentID: idgen.Next(), TenantID: tenantID, UserID: userID,
		Content: content, Images: imgJSON, Visible: visible, CreatedAt: time.Now(),
		City: in.City, PlaceName: in.PlaceName,
	}
	// 成对写入:只有一半的坐标算不出距离,不如不写。
	if in.Lat != nil && in.Lng != nil {
		m.Lat, m.Lng = *in.Lat, *in.Lng
	}
	return m, s.db.Create(m).Error
}

// FeedItem 广场卡片:动态 + 作者信息 + 我是否已赞 + 评论预览。
type FeedItem struct {
	model.Moment
	Nickname   string        `gorm:"-" json:"nickname"`
	Avatar     string        `gorm:"-" json:"avatar"`
	Gender     int8          `gorm:"-" json:"gender"`
	IsVerified bool          `gorm:"-" json:"is_verified"`
	Liked      bool          `gorm:"-" json:"liked"`
	Following  bool          `gorm:"-" json:"following"` // 我是否关注了作者(Relation type=like)
	Previews   []CommentView `gorm:"-" json:"previews"`  // 最新 2 条评论预览
}

// relationFollow 「关注」复用 Relation.Type=like——没有独立的 follow 表。
const relationFollow = "like"

// followingSet 我关注了这批作者里的哪些。一次查完,避免 feed 每条再查一次。
func (s *Service) followingSet(tenantID, viewerID int64, authorIDs []int64) map[int64]bool {
	set := map[int64]bool{}
	if len(authorIDs) == 0 {
		return set
	}
	var ids []int64
	s.db.Model(&model.Relation{}).
		Where("tenant_id = ? AND user_a = ? AND type = ? AND user_b IN ?",
			tenantID, viewerID, relationFollow, authorIDs).
		Pluck("user_b", &ids)
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// CommentView 评论展示(带评论者/被回复者昵称)。Type=gift 时 Content 为 JSON{name,icon,coins}。
type CommentView struct {
	CommentID     int64     `json:"comment_id,string"`
	MomentID      int64     `json:"moment_id,string"`
	UserID        int64     `json:"user_id,string"`
	Nickname      string    `json:"nickname"`
	Avatar        string    `json:"avatar"`
	ReplyToUserID int64     `json:"reply_to_user_id,string"`
	ReplyToNick   string    `json:"reply_to_nick"`
	Type          string    `json:"type"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"created_at"`
}

// Feed 公开广场:visible=public、排除拉黑双向、时间倒序。批量补作者/已赞/评论预览,避免 N+1。
//
// [tab] 分流:`following` 只看我关注的人,`city` 只看同城,其余为全部。
// ⚠️ 这个参数以前是**客户端传了、服务端不读**——三个 Tab 拿到的是同一份列表,
// 表现就是「关注了也没用」。
func (s *Service) Feed(tenantID, viewerID int64, tab string, page, size int) ([]FeedItem, error) {
	if size <= 0 || size > 30 {
		size = 20
	}
	var blocked []int64
	s.db.Model(&model.Block{}).Where("user_id = ?", viewerID).Pluck("target_id", &blocked)
	var blockedMe []int64
	s.db.Model(&model.Block{}).Where("target_id = ?", viewerID).Pluck("user_id", &blockedMe)
	blocked = append(blocked, blockedMe...)

	q := s.db.Where("tenant_id = ? AND visible = ?", tenantID, "public")
	if len(blocked) > 0 {
		q = q.Where("user_id NOT IN ?", blocked)
	}
	switch tab {
	case "following":
		// 一个都没关注时返回空列表,而不是退化成「全部」——
		// 那样用户会以为关注生效了。
		q = q.Where("user_id IN (?)",
			s.db.Model(&model.Relation{}).Select("user_b").
				Where("tenant_id = ? AND user_a = ? AND type = ?", tenantID, viewerID, relationFollow))
	case "city":
		var city string
		s.db.Model(&model.User{}).Where("user_id = ?", viewerID).Pluck("city", &city)
		if city != "" {
			q = q.Where("user_id IN (?)",
				s.db.Model(&model.User{}).Select("user_id").
					Where("tenant_id = ? AND city = ?", tenantID, city))
		}
	}
	var moments []model.Moment
	if err := q.Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&moments).Error; err != nil {
		return nil, err
	}
	items := make([]FeedItem, len(moments))
	if len(moments) == 0 {
		return items, nil
	}
	ids := make([]int64, len(moments))
	authorIDs := make([]int64, 0, len(moments))
	seen := map[int64]bool{}
	for i, m := range moments {
		items[i] = FeedItem{Moment: m, Previews: []CommentView{}}
		ids[i] = m.MomentID
		if !seen[m.UserID] {
			seen[m.UserID] = true
			authorIDs = append(authorIDs, m.UserID)
		}
	}
	// 作者信息
	users := s.usersMap(authorIDs)
	// 我是否关注了作者
	followingSet := s.followingSet(tenantID, viewerID, authorIDs)
	// 我是否已赞
	var likedIDs []int64
	s.db.Model(&model.MomentLike{}).Where("user_id = ? AND moment_id IN ?", viewerID, ids).Pluck("moment_id", &likedIDs)
	likedSet := make(map[int64]bool, len(likedIDs))
	for _, id := range likedIDs {
		likedSet[id] = true
	}
	// 评论预览:一次取这批动态的最新评论(上限 200),内存分组取每条前 2
	var cmts []model.MomentComment
	s.db.Where("moment_id IN ?", ids).Order("created_at desc").Limit(200).Find(&cmts)
	previews := s.buildCommentViews(cmts)
	byMoment := map[int64][]CommentView{}
	for _, cv := range previews {
		if len(byMoment[cv.MomentID]) < 2 {
			byMoment[cv.MomentID] = append(byMoment[cv.MomentID], cv)
		}
	}
	for i := range items {
		u := users[items[i].UserID]
		items[i].Nickname, items[i].Avatar, items[i].Gender, items[i].IsVerified = u.Nickname, u.Avatar, u.Gender, u.IsVerified
		items[i].Liked = likedSet[items[i].MomentID]
		items[i].Following = followingSet[items[i].UserID]
		if p, ok := byMoment[items[i].MomentID]; ok {
			// 预览按时间正序展示
			for l, r := 0, len(p)-1; l < r; l, r = l+1, r-1 {
				p[l], p[r] = p[r], p[l]
			}
			items[i].Previews = p
		}
	}
	return items, nil
}

// UserMoments 某用户的公开动态(用户主页「TA 的动态」预览列表)。只 public、时间倒序。
// 简化版：不带评论预览(预览列表用不上)，但补作者/已赞/关注，复用 Feed 的批量语汇。
func (s *Service) UserMoments(tenantID, viewerID, targetID int64, page, size int) ([]FeedItem, error) {
	if size <= 0 || size > 30 {
		size = 20
	}
	var moments []model.Moment
	err := s.db.Where("tenant_id = ? AND user_id = ? AND visible = ?", tenantID, targetID, "public").
		Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&moments).Error
	if err != nil {
		return nil, err
	}
	items := make([]FeedItem, len(moments))
	if len(moments) == 0 {
		return items, nil
	}
	ids := make([]int64, len(moments))
	for i, m := range moments {
		items[i] = FeedItem{Moment: m, Previews: []CommentView{}}
		ids[i] = m.MomentID
	}
	users := s.usersMap([]int64{targetID})
	var likedIDs []int64
	s.db.Model(&model.MomentLike{}).Where("user_id = ? AND moment_id IN ?", viewerID, ids).Pluck("moment_id", &likedIDs)
	likedSet := make(map[int64]bool, len(likedIDs))
	for _, id := range likedIDs {
		likedSet[id] = true
	}
	following := s.followingSet(tenantID, viewerID, []int64{targetID})
	for i := range items {
		u := users[items[i].UserID]
		items[i].Nickname, items[i].Avatar, items[i].Gender, items[i].IsVerified = u.Nickname, u.Avatar, u.Gender, u.IsVerified
		items[i].Liked = likedSet[items[i].MomentID]
		items[i].Following = following[items[i].UserID]
	}
	return items, nil
}

// Detail 单条动态详情(通知/分享落点):visible=public 或本人可见;带作者信息与我是否已赞。
func (s *Service) Detail(tenantID, viewerID, momentID int64) (*FeedItem, error) {
	var m model.Moment
	if err := s.db.First(&m, "tenant_id = ? AND moment_id = ?", tenantID, momentID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "动态已经不在了")
	}
	if m.Visible != "public" && m.UserID != viewerID {
		return nil, errs.New(errs.CodeNotFound, "动态已经不在了")
	}
	item := FeedItem{Moment: m, Previews: []CommentView{}}
	u := s.usersMap([]int64{m.UserID})[m.UserID]
	item.Nickname, item.Avatar, item.Gender, item.IsVerified = u.Nickname, u.Avatar, u.Gender, u.IsVerified
	var n int64
	s.db.Model(&model.MomentLike{}).Where("moment_id = ? AND user_id = ?", momentID, viewerID).Count(&n)
	item.Liked = n > 0
	item.Following = s.followingSet(tenantID, viewerID, []int64{m.UserID})[m.UserID]
	return &item, nil
}

// Comments 某条动态的评论(升序分页)。
func (s *Service) Comments(momentID int64, page, size int) ([]CommentView, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	var cmts []model.MomentComment
	err := s.db.Where("moment_id = ?", momentID).Order("created_at asc").
		Offset((page - 1) * size).Limit(size).Find(&cmts).Error
	if err != nil {
		return nil, err
	}
	return s.buildCommentViews(cmts), nil
}

// AddComment 发评论:审核 → 落库 → comment_count+1 → 通知作者。
func (s *Service) AddComment(tenantID, userID, momentID, replyTo int64, content string) (*CommentView, error) {
	if err := s.mod.CheckUGC(tenantID, userID, moderation.SceneComment, content); err != nil {
		return nil, err
	}
	var m model.Moment
	if err := s.db.First(&m, "tenant_id = ? AND moment_id = ?", tenantID, momentID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "动态不存在")
	}
	cmt := model.MomentComment{
		CommentID: idgen.Next(), TenantID: tenantID, MomentID: momentID,
		UserID: userID, ReplyToUserID: replyTo, Content: content, CreatedAt: time.Now(),
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&cmt).Error; e != nil {
			return e
		}
		return tx.Model(&model.Moment{}).Where("moment_id = ?", momentID).
			UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
	})
	if err != nil {
		return nil, err
	}
	if s.OnCommented != nil && m.UserID != userID {
		s.OnCommented(tenantID, m.UserID, momentID, userID)
	}
	views := s.buildCommentViews([]model.MomentComment{cmt})
	return &views[0], nil
}

// RemoveComment 删除评论:评论者本人或动态作者可删。
func (s *Service) RemoveComment(userID, commentID int64) error {
	var cmt model.MomentComment
	if err := s.db.First(&cmt, "comment_id = ?", commentID).Error; err != nil {
		return errs.New(errs.CodeNotFound, "评论不存在")
	}
	if cmt.UserID != userID {
		var m model.Moment
		if err := s.db.Select("user_id").First(&m, "moment_id = ?", cmt.MomentID).Error; err != nil || m.UserID != userID {
			return errs.New(errs.CodeForbidden, "无权删除该评论")
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Delete(&model.MomentComment{}, "comment_id = ?", commentID).Error; e != nil {
			return e
		}
		return tx.Model(&model.Moment{}).Where("moment_id = ?", cmt.MomentID).
			UpdateColumn("comment_count", gorm.Expr("GREATEST(comment_count - 1, 0)")).Error
	})
}

func (s *Service) Mine(userID int64, page, size int) ([]model.Moment, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	var list []model.Moment
	err := s.db.Where("user_id = ?", userID).
		Order("created_at desc").
		Offset((page - 1) * size).Limit(size).
		Find(&list).Error
	return list, err
}

func (s *Service) Remove(userID, momentID int64) error {
	ret := s.db.Where("moment_id = ? AND user_id = ?", momentID, userID).Delete(&model.Moment{})
	if ret.Error != nil {
		return ret.Error
	}
	if ret.RowsAffected == 0 {
		return errs.New(errs.CodeForbidden, "动态不存在或无权删除")
	}
	return nil
}

// Like 点赞/取消点赞，返回操作后的 liked 状态。
func (s *Service) Like(userID, momentID int64) (liked bool, err error) {
	var n int64
	s.db.Model(&model.MomentLike{}).
		Where("moment_id = ? AND user_id = ?", momentID, userID).Count(&n)
	if n > 0 {
		s.db.Where("moment_id = ? AND user_id = ?", momentID, userID).Delete(&model.MomentLike{})
		s.db.Model(&model.Moment{}).Where("moment_id = ?", momentID).
			UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)"))
		return false, nil
	}
	if err := s.db.Create(&model.MomentLike{MomentID: momentID, UserID: userID, CreatedAt: time.Now()}).Error; err != nil {
		return false, err
	}
	s.db.Model(&model.Moment{}).Where("moment_id = ?", momentID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1"))
	return true, nil
}

// ---- 内部辅助 ----

type userLite struct {
	Nickname   string
	Avatar     string
	Gender     int8
	IsVerified bool
}

func (s *Service) usersMap(ids []int64) map[int64]userLite {
	out := map[int64]userLite{}
	if len(ids) == 0 {
		return out
	}
	var rows []struct {
		UserID     int64
		Nickname   string
		Avatar     string
		Gender     int8
		IsVerified bool
	}
	s.db.Model(&model.User{}).Select("user_id, nickname, avatar, gender, is_verified").
		Where("user_id IN ?", ids).Scan(&rows)
	for _, r := range rows {
		out[r.UserID] = userLite{r.Nickname, r.Avatar, r.Gender, r.IsVerified}
	}
	return out
}

// buildCommentViews 批量补评论者/被回复者昵称。
func (s *Service) buildCommentViews(cmts []model.MomentComment) []CommentView {
	if len(cmts) == 0 {
		return []CommentView{}
	}
	idset := map[int64]bool{}
	ids := make([]int64, 0, len(cmts)*2)
	for _, c := range cmts {
		for _, id := range []int64{c.UserID, c.ReplyToUserID} {
			if id > 0 && !idset[id] {
				idset[id] = true
				ids = append(ids, id)
			}
		}
	}
	users := s.usersMap(ids)
	out := make([]CommentView, len(cmts))
	for i, c := range cmts {
		typ := c.Type
		if typ == "" {
			typ = "text"
		}
		out[i] = CommentView{
			CommentID: c.CommentID, MomentID: c.MomentID, UserID: c.UserID,
			Nickname: users[c.UserID].Nickname, Avatar: users[c.UserID].Avatar,
			ReplyToUserID: c.ReplyToUserID, ReplyToNick: users[c.ReplyToUserID].Nickname,
			Type: typ, Content: c.Content, CreatedAt: c.CreatedAt,
		}
	}
	return out
}

// SendGift 给动态作者送礼:库存先抵扣、没有再扣币(与 chat.SendGift 同一条规则),
// 事务内记 ItemOrder(target)+礼物评论+计数 → 魅力值+ → 通知。
// 礼物评论 content 为 JSON{name,icon,coins},前端据此渲染。
func (s *Service) SendGift(tenantID, senderID, momentID, itemID int64) (*CommentView, error) {
	var m model.Moment
	if err := s.db.First(&m, "tenant_id = ? AND moment_id = ?", tenantID, momentID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "动态不存在")
	}
	var it model.Item
	if err := s.db.First(&it, "item_id = ? AND status = ? AND type = ?", itemID, "active", "gift").Error; err != nil {
		return nil, errs.New(errs.CodeBadRequest, "礼物不存在")
	}
	body, _ := json.Marshal(map[string]interface{}{"name": it.Name, "icon": it.Icon, "coins": it.PriceCoin})
	cmt := model.MomentComment{
		CommentID: idgen.Next(), TenantID: tenantID, MomentID: momentID,
		UserID: senderID, Type: "gift", Content: string(body), CreatedAt: time.Now(),
	}
	bizNo := "momentgift:" + strconv.FormatInt(cmt.CommentID, 10)
	// 背包里有就先用背包的(删一条 target_id=0 的库存行),不再扣币。
	var owned int64
	s.db.Model(&model.ItemOrder{}).
		Where("user_id = ? AND item_id = ? AND target_id = 0", senderID, itemID).Count(&owned)
	price := it.PriceCoin
	if owned > 0 {
		price = 0
	}
	err := s.wlt.Debit(tenantID, senderID, price, wallet.SceneGift, bizNo, func(tx *gorm.DB) error {
		if owned > 0 {
			res := tx.Where("user_id = ? AND item_id = ? AND target_id = 0", senderID, itemID).
				Order("id asc").Limit(1).Delete(&model.ItemOrder{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected < 1 {
				// 事务外数到有库存、事务内却删不到:并发送掉了。整单回滚,别让用户白送。
				return errs.New(errs.CodeItemInsufficient, "礼物数量不足")
			}
		}
		if e := tx.Create(&model.ItemOrder{
			ID: idgen.Next(), TenantID: tenantID, UserID: senderID, ItemID: itemID, TargetID: m.UserID,
			Coins: it.PriceCoin, CreatedAt: time.Now(),
		}).Error; e != nil {
			return e
		}
		if e := tx.Create(&cmt).Error; e != nil {
			return e
		}
		return tx.Model(&model.Moment{}).Where("moment_id = ?", momentID).
			UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
	})
	if err != nil {
		return nil, err
	}
	// 收礼方魅力值累加(与聊天送礼同口径:+礼物金币数)
	s.db.Model(&model.User{}).Where("user_id = ?", m.UserID).
		UpdateColumn("charm", gorm.Expr("charm + ?", it.PriceCoin))
	rank.InvalidateWeekCache(tenantID) // 周榜即时更新
	if s.OnGifted != nil && m.UserID != senderID {
		s.OnGifted(tenantID, m.UserID, momentID, senderID, it.Name)
	}
	views := s.buildCommentViews([]model.MomentComment{cmt})
	return &views[0], nil
}
