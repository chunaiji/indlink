package match

import "testing"

func TestInjectRobots_FillsTopWindow(t *testing.T) {
	cards := make([]UserCard, 20) // 20 个真人
	for i := range cards {
		cards[i] = UserCard{UserID: int64(i + 1), IsRobot: false}
	}
	robots := []UserCard{
		{UserID: 101, IsRobot: true}, {UserID: 102, IsRobot: true},
		{UserID: 103, IsRobot: true}, {UserID: 104, IsRobot: true},
	}
	out := injectRobots(cards, robots, 10, 3, func(k int) int { return 0 }) // 每次插到最前

	cnt := 0
	for i := 0; i < 10; i++ {
		if out[i].IsRobot {
			cnt++
		}
	}
	if cnt != 3 {
		t.Fatalf("前10个机器人数=%d, want 3", cnt)
	}
	if len(out) != 23 {
		t.Fatalf("总长=%d, want 23", len(out))
	}
}

func TestInjectRobots_NoopWhenN0(t *testing.T) {
	cards := []UserCard{{UserID: 1}}
	out := injectRobots(cards, []UserCard{{UserID: 2, IsRobot: true}}, 10, 0, func(k int) int { return 0 })
	if len(out) != 1 {
		t.Fatalf("N=0 应不变, len=%d", len(out))
	}
}
