package ad

import (
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.Group("/ad", auth).POST("/reward", h.reward)
}

func (h *Handler) reward(c *gin.Context) {
	rewarded, coins, balance, count, limit, err := h.svc.Reward(middleware.TenantID(c), middleware.UserID(c))
	if err != nil {
		response.Fail(c, errs.CodeServerError, "领取失败")
		return
	}
	response.OK(c, gin.H{"rewarded": rewarded, "coins": coins, "balance": balance, "count_today": count, "limit": limit})
}
