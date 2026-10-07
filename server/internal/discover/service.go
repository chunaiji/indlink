package discover

import (
	"errors"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/geodist"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// 发现页「按语言 / 兴趣 / 地理找人」。
//
// ⚠️ 与现有 match 模块语义不同：match 做的是「瓶子推荐打分 + MatchLog 行为记录」，
// 候选集是瓶子池；这里的候选集是**用户池**，相似度维度也不一样，所以独立成包。
//
// 去重用 Relation 而不是 MatchLog：MatchLog 的主键维度是 BottleID，
// 拿它存「被滑过的人」语义不对。Relation 本来就是人对人的关系表，加一个
// type=skip 即可，like/skip 两种行为在同一张表里查一次就够。

// RelationSkip 左滑记录的关系类型。
const RelationSkip = "skip"

// 打分权重。建议后续随捞瓶 feed 一起进 sysconfig 做成可调。
const (
	wLanguage = 30 // 每命中一门共同语言
	wInterest = 40 // 兴趣 Jaccard 满分
	wQuiz     = 25 // 答题匹配:每答对一题共同答案(替代语言/兴趣维度)
	wDistance = 25 // 距离衰减满分
	wActive   = 20 // 活跃度满分
	pRobot    = 15 // 机器人惩罚
)

// 候选集上限：SQL 先粗筛这么多条，再在内存里精排取 limit。
// 直接在 SQL 里算距离排序会全表扫，用户量上来必崩。
const candidatePool = 200

type Service struct {
	db     *gorm.DB
	wallet *wallet.Service
}

func New(db *gorm.DB, w *wallet.Service) *Service { return &Service{db: db, wallet: w} }

// Filter 发现页筛选条件，对应原型 C2。
type Filter struct {
	Languages []string
	Interests []string
	MaxKM     *int // nil = 不限距离
	Gender    *int8
	MinAge    int
	MaxAge    int
	Tab       string // recommend/nearby/new
	Limit     int
}

// Candidate 下发给客户端的候选人。
type Candidate struct {
	// ⚠️ 必须同时下发 id 与 user_id。
	//
	// App 端一律用 `id`（见 UserBrief.fromJson）；只给 user_id 的话客户端拿到空串，
	// 后果不是"少显示一个字段"而是整条链路塌掉：
	// 主页路由变成 /discover/user/ 匹配不上（表现为「没有页面」），
	// 打招呼的 target_id 为空被后端拒（表现为「参数错误」）。
	// 小程序读的是 user_id，所以两个都保留。
	ID         string    `json:"id"`
	UserID     int64     `json:"user_id,string"`
	Nickname   string    `json:"nickname"`
	Avatar     string    `json:"avatar"`
	Gender     int8      `json:"gender"`
	Age        int       `json:"age"`
	City       string    `json:"city"`
	Bio        string    `json:"bio"`
	Language   string    `json:"language"`
	Interests  string    `json:"interests"`
	Charm      int64     `json:"charm"`
	DistanceKM *float64  `json:"distance_km,omitempty"`
	Online     bool      `json:"online"`
	LastActive time.Time `json:"last_active_at"`

	score int
	// isRobot 仅供服务端做「同城前 M 保 N 个机器人」注入,不下发——
	// 身份防泄露是硬要求,客户端不该知道谁是机器人。
	isRobot bool
}

// Users 拉候选人列表。
//
// 客户端一次拉 15~20 人缓存在本地，剩 5 张时静默续拉——不可一张一请求，
// 滑动是连续手势，等接口会直接卡住手感。
//
// 排好序截断后再做机器人注入（后台「机器人」分组的 city_robot_top_m / city_robot_top_n）：
// 前 M 张卡里保证至少 N 个机器人，不够的从同城机器人池补、同城不够跨城补。
// 打分里机器人有扣分，纯靠排序前几张几乎不会是机器人，新城市冷启动时
// 用户划到的全是不回话的真人——注入就是为了兜这个。
func (s *Service) Users(tenantID, userID int64, f Filter) ([]Candidate, error) {
	me, err := s.loadMe(userID)
	if err != nil {
		return nil, err
	}
	out, err := s.ranked(tenantID, userID, me, f)
	if err != nil {
		return nil, err
	}
	limit := f.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return s.injectCityRobots(tenantID, userID, me, out), nil
}

// ranked 候选池 → 过滤 → 打分 → 排序，不截断、不注入。
func (s *Service) ranked(tenantID, userID int64, me *model.User, f Filter) ([]Candidate, error) {
	rows, err := s.candidates(tenantID, userID, me, f)
	if err != nil {
		return nil, err
	}

	myLangs := splitCSV(me.Language)
	myInterests := splitCSV(me.Interests)
	maxKM := s.maxKM(tenantID, f)
	quizMode := s.QuizEnabled(tenantID)
	myAnswers := parseAnswers(me.QuizAnswers)

	out := make([]Candidate, 0, len(rows))
	for i := range rows {
		u := &rows[i]
		c := buildCandidate(me, u)

		// 距离超出上限直接丢弃（算得出距离才谈得上超限）。
		if c.DistanceKM != nil && maxKM > 0 && *c.DistanceKM > float64(maxKM) {
			continue
		}

		quizBonus := 0
		if quizMode {
			quizBonus = quizMatch(myAnswers, u.QuizAnswers) * wQuiz
		}
		c.score = score(myLangs, myInterests, c.DistanceKM, maxKM, u, quizMode, quizBonus)
		out = append(out, c)
	}

	// 内存精排：分数高的在前，同分按最近活跃。
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].LastActive.After(out[j].LastActive)
	})
	return out, nil
}

