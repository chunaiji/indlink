// Package rank 社交展示读接口:用户资料卡(含礼物墙) + 魅力周榜。
// 开关走 sysconfig(user_card_enabled / charm_rank_enabled),默认全关,工具形态不露出。
package rank

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/common/appdto"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

// GiftRow 礼物墙一格:同种礼物聚合数量。
type GiftRow struct {
	ItemID int64  `json:"item_id"`
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Count  int64  `json:"count"`
}

// CardResp 用户资料卡(公开字段,严禁下发 openid/手机等隐私)。
type CardResp struct {
	// id 与 user_id 并存:App 端一律读 id,小程序读 user_id。
	// 只给 user_id 会让 App 拿到空 id,页面上的「打招呼/送礼」全部带着空 target_id 发出去。
	ID         string    `json:"id"`
	UserID     int64     `json:"user_id,string"`
	Nickname   string    `json:"nickname"`
	Avatar     string    `json:"avatar"`
	Gender     int8      `json:"gender"`
	Age        int       `json:"age"`
	City       string    `json:"city"`
	Bio        string    `json:"bio"`
	IsVerified bool      `json:"is_verified"`
	Charm      int64     `json:"charm"`
	FansCount  int64     `json:"fans_count"`
	CreatedAt  time.Time `json:"created_at"`
	Gifts      []GiftRow `json:"gifts"`

	// 以下为 App 用户主页(C3)所需的丰富字段,小程序忽略即可。
	// language/interests 逗号分隔串,与 appdto.User 同形状,客户端 stringList 自解。
	Language      string   `json:"language"`
	Interests     string   `json:"interests"`
	DistanceKM    *float64 `json:"distance_km,omitempty"` // 双方都有定位才有值(已粗化)
	BottleCount   int64    `json:"bottle_count"`
	MomentCount   int64    `json:"moment_count"`
	RelationStage string   `json:"relation_stage"`         // stranger/known/familiar
	CommonPoint   string   `json:"common_point,omitempty"` // 共同答题项的选项文案
}

// Card 资料卡:公开资料 + 粉丝数 + 礼物墙(最多 8 种,按数量倒序)。
//
// viewerID 用于算「距离 / 关系阶段 / 共同答题项」;看自己主页时(viewerID==targetID)跳过这几项。
func (s *Service) Card(tenantID, viewerID, targetID int64) (*CardResp, error) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyUserCardEnabled) {
		return nil, errs.New(errs.CodeBadRequest, "功能未开启")
	}
	var u model.User
	if err := s.db.First(&u, "tenant_id = ? AND user_id = ? AND status = ?", tenantID, targetID, "active").Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "用户不存在")
	}
	resp := &CardResp{
		ID:     strconv.FormatInt(u.UserID, 10),
		UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar,
		Gender: u.Gender, Age: u.Age, City: u.City, Bio: u.Bio,
		IsVerified: u.IsVerified, Charm: u.Charm, CreatedAt: u.CreatedAt,
		Language: u.Language, Interests: u.Interests,
		RelationStage: "stranger",
		Gifts:         []GiftRow{},
	}
	s.db.Model(&model.Relation{}).Where("type = ? AND user_b = ?", "like", targetID).Count(&resp.FansCount)
	s.db.Model(&model.Bottle{}).Where("tenant_id = ? AND user_id = ?", tenantID, targetID).Count(&resp.BottleCount)
	s.db.Model(&model.Moment{}).Where("tenant_id = ? AND user_id = ? AND visible = ?", tenantID, targetID, "public").Count(&resp.MomentCount)
	s.db.Raw(`SELECT o.item_id, i.name, i.icon, COUNT(*) AS count
		FROM item_orders o JOIN items i ON i.item_id = o.item_id
		WHERE o.tenant_id = ? AND o.target_id = ? AND i.type = 'gift'
		GROUP BY o.item_id, i.name, i.icon ORDER BY count DESC LIMIT 8`, tenantID, targetID).
		Scan(&resp.Gifts)

	// viewer 相关:距离(需双方定位,已粗化)、关系阶段、共同答题项。看自己主页时跳过。
	if viewerID > 0 && viewerID != targetID {
		var me model.User
		if s.db.Select("lat, lng, quiz_answers").First(&me, "user_id = ?", viewerID).Error == nil {
			resp.DistanceKM = appdto.DistanceFrom(me.Lat, me.Lng, u.Lat, u.Lng)
			resp.CommonPoint = commonQuizPoint(tenantID, me.QuizAnswers, u.QuizAnswers)
		}
		if st := s.relationStage(tenantID, viewerID, targetID); st != "" {
			resp.RelationStage = st
		}
	}
	return resp, nil
}

// relationStage 我与对方的关系阶段(stranger/known/familiar)。取双向关系里的一条,无则陌生。
func (s *Service) relationStage(tenantID, viewerID, targetID int64) string {
	var rel model.Relation
	err := s.db.Where("tenant_id = ? AND ((user_a = ? AND user_b = ?) OR (user_a = ? AND user_b = ?))",
		tenantID, viewerID, targetID, targetID, viewerID).First(&rel).Error
	if err != nil || rel.Stage == "" {
		return "stranger"
	}
	return rel.Stage
}

