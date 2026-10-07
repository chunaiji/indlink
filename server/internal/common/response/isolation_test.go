package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"driftbottle/internal/common/i18n"
)

// TestClientRoutesStayChinese 是这次 i18n 改造最重要的一条回归线:
// 语言中间件只挂 /admin/api,C 端路由必须**永远**下发中文。
//
// wx.request 在部分机型会带 Accept-Language: en。若哪天有人顺手把中间件
// 挂到全局或 C 端路由组上,中文用户就会突然收到英文报错 —— 而这种事
// 在后台自测时完全看不出来。
func TestClientRoutesStayChinese(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const zh = "参数错误"
	// 前提:这条消息确实有英文译文,否则本测试会因「压根没翻」而假通过。
	if !i18n.Has(i18n.En, zh) {
		t.Fatalf("消息表缺 %q 的英文译文,本测试失去意义", zh)
	}

	newEngine := func(withMiddleware bool) *gin.Engine {
		r := gin.New()
		g := r.Group("/x")
		if withMiddleware {
			g.Use(i18n.Middleware())
		}
		g.GET("/fail", func(c *gin.Context) { Fail(c, 1001, zh) })
		return r
	}

	call := func(r *gin.Engine) string {
		req := httptest.NewRequest(http.MethodGet, "/x/fail", nil)
		req.Header.Set("Accept-Language", "en")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var body struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		return body.Msg
	}

	// 没挂中间件(C 端):即便请求头是 en,也必须原样下发中文。
	if got := call(newEngine(false)); got != zh {
		t.Errorf("未挂中间件的路由下发了 %q, 期望原样中文 %q —— C 端被波及了", got, zh)
	}

	// 挂了中间件(后台):同样的请求头必须拿到英文。
	if got := call(newEngine(true)); got == zh {
		t.Errorf("挂了中间件仍下发中文 %q, 期望英文译文", got)
	}
}
