// Package ad 流量主激励视频发币(信任客户端 isEnded + 每日上限 + 幂等 bizNo)。
package ad

import (
	"context"
	"fmt"
	"log"
	"time"

	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/cache"

	"gorm.io/gorm"
)

type Service struct {
	db  *gorm.DB
	wlt *wallet.Service
}

func New(db *gorm.DB, w *wallet.Service) *Service { return &Service{db: db, wlt: w} }

func today() string { return time.Now().Format("20060102") }

func dayKey(tenantID, userID int64, d string) string {
	return fmt.Sprintf("adreward:%d:%d:%s", tenantID, userID, d)
}

// Reward 发放激励视频奖励。返回是否实发、单次币数、当前余额、今日已领次数、每日上限。
// 用 Redis 计数控每日上限(原子 Incr),发币失败回滚计数;bizNo 让 wallet 侧天然幂等。
func (s *Service) Reward(tenantID, userID int64) (rewarded bool, coins int64, balance int64, countToday, limit int, err error) {
	limit = sysconfig.GetInt(tenantID, sysconfig.KeyAdRewardDaily)
	coins = sysconfig.GetInt64(tenantID, sysconfig.KeyAdRewardCoins)
	on := sysconfig.GetBool(tenantID, sysconfig.KeyAdEnabled) &&
		sysconfig.GetBool(tenantID, sysconfig.KeyAdRewardCoinOn) &&
		sysconfig.GetString(tenantID, sysconfig.KeyAdRewardUnit) != ""
	if !on || limit <= 0 || coins <= 0 {
		bal, _ := s.wlt.Balance(userID)
		return false, coins, bal, 0, limit, nil
	}
	ctx := context.Background()
	key := dayKey(tenantID, userID, today())
	n, e := cache.RDB.Incr(ctx, key).Result()
	if e != nil {
		log.Printf("[ad] reward incr err uid=%d: %v", userID, e)
		return false, coins, 0, 0, limit, e
	}
	if n == 1 {
		cache.RDB.Expire(ctx, key, 48*time.Hour)
	}
	countToday = int(n)
	if int(n) > limit {
		cache.RDB.Decr(ctx, key)
		bal, _ := s.wlt.Balance(userID)
		return false, coins, bal, limit, limit, nil
	}
	bizNo := fmt.Sprintf("adreward:%d:%s:%d", userID, today(), n)
	if e := s.wlt.Credit(tenantID, userID, coins, wallet.SceneAdReward, bizNo); e != nil {
		log.Printf("[ad] reward credit err uid=%d: %v", userID, e)
		cache.RDB.Decr(ctx, key)
		return false, coins, 0, countToday - 1, limit, e
	}
	bal, _ := s.wlt.Balance(userID)
	return true, coins, bal, countToday, limit, nil
}
