package user

import "testing"

func TestClientConfig_HasKeys(t *testing.T) {
	c := clientConfig(0) // 用全局默认
	for _, k := range []string{
		"chat_quicks", "reply_quicks", "ui_text_chat_banner",
		"low_balance_threshold", "low_balance_msg", "price_chat", "ws_url",
		"ui_text_quota_title", "ui_text_quota_msg",
	} {
		if _, ok := c[k]; !ok {
			t.Errorf("clientConfig 缺少键 %s", k)
		}
	}
}
