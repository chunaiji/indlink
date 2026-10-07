package admin

import (
	"context"
	"errors"
	"strconv"
	"time"

	"driftbottle/internal/common/i18n"
	"driftbottle/internal/common/response"
	"driftbottle/internal/model"
	"driftbottle/internal/robot"
	"driftbottle/internal/upload"

	"github.com/gin-gonic/gin"
)

var (
	errInvalid = errors.New("用户名或密码错误")
	errBadKey  = errors.New("非法配置项")
)

type Handler struct {
	svc           *Service
	uploadDir     string // 图片上传目录(与 C 端共用)
	publicBaseURL string
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// SetUpload 注入图片上传目录与对外 URL 前缀(main 启动时调用)。
func (h *Handler) SetUpload(dir, publicBaseURL string) {
	h.uploadDir, h.publicBaseURL = dir, publicBaseURL
}

// tenantFromCtx 从 X-Tenant-ID header 或 ?tenant_id= query 中读取当前租户。
// 返回 0 表示"全部租户"（仅用于读操作）。
func tenantFromCtx(c *gin.Context) int64 {
	if h := c.GetHeader("X-Tenant-ID"); h != "" {
		if id, err := strconv.ParseInt(h, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	if q := c.Query("tenant_id"); q != "" {
		if id, err := strconv.ParseInt(q, 10, 64); err == nil {
			return id
		}
	}
	return 0
}

// Register 挂在 /admin/api 下(与 C 端 /api 隔离;SPA 静态另行部署,避免路由冲突)。
func (h *Handler) Register(r *gin.Engine) {
	// 语言中间件挂在 g 而不是 auth 上 —— 登录页也有语言切换器,
	// 登录失败的提示应当跟随所选语言。
	//
	// ⚠️ 只挂这一组。C 端路由不挂,否则 wx.request 在部分机型带的
	// Accept-Language: en 会让小程序中文用户突然收到英文报错。
	g := r.Group("/admin/api", i18n.Middleware())
	g.POST("/login", h.login)

	auth := g.Group("", authMiddleware())
	auth.GET("/me", h.me)
	auth.POST("/password", h.changePassword)
	auth.GET("/stats", h.stats)
	auth.GET("/stats/overview", h.statsOverview)
	auth.GET("/config", h.getConfig)
	auth.PUT("/config", h.putConfig)
	auth.POST("/upload", h.uploadImage) // 配置用图片上传(客服二维码/关联小程序图标等)
	// 机器人内容池
	auth.GET("/robot/content", h.listRobotContent)
	auth.POST("/robot/content", h.createRobotContent)
	auth.PUT("/robot/content/:id", h.updateRobotContent)
	auth.DELETE("/robot/content/:id", h.deleteRobotContent)
	// 租户凭证
	// 服务商配置(支付 / 地图 / 内容安全),按 X-Tenant-ID
	auth.GET("/providers/:kind", h.listProviders)
	auth.PUT("/providers/:kind/:provider", h.saveProvider)
	auth.POST("/providers/:kind/:provider/test", h.probeProvider)
	auth.GET("/credentials", h.listCredentials)
	auth.POST("/credentials", h.createCredential)
	auth.PUT("/credentials/:id", h.updateCredential)
	// Persona
	auth.GET("/persona", h.listPersonas)
	auth.POST("/persona", h.createPersona)
	auth.PUT("/persona/:id", h.updatePersona)
	auth.DELETE("/persona/:id", h.deletePersona)
	auth.POST("/persona/reload", h.reloadPersona)
	auth.GET("/persona/batch-info", h.personaBatchInfo)
	auth.POST("/persona/batch", h.batchCreatePersonas)
	// Robot profiles
	auth.GET("/robot/profiles", h.listRobotProfiles)
	auth.POST("/robot/profiles", h.createRobots)
	auth.PUT("/robot/profiles/:id", h.updateRobotProfile)
	// Keyword rules
	auth.GET("/keyword-rules", h.listKeywordRules)
	auth.POST("/keyword-rules", h.createKeywordRule)
	auth.PUT("/keyword-rules/:id", h.updateKeywordRule)
	auth.DELETE("/keyword-rules/:id", h.deleteKeywordRule)
	auth.PUT("/keyword-rules/:id/toggle", h.toggleKeywordRule)
	// Reply cache
	auth.GET("/reply-cache", h.listReplyCache)
	auth.PUT("/reply-cache/:id", h.updateReplyCache)
	auth.PUT("/reply-cache/:id/toggle", h.toggleReplyCache)
	auth.DELETE("/reply-cache/:id", h.deleteReplyCache)
	// 租户列表
	auth.GET("/tenants", h.listTenants)
	auth.POST("/tenants", h.createTenant)
	auth.PUT("/tenants/:id", h.updateTenant)
	// 用户管理
	auth.GET("/users", h.listUsers)
	auth.GET("/messages", h.listMessages)
	auth.PUT("/users/:id/ban", h.banUser)
	auth.PUT("/users/:id/mute", h.muteUser)
	auth.PUT("/users/:id/tags", h.updateUserTags)
	auth.POST("/users/:id/coins", h.adjustUserCoins)
	auth.POST("/users/:id/robot-chat", h.startRobotChat)
	auth.POST("/users/:id/push", h.pushUser)
	// LLM 测试
	auth.POST("/llm/test", h.testLLM)
	// 会话式消息浏览(全部会话)
	auth.GET("/chats", h.listChats)
	auth.GET("/chats/:id", h.chatDetail)
	// 瓶子管理(审核)
	auth.GET("/bottles", h.listBottles)
	auth.GET("/bottles/:id/replies", h.bottleReplies)
	auth.DELETE("/bottles/:id", h.deleteBottle)
	auth.DELETE("/bottle-replies/:id", h.deleteBottleReply)
	// 外部接口调用日志(微信内容安全/推送/逆地理)
	auth.GET("/apilogs", h.listApiLogs)
	auth.GET("/email-logs", h.listEmailLogs)
	// 动态管理(审核)
	auth.GET("/moments", h.listMoments)
	auth.GET("/moments/:id/comments", h.momentComments)
	auth.DELETE("/moments/:id", h.deleteMoment)
	auth.DELETE("/moment-comments/:id", h.deleteMomentComment)
	// 机器人对话（管理员手动回复）
	auth.GET("/robot/chats", h.listRobotChats)
	auth.GET("/robot/chats/:id/messages", h.robotChatMessages)
	auth.POST("/robot/chats/:id/send", h.sendAsRobot)
	// WS 在线状态
	auth.GET("/online", h.online)
	// 财务流水
	auth.GET("/orders", h.listPayOrders)
	auth.GET("/wallet-txns", h.listWalletTxns)
	// 充值档位
	auth.GET("/packages", h.listPackages)
	auth.POST("/packages", h.createPackage)
	auth.PUT("/packages/:id", h.updatePackage)
	auth.DELETE("/packages/:id", h.deletePackage)
	// 道具/礼物
	auth.GET("/items", h.listItems)
	auth.POST("/items", h.createItem)
	auth.PUT("/items/:id", h.updateItem)
	auth.DELETE("/items/:id", h.deleteItem)
}

func (h *Handler) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	a, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		response.Fail(c, 2002, "用户名或密码错误")
		return
	}
	token, err := genToken(a)
	if err != nil {
		response.Fail(c, 1002, "签发失败")
		return
	}
	response.OK(c, gin.H{"token": token, "username": a.Username, "role": a.Role})
}

func (h *Handler) me(c *gin.Context) {
	response.OK(c, gin.H{
		"id":       c.GetInt64("admin_id"),
		"username": c.GetString("admin_name"),
		"role":     c.GetString("admin_role"),
	})
}

func (h *Handler) changePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "新密码至少 6 位")
		return
	}
	if err := h.svc.ChangePassword(c.GetInt64("admin_id"), req.OldPassword, req.NewPassword); err != nil {
		response.Fail(c, 2002, "原密码错误")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) stats(c *gin.Context) {
	response.OK(c, h.svc.Stats(tenantFromCtx(c)))
}

