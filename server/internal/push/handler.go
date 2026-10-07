package push

import (
	"strings"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/jwtutil"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

// Handler 推送订阅接口。
type Handler struct {
	svc           *Service
	defaultTenant int64
}

func NewHandler(svc *Service, defaultTenant int64) *Handler {
	return &Handler{svc: svc, defaultTenant: defaultTenant}
}

// tenantOf 可选鉴权:有合法 Bearer 则取其 tenant,否则用默认租户。
func (h *Handler) tenantOf(c *gin.Context) int64 {
	a := c.GetHeader("Authorization")
	if strings.HasPrefix(a, "Bearer ") {
		if claims, err := jwtutil.Parse(strings.TrimPrefix(a, "Bearer ")); err == nil {
			return claims.TenantID
		}
	}
	return h.defaultTenant
}

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/push")
	g.GET("/templates", h.templates)        // 无需鉴权(获取模板列表)
	g.POST("/subscribe", auth, h.subscribe) // 需要登录
	// App 端(FCM / APNs)。小程序不调这两条。
	g.POST("/device", auth, h.registerDevice)
	g.DELETE("/device", auth, h.unregisterDevice)
}

type deviceReq struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform"` // android/ios
	Provider string `json:"provider"` // fcm/apns，留空按 platform 推断
}

// registerDevice 登记设备令牌。
//
// 客户端时机：**首次发出瓶子之后**再申请通知权限并上报，
// 放在启动时申请授权率会低得多。
func (h *Handler) registerDevice(c *gin.Context) {
	var req deviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	err := h.svc.RegisterDevice(
		middleware.TenantID(c), middleware.UserID(c),
		req.Platform, req.Provider, req.Token,
	)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "登记失败")
		return
	}
	response.OK(c, nil)
}

func (h *Handler) unregisterDevice(c *gin.Context) {
	var req deviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.UnregisterDevice(middleware.UserID(c), req.Token); err != nil {
		response.Fail(c, errs.CodeServerError, "解绑失败")
		return
	}
	response.OK(c, nil)
}

// templates 返回当前配置的订阅模板 ID 列表,前端用于调用 wx.requestSubscribeMessage。
func (h *Handler) templates(c *gin.Context) {
	response.OK(c, h.svc.GetTemplates(h.tenantOf(c)))
}

type subscribeReq struct {
	TemplateID string `json:"template_id" binding:"required"`
	Scene      string `json:"scene" binding:"required"`
}

// subscribe 记录用户订阅授权，openID 由服务端从用户表自取。
func (h *Handler) subscribe(c *gin.Context) {
	var req subscribeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	tenantID := middleware.TenantID(c)
	userID := middleware.UserID(c)
	if err := h.svc.SubscribeByUser(tenantID, userID, req.TemplateID, req.Scene); err != nil {
		response.Fail(c, errs.CodeServerError, "订阅失败")
		return
	}
	response.OK(c, nil)
}
