package match

import (
	"math/rand"
	"strconv"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"

	"gorm.io/gorm"
)

// Service 提供同城用户列表与扩列墙(活跃/新人/附近)。
type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service { return &Service{db: db} }

// UserCard 列表展示用的精简用户信息。
type UserCard struct {
	UserID      int64     `json:"user_id,string"`
	Nickname    string    `json:"nickname"`
	Avatar      string    `json:"avatar"`
	Gender      int8      `json:"gender"`
	Age         int       `json:"age"`
	City        string    `json:"city"`
	IsVerified  bool      `json:"is_verified"`
	OnlineHint  string    `json:"online_hint"`  // 刚刚/注册N天 等
	CreatedAt   time.Time `json:"created_at"`   // 注册时间
	FollowCount int       `json:"follow_count"` // ta 关注的人数(我喜欢)
	FansCount   int       `json:"fans_count"`   // 关注 ta 的人数(喜欢我)
	Charm       int64     `json:"charm"`        // 魅力值(收礼累加)
	IsRobot     bool      `json:"is_robot"`
}

func toCards(users []model.User) []UserCard {
	out := make([]UserCard, 0, len(users))
	now := time.Now()
	for _, u := range users {
		hint := "刚刚在线"
		if now.Sub(u.LastActiveAt) > time.Hour {
			days := int(now.Sub(u.CreatedAt).Hours() / 24)
			hint = "注册 " + strconv.Itoa(days) + " 天"
		}
		out = append(out, UserCard{
			UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar,
			Gender: u.Gender, Age: u.Age, City: u.City, IsVerified: u.IsVerified,
			OnlineHint: hint, CreatedAt: u.CreatedAt, Charm: u.Charm, IsRobot: u.IsRobot,
		})
	}
	return out
}

// fillCounts 批量补关注/被关注数,避免逐条查询(N+1)。
func (s *Service) fillCounts(cards []UserCard) {
	if len(cards) == 0 {
		return
	}
	ids := make([]int64, len(cards))
	for i, c := range cards {
		ids[i] = c.UserID
	}
	type cnt struct {
		ID int64
		N  int
	}
	follow := map[int64]int{} // user_a 维度:ta 关注/喜欢的人数
	fans := map[int64]int{}   // user_b 维度:关注/喜欢 ta 的人数
	var fr []cnt
	s.db.Model(&model.Relation{}).Select("user_a as id, count(*) as n").
		Where("type = ? AND user_a IN ?", "like", ids).Group("user_a").Scan(&fr)
	for _, r := range fr {
		follow[r.ID] = r.N
	}
	var fa []cnt
	s.db.Model(&model.Relation{}).Select("user_b as id, count(*) as n").
		Where("type = ? AND user_b IN ?", "like", ids).Group("user_b").Scan(&fa)
	for _, r := range fa {
		fans[r.ID] = r.N
	}
	for i := range cards {
		cards[i].FollowCount = follow[cards[i].UserID]
		cards[i].FansCount = fans[cards[i].UserID]
	}
}

// blockedIDs 返回"我拉黑的"和"拉黑了我的"用户 ID 集合。
func (s *Service) blockedIDs(selfID int64) []int64 {
	var a, b []int64
	s.db.Model(&model.Block{}).Where("user_id = ?", selfID).Pluck("target_id", &a)
	s.db.Model(&model.Block{}).Where("target_id = ?", selfID).Pluck("user_id", &b)
	return append(a, b...)
}

// CityUsers 同城用户:按城市/性别/关键词(昵称或城市模糊)筛选,排除自己,限本租户。
// sort: active(默认,最近活跃) / new(新人) / fans(被关注多)
func (s *Service) CityUsers(tenantID, selfID int64, city, keyword string, gender int8, sort string, page, size int) ([]UserCard, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	q := s.db.Model(&model.User{}).Where("tenant_id = ? AND user_id <> ? AND status = ?", tenantID, selfID, "active")
	if ids := s.blockedIDs(selfID); len(ids) > 0 {
		q = q.Where("user_id NOT IN ?", ids)
	}
	if city != "" {
		q = q.Where("city = ?", city)
	}
	if gender > 0 {
		q = q.Where("gender = ?", gender)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("nickname LIKE ? OR city LIKE ?", kw, kw)
	}
	switch sort {
	case "new":
		q = q.Order("created_at desc")
	default: // active
		q = q.Order("last_active_at desc")
	}
	var users []model.User
	err := q.Offset((page - 1) * size).Limit(size).Find(&users).Error
	if err != nil {
		return nil, err
	}
	cards := toCards(users)
	s.fillCounts(cards)

	// 同城机器人注入:前 M 个位置保 N 个机器人(随机位置)。
	if n := sysconfig.GetInt(tenantID, sysconfig.KeyCityRobotTopN); n > 0 {
		m := sysconfig.GetInt(tenantID, sysconfig.KeyCityRobotTopM)
		if m <= 0 {
			m = 10
		}
		win := m
		if len(cards) < win {
			win = len(cards)
		}
		have := 0
		for i := 0; i < win; i++ {
			if cards[i].IsRobot {
				have++
			}
		}
		if need := n - have; need > 0 {
			exclude := map[int64]bool{}
			for _, c := range cards {
				exclude[c.UserID] = true
			}
			robots := s.candidateRobots(tenantID, selfID, city, exclude, s.blockedIDs(selfID), need)
			cards = injectRobots(cards, robots, m, n, func(k int) int { return rand.Intn(k) })
		}
	}

	return cards, nil
}

