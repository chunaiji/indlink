package provider

import (
	"strings"
	"testing"
)

// 每个 (kind, provider) 只能有一份定义;Lookup 字段必须真的存在;必填字段要有双语标签。
func TestDefinitionsAreConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range definitions {
		k := d.Kind + "/" + d.Provider
		if seen[k] {
			t.Errorf("重复定义 %s", k)
		}
		seen[k] = true
		if d.LabelZh == "" || d.LabelEn == "" {
			t.Errorf("%s 缺双语名", k)
		}
		keys := map[string]bool{}
		for _, f := range d.Fields {
			if f.Key == "" || f.LabelZh == "" || f.LabelEn == "" {
				t.Errorf("%s 字段 %q 缺标签", k, f.Key)
			}
			keys[f.Key] = true
		}
		for _, lk := range []string{d.LookupA, d.LookupB} {
			if lk != "" && !keys[lk] {
				t.Errorf("%s 的 Lookup 字段 %q 不存在", k, lk)
			}
		}
	}
	for _, kind := range []string{KindPay, KindMap, KindModeration} {
		if len(Definitions(kind)) == 0 {
			t.Errorf("领域 %s 没有任何服务商", kind)
		}
	}
	if !SingleActive(KindMap) || !SingleActive(KindModeration) || SingleActive(KindPay) {
		t.Fatal("地图 / 内容安全单选,支付多选")
	}
}

func TestMissingListsRequiredFieldsOnly(t *testing.T) {
	d, _ := Find(KindPay, "alipay")
	miss := Missing(d, map[string]string{"app_id": "2021x"})
	if len(miss) != 2 || miss[0] != "private_key" || miss[1] != "alipay_public_key" {
		t.Fatalf("got %v", miss)
	}
	if got := Missing(d, map[string]string{"app_id": "a", "private_key": "p", "alipay_public_key": "k"}); len(got) != 0 {
		t.Fatalf("齐全时不该缺: %v", got)
	}
}

// 微信支付的平台公钥与公钥 ID 只在**回调验签**时才需要，下单不用。
// 商户后台拿这两样要等审核，把它们设成必填会让租户在拿到之前根本存不了配置、
// 也测不了支付。缺了只会让回调验签报错(driver_wx.go 有显式兜底)，不会 panic。
func TestWechatPlatformKeyFieldsAreOptional(t *testing.T) {
	d, _ := Find(KindPay, "wechat")
	full := map[string]string{
		"app_id": "wxapp", "mch_id": "16000", "apiv3_key": "k",
		"serial_no": "SN", "private_key": "PRIV",
	}
	if miss := Missing(d, full); len(miss) != 0 {
		t.Fatalf("没填平台公钥/公钥 ID 不该算缺项: %v", miss)
	}
	for _, key := range []string{"platform_key", "platform_serial"} {
		var found bool
		for _, f := range d.Fields {
			if f.Key == key {
				found = true
				if f.Required {
					t.Errorf("%s 不该是必填", key)
				}
				if f.HelpZh == "" {
					t.Errorf("%s 设成选填后必须写清楚不填的后果(回调验不了签)", key)
				}
			}
		}
		if !found {
			t.Errorf("字段 %s 不见了", key)
		}
	}
}

// 支付宝 PID 只给「支付宝授权登录」用，跟收款无关。不少商户根本没有这一项，
// 它必须保持选填，并且说明里要讲清楚不填只影响登录。
func TestAlipayPIDStaysOptionalAndSaysWhy(t *testing.T) {
	d, _ := Find(KindPay, "alipay")
	for _, f := range d.Fields {
		if f.Key != "pid" {
			continue
		}
		if f.Required {
			t.Error("pid 不该是必填")
		}
		if f.HelpZh == "" || f.HelpEn == "" {
			t.Error("pid 要有中英文说明:不填只影响支付宝登录,不影响支付")
		}
		return
	}
	t.Fatal("支付宝少了 pid 字段")
}

// 追加到 server/internal/provider/schema_test.go

// 依赖判定:被依赖卡片缺必填时,本卡片也算没配齐。
// 没有这条,运营在支付页删了支付宝私钥,登录页还会露出支付宝按钮,
// 用户点进去拿到的是「签名错误」——而后台两张卡片都显示绿色。
func TestMissingWithReportsDependencyGaps(t *testing.T) {
	d := Definition{
		Kind: "sso", Provider: "alipay",
		DependsOn: &Ref{Kind: KindPay, Provider: "alipay"},
		Fields:    []Field{{Key: "pid", Required: true}},
	}
	// 自己齐了,被依赖的一个字段都没有
	miss := MissingWith(d, map[string]string{"pid": "2088x"}, nil)
	if len(miss) == 0 {
		t.Fatal("被依赖卡片没配时必须报缺项")
	}
	for _, m := range miss {
		if !strings.HasPrefix(m, "pay/alipay.") {
			t.Errorf("被依赖卡片的缺项要带来源前缀,便于后台提示去哪配: %q", m)
		}
	}
	// 两边都齐
	dep := map[string]string{"app_id": "2021", "private_key": "P", "alipay_public_key": "K"}
	if got := MissingWith(d, map[string]string{"pid": "2088x"}, dep); len(got) != 0 {
		t.Fatalf("两边都齐时不该缺: %v", got)
	}
	// 自己缺,报自己的,不带前缀
	got := MissingWith(d, nil, dep)
	if len(got) != 1 || got[0] != "pid" {
		t.Fatalf("自己的缺项不带前缀: %v", got)
	}
}

