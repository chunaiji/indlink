package bottle

import (
	"math"
	"testing"
)

// distanceScore 距离衰减打分。
//
// 三条边界比公式本身重要得多：
//   - 瓶子没有经纬度(存量数据) → 不计分也**不惩罚**,退回 wCity 维度
//   - 浏览者没有经纬度(拒绝定位) → 同样不计分,与 Z6「降级不阻断」一致
//   - 有定位时越近分越高,且不为负
func TestDistanceScore(t *testing.T) {
	const w = 15
	mumbaiLat, mumbaiLng := 19.0760, 72.8777

	t.Run("瓶子无经纬度时不计分也不惩罚", func(t *testing.T) {
		if got := distanceScore(mumbaiLat, mumbaiLng, 0, 0, w); got != 0 {
			t.Errorf("存量瓶子应得 0 分, 实际 %.3f", got)
		}
	})

	t.Run("浏览者无经纬度时不计分", func(t *testing.T) {
		if got := distanceScore(0, 0, mumbaiLat, mumbaiLng, w); got != 0 {
			t.Errorf("未定位的浏览者应得 0 分, 实际 %.3f", got)
		}
	})

	t.Run("同一点拿满分", func(t *testing.T) {
		got := distanceScore(mumbaiLat, mumbaiLng, mumbaiLat, mumbaiLng, w)
		if math.Abs(got-w) > 0.001 {
			t.Errorf("零距离应拿满分 %.0f, 实际 %.3f", float64(w), got)
		}
	})

	t.Run("越近分越高", func(t *testing.T) {
		near := distanceScore(mumbaiLat, mumbaiLng, 19.0596, 72.8295, w) // 约 6km
		far := distanceScore(mumbaiLat, mumbaiLng, 28.6139, 77.2090, w)  // 约 1150km
		if near <= far {
			t.Errorf("近的应得分更高: near=%.3f far=%.3f", near, far)
		}
	})

	t.Run("再远也不给负分", func(t *testing.T) {
		// 对跖点附近
		if got := distanceScore(mumbaiLat, mumbaiLng, -19.0760, -107.1223, w); got < 0 {
			t.Errorf("距离项不应为负, 实际 %.3f", got)
		}
	})
}
