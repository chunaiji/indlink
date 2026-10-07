package admin

import (
	"strings"
	"testing"

	"driftbottle/internal/common/i18n"
	"driftbottle/internal/model"
)

// —— 机密项脱敏 ——

// 机密值绝不下发。现状是 LLM Key / SMTP 授权码 / IAP 私钥的明文
// 随配置接口一起进浏览器,任何能看到那次响应的人都拿得到。
func TestSecretFieldsNeverCarryValue(t *testing.T) {
	m := configFieldMeta{
		Key: "x", LabelZh: "密钥", LabelEn: "Key",
		Group: GroupAI, Type: "text", Secret: true,
	}
	got := resolveField(m, "super-secret-value", i18n.ZhCN)
	if got.Value != "" {
		t.Errorf("机密项下发了值 %q,必须为空", got.Value)
	}
	if !got.Secret {
		t.Error("机密项应当标记 secret=true,前端要据此渲染成密码框")
	}
	if !got.IsSet {
		t.Error("有值时 is_set 应为 true —— 否则运维看不出配没配")
	}
}

func TestSecretFieldUnsetReportsIsSetFalse(t *testing.T) {
	m := configFieldMeta{Key: "x", LabelZh: "密钥", LabelEn: "Key", Group: GroupAI, Type: "text", Secret: true}
	if got := resolveField(m, "", i18n.ZhCN); got.IsSet {
		t.Error("空值时 is_set 应为 false")
	}
	// 只有空白也算没配
	if got := resolveField(m, "   ", i18n.ZhCN); got.IsSet {
		t.Error("纯空白应当算作未设置")
	}
}

// 非机密项照常下发,别把普通配置也遮了。
func TestNonSecretFieldKeepsValue(t *testing.T) {
	m := configFieldMeta{Key: "x", LabelZh: "数量", LabelEn: "Count", Group: GroupAI, Type: "int"}
	got := resolveField(m, "42", i18n.ZhCN)
	if got.Value != "42" {
		t.Errorf("非机密项的值被吞了: %q", got.Value)
	}
	if got.Secret || got.IsSet {
		t.Error("非机密项不该带 secret/is_set 语义")
	}
}

// 机密项写空 = 保持不变。
//
// 这是整个脱敏方案的安全绳:前端显示的是掩码不是真值,
// 用户 Tab 划过输入框触发 blur 就会提交一次——不挡住的话,
// 一次无意的聚焦就能把真密钥清成空串。
func TestShouldSkipSecretWrite(t *testing.T) {
	if !shouldSkipWrite(true, "") {
		t.Error("机密项写空必须跳过,否则误触会清掉真密钥")
	}
	if !shouldSkipWrite(true, "   ") {
		t.Error("机密项写纯空白同样要跳过")
	}
	if shouldSkipWrite(true, "new-key") {
		t.Error("机密项有新值时必须写入")
	}
	// 非机密项写空是合法的「清空这项配置」
	if shouldSkipWrite(false, "") {
		t.Error("非机密项写空是合法操作,不该跳过")
	}
}

// 每个标 Secret 的 key 都该真的是机密,别把开关也标上。
func TestSecretFieldsAreTextLike(t *testing.T) {
	for _, f := range configMeta {
		if !f.Secret {
			continue
		}
		if f.Type == "bool" || f.Type == "int" {
			t.Errorf("%s 是 %s 类型,不该标成机密", f.Key, f.Type)
		}
	}
}

