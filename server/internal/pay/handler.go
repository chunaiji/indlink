package pay

import (
	"bytes"
	"io"
	"log"
	"strconv"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 挂载路由。auth 为鉴权中间件;回调接口不鉴权(平台直接调用)。
func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.GET("/pay/packages", h.packages)
	api.POST("/pay/order", auth, h.createOrder)
	api.GET("/pay/orders", auth, h.orders)
	// 单笔查单:H7a 轮询、H9 主动查单、H10 点击处理中的订单都用它
	api.GET("/pay/order/:orderNo", auth, h.order)
	// 联调用的模拟结算。路由常驻,开关在 handler 内部按租户查
	// (sysconfig 是租户级且可热更,启动时判断会读成租户 0 的值且永不更新)。
	api.POST("/pay/mock/settle", auth, h.mockSettle)
	// Apple 内购:verify 需要登录(要知道给谁发币);notify 是苹果服务器直接调,不鉴权
	api.POST("/pay/iap/verify", auth, h.iapVerify)
	api.POST("/pay/iap/notify", h.iapNotify)
	// Google Play:客户端拿到 purchaseToken 后来核实入账(对称 iap/verify)
	api.POST("/pay/play/verify", auth, h.playVerify)
	// 通用回调:微信按 Wechatpay-Serial 头、支付宝按 app_id 自动识别租户,notify_url 填 /api/pay/callback/<platform> 即可
	api.POST("/pay/callback/:platform", h.callbackAuto)
	// 兼容旧版带 tenantId 的回调地址
	api.POST("/pay/callback/:platform/:tenantId", h.callback)
}

func (h *Handler) packages(c *gin.Context) {
	list, err := h.svc.Packages()
	if err != nil {
		response.Fail(c, errs.CodeServerError, "获取充值档位失败")
		return
	}
	response.OK(c, list)
}

type createOrderReq struct {
	PackageID int64  `json:"package_id" binding:"required"`
	Channel   string `json:"channel"` // App 端:wx_app / alipay_app;小程序留空
}

func (h *Handler) createOrder(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	uid := middleware.UserID(c)
	tid := middleware.TenantID(c)
	platform := middleware.Platform(c) // 以登录平台决定支付渠道
	orderNo, params, err := h.svc.CreateOrder(tid, uid, req.PackageID, platform, req.Channel)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "下单失败:"+err.Error())
		return
	}
	response.OK(c, gin.H{"order_no": orderNo, "pay_params": params})
}

func (h *Handler) orders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.Orders(middleware.UserID(c), page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, list)
}

// order 单笔查单。返回订单当前状态,客户端据此判断支付是否到账。
//
// 直接下发 model.PayOrder:它的 JSON tag 已经是对外形状。其中 price_fen 这个名字
// **不许改**(model.go:318,线上小程序订单页直接读它)。
func (h *Handler) order(c *gin.Context) {
	o, err := h.svc.Order(middleware.UserID(c), c.Param("orderNo"))
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	// pending 太久就顺手向渠道问一次(回调丢了的补偿),入账仍只走 HandleCallback。
	response.OK(c, h.svc.SyncIfStale(o))
}

type mockSettleReq struct {
	OrderNo string `json:"order_no" binding:"required"`
	Result  string `json:"result" binding:"required"` // success / fail
}

// mockSettle 联调用:模拟渠道回调。开关关闭时当作接口不存在。
func (h *Handler) mockSettle(c *gin.Context) {
	var req mockSettleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	err := h.svc.SettleMock(middleware.TenantID(c), middleware.UserID(c), req.OrderNo, req.Result)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "结算失败")
		return
	}
	response.OK(c, gin.H{"order_no": req.OrderNo, "result": req.Result})
}

type iapVerifyReq struct {
	// SignedTransaction StoreKit 2 的 Transaction.jwsRepresentation。
	SignedTransaction string `json:"signed_transaction" binding:"required"`
}

// iapVerify 校验内购交易并入账。
//
// 客户端必须**等本接口成功返回后才 finish** 这笔 transaction。
func (h *Handler) iapVerify(c *gin.Context) {
	var req iapVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	rec, err := h.svc.VerifyIAP(middleware.TenantID(c), middleware.UserID(c), req.SignedTransaction)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "入账失败")
		return
	}
	response.OK(c, gin.H{"transaction_id": rec.TransactionID, "coins": rec.Coins, "order_no": rec.OrderNo})
}