// commonQuizPoint 我与对方选了相同答案的第一题的选项文案(供 C3「共同点」)。
// 题库与答案格式同 discover 答题匹配:题库 "qkey|问题|opt1,opt2",答案 "qkey:idx,..."。
func commonQuizPoint(tenantID int64, mine, theirs string) string {
	if mine == "" || theirs == "" {
		return ""
	}
	a, b := parseQuizAnswers(mine), parseQuizAnswers(theirs)
	for _, line := range strings.Split(sysconfig.GetString(tenantID, sysconfig.KeyAppDiscoverQuiz), "\n") {
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) < 3 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		mi, ok1 := a[key]
		ti, ok2 := b[key]
		if !ok1 || !ok2 || mi != ti {
			continue
		}
		opts := strings.Split(parts[2], ",")
		if idx, err := strconv.Atoi(mi); err == nil && idx >= 0 && idx < len(opts) {
			return strings.TrimSpace(opts[idx])
		}
	}
	return ""
}

// parseQuizAnswers 解析 "qkey:idx,qkey:idx" → map[qkey]idx。
func parseQuizAnswers(s string) map[string]string {
	out := map[string]string{}
	for _, p := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) == 2 && kv[0] != "" && kv[1] != "" {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

// RankRow 周榜一行。
type RankRow struct {
	UserID     int64  `json:"user_id,string"`
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar"`
	Gender     int8   `json:"gender"`
	IsVerified bool   `json:"is_verified"`
	WeekCharm  int64  `json:"week_charm"` // 本周收礼金币数
}

func weekStart(now time.Time) time.Time {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	offset := (int(day.Weekday()) + 6) % 7 // 周一为一周起点
	return day.AddDate(0, 0, -offset)
}

func weekCacheKey(tenantID int64) string { return fmt.Sprintf("rank:charm:week:%d", tenantID) }

// InvalidateWeekCache 送礼成功后调用,让周榜即时反映新礼物(否则要等缓存过期)。
func InvalidateWeekCache(tenantID int64) {
	cache.RDB.Del(context.Background(), weekCacheKey(tenantID))
}

// CharmWeek 魅力周榜 Top20(本周一零点起收礼金币聚合;Redis 缓存 5 分钟)。
func (s *Service) CharmWeek(tenantID int64) ([]RankRow, error) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyCharmRankEnabled) {
		return nil, errs.New(errs.CodeBadRequest, "功能未开启")
	}
	ctx := context.Background()
	key := weekCacheKey(tenantID)
	if raw, err := cache.RDB.Get(ctx, key).Result(); err == nil && raw != "" {
		var cached []RankRow
		if json.Unmarshal([]byte(raw), &cached) == nil {
			return cached, nil
		}
	}
	type agg struct {
		TargetID  int64
		WeekCharm int64
	}
	var rows []agg
	s.db.Raw(`SELECT o.target_id, SUM(o.coins) AS week_charm
		FROM item_orders o JOIN items i ON i.item_id = o.item_id
		WHERE o.tenant_id = ? AND o.target_id > 0 AND i.type = 'gift' AND o.created_at >= ?
		GROUP BY o.target_id ORDER BY week_charm DESC LIMIT 20`, tenantID, weekStart(time.Now())).
		Scan(&rows)
	list := make([]RankRow, 0, len(rows))
	if len(rows) > 0 {
		ids := make([]int64, len(rows))
		charm := make(map[int64]int64, len(rows))
		for i, r := range rows {
			ids[i] = r.TargetID
			charm[r.TargetID] = r.WeekCharm
		}
		var users []model.User
		s.db.Select("user_id, nickname, avatar, gender, is_verified").
			Where("user_id IN ? AND status = ?", ids, "active").Find(&users)
		byID := make(map[int64]model.User, len(users))
		for _, u := range users {
			byID[u.UserID] = u
		}
		for _, r := range rows { // 保持聚合排序
			u, ok := byID[r.TargetID]
			if !ok {
				continue
			}
			list = append(list, RankRow{
				UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar,
				Gender: u.Gender, IsVerified: u.IsVerified, WeekCharm: r.WeekCharm,
			})
		}
	}
	// 空榜只缓存 30 秒(避免"送礼前打开过榜单,空结果占住缓存"),有数据缓存 2 分钟
	ttl := 2 * time.Minute
	if len(list) == 0 {
		ttl = 30 * time.Second
	}
	if raw, err := json.Marshal(list); err == nil {
		cache.RDB.Set(ctx, key, raw, ttl)
	}
	return list, nil
}

// ---- HTTP ----

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	// 注:用 /user/card/:id 而非 /user/:id/card,避开与 /user/profile 等静态路由的参数段冲突
	api.GET("/user/card/:id", auth, h.card)
	api.GET("/rank/charm", auth, h.charm)
}

func (h *Handler) card(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	card, err := h.svc.Card(middleware.TenantID(c), middleware.UserID(c), id)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, card)
}

func (h *Handler) charm(c *gin.Context) {
	list, err := h.svc.CharmWeek(middleware.TenantID(c))
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, list)
}
