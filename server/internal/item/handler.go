package item

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
	api.GET("/item/list", auth, h.list)
	api.POST("/item/buy", auth, h.buy)
	api.GET("/item/my", auth, h.myItems)
	api.GET("/item/orders", auth, h.orders)
}

// orders 我的道具流水(买进 / 送出 / 收到)。
func (h *Handler) orders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	list, err := h.svc.Orders(middleware.TenantID(c), middleware.UserID(c), page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list})
}

func (h *Handler) myItems(c *gin.Context) {
	m, err := h.svc.MyItems(middleware.UserID(c))
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, m)
}

func (h *Handler) list(c *gin.Context) {
	list, err := h.svc.List()
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	if middleware.Platform(c) == "app" {
		response.OK(c, appdto.FromItems(list))
		return
	}
	response.OK(c, list)
}

type buyReq struct {
	ItemID   int64  `json:"item_id" binding:"required"`
	TargetID string `json:"target_id"` // 可选,字符串形式的赠送对象ID
}

func (h *Handler) buy(c *gin.Context) {
	var req buyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	var target int64
	if req.TargetID != "" {
		target, _ = strconv.ParseInt(req.TargetID, 10, 64)
	}
	err := h.svc.Buy(middleware.TenantID(c), middleware.UserID(c), req.ItemID, target)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "购买失败")
		return
	}
	response.OK(c, nil)
}
