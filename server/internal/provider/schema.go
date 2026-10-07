// Package provider 第三方服务商(支付 / 地图 / 内容安全)的声明式配置:
// 服务商长什么样写在这里,后台页面按它渲染,业务包按它取值。加一家服务商 = 加一个 Definition。
package provider

import "strings"

const (
	KindPay        = "pay"
	KindMap        = "map"
	KindModeration = "moderation"
	KindSSO        = "sso"

	PlatformBoth = "both"
	PlatformMP   = "miniprogram"
	PlatformApp  = "app"
)

type Field struct {
	Key      string
	LabelZh  string
	LabelEn  string
	Type     string // text / textarea / bool / select
	Secret   bool
	Required bool
	Options  []string
	Default  string
	HelpZh   string
	HelpEn   string
}

// Ref 指向另一张卡片。
type Ref struct{ Kind, Provider string }

type Definition struct {
	Kind     string
	Provider string
	LabelZh  string
	LabelEn  string
	Platform string // 对哪种租户显示
	Fields   []Field
	LookupA  string // 投影到 ProviderConfig.LookupA 的字段键(回调反查)
	LookupB  string
	DocURL   string
	// DependsOn 本卡片的凭证来自另一张卡片(同租户)。
	//
	// 用于「同一个第三方应用同时服务两个域」的情形:支付宝登录与支付
	// 是同一个应用、同一把私钥(internal/user/alipay_auth.go 一直这么取)。让登录卡片
	// 自己再存一份私钥,运营就要填两遍,迟早填歪一个——而症状是「支付好的,
	// 登录报签名错误」。
	//
	// 只支持一跳:被依赖的卡片自己不能再声明 DependsOn。
	DependsOn *Ref
}

const notifyBase = "https://ambertu.com/message/api/pay/callback/"

