// Package checkin 每日签到领金币(#增长1)。7 天阶梯:连续第 N 天发 ladder[(N-1)%7],断签重头。
package checkin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/cache"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

func today() string { return time.Now().Format("20060102") }
func bizNo(userID int64, date string) string {
	return fmt.Sprintf("checkin:%d:%s", userID, date)
}

// ladder 解析阶梯配置;解析失败/为空回退 [checkin_coins] 单档。
func ladder(tenantID int64) []int64 {
	raw := sysconfig.GetString(tenantID, sysconfig.KeyCheckinLadder)
	var out []int64
	for _, p := range strings.Split(raw, ",") {
		n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || n < 0 {
			out = nil
			break
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		out = []int64{sysconfig.GetInt64(tenantID, sysconfig.KeyCheckinCoins)}
	}
	return out
}

func ladderCoins(lad []int64, dayIdx int) int64 { // dayIdx 从 1 起
	return lad[(dayIdx-1)%len(lad)]
}

// streakEndingAt 以 end(20060102) 为终点的连续签到天数;end 当天未签返回 0。最多回溯 366 天。
func (s *Service) streakEndingAt(userID int64, end time.Time) int {
	var dates []string
	s.db.Model(&model.CheckinLog{}).
		Where("user_id = ? AND date >= ?", userID, end.AddDate(0, 0, -366).Format("20060102")).
		Pluck("date", &dates)
	set := make(map[string]bool, len(dates))
	for _, d := range dates {
		set[d] = true
	}
	n := 0
	for d := end; set[d.Format("20060102")]; d = d.AddDate(0, 0, -1) {
		n++
	}
	return n
}

func makeupKey(tenantID, userID int64) string {
	return fmt.Sprintf("checkin:makeup:%d:%d:%s", tenantID, userID, time.Now().Format("200601"))
}

func makeupUsed(tenantID, userID int64) int {
	n, _ := cache.RDB.Get(context.Background(), makeupKey(tenantID, userID)).Int()
	return n
}

// StatusResp 签到状态(coins 为兼容旧客户端保留 = today_coins)。
type StatusResp struct {
	Enabled     bool    `json:"enabled"`
	Coins       int64   `json:"coins"`
	SignedToday bool    `json:"signed_today"`
	Streak      int     `json:"streak"`       // 当前连续天数(今天已签则含今天,否则截至昨天)
	TodayIndex  int     `json:"today_index"`  // 今天是连续第几天(1-based,无论是否已签)
	TodayCoins  int64   `json:"today_coins"`  // 今天档位币数
	Ladder      []int64 `json:"ladder"`       // 阶梯配置
	CanMakeup   bool    `json:"can_makeup"`   // 昨天缺签且本月补签次数未用完
	MakeupUsed  int     `json:"makeup_used"`  // 本月已补签次数
	MakeupLimit int     `json:"makeup_limit"` // 每月补签上限
}

// Status 返回签到状态与阶梯进度。
func (s *Service) Status(tenantID, userID int64) StatusResp {
	r := StatusResp{
		Enabled:     sysconfig.GetBool(tenantID, sysconfig.KeyCheckinEnabled),
		Ladder:      ladder(tenantID),
		MakeupLimit: sysconfig.GetInt(tenantID, sysconfig.KeyCheckinMakeupLimit),
	}
	now := time.Now()
	todayStreak := s.streakEndingAt(userID, now)
	r.SignedToday = todayStreak > 0
	if r.SignedToday {
		r.Streak = todayStreak
		r.TodayIndex = todayStreak
	} else {
		r.Streak = s.streakEndingAt(userID, now.AddDate(0, 0, -1))
		r.TodayIndex = r.Streak + 1
	}
	r.TodayCoins = ladderCoins(r.Ladder, r.TodayIndex)
	r.Coins = r.TodayCoins
	r.MakeupUsed = makeupUsed(tenantID, userID)
	yesterdaySigned := s.streakEndingAt(userID, now.AddDate(0, 0, -1)) > 0
	r.CanMakeup = r.Enabled && !yesterdaySigned && r.MakeupLimit > 0 && r.MakeupUsed < r.MakeupLimit
	return r
}

// Sign 执行签到:按连续天数档位发币。重复签到返回 already=true 不重复发币。
func (s *Service) Sign(tenantID, userID int64) (coins int64, balance int64, already bool, dayIdx int, err error) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyCheckinEnabled) {
		return 0, 0, false, 0, errs.New(errs.CodeBadRequest, "签到未开启")
	}
	now := time.Now()
	date := now.Format("20060102")
	dayIdx = s.streakEndingAt(userID, now.AddDate(0, 0, -1)) + 1
	reward := ladderCoins(ladder(tenantID), dayIdx)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		rec := model.CheckinLog{TenantID: tenantID, UserID: userID, Date: date, Coins: reward, CreatedAt: now}
		if e := tx.Create(&rec).Error; e != nil {
			if isDuplicateKey(e) {
				already = true
				return nil
			}
			return e // 真实 DB 错误：回滚并上抛
		}
		if reward > 0 {
			if e := wallet.CreditTx(tx, tenantID, userID, reward, wallet.SceneCheckin, bizNo(userID, date)); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, false, 0, err
	}

	var w model.Wallet
	s.db.Select("balance").First(&w, "user_id = ?", userID)
	if already {
		return 0, w.Balance, true, dayIdx, nil
	}
	return reward, w.Balance, false, dayIdx, nil
}

