package sysconfig

import (
	"strings"
	"time"

	"driftbottle/internal/common/jwtutil"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

// Handler 公开只读的系统配置接口(可选鉴权:有合法 Bearer 取其租户,否则用默认租户)。
type Handler struct {
	defaultTenant int64

	// appTenant App 端专用的回落租户(APP_DEFAULT_TENANT_ID)。
	//
	// 只有 /app-config 用它。App 与小程序是两个租户,而 /app-config 免鉴权
	// (冷启动时还没登录),回落到小程序租户就会下发一份不属于 App 的配置 ——
	// 2026-09-22 线上的 Google 登录就是这么空转的:client ID 配在 App 租户,
	// 接口却去问小程序租户要,拿回空串。
	//
	// 为 0(单租户部署没设这个环境变量)时回落 defaultTenant。
	appTenant int64

	// credLookup 查某租户某平台的凭证 appid(来自 tenant.Store);nil = 单租户部署,一律视为未配置。
	// /app-config 用它决定微信 / 支付宝按钮露不露:开关开 && 有凭证。
	credLookup CredLookup

	// payUsable provider.Store.Usable 的支付投影:渠道「启用且必填齐全」才对客户端露出;nil = 全不可用。
	payUsable func(tenantID int64, provider string) bool

	// mapProvider 当前生效的地图服务商(tencent/google/amap/baidu),App 端将来据此切地图 SDK;nil / 没配 = ""。
	mapProvider func(tenantID int64) string

	// ssoUsable / ssoField provider.Store 的 sso 域投影:登录渠道「启用且必填齐全」
	// 才对客户端露出。nil = 单租户部署或没接,一律视为不可用。
	//
	// sysconfig 包不 import provider——注入进来,和 payUsable 同一个路子。
	ssoUsable func(tenantID int64, provider string) bool
	ssoField  func(tenantID int64, provider, key string) string
}

// WithSSOUsable 注入第三方登录渠道可用性(main.go 传 providerStore.Usable)。
func (h *Handler) WithSSOUsable(fn func(int64, string) bool) *Handler {
	h.ssoUsable = fn
	return h
}

// WithSSOField 注入第三方登录卡片的字段取值(main.go 传 providerStore.Get + Resolved.Get)。
func (h *Handler) WithSSOField(fn func(int64, string, string) string) *Handler {
	h.ssoField = fn
	return h
}

func (h *Handler) sso(tid int64, p string) bool { return h.ssoUsable != nil && h.ssoUsable(tid, p) }

func (h *Handler) ssoVal(tid int64, p, k string) string {
	if h.ssoField == nil {
		return ""
	}
	return h.ssoField(tid, p, k)
}

// WithMapProvider 注入地图服务商查询。
func (h *Handler) WithMapProvider(fn func(int64) string) *Handler {
	h.mapProvider = fn
	return h
}

func (h *Handler) mapProv(tid int64) string {
	if h.mapProvider == nil {
		return ""
	}
	return h.mapProvider(tid)
}

// WithPayUsable 注入支付渠道可用性(main.go 传 providerStore.Usable)。
func (h *Handler) WithPayUsable(fn func(int64, string) bool) *Handler {
	h.payUsable = fn
	return h
}

func (h *Handler) pay(tid int64, p string) bool { return h.payUsable != nil && h.payUsable(tid, p) }

// CredLookup 返回 (appid, 是否已配置)。
type CredLookup func(tenantID int64, platform string) (appID string, ok bool)

func NewHandler(defaultTenant, appTenant int64) *Handler {
	return &Handler{defaultTenant: defaultTenant, appTenant: appTenant}
}

// WithCredLookup 注入凭证查询(main.go 传 credStore.ByTenantPlatform)。
func (h *Handler) WithCredLookup(fn CredLookup) *Handler {
	h.credLookup = fn
	return h
}

func (h *Handler) credAppID(tid int64, platform string) (string, bool) {
	if h.credLookup == nil {
		return "", false
	}
	return h.credLookup(tid, platform)
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

// appTenantOf /app-config 专用的租户解析:有 Bearer 用它的租户,否则回落 App 租户。
//
// ⚠️ 不要把这条并进 tenantOf —— /notice /tabs /features /ads 都在用那个,
// 那些是小程序的接口,一起改等于把小程序的免登配置整个换成 App 的。
func (h *Handler) appTenantOf(c *gin.Context) int64 {
	a := c.GetHeader("Authorization")
	if strings.HasPrefix(a, "Bearer ") {
		if claims, err := jwtutil.Parse(strings.TrimPrefix(a, "Bearer ")); err == nil {
			return claims.TenantID
		}
	}
	if h.appTenant != 0 {
		return h.appTenant
	}
	return h.defaultTenant
}

func (h *Handler) Register(api *gin.RouterGroup) {
	api.GET("/notice", h.getNotice)
	api.GET("/tabs", h.getTabs)
	api.GET("/mine-functions", h.getMineFunctions)
	api.GET("/hook-count", h.getHookCount)
	api.GET("/ads", h.getAds)
	api.GET("/pages-config", h.getPagesConfig)
	api.GET("/features", h.getFeatures)
	// 法务文本。必须免鉴权——登录页的「同意用户协议」就要能点开。
	api.GET("/legal/:doc", h.getLegal)
	// 完善资料的可选项(语言/兴趣/年龄范围),运营后台可配。
	api.GET("/profile-options", h.getProfileOptions)
	// App 启动配置(文案/入口显隐/客服)。免鉴权:冷启动时还没登录就要用。
	api.GET("/app-config", h.getAppConfig)
}

// getAppConfig 一次下发 App 启动要用的运营配置。
//
// 为什么合成一个接口而不让 App 分头调 /features + /mine-functions + …:
// 冷启动串行四个请求,首屏要么干等要么先渲染再跳变。
//
// 语种在这里读 ?lang= 而不是挂语言中间件 —— 那条中间件只挂 /admin/api,
// C 端恒定中文是 response/isolation_test.go 守着的线。/legal 当初也是这么绕的。
//
// 所有文案默认空串,空 = App 用内置 ARB 文案。这批键刻意不复用小程序的
// ui_text_* / fn_show_* / contact_*:默认值是全局的没法按端分叉,详见 sysconfig.go。
func (h *Handler) getAppConfig(c *gin.Context) {
	tid := h.appTenantOf(c)
	en := strings.HasPrefix(strings.ToLower(c.Query("lang")), "en")
	// text 英文请求先取 _en,空则回退中文键 —— 少配一门语言不该变成开天窗。
	text := func(zhKey, enKey string) string {
		if en {
			if v := GetString(tid, enKey); v != "" {
				return v
			}
		}
		return GetString(tid, zhKey)
	}
	show := func(k string) bool { return GetString(tid, k) != "0" }
	response.OK(c, gin.H{
		"support": gin.H{
			"text":  text(KeyAppContactText, KeyAppContactTextEN),
			"image": GetString(tid, KeyAppContactImage),
		},
		// 启动页图片(去空),App 静默缓存、下次冷启动轮播其一。
		"splash": gin.H{"images": SplashImages(tid)},
		// 海面同时漂几个瓶子(5~9)。
		"ocean": gin.H{"bottle_count": OceanBottleCount(tid)},
		"mine": gin.H{
			"items":      show(KeyAppFnItems),
			"wallet":     show(KeyAppFnWallet),
			"recharge":   show(KeyAppFnRecharge),
			"wallet_log": show(KeyAppFnWalletLog),
			"blocklist":  show(KeyAppFnBlocklist),
		},
		// 第三方登录渠道。来源是服务商配置(后台「服务商 → 登录」页):启用且必填齐全。
		// 四家一个判法——迁移前这里是三种写法:微信判开关+凭证、支付宝判开关+**错的**凭证
		// (密钥其实在支付卡片上)、Google 和 Apple 压根不判。
		"auth": gin.H{
			// 手机号 / 邮箱登录注册开关,默认都开;App 侧两个都关按都开处理。不属于服务商。
			"phone": show(KeyAppLoginPhoneEnabled),
			"email": show(KeyAppLoginEmailEnabled),

			"wechat":                h.sso(tid, "wechat"),
			"wechat_app_id":         h.ssoVal(tid, "wechat", "app_id"), // 客户端注册微信 SDK 用
			"wechat_universal_link": h.ssoVal(tid, "wechat", "universal_link"),
			"alipay":                h.sso(tid, "alipay"),
			"apple":                 h.sso(tid, "apple"),
			"google":                h.sso(tid, "google"),
			// 多值时取第一个,约定第一个是 Web client ID(见 user.primaryClientID)。
			// 安卓的 Google 登录必须拿它当 serverClientId,否则 idToken 为 null,
			// 症状是「点了没反应」——服务端日志里连一条请求都不会有。
			"google_client_id": firstCSV(h.ssoVal(tid, "google", "client_id")),
		},
		"pay": gin.H{
			"wechat":      h.pay(tid, "wechat"),
			"alipay":      h.pay(tid, "alipay"),
			"apple":       h.pay(tid, "apple"),
			"google_play": h.pay(tid, "google_play"),
		},
		// 当前生效的地图服务商(后台「服务商 → 地图」单选)。这轮只下发,端上切 SDK 另开。
		"map": gin.H{"provider": h.mapProv(tid)},
		"copy": gin.H{
			"anon_sender": text(KeyAppUIAnonSender, KeyAppUIAnonSenderEN),
			"ocean_title": text(KeyAppUIOceanTitle, KeyAppUIOceanTitleEN),
			"quota_title": text(KeyAppUIQuotaTitle, KeyAppUIQuotaTitleEN),
			"quota_body":  text(KeyAppUIQuotaBody, KeyAppUIQuotaBodyEN),
		},
		// 首次登录前隐私授权弹框开关(合规,默认关)。前端 !privacyAgreed && 开时才弹。
		"privacy_gate": GetString(tid, KeyAppPrivacyGateEnabled) == "1",
		// 聊天扣费规则(M / N / L),App 只拿来显示(打招呼按钮的价、聊天页的提示);真扣在服务端。
		"pricing": gin.H{
			"chat_start":     GetInt(tid, KeyPriceChat),
			"chat_msg":       GetInt(tid, KeyPriceMsg),
			"chat_free_msgs": GetInt(tid, KeyChatFreeMsgs),
			// 发现页:撤回单价 / 左滑扣币开关与单价。以前进发现页才拉(鉴权接口),
			// 第一次点撤回时还没回来,弹层写 0 币(真机反馈);启动配置一次带齐。
			"rewind":              GetInt(tid, KeyAppRewindPrice),
			"skip_charge_enabled": GetString(tid, KeyAppDiscoverSkipChargeEnabled) == "1",
			"skip_price":          GetInt(tid, KeyAppDiscoverSkipPrice),
		},
	})
}

// OceanBottleCount 海面瓶子数,夹在 [5, 9]——少于 5 海面显得空,多于 9 水带放不下。
func OceanBottleCount(tid int64) int {
	n := GetInt(tid, KeyOceanBottleCount)
	if n < 5 {
		return 5
	}
	if n > 9 {
		return 9
	}
	return n
}

// SplashImages 已配置的启动页图片 URL(去空,保持 1→5 顺序)。
func SplashImages(tid int64) []string {
	out := []string{}
	for _, k := range []string{KeyAppSplashImage1, KeyAppSplashImage2, KeyAppSplashImage3, KeyAppSplashImage4, KeyAppSplashImage5} {
		if v := strings.TrimSpace(GetString(tid, k)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// getProfileOptions 下发「完善资料」页要用的选项，运营后台可改，加一门语言不用发版。
//
// 语言/兴趣留空时返回空数组，客户端回退到内置表——不能因为运营没配就让注册走不下去。
func (h *Handler) getProfileOptions(c *gin.Context) {
	tid := h.tenantOf(c)
	minAge := GetInt(tid, KeyAppProfileMinAge)
	if minAge < 18 {
		minAge = 18 // 合规底线，后台填错也不放行未成年
	}
	maxAge := GetInt(tid, KeyAppProfileMaxAge)
	if maxAge <= minAge {
		maxAge = 60
	}
	response.OK(c, gin.H{
		"languages":     splitLines(GetString(tid, KeyAppProfileLanguages)),
		"interests":     parseInterests(GetString(tid, KeyAppProfileInterests)),
		"min_interests": GetInt(tid, KeyAppProfileMinInterests),
		"min_age":       minAge,
		"max_age":       maxAge,
	})
}

// splitLines 按行拆分并去空行，供 textarea 型配置使用。
func splitLines(s string) []string {
	out := make([]string, 0, 8)
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// parseInterests 解析 `key|中文|English` 一行一条。
// 缺中英文时用 key 兜底，少一列不至于让整条配置失效。
func parseInterests(s string) []gin.H {
	lines := splitLines(s)
	out := make([]gin.H, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, "|")
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		zh, en := key, key
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			zh = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
			en = strings.TrimSpace(parts[2])
		}
		out = append(out, gin.H{"key": key, "zh": zh, "en": en})
	}
	return out
}

// getLegal 下发用户协议 / 隐私政策正文(后台可改,改文案不用发版)。
//
// 回退顺序:请求语种 → 中文 → 空。正文为空时客户端回退到内置 H5 地址,
// 所以这里给空串是合法结果,不是错误。
func (h *Handler) getLegal(c *gin.Context) {
	// 协议只有 App 在用(小程序走自己的页面),而且登录页就要打开——那时没有 Bearer,
	// tenantOf 会回落到小程序租户,后台配在 App 租户下的正文就"准备中"了(真机反馈)。
	tid := h.appTenantOf(c)
	zhKey, enKey := KeyAppLegalTerms, KeyAppLegalTermsEN
	if c.Param("doc") == "privacy" {
		zhKey, enKey = KeyAppLegalPrivacy, KeyAppLegalPrivacyEN
	}
	body := GetString(tid, zhKey)
	if strings.HasPrefix(strings.ToLower(c.Query("lang")), "en") {
		if en := GetString(tid, enKey); en != "" {
			body = en
		}
	}
	response.OK(c, gin.H{"content": body})
}

// getFeatures 返回留存四功能开关(默认全关,工具形态不露出)+ 首页关联小程序配置。
func (h *Handler) getFeatures(c *gin.Context) {
	tid := h.tenantOf(c)
	response.OK(c, gin.H{
		"bottle_trace": GetString(tid, KeyBottleTraceEnabled) == "1",
		"night_bottle": GetString(tid, KeyNightBottleEnabled) == "1",
		"night_start":  GetInt(tid, KeyNightStart),
		"night_end":    GetInt(tid, KeyNightEnd),
		"user_card":    GetString(tid, KeyUserCardEnabled) == "1",
		"charm_rank":   GetString(tid, KeyCharmRankEnabled) == "1",
		"square":       GetString(tid, KeySquareEnabled) == "1",
		"link_mp":      h.linkMPConfig(tid),
		"camera_wm": gin.H{ // 水印相机品牌平铺水印样式
			"text":  GetString(tid, KeyWmTileText),
			"color": GetString(tid, KeyWmTileColor),
			"size":  GetInt(tid, KeyWmTileSize),
		},
	})
}

// linkMPConfig 组装首页关联小程序配置:最多 4 组,appid 为空的组跳过。
func (h *Handler) linkMPConfig(tid int64) gin.H {
	groups := [][4]string{
		{KeyLinkMP1AppID, KeyLinkMP1Title, KeyLinkMP1Icon, KeyLinkMP1Path},
		{KeyLinkMP2AppID, KeyLinkMP2Title, KeyLinkMP2Icon, KeyLinkMP2Path},
		{KeyLinkMP3AppID, KeyLinkMP3Title, KeyLinkMP3Icon, KeyLinkMP3Path},
		{KeyLinkMP4AppID, KeyLinkMP4Title, KeyLinkMP4Icon, KeyLinkMP4Path},
	}
	list := make([]gin.H, 0, 4)
	for _, g := range groups {
		appid := GetString(tid, g[0])
		if appid == "" {
			continue
		}
		list = append(list, gin.H{
			"appid": appid,
			"title": GetString(tid, g[1]),
			"icon":  GetString(tid, g[2]),
			"path":  GetString(tid, g[3]),
		})
	}
	return gin.H{
		"on":    GetString(tid, KeyLinkMPEnabled) == "1" && len(list) > 0,
		"title": GetString(tid, KeyLinkMPTitle),
		"list":  list,
	}
}

// getAds 按租户下发流量主广告配置。unit 按类型解析(banner/插屏/激励/原生各一个),
// 各广告位只有开关;输出仍是按位 {on, unit},客户端无需改动。
func (h *Handler) getAds(c *gin.Context) {
	tid := h.tenantOf(c)
	bannerUnit := GetString(tid, KeyAdBannerUnit)
	interUnit := GetString(tid, KeyAdInterUnit)
	rewardUnit := GetString(tid, KeyAdRewardUnit)
	nativeUnit := GetString(tid, KeyAdNativeUnit)
	slot := func(onKey, unit string) gin.H {
		return gin.H{"on": GetString(tid, onKey) == "1", "unit": unit}
	}
	reward := slot(KeyAdRewardCoinOn, rewardUnit)
	reward["coins"] = GetInt64(tid, KeyAdRewardCoins)
	reward["daily"] = GetInt(tid, KeyAdRewardDaily)
	response.OK(c, gin.H{
		"enabled":       GetString(tid, KeyAdEnabled) == "1",
		"inter_gap_sec": GetInt(tid, KeyAdInterGapSec),
		"slots": gin.H{
			"banner_ocean":      slot(KeyAdBannerOceanOn, bannerUnit),
			"banner_city":       slot(KeyAdBannerCityOn, bannerUnit),
			"banner_expand":     slot(KeyAdBannerExpandOn, bannerUnit),
			"banner_message":    slot(KeyAdBannerMessageOn, bannerUnit),
			"banner_mine":       slot(KeyAdBannerMineOn, bannerUnit),
			"banner_detail":     slot(KeyAdBannerDetailOn, bannerUnit),
			"banner_chat":       slot(KeyAdBannerChatOn, bannerUnit),
			"banner_collection": slot(KeyAdBannerCollectionOn, bannerUnit),
			"banner_orders":     slot(KeyAdBannerOrdersOn, bannerUnit),
			"banner_walletlog":  slot(KeyAdBannerWalletlogOn, bannerUnit),
			"banner_viewed":     slot(KeyAdBannerViewedOn, bannerUnit),
			"banner_privacy":    slot(KeyAdBannerPrivacyOn, bannerUnit),
			"inter_scoop":       slot(KeyAdInterScoopOn, interUnit),
			"inter_detail":      slot(KeyAdInterDetailOn, interUnit),
			"reward_coin":       reward,
			"grid_mine":         slot(KeyAdGridMineOn, nativeUnit),
		},
	})
}

// getHookCount 首页「今日已有 N 条回应」:base + 当日已过分钟数 × 每分钟净增(add-sub)+ 抖动。
// 按天计算,每天回到 base 再增长;运营在管理台配 base/add/sub。
func (h *Handler) getHookCount(c *gin.Context) {
	tid := h.tenantOf(c)
	base := GetInt(tid, KeyHookBase)
	add := GetInt(tid, KeyHookAddPerMin)
	sub := GetInt(tid, KeyHookSubPerMin)
	now := time.Now()
	mins := now.Hour()*60 + now.Minute()
	net := add - sub
	if net < 0 {
		net = 0
	}
	// 抖动:用「当前分钟」做确定性伪随机,避免每次请求大幅跳动
	jitter := (now.Hour()*60 + now.Minute()) % (add + 1)
	count := base + mins*net + jitter
	if count < base {
		count = base
	}
	response.OK(c, gin.H{"count": count, "online": OnlineCount(tid, now)})
}

// OnlineCount 「N 人在线」的展示数。
//
// 真在线数在冷启动期是个位数,放在海面上等于告诉用户"这里没人"。做法与回应总数一致:
// 运营给一个基数,服务端按时段曲线起伏(晚间高峰、凌晨低谷),再按分钟做确定性抖动,
// 同一分钟内所有请求拿到同一个数,不会刷新一下跳一下。base 为 0 返回 0。
func OnlineCount(tid int64, now time.Time) int {
	base := GetInt(tid, KeyOnlineBase)
	if base <= 0 {
		return 0
	}
	// 24 小时曲线:凌晨 2–6 点最低(0.35),午后平稳(1.0),20–23 点高峰(1.45)。
	curve := [24]float64{
		0.7, 0.5, 0.35, 0.3, 0.3, 0.35, 0.5, 0.7, 0.85, 0.95, 1.0, 1.0,
		1.05, 1.0, 0.95, 0.95, 1.0, 1.1, 1.25, 1.35, 1.45, 1.45, 1.3, 1.0,
	}
	h := now.Hour()
	// 小时之间线性插值,整点不跳变。
	next := curve[(h+1)%24]
	f := curve[h] + (next-curve[h])*float64(now.Minute())/60
	jitter := GetInt(tid, KeyOnlineJitter)
	if jitter < 0 {
		jitter = 0
	}
	// 以「当天第几分钟」做种子的确定性抖动,落在 [-jitter, +jitter]。
	seed := now.YearDay()*1440 + h*60 + now.Minute()
	j := 0
	if jitter > 0 {
		j = (seed*7919)%(2*jitter+1) - jitter
	}
	n := int(float64(base)*f) + j
	if n < 1 {
		n = 1
	}
	return n
}

// getMineFunctions 返回「我的-其他功能」各项是否显示(运营管理台可配)。
func (h *Handler) getMineFunctions(c *gin.Context) {
	tid := h.tenantOf(c)
	show := func(k string) bool { return GetString(tid, k) != "0" }
	response.OK(c, gin.H{
		"verify":            show(KeyFnVerify),
		"avatar":            show(KeyFnAvatar),
		"viewed":            show(KeyFnViewed),
		"items":             show(KeyFnItems),
		"collection":        show(KeyFnCollection),
		"wallet":            show(KeyFnWallet),
		"orders":            show(KeyFnOrders),
		"recharge":          show(KeyFnRecharge),
		"walletLog":         show(KeyFnWalletLog),
		"blocklist":         show(KeyFnBlocklist),
		"contact":           show(KeyFnContact),
		"settings":          show(KeyFnSettings),
		"moments":           show(KeyFnMoments),
		"contact_text":      GetString(tid, KeyContactText),      // 客服弹框文案(后台可配)
		"contact_image":     GetString(tid, KeyContactImage),     // 客服弹框图片(如客服微信二维码,可空)
		"complete_tip":      GetString(tid, KeyMineTipOn) != "0", // 完善资料跑马灯开关
		"complete_tip_text": GetString(tid, KeyMineTipText),      // 跑马灯文案
	})
}

// getNotice 返回首页公告(body 空时前端不弹框)。
func (h *Handler) getNotice(c *gin.Context) {
	tid := h.tenantOf(c)
	response.OK(c, gin.H{
		"title": GetString(tid, KeyNoticeTitle),
		"body":  GetString(tid, KeyNoticeBody),
	})
}

// getTabs 返回各 Tab 是否显示。
func (h *Handler) getTabs(c *gin.Context) {
	tid := h.tenantOf(c)
	response.OK(c, gin.H{
		"home":    GetString(tid, KeyTabHome) != "0",
		"city":    GetString(tid, KeyTabCity) != "0",
		"expand":  GetString(tid, KeyTabExpand) != "0",
		"message": GetString(tid, KeyTabMessage) != "0",
		"mine":    GetString(tid, KeyTabMine) != "0",
		"privacy": GetString(tid, KeyTabPrivacy) != "0",
	})
}

// getPagesConfig 返回全站页面覆盖开关与图片地址(图片空时前端用默认图)。
func (h *Handler) getPagesConfig(c *gin.Context) {
	tid := h.tenantOf(c)
	response.OK(c, gin.H{
		"cover_on":    GetString(tid, KeyPagesCoverOn) == "1",
		"cover_image": GetString(tid, KeyPagesCoverImage),
	})
}

// firstCSV 取逗号分隔配置的第一个值。
//
// 与 user.primaryClientID 同语义,刻意各写一份:sysconfig 被 user 依赖,
// 反过来引就是导入环。两处都只有五行,共享它反而要新开一个包。
// 约定在 app_google_client_id 的注释里,两边改动请一起改。
func firstCSV(raw string) string {
	for _, p := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(p); v != "" {
			return v
		}
	}
	return ""
}
