package robot

import (
	gocontext "context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"
)

// trailingRobotMsgs 统计消息(升序)末尾连续由 botUserID 发送的条数。
func trailingRobotMsgs(msgs []model.Message, botUserID int64) int {
	cnt := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].SenderID == botUserID {
			cnt++
		} else {
			break
		}
	}
	return cnt
}

// isOutreachCandidate 判断真人是否为破冰目标:新注册 或 长期沉默。
func isOutreachCandidate(created, lastActive, now time.Time, newHours, silentDays int) bool {
	if now.Sub(created) <= time.Duration(newHours)*time.Hour {
		return true
	}
	if now.Sub(lastActive) >= time.Duration(silentDays)*24*time.Hour {
		return true
	}
	return false
}

var openers = []string{
	"嗨,刷到你,感觉你挺有意思的,想认识一下~",
	"你好呀,今天过得怎么样?",
	"突然想找人聊聊天,你最近在忙什么呀?",
	"看到你就想打个招呼,你平时喜欢做什么?",
	"路过你的主页,想跟你说说话~",
	"今天心情不错,想找个人分享一下,你呢?",
	"感觉你是个有故事的人,想听你讲讲。",
	"嘿,好巧,能认识一下吗?",
}

// bottleChatOpeners 回信后引导加聊的开场白变体(maybeReply 用)。
var bottleChatOpeners = []string{
	"刚看到你的瓶子,挺有共鸣的,想和你多聊两句~",
	"你那个瓶子我看了好几遍,想跟你聊聊。",
	"捞到你的瓶子啦,感觉我们会聊得来~",
	"看了你写的东西,有点想认识你。",
	"你的瓶子写得真好,方便聊聊吗?",
}

var nudges = []string{
	"还在吗?突然想到还没听你说完呢~",
	"在忙吗?有空回我一句呀。",
	"是不是走开啦?我还想接着聊呢。",
	"忙完了叫我一声呀~",
	"咦,人呢?我等你回来聊~",
	"你一忙起来就不理我啦?",
	"想听听你的想法,回来告诉我呀。",
	"没事,你先忙,我在这儿等你~",
}

func randLine(pool []string) string { return pool[rand.Intn(len(pool))] }

