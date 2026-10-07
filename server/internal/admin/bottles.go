package admin

import (
	"time"

	"driftbottle/internal/common/response"
	"driftbottle/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strconv"
)

// BottleRow 后台瓶子列表行(含作者信息,审核不受匿名影响)。
type BottleRow struct {
	BottleID    int64     `json:"bottle_id,string"`
	TenantID    int64     `json:"tenant_id,string"`
	UserID      int64     `json:"user_id,string"`
	Nickname    string    `json:"nickname"`
	IsRobot     bool      `json:"is_robot"`
	Gender      int8      `json:"gender"`
	Age         int       `json:"age"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"`
	MediaURL    string    `json:"media_url"`
	Tags        string    `json:"tags"`
	City        string    `json:"city"`
	Status      string    `json:"status"`
	ReplyCount  int       `json:"reply_count"`
	LikeCount   int       `json:"like_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// BottleReplyRow 后台瓶子回应行(含回复者信息)。
type BottleReplyRow struct {
	ReplyID   int64     `json:"reply_id,string"`
	UserID    int64     `json:"user_id,string"`
	Nickname  string    `json:"nickname"`
	IsRobot   bool      `json:"is_robot"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ListBottles 后台瓶子列表,按创建时间倒序。
// status: active/expired/deleted;空为默认视图(不含 deleted)。robotFilter: robot/human/""。
func (s *Service) ListBottles(tenantID int64, keyword, status, robotFilter string, page, size int) ([]BottleRow, int64, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	if page < 1 {
		page = 1
	}
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if tenantID > 0 {
			q = q.Where("bottles.tenant_id = ?", tenantID)
		}
		switch status {
		case "active":
			q = q.Where("bottles.status = ?", "active")
		case "expired":
			q = q.Where("bottles.status = ?", "expired")
		case "deleted":
			q = q.Where("bottles.status = ?", "deleted")
		default: // 默认视图不展示已删除
			q = q.Where("bottles.status != ?", "deleted")
		}
		switch robotFilter {
		case "robot":
			q = q.Where("users.is_robot = ?", true)
		case "human":
			q = q.Where("users.is_robot = ?", false)
		}
		if keyword != "" {
			q = q.Where("bottles.content LIKE ?", "%"+keyword+"%")
		}
		return q
	}

	var total int64
	applyFilters(s.db.Table("bottles").
		Joins("LEFT JOIN users ON users.user_id = bottles.user_id")).Count(&total)

	var rows []BottleRow
	err := applyFilters(
		s.db.Table("bottles").
			Select("bottles.bottle_id, bottles.tenant_id, bottles.user_id, "+
				"COALESCE(users.nickname,'') AS nickname, COALESCE(users.is_robot,false) AS is_robot, "+
				"users.gender, users.age, bottles.content, bottles.content_type, bottles.media_url, "+
				"bottles.tags, bottles.city, bottles.status, bottles.reply_count, bottles.like_count, bottles.created_at").
			Joins("LEFT JOIN users ON users.user_id = bottles.user_id"),
	).Order("bottles.created_at DESC").Offset((page - 1) * size).Limit(size).Scan(&rows).Error
	return rows, total, err
}

// ListBottleReplies 某瓶子的全部回应(升序),含回复者昵称/是否机器人。
func (s *Service) ListBottleReplies(bottleID int64) ([]BottleReplyRow, error) {
	var rows []BottleReplyRow
	err := s.db.Table("bottle_replies").
		Select("bottle_replies.reply_id, bottle_replies.user_id, "+
			"COALESCE(users.nickname,'') AS nickname, COALESCE(users.is_robot,false) AS is_robot, "+
			"bottle_replies.content, bottle_replies.created_at").
		Joins("LEFT JOIN users ON users.user_id = bottle_replies.user_id").
		Where("bottle_replies.bottle_id = ?", bottleID).
		Order("bottle_replies.created_at ASC").
		Scan(&rows).Error
	return rows, err
}

// DeleteBottle 软删瓶子(status=deleted,可恢复)。
func (s *Service) DeleteBottle(bottleID int64) error {
	return s.db.Model(&model.Bottle{}).Where("bottle_id = ?", bottleID).
		Update("status", "deleted").Error
}

// DeleteBottleReply 硬删单条回应,并同步瓶子 reply_count 减 1(回应表无状态字段)。
func (s *Service) DeleteBottleReply(replyID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var r model.BottleReply
		if err := tx.First(&r, "reply_id = ?", replyID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.BottleReply{}, "reply_id = ?", replyID).Error; err != nil {
			return err
		}
		return tx.Model(&model.Bottle{}).Where("bottle_id = ? AND reply_count > 0", r.BottleID).
			Update("reply_count", gorm.Expr("reply_count - 1")).Error
	})
}

// ---- handlers ----

func (h *Handler) listBottles(c *gin.Context) {
	tid := tenantFromCtx(c)
	keyword := c.Query("keyword")
	status := c.Query("status")
	robotFilter := c.Query("robot")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := h.svc.ListBottles(tid, keyword, status, robotFilter, page, size)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func (h *Handler) bottleReplies(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.svc.ListBottleReplies(id)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) deleteBottle(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteBottle(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) deleteBottleReply(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteBottleReply(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}
