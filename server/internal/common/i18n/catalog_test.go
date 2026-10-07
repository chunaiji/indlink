package i18n

import "testing"

func TestTFallsBackToChinese(t *testing.T) {
	// 消息表里没有的原文必须原样返回 —— 永远不允许空白上线。
	const unknown = "这条消息不在表里"
	if got := T(En, unknown); got != unknown {
		t.Errorf("T(En, 未收录) = %q, 期望原样返回 %q", got, unknown)
	}
}

func TestTReturnsChineseAsIs(t *testing.T) {
	const zh = "参数错误"
	if got := T(ZhCN, zh); got != zh {
		t.Errorf("T(ZhCN, %q) = %q, 期望原样返回", zh, got)
	}
	// 空语言等同中文 —— FromContext 在无中间件时返回 ZhCN,但防御性再兜一层。
	if got := T("", zh); got != zh {
		t.Errorf(`T("", %q) = %q, 期望原样返回`, zh, got)
	}
}

func TestTTranslatesKnownMessage(t *testing.T) {
	const zh = "参数错误"
	got := T(En, zh)
	if got == zh {
		t.Fatalf("T(En, %q) 未翻译,消息表缺这一条", zh)
	}
	if got == "" {
		t.Fatalf("T(En, %q) 返回空串", zh)
	}
}

func TestHas(t *testing.T) {
	if !Has(En, "参数错误") {
		t.Error(`Has(En, "参数错误") = false, 期望 true`)
	}
	if Has(En, "这条消息不在表里") {
		t.Error("Has 对未收录消息应返回 false")
	}
	// 中文本身不需要收录,Has(ZhCN, ...) 恒为 true 会让覆盖测试失去意义。
	if Has(ZhCN, "参数错误") {
		t.Error("Has(ZhCN, ...) 应为 false —— 中文是 key 本身,不是译文")
	}
}
