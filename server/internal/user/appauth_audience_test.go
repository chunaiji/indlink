package user

import (
	"reflect"
	"testing"
)

// Google 的 ID token 里 aud 是「哪个 client 发起的登录」,而同一个项目下
// Android 与 iOS 拿到的值不同:
//   - Android 配了 serverClientId 之后, aud = Web client ID
//   - iOS 走 Info.plist 那条路, aud = iOS client ID
//
// 所以校验只认单个值时,两个平台必然有一个登录不了。
// Google 自己的建议就是接受你名下任意一个 client ID。
func TestSplitClientIDs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"单个(旧配置照常工作)", "web.apps.googleusercontent.com",
			[]string{"web.apps.googleusercontent.com"}},
		{"两个平台各一个", "web.apps.googleusercontent.com,ios.apps.googleusercontent.com",
			[]string{"web.apps.googleusercontent.com", "ios.apps.googleusercontent.com"}},
		{"后台粘贴常见的空格与换行", " web.x \n, ios.x ,",
			[]string{"web.x", "ios.x"}},
		{"空配置返回空,让调用方报「未配置」", "", nil},
		{"只有分隔符也算没配", " , , ", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := splitClientIDs(c.raw); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("splitClientIDs(%q) = %#v, want %#v", c.raw, got, c.want)
			}
		})
	}
}

// 第一个值是 Web client ID —— /app-config 把它下发给 Android 当 serverClientId。
// 顺序有语义,不能随手排序或去重打乱。
func TestPrimaryClientIDIsTheFirstOne(t *testing.T) {
	if got := primaryClientID(" web.x , ios.x "); got != "web.x" {
		t.Fatalf("primaryClientID = %q, want web.x", got)
	}
	if got := primaryClientID(""); got != "" {
		t.Fatalf("没配时应返回空串, got %q", got)
	}
}