var definitions = []Definition{
	// ---------------- 支付 ----------------
	{
		Kind: KindPay, Provider: "wechat", LabelZh: "微信支付", LabelEn: "WeChat Pay", Platform: PlatformBoth,
		LookupA: "mch_id", LookupB: "platform_serial", DocURL: "https://pay.weixin.qq.com",
		Fields: []Field{
			{Key: "app_id", LabelZh: "AppID(小程序 / 开放平台移动应用)", LabelEn: "AppID (mini-program / open-platform app)", Type: "text", Required: true,
				HelpZh: "小程序租户填小程序 AppID,App 租户填开放平台移动应用 AppID;必须与商户号绑定", HelpEn: "Mini-program AppID for mini-program tenants, Open Platform app AppID for App tenants; must be bound to the merchant"},
			{Key: "mch_id", LabelZh: "商户号", LabelEn: "Merchant ID", Type: "text", Required: true},
			{Key: "apiv3_key", LabelZh: "APIv3 密钥", LabelEn: "APIv3 key", Type: "text", Secret: true, Required: true},
			{Key: "serial_no", LabelZh: "商户 API 证书序列号", LabelEn: "Merchant certificate serial", Type: "text", Required: true},
			{Key: "private_key", LabelZh: "商户 API 私钥 PEM", LabelEn: "Merchant private key PEM", Type: "textarea", Secret: true, Required: true},
			{Key: "platform_key", LabelZh: "微信支付平台公钥 PEM(选填)", LabelEn: "WeChat Pay platform public key PEM (optional)", Type: "textarea",
				HelpZh: "只用于支付回调验签，下单不需要。不填可以正常发起支付，但回调会被拒掉、订单不会自动到账——正式收款前务必补上",
				HelpEn: "Used only to verify payment callbacks, not to place orders. Without it payment still starts, but callbacks are rejected and orders are never credited; fill it in before taking real money"},
			{Key: "platform_serial", LabelZh: "平台公钥 ID(PUB_KEY_ID_…，选填)", LabelEn: "Platform key ID (PUB_KEY_ID_…, optional)", Type: "text",
				HelpZh: "和平台公钥配套，用来比对回调头 Wechatpay-Serial。留空则不比对",
				HelpEn: "Pairs with the platform public key to match the Wechatpay-Serial callback header; left empty, the header is not checked"},
			{Key: "notify_url", LabelZh: "回调地址", LabelEn: "Notify URL", Type: "text", Default: notifyBase + "wechat"},
		},
	},
	{
		Kind: KindPay, Provider: "alipay", LabelZh: "支付宝", LabelEn: "Alipay", Platform: PlatformBoth,
		LookupA: "app_id", DocURL: "https://open.alipay.com",
		Fields: []Field{
			{Key: "app_id", LabelZh: "应用 AppID", LabelEn: "App ID", Type: "text", Required: true},
			{Key: "private_key", LabelZh: "应用私钥", LabelEn: "App private key", Type: "textarea", Secret: true, Required: true},
			{Key: "alipay_public_key", LabelZh: "支付宝公钥(不是应用公钥)", LabelEn: "Alipay public key (not the app public key)", Type: "textarea", Required: true},
			{Key: "pid", LabelZh: "商户 PID(2088 开头，选填)", LabelEn: "Partner ID (2088…, optional)", Type: "text",
				HelpZh: "只给「支付宝授权登录」用，跟收款无关。没有就留空，不影响支付",
				HelpEn: "Only used for Alipay sign-in, not for payment. Leave empty if the merchant has none"},
			{Key: "notify_url", LabelZh: "回调地址", LabelEn: "Notify URL", Type: "text", Default: notifyBase + "alipay"},
			{Key: "sandbox", LabelZh: "沙箱环境", LabelEn: "Sandbox", Type: "bool", Default: "0"},
		},
	},
	{
		Kind: KindPay, Provider: "apple", LabelZh: "iOS 内购(App Store)", LabelEn: "iOS in-app purchase (App Store)", Platform: PlatformApp,
		LookupA: "bundle_id", DocURL: "https://appstoreconnect.apple.com",
		Fields: []Field{
			{Key: "bundle_id", LabelZh: "Bundle ID", LabelEn: "Bundle ID", Type: "text", Required: true},
			{Key: "issuer_id", LabelZh: "App Store Connect Issuer ID", LabelEn: "App Store Connect Issuer ID", Type: "text", Required: true},
			{Key: "key_id", LabelZh: "内购私钥 Key ID", LabelEn: "IAP key ID", Type: "text", Required: true},
			{Key: "p8_key", LabelZh: ".p8 私钥", LabelEn: ".p8 private key", Type: "textarea", Secret: true, Required: true},
			{Key: "sandbox", LabelZh: "允许沙盒票据入账(上线前关闭)", LabelEn: "Accept sandbox receipts (turn off before launch)", Type: "bool", Default: "0"},
		},
	},
	{
		Kind: KindPay, Provider: "google_play", LabelZh: "Google Play 结算", LabelEn: "Google Play Billing", Platform: PlatformApp,
		LookupA: "package_name", DocURL: "https://play.google.com/console",
		Fields: []Field{
			{Key: "package_name", LabelZh: "应用包名", LabelEn: "Package name", Type: "text", Required: true},
			{Key: "service_account_json", LabelZh: "服务账号 JSON", LabelEn: "Service account JSON", Type: "textarea", Secret: true, Required: true,
				HelpZh: "Google Cloud 服务账号密钥文件全文,需在 Play Console 授予「查看财务数据」与「管理订单」", HelpEn: "Full service-account key file; grant it order/financial permissions in Play Console"},
		},
	},
	// ---------------- 第三方登录(App 专属) ----------------
	{
		Kind: KindSSO, Provider: "wechat", LabelZh: "微信登录", LabelEn: "WeChat sign-in", Platform: PlatformApp,
		DocURL: "https://open.weixin.qq.com",
		Fields: []Field{
			{Key: "app_id", LabelZh: "开放平台移动应用 AppID", LabelEn: "Open Platform app AppID", Type: "text", Required: true,
				HelpZh: "微信开放平台「移动应用」的 AppID,不是小程序的;要通过应用审核才能用",
				HelpEn: "AppID of the WeChat Open Platform mobile app, not the mini-program; the app must pass review first"},
			{Key: "app_secret", LabelZh: "AppSecret", LabelEn: "AppSecret", Type: "text", Secret: true, Required: true},
			{Key: "universal_link", LabelZh: "Universal Link(iOS 必填)", LabelEn: "Universal Link (required on iOS)", Type: "text",
				HelpZh: "iOS 微信 SDK 必填,安卓忽略。不填只影响 iOS,安卓登录照常",
				HelpEn: "Required by the iOS WeChat SDK and ignored on Android; leaving it empty only breaks iOS"},
		},
	},
	{
		Kind: KindSSO, Provider: "alipay", LabelZh: "支付宝登录(密钥来自支付页)", LabelEn: "Alipay sign-in (keys come from the payment page)", Platform: PlatformApp,
		DependsOn: &Ref{Kind: KindPay, Provider: "alipay"}, DocURL: "https://open.alipay.com",
		Fields: []Field{
			{Key: "pid", LabelZh: "商户 PID(2088 开头)", LabelEn: "Partner ID (2088…)", Type: "text", Required: true,
				HelpZh: "App 授权登录签名要用。应用 AppID 与私钥与「支付 → 支付宝」共用同一个应用,在那边配",
				HelpEn: "Needed to sign the app sign-in request. The app ID and private key are shared with Providers → Payment → Alipay; set them there"},
		},
	},
	{
		Kind: KindSSO, Provider: "google", LabelZh: "Google 登录", LabelEn: "Google sign-in", Platform: PlatformApp,
		DocURL: "https://console.cloud.google.com/apis/credentials",
		Fields: []Field{
			{Key: "client_id", LabelZh: "Client ID(多个用逗号分隔)", LabelEn: "Client ID (comma-separated for several)", Type: "text", Required: true,
				HelpZh: "校验 Google ID Token 的 aud。安卓还要用第一个当 serverClientId,所以第一个必须是 Web client ID",
				HelpEn: "Audiences accepted when verifying the Google ID token. Android also uses the first one as serverClientId, so put the Web client ID first"},
		},
	},
	{
		Kind: KindSSO, Provider: "apple", LabelZh: "Apple 登录", LabelEn: "Apple sign-in", Platform: PlatformApp,
		DocURL: "https://developer.apple.com/account/resources/identifiers",
		Fields: []Field{
			{Key: "bundle_id", LabelZh: "Bundle ID(多个用逗号分隔)", LabelEn: "Bundle ID (comma-separated for several)", Type: "text", Required: true,
				HelpZh: "⚠️ 关掉这一家在 iOS 上有下架风险:App Store 审核指南 4.8 要求,只要提供了任何第三方登录,就必须同时提供 Apple 登录",
				HelpEn: "⚠️ Turning this off risks iOS rejection: App Store guideline 4.8 requires Sign in with Apple whenever any other third-party sign-in is offered"},
		},
	},
	// ---------------- 地图(单选) ----------------
	{Kind: KindMap, Provider: "tencent", LabelZh: "腾讯位置服务", LabelEn: "Tencent Location", Platform: PlatformBoth, DocURL: "https://lbs.qq.com",
		Fields: []Field{{Key: "key", LabelZh: "Key", LabelEn: "Key", Type: "text", Secret: true, Required: true}}},
	{Kind: KindMap, Provider: "google", LabelZh: "Google Maps", LabelEn: "Google Maps", Platform: PlatformBoth, DocURL: "https://console.cloud.google.com/google/maps-apis",
		Fields: []Field{{Key: "key", LabelZh: "Geocoding API Key", LabelEn: "Geocoding API key", Type: "text", Secret: true, Required: true}}},
	{Kind: KindMap, Provider: "amap", LabelZh: "高德地图", LabelEn: "AMap", Platform: PlatformBoth, DocURL: "https://console.amap.com",
		Fields: []Field{{Key: "key", LabelZh: "Web 服务 Key", LabelEn: "Web service key", Type: "text", Secret: true, Required: true}}},
	{Kind: KindMap, Provider: "baidu", LabelZh: "百度地图", LabelEn: "Baidu Maps", Platform: PlatformBoth, DocURL: "https://lbsyun.baidu.com",
		Fields: []Field{
			{Key: "ak", LabelZh: "AK", LabelEn: "AK", Type: "text", Secret: true, Required: true},
			{Key: "sk", LabelZh: "SK(开了 SN 校验才填)", LabelEn: "SK (only if SN check is on)", Type: "text", Secret: true},
		}},
	// ---------------- 内容安全(单选) ----------------
	{Kind: KindModeration, Provider: "wechat", LabelZh: "微信内容安全", LabelEn: "WeChat content security", Platform: PlatformMP, DocURL: "https://developers.weixin.qq.com/miniprogram/dev/OpenApiDoc/sec-center/sec-check/msgSecCheck.html",
		Fields: []Field{
			{Key: "text_on", LabelZh: "文本检测(msgSecCheck)", LabelEn: "Text check (msgSecCheck)", Type: "bool", Default: "1"},
			{Key: "image_on", LabelZh: "图片检测(mediaCheckAsync)", LabelEn: "Image check (mediaCheckAsync)", Type: "bool", Default: "1"},
		}},
	{Kind: KindModeration, Provider: "alipay", LabelZh: "支付宝内容安全", LabelEn: "Alipay content security", Platform: PlatformBoth, DocURL: "https://opendocs.alipay.com/open/02fnk6",
		Fields: []Field{
			{Key: "text_on", LabelZh: "文本检测", LabelEn: "Text check", Type: "bool", Default: "1"},
			{Key: "image_on", LabelZh: "图片检测", LabelEn: "Image check", Type: "bool", Default: "1"},
			{Key: "products", LabelZh: "检测项(逗号分隔)", LabelEn: "Detection products (comma-separated)", Type: "text",
				Default: "TJ_POLITICS_MC,TJ_PORN_MC,TJ_ILLEGAL_MC,TJ_TERRORISM_MC,TJ_ABUSES_MC",
				HelpZh:  "TJ_POLITICS_MC 敏感 / TJ_PORN_MC 色情 / TJ_ILLEGAL_MC 违禁 / TJ_TERRORISM_MC 暴恐 / TJ_ABUSES_MC 谩骂",
				HelpEn:  "Politics / porn / illegal / terrorism / abuse detection codes"},
			{Key: "channel", LabelZh: "来源渠道", LabelEn: "Channel", Type: "text", Default: "tinyapp-eco-open",
				HelpZh: "支付宝分配的来源渠道。开通该服务时支付宝会给,拿不准先留默认值",
				HelpEn: "Source channel assigned by Alipay when the service is activated"},
			{Key: "tenants", LabelZh: "支付宝侧租户 ID", LabelEn: "Alipay tenant ID", Type: "text",
				HelpZh: "开通内容安全服务时支付宝分配,没有就留空",
				HelpEn: "Assigned by Alipay when the content-safety service is activated; leave empty if none"},
		}},
}

