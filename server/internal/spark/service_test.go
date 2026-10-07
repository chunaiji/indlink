// server/internal/spark/service_test.go
package spark

import "testing"

func cands(ids ...int64) []Candidate {
	out := make([]Candidate, 0, len(ids))
	for _, id := range ids {
		out = append(out, Candidate{UserID: id, Nickname: "u"})
	}
	return out
}

func idsOf(cs []Candidate) []int64 {
	out := make([]int64, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.UserID)
	}
	return out
}

// 候选过滤的每一条规则各来一例:自己、已拉黑、当日已配过、配额已满。
func TestEligiblePeers(t *testing.T) {
	online := cands(1, 2, 3, 4, 5)
	got := eligiblePeers(
		1,
		online,
		func(peer int64) bool { return peer == 2 }, // 2 当日已配过
		func(peer int64) bool { return peer == 3 }, // 3 已拉黑
		func(peer int64) bool { return peer != 4 }, // 4 配额已满(capLeft=false)
	)
	if len(got) != 1 || got[0].UserID != 5 {
		t.Fatalf("只应剩 5, got %v", idsOf(got))
	}
	// 一个都不剩时返回空而不是 nil 解引用
	if got := eligiblePeers(1, cands(1), func(int64) bool { return false },
		func(int64) bool { return false }, func(int64) bool { return true }); len(got) != 0 {
		t.Fatalf("只有自己时应为空, got %v", idsOf(got))
	}
}

// 选中的候选 ClaimDaily 失败(额度被另一个 tick 抢走)时要换下一个,不能整轮放弃——
// 否则在线人少的时候,一个额度满了的人会把别人的匹配机会一起堵掉。
func TestMatchRealFallsThroughWhenClaimFails(t *testing.T) {
	var claimed []int64
	s := New(Deps{ClaimDaily: func(_ int64, userID int64, _ int) bool {
		claimed = append(claimed, userID)
		return userID == 9 // 只有 9 能占到额度
	}})
	// roll 恒 0 = 总是先挑列表里的第一个
	got, ok := s.matchReal(1, 100, cands(7, 8, 9), 3, func(int) int { return 0 })
	if !ok || got.UserID != 9 {
		t.Fatalf("应最终选中 9, got %+v ok=%v", got, ok)
	}
	if len(claimed) != 3 {
		t.Fatalf("应依次尝试 7、8、9, got %v", claimed)
	}
}

func TestMatchRealReturnsFalseWhenNobodyClaims(t *testing.T) {
	s := New(Deps{ClaimDaily: func(int64, int64, int) bool { return false }})
	if _, ok := s.matchReal(1, 100, cands(7, 8), 3, func(int) int { return 0 }); ok {
		t.Fatal("全都占不到额度时应返回 false,由调用方退回机器人")
	}
}

// 同一分钟里 A 扫到 B、B 也扫到 A:第二次必须被 pairedToday 挡掉,否则两人互弹两遍。
// 这条盯的是 eligiblePeers 真的把 pairedToday 用在了过滤上(而不是只过滤了别的)。
func TestEligiblePeersHonoursPairedToday(t *testing.T) {
	// 模拟:A(1) 刚和 B(2) 配过,记在同一个与顺序无关的键上
	done := map[string]bool{pairKey(7, 1, 2): true}
	pairedFor := func(self int64) func(int64) bool {
		return func(peer int64) bool { return done[pairKey(7, self, peer)] }
	}
	noBlock := func(int64) bool { return false }
	always := func(int64) bool { return true }

	if got := eligiblePeers(1, cands(1, 2), pairedFor(1), noBlock, always); len(got) != 0 {
		t.Fatalf("A 看 B 应已被排除, got %v", idsOf(got))
	}
	if got := eligiblePeers(2, cands(1, 2), pairedFor(2), noBlock, always); len(got) != 0 {
		t.Fatalf("反方向 B 看 A 同样要被排除(pairKey 与顺序无关), got %v", idsOf(got))
	}
}
