package user

import (
	"strconv"

	"driftbottle/internal/geo"
	"driftbottle/pkg/geodist"
)

// cityRefreshKM 位置移动多远才值得重新反查城市。
//
// 逆地理是按量计费的外呼,而 App 每次启动都会上报一次位置。取 2km:
// 市内通勤不会触发,跨城必然触发——城市这个粒度上它足够糙,也足够准。
const cityRefreshKM = 2.0

// shouldRefreshCity 这次位置上报值不值得花一次逆地理调用。
//
// 纯函数,不碰 DB 与网络——「要不要花钱」这个判断必须能被测试钉住。
func shouldRefreshCity(curCity string, oldLat, oldLng, newLat, newLng float64) bool {
	// 新坐标本身没意义,查了也没用。
	if !geodist.HasFix(newLat, newLng) {
		return false
	}
	// 还没有城市:这是最该查的一次,也是「在 XXX 捞起瓶子」能不能显示的前提。
	if curCity == "" {
		return true
	}
	// 没有旧坐标可比,无从判断是否移动过。
	if !geodist.HasFix(oldLat, oldLng) {
		return true
	}
	return geodist.KM(oldLat, oldLng, newLat, newLng) >= cityRefreshKM
}

// cityAt 反查城市。拿不到返回空串——调用方据此跳过,不覆盖已有值。
//
// 失败一律静默:位置上报本来就是「锦上添花、失败不打扰用户」的链路,
// 没理由让一次逆地理超时把整个资料更新带崩。
func (s *Service) cityAt(tenantID int64, lat, lng float64) string {
	if s.providers == nil {
		return ""
	}
	res, ok := geo.CityFor(
		s.providers, tenantID,
		strconv.FormatFloat(lat, 'f', -1, 64),
		strconv.FormatFloat(lng, 'f', -1, 64),
	)
	if !ok {
		return ""
	}
	return res
}
