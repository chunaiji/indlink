package match

import (
	"strconv"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("", auth)
	g.GET("/city/users", h.cityUsers)
	g.GET("/expand/wall", h.expandWall)
}

func (h *Handler) cityUsers(c *gin.Context) {
	city := c.Query("city")
	keyword := c.Query("keyword")
	sort := c.Query("sort")
	gender := int8(atoi(c.Query("gender")))
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 20)
	list, err := h.svc.CityUsers(middleware.TenantID(c), middleware.UserID(c), city, keyword, gender, sort, page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, list)
}

func (h *Handler) expandWall(c *gin.Context) {
	wallType := c.DefaultQuery("type", "active")
	city := c.Query("city")
	gender := int8(atoi(c.Query("gender")))
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 20)
	list, err := h.svc.ExpandWall(middleware.TenantID(c), middleware.UserID(c), wallType, city, gender, page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, list)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}
