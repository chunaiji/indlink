package spark

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
	api.POST("/spark/accept", auth, h.accept)
}

type acceptReq struct {
	PeerID string `json:"peer_id" binding:"required"`
}

// accept 真人火花点击「去聊聊」:校验配对记录后免费建会话。
func (h *Handler) accept(c *gin.Context) {
	var req acceptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	peerID, err := strconv.ParseInt(req.PeerID, 10, 64)
	if err != nil || peerID <= 0 {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	chatID, err := h.svc.Accept(middleware.TenantID(c), middleware.UserID(c), peerID)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "打开会话失败")
		return
	}
	response.OK(c, gin.H{"chat_id": strconv.FormatInt(chatID, 10)})
}
