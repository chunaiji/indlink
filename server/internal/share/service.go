// Package share 分享领金币(#增长3)：信任客户端触发 + Redis 按天上限兜量。
package share

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

func today() string                               { return time.Now().Format("20060102") }
func dayKey(tenantID, userID int64, d string) string {
	return fmt.Sprintf("share:%d:%d:%s", tenantID, userID, d)
}

// Reward 占用一次每日分享额度并发放金币。达上限返回 rewarded=false(不报错)。
func (s *Service) Reward(tenantID, userID int64) (rewarded bool, coins int64, balance int64, countToday int, limit int, err error) {
	limit = sysconfig.GetInt(tenantID, sysconfig.KeyShareRewardDailyLimit)
	coins = sysconfig.GetInt64(tenantID, sysconfig.KeyShareRewardCoins)

	if !sysconfig.GetBool(tenantID, sysconfig.KeyShareRewardEnabled) || limit <= 0 || coins <= 0 {
		bal, _ := s.wlt.Balance(userID)
		return false, coins, bal, 0, limit, nil
	}

	ctx := context.Background()
	key := dayKey(tenantID, userID, today())
	n, e := cache.RDB.Incr(ctx, key).Result()
	if e != nil {
		log.Printf("[share] Reward incr redis err uid=%d: %v", userID, e)
		return false, coins, 0, 0, limit, e
	}
	if n == 1 {
		cache.RDB.Expire(ctx, key, 48*time.Hour)
	}
	countToday = int(n)
	if int(n) > limit {
		// 超额回滚计数，不发币
		cache.RDB.Decr(ctx, key)
		countToday = limit
		bal, _ := s.wlt.Balance(userID)
		return false, coins, bal, countToday, limit, nil
	}

	bizNo := fmt.Sprintf("share:%d:%s:%d", userID, today(), n)
	if e := s.wlt.Credit(tenantID, userID, coins, wallet.SceneShare, bizNo); e != nil {
		log.Printf("[share] Reward credit err uid=%d coins=%d: %v", userID, coins, e)
		cache.RDB.Decr(ctx, key) // 发币失败回滚计数
		return false, coins, 0, countToday - 1, limit, e
	}
	bal, be := s.wlt.Balance(userID)
	if be != nil {
		log.Printf("[share] Reward balance read err uid=%d: %v", userID, be)
	}
	return true, coins, bal, countToday, limit, nil
}
