// Package quota 每日扔/捞次数限制(#5):免费次数走 Redis 按天计数,超额消耗购买的「次数包」余额。
package quota

import (
	"context"
	"fmt"
	"time"

	"driftbottle/pkg/cache"
)

const (
	ActionThrow = "throw"
	ActionScoop = "scoop"
)

func today() string { return time.Now().Format("20060102") }

// 免费次数:按天 key,48h 过期(跨天自动失效)
func freeKey(tenantID, userID int64, action string) string {
	return fmt.Sprintf("q:%d:%d:%s:%s", tenantID, userID, action, today())
}

// 购买的次数包余额:持久,无过期
func packKey(tenantID, userID int64, action string) string {
	return fmt.Sprintf("qp:%d:%d:%s", tenantID, userID, action)
}

// Status 返回今日已用免费次数 + 剩余次数包。
func Status(tenantID, userID int64, action string) (freeUsed int, packLeft int) {
	ctx := context.Background()
	freeUsed, _ = cache.RDB.Get(ctx, freeKey(tenantID, userID, action)).Int()
	packLeft, _ = cache.RDB.Get(ctx, packKey(tenantID, userID, action)).Int()
	if freeUsed < 0 {
		freeUsed = 0
	}
	if packLeft < 0 {
		packLeft = 0
	}
	return
}

// Remaining 返回今日剩余可用次数(免费剩余 + 次数包剩余)。
func Remaining(tenantID, userID int64, action string, freeLimit int) (freeLeft, packLeft int) {
	used, pack := Status(tenantID, userID, action)
	fl := freeLimit - used
	if fl < 0 {
		fl = 0
	}
	return fl, pack
}

// TryConsume 消耗一次:优先免费额度,用尽则扣次数包;都没有返回 false。
func TryConsume(tenantID, userID int64, action string, freeLimit int) bool {
	ctx := context.Background()
	fk := freeKey(tenantID, userID, action)
	used, _ := cache.RDB.Get(ctx, fk).Int()
	if used < freeLimit {
		// 占用一个免费次数;首次设置 48h 过期
		n, err := cache.RDB.Incr(ctx, fk).Result()
		if err != nil {
			return false
		}
		if n == 1 {
			cache.RDB.Expire(ctx, fk, 48*time.Hour)
		}
		return true
	}
	// 免费用尽 → 扣次数包(DECR 后 >=0 才算成功,否则回滚)
	pk := packKey(tenantID, userID, action)
	left, err := cache.RDB.Decr(ctx, pk).Result()
	if err != nil {
		return false
	}
	if left < 0 {
		cache.RDB.Incr(ctx, pk) // 回滚,无次数包
		return false
	}
	return true
}

// Refund 退回一次消耗(空捞等场景):优先退免费计数,免费为 0 则退回次数包。
func Refund(tenantID, userID int64, action string, freeLimit int) {
	ctx := context.Background()
	fk := freeKey(tenantID, userID, action)
	used, _ := cache.RDB.Get(ctx, fk).Int()
	if used > 0 {
		cache.RDB.Decr(ctx, fk)
		return
	}
	cache.RDB.Incr(ctx, packKey(tenantID, userID, action))
}

// AddPack 购买次数包:增加余额。
func AddPack(tenantID, userID int64, action string, n int) {
	if n <= 0 {
		return
	}
	cache.RDB.IncrBy(context.Background(), packKey(tenantID, userID, action), int64(n))
}
