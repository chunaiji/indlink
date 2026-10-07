package admin

import (
	"testing"
	"time"
)

// cst 是测试里用的东八区,与实现保持一致。
var testCST = time.FixedZone("CST", 8*3600)

func TestPeriodStarts_Weekday(t *testing.T) {
	// 2026-07-01 15:30 CST 是星期三。
	now := time.Date(2026, 7, 1, 15, 30, 0, 0, testCST)
	today, week, month, year := periodStarts(now)

	wantToday := time.Date(2026, 7, 1, 0, 0, 0, 0, testCST)
	if !today.Equal(wantToday) {
		t.Errorf("today = %v, want %v", today, wantToday)
	}
	// 当周周一是 2026-06-29。
	wantWeek := time.Date(2026, 6, 29, 0, 0, 0, 0, testCST)
	if !week.Equal(wantWeek) {
		t.Errorf("week = %v, want %v", week, wantWeek)
	}
	wantMonth := time.Date(2026, 7, 1, 0, 0, 0, 0, testCST)
	if !month.Equal(wantMonth) {
		t.Errorf("month = %v, want %v", month, wantMonth)
	}
	wantYear := time.Date(2026, 1, 1, 0, 0, 0, 0, testCST)
	if !year.Equal(wantYear) {
		t.Errorf("year = %v, want %v", year, wantYear)
	}
}

func TestPeriodStarts_Sunday(t *testing.T) {
	// 2026-07-05 是星期日,当周周一应是 2026-06-29。
	now := time.Date(2026, 7, 5, 9, 0, 0, 0, testCST)
	_, week, _, _ := periodStarts(now)
	wantWeek := time.Date(2026, 6, 29, 0, 0, 0, 0, testCST)
	if !week.Equal(wantWeek) {
		t.Errorf("sunday week = %v, want %v", week, wantWeek)
	}
}

func TestPeriodStarts_UsesCST(t *testing.T) {
	// 传入 UTC 时间,边界仍应落在 CST 当日零点。
	// 2026-06-30 17:00 UTC == 2026-07-01 01:00 CST,所以今日起点是 07-01 00:00 CST。
	now := time.Date(2026, 6, 30, 17, 0, 0, 0, time.UTC)
	today, _, _, _ := periodStarts(now)
	wantToday := time.Date(2026, 7, 1, 0, 0, 0, 0, testCST)
	if !today.Equal(wantToday) {
		t.Errorf("today(from UTC) = %v, want %v (CST)", today, wantToday)
	}
}

func TestTrendDays_LengthAndBounds(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 30, 0, 0, testCST)
	days := trendDays(now, 30)
	if len(days) != 30 {
		t.Fatalf("len = %d, want 30", len(days))
	}
	if got := days[29].Format("2006-01-02"); got != "2026-07-01" {
		t.Errorf("last day = %s, want 2026-07-01", got)
	}
	if got := days[0].Format("2006-01-02"); got != "2026-06-02" {
		t.Errorf("first day = %s, want 2026-06-02", got)
	}
}

func TestFillTrend_AlignsAndZeroFills(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 30, 0, 0, testCST)
	days := trendDays(now, 30)
	byDay := map[string]int64{
		"2026-07-01": 5,
		"2026-06-29": 2,
	}
	vals := fillTrend(days, byDay)
	if len(vals) != 30 {
		t.Fatalf("len = %d, want 30", len(vals))
	}
	if vals[29] != 5 {
		t.Errorf("last (07-01) = %d, want 5", vals[29])
	}
	if vals[27] != 2 { // 06-29 是倒数第3天
		t.Errorf("06-29 = %d, want 2", vals[27])
	}
	if vals[0] != 0 {
		t.Errorf("first (06-02, missing) = %d, want 0", vals[0])
	}
}
