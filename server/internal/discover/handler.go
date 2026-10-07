package discover

import (
	"strconv"
	"strings"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/discover", auth)
	g.GET("/users", h.users)
	g.GET("/count", h.count)
	// 左滑落库。右滑走已有的 POST /api/relation/like，不重复造。
	g.POST("/skip", h.skip)
	// 撤回：GET 取价格(进页面时拉一次,供二次确认弹层显示)，POST 真扣费执行。
	g.GET("/rewind", h.rewindPrice)
	g.POST("/rewind", h.rewind)
	// 答题匹配：GET 拉题库+我的答案(筛选页渲染)，POST 存我的答案。开关关时 enabled=false。
	g.GET("/quiz", h.quiz)
	g.POST("/quiz", h.saveQuiz)
}

// parseFilter 查询参数 → Filter。空值一律按「不限」处理。
func parseFilter(c *gin.Context) Filter {
	f := Filter{
		Languages: splitParam(c.Query("languages")),
		Interests: splitParam(c.Query("interests")),
		Tab:       c.Query("tab"),
	}
	if v := c.Query("max_distance_km"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			f.MaxKM = &n
		}
	}
	if v := c.Query("gender"); v != "" {
		switch v {
		case "male", "1":
			g := int8(1)
			f.Gender = &g
		case "female", "2":
			g := int8(2)
			f.Gender = &g
		}
	}
	f.MinAge, _ = strconv.Atoi(c.Query("min_age"))
	f.MaxAge, _ = strconv.Atoi(c.Query("max_age"))
	f.Limit, _ = strconv.Atoi(c.Query("limit"))
	return f
}

func splitParam(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func (h *Handler) users(c *gin.Context) {
	list, err := h.svc.Users(middleware.TenantID(c), middleware.UserID(c), parseFilter(c))
	if err != nil {
		response.Fail(c, errs.CodeServerError, "获取推荐失败")
		return
	}
	response.OK(c, gin.H{"list": list})
}

func (h *Handler) count(c *gin.Context) {
	n, err := h.svc.Count(middleware.TenantID(c), middleware.UserID(c), parseFilter(c))
	if err != nil {
		response.Fail(c, errs.CodeServerError, "统计失败")
		return
	}
	response.OK(c, gin.H{"count": n})
}

type skipReq struct {
	TargetID string `json:"target_id" binding:"required"`
}

func (h *Handler) skip(c *gin.Context) {
	var req skipReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	// ID 一律字符串传输(JS 端 int64 会丢精度)，服务端自己转回来。
	targetID, err := strconv.ParseInt(req.TargetID, 10, 64)
	if err != nil || targetID == 0 {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.Pass(middleware.TenantID(c), middleware.UserID(c), targetID); err != nil {
		// 开了左滑扣币时余额不足要原样把 5001 透出去，客户端靠它弹充值。
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "操作失败")
		return
	}
	response.OK(c, nil)
}

// rewindPrice 进发现页拉一次的定价配置：撤回单价 + 左滑跳过扣币开关/单价。
// 二次确认弹层显示的数字必须与服务端真扣的一致。
func (h *Handler) rewindPrice(c *gin.Context) {
	tid := middleware.TenantID(c)
	response.OK(c, gin.H{
		"price":               h.svc.RewindPrice(tid),
		"skip_charge_enabled": h.svc.SkipChargeEnabled(tid),
		"skip_price":          h.svc.SkipPrice(tid),
	})
}

func (h *Handler) rewind(c *gin.Context) {
	user, err := h.svc.Rewind(middleware.TenantID(c), middleware.UserID(c))
	if err != nil {
		// 余额不足要原样把 5001 透出去，客户端靠它弹充值而不是通用 toast。
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "撤回失败")
		return
	}
	response.OK(c, gin.H{"user": user})
}

// quiz 拉答题匹配题库 + 我已保存的答案。开关关时 enabled=false，
// 前端据此决定筛选页展示答题 UI 还是回落语言/兴趣。
func (h *Handler) quiz(c *gin.Context) {
	tid := middleware.TenantID(c)
	response.OK(c, gin.H{
		"enabled":   h.svc.QuizEnabled(tid),
		"questions": h.svc.Quiz(tid),
		"answers":   h.svc.MyQuizAnswers(middleware.UserID(c)),
	})
}

type saveQuizReq struct {
	// 紧凑串 "qkey:idx,qkey:idx"，前端拼好；空串合法(清空我的答案)。
	Answers string `json:"answers"`
}

func (h *Handler) saveQuiz(c *gin.Context) {
	var req saveQuizReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.SaveQuizAnswers(middleware.UserID(c), req.Answers); err != nil {
		response.Fail(c, errs.CodeServerError, "保存失败")
		return
	}
	response.OK(c, nil)
}
