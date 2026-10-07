// Package geo 逆地理编码代理(水印相机地址水印 / 发现页):
// 前端只传经纬度,服务端持 key 调当前生效的地图服务商(后台「服务商 → 地图」页单选),避免 key 泄露与前端域名白名单问题。
package geo

import (
	"strings"

	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"
	"driftbottle/internal/provider"
	"driftbottle/pkg/apilog"

	"github.com/gin-gonic/gin"
)

type Handler struct{ providers *provider.Store }

func NewHandler(ps *provider.Store) *Handler { return &Handler{providers: ps} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.GET("/geo/regeo", auth, h.regeo)
}

func empty(c *gin.Context) { response.OK(c, gin.H{"address": "", "city": "", "place": ""}) }

// regeo 经纬度 → 地址。没有生效的地图服务商 / 调用失败 → 空地址(前端降级显示经纬度)。
func (h *Handler) regeo(c *gin.Context) {
	lat, lng := c.Query("lat"), c.Query("lng")
	if lat == "" || lng == "" {
		empty(c)
		return
	}
	tenantID := middleware.TenantID(c)
	lang := "en"
	if al := c.GetHeader("Accept-Language"); al != "" {
		if i := strings.IndexAny(al, ",;"); i > 0 {
			lang = strings.TrimSpace(al[:i])
		} else {
			lang = strings.TrimSpace(al)
		}
	}
	res, ok := regeoWith(h.providers, tenantID, lat, lng, lang)
	if !ok {
		empty(c)
		return
	}
	response.OK(c, gin.H{"address": res.Address, "city": res.City, "place": res.Place})
}

// CityFor 反查某个坐标所在城市,供 HTTP 之外的调用方(如用户资料更新)复用。
//
// 抽出来是因为「选哪个服务商」这件事只该有一处实现:它牵涉单选域的
// Active() 语义与适配器分派,各写一遍迟早走岔。
func CityFor(ps *provider.Store, tenantID int64, lat, lng string) (string, bool) {
	res, ok := regeoWith(ps, tenantID, lat, lng, "zh")
	if !ok {
		return "", false
	}
	return res.City, res.City != ""
}

// regeoWith 真正干活的那一段:挑服务商 → 建适配器 → 调用 → 记接口日志。
func regeoWith(ps *provider.Store, tenantID int64, lat, lng, lang string) (Result, bool) {
	if ps == nil {
		return Result{}, false
	}
	row, ok := ps.Active(tenantID, provider.KindMap)
	if !ok {
		return Result{}, false
	}
	g, ok := GeocoderFor(row)
	if !ok {
		return Result{}, false
	}
	res, err := g.Regeo(lat, lng, lang)
	if err != nil {
		apilog.Record(tenantID, "geo_"+row.Provider, lat+","+lng, -1, err.Error(), false)
		return Result{}, false
	}
	apilog.Record(tenantID, "geo_"+row.Provider, lat+","+lng, 0, res.Address, true)
	return res, true
}
