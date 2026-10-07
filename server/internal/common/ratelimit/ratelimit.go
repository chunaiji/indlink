package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"
	"driftbottle/pkg/cache"

	"github.com/gin-gonic/gin"
)

// PerUser 基于 Redis 的滑动窗口限流(每用户每接口)。
// limit 次 / window 秒。未登录时按 IP 限流。
func PerUser(name string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := buildKey(c, name)
		ctx := context.Background()
		cnt, err := cache.RDB.Incr(ctx, key).Result()
		if err == nil {
			if cnt == 1 {
				cache.RDB.Expire(ctx, key, window)
			}
			if cnt > int64(limit) {
				response.Abort(c, http.StatusTooManyRequests, errs.CodeRateLimited, "操作太频繁,请稍后再试")
				return
			}
		}
		c.Next()
	}
}

func buildKey(c *gin.Context, name string) string {
	if uid := middleware.UserID(c); uid != 0 {
		return fmt.Sprintf("rl:%s:u:%d", name, uid)
	}
	return fmt.Sprintf("rl:%s:ip:%s", name, c.ClientIP())
}
