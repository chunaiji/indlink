package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"driftbottle/internal/bottle"
	"driftbottle/internal/chat"
	"driftbottle/internal/item"
	"driftbottle/internal/match"
	"driftbottle/internal/moderation"
	"driftbottle/internal/pay"
	"driftbottle/internal/relation"
	"driftbottle/internal/upload"
	"driftbottle/internal/user"
	"driftbottle/internal/wallet"

	"github.com/gin-gonic/gin"
)

// TestRouteRegistration 确认所有模块路由能装配进同一引擎而不冲突/panic。
// Register 仅构建路由树,不触碰 service,故可传 nil。
func TestRouteRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("路由注册 panic(存在路径冲突): %v", r)
		}
	}()

	r := gin.New()
	api := r.Group("/api")
	noop := func(c *gin.Context) { c.Next() }

	user.NewHandler(nil).Register(api, noop)
	wallet.NewHandler(nil).Register(api, noop)
	pay.NewHandler(nil).Register(api, noop)
	bottle.NewHandler(nil).Register(api, noop, noop, noop)
	match.NewHandler(nil).Register(api, noop)
	relation.NewHandler(nil).Register(api, noop)
	item.NewHandler(nil).Register(api, noop)
	moderation.NewHandler(nil).Register(api, noop)
	upload.NewHandler("./uploads", "").Register(api, noop)
	ch := chat.NewHandler(nil)
	ch.Register(api, noop)
	ch.RegisterWS(r, "/ws")
}

// TestOAuthBindRoutesRequireAuth 确认绑定/解绑三条路由挂在 auth 之后，
// 而登录两条**不**挂。
//
// 为什么值得一个测试：这类错误完全静默。把 bind 写在 auth 之前，
// 接口照常返回 200，只是 middleware.UserID(c) 取到 0，
// 于是「绑定到当前用户」变成「绑定到用户 0」——没人会立刻发现。
//
// 手法：用一个会 401 中断的哨兵中间件替代 auth。挂了 auth 的路由拿到 401；
// 没挂的会落到 handler，因 body 为空而在 ShouldBindJSON 处返回 200 + 1001
// （此时 service 为 nil 也不会被碰到）。两者据此区分。
func TestOAuthBindRoutesRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	sentinel := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	user.NewHandler(nil).Register(api, sentinel)

	protected := []string{"/api/auth/google/bind", "/api/auth/apple/bind", "/api/auth/unbind"}
	for _, path := range protected {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s 必须要求登录态,期望 401,实际 %d", path, w.Code)
		}
	}

	// 登录接口反过来:未登录也必须能访问,否则没人能登进来。
	public := []string{"/api/auth/google", "/api/auth/apple"}
	for _, path := range public {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code == http.StatusUnauthorized {
			t.Errorf("%s 是登录入口,不能要求登录态", path)
		}
	}
}
