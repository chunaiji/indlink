package geo

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type amap struct{ key, base string }

func newAmap(key, base string) Geocoder {
	if base == "" {
		base = "https://restapi.amap.com"
	}
	return &amap{key: key, base: base}
}

// Regeo 高德要 location=经度,纬度(与腾讯 / 百度相反)。
func (g *amap) Regeo(lat, lng, _ string) (Result, error) {
	raw, err := getJSON(fmt.Sprintf("%s/v3/geocode/regeo?key=%s&location=%s,%s&extensions=base",
		g.base, url.QueryEscape(g.key), url.QueryEscape(lng), url.QueryEscape(lat)))
	if err != nil {
		return Result{}, err
	}
	return parseAmap(raw)
}

// flexString 高德在直辖市把 city 给成 [](空数组)而不是字符串。
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*f = flexString(s)
		return nil
	}
	*f = "" // 数组 / null 一律当空
	return nil
}

func parseAmap(raw []byte) (Result, error) {
	var r struct {
		Status    string `json:"status"`
		Info      string `json:"info"`
		Regeocode struct {
			FormattedAddress flexString `json:"formatted_address"`
			AddressComponent struct {
				Province flexString `json:"province"`
				City     flexString `json:"city"`
				District flexString `json:"district"`
			} `json:"addressComponent"`
		} `json:"regeocode"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	if r.Status != "1" {
		return Result{}, fmt.Errorf("高德 %s", r.Info)
	}
	ac := r.Regeocode.AddressComponent
	city := string(ac.City)
	if city == "" {
		city = string(ac.Province)
	}
	place := string(ac.District)
	if place == "" {
		place = city
	}
	return Result{Address: string(r.Regeocode.FormattedAddress), City: city, Place: place}, nil
}