// Makeup 看完激励视频后补签昨天:月上限 Redis 计数,币按昨天所处档位。
// 与 ad 模块一致信任客户端看完回调;bizNo 与正常签到同格式,钱包侧天然幂等。
func (s *Service) Makeup(tenantID, userID int64) (coins int64, balance int64, err error) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyCheckinEnabled) {
		return 0, 0, errs.New(errs.CodeBadRequest, "签到未开启")
	}
	limit := sysconfig.GetInt(tenantID, sysconfig.KeyCheckinMakeupLimit)
	if limit <= 0 {
		return 0, 0, errs.New(errs.CodeBadRequest, "补签未开启")
	}
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	if s.streakEndingAt(userID, yesterday) > 0 {
		return 0, 0, errs.New(errs.CodeBadRequest, "昨天已签到,无需补签")
	}

	// 月上限:原子 Incr,超限/落库失败回滚计数(参照 ad 模块)
	ctx := context.Background()
	key := makeupKey(tenantID, userID)
	n, e := cache.RDB.Incr(ctx, key).Result()
	if e != nil {
		return 0, 0, e
	}
	if n == 1 {
		cache.RDB.Expire(ctx, key, 45*24*time.Hour)
	}
	if int(n) > limit {
		cache.RDB.Decr(ctx, key)
		return 0, 0, errs.New(errs.CodeBadRequest, "本月补签次数已用完")
	}

	date := yesterday.Format("20060102")
	dayIdx := s.streakEndingAt(userID, yesterday.AddDate(0, 0, -1)) + 1
	reward := ladderCoins(ladder(tenantID), dayIdx)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		rec := model.CheckinLog{TenantID: tenantID, UserID: userID, Date: date, Coins: reward, CreatedAt: now}
		if e := tx.Create(&rec).Error; e != nil {
			if isDuplicateKey(e) {
				return errs.New(errs.CodeBadRequest, "昨天已签到,无需补签")
			}
			return e
		}
		if reward > 0 {
			if e := wallet.CreditTx(tx, tenantID, userID, reward, wallet.SceneCheckin, bizNo(userID, date)); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		cache.RDB.Decr(ctx, key)
		return 0, 0, err
	}

	var w model.Wallet
	s.db.Select("balance").First(&w, "user_id = ?", userID)
	return reward, w.Balance, nil
}

// isDuplicateKey 判断是否 MySQL 唯一键冲突(ER_DUP_ENTRY 1062)。
func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
