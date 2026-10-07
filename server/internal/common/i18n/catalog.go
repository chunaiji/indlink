package i18n

// catalog 以**中文原文**为 key。
//
// 为什么不用消息常量:97 处源码一行不改,且查不到时天然回退中文,
// 永远不会有空白或 MISSING_KEY 上线。代价是原文漂移会静默失效 ——
// 由 internal/admin/i18n_coverage_test.go 的 AST 覆盖测试兜住。
//
// 只收录 /admin/api 会下发的消息。C 端消息不在此列(中间件也没挂上去)。
var catalog = map[Lang]map[string]string{
	En: {
		// ── 通用 ───────────────────────────────────────────
		"参数错误":  "Invalid parameter",
		"查询失败":  "Query failed",
		"创建失败":  "Create failed",
		"更新失败":  "Update failed",
		"保存失败":  "Save failed",
		"删除失败":  "Delete failed",
		"发送失败":  "Send failed",
		"操作失败":  "Operation failed",
		"备注过长":  "Remark is too long",

		// ── 金币调账 (service.go AdjustCoins / wallet.AdminAdjust) ──
		"用户不存在":     "User not found",
		"机器人账号不能调账": "Bot accounts cannot be adjusted",
		"钱包服务未配置":   "Wallet service is not configured",
		"数量须在 1~50 之间": "Count must be between 1 and 50",
		"金额非法":      "Invalid amount",
		"金币余额不足":    "Insufficient coin balance",
		"上传未配置": "Upload is not configured",

		// ── 租户凭证 (service.go CreateCredential) ──
		"不支持的平台":      "Unsupported platform",
		"租户不存在":       "Tenant not found",
		"该 AppID 已被占用": "This AppID is already in use",
		"该 AppID 已被其他租户占用": "This AppID is already used by another tenant",

		// ── 服务商配置 (providers.go) ──
		"请先选择租户":            "Select a tenant first",
		"未知的服务商领域":          "Unknown provider category",
		"未知的服务商":            "Unknown provider",
		"服务商存储未初始化":         "Provider store not initialised",
		"请先在「支付」页配置并启用支付宝": "Configure and enable Alipay on the Payments page first",
		"探活未配置":             "Connectivity test not configured",

		// ── 鉴权 (auth.go / handler.go) ────────────────────
		"未登录":       "Not signed in",
		"登录已失效":     "Session expired",
		"签发失败":      "Failed to issue token",
		"用户名或密码错误":  "Incorrect username or password",
		"原密码错误":     "Current password is incorrect",
		"新密码至少 6 位": "New password must be at least 6 characters",

		// ── 配置 ───────────────────────────────────────────
		"非法配置项": "Config key not allowed",

		// ── 租户 (service.go / tenant.go) ──────────────────
		"租户名不能为空":                      "Tenant name is required",
		"小程序租户必须填写 AppID 与 Secret":     "Mini-program tenants require an AppID and Secret",
		"该 wx AppID 已被其他租户占用":          "This WeChat AppID is already taken by another tenant",
		"状态仅支持 active/disabled":        "Status must be either active or disabled",
		"该租户尚无 wx 凭证,首次配置需同时填写 Secret": "This tenant has no WeChat credential yet; Secret is required on first setup",

		// ── 人格批量复制 (service.go) ───────────────────────
		"请先在右上角选择具体租户":      "Select a specific tenant in the top-right corner first",
		"数量必须大于 0":          "Count must be greater than 0",
		"当前已是模板租户,请切换到其他租户": "This is already the template tenant; switch to another one",
		"模板租户暂无可复制的人格":      "The template tenant has no personas to copy",

		// ── 机器人 / 推送 ───────────────────────────────────
		"推送服务未配置":  "Push service is not configured",
		"推送失败":     "Push failed",
		"发起对话失败":   "Failed to start conversation",
		"LLM 调用失败": "LLM request failed",
	},
}

// T 把中文原文译成目标语言。查不到一律返回原文。
func T(lang Lang, zh string) string {
	if lang == ZhCN || lang == "" {
		return zh
	}
	m, ok := catalog[lang]
	if !ok {
		return zh
	}
	if s, ok := m[zh]; ok && s != "" {
		return s
	}
	return zh
}

// Has 报告某语言是否已收录该原文的译文。供 AST 覆盖测试使用。
func Has(lang Lang, zh string) bool {
	m, ok := catalog[lang]
	if !ok {
		return false
	}
	s, ok := m[zh]
	return ok && s != ""
}
