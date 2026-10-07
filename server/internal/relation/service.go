package relation

import (
	"errors"
	"strconv"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/rank"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// Service 处理喜欢、看过、关系强度。
type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service { return &Service{db: db} }

// Like 我喜欢某人(幂等)。
func (s *Service) Like(tenantID, userID, targetID int64) error {
	if userID == targetID {
		return nil
	}
	var r model.Relation
	err := s.db.First(&r, "user_a = ? AND user_b = ? AND type = ?", userID, targetID, "like").Error
	if err == nil {
		return nil // 已喜欢
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	// 首次喜欢给对方加魅力(只增不减:取消喜欢不扣,见 Unlike)。与建关系同事务,
	// 魅力涨了关系没落库、或反过来,排行榜和资料页就对不上。
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.Relation{
			RelationID: idgen.Next(), TenantID: tenantID, UserA: userID, UserB: targetID, Type: "like",
			Stage: "stranger", CreatedAt: time.Now(), LastInteractionAt: time.Now(),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.User{}).Where("user_id = ?", targetID).
			UpdateColumn("charm", gorm.Expr("charm + ?", likeCharm)).Error
	})
	if err != nil {
		return err
	}
	rank.InvalidateWeekCache(tenantID)
	return nil
}

// likeCharm 被喜欢一次涨的魅力值。送礼涨的是礼物金币数,点赞是免费动作,固定 1。
const likeCharm = 1

// Unlike 取消喜欢 / 取关。
//
// App 的关注按钮是 toggle,而之前只有 Like 没有反向操作——点「已关注」静默什么也不发生,
// 表现就是「关注无效」。
func (s *Service) Unlike(tenantID, userID, targetID int64) error {
	return s.db.Where("tenant_id = ? AND user_a = ? AND user_b = ? AND type = ?",
		tenantID, userID, targetID, "like").Delete(&model.Relation{}).Error
}

// MarkViewed 记录"看过"(用于看过我)。幂等更新时间。
func (s *Service) MarkViewed(tenantID, userID, targetID int64) error {
	if userID == targetID {
		return nil
	}
	var r model.Relation
	err := s.db.First(&r, "user_a = ? AND user_b = ? AND type = ?", userID, targetID, "viewed").Error
	if err == nil {
		return s.db.Model(&r).Update("last_interaction_at", time.Now()).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Create(&model.Relation{
		RelationID: idgen.Next(), TenantID: tenantID, UserA: userID, UserB: targetID, Type: "viewed",
		CreatedAt: time.Now(), LastInteractionAt: time.Now(),
	}).Error
}

type Stats struct {
	ILike    int64 `json:"i_like"`
	LikeMe   int64 `json:"like_me"`
	ViewedMe int64 `json:"viewed_me"`
}

func (s *Service) StatsOf(userID int64) (Stats, error) {
	var st Stats
	s.db.Model(&model.Relation{}).Where("user_a = ? AND type = ?", userID, "like").Count(&st.ILike)
	s.db.Model(&model.Relation{}).Where("user_b = ? AND type = ?", userID, "like").Count(&st.LikeMe)
	s.db.Model(&model.Relation{}).Where("user_b = ? AND type = ?", userID, "viewed").Count(&st.ViewedMe)
	return st, nil
}

// ViewedCard 我浏览过的用户精简卡片。
type ViewedCard struct {
	UserID   int64     `json:"user_id,string"`
	Nickname string    `json:"nickname"`
	Avatar   string    `json:"avatar"`
	City     string    `json:"city"`
	ViewedAt time.Time `json:"viewed_at"`
}

// ViewedByMe 返回"我看过谁"列表，按最近浏览时间降序，带分页。
func (s *Service) ViewedByMe(userID int64, page, size int) ([]ViewedCard, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	type relRow struct {
		UserB             int64
		LastInteractionAt time.Time
	}
	var rels []relRow
	err := s.db.Model(&model.Relation{}).
		Select("user_b, last_interaction_at").
		Where("user_a = ? AND type = ?", userID, "viewed").
		Order("last_interaction_at desc").
		Offset((page-1)*size).Limit(size).
		Scan(&rels).Error
	if err != nil {
		return nil, err
	}
	if len(rels) == 0 {
		return []ViewedCard{}, nil
	}
	ids := make([]int64, len(rels))
	timeMap := make(map[int64]time.Time, len(rels))
	for i, r := range rels {
		ids[i] = r.UserB
		timeMap[r.UserB] = r.LastInteractionAt
	}
	var users []model.User
	if err = s.db.Where("user_id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	userMap := make(map[int64]model.User, len(users))
	for _, u := range users {
		userMap[u.UserID] = u
	}
	out := make([]ViewedCard, 0, len(ids))
	for _, id := range ids {
		u, ok := userMap[id]
		if !ok {
			continue
		}
		out = append(out, ViewedCard{UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar, City: u.City, ViewedAt: timeMap[id]})
	}
	return out, nil
}

// List 按类型返回相关用户 ID 列表(字符串形式,避免 JS 端 int64 精度丢失)。
// type: ilike/likeme/viewed
func (s *Service) List(userID int64, typ string, page, size int) ([]string, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	q := s.db.Model(&model.Relation{})
	var col string
	switch typ {
	case "likeme":
		q = q.Where("user_b = ? AND type = ?", userID, "like")
		col = "user_a"
	case "viewed":
		q = q.Where("user_b = ? AND type = ?", userID, "viewed")
		col = "user_a"
	default: // ilike
		q = q.Where("user_a = ? AND type = ?", userID, "like")
		col = "user_b"
	}
	var ids []int64
	if err := q.Order("last_interaction_at desc").Offset((page - 1) * size).Limit(size).Pluck(col, &ids).Error; err != nil {
		return nil, err
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = strconv.FormatInt(id, 10)
	}
	return out, nil
}