// injectCityRobots 按 city_robot_top_m / city_robot_top_n 往前 M 张里补机器人。
// N=0 关闭。补进来的机器人要过和真人一样的硬过滤（黑名单、已滑过），否则刚左滑掉的
// 机器人下一刷又冒出来。
func (s *Service) injectCityRobots(tenantID, userID int64, me *model.User, out []Candidate) []Candidate {
	n := sysconfig.GetInt(tenantID, sysconfig.KeyCityRobotTopN)
	if n <= 0 {
		return out
	}
	m := sysconfig.GetInt(tenantID, sysconfig.KeyCityRobotTopM)
	if m <= 0 {
		m = 10
	}
	win := m
	if len(out) < win {
		win = len(out)
	}
	have := 0
	for i := 0; i < win; i++ {
		if out[i].isRobot {
			have++
		}
	}
	need := n - have
	if need <= 0 {
		return out
	}
	exclude := make(map[int64]bool, len(out))
	for _, c := range out {
		exclude[c.UserID] = true
	}
	robots := s.robotPool(tenantID, userID, me, exclude, need)
	return injectRobots(out, robots, m, rand.Intn)
}

// injectRobots 把 robots 逐个随机插到 cards 的前 m 个位置里。pick(k) 返回 [0,k)，便于测试。
// 后插的会把先插的往后挤，所以第 k 个的位置上限收紧 (len-1-k)，保证全部落在前 m。
func injectRobots(cards, robots []Candidate, m int, pick func(int) int) []Candidate {
	for k, r := range robots {
		bound := m - (len(robots) - 1 - k)
		if bound < 1 {
			bound = 1
		}
		if bound > len(cards)+1 {
			bound = len(cards) + 1 // 可插入位置 [0, len(cards)]
		}
		pos := pick(bound)
		if pos > len(cards) {
			pos = len(cards)
		}
		cards = append(cards, Candidate{})
		copy(cards[pos+1:], cards[pos:])
		cards[pos] = r
	}
	return cards
}

