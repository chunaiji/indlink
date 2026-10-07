package collection

import (
	"time"

	"driftbottle/internal/model"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

// CollectedBottle 收藏列表返回体:收藏元信息 + 瓶子详情。
type CollectedBottle struct {
	CollectionID int64     `json:"collection_id,string"`
	CollectedAt  time.Time `json:"collected_at"`
	model.Bottle
}

// Add 收藏(幂等)。
func (s *Service) Add(tenantID, userID, targetID int64, targetType string) error {
	var cnt int64
	s.db.Model(&model.Collection{}).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Count(&cnt)
	if cnt > 0 {
		return nil
	}
	return s.db.Create(&model.Collection{
		ID: idgen.Next(), TenantID: tenantID, UserID: userID,
		TargetType: targetType, TargetID: targetID, CreatedAt: time.Now(),
	}).Error
}

// Remove 取消收藏。
func (s *Service) Remove(userID, targetID int64, targetType string) error {
	return s.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Delete(&model.Collection{}).Error
}

// List 收藏列表(仅 bottle),按收藏时间降序。
func (s *Service) List(userID int64, page, size int) ([]CollectedBottle, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	var cols []model.Collection
	if err := s.db.Where("user_id = ? AND target_type = ?", userID, "bottle").
		Order("created_at desc").
		Offset((page-1)*size).Limit(size).
		Find(&cols).Error; err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return []CollectedBottle{}, nil
	}
	ids := make([]int64, len(cols))
	colMap := make(map[int64]model.Collection, len(cols))
	for i, c := range cols {
		ids[i] = c.TargetID
		colMap[c.TargetID] = c
	}
	var bottles []model.Bottle
	if err := s.db.Where("bottle_id IN ? AND status <> ?", ids, "deleted").Find(&bottles).Error; err != nil {
		return nil, err
	}
	bottleMap := make(map[int64]model.Bottle, len(bottles))
	for _, b := range bottles {
		bottleMap[b.BottleID] = b
	}
	out := make([]CollectedBottle, 0, len(ids))
	for _, id := range ids {
		b, ok := bottleMap[id]
		if !ok {
			continue
		}
		c := colMap[id]
		out = append(out, CollectedBottle{CollectionID: c.ID, CollectedAt: c.CreatedAt, Bottle: b})
	}
	return out, nil
}
