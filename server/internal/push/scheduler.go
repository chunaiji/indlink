package push

import (
	"log"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
)

// StartCheckinCron 启动每日 09:00 签到提醒定时任务。
func StartCheckinCron(svc *Service) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))
			svc.sendCheckinAll()
		}
	}()
}

// StartNightCron 深夜场开场前 5 分钟向 activity 订阅用户推送(开关/时段按各租户 sysconfig)。
// 每分钟对表:命中「night_start 前 5 分钟」这一分钟时触发,多租户各自判断。
func StartNightCron(svc *Service) {
	go func() {
		for {
			now := time.Now()
			next := now.Truncate(time.Minute).Add(time.Minute)
			time.Sleep(time.Until(next))
			svc.sendNightAll(time.Now())
		}
	}()
}

func (s *Service) sendNightAll(now time.Time) {
	var subs []model.PushSubscription
	s.db.Where("scene = ?", "activity").Find(&subs)
	if len(subs) == 0 {
		return
	}
	// 按租户过滤:开关开 且 当前正是 night_start 前 5 分钟
	okTenant := map[int64]bool{}
	title := map[int64]string{}
	for _, sub := range subs {
		if _, seen := okTenant[sub.TenantID]; seen {
			continue
		}
		enabled := sysconfig.GetBool(sub.TenantID, sysconfig.KeyNightBottleEnabled)
		start := sysconfig.GetInt(sub.TenantID, sysconfig.KeyNightStart)
		fire := time.Date(now.Year(), now.Month(), now.Day(), start, 0, 0, 0, now.Location()).Add(-5 * time.Minute)
		okTenant[sub.TenantID] = enabled && now.Format("15:04") == fire.Format("15:04")
		title[sub.TenantID] = sysconfig.GetString(sub.TenantID, sysconfig.KeyNightPushTitle)
	}
	sent := 0
	for _, sub := range subs {
		if !okTenant[sub.TenantID] {
			continue
		}
		sub := sub
		sent++
		go s.sendOne(sub.TenantID, sub.OpenID, sub.TemplateID, "/pages/ocean/ocean", map[string]MsgValue{
			"thing1": {Value: "深夜瓶"},
			"thing2": {Value: title[sub.TenantID]},
			"thing3": {Value: "漂流瓶"},
		})
	}
	if sent > 0 {
		log.Printf("[push/cron] night 推送 count=%d", sent)
	}
}

func (s *Service) sendCheckinAll() {
	var subs []model.PushSubscription
	s.db.Where("scene = ?", "checkin").Find(&subs)
	if len(subs) == 0 {
		return
	}

	dateStr := time.Now().Format("2006年01月02日")
	log.Printf("[push/cron] checkin 推送 count=%d date=%s", len(subs), dateStr)

	for _, sub := range subs {
		sub := sub
		go s.sendOne(sub.TenantID, sub.OpenID, sub.TemplateID, "/pages/index/index", map[string]MsgValue{
			"thing1":     {Value: "每日签到"},
			"thing2":     {Value: "待签到"},
			"date_time1": {Value: dateStr + " 09:00"},
		})
	}
}
