package admin

import (
	"strings"

	"driftbottle/internal/common/i18n"
	"driftbottle/internal/sysconfig"
)

// ConfigField 是下发给后台的配置项,Label / GroupLabel / SectionLabel 已按请求语言解析。
type ConfigField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Group 是稳定 code,不随语言变化 —— 前端用它做逻辑判断
	// (如 AI 引擎分组末尾的 LLM 连接测试按钮)。显示用 GroupLabel。
	Group      string `json:"group"`
	GroupLabel string `json:"group_label"`
	// Section 分区 code,决定这一项落在后台配置页的哪个 Tab。
	// 与 Group 同为稳定标识;显示用 SectionLabel。
	Section      string `json:"section"`
	SectionLabel string `json:"section_label"`
	Type         string `json:"type"` // bool / int / text / textarea / image
	// Value 机密项恒为空 —— 值不出服务端。看「配没配」用 IsSet。
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
	IsSet  bool   `json:"is_set"`
}

// configFieldMeta 是配置项的静态元信息,双语内联。
type configFieldMeta struct {
	Key     string
	LabelZh string
	LabelEn string
	Group   string
	Type    string
	// Secret 机密项:值不下发给浏览器,写空视为「保持不变」。
	Secret bool
	// Platform 端属性,空则继承分组。只在个别配置项与所属分组不一致时才写。
	Platform string
}

// 端属性。决定某项配置对哪种租户可见。
const (
	PlatformBoth = "both"
	PlatformMP   = "miniprogram"
	PlatformApp  = "app"
)

// effectivePlatform 字段没声明就继承分组的。
func effectivePlatform(m configFieldMeta, g groupInfo) string {
	if m.Platform != "" {
		return m.Platform
	}
	return g.Platform
}

// visibleFor 该端属性的配置项对这种租户是否可见。
//
// tenantType 为空 = 后台选的是「全部租户」,此时在配全局默认,必须看得见全部。
func visibleFor(platform, tenantType string) bool {
	if tenantType == "" || platform == PlatformBoth {
		return true
	}
	return platform == tenantType
}

// shouldSkipWrite 这次写入该不该被丢弃。
//
// 机密项写空一律跳过:前端显示的是掩码不是真值,用户 Tab 划过输入框
// 就会触发一次 blur 提交——不挡住的话,一次无意的聚焦就能清掉真密钥。
// 非机密项写空是合法的「清空这项配置」。
func shouldSkipWrite(secret bool, value string) bool {
	return secret && strings.TrimSpace(value) == ""
}

// 分组 code。前端拿它做逻辑判断,**必须稳定**,不能跟着显示名变。
const (
	GroupCommon       = "common"
	GroupPrice        = "price"
	GroupRobot        = "robot"
	GroupMatch        = "match"
	GroupShare        = "share"
	GroupNotice       = "notice"
	GroupAI           = "ai"
	GroupSupport      = "support"
	GroupAds          = "ads"
	GroupMine         = "mine"
	GroupGrowth       = "growth"
	GroupUIText       = "ui_text"
	GroupNav          = "nav"
	GroupPageOverride = "page_override"
	GroupHomeCount    = "home_count"
	GroupQuota        = "quota"
	GroupChat         = "chat"
	GroupQuickReply   = "quick_reply"
	GroupCamera       = "camera"
	GroupPush         = "push"
	GroupRetention    = "retention"
	GroupAppAuth      = "app_auth"
	GroupPayDebug     = "pay_debug"
	GroupSMTP         = "smtp"
	GroupAppPush      = "app_push"
	GroupAppLog       = "app_log"
	GroupAppLegal     = "app_legal"
	GroupAppProfile   = "app_profile"
	GroupAppDiscover  = "app_discover"
	GroupSpark        = "spark"
	GroupAppUIText    = "app_ui_text"
	GroupAppMine      = "app_mine"
	GroupAppSupport   = "app_support"
	GroupAppSplash    = "app_splash"
)

// 分区 code。配置页按分区分 Tab —— 两百多项挤在一页上找不着东西。
// 与分组 code 一样必须稳定:前端路由 /config/:section 用的就是它。
const (
	SectionApp      = "app"
	SectionAI       = "ai"
	SectionUI       = "ui"
	SectionGrowth   = "growth"
	SectionPay      = "pay"
	SectionAds      = "ads"
	SectionSupport  = "support"
	SectionPlatform = "platform"
)

// sectionLabels 分区显示名。⚠️ 顺序不在这里 —— map 无序,Tab 顺序见 sectionOrder。
var sectionLabels = map[string]map[i18n.Lang]string{
	SectionApp:      {i18n.ZhCN: "App", i18n.En: "App"},
	SectionAI:       {i18n.ZhCN: "AI 与机器人", i18n.En: "AI & bots"},
	SectionUI:       {i18n.ZhCN: "界面与文案", i18n.En: "UI & copy"},
	SectionGrowth:   {i18n.ZhCN: "运营与变现", i18n.En: "Growth & revenue"},
	SectionPay:      {i18n.ZhCN: "支付", i18n.En: "Payments"},
	SectionAds:      {i18n.ZhCN: "广告", i18n.En: "Ads"},
	SectionSupport:  {i18n.ZhCN: "客服与跳转", i18n.En: "Support & links"},
	SectionPlatform: {i18n.ZhCN: "系统与安全", i18n.En: "System & safety"},
}

// sectionOrder Tab 的展示顺序。
var sectionOrder = []string{
	SectionApp, SectionAI, SectionUI, SectionGrowth, SectionPay,
	SectionAds, SectionSupport, SectionPlatform,
}

// groupMeta 分组的显示名、所属分区与适用端。
type groupInfo struct {
	LabelZh string
	LabelEn string
	Section string
	// Platform 这一组配置对哪种租户有意义。
	// 小程序租户不该看见 App 日志,App 租户也不该看见微信流量主。
	Platform string
}