// robotPool 取 need 个机器人候选：同城优先，不足跨城补（跨城的城市显示为我的城市，
// 营造同城人气，与 match.candidateRobots 口径一致）。
func (s *Service) robotPool(tenantID, userID int64, me *model.User, exclude map[int64]bool, need int) []Candidate {
	fetch := func(sameCity bool) []model.User {
		q := s.db.Model(&model.User{}).
			Where("tenant_id = ? AND is_robot = 1 AND status = ? AND user_id <> ?", tenantID, "active", userID)
		q = q.Where("user_id NOT IN (?)",
			s.db.Model(&model.Block{}).Select("target_id").Where("tenant_id = ? AND user_id = ?", tenantID, userID))
		q = q.Where("user_id NOT IN (?)",
			s.db.Model(&model.Block{}).Select("user_id").Where("tenant_id = ? AND target_id = ?", tenantID, userID))
		q = q.Where("user_id NOT IN (?)",
			s.db.Model(&model.Relation{}).Select("user_b").
				Where("tenant_id = ? AND user_a = ? AND type IN ?", tenantID, userID, []string{"like", RelationSkip}))
		if me.City != "" {
			if sameCity {
				q = q.Where("city = ?", me.City)
			} else {
				q = q.Where("city <> ?", me.City)
			}
		}
		var us []model.User
		q.Order("RAND()").Limit(need * 3).Find(&us)
		return us
	}
	picked := make([]Candidate, 0, need)
	add := func(us []model.User, overrideCity bool) {
		for i := range us {
			if len(picked) >= need {
				return
			}
			u := &us[i]
			if exclude[u.UserID] {
				continue
			}
			if overrideCity && me.City != "" {
				u.City = me.City
			}
			exclude[u.UserID] = true
			picked = append(picked, buildCandidate(me, u))
		}
	}
	add(fetch(true), false)
	if len(picked) < need {
		add(fetch(false), true)
	}
	return picked
}

// ---- 答题匹配（替代语言/兴趣维度，加分排序；后台题库可配，默认关）----

