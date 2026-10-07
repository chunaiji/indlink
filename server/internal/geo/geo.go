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
	if h.providers == nil {
		empty(c)
		return
	}
	row, ok := h.providers.Active(tenantID, provider.KindMap)
	if !ok {
		empty(c)
		return
	}
	g, ok := GeocoderFor(row)
	if !ok {
		empty(c)
		return
	}
	lang := "en"
	if al := c.GetHeader("Accept-Language"); al != "" {
		if i := strings.IndexAny(al, ",;"); i > 0 {
			lang = strings.TrimSpace(al[:i])
		} else {
			lang = strings.TrimSpace(al)
		}
	}
	res, err := g.Regeo(lat, lng, lang)
	if err != nil {
		apilog.Record(tenantID, "geo_"+row.Provider, lat+","+lng, -1, err.Error(), false)
		empty(c)
		return
	}
	apilog.Record(tenantID, "geo_"+row.Provider, lat+","+lng, 0, res.Address, true)
	response.OK(c, gin.H{"address": res.Address, "city": res.City, "place": res.Place})
}
