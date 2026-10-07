package geodist

import (
	"math"
	"testing"
)

// KM 用 haversine 算球面距离。取两个已知城市对照，容差 2%。
func TestKM(t *testing.T) {
	cases := []struct {
		name                   string
		lat1, lng1, lat2, lng2 float64
		wantKM                 float64
	}{
		// 孟买 ↔ 德里，公认约 1150 km
		{"孟买-德里", 19.0760, 72.8777, 28.6139, 77.2090, 1150},
		// 孟买市内：Bandra ↔ Colaba，约 17 km
		{"孟买市内", 19.0596, 72.8295, 18.9067, 72.8147, 17},
		{"同一点为 0", 19.0760, 72.8777, 19.0760, 72.8777, 0},
	}
	for _, c := range cases {
		got := KM(c.lat1, c.lng1, c.lat2, c.lng2)
		if c.wantKM == 0 {
			if got != 0 {
				t.Errorf("%s: 同一点应为 0, 实际 %.3f", c.name, got)
			}
			continue
		}
		if math.Abs(got-c.wantKM)/c.wantKM > 0.02 {
			t.Errorf("%s: 期望约 %.0f km, 实际 %.1f km", c.name, c.wantKM, got)
		}
	}
}

// HasFix 判断有没有真实定位。
//
// 这是整个距离链路最关键的一个判断：Bottle 加了经纬度列之后，
// **存量瓶子全是 0**，而 (0,0) 是几内亚湾里一个真实存在的坐标。
// 不判空就会把所有老瓶子算成在西非，距离排序全乱。
func TestHasFix(t *testing.T) {
	cases := []struct {
		name     string
		lat, lng float64
		want     bool
	}{
		{"存量数据的零值", 0, 0, false},
		{"孟买", 19.0760, 72.8777, true},
		{"只有纬度也算有定位", 19.0760, 0, true},
		{"只有经度也算有定位", 0, 72.8777, true},
		{"南半球负值", -33.8688, 151.2093, true},
	}
	for _, c := range cases {
		if got := HasFix(c.lat, c.lng); got != c.want {
			t.Errorf("%s: 期望 %v, 实际 %v", c.name, c.want, got)
		}
	}
}

// CoarseKM 对外展示用的距离粗化。
//
// 关键性质不是「分档对不对」，而是**同一档里的不同真实距离必须粗化成同一个值**——
// 这才是抵抗多点采样三角定位的那一下。逐档各取两个不同输入验证。
func TestCoarseKM(t *testing.T) {
	cases := []struct {
		name string
		in   []float64 // 这些不同的真实距离
		want float64   // 必须都粗化成同一个值
	}{
		{"1km 内一律 0.5", []float64{0.05, 0.3, 0.87, 0.999}, 0.5},
		{"10km 内取 0.5 的倍数", []float64{5.76, 6.0, 6.24}, 6.0},
		{"100km 内取整", []float64{42.4, 42.49}, 42},
		{"百公里以上取 10 的倍数", []float64{1148, 1152}, 1150},
	}
	for _, c := range cases {
		for _, in := range c.in {
			if got := CoarseKM(in); got != c.want {
				t.Errorf("%s: CoarseKM(%.3f) 期望 %.1f, 实际 %.1f", c.name, in, c.want, got)
			}
		}
	}
	// 粗化必须是稳定的:同一输入永远同一输出,否则多次请求本身就泄露了信息
	if CoarseKM(7.3) != CoarseKM(7.3) {
		t.Error("粗化必须稳定")
	}
	// 不能把远的粗化得比近的还小
	if CoarseKM(5) > CoarseKM(50) {
		t.Error("粗化不能颠倒远近")
	}
}

// Bucket 把经纬度粗化成缓存分桶键（约 40km 量级）。
// 市内移动不换桶、跨城换桶，这是它唯一要满足的性质。
func TestBucket(t *testing.T) {
	// 孟买市内两点（相距约 17km）应同桶
	a := Bucket(19.0596, 72.8295)
	b := Bucket(18.9067, 72.8147)
	if a != b {
		t.Errorf("市内两点应同桶: %q vs %q", a, b)
	}
	// 孟买 vs 德里（约 1150km）必须不同桶
	if c := Bucket(28.6139, 77.2090); c == a {
		t.Errorf("孟买与德里不应同桶, 都是 %q", c)
	}
	// 无定位时给一个稳定的固定桶，不能每次都不同——否则 feed 永远命中不了缓存
	if Bucket(0, 0) != Bucket(0, 0) {
		t.Error("无定位时分桶必须稳定")
	}
	if Bucket(0, 0) == a {
		t.Error("无定位的桶不能和有定位的桶相同")
	}
}
