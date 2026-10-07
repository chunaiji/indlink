package user

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// 国内版 App 的微信 / 支付宝登录与绑定路由必须注册上;少一条,客户端那边就是 404。
func TestOAuthRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noop := func(c *gin.Context) { c.Next() }
	NewHandler(&Service{}).Register(r.Group("/api"), noop)
	want := map[string]bool{
		"POST /api/auth/wechat": false, "POST /api/auth/alipay": false, "GET /api/auth/alipay/auth-info": false,
		"POST /api/auth/wechat/bind": false, "POST /api/auth/alipay/bind": false,
	}
	for _, ri := range r.Routes() {
		if _, ok := want[ri.Method+" "+ri.Path]; ok {
			want[ri.Method+" "+ri.Path] = true
		}
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("路由未注册: %s", k)
		}
	}
}
