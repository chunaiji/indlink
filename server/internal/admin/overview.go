package admin

import (
	"math"
	"time"

	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// cst 后台统计统一用东八区计算「今日/本周/本月」边界与趋势分桶。
// 修正历史代码 time.Now().Truncate(24h) 按 UTC 截断、东八区偏 8 小时的问题。
var cst = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*3600)
}()

// periodStarts 返回 now 所在 CST 下的「今日 / 本周一 / 本月 1 号 / 本年 1 月 1 号」零点。
func periodStarts(now time.Time) (today, week, month, year time.Time) {
	n := now.In(cst)
	today = time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, cst)
	// 周一作为一周起点;Go 中 Sunday=0,Monday=1。
	offset := (int(today.Weekday()) + 6) % 7
	week = today.AddDate(0, 0, -offset)
	month = time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, cst)
	year = time.Date(n.Year(), 1, 1, 0, 0, 0, 0, cst)
	return
}

// trendDays 返回近 days 天的 CST 日零点切片,最旧在前、最后一个是 now 当天。
func trendDays(now time.Time, days int) []time.Time {
	n := now.In(cst)
	last := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, cst)
	out := make([]time.Time, days)
	for i := 0; i < days; i++ {
		out[i] = last.AddDate(0, 0, -(days - 1 - i))
	}
	return out
}

