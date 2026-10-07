package chat

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/common/appdto"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/jwtutil"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// isApp App 端要另一套响应形状(见 common/appdto)。小程序走原分支,零改动。
func isApp(c *gin.Context) bool { return middleware.Platform(c) == "app" }

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 小程序场景同源校验由网关处理,这里放开
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/chat", auth)
	g.POST("/start", h.start)
	g.GET("/list", h.list)
	g.GET("/unread", h.unread)
	g.GET("/presence", h.presence) // 静态段先注册,避免和下面的 :id 混淆
	g.GET("/:id/messages", h.messages)
	g.POST("/:id/send", h.send)
	g.POST("/:id/gift", h.gift)
}

// presence 批量查在线状态:GET /api/chat/presence?user_ids=1,2,3
//
// 一次问一批,不要每个头像发一个请求。
func (h *Handler) presence(c *gin.Context) {
	raw := c.Query("user_ids")
	if raw == "" {
		response.OK(c, gin.H{"list": []PresenceItem{}})
		return
	}
	var ids []int64
	for _, s := range strings.Split(raw, ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	list, err := h.svc.Presence(middleware.TenantID(c), ids)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list})
}

// unread 当前用户未读消息总数(#2)。
func (h *Handler) unread(c *gin.Context) {
	n := h.svc.UnreadCount(middleware.TenantID(c), middleware.UserID(c))
	response.OK(c, gin.H{"count": n})
}

// RegisterWS 挂载 WebSocket(不走 Bearer 中间件,token 从 query 取)。
func (h *Handler) RegisterWS(r *gin.Engine, path string) {
	r.GET(path, h.ws)
}

type startReq struct {
	TargetID     int64  `json:"target_id,string" binding:"required"`
	SourceBottle string `json:"source_bottle_id"` // 可选,字符串形式的瓶子ID
}

func (h *Handler) start(c *gin.Context) {
	var req startReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	var src int64
	if req.SourceBottle != "" {
		src, _ = strconv.ParseInt(req.SourceBottle, 10, 64)
	}
	chat, err := h.svc.StartChat(middleware.TenantID(c), middleware.UserID(c), req.TargetID, src)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	if isApp(c) {
		// 刚建的会话要立刻渲染聊天页标题,得带上对方昵称头像。
		pid, nick, avatar := h.svc.PartnerBrief(chat, middleware.UserID(c))
		response.OK(c, appdto.FromChat(appdto.ChatInput{
			Chat: chat, PartnerID: pid, PartnerNickname: nick, PartnerAvatar: avatar,
		}))
		return
	}
	response.OK(c, chat)
}

func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.ListChatsWithPartner(middleware.TenantID(c), middleware.UserID(c), page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	if isApp(c) {
		out := make([]appdto.Conversation, len(list))
		for i := range list {
			out[i] = appdto.FromChat(appdto.ChatInput{
				Chat:            &list[i].Chat,
				PartnerID:       list[i].PartnerID,
				PartnerNickname: list[i].PartnerNickname,
				PartnerAvatar:   list[i].PartnerAvatar,
				UnreadCount:     list[i].UnreadCount,
			})
		}
		response.OK(c, out)
		return
	}
	response.OK(c, list)
}

func (h *Handler) messages(c *gin.Context) {
	chatID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.History(middleware.TenantID(c), middleware.UserID(c), chatID, page, size)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	if isApp(c) {
		response.OK(c, appdto.FromMessages(list))
		return
	}
	response.OK(c, list)
}

type sendReq struct {
	Content string `json:"content" binding:"required"`
	Type    string `json:"type"`
}

func (h *Handler) send(c *gin.Context) {
	var req sendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	chatID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	msg, err := h.svc.SendMessage(middleware.TenantID(c), middleware.UserID(c), chatID, req.Content, req.Type)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	if isApp(c) {
		response.OK(c, appdto.FromMessage(msg))
		return
	}
	response.OK(c, msg)
}

type giftReq struct {
	ItemID int64 `json:"item_id" binding:"required"`
	// Qty 连送数量。小程序不传(按 1 处理),App 的连送合并靠它。
	Qty int `json:"qty"`
}

func (h *Handler) gift(c *gin.Context) {
	var req giftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	chatID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	msg, err := h.svc.SendGift(middleware.TenantID(c), middleware.UserID(c), chatID, req.ItemID, req.Qty)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	if isApp(c) {
		response.OK(c, appdto.FromMessage(msg))
		return
	}
	response.OK(c, msg)
}

// ws 升级 WebSocket;token 从 query 取并校验。
func (h *Handler) ws(c *gin.Context) {
	token := c.Query("token")
	claims, err := jwtutil.Parse(token)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	conn := &Conn{userID: claims.UserID, ws: wsConn, send: make(chan []byte, 32), hub: h.svc.Hub()}
	h.svc.Hub().add(conn)
	go conn.writePump()
	conn.readPump()
}

// readPump 读取客户端消息(主要用于心跳/在线维持),退出时下线。
func (c *Conn) readPump() {
	defer func() {
		c.hub.remove(c)
		_ = c.ws.Close()
		close(c.send)
	}()
	c.ws.SetReadLimit(4096)
	_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	for {
		if _, _, err := c.ws.ReadMessage(); err != nil {
			break
		}
		_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

// writePump 推送服务端消息 + 定时 ping。
func (c *Conn) writePump() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func writeBizErr(c *gin.Context, err error) {
	if be, ok := err.(*errs.BizError); ok {
		response.Fail(c, be.Code, be.Msg)
		return
	}
	response.Fail(c, errs.CodeServerError, "服务异常")
}
