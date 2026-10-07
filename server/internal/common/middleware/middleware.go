package middleware

import (
	"net/http"
	"strings"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/jwtutil"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

const CtxUserID = "uid"
const CtxTenantID = "tid"
const CtxPlatform = "plat"

// OnAuthenticated 鉴权成功后回调(可选)，由 main 注入；用于刷新用户活跃时间。
var OnAuthenticated func(tenantID, userID int64)

// IsRevoked 判断该用户的令牌是否已被吊销(账号注销)。由 main 注入；nil 时跳过。
// 实现须廉价(查 Redis)——它跑在每一个鉴权请求上。
var IsRevoked func(userID int64) bool

// Auth 校验 Authorization: Bearer <token>,注入 uid/plat 到上下文。
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" || !strings.HasPrefix(h, "Bearer ") {
			response.Abort(c, http.StatusUnauthorized, errs.CodeUnauthorized, "未登录")
			return
		}
		claims, err := jwtutil.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			response.Abort(c, http.StatusUnauthorized, errs.CodeUnauthorized, "登录已失效")
			return
		}
		// 账号注销后旧 JWT 必须立即失效，否则删号前签发的令牌还能用到过期。
		if IsRevoked != nil && IsRevoked(claims.UserID) {
			response.Abort(c, http.StatusUnauthorized, errs.CodeUnauthorized, "账号已注销")
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxTenantID, claims.TenantID)
		c.Set(CtxPlatform, claims.Platform)
		if OnAuthenticated != nil {
			// 同步调用：内部已做 Redis 去抖判定(廉价单次往返)，仅命中时才异步写库(FF4)
			OnAuthenticated(claims.TenantID, claims.UserID)
		}
		c.Next()
	}
}

// UserID 从上下文取当前登录用户 ID。
func UserID(c *gin.Context) int64 {
	if v, ok := c.Get(CtxUserID); ok {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

// TenantID 从上下文取当前租户 ID。
func TenantID(c *gin.Context) int64 {
	if v, ok := c.Get(CtxTenantID); ok {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

// Platform 从上下文取登录平台。
func Platform(c *gin.Context) string {
	if v, ok := c.Get(CtxPlatform); ok {
		if p, ok := v.(string); ok {
			return p
		}
	}
	return ""
}

// CORS 开发期放开跨域(生产由 Nginx 控制)。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
