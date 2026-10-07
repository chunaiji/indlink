package robot

import (
	"testing"
	"time"

	"driftbottle/internal/model"
)

func TestTrailingRobotMsgs(t *testing.T) {
	bot := int64(9)
	msgs := []model.Message{ // 时间升序
		{SenderID: 1}, {SenderID: 9}, {SenderID: 1}, {SenderID: 9}, {SenderID: 9},
	}
	if got := trailingRobotMsgs(msgs, bot); got != 2 {
		t.Fatalf("trailing=%d, want 2", got)
	}
	// 末尾是真人 → 0
	msgs2 := []model.Message{{SenderID: 9}, {SenderID: 1}}
	if got := trailingRobotMsgs(msgs2, bot); got != 0 {
		t.Fatalf("trailing=%d, want 0", got)
	}
}

func TestIsOutreachCandidate(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	// 新用户:2小时前注册
	if !isOutreachCandidate(now.Add(-2*time.Hour), now, now, 24, 7) {
		t.Fatal("新用户应命中")
	}
	// 沉默用户:10天未活跃(注册很早)
	if !isOutreachCandidate(now.Add(-100*24*time.Hour), now.Add(-10*24*time.Hour), now, 24, 7) {
		t.Fatal("沉默用户应命中")
	}
	// 老用户且近期活跃 → 不命中
	if isOutreachCandidate(now.Add(-100*24*time.Hour), now.Add(-1*time.Hour), now, 24, 7) {
		t.Fatal("活跃老用户不应命中")
	}
}
