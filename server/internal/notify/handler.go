package notify

import (
	"strconv"

	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/notify", auth)
	g.GET("/list", h.list)
	g.GET("/unread", h.unread)
	g.POST("/read", h.read)
}

func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	uid := middleware.UserID(c)
	list := h.svc.List(uid, page, size)
	h.svc.MarkRead(uid, 0) // 拉取列表即视为已读
	response.OK(c, list)
}

func (h *Handler) unread(c *gin.Context) {
	response.OK(c, gin.H{"count": h.svc.UnreadCount(middleware.UserID(c))})
}

func (h *Handler) read(c *gin.Context) {
	var req struct {
		ID int64 `json:"id,string"`
	}
	_ = c.ShouldBindJSON(&req)
	h.svc.MarkRead(middleware.UserID(c), req.ID)
	response.OK(c, gin.H{"ok": true})
}
