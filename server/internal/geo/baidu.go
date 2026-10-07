package geo

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type baidu struct{ ak, sk, base string }

func newBaidu(ak, sk, base string) Geocoder {
	if base == "" {
		base = "https://api.map.baidu.com"
	}
	return &baidu{ak: ak, sk: sk, base: base}
}

// baiduSN 百度 SN 校验:md5(urlencode(path + "?" + query + sk))。请求用同一个 query 串,顺序一致即可。
func baiduSN(path string, params url.Values, sk string) string {
	query := params.Encode()
	raw := url.QueryEscape(path + "?" + query + sk)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (g *baidu) Regeo(lat, lng, _ string) (Result, error) {
	const path = "/reverse_geocoding/v3/"
	params := url.Values{}
	params.Set("ak", g.ak)
	params.Set("output", "json")
	params.Set("coordtype", "wgs84ll")
	params.Set("location", lat+","+lng)
	query := params.Encode()
	if g.sk != "" {
		query += "&sn=" + baiduSN(path, params, g.sk)
	}
	raw, err := getJSON(g.base + path + "?" + query)
	if err != nil {
		return Result{}, err
	}
	return parseBaidu(raw)
}

func parseBaidu(raw []byte) (Result, error) {
	var r struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Result  struct {
			// 直辖市下百度与高德一样会把 city 给成空数组,用 flexString 吃掉(见 amap.go)
			FormattedAddress flexString `json:"formatted_address"`
			AddressComponent struct {
				Province flexString `json:"province"`
				City     flexString `json:"city"`
				District flexString `json:"district"`
			} `json:"addressComponent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	if r.Status != 0 {
		return Result{}, fmt.Errorf("百度地图 %d: %s", r.Status, r.Message)
	}
	ac := r.Result.AddressComponent
	city := strings.TrimSpace(string(ac.City))
	if city == "" {
		city = string(ac.Province)
	}
	place := string(ac.District)
	if place == "" {
		place = city
	}
	return Result{Address: string(r.Result.FormattedAddress), City: city, Place: place}, nil
}
