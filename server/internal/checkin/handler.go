package checkin

import (
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/checkin", auth)
	g.GET("/status", h.status)
	g.POST("", h.sign)
	g.POST("/makeup", h.makeup)
}

func (h *Handler) status(c *gin.Context) {
	response.OK(c, h.svc.Status(middleware.TenantID(c), middleware.UserID(c)))
}

func (h *Handler) sign(c *gin.Context) {
	coins, balance, already, dayIdx, err := h.svc.Sign(middleware.TenantID(c), middleware.UserID(c))
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "签到失败")
		return
	}
	response.OK(c, gin.H{"coins": coins, "balance": balance, "already_signed": already, "day_index": dayIdx})
}

func (h *Handler) makeup(c *gin.Context) {
	coins, balance, err := h.svc.Makeup(middleware.TenantID(c), middleware.UserID(c))
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "补签失败")
		return
	}
	response.OK(c, gin.H{"coins": coins, "balance": balance})
}