// 名字里带 secret/key/token 的配置项,大概率是机密 —— 漏标一个就是明文外泄。
// 白名单里的是「名字像但其实公开」的,必须逐个写明理由。
func TestNoUnmarkedSecrets(t *testing.T) {
	notSecret := map[string]string{
		"push_wx_appid":     "AppID 公开",
		"app_iap_issuer_id": "Issuer ID 是标识不是密钥",
		"app_iap_key_id":    "Key ID 是标识,真正的密钥是 .p8 内容",
		"app_sms_template":  "模板 ID 不是密钥",
		"llm_max_tokens":    "这里的 token 是 LLM 的计费单位,不是凭证",
	}
	for _, f := range configMeta {
		if f.Secret {
			continue
		}
		k := strings.ToLower(f.Key)
		looksSecret := strings.Contains(k, "secret") || strings.Contains(k, "_key") ||
			strings.HasSuffix(k, "key") || strings.Contains(k, "token") || strings.Contains(k, "_enc")
		if looksSecret && notSecret[f.Key] == "" {
			t.Errorf("%s 名字像机密却没标 Secret;确实不是的话请加进 notSecret 白名单并写明理由", f.Key)
		}
	}
}

// —— 按租户类型过滤 ——

func TestEffectivePlatformFallsBackToGroup(t *testing.T) {
	// 字段没声明 → 用分组的
	got := effectivePlatform(configFieldMeta{Group: GroupAds}, groupMeta[GroupAds])
	if got != PlatformMP {
		t.Errorf("广告组应当是小程序专属,得到 %q", got)
	}
	// 字段声明了 → 覆盖分组
	got = effectivePlatform(
		configFieldMeta{Group: GroupCommon, Platform: PlatformMP},
		groupMeta[GroupCommon],
	)
	if got != PlatformMP {
		t.Errorf("字段级声明该覆盖分组,得到 %q", got)
	}
}

func TestVisibleForTenantType(t *testing.T) {
	cases := []struct {
		platform, tenantType string
		want                 bool
	}{
		{PlatformBoth, model.TenantTypeMiniProgram, true},
		{PlatformBoth, model.TenantTypeApp, true},
		{PlatformMP, model.TenantTypeMiniProgram, true},
		{PlatformMP, model.TenantTypeApp, false},
		{PlatformApp, model.TenantTypeApp, true},
		{PlatformApp, model.TenantTypeMiniProgram, false},
		// 空租户类型 = 选了「全部租户」,在配全局默认,必须看得见全部
		{PlatformMP, "", true},
		{PlatformApp, "", true},
	}
	for _, c := range cases {
		if got := visibleFor(c.platform, c.tenantType); got != c.want {
			t.Errorf("visibleFor(%q, %q) = %v, 期望 %v", c.platform, c.tenantType, got, c.want)
		}
	}
}

// 每个分组都要声明端属性,漏了会默默按「两端都显示」处理,
// 而那正是这个功能要消灭的现状。
func TestEveryGroupDeclaresPlatform(t *testing.T) {
	ok := map[string]bool{PlatformBoth: true, PlatformMP: true, PlatformApp: true}
	for code, g := range groupMeta {
		if !ok[g.Platform] {
			t.Errorf("分组 %s 的端属性 %q 非法,必须是 both/miniprogram/app 之一", code, g.Platform)
		}
	}
	for _, f := range configMeta {
		if f.Platform != "" && !ok[f.Platform] {
			t.Errorf("配置项 %s 的端属性 %q 非法", f.Key, f.Platform)
		}
	}
}

// App 租户不该看到小程序专属的东西,反之亦然 —— 抽查几个最明显的。
func TestPlatformAssignmentSanity(t *testing.T) {
	want := map[string]string{
		GroupAds:      PlatformMP,  // 微信流量主
		GroupSupport:  PlatformMP,  // 关联小程序跳转
		GroupNav:      PlatformMP,  // 小程序 tab 显隐
		GroupCamera:   PlatformMP,  // 水印相机是小程序功能
		GroupAppAuth:  PlatformApp, // 手机/邮箱/Google 登录是 App 的
		GroupAppLog:   PlatformApp,
		GroupAppLegal: PlatformApp,
		GroupAI:       PlatformBoth, // 机器人两端都跑
	}
	for code, expect := range want {
		g, ok := groupMeta[code]
		if !ok {
			t.Errorf("分组 %s 不存在", code)
			continue
		}
		if g.Platform != expect {
			t.Errorf("分组 %s 的端属性 = %q, 期望 %q", code, g.Platform, expect)
		}
	}
}
