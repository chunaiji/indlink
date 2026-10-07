package discover

import (
	"testing"
	"time"

	"driftbottle/internal/model"
)

// 机器人永远在线：发现页卡片不能让机器人看起来「离线很久」，否则用户不会跟它开聊。
func TestRobotCandidateAlwaysOnline(t *testing.T) {
	me := &model.User{UserID: 1}
	stale := time.Now().Add(-48 * time.Hour)
	bot := buildCandidate(me, &model.User{UserID: 2, IsRobot: true, LastActiveAt: stale})
	if !bot.Online || !bot.isRobot {
		t.Fatalf("robot should be online & flagged, got online=%v robot=%v", bot.Online, bot.isRobot)
	}
	human := buildCandidate(me, &model.User{UserID: 3, LastActiveAt: stale})
	if human.Online {
		t.Fatal("stale human must not be online")
	}
}

// 注入只落在前 M 个位置，且不丢任何原卡片。
func TestInjectRobotsWithinTopM(t *testing.T) {
	cards := make([]Candidate, 0, 20)
	for i := int64(1); i <= 20; i++ {
		cards = append(cards, Candidate{UserID: i})
	}
	robots := []Candidate{{UserID: 101, isRobot: true}, {UserID: 102, isRobot: true}}
	// pick 总是取 bound-1：最靠后的合法位置，即 M-1。
	out := injectRobots(cards, robots, 10, func(k int) int { return k - 1 })
	if len(out) != 22 {
		t.Fatalf("want 22 cards, got %d", len(out))
	}
	for i, c := range out {
		if c.isRobot && i >= 10 {
			t.Fatalf("robot at %d, outside top 10", i)
		}
	}
	seen := map[int64]bool{}
	for _, c := range out {
		seen[c.UserID] = true
	}
	if len(seen) != 22 {
		t.Fatal("a card was lost or duplicated")
	}

	// 原列表比 M 短时也能插（可插位置 [0, len]）。
	short := injectRobots([]Candidate{{UserID: 1}}, robots, 10, func(k int) int { return k - 1 })
	if len(short) != 3 {
		t.Fatalf("want 3, got %d", len(short))
	}
}
