package sysconfig

import "testing"

func TestResolveFallback(t *testing.T) {
	// 直接构造按租户缓存:tenant 0 全局, tenant 100 覆盖
	mu.Lock()
	cached = map[int64]map[string]string{
		0:   {"k_global": "g", "k_both": "G"},
		100: {"k_both": "T", "k_empty": ""},
	}
	mu.Unlock()
	defaults["k_default"] = "D"

	cases := []struct {
		tid       int64
		key, want string
	}{
		{100, "k_both", "T"},     // 租户值优先
		{100, "k_global", "g"},   // 回退全局
		{100, "k_default", "D"},  // 回退代码默认
		{100, "k_empty", "g_or"}, // 租户空串 → 回退(此处全局无该键→默认空)
		{200, "k_global", "g"},   // 无该租户 → 全局
	}
	// k_empty 期望:租户空串视为未设→回退全局(无)→默认(无)→""
	cases[3].want = ""
	for _, c := range cases {
		if got := GetString(c.tid, c.key); got != c.want {
			t.Errorf("GetString(%d,%q)=%q want %q", c.tid, c.key, got, c.want)
		}
	}
}
