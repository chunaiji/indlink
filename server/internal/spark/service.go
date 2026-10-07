// server/internal/spark/service.go
package spark

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"

	"gorm.io/gorm"
)

// Candidate 一个可被配对的人(真人或机器人)。
type Candidate struct {
	UserID   int64
	Nickname string
	Avatar   string
}

// Deps 外部依赖全部以函数注入:调度器要能在没有 DB / Redis / WS 的情况下被测。
type Deps struct {
	DB         *gorm.DB
	Online     func() []int64                              // chat.Hub.OnlineUserIDs
	Push       func(userID int64, payload any)             // chat.Hub.PushTo
	EnsureChat func(tenantID, u1, u2 int64) (int64, error) // chat.Service.EnsureFreeChat
	SendAs     func(tenantID, fromUserID, chatID int64, text, kind string) (*model.Message, error)
	Opening    func(tenantID, botUserID int64) string     // 机器人开场白
	ClaimDaily func(tenantID, userID int64, cap int) bool // robot.ClaimDailyReach
	IsBlocked  func(a, b int64) bool                      // moderation.Service.IsBlocked
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

// eligiblePeers 真人候选过滤:排除自己、当日已配过的、互相拉黑的、配额已满的。
//
// capLeft 是**只读**检查,真正占额度在选中之后的 ClaimDaily;两者之间的窗口里
// 对方可能被另一个 tick 占满——所以 matchReal 选中后 ClaimDaily 失败要换下一个。
func eligiblePeers(self int64, online []Candidate, pairedToday func(peer int64) bool,
	blocked func(peer int64) bool, capLeft func(peer int64) bool) []Candidate {
	out := make([]Candidate, 0, len(online))
	for _, c := range online {
		if c.UserID == self || pairedToday(c.UserID) || blocked(c.UserID) || !capLeft(c.UserID) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// matchReal 从候选里挑一个能真正占到额度的人。挑中后 ClaimDaily 失败就换下一个,
// 全都占不到才返回 false(调用方据此退回机器人)。
func (s *Service) matchReal(tenantID, self int64, peers []Candidate, cap int, roll func(int) int) (Candidate, bool) {
	pool := make([]Candidate, len(peers))
	copy(pool, peers)
	for len(pool) > 0 {
		i := roll(len(pool))
		pick := pool[i]
		pool = append(pool[:i], pool[i+1:]...)
		if s.d.ClaimDaily(tenantID, pick.UserID, cap) {
			return pick, true
		}
	}
	return Candidate{}, false
}

// 调度器状态键。
func nextKey(tenantID, userID int64) string { return fmt.Sprintf("spark:next:%d:%d", tenantID, userID) }
func seenKey(tenantID, userID int64) string { return fmt.Sprintf("spark:seen:%d:%d", tenantID, userID) }

// endOfDay 今天剩余时间,配对去重键的 TTL。
func endOfDay(now time.Time) time.Duration {
	y, m, d := now.Date()
	return time.Until(time.Date(y, m, d, 23, 59, 59, 0, now.Location()))
}

// Start 启动调度器:固定每分钟扫一次在线用户,到点的人各自匹配一次。
//
// 为什么是「固定 tick + 每人一个 Redis TTL 键」而不是给每人起一个定时器:
// 在线用户可能成千上万,每人一个 goroutine 定时器的代价远高于每分钟扫一遍。
func Start(d Deps, tenantID int64) {
	go func() {
		for {
			time.Sleep(time.Minute)
			func() {
				defer func() {
					// 调度器是常驻 goroutine:一次 panic 就让功能静默消失,
					// 这里兜住并打日志,下一分钟继续。
					if r := recover(); r != nil {
						log.Printf("[spark] tick panic: %v", r)
					}
				}()
				New(d).tick(tenantID, time.Now())
			}()
		}
	}()
	log.Printf("[spark] 调度器已启动(每 1 分钟一次;总开关 spark_enabled)")
}

func (s *Service) tick(tenantID int64, now time.Time) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeySparkEnabled) {
		return
	}
	if !inWindow(now.Hour(), sysconfig.GetInt(tenantID, sysconfig.KeySparkWindowStart),
		sysconfig.GetInt(tenantID, sysconfig.KeySparkWindowEnd)) {
		return
	}
	online := s.d.Online()
	if len(online) == 0 {
		return
	}
	// 在线 ID → 同租户的真人资料
	var users []model.User
	s.d.DB.Select("user_id, nickname, avatar, is_robot, status").
		Where("tenant_id = ? AND user_id IN ? AND is_robot = ? AND status = ?",
			tenantID, online, false, "active").Find(&users)
	if len(users) == 0 {
		return
	}
	pool := make([]Candidate, 0, len(users))
	for _, u := range users {
		pool = append(pool, Candidate{UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar})
	}

	ctx := context.Background()
	dailyCap := sysconfig.GetInt(tenantID, sysconfig.KeySparkUserDailyCap)
	if dailyCap <= 0 {
		dailyCap = 1
	}
	ratio := sysconfig.GetInt(tenantID, sysconfig.KeySparkRealRatio)
	minM := sysconfig.GetInt(tenantID, sysconfig.KeySparkIntervalMin)
	maxM := sysconfig.GetInt(tenantID, sysconfig.KeySparkIntervalMax)

	for _, self := range pool {
		if cache.RDB.Exists(ctx, nextKey(tenantID, self.UserID)).Val() > 0 {
			continue // 还没到点
		}
		// 先设下次时刻:匹配失败也不要在同一分钟原地重试
		cache.RDB.Set(ctx, nextKey(tenantID, self.UserID), 1, nextInterval(minM, maxM, rand.Intn))
		// 首次被扫到只设键不弹窗,否则用户一打开 App 就被糊脸
		if cache.RDB.SetNX(ctx, seenKey(tenantID, self.UserID), 1, 24*time.Hour).Val() {
			continue
		}
		if !s.d.ClaimDaily(tenantID, self.UserID, dailyCap) {
			continue
		}
		s.matchOne(ctx, tenantID, self, pool, dailyCap, ratio, now)
	}
}

// matchOne 给一个人配一次。真人候选为空或都占不到额度时退回机器人,不浪费这次机会。
func (s *Service) matchOne(ctx context.Context, tenantID int64, self Candidate,
	pool []Candidate, dailyCap, ratio int, now time.Time) {
	paired := func(peer int64) bool {
		return cache.RDB.Exists(ctx, pairKey(tenantID, self.UserID, peer)).Val() > 0
	}
	if pickKind(ratio, rand.Intn) == KindReal {
		peers := eligiblePeers(self.UserID, pool, paired,
			func(peer int64) bool { return s.d.IsBlocked(self.UserID, peer) },
			func(peer int64) bool { return true }, // 只读额度检查交给 matchReal 的 ClaimDaily
		)
		if peer, ok := s.matchReal(tenantID, self.UserID, peers, dailyCap, rand.Intn); ok {
			s.deliverReal(ctx, tenantID, self, peer, now)
			return
		}
	}
	s.deliverRobot(ctx, tenantID, self, paired, now)
}
