package admin

import (
	"strconv"
	"strings"

	"driftbottle/internal/common/response"
	"driftbottle/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MomentRow 后台动态列表行(含作者信息)。
type MomentRow struct {
	model.Moment
	Nickname string `gorm:"-" json:"nickname"`
	IsRobot  bool   `gorm:"-" json:"is_robot"`
}

// ListMoments 动态列表(审核):keyword 按内容模糊,robot 筛 human/robot。
func (s *Service) ListMoments(tenantID int64, keyword, robot string, page, size int) ([]MomentRow, int64, error) {
	if size <= 0 || size > 100 {
		size = 20
	}
	q := s.db.Model(&model.Moment{})
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if keyword != "" {
		q = q.Where("content LIKE ?", "%"+keyword+"%")
	}
	if robot == "human" || robot == "robot" {
		sub := s.db.Model(&model.User{}).Select("user_id").Where("is_robot = ?", robot == "robot")
		q = q.Where("user_id IN (?)", sub)
	}
	var total int64
	q.Count(&total)
	var moments []model.Moment
	if err := q.Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&moments).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]MomentRow, len(moments))
	ids := make([]int64, 0, len(moments))
	seen := map[int64]bool{}
	for i, m := range moments {
		rows[i] = MomentRow{Moment: m}
		if !seen[m.UserID] {
			seen[m.UserID] = true
			ids = append(ids, m.UserID)
		}
	}
	if len(ids) > 0 {
		var us []struct {
			UserID   int64
			Nickname string
			IsRobot  bool
		}
		s.db.Model(&model.User{}).Select("user_id, nickname, is_robot").Where("user_id IN ?", ids).Scan(&us)
		byID := map[int64]struct {
			Nick  string
			Robot bool
		}{}
		for _, u := range us {
			byID[u.UserID] = struct {
				Nick  string
				Robot bool
			}{u.Nickname, u.IsRobot}
		}
		for i := range rows {
			rows[i].Nickname = byID[rows[i].UserID].Nick
			rows[i].IsRobot = byID[rows[i].UserID].Robot
		}
	}
	return rows, total, nil
}

// DeleteMoment 删除动态(连带评论与点赞)。
func (s *Service) DeleteMoment(momentID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Delete(&model.Moment{}, "moment_id = ?", momentID).Error; e != nil {
			return e
		}
		tx.Delete(&model.MomentComment{}, "moment_id = ?", momentID)
		tx.Delete(&model.MomentLike{}, "moment_id = ?", momentID)
		return nil
	})
}

// DeleteMomentComment 删除单条评论并同步计数。
func (s *Service) DeleteMomentComment(commentID int64) error {
	var cmt model.MomentComment
	if err := s.db.First(&cmt, "comment_id = ?", commentID).Error; err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Delete(&model.MomentComment{}, "comment_id = ?", commentID).Error; e != nil {
			return e
		}
		return tx.Model(&model.Moment{}).Where("moment_id = ?", cmt.MomentID).
			UpdateColumn("comment_count", gorm.Expr("GREATEST(comment_count - 1, 0)")).Error
	})
}

// ---- HTTP ----

func (h *Handler) listMoments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := h.svc.ListMoments(tenantFromCtx(c), c.Query("keyword"), c.Query("robot"), page, size)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func (h *Handler) momentComments(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var cmts []model.MomentComment
	h.svc.db.Where("moment_id = ?", id).Order("created_at asc").Limit(200).Find(&cmts)
	// 补昵称
	ids := make([]int64, 0, len(cmts))
	seen := map[int64]bool{}
	for _, cm := range cmts {
		if !seen[cm.UserID] {
			seen[cm.UserID] = true
			ids = append(ids, cm.UserID)
		}
	}
	nick := map[int64]string{}
	if len(ids) > 0 {
		var us []struct {
			UserID   int64
			Nickname string
		}
		h.svc.db.Model(&model.User{}).Select("user_id, nickname").Where("user_id IN ?", ids).Scan(&us)
		for _, u := range us {
			nick[u.UserID] = u.Nickname
		}
	}
	out := make([]gin.H, len(cmts))
	for i, cm := range cmts {
		out[i] = gin.H{
			"comment_id": strconv.FormatInt(cm.CommentID, 10), "user_id": strconv.FormatInt(cm.UserID, 10),
			"nickname": nick[cm.UserID], "content": cm.Content, "created_at": cm.CreatedAt,
		}
	}
	response.OK(c, out)
}

func (h *Handler) deleteMoment(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteMoment(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) deleteMomentComment(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteMomentComment(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// listEmailLogs 邮件发送记录(状态/收件人筛选,时间倒序)。
//
// ⚠️ 返回体里**含明文验证码**。这是产品要求,但意味着后台账号本身就是一把
// 能登录任意用户的钥匙——后台密码要按凭据级别来管。
func (h *Handler) listEmailLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if size <= 0 || size > 200 {
		size = 50
	}
	if page < 1 {
		page = 1
	}
	q := h.svc.db.Model(&model.EmailLog{})
	if tid := tenantFromCtx(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if p := c.Query("purpose"); p != "" {
		q = q.Where("purpose = ?", p)
	}
	// 收件人支持模糊查:客服拿到的往往是用户口述的邮箱,未必完全准确。
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		q = q.Where("`to` LIKE ?", "%"+to+"%")
	}
	var total int64
	q.Count(&total)
	var list []model.EmailLog
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	response.OK(c, gin.H{"list": list, "total": total})
}

// listApiLogs 外部接口调用日志(kind 筛选,时间倒序)。
func (h *Handler) listApiLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if size <= 0 || size > 200 {
		size = 50
	}
	if page < 1 {
		page = 1
	}
	q := h.svc.db.Model(&model.ApiCallLog{})
	if tid := tenantFromCtx(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if kind := c.Query("kind"); kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var total int64
	q.Count(&total)
	var list []model.ApiCallLog
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	response.OK(c, gin.H{"list": list, "total": total})
}