// Definitions 某领域的全部服务商,按声明顺序。
func Definitions(kind string) []Definition {
	var out []Definition
	for _, d := range definitions {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

func Find(kind, provider string) (Definition, bool) {
	for _, d := range definitions {
		if d.Kind == kind && d.Provider == provider {
			return d, true
		}
	}
	return Definition{}, false
}

// SingleActive 该领域是否「同一时刻只能用一家」。支付由客户端选渠道,可多家并存。
func SingleActive(kind string) bool { return kind == KindMap || kind == KindModeration }

// Missing 缺哪些必填字段(按声明顺序)。
func Missing(d Definition, fields map[string]string) []string {
	var miss []string
	for _, f := range d.Fields {
		if f.Required && strings.TrimSpace(fields[f.Key]) == "" {
			miss = append(miss, f.Key)
		}
	}
	return miss
}

// MissingWith 自己的缺项,外加被依赖卡片的缺项(后者带 "<kind>/<provider>." 前缀,
// 让后台能提示运营该去哪一页补)。depFields 传 nil 表示被依赖的卡片从没配过。
//
// 依赖的卡片定义不存在时按「没有依赖」处理:schema 写错只该让这张卡片失去保护,
// 不该把整个服务商页和 /app-config 一起打挂。
func MissingWith(d Definition, fields, depFields map[string]string) []string {
	miss := Missing(d, fields)
	if d.DependsOn == nil {
		return miss
	}
	dep, ok := Find(d.DependsOn.Kind, d.DependsOn.Provider)
	if !ok {
		return miss
	}
	prefix := d.DependsOn.Kind + "/" + d.DependsOn.Provider + "."
	for _, k := range Missing(dep, depFields) {
		miss = append(miss, prefix+k)
	}
	return miss
}
