// Package geodist 地理距离与分桶。
//
// 抽成公共包是因为 bottle(捞瓶打分) 与 discover(发现页) 都要算距离，
// 而 discover 里原本有一份私有的 haversineKM。两份实现意味着两处要各自维护
// 「零值怎么处理」这类边界，迟早漂移。
package geodist

import (
	"fmt"
	"math"
)

// KM 两点间球面距离(公里)，haversine。
func KM(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

// HasFix 是否有真实定位。
//
// ⚠️ 这是距离链路上最容易出事的一处。Bottle / Moment 加经纬度列之后，
// **存量数据全是 0**，而 (0.0, 0.0) 是几内亚湾里一个真实坐标点。
// 不判空的话，所有老瓶子会被算成在西非，距离排序整个失真。
// 凡是要用经纬度的地方，先过这一关。
func HasFix(lat, lng float64) bool {
	return lat != 0 || lng != 0
}

// CoarseKM 距离粗化，用于对外展示。
//
// 不要把精确距离下发给别人：距离本身是标量看着无害，但攻击者在不同位置
// 多次采样同一个目标，三点即可解出真实坐标。这是 Tinder、Grindr
// 都真实出过事的攻击面，粗化是成本最低的缓解。
//
// 分档：1km 内一律 0.5；10km 内取 0.5 的整数倍；100km 内取整；再远取 10 的整数倍。
// 近距离保留一点分辨率（用户确实关心「500 米」和「8 公里」的差别），
// 远距离没人在乎个位数。
func CoarseKM(d float64) float64 {
	switch {
	case d < 1:
		return 0.5
	case d < 10:
		return math.Round(d*2) / 2
	case d < 100:
		return math.Round(d)
	default:
		return math.Round(d/10) * 10
	}
}

// noFixBucket 无定位时的固定桶名。
//
// 必须固定：如果每次返回不同的值，feed 缓存键每次都变，
// 没开定位的用户就永远命中不了缓存，每次捞瓶都触发一次全量重建。
const noFixBucket = "nofix"

// Bucket 经纬度 → 缓存分桶键，粒度约 40km × 40km。
//
// 用途是给捞瓶 feed 的缓存键加一个位置维度：用户跨城之后立刻换桶、
// 立刻重建队列，而市内移动不换桶、不浪费重建。
//
// 取 0.35° 而不是更细的粒度，是因为 feed 重建是一次带打分的全表扫描，
// 桶太细会让重建过于频繁。0.35° 纬度约 39km；经度方向随纬度收缩，
// 在印度(北纬 20° 上下)约 36km，量级合适。
func Bucket(lat, lng float64) string {
	if !HasFix(lat, lng) {
		return noFixBucket
	}
	const size = 0.35
	return fmt.Sprintf("%d_%d", int(math.Floor(lat/size)), int(math.Floor(lng/size)))
}
