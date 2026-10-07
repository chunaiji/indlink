package bottle

import (
	"strconv"

	"driftbottle/internal/common/appdto"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// isApp App 端要另一套响应形状(见 common/appdto)。
//
// 按 JWT 里的 platform 分派而不是另开一组 /app/* 路由:鉴权、限流、租户解析
// 全都已经挂在这批路由上,复制一份只会多一处会漂移的地方。小程序走原分支,零改动。
func isApp(c *gin.Context) bool { return middleware.Platform(c) == "app" }

// Register 挂载漂流瓶路由。create/reply 带限流(防刷瓶)。
func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc, throwLimit, replyLimit gin.HandlerFunc) {
	g := api.Group("/bottle", auth)
	g.POST("/create", throwLimit, h.create)
	g.GET("/scoop", h.scoop)
	g.GET("/scoop-one", h.scoopOne)
	g.GET("/quota", h.quota)
	g.GET("/mine", h.mine)
	// 我捞过的瓶子(?liked=1 只看点过赞的)。数据源 MatchLog,与 feed 已看过去重同源。
	g.GET("/scooped", h.scooped)
	g.GET("/:id/trace", h.trace)
	g.GET("/:id", h.detail)
	g.GET("/:id/replies", h.replies)
	g.POST("/:id/reply", replyLimit, h.reply)
	g.POST("/reply/:rid/unlock", h.unlock)
	// 整瓶解锁(App 的「解锁 N 条回信」按钮):按条累计价格,只扣一笔。
	g.POST(":id/unlock", h.unlockBottle)
	g.POST("/:id/like", h.like)
	g.POST("/:id/skip", h.skip)
}

type createReq struct {
	Content     string   `json:"content"`
	ContentType string   `json:"content_type"`
	MediaURL    string   `json:"media_url"`
	Tags        []string `json:"tags"`
	IsAnonymous bool     `json:"is_anonymous"`
	Scope       string   `json:"scope"`
	City        string   `json:"city"`
	Visibility  string   `json:"visibility"`
	Night       bool     `json:"night"` // 深夜瓶(夜场时段有效)
	// 位置为可选:用户可以选择不带地点(原型 M2 的「不显示地点」)。
	// 用指针区分「没传」与「传了 0」——0 是几内亚湾的真实坐标。
	Lat       *float64 `json:"lat"`
	Lng       *float64 `json:"lng"`
	PlaceName string   `json:"place_name"`
}

func (h *Handler) create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	b, err := h.svc.Create(middleware.TenantID(c), middleware.UserID(c), CreateInput{
		Content: req.Content, ContentType: req.ContentType, MediaURL: req.MediaURL,
		Tags: req.Tags, IsAnonymous: req.IsAnonymous, Scope: req.Scope, City: req.City, Visibility: req.Visibility,
		Night: req.Night,
		Lat:   req.Lat, Lng: req.Lng, PlaceName: req.PlaceName,
	})
	if err != nil {
		writeBizErr(c, err)
		return
	}
	if isApp(c) {
		response.OK(c, appdto.FromBottle(b))
		return
	}
	response.OK(c, b)
}

func (h *Handler) scoop(c *gin.Context) {
	city := c.Query("city")
	var tags []string
	if t := c.Query("tags"); t != "" {
		tags = splitComma(t)
	}
	ft := Filter{Scope: c.Query("scope")}
	if g, _ := strconv.Atoi(c.Query("gender")); g > 0 {
		ft.Gender = int8(g)
	}
	list, err := h.svc.Scoop(middleware.TenantID(c), middleware.UserID(c), city, tags, ft)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "捞瓶失败")
		return
	}
	if isApp(c) {
		response.OK(c, h.appBottles(c, list))
		return
	}
	response.OK(c, list)
}

// appBottles 批量转 App 结构并补上 viewer 的赞/藏状态与距离。
//
// 三次查询封顶(两条 IN + 一次浏览者位置),不是 N+1——
// 浏览者位置在循环**外面**取一次,循环里只做纯计算。
func (h *Handler) appBottles(c *gin.Context, list []model.Bottle) []appdto.Bottle {
	ids := make([]int64, len(list))
	for i := range list {
		ids[i] = list[i].BottleID
	}
	tenantID, userID := middleware.TenantID(c), middleware.UserID(c)
	flags := h.svc.ViewerFlagsFor(tenantID, userID, ids)
	vLat, vLng := h.svc.ViewerFix(tenantID, userID)
	out := make([]appdto.Bottle, len(list))
	for i := range list {
		liked, collected := flags.Of(list[i].BottleID)
		out[i] = appdto.FromBottleWith(&list[i], 0, 0, liked, collected)
		out[i].DistanceKM = appdto.DistanceFrom(vLat, vLng, list[i].Lat, list[i].Lng)
	}
	return out
}

