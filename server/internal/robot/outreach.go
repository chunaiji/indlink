package robot

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"
)

// inWindow 判断当前小时是否在 [start, end) 内；start>=end 视为关闭(返回 false)。
func inWindow(hour, start, end int) bool {
	if start >= end {
		return false
	}
	return hour >= start && hour < end
}

// sampleCount 抽样数量 = total*pct/100，向下取整。
func sampleCount(total, pct int) int {
	if total <= 0 || pct <= 0 {
		return 0
	}
	return total * pct / 100
}

// StartOutreachCron 启动机器人主动触达定时器(#增长5)。
func StartOutreachCron(rsvc *Service, sender MessageSender, defaultTenant int64) {
	go func() {
		for {
			tickMin := sysconfig.GetInt(defaultTenant, sysconfig.KeyOutreachTickMin)
			if tickMin <= 0 {
				tickMin = 30
			}
			time.Sleep(time.Duration(tickMin) * time.Minute)
			outreachTick(rsvc, sender, defaultTenant)
		}
	}()
	log.Printf("[robot/outreach] cron started")
}

func outreachTick(rsvc *Service, sender MessageSender, tenantID int64) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyOutreachEnabled) {
		return
	}
	now := time.Now()
	if !inWindow(now.Hour(), sysconfig.GetInt(tenantID, sysconfig.KeyOutreachWindowStart), sysconfig.GetInt(tenantID, sysconfig.KeyOutreachWindowEnd)) {
		return
	}
	lookback := sysconfig.GetInt(tenantID, sysconfig.KeyOutreachLookbackMin)
	if lookback <= 0 {
		lookback = 30
	}
	since := now.Add(-time.Duration(lookback) * time.Minute)

	// 近期活跃真人候选
	var candidates []model.User
	rsvc.db.Where("tenant_id = ? AND is_robot = ? AND status = ? AND is_muted = ? AND last_active_at >= ?",
		tenantID, false, "active", false, since).
		Limit(2000).Find(&candidates)
	if len(candidates) == 0 {
		return
	}

	// 抽样
	pick := sampleCount(len(candidates), sysconfig.GetInt(tenantID, sysconfig.KeyOutreachProbability))
	if pick == 0 {
		return
	}
	rand.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	if pick > len(candidates) {
		pick = len(candidates)
	}
	selected := candidates[:pick]

	// 机器人池
	var robots []model.User
	rsvc.db.Where("tenant_id = ? AND is_robot = ? AND status = ?", tenantID, true, "active").Find(&robots)
	if len(robots) == 0 {
		return
	}

	dailyCap := sysconfig.GetInt(tenantID, sysconfig.KeyOutreachUserDailyCap)
	if dailyCap <= 0 {
		dailyCap = 1
	}
	sent := 0
	for _, u := range selected {
		if !ClaimDailyReach(tenantID, u.UserID, dailyCap) {
			continue
		}
		bot := robots[rand.Intn(len(robots))]
		chatID, err := sender.EnsureRobotChat(tenantID, bot.UserID, u.UserID)
		if err != nil {
			continue
		}
		text := genOpening(rsvc, tenantID, bot.UserID)
		if text == "" {
			continue
		}
		if _, err := sender.SendMessage(tenantID, bot.UserID, chatID, text, "text"); err != nil {
			continue
		}
		sent++
	}
	if sent > 0 {
		log.Printf("[robot/outreach] tenant=%d 触达 %d 人 (候选=%d 抽样=%d)", tenantID, sent, len(candidates), pick)
	}
}

// ClaimDailyReach 占用一次用户当日被打扰额度；超过 dailyCap 返回 false。
//
// 机器人主动触达与火花匹配共用这一个计数器(Redis key outreach:租户:用户:日期)，
// 所以同一天内两者互斥——否则一个用户可能上午被机器人搭讪、下午又被火花弹窗，
// 叠加起来就是骚扰。
func ClaimDailyReach(tenantID, userID int64, dailyCap int) bool {
	ctx := context.Background()
	key := fmt.Sprintf("outreach:%d:%d:%s", tenantID, userID, time.Now().Format("20060102"))
	n, err := cache.RDB.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if n == 1 {
		cache.RDB.Expire(ctx, key, 48*time.Hour)
	}
	if int(n) > dailyCap {
		cache.RDB.Decr(ctx, key)
		return false
	}
	return true
}

// 追加到 server/internal/robot/outreach.go
// Opening 供外部(spark)复用机器人开场白生成:LLM 失败时回退静态内容池。
func (s *Service) Opening(tenantID, botUserID int64) string {
	return genOpening(s, tenantID, botUserID)
}

// genOpening 生成机器人主动开场白；LLM 失败回退静态内容池(reply)。
func genOpening(rsvc *Service, tenantID, botUserID int64) string {
	if sysconfig.GetBool(tenantID, sysconfig.KeyAIChatEnabled) || sysconfig.GetBool(tenantID, sysconfig.KeyAIBotEnabled) {
		persona := rsvc.registry.Get(botUserID)
		var bot model.User
		rsvc.db.Select("language").First(&bot, "user_id = ?", botUserID)
		sys := buildPromptLang(persona, model.RobotMemory{Familiarity: 0.3}, bot.Language)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ask := "请你主动发起聊天，发一句自然、简短、符合你人设的开场问候，不要透露AI身份，不超过30字。"
		if IsEnglish(bot.Language) {
			ask = "Start the conversation: send one short, natural opening line that fits your character. Do not reveal you are an AI. Under 20 words."
		}
		reply, err := rsvc.llm.Chat(ctx, tenantID, sys, nil, ask)
		reply = sanitizeOutgoingReply(reply, "")
		if err == nil && reply != "" && !containsAIConfession(reply) {
			return reply
		}
	}
	// 回退静态池
	var pool []model.RobotContent
	rsvc.db.Where("tenant_id = ? AND type = ?", tenantID, "reply").Find(&pool)
	if len(pool) == 0 {
		return ""
	}
	return sanitizeOutgoingReply(pool[rand.Intn(len(pool))].Text, "")
}
