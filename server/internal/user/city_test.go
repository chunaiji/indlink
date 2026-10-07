package user

import "testing"

// 城市靠逆地理反查,而逆地理是按量计费的外呼。App 每次启动都会上报一次位置,
// 无条件反查 = 每用户每次启动一次调用。这组用例钉住「什么时候才值得查」。
func TestShouldRefreshCity(t *testing.T) {
	// 深圳与东莞之间约 50km;深圳市内两点约 300m。
	const szLat, szLng = 22.5431, 114.0579
	const dgLat, dgLng = 22.7832, 113.9281
	const szNearLat, szNearLng = 22.5458, 114.0579

	cases := []struct {
		name    string
		city    string
		oldLat  float64
		oldLng  float64
		newLat  float64
		newLng  float64
		want    bool
	}{
		{"城市为空必须查", "", szLat, szLng, szLat, szLng, true},
		{"从没记过坐标也要查", "", 0, 0, szLat, szLng, true},
		{"原地没动不查", "深圳市", szLat, szLng, szLat, szLng, false},
		{"市内挪了几百米不查", "深圳市", szLat, szLng, szNearLat, szNearLng, false},
		{"跨城要查", "深圳市", szLat, szLng, dgLat, dgLng, true},
		{"新坐标无效时不查", "深圳市", szLat, szLng, 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldRefreshCity(c.city, c.oldLat, c.oldLng, c.newLat, c.newLng)
			if got != c.want {
				t.Fatalf("shouldRefreshCity(%q, %v,%v -> %v,%v) = %v, want %v",
					c.city, c.oldLat, c.oldLng, c.newLat, c.newLng, got, c.want)
			}
		})
	}
}