// QuizQuestion 一道题：key + 问题 + 选项，下发给前端筛选页渲染。
type QuizQuestion struct {
	Key      string   `json:"key"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

// QuizEnabled 答题匹配是否开启。
func (s *Service) QuizEnabled(tenantID int64) bool {
	return sysconfig.GetInt64(tenantID, sysconfig.KeyAppDiscoverQuizEnabled) == 1
}

// Quiz 题库：解析后台配置「每行 qkey|问题|逗号分隔选项」。
func (s *Service) Quiz(tenantID int64) []QuizQuestion {
	out := []QuizQuestion{}
	for _, line := range strings.Split(sysconfig.GetString(tenantID, sysconfig.KeyAppDiscoverQuiz), "\n") {
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) < 3 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		var opts []string
		for _, o := range strings.Split(parts[2], ",") {
			if o = strings.TrimSpace(o); o != "" {
				opts = append(opts, o)
			}
		}
		if len(opts) < 2 {
			continue
		}
		out = append(out, QuizQuestion{Key: strings.TrimSpace(parts[0]), Question: strings.TrimSpace(parts[1]), Options: opts})
	}
	return out
}

// MyQuizAnswers 读我的答案串（供筛选页回显）。
func (s *Service) MyQuizAnswers(userID int64) string {
	var u model.User
	s.db.Select("quiz_answers").First(&u, "user_id = ?", userID)
	return u.QuizAnswers
}

// SaveQuizAnswers 存我的答案（紧凑串 "qkey:idx,..."，前端拼好）。
func (s *Service) SaveQuizAnswers(userID int64, answers string) error {
	return s.db.Model(&model.User{}).Where("user_id = ?", userID).
		Update("quiz_answers", answers).Error
}

// parseAnswers 解析 "qkey:idx,qkey:idx" → map[qkey]idx。
func parseAnswers(s string) map[string]string {
	out := map[string]string{}
	for _, p := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) == 2 && kv[0] != "" && kv[1] != "" {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

// quizMatch 我与对方答相同答案的题数。
func quizMatch(mine map[string]string, theirs string) int {
	if len(mine) == 0 || theirs == "" {
		return 0
	}
	other := parseAnswers(theirs)
	n := 0
	for k, v := range mine {
		if other[k] == v {
			n++
		}
	}
	return n
}

// Count 符合当前筛选的人数，供筛选页底部的「应用筛选 · N 人符合」。
// 不走机器人注入：那是排序层的兜底，不该改变「有多少人符合条件」。
func (s *Service) Count(tenantID, userID int64, f Filter) (int, error) {
	me, err := s.loadMe(userID)
	if err != nil {
		return 0, err
	}
	list, err := s.ranked(tenantID, userID, me, f)
	if err != nil {
		return 0, err
	}
	return len(list), nil
}

// SkipChargeEnabled 左滑跳过是否扣币(付费功能,默认关)。
func (s *Service) SkipChargeEnabled(tenantID int64) bool {
	return sysconfig.GetInt64(tenantID, sysconfig.KeyAppDiscoverSkipChargeEnabled) == 1
}

// SkipPrice 左滑跳过一次的金币价格(后台可调)。
func (s *Service) SkipPrice(tenantID int64) int64 {
	return sysconfig.GetInt64(tenantID, sysconfig.KeyAppDiscoverSkipPrice)
}

// Pass 左滑：落库，否则下次拉人去重不掉，同一个人会反复推上来。
//
// 开启左滑扣币时(付费功能,默认关)：扣币与落库同一事务——先扣后落之间崩掉，
// 用户就是付了钱没记上、下次又被推同一个人。已跳过过的人幂等返回、不重复扣费。
func (s *Service) Pass(tenantID, userID, targetID int64) error {
	var n int64
	s.db.Model(&model.Relation{}).
		Where("tenant_id = ? AND user_a = ? AND user_b = ? AND type = ?", tenantID, userID, targetID, RelationSkip).
		Count(&n)
	if n > 0 {
		return nil
	}

	rel := &model.Relation{
		RelationID: idgen.Next(),
		TenantID:   tenantID,
		UserA:      userID,
		UserB:      targetID,
		Type:       RelationSkip,
		Stage:      "stranger",
		CreatedAt:  time.Now(),
	}

	// 未开启扣费(或价格配成 0)：沿用原行为，只落库。
	if !s.SkipChargeEnabled(tenantID) || s.SkipPrice(tenantID) <= 0 {
		return s.db.Create(rel).Error
	}

	bizNo := "skip:" + strconv.FormatInt(rel.RelationID, 10)
	return s.wallet.Debit(tenantID, userID, s.SkipPrice(tenantID), wallet.SceneDiscoverSkip, bizNo,
		func(tx *gorm.DB) error {
			return tx.Create(rel).Error
		})
}

// buildCandidate User → Candidate。距离两边都有坐标才算得出来。
func buildCandidate(me, u *model.User) Candidate {
	c := Candidate{
		ID:     strconv.FormatInt(u.UserID, 10),
		UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar,
		Gender: u.Gender, Age: u.Age, City: u.City, Bio: u.Bio,
		Language: u.Language, Interests: u.Interests, Charm: u.Charm,
		LastActive: u.LastActiveAt,
		// 机器人永远在线：它随时能回话，这是真话不是假话；真人按 5 分钟内活跃算。
		Online:  u.IsRobot || time.Since(u.LastActiveAt) < 5*time.Minute,
		isRobot: u.IsRobot,
	}
	if me.Lat != 0 && me.Lng != 0 && u.Lat != 0 && u.Lng != 0 {
		d := geodist.KM(me.Lat, me.Lng, u.Lat, u.Lng)
		c.DistanceKM = &d
	}
	return c
}

// RewindPrice 撤回一次的金币价格(后台可调)。
func (s *Service) RewindPrice(tenantID int64) int64 {
	return sysconfig.GetInt64(tenantID, sysconfig.KeyAppRewindPrice)
}

// Rewind 撤回上一次左滑：扣币把刚划掉的人放回候选池。
//
// 只撤 skip 不撤 like：右滑已经写进关系链、可能触发了匹配与通知，
// 回滚那些副作用不是删一行能兜住的。
//
// 扣费与删记录必须同一事务——先扣后删之间崩掉，用户就是付了钱没撤回。
func (s *Service) Rewind(tenantID, userID int64) (*Candidate, error) {
	var rel model.Relation
	err := s.db.Where("tenant_id = ? AND user_a = ? AND type = ?", tenantID, userID, RelationSkip).
		// created_at 只到秒，连划两下会同秒；用雪花 ID 兜底保证拿到的是最后一条。
		Order("created_at desc, relation_id desc").First(&rel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.New(errs.CodeBadRequest, "没有可撤回的人")
	}
	if err != nil {
		return nil, err
	}

	me, err := s.loadMe(userID)
	if err != nil {
		return nil, err
	}
	var target model.User
	if err := s.db.First(&target, "user_id = ?", rel.UserB).Error; err != nil {
		return nil, errs.New(errs.CodeBadRequest, "对方账号已不存在")
	}

	bizNo := "rewind:" + strconv.FormatInt(rel.RelationID, 10)
	err = s.wallet.Debit(tenantID, userID, s.RewindPrice(tenantID), wallet.SceneRewind, bizNo,
		func(tx *gorm.DB) error {
			res := tx.Where("relation_id = ?", rel.RelationID).Delete(&model.Relation{})
			if res.Error != nil {
				return res.Error
			}
			// 0 行 = 并发下已被另一个请求撤走，整笔回滚，别让用户白扣。
			if res.RowsAffected == 0 {
				return errs.New(errs.CodeBadRequest, "没有可撤回的人")
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	c := buildCandidate(me, &target)
	return &c, nil
}

func (s *Service) loadMe(userID int64) (*model.User, error) {
	var me model.User
	if err := s.db.First(&me, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &me, nil
}

func (s *Service) maxKM(tenantID int64, f Filter) int {
	if f.MaxKM != nil {
		return *f.MaxKM
	}
	return sysconfig.GetInt(tenantID, sysconfig.KeyAppDiscoverMaxKM)
}

// candidates 只做**硬过滤**：租户、状态、黑名单、已滑过、性别、年龄、语言、兴趣、
// 以及有位置时的 bounding box 预筛。距离精算与打分放到内存里做。
func (s *Service) candidates(tenantID, userID int64, me *model.User, f Filter) ([]model.User, error) {
	q := s.db.Model(&model.User{}).
		Where("tenant_id = ? AND status = ? AND user_id <> ?", tenantID, "active", userID)

	// 双向拉黑都要排除
	q = q.Where("user_id NOT IN (?)",
		s.db.Model(&model.Block{}).Select("target_id").Where("tenant_id = ? AND user_id = ?", tenantID, userID))
	q = q.Where("user_id NOT IN (?)",
		s.db.Model(&model.Block{}).Select("user_id").Where("tenant_id = ? AND target_id = ?", tenantID, userID))

	// 已经右滑/左滑过的不再出现
	q = q.Where("user_id NOT IN (?)",
		s.db.Model(&model.Relation{}).Select("user_b").
			Where("tenant_id = ? AND user_a = ? AND type IN ?", tenantID, userID, []string{"like", RelationSkip}))

	if f.Gender != nil && *f.Gender > 0 {
		q = q.Where("gender = ?", *f.Gender)
	}
	if f.MinAge > 0 {
		q = q.Where("age >= ?", f.MinAge)
	}
	if f.MaxAge > 0 {
		q = q.Where("age <= ?", f.MaxAge)
	}
	if f.Tab == "new" {
		q = q.Where("created_at >= ?", time.Now().AddDate(0, 0, -7))
	}

	// 语言/兴趣：命中任意一个即可（筛选是「或」语义，不是「且」）。
	// ⚠️ 答题匹配开启时，语言/兴趣被答题维度**替代**（产品选择:替代不叠加），
	// 这里连硬过滤一起跳过——否则前端残留的旧语言/兴趣条件会变成隐形筛选，
	// 把候选人整片筛掉。评分侧的 quizMode 分支同理只加 quizBonus。
	if !s.QuizEnabled(tenantID) {
		if conds, args := csvAnyMatch("language", f.Languages); conds != "" {
			q = q.Where(conds, args...)
		}
		if conds, args := csvAnyMatch("interests", f.Interests); conds != "" {
			q = q.Where(conds, args...)
		}
	}

	// bounding box 预筛：把距离筛选压进索引，避免全表算 haversine。
	// 先用矩形粗筛，再在内存里用精确距离二次过滤。
	//
	// ⚠️ 必须放行没有坐标的人（lat/lng 全 0）。
	// 内存那一层只在「算得出距离」时才按上限丢弃，SQL 这层如果不放行，
	// 就比内存层更严——所有没开过定位的用户被整片筛掉，
	// 表现是「发现页只剩机器人（有 seed 坐标），看不到真实用户」。
	if maxKM := s.maxKM(tenantID, f); maxKM > 0 && me.Lat != 0 && me.Lng != 0 {
		dLat := float64(maxKM) / 111.0
		cos := math.Cos(me.Lat * math.Pi / 180)
		if cos < 0.01 {
			cos = 0.01 // 极地附近兜底，避免除零放大成全表
		}
		dLng := float64(maxKM) / (111.0 * cos)
		q = q.Where("((lat BETWEEN ? AND ? AND lng BETWEEN ? AND ?) OR (lat = 0 AND lng = 0))",
			me.Lat-dLat, me.Lat+dLat, me.Lng-dLng, me.Lng+dLng)
	}

	var rows []model.User
	// GORM v2 的 Order 只认 string，clause.Expr 会被静默忽略。
	err := q.Order("last_active_at desc").Limit(candidatePool).Find(&rows).Error
	return rows, err
}

// csvAnyMatch 为逗号分隔字段拼「命中任意一个」的条件。
// 用 FIND_IN_SET 而不是 LIKE，避免 "music" 命中 "musical"。
func csvAnyMatch(column string, values []string) (string, []interface{}) {
	var parts []string
	var args []interface{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		parts = append(parts, "FIND_IN_SET(?, "+column+") > 0")
		args = append(args, v)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

// score 共性(语言/兴趣 或 答题) × 距离衰减 × 活跃度，机器人扣分。
//
// quizMode=true 时用「答相同答案」的 quizBonus 替代语言/兴趣维度(产品选择:替代不叠加)。
func score(myLangs, myInterests []string, distKM *float64, maxKM int, u *model.User, quizMode bool, quizBonus int) int {
	s := 0

	if quizMode {
		s += quizBonus
	} else {
		// 语言：能说同一种语言是聊下去的前提，按命中数累加
		s += len(intersect(myLangs, splitCSV(u.Language))) * wLanguage

		// 兴趣：Jaccard，避免「兴趣填得多的人」天然占优
		his := splitCSV(u.Interests)
		if len(myInterests) > 0 && len(his) > 0 {
			inter := len(intersect(myInterests, his))
			union := len(myInterests) + len(his) - inter
			if union > 0 {
				s += inter * wInterest / union
			}
		}
	}

	// 距离：越近越高，线性衰减
	if distKM != nil && maxKM > 0 {
		ratio := 1 - *distKM/float64(maxKM)
		if ratio > 0 {
			s += int(ratio * wDistance)
		}
	}

	// 活跃度：24 小时内线性衰减
	if h := time.Since(u.LastActiveAt).Hours(); h < 24 {
		s += int((1 - h/24) * wActive)
	}

	if u.IsRobot {
		s -= pRobot
	}
	return s
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func intersect(a, b []string) []string {
	set := make(map[string]bool, len(a))
	for _, v := range a {
		set[v] = true
	}
	var out []string
	for _, v := range b {
		if set[v] {
			out = append(out, v)
		}
	}
	return out
}