var groupMeta = map[string]groupInfo{
	// —— App(Flutter 端专属) ——
	GroupAppAuth:     {"App 登录", "App sign-in", SectionApp, PlatformApp},
	GroupAppProfile:  {"App 完善资料", "App onboarding", SectionApp, PlatformApp},
	GroupAppLog:      {"App 日志", "App logging", SectionApp, PlatformApp},
	GroupAppLegal:    {"App 协议", "App legal", SectionApp, PlatformApp},
	GroupAppPush:     {"App 推送", "App push", SectionApp, PlatformApp},
	GroupAppDiscover: {"App 发现", "App discover", SectionApp, PlatformApp},
	GroupSpark:       {"主动匹配", "Proactive matching", SectionGrowth, PlatformApp},
	GroupAppSplash:   {"App 启动页", "App splash", SectionApp, PlatformApp},

	// —— AI 与机器人(两端共用同一套机器人与 LLM) ——
	GroupAI:    {"AI 引擎", "AI engine", SectionAI, PlatformBoth},
	GroupRobot: {"机器人", "Bots", SectionAI, PlatformBoth},

	// —— 界面与文案 ——
	// 两端页面结构不同,所以各有一套:小程序是 6 个可配 tab + 页面覆盖,
	// App 是 5 个固定 tab(routes.dart:5 明确不走 /api/tabs)。
	// 能共用的只有首页计数 —— 它是纯数字,App 已经在读 /hook-count 了。
	GroupNav:          {"导航", "Navigation", SectionUI, PlatformMP},
	GroupMine:         {"我的功能", "Profile features", SectionUI, PlatformMP},
	GroupUIText:       {"UI 文案", "UI copy", SectionUI, PlatformMP},
	GroupPageOverride: {"页面覆盖", "Page overrides", SectionUI, PlatformMP},
	GroupQuickReply:   {"快捷回复", "Quick replies", SectionUI, PlatformMP},
	GroupHomeCount:    {"首页计数", "Home counters", SectionUI, PlatformBoth},
	GroupAppUIText:    {"App 文案", "App copy", SectionUI, PlatformApp},
	GroupAppMine:      {"App 我的入口", "App profile entries", SectionUI, PlatformApp},

	// —— 运营与变现 ——
	GroupPrice:     {"价格", "Pricing", SectionGrowth, PlatformBoth},
	GroupQuota:     {"每日限额", "Daily quota", SectionGrowth, PlatformBoth},
	GroupGrowth:    {"增长运营", "Growth", SectionGrowth, PlatformBoth},
	GroupRetention: {"留存功能", "Retention", SectionGrowth, PlatformBoth},
	GroupShare:     {"分享", "Sharing", SectionGrowth, PlatformMP},
	GroupNotice:    {"公告", "Announcement", SectionGrowth, PlatformBoth},

	// —— 支付 ——
	// 三端分开:各自的凭证与开关互不相干,混在一组只会让人填错地方。
	// Google Play 暂缺 —— 后端(play-billing 计划 Task 2–5)未实现,
	// 现在摆上配置框只会让人填完以为配好了,而根本没有消费方。
	GroupPayDebug: {"支付联调", "Payment testing", SectionPay, PlatformApp},

	// —— 广告(微信流量主) ——
	GroupAds: {"流量主广告", "Ads", SectionAds, PlatformMP},

	// —— 客服与跳转 ——
	// 小程序这组里 18/20 项是「关联小程序 1-4」,App 跳小程序要接微信 OpenSDK
	// 并做主体绑定,不是配置能解决的,所以整组留给小程序,App 另给一组。
	GroupSupport:    {"客服与跳转", "Support & links", SectionSupport, PlatformMP},
	GroupAppSupport: {"App 客服", "App support", SectionSupport, PlatformApp},

	// —— 系统与安全 ——
	GroupCommon: {"通用", "General", SectionPlatform, PlatformBoth},
	GroupCamera: {"水印相机", "Watermark camera", SectionPlatform, PlatformMP},
	GroupPush:   {"微信订阅消息", "WeChat push", SectionPlatform, PlatformMP},
	GroupMatch:  {"匹配权重", "Match weights", SectionPlatform, PlatformBoth},
	GroupChat:   {"聊天", "Chat", SectionPlatform, PlatformBoth},
	// SMTP 独立成组:它不只服务 App 登录验证码,是一条通用发信能力。
	GroupSMTP: {"邮件发送 (SMTP)", "Email (SMTP)", SectionPlatform, PlatformBoth},
	// 地理合一组:小程序用腾讯(地址水印)、App 用 Google(逆地理),
	// 本是同一件事的两个供应商,原先拆在「水印相机」与「App 地理」两处。
}

// pick 选语言,英文缺失时退回中文。
func pick(zh, en string, lang i18n.Lang) string {
	if lang == i18n.En && en != "" {
		return en
	}
	return zh
}

// resolveField 按语言把静态元信息 + 当前值组装成下发结构。
func resolveField(m configFieldMeta, value string, lang i18n.Lang) ConfigField {
	// 兜底:分组元信息缺失时退回 code 并归入系统分区。
	// 宁可显示一个难看的 code,也不能让配置项从界面上消失。
	groupLabel, section := m.Group, SectionPlatform
	if g, ok := groupMeta[m.Group]; ok {
		groupLabel = pick(g.LabelZh, g.LabelEn, lang)
		section = g.Section
	}
	sectionLabel := section
	if labels, ok := sectionLabels[section]; ok {
		sectionLabel = pick(labels[i18n.ZhCN], labels[i18n.En], lang)
	}
	out := ConfigField{
		Key:   m.Key,
		Label: pick(m.LabelZh, m.LabelEn, lang),
		Group: m.Group, GroupLabel: groupLabel,
		Section: section, SectionLabel: sectionLabel,
		Type: m.Type, Value: value,
	}
	// 机密项:值到此为止,只告诉前端配没配。
	if m.Secret {
		out.Secret = true
		out.IsSet = strings.TrimSpace(value) != ""
		out.Value = ""
	}
	return out
}

// Sections 按固定顺序返回分区列表,供后台渲染 Tab。
func Sections(lang i18n.Lang) []map[string]string {
	out := make([]map[string]string, 0, len(sectionOrder))
	for _, code := range sectionOrder {
		labels := sectionLabels[code]
		out = append(out, map[string]string{
			"code":  code,
			"label": pick(labels[i18n.ZhCN], labels[i18n.En], lang),
		})
	}
	return out
}

