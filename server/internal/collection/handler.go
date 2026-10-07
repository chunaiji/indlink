package collection

import (
	"strconv"

	"driftbottle/internal/common/appdto"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/collection", auth)
	g.POST("/add", h.add)
	g.POST("/remove", h.remove)
	g.GET("/list", h.list)
}

type colReq struct {
	TargetID   int64  `json:"target_id,string" binding:"required"`
	TargetType string `json:"target_type" binding:"required"`
}

func (h *Handler) add(c *gin.Context) {
	var req colReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.Add(middleware.TenantID(c), middleware.UserID(c), req.TargetID, req.TargetType); err != nil {
		response.Fail(c, errs.CodeServerError, "收藏失败")
		return
	}
	response.OK(c, nil)
}

func (h *Handler) remove(c *gin.Context) {
	var req colReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.Remove(middleware.UserID(c), req.TargetID, req.TargetType); err != nil {
		response.Fail(c, errs.CodeServerError, "操作失败")
		return
	}
	response.OK(c, nil)
}

func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.List(middleware.UserID(c), page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	// App 端要 appdto 结构(id / author 嵌套 / images 数组)。原样下发 model.Bottle
	// 客户端读不到 id——收藏 tab 点进详情会「没有页面」,列表字段也全错位。小程序仍收原结构。
	if middleware.Platform(c) == "app" {
		out := make([]appdto.Bottle, len(list))
		for i := range list {
			// 收藏列表里的瓶子天然 collected=true;liked 由详情页再拉。
			out[i] = appdto.FromBottleWith(&list[i].Bottle, 0, 0, false, true)
		}
		response.OK(c, out)
		return
	}
	response.OK(c, list)
}
