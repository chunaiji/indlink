package user

import (
	"driftbottle/internal/common/appdto"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/jwtutil"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.POST("/auth/login", h.login)

	// App 端登录(小程序不走这几条)。返回结构与 /auth/login 完全一致。
	api.POST("/auth/otp/send", h.otpSend)
	api.POST("/auth/otp/verify", h.otpVerify)
	api.POST("/auth/register", h.register)             // 验证码 + 密码,一次建号
	api.POST("/auth/login/password", h.loginPassword)  // 日常登录
	api.POST("/auth/password/reset", h.resetPassword)  // 忘记密码 / 老账号补设密码
	api.POST("/auth/google", h.loginGoogle)
	api.POST("/auth/apple", h.loginApple)
	// 国内版:微信 / 支付宝(code 换身份在服务端做,应用密钥不出服务端)
	api.POST("/auth/wechat", h.loginWechat)
	api.POST("/auth/alipay", h.loginAlipay)
	api.GET("/auth/alipay/auth-info", h.alipayAuthInfo)

	// 绑定/解绑需要登录态：绑定是「把这个第三方身份挂到我头上」，
	// 没有「我」就无从谈起。这三条单独挂 auth。
	api.POST("/auth/google/bind", auth, h.bindGoogle)
	api.POST("/auth/apple/bind", auth, h.bindApple)
	api.POST("/auth/wechat/bind", auth, h.bindWechat)
	api.POST("/auth/alipay/bind", auth, h.bindAlipay)
	api.POST("/auth/unbind", auth, h.unbind)

	g := api.Group("/user", auth)
	g.GET("/profile", h.profile)
	g.POST("/update", h.update)
	g.POST("/verify", h.verify)
	g.DELETE("/account", h.deleteAccount) // App Store 5.1.1(v) 强制
}

// appLoginResp 统一 App 端登录应答：{token, user, is_new} + clientConfig。
func (h *Handler) appLoginResp(c *gin.Context, u *model.User, isNew bool, err error) {
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "登录失败")
		return
	}
	token, err := jwtutil.Generate(u.UserID, u.TenantID, PlatformApp)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "签发令牌失败")
		return
	}
	// 这条只服务 App 登录(OTP / Google / Apple),直接下发 App 形状,不必判 platform。
	resp := gin.H{"token": token, "user": h.selfDTO(u), "is_new": isNew}
	for k, v := range clientConfig(u.TenantID) {
		resp[k] = v
	}
	response.OK(c, resp)
}

// otpSendReq 手机号与邮箱二选一：填了 email 走邮件，否则走短信。
//
// 两个渠道共用这一条路由而不是各开一条，是为了让客户端只维护一个方法、
// 验证码页整屏复用。因此 phone / dial_code 不能再标 binding:"required"，
// 二选一的校验只能自己做。
type otpSendReq struct {
	AppID    string `json:"appid"`
	DialCode string `json:"dial_code"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	// Purpose register / reset / login。**必填**——验证码与用途绑定，
	// 否则注册场景发的码能拿去重置别人的密码。缺省按 login 兼容旧客户端。
	Purpose string `json:"purpose"`
}

func (h *Handler) otpSend(c *gin.Context) {
	var req otpSendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if req.Email == "" && req.Phone == "" {
		response.Fail(c, errs.CodeBadRequest, "手机号与邮箱至少填一个")
		return
	}
	purpose := req.Purpose
	if purpose == "" {
		purpose = PurposeLogin
	}

	var err error
	if req.Email != "" {
		err = h.svc.SendEmailOTP(req.AppID, req.Email, c.ClientIP(), purpose)
	} else {
		err = h.svc.SendOTP(req.AppID, req.DialCode, req.Phone, c.ClientIP(), purpose)
	}
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "发送失败")
		return
	}
	response.OK(c, nil)
}

type otpVerifyReq struct {
	AppID    string `json:"appid"`
	DialCode string `json:"dial_code"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Code     string `json:"code" binding:"required"`
}