// configMeta 是后台可编辑配置的白名单 + 元信息。
// 只允许改这里列出的键,避免写入任意键污染配置表。
//
// ⚠️ 顺序即前端的分组显示顺序,不要重排。
var configMeta = []configFieldMeta{
	// 通用开关 / 价格
	{Key: sysconfig.KeyVerifyRequired, LabelZh: "强制真人认证(发瓶/开聊)", LabelEn: "Require identity verification (post / chat)", Group: GroupCommon, Type: "bool"},
	{Key: sysconfig.KeyIOSRechargeOff, LabelZh: "iOS 隐藏充值入口", LabelEn: "Hide top-up entry on iOS", Group: GroupPrice, Type: "bool"},
	{Key: sysconfig.KeyWSURL, LabelZh: "小程序 WebSocket 地址(空=客户端默认)", LabelEn: "Mini-program WebSocket URL (empty = client default)", Group: GroupCommon, Type: "text"},
	{Key: sysconfig.KeyPriceChat, LabelZh: "开聊消耗金币 M", LabelEn: "Coins to start a chat (M)", Group: GroupPrice, Type: "int"},
	{Key: sysconfig.KeyPriceMsg, LabelZh: "每条消息消耗金币 N(0=关闭)", LabelEn: "Coins per message N (0 = off)", Group: GroupPrice, Type: "int"},
	{Key: sysconfig.KeyChatFreeMsgs, LabelZh: "每个会话前 L 条免费", LabelEn: "Free messages per chat (L)", Group: GroupPrice, Type: "int"},
	{Key: sysconfig.KeyPriceUnlock, LabelZh: "解锁回信消耗金币", LabelEn: "Coins to unlock a reply", Group: GroupPrice, Type: "int"},
	{Key: sysconfig.KeyRegReward, LabelZh: "注册奖励金币", LabelEn: "Signup bonus coins", Group: GroupPrice, Type: "int"},
	{Key: sysconfig.KeyFeedSize, LabelZh: "捞瓶 feed 预生成条数", LabelEn: "Scoop feed pre-generated size", Group: GroupCommon, Type: "int"},

	// 机器人
	{Key: sysconfig.KeyRobotEnabled, LabelZh: "机器人总开关", LabelEn: "Bots master switch", Group: GroupRobot, Type: "bool"},
	{Key: sysconfig.KeyRobotCount, LabelZh: "机器人数量", LabelEn: "Bot count", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyRobotThrowPerHr, LabelZh: "每小时投放瓶子数", LabelEn: "Bottles thrown per hour", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyRobotReplyRatio, LabelZh: "机器人回信比例 (%)", LabelEn: "Bot reply rate (%)", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyRobotBottlePoolTarget, LabelZh: "AI瓶子池目标条数", LabelEn: "AI bottle pool target size", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyCityRobotTopM, LabelZh: "同城前M个位置", LabelEn: "Top M nearby slots", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyCityRobotTopN, LabelZh: "同城前M中机器人数(0关)", LabelEn: "Bots among the top M (0 = off)", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyRobotLanguage, LabelZh: "自动补齐机器人的语言(zh/en)", LabelEn: "Language of auto-created bots (zh/en)", Group: GroupRobot, Type: "text"},
	{Key: sysconfig.KeyRobotProactive, LabelZh: "主动引导总开关", LabelEn: "Proactive nudging master switch", Group: GroupRobot, Type: "bool"},
	{Key: sysconfig.KeyRobotBottle2ChatRatio, LabelZh: "回信后引导加聊概率%", LabelEn: "Chance to invite to chat after a reply (%)", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyRobotOutreachDailyCap, LabelZh: "破冰每租户每日上限", LabelEn: "Outreach daily cap per tenant", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyRobotOutreachNewHours, LabelZh: "破冰-新用户窗口(小时)", LabelEn: "Outreach: new-user window (hours)", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyRobotOutreachSilentDays, LabelZh: "破冰-沉默阈值(天)", LabelEn: "Outreach: silence threshold (days)", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyRobotNudgeSilentMin, LabelZh: "沉默追问阈值(分)", LabelEn: "Follow-up silence threshold (minutes)", Group: GroupRobot, Type: "int"},
	{Key: sysconfig.KeyRobotNudgeMax, LabelZh: "每会话最多追问次数", LabelEn: "Max follow-ups per conversation", Group: GroupRobot, Type: "int"},

	// 匹配权重
	{Key: sysconfig.KeyMatchWTag, LabelZh: "标签重合权重", LabelEn: "Tag overlap weight", Group: GroupMatch, Type: "int"},
	{Key: sysconfig.KeyMatchWCity, LabelZh: "同城权重", LabelEn: "Same-city weight", Group: GroupMatch, Type: "int"},
	{Key: sysconfig.KeyMatchWGender, LabelZh: "取向/异性权重", LabelEn: "Orientation / opposite-gender weight", Group: GroupMatch, Type: "int"},
	{Key: sysconfig.KeyMatchWFresh, LabelZh: "新鲜度权重", LabelEn: "Freshness weight", Group: GroupMatch, Type: "int"},
	{Key: sysconfig.KeyMatchWHeat, LabelZh: "热度权重", LabelEn: "Popularity weight", Group: GroupMatch, Type: "int"},
	{Key: sysconfig.KeyMatchWRobot, LabelZh: "机器人惩罚", LabelEn: "Bot penalty", Group: GroupMatch, Type: "int"},
	{Key: sysconfig.KeyMatchWRandom, LabelZh: "随机扰动", LabelEn: "Random jitter", Group: GroupMatch, Type: "int"},
	{Key: sysconfig.KeyMatchWDist, LabelZh: "距离衰减(双方都有定位才生效)", LabelEn: "Distance decay (needs both sides located)", Group: GroupMatch, Type: "int"},

	// 分享
	{Key: sysconfig.KeyShareTitle, LabelZh: "转发标题", LabelEn: "Share title", Group: GroupShare, Type: "text"},
	{Key: sysconfig.KeyShareImage, LabelZh: "转发卡片图 URL(空=默认截图)", LabelEn: "Share card image URL (empty = auto screenshot)", Group: GroupShare, Type: "text"},

	// 公告
	{Key: sysconfig.KeyNoticeTitle, LabelZh: "公告标题", LabelEn: "Announcement title", Group: GroupNotice, Type: "text"},
	{Key: sysconfig.KeyNoticeBody, LabelZh: "公告正文(空=不弹)", LabelEn: "Announcement body (empty = no popup)", Group: GroupNotice, Type: "textarea"},

	// 导航 Tab 可见性
	{Key: sysconfig.KeyTabHome, LabelZh: "首页 Tab", LabelEn: "Home tab", Group: GroupNav, Type: "bool"},
	{Key: sysconfig.KeyTabCity, LabelZh: "同城 Tab", LabelEn: "Nearby tab", Group: GroupNav, Type: "bool"},
	{Key: sysconfig.KeyTabExpand, LabelZh: "扩列 Tab", LabelEn: "Discover tab", Group: GroupNav, Type: "bool"},
	{Key: sysconfig.KeyTabMessage, LabelZh: "消息 Tab", LabelEn: "Messages tab", Group: GroupNav, Type: "bool"},
	{Key: sysconfig.KeyTabMine, LabelZh: "我的 Tab", LabelEn: "Profile tab", Group: GroupNav, Type: "bool"},
	{Key: sysconfig.KeyTabPrivacy, LabelZh: "相机 Tab", LabelEn: "Camera tab", Group: GroupNav, Type: "bool"},

	// App 端(Flutter):小程序不读这些键
	{Key: sysconfig.KeyAppOTPTTL, LabelZh: "验证码有效期(秒)", LabelEn: "OTP lifetime (seconds)", Group: GroupAppAuth, Type: "int"},
	{Key: sysconfig.KeyAppOTPResend, LabelZh: "同号码重发间隔(秒)", LabelEn: "Resend cooldown per identifier (seconds)", Group: GroupAppAuth, Type: "int"},
	{Key: sysconfig.KeyAppOTPDailyCap, LabelZh: "单号码每日发送上限", LabelEn: "Daily send cap per identifier", Group: GroupAppAuth, Type: "int"},
	{Key: sysconfig.KeyAppOTPIPCap, LabelZh: "单 IP 每小时发送上限", LabelEn: "Hourly send cap per IP", Group: GroupAppAuth, Type: "int"},
	{Key: sysconfig.KeyAppOTPDevCode, LabelZh: "开发态万能码(生产必须留空)", LabelEn: "Dev master code (must be empty in production)", Group: GroupAppAuth, Type: "text", Secret: true},
	{Key: sysconfig.KeyAppSMSProvider, LabelZh: "短信服务商(空=不真发,只落日志)", LabelEn: "SMS provider (empty = log only, nothing sent)", Group: GroupAppAuth, Type: "text"},
	{Key: sysconfig.KeyAppSMSKey, LabelZh: "短信服务商 Key", LabelEn: "SMS provider key", Group: GroupAppAuth, Type: "text", Secret: true},
	{Key: sysconfig.KeyAppSMSSecretEnc, LabelZh: "短信服务商密钥(加密存储)", LabelEn: "SMS provider secret (stored encrypted)", Group: GroupAppAuth, Type: "text", Secret: true},
	{Key: sysconfig.KeyAppSMSTemplate, LabelZh: "短信模板 ID(印度需 DLT 注册)", LabelEn: "SMS template ID (India requires DLT registration)", Group: GroupAppAuth, Type: "text"},
	{Key: sysconfig.KeyAppSMSSign, LabelZh: "短信签名", LabelEn: "SMS signature", Group: GroupAppAuth, Type: "text"},
	{Key: sysconfig.KeyAppSMTPHost, LabelZh: "SMTP 主机(如 smtp.gmail.com)", LabelEn: "SMTP host (e.g. smtp.gmail.com)", Group: GroupSMTP, Type: "text"},
	{Key: sysconfig.KeyAppSMTPPort, LabelZh: "SMTP 端口(465 隐式TLS / 587 STARTTLS)", LabelEn: "SMTP port (465 implicit TLS / 587 STARTTLS)", Group: GroupSMTP, Type: "text"},
	{Key: sysconfig.KeyAppSMTPAccount, LabelZh: "SMTP 登录账号", LabelEn: "SMTP username", Group: GroupSMTP, Type: "text"},
	{Key: sysconfig.KeyAppSMTPTokenEnc, LabelZh: "SMTP 授权码(加密存储,非登录密码)", LabelEn: "SMTP app password (stored encrypted, not the login password)", Group: GroupSMTP, Type: "text", Secret: true},
	{Key: sysconfig.KeyAppSMTPFrom, LabelZh: "SMTP 发件地址", LabelEn: "SMTP from address", Group: GroupSMTP, Type: "text"},
	{Key: sysconfig.KeyAppSMTPFromName, LabelZh: "SMTP 发件人显示名", LabelEn: "SMTP from display name", Group: GroupSMTP, Type: "text"},
	{Key: sysconfig.KeyAppLoginPhoneEnabled, LabelZh: "开放手机号登录/注册", LabelEn: "Allow phone sign-in / sign-up", Group: GroupAppAuth, Type: "bool"},
	{Key: sysconfig.KeyAppLoginEmailEnabled, LabelZh: "开放邮箱登录/注册", LabelEn: "Allow email sign-in / sign-up", Group: GroupAppAuth, Type: "bool"},

	{Key: sysconfig.KeyAppPayMockEnabled, LabelZh: "模拟支付渠道(仅联调,上线前关闭)", LabelEn: "Mock payment channel (testing only, disable before launch)", Group: GroupPayDebug, Type: "bool"},

	{Key: sysconfig.KeyAppDiscoverMaxKM, LabelZh: "发现页距离上限(km)", LabelEn: "Discover distance limit (km)", Group: GroupAppDiscover, Type: "int"},

	{Key: sysconfig.KeyAppRewindPrice, LabelZh: "撤回上一次左滑消耗金币", LabelEn: "Coins to rewind the last left swipe", Group: GroupAppDiscover, Type: "int"},

	{Key: sysconfig.KeyAppDiscoverSkipChargeEnabled, LabelZh: "左滑跳过扣币(默认关)", LabelEn: "Charge coins for left-swipe pass (off by default)", Group: GroupAppDiscover, Type: "bool"},
	{Key: sysconfig.KeyAppDiscoverSkipPrice, LabelZh: "左滑跳过一次消耗金币", LabelEn: "Coins per left-swipe pass", Group: GroupAppDiscover, Type: "int"},

	{Key: sysconfig.KeyAppDiscoverQuizEnabled, LabelZh: "答题匹配(默认关)", LabelEn: "Quiz matching (off by default)", Group: GroupAppDiscover, Type: "bool"},
	{Key: sysconfig.KeyAppDiscoverQuiz, LabelZh: "答题匹配题库(每行: qkey|问题|逗号分隔选项)", LabelEn: "Quiz bank (one per line: qkey|question|comma-separated options)", Group: GroupAppDiscover, Type: "textarea"},

	{Key: sysconfig.KeySparkEnabled, LabelZh: "主动匹配总开关(默认关)", LabelEn: "Proactive matching master switch (off by default)", Group: GroupSpark, Type: "bool"},
	{Key: sysconfig.KeySparkIntervalMin, LabelZh: "每人匹配最小间隔(分钟)", LabelEn: "Minimum interval per user (minutes)", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkIntervalMax, LabelZh: "每人匹配最大间隔(分钟)", LabelEn: "Maximum interval per user (minutes)", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkRealRatio, LabelZh: "配到真人的概率(%,其余为机器人)", LabelEn: "Chance of matching a real user (%); rest are bots", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkWindowStart, LabelZh: "匹配时段开始小时", LabelEn: "Matching window start hour", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkWindowEnd, LabelZh: "匹配时段结束小时(不含)", LabelEn: "Matching window end hour (exclusive)", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkUserDailyCap, LabelZh: "每人每日匹配上限(与主动触达共用)", LabelEn: "Daily matches per user (shared with bot outreach)", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkTitle, LabelZh: "弹窗标题", LabelEn: "Popup title", Group: GroupSpark, Type: "text"},
	{Key: sysconfig.KeySparkTitleEN, LabelZh: "弹窗标题(英文)", LabelEn: "Popup title (English)", Group: GroupSpark, Type: "text"},
	{Key: sysconfig.KeySparkText, LabelZh: "弹窗正文(支持 {nickname})", LabelEn: "Popup body (supports {nickname})", Group: GroupSpark, Type: "text"},
	{Key: sysconfig.KeySparkTextEN, LabelZh: "弹窗正文(英文)", LabelEn: "Popup body (English)", Group: GroupSpark, Type: "text"},

	{Key: sysconfig.KeyAppProfileLanguages, LabelZh: "可选语言(一行一个,用本族自称;空=用客户端内置表)", LabelEn: "Selectable languages (one per line, endonym; empty = client built-in list)", Group: GroupAppProfile, Type: "textarea"},
	{Key: sysconfig.KeyAppProfileInterests, LabelZh: "可选兴趣(一行一条 key|中文|English;空=用客户端内置表)", LabelEn: "Selectable interests (one per line: key|Chinese|English; empty = client built-in list)", Group: GroupAppProfile, Type: "textarea"},
	{Key: sysconfig.KeyAppProfileMinInterests, LabelZh: "至少选几个兴趣", LabelEn: "Minimum interests to pick", Group: GroupAppProfile, Type: "int"},
	{Key: sysconfig.KeyAppProfileMinAge, LabelZh: "年龄下限(低于 18 会被强制抬到 18)", LabelEn: "Minimum age (values below 18 are forced to 18)", Group: GroupAppProfile, Type: "int"},
	{Key: sysconfig.KeyAppProfileMaxAge, LabelZh: "年龄上限", LabelEn: "Maximum age", Group: GroupAppProfile, Type: "int"},

	{Key: sysconfig.KeyAppLegalTerms, LabelZh: "用户协议正文(中文,空=回退内置 H5)", LabelEn: "Terms of service (Chinese; empty = fall back to built-in H5)", Group: GroupAppLegal, Type: "textarea"},
	{Key: sysconfig.KeyAppLegalPrivacy, LabelZh: "隐私政策正文(中文,空=回退内置 H5)", LabelEn: "Privacy policy (Chinese; empty = fall back to built-in H5)", Group: GroupAppLegal, Type: "textarea"},
	{Key: sysconfig.KeyAppLegalTermsEN, LabelZh: "用户协议正文(英文,空=回退中文)", LabelEn: "Terms of service (English; empty = fall back to Chinese)", Group: GroupAppLegal, Type: "textarea"},
	{Key: sysconfig.KeyAppLegalPrivacyEN, LabelZh: "隐私政策正文(英文,空=回退中文)", LabelEn: "Privacy policy (English; empty = fall back to Chinese)", Group: GroupAppLegal, Type: "textarea"},

	{Key: sysconfig.KeyAppPushEnabled, LabelZh: "App 推送总开关(FCM/APNs)", LabelEn: "App push master switch (FCM/APNs)", Group: GroupAppPush, Type: "bool"},

	// App 文案。经 GET /api/app-config 下发,改完下次冷启动生效,不用发版。
	// 全部默认空 = 用 App 内置文案;英文空则回退中文。
	{Key: sysconfig.KeyAppUIAnonSender, LabelZh: "匿名瓶发件人名称(空=用 App 内置文案)", LabelEn: "Anonymous bottle sender name (empty = app built-in copy)", Group: GroupAppUIText, Type: "text"},
	{Key: sysconfig.KeyAppUIAnonSenderEN, LabelZh: "匿名瓶发件人名称(英文,空=回退中文)", LabelEn: "Anonymous bottle sender name (English; empty = fall back to Chinese)", Group: GroupAppUIText, Type: "text"},
	{Key: sysconfig.KeyAppUIOceanTitle, LabelZh: "海洋页标题(空=用 App 内置文案)", LabelEn: "Ocean page title (empty = app built-in copy)", Group: GroupAppUIText, Type: "text"},
	{Key: sysconfig.KeyAppUIOceanTitleEN, LabelZh: "海洋页标题(英文,空=回退中文)", LabelEn: "Ocean page title (English; empty = fall back to Chinese)", Group: GroupAppUIText, Type: "text"},
	{Key: sysconfig.KeyAppUIQuotaTitle, LabelZh: "次数用完弹框标题({n}=每日次数,空=用 App 内置文案)", LabelEn: "Quota-exhausted dialog title ({n} = daily quota; empty = app built-in copy)", Group: GroupAppUIText, Type: "text"},
	{Key: sysconfig.KeyAppUIQuotaTitleEN, LabelZh: "次数用完弹框标题(英文,空=回退中文)", LabelEn: "Quota-exhausted dialog title (English; empty = fall back to Chinese)", Group: GroupAppUIText, Type: "text"},
	{Key: sysconfig.KeyAppUIQuotaBody, LabelZh: "次数用完弹框正文(空=用 App 内置文案)", LabelEn: "Quota-exhausted dialog body (empty = app built-in copy)", Group: GroupAppUIText, Type: "textarea"},
	{Key: sysconfig.KeyAppUIQuotaBodyEN, LabelZh: "次数用完弹框正文(英文,空=回退中文)", LabelEn: "Quota-exhausted dialog body (English; empty = fall back to Chinese)", Group: GroupAppUIText, Type: "textarea"},

	// App「我的」入口显隐。默认全开 —— 这是 App 现在的样子,关掉才是改变。
	// 刻意没有「设置」和「客服」开关,原因见 sysconfig.go 那批键上的注释。
	{Key: sysconfig.KeyAppFnWallet, LabelZh: "余额卡片", LabelEn: "Balance card", Group: GroupAppMine, Type: "bool"},
	{Key: sysconfig.KeyAppFnRecharge, LabelZh: "充值按钮", LabelEn: "Top-up button", Group: GroupAppMine, Type: "bool"},
	{Key: sysconfig.KeyAppFnWalletLog, LabelZh: "金币流水", LabelEn: "Coin ledger", Group: GroupAppMine, Type: "bool"},
	{Key: sysconfig.KeyAppFnItems, LabelZh: "道具", LabelEn: "Gift shop", Group: GroupAppMine, Type: "bool"},
	{Key: sysconfig.KeyAppFnBlocklist, LabelZh: "黑名单", LabelEn: "Blocklist", Group: GroupAppMine, Type: "bool"},
	{Key: sysconfig.KeyAppPrivacyGateEnabled, LabelZh: "首次登录前隐私授权弹框(合规,默认关)", LabelEn: "Privacy consent gate before login (off by default)", Group: GroupAppMine, Type: "bool"},

	// App 客服。空则 App 回退到内置 mailto:,所以不配也一定有求助入口。
	{Key: sysconfig.KeyAppContactText, LabelZh: "客服弹框文案(空=用 App 内置邮箱)", LabelEn: "Support dialog text (empty = app built-in email)", Group: GroupAppSupport, Type: "textarea"},
	{Key: sysconfig.KeyAppContactTextEN, LabelZh: "客服弹框文案(英文,空=回退中文)", LabelEn: "Support dialog text (English; empty = fall back to Chinese)", Group: GroupAppSupport, Type: "textarea"},
	{Key: sysconfig.KeyAppContactImage, LabelZh: "客服图片(如二维码,不分语种)", LabelEn: "Support image (e.g. QR code, language-independent)", Group: GroupAppSupport, Type: "image"},
	{Key: sysconfig.KeyAppSplashImage1, LabelZh: "启动页图片 1(竖图,空=内置启动页)", LabelEn: "Splash image 1 (portrait; empty = built-in)", Group: GroupAppSplash, Type: "image"},
	{Key: sysconfig.KeyAppSplashImage2, LabelZh: "启动页图片 2", LabelEn: "Splash image 2", Group: GroupAppSplash, Type: "image"},
	{Key: sysconfig.KeyAppSplashImage3, LabelZh: "启动页图片 3", LabelEn: "Splash image 3", Group: GroupAppSplash, Type: "image"},
	{Key: sysconfig.KeyAppSplashImage4, LabelZh: "启动页图片 4", LabelEn: "Splash image 4", Group: GroupAppSplash, Type: "image"},
	{Key: sysconfig.KeyAppSplashImage5, LabelZh: "启动页图片 5", LabelEn: "Splash image 5", Group: GroupAppSplash, Type: "image"},

	// App 本地日志。改这几项后用户下次登录或拉资料即生效,不必重装。
	{Key: sysconfig.KeyAppLogEnabled, LabelZh: "App 日志总开关", LabelEn: "App logging master switch", Group: GroupAppLog, Type: "bool"},
	{Key: sysconfig.KeyAppLogLevel, LabelZh: "最低记录级别(debug/info/warn/error)", LabelEn: "Minimum log level (debug/info/warn/error)", Group: GroupAppLog, Type: "text"},
	{Key: sysconfig.KeyAppLogBody, LabelZh: "记录请求/响应体(排查时临时开,用完务必关)", LabelEn: "Log request/response bodies (debugging only, turn off after)", Group: GroupAppLog, Type: "bool"},
	{Key: sysconfig.KeyAppLogMaxMB, LabelZh: "本地日志总量上限(MB)", LabelEn: "Local log size cap (MB)", Group: GroupAppLog, Type: "int"},
	{Key: sysconfig.KeyAppLogRetainDays, LabelZh: "本地日志保留天数", LabelEn: "Local log retention (days)", Group: GroupAppLog, Type: "int"},

	// 全站页面图片覆盖(同城/首页/扩列/消息/充值/道具/流水/订单/收藏/动态)
	{Key: sysconfig.KeyPagesCoverOn, LabelZh: "页面覆盖总开关(同城/首页/扩列/消息/充值/道具/流水/订单/收藏/动态)", LabelEn: "Page override master switch (nearby / home / discover / messages / top-up / shop / ledger / orders / collection / moments)", Group: GroupPageOverride, Type: "bool"},
	{Key: sysconfig.KeyPagesCoverImage, LabelZh: "覆盖图 URL(空=用默认图)", LabelEn: "Override image URL (empty = default image)", Group: GroupPageOverride, Type: "text"},

	// 我的-其他功能项 可见性
	{Key: sysconfig.KeyFnVerify, LabelZh: "真人认证", LabelEn: "Identity verification", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnAvatar, LabelZh: "换头像", LabelEn: "Change avatar", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnViewed, LabelZh: "浏览记录", LabelEn: "Browsing history", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnItems, LabelZh: "道具商城(默认隐藏)", LabelEn: "Gift shop (hidden by default)", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnCollection, LabelZh: "我的收藏", LabelEn: "My collection", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnWallet, LabelZh: "我的钱包(余额卡片)", LabelEn: "My wallet (balance card)", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnRecharge, LabelZh: "充值入口(默认隐藏)", LabelEn: "Top-up entry (hidden by default)", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnWalletLog, LabelZh: "金币流水(默认隐藏)", LabelEn: "Coin ledger (hidden by default)", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnOrders, LabelZh: "我的订单(默认隐藏)", LabelEn: "My orders (hidden by default)", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnBlocklist, LabelZh: "黑名单", LabelEn: "Blocklist", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnContact, LabelZh: "客服", LabelEn: "Support", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnSettings, LabelZh: "设置", LabelEn: "Settings", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyFnMoments, LabelZh: "我的动态", LabelEn: "My moments", Group: GroupMine, Type: "bool"},

	// 推送模板
	{Key: sysconfig.KeyPushTplReply, LabelZh: "回信推送模板 ID", LabelEn: "Reply push template ID", Group: GroupPush, Type: "text"},
	{Key: sysconfig.KeyPushTplChat, LabelZh: "聊天推送模板 ID", LabelEn: "Chat push template ID", Group: GroupPush, Type: "text"},
	{Key: sysconfig.KeyPushTplActivity, LabelZh: "活动预约提醒模板 ID", LabelEn: "Event reminder template ID", Group: GroupPush, Type: "text"},
	{Key: sysconfig.KeyPushTplWorkRecommend, LabelZh: "新作品推荐提醒模板 ID", LabelEn: "New content recommendation template ID", Group: GroupPush, Type: "text"},
	{Key: sysconfig.KeyPushTplCheckin, LabelZh: "签到提醒模板 ID", LabelEn: "Check-in reminder template ID", Group: GroupPush, Type: "text"},
	{Key: sysconfig.KeyPushWxAppID, LabelZh: "推送用 AppID", LabelEn: "Push AppID", Group: GroupPush, Type: "text"},
	{Key: sysconfig.KeyPushWxSecret, LabelZh: "推送用 AppSecret", LabelEn: "Push AppSecret", Group: GroupPush, Type: "text", Secret: true},
	{Key: sysconfig.KeyPushSubscribePrompt, LabelZh: "订阅弹框引导开关", LabelEn: "Subscription prompt switch", Group: GroupPush, Type: "bool"},

	// AI 引擎
	{Key: sysconfig.KeyAIBotEnabled, LabelZh: "AI 回复总开关", LabelEn: "AI reply master switch", Group: GroupAI, Type: "bool"},
	{Key: sysconfig.KeyAIChatEnabled, LabelZh: "聊天续接 AI 开关", LabelEn: "AI chat continuation switch", Group: GroupAI, Type: "bool"},
	{Key: sysconfig.KeyLLMAPIEndpoint, LabelZh: "LLM API 地址（OpenAI 兼容）", LabelEn: "LLM API endpoint (OpenAI-compatible)", Group: GroupAI, Type: "text"},
	{Key: sysconfig.KeyLLMAPIKey, LabelZh: "LLM API Key（加密存储）", LabelEn: "LLM API key (stored encrypted)", Group: GroupAI, Type: "text", Secret: true},
	{Key: sysconfig.KeyLLMModel, LabelZh: "模型名", LabelEn: "Model name", Group: GroupAI, Type: "text"},
	{Key: sysconfig.KeyLLMTemperature, LabelZh: "Temperature × 10（8 = 0.8）", LabelEn: "Temperature × 10 (8 = 0.8)", Group: GroupAI, Type: "int"},
	{Key: sysconfig.KeyLLMMaxTokens, LabelZh: "单次回复最大 Token", LabelEn: "Max tokens per reply", Group: GroupAI, Type: "int"},
	{Key: sysconfig.KeyAIReplyDelayMin, LabelZh: "模拟打字最小延迟（ms）", LabelEn: "Simulated typing delay, min (ms)", Group: GroupAI, Type: "int"},
	{Key: sysconfig.KeyAIReplyDelayMax, LabelZh: "模拟打字最大延迟（ms）", LabelEn: "Simulated typing delay, max (ms)", Group: GroupAI, Type: "int"},
	{Key: sysconfig.KeyLLMConcurrency, LabelZh: "LLM 并发上限", LabelEn: "LLM concurrency limit", Group: GroupAI, Type: "int"},

	// 每日次数限制
	{Key: sysconfig.KeyQuotaThrowDaily, LabelZh: "每日免费扔瓶次数", LabelEn: "Free throws per day", Group: GroupQuota, Type: "int"},
	{Key: sysconfig.KeyQuotaScoopDaily, LabelZh: "每日免费捞瓶次数", LabelEn: "Free scoops per day", Group: GroupQuota, Type: "int"},
	{Key: sysconfig.KeyQuotaThrowPack, LabelZh: "扔瓶次数包(每个给N次)", LabelEn: "Throw pack size (N per pack)", Group: GroupQuota, Type: "int"},
	{Key: sysconfig.KeyQuotaScoopPack, LabelZh: "捞瓶次数包(每个给N次)", LabelEn: "Scoop pack size (N per pack)", Group: GroupQuota, Type: "int"},

	// UI 文案
	{Key: sysconfig.KeyUITextAnonSender, LabelZh: "匿名瓶发件人名称", LabelEn: "Anonymous bottle sender name", Group: GroupUIText, Type: "text"},
	{Key: sysconfig.KeyUITextAnonFriend, LabelZh: "弹框匿名用户称呼", LabelEn: "Anonymous user label in dialogs", Group: GroupUIText, Type: "text"},
	{Key: sysconfig.KeyUITextSomeFriend, LabelZh: "弹框普通用户称呼", LabelEn: "Regular user label in dialogs", Group: GroupUIText, Type: "text"},
	{Key: sysconfig.KeyUITextNavTitle, LabelZh: "海洋页导航标题", LabelEn: "Ocean page title", Group: GroupUIText, Type: "text"},
	{Key: sysconfig.KeyUITextChatBanner, LabelZh: "聊天页横幅文案", LabelEn: "Chat page banner text", Group: GroupUIText, Type: "text"},
	{Key: sysconfig.KeyUITextQuotaTitle, LabelZh: "次数用完弹框标题", LabelEn: "Quota-exhausted dialog title", Group: GroupUIText, Type: "text"},
	{Key: sysconfig.KeyUITextQuotaMsg, LabelZh: "次数用完弹框内容", LabelEn: "Quota-exhausted dialog body", Group: GroupUIText, Type: "text"},

	// 聊天
	{Key: sysconfig.KeyLowBalanceThreshold, LabelZh: "余额不足阈值(猛币,0关)", LabelEn: "Low balance threshold (coins, 0 = off)", Group: GroupChat, Type: "int"},
	{Key: sysconfig.KeyLowBalanceMsg, LabelZh: "余额不足系统消息", LabelEn: "Low balance system message", Group: GroupChat, Type: "text"},

	// 快捷回复(逗号分隔)
	{Key: sysconfig.KeyChatQuicks, LabelZh: "聊天页快捷回复(逗号分隔)", LabelEn: "Chat quick replies (comma separated)", Group: GroupQuickReply, Type: "text"},
	{Key: sysconfig.KeyReplyQuicks, LabelZh: "回信页快捷回复(逗号分隔)", LabelEn: "Bottle reply quick replies (comma separated)", Group: GroupQuickReply, Type: "text"},

	// 首页计数 + 回信脱敏
	{Key: sysconfig.KeyHookBase, LabelZh: "今日回应数·基数", LabelEn: "Today's responses · base count", Group: GroupHomeCount, Type: "int"},
	{Key: sysconfig.KeyHookAddPerMin, LabelZh: "每分钟随机增加上限", LabelEn: "Max random increase per minute", Group: GroupHomeCount, Type: "int"},
	{Key: sysconfig.KeyHookSubPerMin, LabelZh: "每分钟随机减少上限", LabelEn: "Max random decrease per minute", Group: GroupHomeCount, Type: "int"},
	{Key: sysconfig.KeyOnlineBase, LabelZh: "在线人数基数(0=不显示)", LabelEn: "Online count base (0 = hide)", Group: GroupHomeCount, Type: "int"},
	{Key: sysconfig.KeyOnlineJitter, LabelZh: "在线人数每分钟抖动(±)", LabelEn: "Online count jitter per minute (±)", Group: GroupHomeCount, Type: "int"},
	{Key: sysconfig.KeyOceanBottleCount, LabelZh: "App 海面漂浮瓶子数(5~9)", LabelEn: "Bottles floating on the App sea (5–9)", Group: GroupHomeCount, Type: "int"},
	{Key: sysconfig.KeyReplyMaskLen, LabelZh: "回信明文字数(其余脱敏)", LabelEn: "Plain-text characters in a reply (rest masked)", Group: GroupPrice, Type: "int"},

	// 增长运营
	{Key: sysconfig.KeyCheckinEnabled, LabelZh: "每日签到总开关", LabelEn: "Daily check-in master switch", Group: GroupGrowth, Type: "bool"},
	{Key: sysconfig.KeyCheckinCoins, LabelZh: "签到发放金币 N(阶梯后备)", LabelEn: "Check-in coins N (ladder fallback)", Group: GroupGrowth, Type: "int"},
	{Key: sysconfig.KeyCheckinLadder, LabelZh: "7天阶梯币数(逗号分隔)", LabelEn: "7-day ladder coins (comma separated)", Group: GroupGrowth, Type: "text"},
	{Key: sysconfig.KeyCheckinMakeupLimit, LabelZh: "每月看视频补签上限(0=关)", LabelEn: "Monthly video make-up check-ins (0 = off)", Group: GroupGrowth, Type: "int"},

	// 水印相机
	{Key: sysconfig.KeyWmTileText, LabelZh: "平铺水印文字", LabelEn: "Tiled watermark text", Group: GroupCamera, Type: "text"},
	{Key: sysconfig.KeyWmTileColor, LabelZh: "平铺水印颜色(rgba,含透明度)", LabelEn: "Tiled watermark color (rgba with alpha)", Group: GroupCamera, Type: "text"},
	{Key: sysconfig.KeyWmTileSize, LabelZh: "平铺水印字号(px)", LabelEn: "Tiled watermark font size (px)", Group: GroupCamera, Type: "int"},

	// 微信内容安全(过审要求;openid 取自用户,支付宝用户自动跳过在线检测)
	// 注:检测结果回调依赖微信"消息推送"(启用会影响客服消息),暂不启用;
	// 如需回调自动删违规图,加回 KeySecCallbackToken 配置项并在小程序后台配消息推送。

	{Key: sysconfig.KeyMineTipOn, LabelZh: "完善资料跑马灯开关(我的页)", LabelEn: "Profile completion marquee switch (profile page)", Group: GroupMine, Type: "bool"},
	{Key: sysconfig.KeyMineTipText, LabelZh: "完善资料跑马灯文案", LabelEn: "Profile completion marquee text", Group: GroupMine, Type: "text"},

	// 客服 + 首页关联小程序(最多4个,icon 为 image 类型可直接上传)
	{Key: sysconfig.KeyContactText, LabelZh: "联系客服弹框文案", LabelEn: "Support dialog text", Group: GroupSupport, Type: "textarea"},
	{Key: sysconfig.KeyContactImage, LabelZh: "联系客服图片(如客服微信二维码)", LabelEn: "Support image (e.g. WeChat QR code)", Group: GroupSupport, Type: "image"},
	{Key: sysconfig.KeyLinkMPEnabled, LabelZh: "首页关联小程序入口开关", LabelEn: "Linked mini-program entry switch (home)", Group: GroupSupport, Type: "bool"},
	{Key: sysconfig.KeyLinkMPTitle, LabelZh: "入口胶囊文案", LabelEn: "Entry pill text", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP1AppID, LabelZh: "小程序1 AppID", LabelEn: "Mini-program 1 AppID", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP1Title, LabelZh: "小程序1 名称", LabelEn: "Mini-program 1 name", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP1Icon, LabelZh: "小程序1 图标", LabelEn: "Mini-program 1 icon", Group: GroupSupport, Type: "image"},
	{Key: sysconfig.KeyLinkMP1Path, LabelZh: "小程序1 页面路径(可空)", LabelEn: "Mini-program 1 page path (optional)", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP2AppID, LabelZh: "小程序2 AppID", LabelEn: "Mini-program 2 AppID", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP2Title, LabelZh: "小程序2 名称", LabelEn: "Mini-program 2 name", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP2Icon, LabelZh: "小程序2 图标", LabelEn: "Mini-program 2 icon", Group: GroupSupport, Type: "image"},
	{Key: sysconfig.KeyLinkMP2Path, LabelZh: "小程序2 页面路径(可空)", LabelEn: "Mini-program 2 page path (optional)", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP3AppID, LabelZh: "小程序3 AppID", LabelEn: "Mini-program 3 AppID", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP3Title, LabelZh: "小程序3 名称", LabelEn: "Mini-program 3 name", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP3Icon, LabelZh: "小程序3 图标", LabelEn: "Mini-program 3 icon", Group: GroupSupport, Type: "image"},
	{Key: sysconfig.KeyLinkMP3Path, LabelZh: "小程序3 页面路径(可空)", LabelEn: "Mini-program 3 page path (optional)", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP4AppID, LabelZh: "小程序4 AppID", LabelEn: "Mini-program 4 AppID", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP4Title, LabelZh: "小程序4 名称", LabelEn: "Mini-program 4 name", Group: GroupSupport, Type: "text"},
	{Key: sysconfig.KeyLinkMP4Icon, LabelZh: "小程序4 图标", LabelEn: "Mini-program 4 icon", Group: GroupSupport, Type: "image"},
	{Key: sysconfig.KeyLinkMP4Path, LabelZh: "小程序4 页面路径(可空)", LabelEn: "Mini-program 4 page path (optional)", Group: GroupSupport, Type: "text"},

	// 留存四功能(默认全关,工具形态勿开)
	{Key: sysconfig.KeyBottleTraceEnabled, LabelZh: "漂流轨迹开关", LabelEn: "Drift trace switch", Group: GroupRetention, Type: "bool"},
	{Key: sysconfig.KeyNightBottleEnabled, LabelZh: "深夜瓶开关", LabelEn: "Night bottle switch", Group: GroupRetention, Type: "bool"},
	{Key: sysconfig.KeyNightStart, LabelZh: "深夜场开始(小时0-23)", LabelEn: "Night session start (hour 0-23)", Group: GroupRetention, Type: "int"},
	{Key: sysconfig.KeyNightEnd, LabelZh: "深夜场结束(次日小时)", LabelEn: "Night session end (hour, next day)", Group: GroupRetention, Type: "int"},
	{Key: sysconfig.KeyNightPushTitle, LabelZh: "深夜场推送文案", LabelEn: "Night session push text", Group: GroupRetention, Type: "text"},
	{Key: sysconfig.KeyUserCardEnabled, LabelZh: "用户资料卡开关", LabelEn: "User profile card switch", Group: GroupRetention, Type: "bool"},
	{Key: sysconfig.KeyCharmRankEnabled, LabelZh: "魅力周榜开关", LabelEn: "Weekly charm ranking switch", Group: GroupRetention, Type: "bool"},
	{Key: sysconfig.KeySquareEnabled, LabelZh: "动态广场开关(扩列tab变朋友圈)", LabelEn: "Moments square switch (turns the Discover tab into a feed)", Group: GroupRetention, Type: "bool"},
	{Key: sysconfig.KeyShareRewardEnabled, LabelZh: "分享奖励总开关", LabelEn: "Share reward master switch", Group: GroupGrowth, Type: "bool"},
	{Key: sysconfig.KeyShareRewardCoins, LabelZh: "单次分享奖励 M", LabelEn: "Coins per share M", Group: GroupGrowth, Type: "int"},
	{Key: sysconfig.KeyShareRewardDailyLimit, LabelZh: "每日分享领奖次数上限", LabelEn: "Daily share reward cap", Group: GroupGrowth, Type: "int"},
	{Key: sysconfig.KeyOutreachEnabled, LabelZh: "机器人主动触达总开关", LabelEn: "Bot outreach master switch", Group: GroupGrowth, Type: "bool"},
	{Key: sysconfig.KeyOutreachWindowStart, LabelZh: "触达时段开始(小时0-23)", LabelEn: "Outreach window start (hour 0-23)", Group: GroupGrowth, Type: "int"},
	{Key: sysconfig.KeyOutreachWindowEnd, LabelZh: "触达时段结束(小时0-23)", LabelEn: "Outreach window end (hour 0-23)", Group: GroupGrowth, Type: "int"},
	{Key: sysconfig.KeyOutreachProbability, LabelZh: "活跃用户抽样比例(%)", LabelEn: "Active user sampling rate (%)", Group: GroupGrowth, Type: "int"},
	{Key: sysconfig.KeyOutreachLookbackMin, LabelZh: "回溯活跃窗口(分钟)", LabelEn: "Activity lookback window (minutes)", Group: GroupGrowth, Type: "int"},
	{Key: sysconfig.KeyOutreachTickMin, LabelZh: "触达调度间隔(分钟)", LabelEn: "Outreach scheduling interval (minutes)", Group: GroupGrowth, Type: "int"},
	{Key: sysconfig.KeyOutreachUserDailyCap, LabelZh: "同用户每日触达上限", LabelEn: "Daily outreach cap per user", Group: GroupGrowth, Type: "int"},

	// 流量主广告 — 总开关 + 激励参数
	{Key: sysconfig.KeyAdEnabled, LabelZh: "流量主总开关", LabelEn: "Ads master switch", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdInterGapSec, LabelZh: "插屏最小间隔(秒)", LabelEn: "Interstitial minimum gap (seconds)", Group: GroupAds, Type: "int"},
	{Key: sysconfig.KeyAdRewardCoins, LabelZh: "激励视频每次金币", LabelEn: "Coins per rewarded video", Group: GroupAds, Type: "int"},
	{Key: sysconfig.KeyAdRewardDaily, LabelZh: "激励视频每日上限", LabelEn: "Rewarded videos per day", Group: GroupAds, Type: "int"},
	// 各类型广告位 ID(同类型共用,只需各填一个)
	{Key: sysconfig.KeyAdBannerUnit, LabelZh: "原生模板广告位ID(信息流/列表位,原Banner)", LabelEn: "Native template unit ID (feed / list slots, formerly Banner)", Group: GroupAds, Type: "text"},
	{Key: sysconfig.KeyAdInterUnit, LabelZh: "插屏广告位ID", LabelEn: "Interstitial unit ID", Group: GroupAds, Type: "text"},
	{Key: sysconfig.KeyAdRewardUnit, LabelZh: "激励视频广告位ID", LabelEn: "Rewarded video unit ID", Group: GroupAds, Type: "text"},
	{Key: sysconfig.KeyAdNativeUnit, LabelZh: "原生模板广告位ID(我的页宫格)", LabelEn: "Native template unit ID (profile grid)", Group: GroupAds, Type: "text"},
	// 各广告位开关(共用所属类型的广告位ID)
	{Key: sysconfig.KeyAdBannerOceanOn, LabelZh: "[原生·信息流] 首页", LabelEn: "[Native · feed] Home", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerCityOn, LabelZh: "[原生·信息流] 同城", LabelEn: "[Native · feed] Nearby", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerExpandOn, LabelZh: "[原生·信息流] 扩列", LabelEn: "[Native · feed] Discover", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerMessageOn, LabelZh: "[原生·信息流] 消息", LabelEn: "[Native · feed] Messages", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerMineOn, LabelZh: "[原生·信息流] 我的", LabelEn: "[Native · feed] Profile", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerDetailOn, LabelZh: "[原生·信息流] 详情", LabelEn: "[Native · feed] Detail", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerChatOn, LabelZh: "[原生·信息流] 聊天", LabelEn: "[Native · feed] Chat", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerCollectionOn, LabelZh: "[原生·信息流] 收藏", LabelEn: "[Native · feed] Collection", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerOrdersOn, LabelZh: "[原生·信息流] 订单", LabelEn: "[Native · feed] Orders", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerWalletlogOn, LabelZh: "[原生·信息流] 流水", LabelEn: "[Native · feed] Ledger", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerViewedOn, LabelZh: "[原生·信息流] 浏览记录", LabelEn: "[Native · feed] Browsing history", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdBannerPrivacyOn, LabelZh: "[原生·信息流] 相机", LabelEn: "[Native · feed] Camera", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdInterScoopOn, LabelZh: "[插屏] 捞瓶后", LabelEn: "[Interstitial] After scooping", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdInterDetailOn, LabelZh: "[插屏] 进详情", LabelEn: "[Interstitial] Entering detail", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdRewardCoinOn, LabelZh: "[激励] 看视频领金币", LabelEn: "[Rewarded] Watch video for coins", Group: GroupAds, Type: "bool"},
	{Key: sysconfig.KeyAdGridMineOn, LabelZh: "[原生] 我的页", LabelEn: "[Native] Profile page", Group: GroupAds, Type: "bool"},
}

// isSecretKey 该键是否为机密项。写入端用它决定空值要不要丢弃。
func isSecretKey(k string) bool {
	for _, f := range configMeta {
		if f.Key == k {
			return f.Secret
		}
	}
	return false
}

// allowedKeys 用于校验 PUT 写入的键是否在白名单内。
func allowedKey(k string) bool {
	for _, f := range configMeta {
		if f.Key == k {
			return true
		}
	}
	return false
}
