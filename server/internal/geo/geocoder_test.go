package geo

import (
	"net/url"
	"testing"
)

func TestParseTencent(t *testing.T) {
	raw := `{"status":0,"result":{"address":"北京市东城区东长安街","formatted_addresses":{"recommend":"天安门"},"address_component":{"city":"北京市","district":"东城区"}}}`
	r, err := parseTencent([]byte(raw))
	if err != nil || r.Address != "天安门" || r.City != "北京市" || r.Place != "东城区" {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := parseTencent([]byte(`{"status":110,"message":"key error"}`)); err == nil {
		t.Fatal("非 0 status 应报错")
	}
}

func TestParseGoogle(t *testing.T) {
	raw := `{"status":"OK","results":[{"formatted_address":"Bandra West, Mumbai","address_components":[{"long_name":"Bandra West","types":["sublocality_level_1","sublocality"]},{"long_name":"Mumbai","types":["locality"]}]}]}`
	r, err := parseGoogle([]byte(raw))
	if err != nil || r.City != "Mumbai" || r.Place != "Bandra West" {
		t.Fatalf("%+v %v", r, err)
	}
}

// 高德:直辖市的 city 是空数组 [] 而不是字符串,解析不能炸,退回 province。
func TestParseAmapMunicipality(t *testing.T) {
	raw := `{"status":"1","regeocode":{"formatted_address":"北京市东城区东华门街道天安门","addressComponent":{"province":"北京市","city":[],"district":"东城区"}}}`
	r, err := parseAmap([]byte(raw))
	if err != nil || r.City != "北京市" || r.Place != "东城区" || r.Address == "" {
		t.Fatalf("%+v %v", r, err)
	}
	raw2 := `{"status":"1","regeocode":{"formatted_address":"广东省深圳市南山区","addressComponent":{"province":"广东省","city":"深圳市","district":"南山区"}}}`
	r, _ = parseAmap([]byte(raw2))
	if r.City != "深圳市" {
		t.Fatalf("普通城市 city 应为字符串: %+v", r)
	}
	if _, err := parseAmap([]byte(`{"status":"0","info":"INVALID_USER_KEY"}`)); err == nil {
		t.Fatal("status 0 应报错")
	}
}

func TestParseBaidu(t *testing.T) {
	raw := `{"status":0,"result":{"formatted_address":"北京市东城区东长安街","addressComponent":{"city":"北京市","district":"东城区","province":"北京市"}}}`
	r, err := parseBaidu([]byte(raw))
	if err != nil || r.City != "北京市" || r.Place != "东城区" {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := parseBaidu([]byte(`{"status":240,"message":"APP 服务被禁用"}`)); err == nil {
		t.Fatal("非 0 status 应报错")
	}
}

// 直辖市:百度与高德一样会把 city 给成空数组,解析不能炸掉整条地址。
func TestParseBaiduMunicipality(t *testing.T) {
	raw := `{"status":0,"result":{"formatted_address":"北京市东城区东长安街","addressComponent":{"province":"北京市","city":[],"district":"东城区"}}}`
	r, err := parseBaidu([]byte(raw))
	if err != nil {
		t.Fatalf("city 为数组时不该报错: %v", err)
	}
	if r.Address == "" {
		t.Fatal("地址不能丢")
	}
	if r.City != "北京市" || r.Place != "东城区" {
		t.Fatalf("%+v", r)
	}
}

// 百度 SN:md5(urlencode(path?query + sk))。
func TestBaiduSN(t *testing.T) {
	params := url.Values{}
	params.Set("address", "百度大厦")
	params.Set("output", "json")
	params.Set("ak", "yourak")
	got := baiduSN("/geocoder/v2/", params, "yoursk")
	if len(got) != 32 {
		t.Fatalf("sn 应为 32 位 md5, got %q", got)
	}
	if got2 := baiduSN("/geocoder/v2/", params, "othersk"); got2 == got {
		t.Fatal("sk 不同 sn 应不同")
	}
}

func TestGeocoderForUnknownProvider(t *testing.T) {
	if _, ok := GeocoderFor(nil); ok {
		t.Fatal("nil 行没有 geocoder")
	}
}