func (h *Handler) otpVerify(c *gin.Context) {
	var req otpVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if req.Email == "" && req.Phone == "" {
		response.Fail(c, errs.CodeBadRequest, "手机号与邮箱至少填一个")
		return
	}

	var (
		u     *model.User
		isNew bool
		err   error
	)
	if req.Email != "" {
		u, isNew, err = h.svc.VerifyEmailOTP(req.AppID, req.Email, req.Code)
	} else {
		u, isNew, err = h.svc.VerifyOTP(req.AppID, req.DialCode, req.Phone, req.Code)
	}
	h.appLoginResp(c, u, isNew, err)
}

// credentialReq 注册 / 密码登录 / 重置密码共用的请求体。
// 手机号与邮箱二选一；Code 只有注册和重置要填。
type credentialReq struct {
	AppID    string `json:"appid"`
	DialCode string `json:"dial_code"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

func (h *Handler) register(c *gin.Context) {
	var req credentialReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, err := h.svc.Register(req.AppID, req.DialCode, req.Phone, req.Email, req.Code, req.Password)
	// 注册出来的必然是新用户,直接进「完善资料」引导。
	h.appLoginResp(c, u, true, err)
}

func (h *Handler) loginPassword(c *gin.Context) {
	var req credentialReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, err := h.svc.LoginWithPassword(req.AppID, req.DialCode, req.Phone, req.Email, req.Password)
	h.appLoginResp(c, u, false, err)
}

// resetPassword 改密后直接签发令牌,免得用户刚设完密码又要登一次。
func (h *Handler) resetPassword(c *gin.Context) {
	var req credentialReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, err := h.svc.ResetPassword(req.AppID, req.DialCode, req.Phone, req.Email, req.Code, req.Password)
	h.appLoginResp(c, u, false, err)
}

type oauthLoginReq struct {
	AppID   string `json:"appid"`
	IDToken string `json:"id_token" binding:"required"`
}

func (h *Handler) loginGoogle(c *gin.Context) {
	var req oauthLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, isNew, err := h.svc.LoginWithGoogle(req.AppID, req.IDToken)
	h.appLoginResp(c, u, isNew, err)
}

func (h *Handler) loginApple(c *gin.Context) {
	var req oauthLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, isNew, err := h.svc.LoginWithApple(req.AppID, req.IDToken)
	h.appLoginResp(c, u, isNew, err)
}

func (h *Handler) bindGoogle(c *gin.Context) { h.bindOAuth(c, "google") }
func (h *Handler) bindApple(c *gin.Context)  { h.bindOAuth(c, "apple") }

// bindOAuth 绑定 Google / Apple。流程与登录一致，只是最后一步写到当前用户上。
func (h *Handler) bindOAuth(c *gin.Context, provider string) {
	var req oauthLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	h.finishBind(c, provider, req.AppID, req.IDToken)
}

func (h *Handler) bindWechat(c *gin.Context) {
	var req codeLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	h.finishBind(c, "wechat", req.AppID, req.Code)
}

func (h *Handler) bindAlipay(c *gin.Context) {
	var req alipayLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	h.finishBind(c, "alipay", req.AppID, req.AuthCode)
}

// finishBind 凭据 → 身份 → 绑到当前用户。四家共用:验凭据那一步与登录完全相同。
func (h *Handler) finishBind(c *gin.Context, provider, appid, credential string) {
	sub, _, _, tenantID, err := h.svc.verifyOAuthSub(appid, provider, credential)
	if err == nil {
		err = h.svc.BindOAuth(tenantID, middleware.UserID(c), provider, sub)
	}
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "绑定失败")
		return
	}
	response.OK(c, gin.H{"provider": provider, "bound": true})
}

func (h *Handler) unbind(c *gin.Context) {
	var req struct {
		Provider string `json:"provider" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.UnbindOAuth(middleware.TenantID(c), middleware.UserID(c), req.Provider); err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "解绑失败")
		return
	}
	response.OK(c, gin.H{"provider": req.Provider, "bound": false})
}