// ExpandWall 扩列墙:active=最近活跃 / new=新人 / nearby=同城。
func (s *Service) ExpandWall(tenantID, selfID int64, wallType, city string, gender int8, page, size int) ([]UserCard, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	q := s.db.Model(&model.User{}).Where("tenant_id = ? AND user_id <> ? AND status = ?", tenantID, selfID, "active")
	if ids := s.blockedIDs(selfID); len(ids) > 0 {
		q = q.Where("user_id NOT IN ?", ids)
	}
	if gender > 0 {
		q = q.Where("gender = ?", gender)
	}
	switch wallType {
	case "new":
		q = q.Order("created_at desc")
	case "nearby":
		if city != "" {
			q = q.Where("city = ?", city)
		}
		q = q.Order("last_active_at desc")
	default: // active
		q = q.Order("last_active_at desc")
	}
	var users []model.User
	if err := q.Offset((page - 1) * size).Limit(size).Find(&users).Error; err != nil {
		return nil, err
	}
	cards := toCards(users)
	s.fillCounts(cards)
	return cards, nil
}

// injectRobots 在 cards 前 m 个位置随机插入机器人,使前 m 内机器人数尽量达到 n。
// pick(k) 返回 [0,k) 的随机位置(便于测试)。robots 为已去重的候选(标记 IsRobot=true)。
func injectRobots(cards, robots []UserCard, m, n int, pick func(int) int) []UserCard {
	if n <= 0 || len(robots) == 0 || m <= 0 {
		return cards
	}
	win := m
	if len(cards) < win {
		win = len(cards)
	}
	have := 0
	for i := 0; i < win; i++ {
		if cards[i].IsRobot {
			have++
		}
	}
	need := n - have
	if need > len(robots) {
		need = len(robots)
	}
	for k := 0; k < need; k++ {
		bound := m
		if bound > len(cards)+1 {
			bound = len(cards) + 1 // 可插入位置 [0, len(cards)]
		}
		pos := pick(bound)
		if pos > len(cards) {
			pos = len(cards)
		}
		r := robots[k]
		r.IsRobot = true
		cards = append(cards, UserCard{})
		copy(cards[pos+1:], cards[pos:])
		cards[pos] = r
	}
	return cards
}

// candidateRobots 取本租户机器人卡片:同城优先,不足用其他城市补;跨城的城市显示为请求城市。
func (s *Service) candidateRobots(tenantID, selfID int64, city string, exclude map[int64]bool, blocked []int64, need int) []UserCard {
	fetch := func(sameCity bool) []model.User {
		q := s.db.Model(&model.User{}).
			Where("tenant_id = ? AND is_robot = 1 AND status = ? AND user_id <> ?", tenantID, "active", selfID)
		if len(blocked) > 0 {
			q = q.Where("user_id NOT IN ?", blocked)
		}
		if city != "" {
			if sameCity {
				q = q.Where("city = ?", city)
			} else {
				q = q.Where("city <> ?", city)
			}
		}
		var us []model.User
		q.Order("RAND()").Limit(need * 3).Find(&us)
		return us
	}
	picked := make([]model.User, 0, need)
	add := func(us []model.User, overrideCity bool) {
		for _, u := range us {
			if len(picked) >= need {
				return
			}
			if exclude[u.UserID] {
				continue
			}
			if overrideCity && city != "" {
				u.City = city // 跨城补进来的,显示为请求城市(营造同城人气)
			}
			exclude[u.UserID] = true
			picked = append(picked, u)
		}
	}
	add(fetch(true), false)
	if len(picked) < need {
		add(fetch(false), true)
	}
	cards := toCards(picked)
	for i := range cards {
		cards[i].IsRobot = true
	}
	s.fillCounts(cards)
	return cards
}