// fillTrend 把按天聚合的结果(键为 "2006-01-02")对齐到 days 窗口,缺失日补 0。
func fillTrend(days []time.Time, byDay map[string]int64) []int64 {
	out := make([]int64, len(days))
	for i, d := range days {
		out[i] = byDay[d.Format("2006-01-02")]
	}
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// scope 在 tenantID>0 时按租户过滤,=0 表示全租户汇总。
func scope(db *gorm.DB, tenantID int64) *gorm.DB {
	if tenantID > 0 {
		return db.Where("tenant_id = ?", tenantID)
	}
	return db
}

// PeriodInt / PeriodYuan 同一指标的「今日/本周/本月/本年」四段值。
type PeriodInt struct {
	Today int64 `json:"today"`
	Week  int64 `json:"week"`
	Month int64 `json:"month"`
	Year  int64 `json:"year"`
}

type PeriodYuan struct {
	Today float64 `json:"today"`
	Week  float64 `json:"week"`
	Month float64 `json:"month"`
	Year  float64 `json:"year"`
}

type OverviewKPI struct {
	NewUsers       PeriodInt  `json:"new_users"`       // 真人新增(is_robot=0)
	ActiveUsers    PeriodInt  `json:"active_users"`    // 真人活跃(last_active_at 在期内)
	RechargeYuan   PeriodYuan `json:"recharge_yuan"`   // 已支付订单 price_fen/100
	RechargeOrders PeriodInt  `json:"recharge_orders"` // 已支付订单数
	ConsumeCoins   PeriodInt  `json:"consume_coins"`   // 流水 debit 的 coins 求和
	PayingUsers    PeriodInt  `json:"paying_users"`    // 已支付订单去重用户
	ArpuYuan       PeriodYuan `json:"arpu_yuan"`       // 充值额/付费人数
}

type OverviewTrend struct {
	Dates        []string  `json:"dates"`         // "01-02",近 N 天
	NewUsers     []int64   `json:"new_users"`
	RechargeYuan []float64 `json:"recharge_yuan"` // 元
	ConsumeCoins []int64   `json:"consume_coins"` // 猛币
}

type OverviewResp struct {
	KPI   OverviewKPI   `json:"kpi"`
	Trend OverviewTrend `json:"trend"`
}

// Overview 后台概览的当期 KPI(今日/本周/本月/本年)+ 近 30 天日趋势。
// tenantID=0 为全租户汇总。时间口径见 periodStarts/trendDays(均按 CST)。
// 趋势按 DB 会话墙钟日分组,生产部署为 CST 服务器(loc=Local),与 CST 标签对齐。
func (s *Service) Overview(tenantID int64) OverviewResp {
	now := time.Now()
	today, week, month, year := periodStarts(now)
	var resp OverviewResp

	// 真人新增用户
	newUsers := func(since time.Time) int64 {
		var n int64
		scope(s.db.Model(&model.User{}).Where("is_robot = 0 AND created_at >= ?", since), tenantID).Count(&n)
		return n
	}
	resp.KPI.NewUsers = PeriodInt{newUsers(today), newUsers(week), newUsers(month), newUsers(year)}

	// 真人活跃用户(last_active_at 在期内)
	activeUsers := func(since time.Time) int64 {
		var n int64
		scope(s.db.Model(&model.User{}).Where("is_robot = 0 AND last_active_at >= ?", since), tenantID).Count(&n)
		return n
	}
	resp.KPI.ActiveUsers = PeriodInt{activeUsers(today), activeUsers(week), activeUsers(month), activeUsers(year)}

	// 充值额(元)
	rechargeFen := func(since time.Time) int64 {
		var fen int64
		scope(s.db.Model(&model.PayOrder{}).Where("status = 'paid' AND paid_at >= ?", since), tenantID).
			Select("COALESCE(SUM(price_fen),0)").Scan(&fen)
		return fen
	}
	rT, rW, rM, rY := rechargeFen(today), rechargeFen(week), rechargeFen(month), rechargeFen(year)
	resp.KPI.RechargeYuan = PeriodYuan{round2(float64(rT) / 100), round2(float64(rW) / 100), round2(float64(rM) / 100), round2(float64(rY) / 100)}

	// 已支付订单数
	rechargeCnt := func(since time.Time) int64 {
		var n int64
		scope(s.db.Model(&model.PayOrder{}).Where("status = 'paid' AND paid_at >= ?", since), tenantID).Count(&n)
		return n
	}
	resp.KPI.RechargeOrders = PeriodInt{rechargeCnt(today), rechargeCnt(week), rechargeCnt(month), rechargeCnt(year)}

	// 消费流水(猛币)
	consume := func(since time.Time) int64 {
		var c int64
		scope(s.db.Model(&model.WalletTxn{}).Where("direction = 'debit' AND created_at >= ?", since), tenantID).
			Select("COALESCE(SUM(coins),0)").Scan(&c)
		return c
	}
	resp.KPI.ConsumeCoins = PeriodInt{consume(today), consume(week), consume(month), consume(year)}

	// 付费用户(去重)
	paying := func(since time.Time) int64 {
		var n int64
		scope(s.db.Model(&model.PayOrder{}).Where("status = 'paid' AND paid_at >= ?", since), tenantID).
			Select("COUNT(DISTINCT user_id)").Scan(&n)
		return n
	}
	pT, pW, pM, pY := paying(today), paying(week), paying(month), paying(year)
	resp.KPI.PayingUsers = PeriodInt{pT, pW, pM, pY}

	arpu := func(fen, users int64) float64 {
		if users <= 0 {
			return 0
		}
		return round2(float64(fen) / 100 / float64(users))
	}
	resp.KPI.ArpuYuan = PeriodYuan{arpu(rT, pT), arpu(rW, pW), arpu(rM, pM), arpu(rY, pY)}

	// 近 30 天趋势
	days := trendDays(now, 30)
	windowStart := days[0]
	resp.Trend.Dates = make([]string, len(days))
	for i, d := range days {
		resp.Trend.Dates[i] = d.Format("01-02")
	}

	groupByDay := func(tbl interface{}, where string, agg string, since time.Time, tcol string) map[string]int64 {
		var rows []struct {
			Day string
			V   int64
		}
		scope(s.db.Model(tbl).Where(where, since), tenantID).
			Select("DATE_FORMAT("+tcol+",'%Y-%m-%d') AS day, "+agg+" AS v").
			Group("day").Scan(&rows)
		m := make(map[string]int64, len(rows))
		for _, r := range rows {
			m[r.Day] = r.V
		}
		return m
	}

	resp.Trend.NewUsers = fillTrend(days,
		groupByDay(&model.User{}, "is_robot = 0 AND created_at >= ?", "COUNT(*)", windowStart, "created_at"))

	fen := fillTrend(days,
		groupByDay(&model.PayOrder{}, "status = 'paid' AND paid_at >= ?", "COALESCE(SUM(price_fen),0)", windowStart, "paid_at"))
	resp.Trend.RechargeYuan = make([]float64, len(fen))
	for i, v := range fen {
		resp.Trend.RechargeYuan[i] = round2(float64(v) / 100)
	}

	resp.Trend.ConsumeCoins = fillTrend(days,
		groupByDay(&model.WalletTxn{}, "direction = 'debit' AND created_at >= ?", "COALESCE(SUM(coins),0)", windowStart, "created_at"))

	return resp
}
