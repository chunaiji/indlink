package relation

import (
	"strconv"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/relation", auth)
	g.POST("/like", h.like)
	// 取关。关注按钮是 toggle,只有 like 没有反向操作等于按钮点了没反应。
	g.POST("/unlike", h.unlike)
	g.GET("/stats", h.stats)
	g.GET("/list", h.list)
	g.GET("/i-viewed", h.iViewed)
}

type likeReq struct {
	TargetID int64 `json:"target_id,string" binding:"required"`
}

func (h *Handler) like(c *gin.Context) {
	var req likeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.Like(middleware.TenantID(c), middleware.UserID(c), req.TargetID); err != nil {
		response.Fail(c, errs.CodeServerError, "操作失败")
		return
	}
	response.OK(c, nil)
}

func (h *Handler) unlike(c *gin.Context) {
	var req likeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.Unlike(middleware.TenantID(c), middleware.UserID(c), req.TargetID); err != nil {
		response.Fail(c, errs.CodeServerError, "操作失败")
		return
	}
	response.OK(c, nil)
}

func (h *Handler) stats(c *gin.Context) {
	st, err := h.svc.StatsOf(middleware.UserID(c))
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, st)
}

func (h *Handler) list(c *gin.Context) {
	typ := c.DefaultQuery("type", "ilike")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	ids, err := h.svc.List(middleware.UserID(c), typ, page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, ids)
}

func (h *Handler) iViewed(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.ViewedByMe(middleware.UserID(c), page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, list)
}
