package admin

import "testing"

func TestValidCredPlatform(t *testing.T) {
	for _, p := range []string{"wx", "alipay", "app"} {
		if !validCredPlatform(p) {
			t.Errorf("%s 应合法", p)
		}
	}
	for _, p := range []string{"", "weixin", "ios", "gplay"} {
		if validCredPlatform(p) {
			t.Errorf("%s 不该合法", p)
		}
	}
}

// 迁移后 App 登录凭证只在「服务商 → 登录」页维护。凭证页继续收 wx_app / alipay_app
// 就会出现两个页面都能配微信,正是这次要消掉的分裂。
func TestCredPlatformsDropsAppSSORows(t *testing.T) {
	for _, p := range []string{"wx_app", "alipay_app"} {
		if validCredPlatform(p) {
			t.Errorf("%s 应已退出凭证页", p)
		}
	}
	for _, p := range []string{"wx", "alipay", "app"} {
		if !validCredPlatform(p) {
			t.Errorf("%s 仍应可用", p)
		}
	}
}