// appBottle 单个瓶子转 App 结构并补距离。
//
// 与 appBottles 的区别只是不批量——单条路径(捞一个、瓶子详情)复用它,
// 免得每处各写一遍算距离的三行。
func (h *Handler) appBottle(c *gin.Context, b *model.Bottle, liked, collected bool) appdto.Bottle {
	tid, uid := middleware.TenantID(c), middleware.UserID(c)
	d := appdto.FromBottleWith(b, 0, 0, liked, collected)
	vLat, vLng := h.svc.ViewerFix(tid, uid)
	d.DistanceKM = appdto.DistanceFrom(vLat, vLng, b.Lat, b.Lng)
	// 轨迹跟着单条路径一起下发:详情页要它渲染「这个瓶子漂过哪些地方」,
	// 捞一个也要它——捞起那一刻就能提示「你在 X 捞起了来自 Y 的瓶子」,
	// 不必为一句提示再打一次 /trace。
	//
	// **只在单条路径做**:列表(appBottles)带上就是 N+1。
	if h.svc.CanSeeTrace(b, uid) {
		if a := h.svc.TraceAggFor(tid, []model.Bottle{*b})[b.BottleID]; a != nil {
			d.ViewCount, d.CityCount, d.Trace = a.ScoopCount, a.CityCount, a.Nodes
		}
	}
	return d
}

// scoopOne 捞一个(计入每日次数 + 去重)。
func (h *Handler) scoopOne(c *gin.Context) {
	city := c.Query("city")
	var tags []string
	if t := c.Query("tags"); t != "" {
		tags = splitComma(t)
	}
	ft := Filter{Scope: c.Query("scope")}
	if g, _ := strconv.Atoi(c.Query("gender")); g > 0 {
		ft.Gender = int8(g)
	}
	b, err := h.svc.ScoopOne(middleware.TenantID(c), middleware.UserID(c), city, tags, ft)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	// b 为 nil 时 data 为 null,前端提示"海面很安静"——空池子不是错误。
	if isApp(c) && b != nil {
		response.OK(c, h.appBottle(c, b, false, false))
		return
	}
	response.OK(c, b)
}

// quota 今日剩余 扔/捞 次数。
func (h *Handler) quota(c *gin.Context) {
	tid := middleware.TenantID(c)
	t, s := h.svc.QuotaStatus(tid, middleware.UserID(c))
	resp := gin.H{"throw_left": t, "scoop_left": s}
	if isApp(c) {
		// App 的「次数用尽」弹窗要显示「今日 N 次已用完」,光有剩余数凑不出这句话。
		resp["throw_total"] = sysconfig.GetInt(tid, sysconfig.KeyQuotaThrowDaily)
		resp["scoop_total"] = sysconfig.GetInt(tid, sysconfig.KeyQuotaScoopDaily)
	}
	response.OK(c, resp)
}