func (h *Handler) deleteAccount(c *gin.Context) {
	if err := h.svc.DeleteAccount(middleware.UserID(c)); err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "注销失败")
		return
	}
	response.OK(c, nil)
}

type loginReq struct {
	Platform string `json:"platform" binding:"required"` // wx/alipay
	AppID    string `json:"appid"`                       // 多租户必填(前端运行时自取);单租户可空
	Code     string `json:"code" binding:"required"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, isNew, err := h.svc.Login(req.Platform, req.AppID, req.Code)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "登录失败")
		return
	}
	token, err := jwtutil.Generate(u.UserID, u.TenantID, req.Platform)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "签发令牌失败")
		return
	}
	resp := gin.H{"token": token, "user": u, "is_new": isNew}
	for k, v := range clientConfig(u.TenantID) {
		resp[k] = v
	}
	response.OK(c, resp)
}

// clientConfig 聚合客户端要用的 sysconfig 字段;登录与 /user/profile 共用,避免两处漂移。
func clientConfig(tenantID int64) gin.H {
	return gin.H{
		"ios_recharge_off":      sysconfig.GetBool(tenantID, sysconfig.KeyIOSRechargeOff),
		"push_subscribe_prompt": sysconfig.GetBool(tenantID, sysconfig.KeyPushSubscribePrompt),
		"price_chat":            sysconfig.GetInt(tenantID, sysconfig.KeyPriceChat),
		"share_title":           sysconfig.GetString(tenantID, sysconfig.KeyShareTitle),
		"share_image":           sysconfig.GetString(tenantID, sysconfig.KeyShareImage),
		"ws_url":                sysconfig.GetString(tenantID, sysconfig.KeyWSURL),
		"ui_text_anon_sender":   sysconfig.GetString(tenantID, sysconfig.KeyUITextAnonSender),
		"ui_text_anon_friend":   sysconfig.GetString(tenantID, sysconfig.KeyUITextAnonFriend),
		"ui_text_some_friend":   sysconfig.GetString(tenantID, sysconfig.KeyUITextSomeFriend),
		"ui_text_nav_title":     sysconfig.GetString(tenantID, sysconfig.KeyUITextNavTitle),
		"ui_text_chat_banner":   sysconfig.GetString(tenantID, sysconfig.KeyUITextChatBanner),
		"ui_text_quota_title":   sysconfig.GetString(tenantID, sysconfig.KeyUITextQuotaTitle),
		"ui_text_quota_msg":     sysconfig.GetString(tenantID, sysconfig.KeyUITextQuotaMsg),
		"chat_quicks":           sysconfig.GetString(tenantID, sysconfig.KeyChatQuicks),
		"reply_quicks":          sysconfig.GetString(tenantID, sysconfig.KeyReplyQuicks),
		"low_balance_threshold": sysconfig.GetInt(tenantID, sysconfig.KeyLowBalanceThreshold),
		"low_balance_msg":       sysconfig.GetString(tenantID, sysconfig.KeyLowBalanceMsg),

		// App 本地日志。登录与 /user/profile 都会下发,所以改了配置
		// 用户不必重新登录,下次拉资料就生效。
		"app_log_enabled":     sysconfig.GetBool(tenantID, sysconfig.KeyAppLogEnabled),
		"app_log_level":       sysconfig.GetString(tenantID, sysconfig.KeyAppLogLevel),
		"app_log_body":        sysconfig.GetBool(tenantID, sysconfig.KeyAppLogBody),
		"app_log_max_mb":      sysconfig.GetInt(tenantID, sysconfig.KeyAppLogMaxMB),
		"app_log_retain_days": sysconfig.GetInt(tenantID, sysconfig.KeyAppLogRetainDays),
	}
}

func (h *Handler) profile(c *gin.Context) {
	u, err := h.svc.Profile(middleware.UserID(c))
	if err != nil {
		response.Fail(c, errs.CodeNotFound, "用户不存在")
		return
	}
	resp := gin.H{"user": u}
	if middleware.Platform(c) == PlatformApp {
		resp["user"] = h.selfDTO(u)
	}
	for k, v := range clientConfig(u.TenantID) {
		resp[k] = v
	}
	response.OK(c, resp)
}

type updateReq struct {
	Nickname *string `json:"nickname"`
	Avatar   *string `json:"avatar"`
	Bio      *string `json:"bio"`
	Gender   *int8   `json:"gender"`
	Age      *int    `json:"age"`
	Birthday *string `json:"birthday"` // YYYY-MM-DD,空串=清掉;服务端据此重算 age
	City     *string `json:"city"`
	// App 端字段(小程序不传)
	Language  *string  `json:"language"`
	Interests *string  `json:"interests"`
	Lat       *float64 `json:"lat"`
	Lng       *float64 `json:"lng"`
}

func (h *Handler) update(c *gin.Context) {
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	err := h.svc.UpdateProfile(middleware.UserID(c), UpdateProfileInput{
		Nickname: req.Nickname, Avatar: req.Avatar, Bio: req.Bio, Gender: req.Gender, Age: req.Age, City: req.City,
		Birthday: req.Birthday,
		Language: req.Language, Interests: req.Interests, Lat: req.Lat, Lng: req.Lng,
	})
	if err != nil {
		// 业务拒绝(性别已定不可改 / 生日格式不对)要把原话给到客户端,不能一律「更新失败」。
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "更新失败")
		return
	}
	// App 的「完善资料」提交完要直接拿更新后的资料渲染下一屏,
	// 返回 null 会逼客户端再发一次 /user/profile。小程序不读返回值,保持原样。
	if middleware.Platform(c) == PlatformApp {
		if u, e := h.svc.Profile(middleware.UserID(c)); e == nil {
			response.OK(c, h.selfDTO(u))
			return
		}
	}
	response.OK(c, nil)
}

func (h *Handler) verify(c *gin.Context) {
	if err := h.svc.Verify(middleware.UserID(c)); err != nil {
		response.Fail(c, errs.CodeServerError, "提交失败")
		return
	}
	response.OK(c, gin.H{"is_verified": true})
}

// selfDTO 自己的资料:FromUserSelf(含生日)+ 关注/粉丝 + 瓶子/动态计数。
// 登录、/user/profile、/user/update 三处都用它——少算一处,客户端 updateProfile 一刷就把计数清零。
func (h *Handler) selfDTO(u *model.User) appdto.User {
	dto := appdto.FromUserSelf(u)
	dto.FollowingCount, dto.FollowerCount = h.svc.RelationCounts(u.TenantID, u.UserID)
	dto.BottleCount, dto.MomentCount = h.svc.ContentCounts(u.TenantID, u.UserID)
	return dto
}

// ---------------- 微信 / 支付宝 登录(国内版 App) ----------------

type codeLoginReq struct {
	AppID string `json:"appid"`
	Code  string `json:"code" binding:"required"`
}

type alipayLoginReq struct {
	AppID    string `json:"appid"`
	AuthCode string `json:"auth_code" binding:"required"`
}

func (h *Handler) loginWechat(c *gin.Context) {
	var req codeLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, isNew, err := h.svc.LoginWithWechat(req.AppID, req.Code)
	h.appLoginResp(c, u, isNew, err)
}

func (h *Handler) loginAlipay(c *gin.Context) {
	var req alipayLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, isNew, err := h.svc.LoginWithAlipay(req.AppID, req.AuthCode)
	h.appLoginResp(c, u, isNew, err)
}

// alipayAuthInfo 支付宝 SDK 要的服务端签名授权串。免鉴权:登录前就要用。
func (h *Handler) alipayAuthInfo(c *gin.Context) {
	info, err := h.svc.AlipayAuthInfo(c.Query("appid"))
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "获取授权信息失败")
		return
	}
	response.OK(c, gin.H{"auth_info": info})
}
