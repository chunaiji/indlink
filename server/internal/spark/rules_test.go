package spark

import (
	"testing"
	"time"
)

// roll 注入确定性「随机数」:返回固定值,便于断言分支。
func fixedRoll(v int) func(int) int { return func(int) int { return v } }

func TestPickKind(t *testing.T) {
	// ratio=30: roll 返回 0..29 配真人,30..99 配机器人
	if got := pickKind(30, fixedRoll(0)); got != KindReal {
		t.Errorf("roll=0 ratio=30 应配真人, got %v", got)
	}
	if got := pickKind(30, fixedRoll(29)); got != KindReal {
		t.Errorf("roll=29 ratio=30 应配真人, got %v", got)
	}
	if got := pickKind(30, fixedRoll(30)); got != KindRobot {
		t.Errorf("roll=30 ratio=30 应配机器人, got %v", got)
	}
	// 边界:0 永远机器人,100 永远真人
	if got := pickKind(0, fixedRoll(0)); got != KindRobot {
		t.Errorf("ratio=0 应全机器人, got %v", got)
	}
	if got := pickKind(100, fixedRoll(99)); got != KindReal {
		t.Errorf("ratio=100 应全真人, got %v", got)
	}
	// 越界配置夹回 [0,100],不能让负数把 roll 的上界算成负的
	if got := pickKind(-5, fixedRoll(0)); got != KindRobot {
		t.Errorf("ratio=-5 应当成 0, got %v", got)
	}
	if got := pickKind(300, fixedRoll(99)); got != KindReal {
		t.Errorf("ratio=300 应当成 100, got %v", got)
	}
}

// 运营把 min/max 填反、填 0、填负数,都不能让 rand.Intn 收到非正数而 panic——
// 那会把整个调度 goroutine 打死,功能静默消失。
func TestNextInterval(t *testing.T) {
	cases := []struct {
		min, max, roll int
		want           time.Duration
	}{
		{20, 60, 0, 20 * time.Minute},  // 取下界
		{20, 60, 40, 60 * time.Minute}, // roll 上界(span=41,roll=40)
		{20, 20, 0, 20 * time.Minute},  // 相等
		{60, 20, 0, 20 * time.Minute},  // 填反了:交换后取下界
		{0, 0, 0, defaultInterval},     // 都没配
		{-5, -1, 0, defaultInterval},   // 负数
		{0, 60, 0, defaultInterval},    // 下界非正:整体回落,不要 0 间隔狂刷
	}
	for _, c := range cases {
		if got := nextInterval(c.min, c.max, fixedRoll(c.roll)); got != c.want {
			t.Errorf("nextInterval(%d,%d,roll=%d) = %v want %v", c.min, c.max, c.roll, got, c.want)
		}
	}
}

func TestInWindow(t *testing.T) {
	if !inWindow(12, 10, 23) || !inWindow(10, 10, 23) {
		t.Error("区间内与左端点应在窗内")
	}
	if inWindow(23, 10, 23) || inWindow(9, 10, 23) {
		t.Error("右端点不含,左端点之前不在窗内")
	}
	if inWindow(12, 23, 10) {
		t.Error("start >= end 视为未配置,一律不在窗内(与 outreach 同语义)")
	}
}

// 去重键与两个人的先后顺序无关:否则 A→B 与 B→A 会被当成两对,同一分钟互弹两次。
func TestPairKeyIsOrderIndependent(t *testing.T) {
	if pairKey(7, 100, 200) != pairKey(7, 200, 100) {
		t.Fatal("pairKey 必须与顺序无关")
	}
	if pairKey(7, 100, 200) == pairKey(8, 100, 200) {
		t.Fatal("不同租户必须是不同的键")
	}
}