// proactiveOutreach 对新/沉默真人主动发起一次开场私聊(每租户每日上限 + 每真人每日占位)。
func (s *Service) proactiveOutreach(tenantID int64, robots []model.User) {
	sender := s.getSender()
	if sender == nil {
		return
	}
	dailyCap := sysconfig.GetInt(tenantID, sysconfig.KeyRobotOutreachDailyCap)
	if dailyCap <= 0 {
		return
	}
	ctx := gocontext.Background()
	dayKey := fmt.Sprintf("robot_outreach:%d:%s", tenantID, time.Now().Format("20060102"))
	used, _ := cache.RDB.Get(ctx, dayKey).Int()
	if used >= dailyCap {
		return
	}

	newHours := sysconfig.GetInt(tenantID, sysconfig.KeyRobotOutreachNewHours)
	silentDays := sysconfig.GetInt(tenantID, sysconfig.KeyRobotOutreachSilentDays)
	now := time.Now()
	newSince := now.Add(-time.Duration(newHours) * time.Hour)
	silentBefore := now.Add(-time.Duration(silentDays) * 24 * time.Hour)

	// 目标:真人、active、(新注册 或 沉默)、且无任何机器人会话。
	// 会话表里机器人可能是 user_a 或 user_b,已有机器人会话的"真人"要取对侧字段,
	// 不能直接选机器人自己那一列(否则 NOT IN 恒真,去重失效)。
	robotIDs := s.db.Model(&model.User{}).Select("user_id").Where("tenant_id = ? AND is_robot = 1", tenantID)
	humanSideOfA := s.db.Model(&model.Chat{}).Select("user_b").
		Where("tenant_id = ? AND user_a IN (?)", tenantID, robotIDs)
	humanSideOfB := s.db.Model(&model.Chat{}).Select("user_a").
		Where("tenant_id = ? AND user_b IN (?)", tenantID, robotIDs)

	batch := dailyCap - used
	if batch > 10 {
		batch = 10 // 每次 tick 最多 10 个,平滑投放
	}

	// SQL 只做粗筛(真人+active+无机器人会话+新注册或沉默的并集时间窗口),
	// 精确判定交给 isOutreachCandidate 在 Go 侧再过滤一遍,让该函数真正参与决策。
	var candidates []model.User
	s.db.Where("tenant_id = ? AND is_robot = 0 AND status = ?", tenantID, "active").
		Where("(created_at >= ? OR last_active_at <= ?)", newSince, silentBefore).
		Where("user_id NOT IN (?)", humanSideOfA).
		Where("user_id NOT IN (?)", humanSideOfB).
		Order("created_at desc").Limit(batch * 3).Find(&candidates)

	userCap := sysconfig.GetInt(tenantID, sysconfig.KeyOutreachUserDailyCap)
	if userCap <= 0 {
		userCap = 1
	}

	sent := 0
	for _, u := range candidates {
		if sent >= batch {
			break
		}
		if !isOutreachCandidate(u.CreatedAt, u.LastActiveAt, now, newHours, silentDays) {
			continue
		}
		// 与旧 outreach.go 共享同一个"每真人每日"占位 key,防止两套机制叠加骚扰同一人。
		if !ClaimDailyReach(tenantID, u.UserID, userCap) {
			continue
		}
		r := robots[rand.Intn(len(robots))]
		chatID, err := sender.EnsureRobotChat(tenantID, r.UserID, u.UserID)
		if err != nil {
			continue
		}
		if _, err := sender.SendMessage(tenantID, r.UserID, chatID, randLine(openers), "text"); err != nil {
			continue
		}
		sent++
	}
	if sent > 0 {
		cache.RDB.IncrBy(ctx, dayKey, int64(sent))
		cache.RDB.Expire(ctx, dayKey, 48*time.Hour)
		log.Printf("[robot] tenant=%d 主动破冰 %d 人", tenantID, sent)
	}
}

// nudgeStalledChats 对"机器人已发言、用户沉默"的会话追问一句(每会话限次)。
func (s *Service) nudgeStalledChats(tenantID int64, robots []model.User) {
	sender := s.getSender()
	if sender == nil {
		return
	}
	silentMin := sysconfig.GetInt(tenantID, sysconfig.KeyRobotNudgeSilentMin)
	maxNudge := sysconfig.GetInt(tenantID, sysconfig.KeyRobotNudgeMax)
	if silentMin <= 0 || maxNudge <= 0 {
		return
	}
	now := time.Now()
	upper := now.Add(-time.Duration(silentMin) * time.Minute)
	lower := upper.Add(-60 * time.Minute) // 只抓刚沉默的一小时窗口,避免反复扫历史

	robotIDs := s.db.Model(&model.User{}).Select("user_id").Where("tenant_id = ? AND is_robot = 1", tenantID)
	var chats []model.Chat
	s.db.Where("tenant_id = ? AND updated_at > ? AND updated_at <= ?", tenantID, lower, upper).
		Where("user_a IN (?) OR user_b IN (?)", robotIDs, robotIDs).
		Order("updated_at asc").Limit(20).Find(&chats)

	sent := 0
	for _, ch := range chats {
		// 找出该会话里的机器人一方
		botID := ch.UserA
		var cntA int64
		s.db.Model(&model.User{}).Where("user_id = ? AND is_robot = 1", ch.UserA).Count(&cntA)
		if cntA == 0 {
			botID = ch.UserB
		}
		msgs, err := sender.HistoryRecent(ch.ChatID, 6)
		if err != nil {
			continue
		}
		tr := trailingRobotMsgs(msgs, botID)
		if tr < 1 || tr > maxNudge {
			continue // 0=用户最后发言(等正常回复);>max=已追够
		}
		if _, err := sender.SendMessage(tenantID, botID, ch.ChatID, randLine(nudges), "text"); err != nil {
			continue
		}
		sent++
	}
	if sent > 0 {
		log.Printf("[robot] tenant=%d 沉默追问 %d 条", tenantID, sent)
	}
}