func (h *Handler) statsOverview(c *gin.Context) {
	response.OK(c, h.svc.Overview(tenantFromCtx(c)))
}

// getConfig 下发配置项 + 分区列表。
//
// 分区单独给一份而不是让前端从 list 里推导:configMeta 是按分组排的,
// 按首次出现推导出来的 Tab 顺序是乱的。顺序是展示信息,该由服务端定。
func (h *Handler) getConfig(c *gin.Context) {
	lang := i18n.FromContext(c)
	response.OK(c, gin.H{
		"sections": Sections(lang),
		"list":     h.svc.Config(tenantFromCtx(c), lang),
	})
}

// uploadImage 管理后台图片上传(复用 C 端存储逻辑与目录)。
func (h *Handler) uploadImage(c *gin.Context) {
	if h.uploadDir == "" {
		response.Fail(c, 1002, "上传未配置")
		return
	}
	url, err := upload.SaveImage(c, h.uploadDir, h.publicBaseURL)
	if err != nil {
		response.Fail(c, 1001, err.Error())
		return
	}
	response.OK(c, gin.H{"url": url})
}

func (h *Handler) putConfig(c *gin.Context) {
	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value"`
		// Clear 显式清空。机密项写空会被当成「保持不变」,要真清掉只能走这里。
		Clear bool `json:"clear"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.SetConfig(tenantFromCtx(c), req.Key, req.Value, req.Clear); err != nil {
		response.FailErr(c, 1001, "保存失败", err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) listRobotContent(c *gin.Context) {
	tid := tenantFromCtx(c)
	list, err := h.svc.ListRobotContent(tid, c.Query("type"))
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) createRobotContent(c *gin.Context) {
	var req struct {
		TenantID int64  `json:"tenant_id,string"`
		Type     string `json:"type" binding:"required"`
		Text     string `json:"text" binding:"required"`
		Tags     string `json:"tags"`
		Weight   int    `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if req.TenantID == 0 {
		req.TenantID = tenantFromCtx(c)
	}
	if req.TenantID == 0 {
		req.TenantID = h.svc.defaultTenantID()
	}
	if err := h.svc.CreateRobotContent(req.TenantID, req.Type, req.Text, req.Tags, req.Weight); err != nil {
		response.Fail(c, 1002, "创建失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) updateRobotContent(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Text   string `json:"text" binding:"required"`
		Tags   string `json:"tags"`
		Weight int    `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.UpdateRobotContent(id, req.Text, req.Tags, req.Weight); err != nil {
		response.Fail(c, 1002, "更新失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) deleteRobotContent(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteRobotContent(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) listCredentials(c *gin.Context) {
	list, err := h.svc.ListCredentials()
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) createCredential(c *gin.Context) {
	var req CreateCredReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.CreateCredential(req); err != nil {
		response.FailErr(c, 1002, "创建失败", err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) updateCredential(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req UpdateCredReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.UpdateCredential(id, req); err != nil {
		response.FailErr(c, 1002, "更新失败", err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) listPersonas(c *gin.Context) {
	tid := tenantFromCtx(c)
	list, err := h.svc.ListPersonas(tid)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) createPersona(c *gin.Context) {
	var req PersonaRow
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if req.TenantID == 0 {
		req.TenantID = tenantFromCtx(c)
	}
	if req.TenantID == 0 {
		req.TenantID = h.svc.defaultTenantID()
	}
	if err := h.svc.CreatePersona(req); err != nil {
		response.Fail(c, 1002, "创建失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// personaBatchInfo 批量复制前置信息:模板可用数 + 当前是否就是模板租户。
func (h *Handler) personaBatchInfo(c *gin.Context) {
	available, isTemplateTenant := h.svc.PersonaTemplateCount(tenantFromCtx(c))
	response.OK(c, gin.H{"available": available, "is_template_tenant": isTemplateTenant})
}

// batchCreatePersonas 从默认租户的人格模板批量复制到当前租户(跳过同名)。
func (h *Handler) batchCreatePersonas(c *gin.Context) {
	var req struct {
		Count int `json:"count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	created, skipped, err := h.svc.BatchCreatePersonas(tenantFromCtx(c), req.Count)
	if err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}
	response.OK(c, gin.H{"created": created, "skipped": skipped})
}

func (h *Handler) updatePersona(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req PersonaRow
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.UpdatePersona(id, req); err != nil {
		response.Fail(c, 1002, "更新失败")
		return
	}
	robot.ReloadGlobal()
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) deletePersona(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeletePersona(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	robot.ReloadGlobal()
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) reloadPersona(c *gin.Context) {
	robot.ReloadGlobal()
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) listRobotProfiles(c *gin.Context) {
	tid := tenantFromCtx(c)
	list, err := h.svc.ListRobotProfiles(tid)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) updateRobotProfile(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		PersonaID *int64  `json:"persona_id,string"`
		Status    string  `json:"status"`
		Nickname  *string `json:"nickname"`
		Gender    *int8   `json:"gender"`
		Age       *int    `json:"age"`
		City      *string `json:"city"`
		Language  *string `json:"language"`
		Interests *string `json:"interests"`
		Bio       *string `json:"bio"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	err := h.svc.UpdateRobotProfile(userID, RobotProfileUpdate{
		PersonaID: req.PersonaID, Status: req.Status, Nickname: req.Nickname, Gender: req.Gender,
		Age: req.Age, City: req.City, Language: req.Language, Interests: req.Interests, Bio: req.Bio,
	})
	if err != nil {
		response.Fail(c, 1002, "更新失败")
		return
	}
	robot.ReloadGlobal()
	response.OK(c, gin.H{"ok": true})
}

// createRobots 后台批量新增机器人:选语言(zh/en)、可选性别与城市,资料随语言生成。
func (h *Handler) createRobots(c *gin.Context) {
	var req struct {
		Count    int    `json:"count"`
		Language string `json:"language"` // zh / en
		Gender   int8   `json:"gender"`   // 0 随机
		City     string `json:"city"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	tid := tenantFromCtx(c)
	if tid == 0 {
		tid = h.svc.defaultTenantID()
	}
	list, err := h.svc.CreateRobots(tid, req.Count, req.Language, req.Gender, req.City)
	if err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}
	robot.ReloadGlobal()
	response.OK(c, gin.H{"created": len(list)})
}

func (h *Handler) listKeywordRules(c *gin.Context) {
	tid := tenantFromCtx(c)
	list, err := h.svc.ListKeywordRules(tid)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) createKeywordRule(c *gin.Context) {
	var req model.RobotKeywordRule
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if req.TenantID == 0 {
		req.TenantID = tenantFromCtx(c)
	}
	if req.TenantID == 0 {
		req.TenantID = h.svc.defaultTenantID()
	}
	if err := h.svc.CreateKeywordRule(req); err != nil {
		response.Fail(c, 1002, "创建失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) updateKeywordRule(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req model.RobotKeywordRule
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.UpdateKeywordRule(id, req); err != nil {
		response.Fail(c, 1002, "更新失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) deleteKeywordRule(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteKeywordRule(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) toggleKeywordRule(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.ToggleKeywordRule(id, req.Status); err != nil {
		response.Fail(c, 1002, "更新失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) listReplyCache(c *gin.Context) {
	tid := tenantFromCtx(c)
	personaRole := c.Query("persona_role")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	list, total, err := h.svc.ListReplyCache(tid, personaRole, page, size)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func (h *Handler) updateReplyCache(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		ResponsesJSON string `json:"responses_json" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.UpdateReplyCacheVariants(id, req.ResponsesJSON); err != nil {
		response.Fail(c, 1002, "更新失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) toggleReplyCache(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.ToggleReplyCache(id, req.Status); err != nil {
		response.Fail(c, 1002, "更新失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) deleteReplyCache(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteReplyCache(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) listTenants(c *gin.Context) {
	list, err := h.svc.ListTenants()
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, list)
}

// updateTenant 编辑租户名称/状态,可选更新 wx 凭证(secret 留空不改)。
func (h *Handler) updateTenant(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		Status string `json:"status"`
		AppID  string `json:"appid"`
		Secret string `json:"secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || id <= 0 {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.UpdateTenant(id, req.Type, req.Name, req.Status, req.AppID, req.Secret); err != nil {
		response.Fail(c, 1001, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) createTenant(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		AppID  string `json:"appid"`
		Secret string `json:"secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	t, err := h.svc.CreateTenant(req.Type, req.Name, req.AppID, req.Secret)
	if err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}
	response.OK(c, t)
}

func (h *Handler) listUsers(c *gin.Context) {
	tid := tenantFromCtx(c) // X-Tenant-ID header 优先,回落 ?tenant_id=;0 = 全部租户
	keyword := c.Query("keyword")
	status := c.Query("status")
	robotFilter := c.Query("robot") // "robot" | "human" | ""
	tag := c.Query("tag")
	google := c.Query("google") // "bound" | "unbound" | ""
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	users, total, err := h.svc.ListUsers(tid, keyword, status, robotFilter, tag, google, page, size)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": users, "total": total})
}

func (h *Handler) listMessages(c *gin.Context) {
	tid, _ := strconv.ParseInt(c.Query("tenant_id"), 10, 64) // 0 = 全部
	uid, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)   // 0 = 不限
	nickname := c.Query("nickname")
	openid := c.Query("openid")
	robotFilter := c.Query("robot") // "robot" | "human" | ""
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := h.svc.ListMessages(tid, uid, nickname, openid, robotFilter, page, size)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func (h *Handler) banUser(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Ban bool `json:"ban"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.BanUser(id, req.Ban); err != nil {
		response.Fail(c, 1002, "操作失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) muteUser(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Mute bool `json:"mute"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.MuteUser(id, req.Mute); err != nil {
		response.Fail(c, 1002, "操作失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) updateUserTags(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Tags string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	// sanitize 的错误信息(超长等)需透给前端提示,故不用笼统文案
	if err := h.svc.UpdateUserTags(id, req.Tags); err != nil {
		response.Fail(c, 1001, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// adjustUserCoins 后台人工调账:delta>0 加币、<0 扣币,remark 记原因进流水。
func (h *Handler) adjustUserCoins(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Delta  int64  `json:"delta"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Delta == 0 || id == 0 {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if n := len([]rune(req.Remark)); n > 64 {
		response.Fail(c, 1001, "备注过长")
		return
	}
	balance, err := h.svc.AdjustCoins(id, req.Delta, req.Remark)
	if err != nil {
		// 余额不足 / 用户不存在 等需原样透给前端
		response.Fail(c, 1002, err.Error())
		return
	}
	response.OK(c, gin.H{"balance": balance})
}

func (h *Handler) startRobotChat(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		BotUserID int64 `json:"bot_user_id,string" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	tid := h.svc.GetUserTenant(userID)
	if tid == 0 {
		tid = h.svc.defaultTenantID()
	}
	chatID, err := h.svc.StartRobotChat(tid, userID, req.BotUserID)
	if err != nil {
		response.FailErr(c, 1002, "发起对话失败", err)
		return
	}
	response.OK(c, gin.H{"chat_id": chatID})
}

func (h *Handler) testLLM(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Prompt == "" {
		req.Prompt = "你好，请回复一句简短的问候语。"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reply, err := robot.TestLLM(ctx, tenantFromCtx(c), req.Prompt)
	if err != nil {
		response.FailErr(c, 1002, "LLM 调用失败", err)
		return
	}
	response.OK(c, gin.H{"reply": reply})
}

func (h *Handler) listChats(c *gin.Context) {
	tid := tenantFromCtx(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := h.svc.ListChats(tid, c.Query("keyword"), page, size)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func (h *Handler) chatDetail(c *gin.Context) {
	chatID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	d, err := h.svc.GetChatDetail(chatID, limit)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, d)
}

func (h *Handler) listRobotChats(c *gin.Context) {
	tid := tenantFromCtx(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := h.svc.ListRobotChats(tid, page, size)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func (h *Handler) robotChatMessages(c *gin.Context) {
	chatID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	msgs, err := h.svc.RobotChatMessages(chatID, limit)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, msgs)
}

func (h *Handler) sendAsRobot(c *gin.Context) {
	chatID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.SendAsRobot(chatID, req.Content); err != nil {
		response.FailErr(c, 1002, "发送失败", err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) listPayOrders(c *gin.Context) {
	tid := tenantFromCtx(c)
	status := c.Query("status")
	userKw := c.Query("user") // 用户ID / 昵称 模糊
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := h.svc.ListPayOrders(tid, status, userKw, page, size)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func (h *Handler) listWalletTxns(c *gin.Context) {
	tid := tenantFromCtx(c)
	direction := c.Query("direction")
	scene := c.Query("scene")
	userKw := c.Query("user")  // 用户ID / 昵称 模糊
	start := c.Query("start")  // YYYY-MM-DD 起(含当天)
	end := c.Query("end")      // YYYY-MM-DD 止(含当天)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := h.svc.ListWalletTxns(tid, direction, scene, userKw, start, end, page, size)
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func (h *Handler) pushUser(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Scene string `json:"scene" binding:"required"`
		F1    string `json:"f1"`
		F2    string `json:"f2"`
		F3    string `json:"f3"`
		Page  string `json:"page"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.PushUser(userID, req.Scene, req.F1, req.F2, req.F3, req.Page); err != nil {
		response.FailErr(c, 1002, "推送失败", err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) listPackages(c *gin.Context) {
	list, err := h.svc.ListPackages()
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) createPackage(c *gin.Context) {
	var req model.CoinPackage
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.CreatePackage(req); err != nil {
		response.Fail(c, 1002, "创建失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) updatePackage(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req model.CoinPackage
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.UpdatePackage(id, req); err != nil {
		response.Fail(c, 1002, "更新失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) deletePackage(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeletePackage(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) listItems(c *gin.Context) {
	list, err := h.svc.ListItems()
	if err != nil {
		response.Fail(c, 1002, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) createItem(c *gin.Context) {
	var req model.Item
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.CreateItem(req); err != nil {
		response.Fail(c, 1002, "创建失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) updateItem(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req model.Item
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.UpdateItem(id, req); err != nil {
		response.Fail(c, 1002, "更新失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) deleteItem(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteItem(id); err != nil {
		response.Fail(c, 1002, "删除失败")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) online(c *gin.Context) {
	if h.svc.chatSvc == nil {
		response.OK(c, []string{})
		return
	}
	ids := h.svc.chatSvc.Hub().OnlineUserIDs()
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = strconv.FormatInt(id, 10)
	}
	response.OK(c, strs)
}

// ---------------- 服务商配置 ----------------

func (h *Handler) listProviders(c *gin.Context) {
	list, err := h.svc.ListProviders(tenantFromCtx(c), c.Param("kind"), i18n.FromContext(c))
	if err != nil {
		response.FailErr(c, 1001, "查询失败", err)
		return
	}
	response.OK(c, gin.H{"cards": list})
}

func (h *Handler) saveProvider(c *gin.Context) {
	var req ProviderSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.SaveProvider(tenantFromCtx(c), c.Param("kind"), c.Param("provider"), req); err != nil {
		response.FailErr(c, 1002, "保存失败", err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// probeProvider 测试连通。失败也返回 200:这是探测结果,不是接口错误。
func (h *Handler) probeProvider(c *gin.Context) {
	msg, err := h.svc.ProbeProvider(tenantFromCtx(c), c.Param("kind"), c.Param("provider"))
	if err != nil {
		response.OK(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	response.OK(c, gin.H{"ok": true, "message": msg})
}