type iapNotifyReq struct {
	SignedPayload string `json:"signedPayload"`
}

// iapNotify App Store Server Notifications V2 回调。
//
// 苹果要求 2xx 表示已接收；返回非 2xx 它会重试。处理失败也返回 200 但打日志,
// 避免一条坏通知把重试队列堵死——真正的补偿靠对账。
func (h *Handler) iapNotify(c *gin.Context) {
	var req iapNotifyReq
	if err := c.ShouldBindJSON(&req); err != nil || req.SignedPayload == "" {
		c.String(400, "bad request")
		return
	}
	if err := h.svc.HandleAppleNotification(req.SignedPayload); err != nil {
		log.Printf("[iap] 处理 App Store 通知失败: %v", err)
	}
	c.String(200, "ok")
}

// callbackAuto 通用回调:微信系按 Wechatpay-Serial 头、支付宝系按表单 app_id 反查租户,无需在 URL 中指定 tenantId。
func (h *Handler) callbackAuto(c *gin.Context) {
	platform := canonicalChannel(c.Param("platform")) // wx / wx_app / alipay_app 旧回调路径仍可达
	var candidates []Driver
	switch platform {
	case "wechat":
		// 可能有多家候选(同 serial 多租户 / 平台公钥刚轮换过):逐个验签,过了的才是对的租户。
		candidates = h.svc.WxCallbackDrivers(c.Request.Header.Get("Wechatpay-Serial"))
	case "alipay":
		// ParseForm 消费 Body;驱动的 VerifyCallback 读 r.Form 而不是再 Parse 一次。
		_ = c.Request.ParseForm()
		if d, ok := h.svc.DriverByAlipayAppID(c.Request.Form.Get("app_id")); ok {
			candidates = []Driver{d}
		}
	}
	if len(candidates) == 0 {
		c.String(400, "unknown platform/tenant")
		return
	}
	// 验签要读 Body,多个候选每次都得重放一次。
	// 支付宝那条路 ParseForm 已经把 Body 消费掉了,这里读到空——没关系,
	// AlipayDriver.VerifyCallback 读的是 r.Form 而不是 Body。
	raw, _ := io.ReadAll(c.Request.Body)
	_ = c.Request.Body.Close()
	var d Driver
	var res *CallbackResult
	var err error
	for i, cand := range candidates {
		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
		if res, err = cand.VerifyCallback(c.Request); err == nil {
			d = cand
			break
		}
		if i == len(candidates)-1 {
			log.Printf("[pay] 回调验签失败(%d 个候选都不匹配 serial=%s): %v",
				len(candidates), c.Request.Header.Get("Wechatpay-Serial"), err)
		}
	}
	if d == nil {
		c.String(400, "verify failed")
		return
	}
	if err := h.svc.HandleCallback(platform, res); err != nil {
		c.String(500, "handle failed")
		return
	}
	ct, body := d.SuccessResponse()
	c.Data(200, ct, body)
}

// callback 平台异步回调入口:验签 -> 入账(幂等)。
func (h *Handler) callback(c *gin.Context) {
	platform := canonicalChannel(c.Param("platform"))
	tenantID, _ := strconv.ParseInt(c.Param("tenantId"), 10, 64)
	d, ok := h.svc.DriverForTenant(tenantID, platform)
	if !ok {
		c.String(400, "unknown platform/tenant")
		return
	}
	res, err := d.VerifyCallback(c.Request)
	if err != nil {
		c.String(400, "verify failed")
		return
	}
	if err := h.svc.HandleCallback(platform, res); err != nil {
		// 返回非成功,促使平台重试
		c.String(500, "handle failed")
		return
	}
	ct, body := d.SuccessResponse()
	c.Data(200, ct, body)
}

type playVerifyReq struct {
	ProductID     string `json:"product_id" binding:"required"`
	PurchaseToken string `json:"purchase_token" binding:"required"`
}

// playVerify 校验 Google Play 购买并入账。客户端必须等本接口成功后再 consume。
func (h *Handler) playVerify(c *gin.Context) {
	var req playVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	rec, err := h.svc.VerifyPlay(middleware.TenantID(c), middleware.UserID(c), req.ProductID, req.PurchaseToken)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "入账失败")
		return
	}
	response.OK(c, gin.H{"order_id": rec.OrderID, "coins": rec.Coins, "order_no": rec.OrderNo})
}