func (h *Handler) mine(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.Mine(middleware.TenantID(c), middleware.UserID(c), page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	if isApp(c) {
		// Mine 已经按页批量聚合好了被捞数/城市数(一条 group 查询,不是 N+1),直接带上。
		ids := make([]int64, len(list))
		for i := range list {
			ids[i] = list[i].BottleID
		}
		flags := h.svc.ViewerFlagsFor(middleware.TenantID(c), middleware.UserID(c), ids)
		out := make([]appdto.Bottle, len(list))
		for i := range list {
			liked, collected := flags.Of(list[i].BottleID)
			out[i] = appdto.FromBottleWith(
				&list[i].Bottle, list[i].ScoopCount, list[i].CityCount, liked, collected)
			if list[i].Trace != nil {
				out[i].Trace = list[i].Trace
			}
		}
		response.OK(c, out)
		return
	}
	response.OK(c, list)
}

// scooped 我捞过的瓶子(App 的「我的瓶子·我捞的」)。?liked=1 只看点过赞的。
func (h *Handler) scooped(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.Scooped(middleware.TenantID(c), middleware.UserID(c),
		c.Query("liked") == "1", page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	if isApp(c) {
		ids := make([]int64, len(list))
		for i := range list {
			ids[i] = list[i].BottleID
		}
		flags := h.svc.ViewerFlagsFor(middleware.TenantID(c), middleware.UserID(c), ids)
		out := make([]appdto.Bottle, len(list))
		for i := range list {
			liked, collected := flags.Of(list[i].BottleID)
			// 捞瓶列表不谈「被捞数/城市数」(那是我扔的瓶子的轨迹口径),传 0。
			out[i] = appdto.FromBottleWith(&list[i], 0, 0, liked, collected)
		}
		response.OK(c, out)
		return
	}
	response.OK(c, list)
}

func (h *Handler) detail(c *gin.Context) {
	id := pathID(c, "id")
	tid, uid := middleware.TenantID(c), middleware.UserID(c)
	b, err := h.svc.Detail(tid, id)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	if isApp(c) {
		flags := h.svc.ViewerFlagsFor(tid, uid, []int64{b.BottleID})
		liked, collected := flags.Of(b.BottleID)
		d := h.appBottle(c, b, liked, collected)
		response.OK(c, d)
		return
	}
	response.OK(c, b)
}

// trace 漂流轨迹(瓶主或捞过的人)。App 端返回按城市聚合的节点列表(与详情里的 trace 同形)。
func (h *Handler) trace(c *gin.Context) {
	id := pathID(c, "id")
	tid, uid := middleware.TenantID(c), middleware.UserID(c)
	t, err := h.svc.Trace(tid, uid, id)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	if isApp(c) {
		var b model.Bottle
		if h.svc.db.Select("bottle_id, user_id, city, created_at").First(&b, "bottle_id = ?", id).Error == nil {
			if a := h.svc.TraceAggFor(tid, []model.Bottle{b})[b.BottleID]; a != nil {
				response.OK(c, gin.H{"list": a.Nodes, "scoop_count": a.ScoopCount, "city_count": a.CityCount})
				return
			}
		}
		response.OK(c, gin.H{"list": []appdto.TraceNode{}, "scoop_count": t.ScoopCount, "city_count": t.CityCount})
		return
	}
	response.OK(c, t)
}

func (h *Handler) replies(c *gin.Context) {
	id := pathID(c, "id")
	list, err := h.svc.Replies(middleware.TenantID(c), middleware.UserID(c), id)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	response.OK(c, list)
}

type replyReq struct {
	Content string `json:"content" binding:"required"`
}

func (h *Handler) reply(c *gin.Context) {
	var req replyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	r, err := h.svc.Reply(middleware.TenantID(c), middleware.UserID(c), pathID(c, "id"), req.Content)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	response.OK(c, r)
}

func (h *Handler) unlock(c *gin.Context) {
	rid, _ := strconv.ParseInt(c.Param("rid"), 10, 64)
	content, err := h.svc.UnlockReply(middleware.TenantID(c), middleware.UserID(c), rid)
	if err != nil {
		writeBizErr(c, err)
		return
	}
	response.OK(c, gin.H{"content": content})
}

func (h *Handler) unlockBottle(c *gin.Context) {
	list, err := h.svc.UnlockBottleReplies(
		middleware.TenantID(c), middleware.UserID(c), pathID(c, "id"))
	if err != nil {
		writeBizErr(c, err)
		return
	}
	// 与 GET :id/replies 同形(裸数组),客户端解析一套代码。
	response.OK(c, list)
}

func (h *Handler) like(c *gin.Context) {
	_ = h.svc.Like(middleware.TenantID(c), middleware.UserID(c), pathID(c, "id"))
	response.OK(c, nil)
}

func (h *Handler) skip(c *gin.Context) {
	_ = h.svc.Skip(middleware.TenantID(c), middleware.UserID(c), pathID(c, "id"))
	response.OK(c, nil)
}

// ---- helpers ----

func pathID(c *gin.Context, name string) int64 {
	v, _ := strconv.ParseInt(c.Param(name), 10, 64)
	return v
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func writeBizErr(c *gin.Context, err error) {
	if be, ok := err.(*errs.BizError); ok {
		response.Fail(c, be.Code, be.Msg)
		return
	}
	response.Fail(c, errs.CodeServerError, "服务异常")
}
