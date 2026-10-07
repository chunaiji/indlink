package wallet

import (
	"strconv"

	"driftbottle/internal/common/appdto"
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
	g := api.Group("/wallet", auth)
	g.GET("/balance", h.balance)
	g.GET("/txns", h.txns)
}

func (h *Handler) balance(c *gin.Context) {
	bal, err := h.svc.Balance(middleware.UserID(c))
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询余额失败")
		return
	}
	response.OK(c, gin.H{"balance": bal})
}

func (h *Handler) txns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.Txns(middleware.UserID(c), page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询流水失败")
		return
	}
	if middleware.Platform(c) == "app" {
		response.OK(c, appdto.FromWalletTxns(list))
		return
	}
	response.OK(c, list)
}
