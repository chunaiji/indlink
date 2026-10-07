package sysconfig

import "testing"

// 多页面覆盖:默认关闭、图片空;租户配置后生效。
func TestPagesCoverDefaults(t *testing.T) {
	mu.Lock()
	cached = map[int64]map[string]string{}
	mu.Unlock()

	if GetBool(0, KeyPagesCoverOn) {
		t.Errorf("pages_cover_on 默认应为关闭")
	}
	if got := GetString(0, KeyPagesCoverImage); got != "" {
		t.Errorf("pages_cover_image 默认应为空, got %q", got)
	}

	mu.Lock()
	cached = map[int64]map[string]string{
		100: {KeyPagesCoverOn: "1", KeyPagesCoverImage: "https://cdn/p.png"},
	}
	mu.Unlock()

	if !GetBool(100, KeyPagesCoverOn) {
		t.Errorf("租户 pages_cover_on 应为开启")
	}
	if got := GetString(100, KeyPagesCoverImage); got != "https://cdn/p.png" {
		t.Errorf("租户 pages_cover_image = %q", got)
	}
}
