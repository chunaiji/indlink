package moderation

import (
	"io"
	"strconv"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("", auth)
	g.POST("/report", h.report)
	g.POST("/block", h.block)
	g.GET("/block/list", h.blockList)
	g.POST("/block/remove", h.unblock)
	// 微信消息推送回调(无鉴权,签名校验;小程序后台"消息推送"配 URL:
	// https://<域名>/message/api/wx/seccheck/callback?tenant=<租户ID>,数据格式 JSON/明文)
	api.GET("/wx/seccheck/callback", h.wxCallbackVerify)
	api.POST("/wx/seccheck/callback", h.wxCallbackEvent)
}

// wxCallbackVerify 微信配置回调 URL 时的一次性验证:签名通过原样返回 echostr。
func (h *Handler) wxCallbackVerify(c *gin.Context) {
	tid, _ := strconv.ParseInt(c.Query("tenant"), 10, 64)
	if !h.svc.VerifyWxSignature(tid, c.Query("signature"), c.Query("timestamp"), c.Query("nonce")) {
		c.String(403, "forbidden")
		return
	}
	c.String(200, c.Query("echostr"))
}

// wxCallbackEvent 接收事件(明文 JSON):校验签名后处理 wxa_media_check 结果。
func (h *Handler) wxCallbackEvent(c *gin.Context) {
	tid, _ := strconv.ParseInt(c.Query("tenant"), 10, 64)
	if !h.svc.VerifyWxSignature(tid, c.Query("signature"), c.Query("timestamp"), c.Query("nonce")) {
		c.String(403, "forbidden")
		return
	}
	raw, _ := io.ReadAll(c.Request.Body)
	h.svc.HandleWxMediaEvent(raw)
	c.String(200, "success") // 微信要求回 success,否则重试推送
}

func (h *Handler) blockList(c *gin.Context) {
	list, err := h.svc.Blocked(middleware.UserID(c))
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) unblock(c *gin.Context) {
	var req blockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.Unblock(middleware.UserID(c), req.TargetID); err != nil {
		response.Fail(c, errs.CodeServerError, "操作失败")
		return
	}
	response.OK(c, nil)
}

type reportReq struct {
	TargetID   int64  `json:"target_id,string" binding:"required"`
	TargetType string `json:"target_type" binding:"required"`
	Reason     string `json:"reason"`
}

func (h *Handler) report(c *gin.Context) {
	var req reportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.Report(middleware.TenantID(c), middleware.UserID(c), req.TargetID, req.TargetType, req.Reason); err != nil {
		response.Fail(c, errs.CodeServerError, "举报失败")
		return
	}
	response.OK(c, nil)
}

type blockReq struct {
	TargetID int64 `json:"target_id,string" binding:"required"`
}

func (h *Handler) block(c *gin.Context) {
	var req blockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.Block(middleware.TenantID(c), middleware.UserID(c), req.TargetID); err != nil {
		response.Fail(c, errs.CodeServerError, "操作失败")
		return
	}
	response.OK(c, nil)
}
