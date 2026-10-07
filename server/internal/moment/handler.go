package moment

import (
	"strconv"

	"driftbottle/internal/common/appdto"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

// isApp App 端要另一套响应形状(见 common/appdto)。小程序走原分支,零改动。
func isApp(c *gin.Context) bool { return middleware.Platform(c) == "app" }

// appMoment FeedItem → App 结构。作者信息 / liked / following 已由 service 批量补好,这里不查库。
func appMoment(it *FeedItem) appdto.Moment {
	return appdto.FromMoment(appdto.MomentInput{
		Moment:    &it.Moment,
		Nickname:  it.Nickname,
		Avatar:    it.Avatar,
		Gender:    it.Gender,
		Liked:     it.Liked,
		Following: it.Following,
	})
}

func appComment(cv *CommentView) appdto.MomentComment {
	return appdto.FromComment(appdto.CommentInput{
		ID:          cv.CommentID,
		UserID:      cv.UserID,
		Nickname:    cv.Nickname,
		Avatar:      cv.Avatar,
		ReplyToNick: cv.ReplyToNick,
		Type:        cv.Type,
		Content:     cv.Content,
		CreatedAt:   cv.CreatedAt,
	})
}

// Register 挂载动态路由。commentLimit 评论限流(防刷,复用回信限流器)。
func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc, commentLimit gin.HandlerFunc) {
	g := api.Group("/moment", auth)
	g.POST("", h.create)
	g.GET("/feed", h.feed)
	g.GET("/mine", h.mine)
	// 某用户的公开动态(用户主页「TA 的动态」预览列表)。
	g.GET("/user/:id", h.userMoments)
	g.POST("/:id/remove", h.remove)
	g.POST("/:id/like", h.like)
	g.GET("/:id/comments", h.comments)
	g.GET("/:id", h.detail)
	g.POST("/:id/comment", commentLimit, h.addComment)
	g.POST("/:id/gift", h.gift)
	g.POST("/comment/:cid/remove", h.removeComment)
}

// detail 单条动态(通知跳转/分享落点)。
func (h *Handler) detail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	item, err := h.svc.Detail(middleware.TenantID(c), middleware.UserID(c), id)
	if err != nil {
		writeBizErr(c, err, "查询失败")
		return
	}
	if isApp(c) {
		response.OK(c, appMoment(item))
		return
	}
	response.OK(c, item)
}

type createReq struct {
	Content string   `json:"content"`
	Visible string   `json:"visible"`
	Images  []string `json:"images"`
	// 位置为可选:用户可以选择不带地点(原型 M2 的「不显示地点」)。
	// Moment 原本一个位置字段都没有,city 也要一起传。
	City      string   `json:"city"`
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
	if len([]rune(req.Content)) > 500 {
		response.Fail(c, errs.CodeBadRequest, "内容不能超过500字")
		return
	}
	m, err := h.svc.Create(middleware.TenantID(c), middleware.UserID(c), CreateInput{
		Content: req.Content, Visible: req.Visible, Images: req.Images,
		City: req.City, Lat: req.Lat, Lng: req.Lng, PlaceName: req.PlaceName,
	})
	if err != nil {
		writeBizErr(c, err, "发布失败")
		return
	}
	if isApp(c) {
		// 刚发的动态:作者是自己、必然未赞。昵称头像客户端自己有,不回填。
		response.OK(c, appdto.FromMoment(appdto.MomentInput{Moment: m}))
		return
	}
	response.OK(c, m)
}

func (h *Handler) feed(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.Feed(middleware.TenantID(c), middleware.UserID(c), c.Query("tab"), page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	if isApp(c) {
		out := make([]appdto.Moment, len(list))
		for i := range list {
			out[i] = appMoment(&list[i])
		}
		response.OK(c, out)
		return
	}
	response.OK(c, list)
}

func (h *Handler) mine(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.Mine(middleware.UserID(c), page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, list)
}

// userMoments 某用户的公开动态(用户主页 C3 的「TA 的动态」)。
func (h *Handler) userMoments(c *gin.Context) {
	uid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.UserMoments(middleware.TenantID(c), middleware.UserID(c), uid, page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	if isApp(c) {
		out := make([]appdto.Moment, len(list))
		for i := range list {
			out[i] = appMoment(&list[i])
		}
		response.OK(c, out)
		return
	}
	response.OK(c, list)
}

func (h *Handler) remove(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.Remove(middleware.UserID(c), id); err != nil {
		writeBizErr(c, err, "删除失败")
		return
	}
	response.OK(c, nil)
}

func (h *Handler) like(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	liked, err := h.svc.Like(middleware.UserID(c), id)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "操作失败")
		return
	}
	response.OK(c, gin.H{"liked": liked})
}

func (h *Handler) comments(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, err := h.svc.Comments(id, page, size)
	if err != nil {
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	if isApp(c) {
		out := make([]appdto.MomentComment, len(list))
		for i := range list {
			out[i] = appComment(&list[i])
		}
		response.OK(c, out)
		return
	}
	response.OK(c, list)
}

type commentReq struct {
	Content string `json:"content" binding:"required"`
	ReplyTo string `json:"reply_to"` // 回复某人 user_id,字符串防精度丢失;空=直接评论
}

func (h *Handler) addComment(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req commentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if len([]rune(req.Content)) > 200 {
		response.Fail(c, errs.CodeBadRequest, "评论不能超过200字")
		return
	}
	replyTo, _ := strconv.ParseInt(req.ReplyTo, 10, 64)
	cv, err := h.svc.AddComment(middleware.TenantID(c), middleware.UserID(c), id, replyTo, req.Content)
	if err != nil {
		writeBizErr(c, err, "评论失败")
		return
	}
	if isApp(c) {
		response.OK(c, appComment(cv))
		return
	}
	response.OK(c, cv)
}

type giftReq struct {
	ItemID int64 `json:"item_id" binding:"required"`
}

// gift 给动态作者送礼(余额不足返回钱包侧业务码,前端引导充值)。
func (h *Handler) gift(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req giftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	cv, err := h.svc.SendGift(middleware.TenantID(c), middleware.UserID(c), id, req.ItemID)
	if err != nil {
		writeBizErr(c, err, "赠送失败")
		return
	}
	if isApp(c) {
		response.OK(c, appComment(cv))
		return
	}
	response.OK(c, cv)
}

func (h *Handler) removeComment(c *gin.Context) {
	cid, _ := strconv.ParseInt(c.Param("cid"), 10, 64)
	if err := h.svc.RemoveComment(middleware.UserID(c), cid); err != nil {
		writeBizErr(c, err, "删除失败")
		return
	}
	response.OK(c, nil)
}

func writeBizErr(c *gin.Context, err error, fallback string) {
	if be, ok := err.(*errs.BizError); ok {
		response.Fail(c, be.Code, be.Msg)
		return
	}
	response.Fail(c, errs.CodeServerError, fallback)
}