// schema 写错把 DependsOn 指到不存在的卡片上,不能 panic——
// 那会让后台服务商页和 /app-config 一起挂,而不只是这一张卡片不可用。
func TestMissingWithSurvivesUnknownDependency(t *testing.T) {
	d := Definition{
		Kind: "sso", Provider: "x",
		DependsOn: &Ref{Kind: "nope", Provider: "nope"},
		Fields:    []Field{{Key: "k", Required: true}},
	}
	got := MissingWith(d, map[string]string{"k": "v"}, nil)
	if len(got) != 0 {
		t.Fatalf("依赖的卡片定义不存在时按「无依赖」处理,got %v", got)
	}
}

// 老的 Missing 不变:只看自己的字段。现有调用方(后台卡片、Usable)都还在用它。
func TestMissingStillIgnoresDependencies(t *testing.T) {
	d := Definition{
		Kind: "sso", Provider: "alipay",
		DependsOn: &Ref{Kind: KindPay, Provider: "alipay"},
		Fields:    []Field{{Key: "pid", Required: true}},
	}
	if got := Missing(d, map[string]string{"pid": "2088x"}); len(got) != 0 {
		t.Fatalf("Missing 只管自己的字段: %v", got)
	}
}

// 追加到 server/internal/provider/schema_test.go

// 四家 SSO 卡片的形状。这条测的是「配置的人打开页面能不能照着填完」,
// 所以盯的是必填集合和依赖关系,不是文案。
func TestSSODefinitions(t *testing.T) {
	want := map[string][]string{
		"wechat": {"app_id", "app_secret"},
		"alipay": {"pid"},
		"google": {"client_id"},
		"apple":  {"bundle_id"},
	}
	defs := Definitions(KindSSO)
	if len(defs) != 4 {
		t.Fatalf("应有 4 家, got %d", len(defs))
	}
	for _, d := range defs {
		if d.Platform != PlatformApp {
			t.Errorf("%s 只对 App 租户显示,小程序不走这些", d.Provider)
		}
		req := []string{}
		for _, f := range d.Fields {
			if f.Required {
				req = append(req, f.Key)
			}
		}
		if got := want[d.Provider]; got == nil {
			t.Errorf("多出一家 %s", d.Provider)
		} else if strings.Join(req, ",") != strings.Join(got, ",") {
			t.Errorf("%s 必填项 = %v, want %v", d.Provider, req, got)
		}
	}
	// SSO 四家并存,不是单选
	if SingleActive(KindSSO) {
		t.Error("SSO 不能是单选:一个 App 同时提供多个登录渠道")
	}
}

// 支付宝登录的密钥来自支付卡片,自己只存 PID。
func TestSSOAlipayDependsOnPay(t *testing.T) {
	d, ok := Find(KindSSO, "alipay")
	if !ok {
		t.Fatal("没有 sso/alipay")
	}
	if d.DependsOn == nil || d.DependsOn.Kind != KindPay || d.DependsOn.Provider != "alipay" {
		t.Fatalf("sso/alipay 必须依赖 pay/alipay, got %+v", d.DependsOn)
	}
	// 自己不能再存一份私钥,否则就是两份会漂移的配置
	for _, f := range d.Fields {
		if f.Key == "private_key" || f.Key == "alipay_public_key" {
			t.Errorf("sso/alipay 不该自带 %s,它来自支付卡片", f.Key)
		}
	}
	// 登录卡片上要讲清密钥在哪配,否则运营会以为登录坏了
	if !strings.Contains(d.LabelZh+d.Fields[0].HelpZh, "支付") {
		t.Error("要在卡片上说明密钥来自支付页")
	}
}

// 微信不走依赖:开放平台移动应用登录要 appid+secret,而支付要 appid+商户号+三把密钥,
// secret 支付根本不用。两者 appid 常相同但不保证同一个应用。
func TestSSOWechatIsStandalone(t *testing.T) {
	d, _ := Find(KindSSO, "wechat")
	if d.DependsOn != nil {
		t.Error("微信登录不依赖微信支付")
	}
	var secretIsSecret bool
	for _, f := range d.Fields {
		if f.Key == "app_secret" {
			secretIsSecret = f.Secret
		}
	}
	if !secretIsSecret {
		t.Error("app_secret 必须标 Secret,否则后台会把它明文回显")
	}
}

// 只支持一跳:被依赖的卡片自己不能再声明依赖。
func TestDependenciesAreAtMostOneHop(t *testing.T) {
	for _, d := range definitions {
		if d.DependsOn == nil {
			continue
		}
		dep, ok := Find(d.DependsOn.Kind, d.DependsOn.Provider)
		if !ok {
			t.Errorf("%s/%s 依赖了不存在的 %s/%s", d.Kind, d.Provider, d.DependsOn.Kind, d.DependsOn.Provider)
			continue
		}
		if dep.DependsOn != nil {
			t.Errorf("%s/%s 的依赖又有依赖,只支持一跳", d.Kind, d.Provider)
		}
	}
}
