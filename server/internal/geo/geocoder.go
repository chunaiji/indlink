package geo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"driftbottle/internal/provider"
)

type Result struct{ Address, City, Place string }

// Geocoder 经纬度 → 地址。lang 只有 Google 用(跟随 Accept-Language)。
type Geocoder interface {
	Regeo(lat, lng, lang string) (Result, error)
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

func getJSON(u string) ([]byte, error) {
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// GeocoderFor 服务商行 → 实现。未知服务商 / 空行返回 false。
func GeocoderFor(row *provider.Resolved) (Geocoder, bool) {
	if row == nil {
		return nil, false
	}
	switch row.Provider {
	case "tencent":
		return newTencent(row.Get("key"), ""), true
	case "google":
		return newGoogle(row.Get("key"), ""), true
	case "amap":
		return newAmap(row.Get("key"), ""), true
	case "baidu":
		return newBaidu(row.Get("ak"), row.Get("sk"), ""), true
	}
	return nil, false
}

// ---------------- 腾讯 ----------------

type tencent struct{ key, base string }

func newTencent(key, base string) Geocoder {
	if base == "" {
		base = "https://apis.map.qq.com"
	}
	return &tencent{key: key, base: base}
}

func (g *tencent) Regeo(lat, lng, _ string) (Result, error) {
	raw, err := getJSON(fmt.Sprintf("%s/ws/geocoder/v1/?location=%s,%s&key=%s", g.base, url.QueryEscape(lat), url.QueryEscape(lng), url.QueryEscape(g.key)))
	if err != nil {
		return Result{}, err
	}
	return parseTencent(raw)
}

func parseTencent(raw []byte) (Result, error) {
	var r struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Result  struct {
			Address            string `json:"address"`
			FormattedAddresses struct {
				Recommend string `json:"recommend"`
			} `json:"formatted_addresses"`
			AddressComponent struct {
				City     string `json:"city"`
				District string `json:"district"`
			} `json:"address_component"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	if r.Status != 0 {
		return Result{}, fmt.Errorf("腾讯位置服务 %d: %s", r.Status, r.Message)
	}
	addr := r.Result.FormattedAddresses.Recommend
	if addr == "" {
		addr = r.Result.Address
	}
	// place 是给界面看的短地名。腾讯用 district(区),取不到退回 city,保证这个字段永远有值可显示。
	place := r.Result.AddressComponent.District
	if place == "" {
		place = r.Result.AddressComponent.City
	}
	return Result{Address: addr, City: r.Result.AddressComponent.City, Place: place}, nil
}

// ---------------- Google ----------------

type google struct{ key, base string }

func newGoogle(key, base string) Geocoder {
	if base == "" {
		base = "https://maps.googleapis.com"
	}
	return &google{key: key, base: base}
}

// Regeo 应答语言跟随请求方的 Accept-Language:印地语用户看到的地址应该是印地语的。
func (g *google) Regeo(lat, lng, lang string) (Result, error) {
	if lang == "" {
		lang = "en"
	}
	raw, err := getJSON(fmt.Sprintf("%s/maps/api/geocode/json?latlng=%s,%s&key=%s&language=%s",
		g.base, url.QueryEscape(lat), url.QueryEscape(lng), url.QueryEscape(g.key), url.QueryEscape(lang)))
	if err != nil {
		return Result{}, err
	}
	return parseGoogle(raw)
}

func parseGoogle(raw []byte) (Result, error) {
	var r struct {
		Status  string `json:"status"`
		Results []struct {
			FormattedAddress  string `json:"formatted_address"`
			AddressComponents []struct {
				LongName string   `json:"long_name"`
				Types    []string `json:"types"`
			} `json:"address_components"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	if r.Status != "OK" || len(r.Results) == 0 {
		return Result{}, errors.New("Google Geocoding " + r.Status)
	}
	out := Result{Address: r.Results[0].FormattedAddress}
	for _, comp := range r.Results[0].AddressComponents {
		for _, t := range comp.Types {
			// locality 是「市」;印度部分地区只有 administrative_area_level_2,兜一层
			if t == "locality" {
				out.City = comp.LongName
			} else if out.City == "" && t == "administrative_area_level_2" {
				out.City = comp.LongName
			}
			// place 是给界面看的短地名("Bandra West"),formatted_address 太长会撑破布局
			if out.Place == "" && (t == "sublocality" || t == "sublocality_level_1" || t == "neighborhood") {
				out.Place = comp.LongName
			}
		}
	}
	if out.Place == "" {
		out.Place = out.City
	}
	return out, nil
}
