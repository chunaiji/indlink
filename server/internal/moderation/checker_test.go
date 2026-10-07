package moderation

import (
	"errors"
	"testing"
)

// 支付宝内容检测的判定字段是 suggestion(参考实现 CH.Super.Project 的 AliPayApplication.ContentDetect):
// pass 放行;review 交人工也放行——把不确定的判定直接拦下来,代价是正常用户发不出东西;
// 其余一律拦。空 suggestion 视为坏响应报错,由调用方按「故障放行」处理。
func TestAlipayVerdict(t *testing.T) {
	cases := []struct {
		node    string
		blocked bool
		wantErr bool
	}{
		{`{"code":"10000","suggestion":"pass","detect_check_labels":[]}`, false, false},
		{`{"code":"10000","suggestion":"review"}`, false, false},
		{`{"code":"10000","suggestion":"block","detect_check_labels":[{"label":"TJ_PORN_MC","rate":"0.98"}]}`, true, false},
		{`{"code":"10000","suggestion":"reject"}`, true, false},
		{`{"code":"10000"}`, false, true},
		{`not json`, false, true},
	}
	for _, c := range cases {
		blocked, err := alipayVerdict([]byte(c.node))
		if blocked != c.blocked || (err != nil) != c.wantErr {
			t.Errorf("alipayVerdict(%s) = %v,%v want %v,err=%v", c.node, blocked, err, c.blocked, c.wantErr)
		}
	}
}

// 送检请求体必须照参考实现的形状:data_list 是数组、content_type 区分文本/图片、
// products 是逗号分隔的检测项、request_id 每次不同(支付宝按它去重)。
func TestAlipayDetectBiz(t *testing.T) {
	row := map[string]string{"products": "TJ_PORN_MC", "channel": "tinyapp-eco-open", "tenants": "T1"}
	biz := alipayDetectBiz(row, []string{"一句话"}, "TEXT", "2088open", "req-1")
	if biz["content_type"] != "TEXT" || biz["open_id"] != "2088open" || biz["channel"] != "tinyapp-eco-open" {
		t.Fatalf("%+v", biz)
	}
	if biz["products"] != "TJ_PORN_MC" || biz["tenants"] != "T1" || biz["request_id"] != "req-1" {
		t.Fatalf("%+v", biz)
	}
	if biz["data_list"] != `["一句话"]` {
		t.Fatalf("data_list 必须是 JSON 数组, got %s", biz["data_list"])
	}
	// 没配 products / channel 时用内置默认,而不是发个空值过去
	biz = alipayDetectBiz(map[string]string{}, []string{"x"}, "PICTURE", "o", "r")
	if biz["products"] == "" || biz["channel"] == "" {
		t.Fatalf("默认值缺失: %+v", biz)
	}
}

// 没有生效的服务商 → 不在线检测,只走本地词库。
func TestCheckerForNoneWhenNoActiveProvider(t *testing.T) {
	s := New(nil)
	if _, _, ok := s.checkerFor(1); ok {
		t.Fatal("未注入 providers 时不该有 checker")
	}
}

// 探活必须真的去调 API。旧实现复用生产 checker,而生产 checker 是**故意容错**的
// (本地词库已兜底,API 故障一律放行),于是 appid 填错也报「检测通过」,运营照着绿灯上线,
// 审核那天才发现内容安全根本没开。
func TestProbeTextFailsWhenNotConfigured(t *testing.T) {
	s := New(nil)
	if err := s.ProbeText(1); err == nil {
		t.Fatal("没有生效的服务商时探活必须报错")
	}
}

// 微信探活不能走 msgSecCheck:那个接口结构上需要真实 openid,拿 userID=0 调它会在
// 发 HTTP 之前就 return nil,等于永远绿灯。改为探 access_token。
func TestWechatProbeDoesNotUseMsgSecCheck(t *testing.T) {
	s := New(nil)
	w := &wechatChecker{s: s}
	// tokenFn 未注入 = 拿不到 access_token,探活必须报错而不是静默通过
	if err := w.Probe(1); err == nil {
		t.Fatal("未注入 token 来源时微信探活应报错")
	}
	called := false
	s.tokenFn = func(int64) (string, error) { called = true; return "", errors.New("appid/secret 未配置") }
	if err := w.Probe(1); err == nil {
		t.Fatal("取 token 失败时应把错误透出来")
	}
	if !called {
		t.Fatal("探活必须真的去取 access_token")
	}
}
