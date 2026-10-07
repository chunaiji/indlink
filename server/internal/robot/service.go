// Package robot 注入运营机器人,使海洋/同城有"人气":按管理台配置定时投放瓶子、对真人瓶回信。
// 机器人就是 IsRobot=true 的 User;内容取自 robot_content 池;匹配时已在 feed.go 降权。
package robot

import (
	gocontext "context"
	"log"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

type Service struct {
	db       *gorm.DB
	mu       sync.Mutex
	acc      map[int64]float64 // 每租户投放速率累加器(throw_per_hour 的小数累计)
	registry *BotRegistry
	memory   *MemoryStore
	cache    *ReplyCache
	llm      *LLMClient
	matchers sync.Map // tenantID → *KeywordMatcher
	sender   MessageSender
}

func New(db *gorm.DB, defaultTenant int64) *Service {
	return &Service{
		db:       db,
		acc:      map[int64]float64{},
		registry: newBotRegistry(db),
		memory:   newMemoryStore(db),
		cache:    newReplyCache(db),
		llm:      newLLMClient(defaultTenant),
	}
}

// Registry 供 main.go 在 InitWorkerPool 时传入。
func (s *Service) Registry() *BotRegistry { return s.registry }

// Memory 供 main.go 在 InitWorkerPool 时传入。
func (s *Service) Memory() *MemoryStore { return s.memory }

// Cache 供 main.go 在 InitWorkerPool 时传入。
func (s *Service) Cache() *ReplyCache { return s.cache }

// LLM 供 main.go 在 InitWorkerPool 时传入。
func (s *Service) LLM() *LLMClient { return s.llm }

// SetSender 注入 chat.Service(通过 MessageSender),供主动引导发消息。
// 与 getSender 共用 s.mu,避免 main goroutine 写、scheduler goroutine 读的数据竞争。
func (s *Service) SetSender(sender MessageSender) {
	s.mu.Lock()
	s.sender = sender
	s.mu.Unlock()
}

// getSender 并发安全地读取 sender,调用方应先判空再使用。
func (s *Service) getSender() MessageSender {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sender
}

// getMatcher 懒初始化每租户的关键字规则匹配器。
func (s *Service) getMatcher(tenantID int64) *KeywordMatcher {
	if v, ok := s.matchers.Load(tenantID); ok {
		return v.(*KeywordMatcher)
	}
	m := newKeywordMatcher(s.db, tenantID)
	s.matchers.Store(tenantID, m)
	return m
}

// GetMatcher 供 main.go 在 InitWorkerPool 时传入（使用 defaultTenantID）。
func (s *Service) GetMatcher(tenantID int64) *KeywordMatcher { return s.getMatcher(tenantID) }

// 资料池与按语言生成见 profilegen.go。

// EnsureRobots 保证本租户机器人数量达到 target(每次最多新建 maxCreate 个,避免突发)。
func (s *Service) EnsureRobots(tenantID int64, target int) []model.User {
	var existing []model.User
	s.db.Where("tenant_id = ? AND is_robot = ?", tenantID, true).Find(&existing)

	// 补齐存量机器人的空头像（首次运行或头像池新增后自动回填），按性别取池。
	// 未认证机器人是刻意素人化的(仿真实新用户:无头像/用户XX昵称),不回填。
	for i := range existing {
		if existing[i].Avatar == "" && existing[i].IsVerified {
			if av := randomAvatarByGender(existing[i].Gender); av != "" {
				s.db.Model(&existing[i]).Update("avatar", av)
				existing[i].Avatar = av
			}
		}
	}

	if len(existing) >= target {
		return existing
	}
	maxCreate := 10
	need := target - len(existing)
	if need > maxCreate {
		need = maxCreate
	}
	// 自动补齐的机器人按租户默认语言生成(后台 robot_language,默认中文);
	// 要指定语言/性别/城市的,走后台「新增机器人」。
	lang := RobotLang(tenantID)
	for i := 0; i < need; i++ {
		u := GenerateRobotUser(tenantID, GenOptions{Lang: lang})
		if err := s.db.Create(&u).Error; err != nil {
			continue
		}
		existing = append(existing, u)
	}
	return existing
}

// ReassignAvatarsByGender 把头像与性别不匹配的机器人重刷为对应性别头像。
// 幂等:男机器人头像若已含 /static/robot_man/、女机器人含 /static/robot/ 则跳过,
// 因此每次启动跑一次安全(已正确的不动,仅修正历史混用)。
func (s *Service) ReassignAvatarsByGender(tenantID int64) {
	var robots []model.User
	s.db.Where("tenant_id = ? AND is_robot = ?", tenantID, true).Find(&robots)
	fixed := 0
	for i := range robots {
		r := &robots[i]
		if r.Avatar == "" || strings.Contains(r.Avatar, genderPathTag(r.Gender)) {
			continue // 无头像(素人机器人,不补)或已是对应性别池,跳过
		}
		if av := randomAvatarByGender(r.Gender); av != "" {
			s.db.Model(r).Update("avatar", av)
			fixed++
		}
	}
	if fixed > 0 {
		log.Printf("[robot] tenant=%d 头像按性别重刷 %d 个", tenantID, fixed)
	}
}

// SeedContent 内容池为空时灌入默认文案(运营后续可在管理台维护)。
func (s *Service) SeedContent(tenantID int64) {
	var n int64
	s.db.Model(&model.RobotContent{}).Where("tenant_id = ?", tenantID).Count(&n)
	if n > 0 {
		return
	}
	bottles := []struct{ text, tags string }{
		{"今天加班到很晚,城市的灯一盏盏灭掉,突然有点想家。", "情感,树洞"},
		{"有没有人也总是在凌晨三点醒来,然后再也睡不着?", "失眠,树洞"},
		{"想找个人一起看海,不说话也可以。", "交友,情感"},
		{"刚搬到新城市,一个朋友都没有,有点孤单。", "交友,树洞"},
		{"今天吃到一家超好吃的小馆子,可惜没人分享。", "吐槽,交友"},
		{"考研倒计时,真的好累,但还是想再坚持一下。", "学习,树洞"},
		{"分手第三十天,好像没那么难过了,又好像还是会想起。", "情感,树洞"},
		{"想养只猫,可是房东不让,只能云吸猫了。", "吐槽,交友"},
		{"如果可以重来,你会对十八岁的自己说什么?", "情感,树洞"},
		{"晚风很温柔,适合给陌生人写一封信。", "交友,情感"},
		{"地铁上看到一对老夫妻手牵手,突然很想有人陪。", "情感,交友"},
		{"减肥第一天就破功了,谁懂啊。", "吐槽,日常"},
		{"下雨天最适合躺着发呆,可惜还要上班。", "吐槽,日常"},
		{"今天被夸了一句,开心到现在。", "日常,治愈"},
		{"一个人吃火锅,店员问几位的时候有点尴尬。", "日常,树洞"},
		{"好久没联系的朋友突然发来消息,心里暖暖的。", "情感,交友"},
		{"深夜的便利店灯光,总让我觉得没那么孤单。", "治愈,树洞"},
		{"想去看海,谁陪我?不说话也行。", "交友,情感"},
		{"加班到最后一个走,城市的夜好安静。", "职场,树洞"},
		{"今天的云像棉花糖,拍了好多张照片。", "日常,治愈"},
		{"emo了,有没有人陪我说说话。", "情感,树洞"},
		{"突然好想吃小时候巷口那家的糖葫芦。", "日常,回忆"},
		{"新的一周,给自己打打气。", "日常,治愈"},
		{"失眠到三点,数羊都没用。", "失眠,树洞"},
		{"想找个人一起打游戏,菜也没关系。", "交友,日常"},
		{"今天对自己说了句辛苦了,眼眶有点热。", "情感,治愈"},
		{"一个人的城市,连生病都要自己扛。", "树洞,情感"},
		{"周末不想出门,只想躺平。", "吐槽,日常"},
		{"路过花店买了一束花给自己。", "治愈,日常"},
		{"想被人认真地问一句:你还好吗?", "情感,树洞"},
	}
	replies := []struct{ text, tags string }{
		{"抱抱你,我也有过这样的时刻,会好起来的。", ""},
		{"看到你的瓶子,觉得不那么孤单了,谢谢你。", ""},
		{"我也是!原来海这头还有人懂。", ""},
		{"加油呀,陌生人,我在为你打气。", ""},
		{"说说细节吧?我想多了解一点。", ""},
		{"愿你今晚好梦,明天会更轻松一些。", ""},
		{"我也刚来这座城市,要不要做个朋友?", ""},
		{"读完心里暖暖的,世界还是很温柔的。", ""},
	}
	now := time.Now()
	rows := make([]model.RobotContent, 0, len(bottles)+len(replies))
	for _, b := range bottles {
		rows = append(rows, model.RobotContent{TenantID: tenantID, Type: "bottle", Text: b.text, Tags: b.tags, Weight: 1, CreatedAt: now})
	}
	for _, r := range replies {
		rows = append(rows, model.RobotContent{TenantID: tenantID, Type: "reply", Text: r.text, Weight: 1, CreatedAt: now})
	}
	s.db.Create(&rows)
}

// Tick 由调度器按 intervalMin 周期调用:按配置投放瓶子 + 概率回信。
func (s *Service) Tick(tenantID int64, intervalMin float64) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyRobotEnabled) {
		return
	}
	target := sysconfig.GetInt(tenantID, sysconfig.KeyRobotCount)
	if target <= 0 {
		return
	}
	s.SeedContent(tenantID)
	robots := s.EnsureRobots(tenantID, target)
	if len(robots) == 0 {
		return
	}

	// 投放速率累加(throw_per_hour 换算到本次 tick;不足 1 累计到下次)
	perHour := float64(sysconfig.GetInt(tenantID, sysconfig.KeyRobotThrowPerHr))
	s.mu.Lock()
	s.acc[tenantID] += perHour * intervalMin / 60.0
	throwN := int(s.acc[tenantID])
	s.acc[tenantID] -= float64(throwN)
	s.mu.Unlock()

	if throwN > 0 {
		s.throwBottles(tenantID, robots, throwN)
	}
	s.maybeReply(tenantID, robots)

	// 主动引导(总开关默认关 + 需 AI 续聊开关一并开启,否则真人回复后 worker 会因 KeyAIChatEnabled 关闭而不回,造成已读不回)
	if sysconfig.GetBool(tenantID, sysconfig.KeyRobotProactive) && sysconfig.GetBool(tenantID, sysconfig.KeyAIChatEnabled) {
		s.proactiveOutreach(tenantID, robots)
		s.nudgeStalledChats(tenantID, robots)
	}
}

// genBottleText 直连 LLM 生成一条造瓶文案(绕过按角色的粗缓存,保证多样),失败返回 ""。
func (s *Service) genBottleText(tenantID int64, personaRole, theme, mood string) string {
	bgCtx, cancel := gocontext.WithTimeout(gocontext.Background(), 15*time.Second)
	defer cancel()
	reply, err := s.llm.Chat(bgCtx, tenantID, s.buildPathABSystemPrompt(personaRole), nil, buildBottlePrompt(theme, mood))
	if err != nil {
		log.Printf("[robot] genBottleText err: %v", err)
		return ""
	}
	if strings.TrimSpace(reply) == "" {
		return ""
	}
	if containsAIConfession(reply) {
		return ""
	}
	reply = sanitizeOutgoingReply(reply, "")
	return strings.TrimSpace(reply)
}

func bottleExists(pool []model.RobotContent, text string) bool {
	for _, c := range pool {
		if c.Text == text {
			return true
		}
	}
	return false
}

func (s *Service) throwBottles(tenantID int64, robots []model.User, n int) {
	var pool []model.RobotContent
	s.db.Where("tenant_id = ? AND type = ?", tenantID, "bottle").Find(&pool)

	poolTarget := sysconfig.GetInt(tenantID, sysconfig.KeyRobotBottlePoolTarget)
	if poolTarget <= 0 {
		poolTarget = 80
	}
	aiEnabled := sysconfig.GetBool(tenantID, sysconfig.KeyAIBotEnabled)
	now := time.Now()

	for i := 0; i < n; i++ {
		r := robots[rand.Intn(len(robots))]
		var text, tags string

		// 池未满且 AI 开启:生成新文案(随机主题)入池;否则从池随机取。
		if aiEnabled && len(pool) < poolTarget {
			theme := bottleThemes[rand.Intn(len(bottleThemes))]
			mood := bottleMoods[rand.Intn(len(bottleMoods))]
			persona := s.registry.Get(r.UserID)
			gen := s.genBottleText(tenantID, persona.AffectiveStyle, theme, mood)
			if gen != "" && !bottleExists(pool, gen) {
				rc := model.RobotContent{TenantID: tenantID, Type: "bottle", Text: gen, Weight: 1, CreatedAt: now}
				if s.db.Create(&rc).Error == nil {
					pool = append(pool, rc)
				}
				text = gen
			}
		}
		if text == "" {
			if len(pool) == 0 {
				continue
			}
			c := pool[rand.Intn(len(pool))]
			text, tags = c.Text, c.Tags
		}

		b := model.Bottle{
			BottleID:    idgen.Next(),
			TenantID:    tenantID,
			UserID:      r.UserID,
			Content:     text,
			ContentType: "text",
			Tags:        tags,
			IsAnonymous: true,
			Scope:       "national",
			City:        r.City,
			Status:      "active",
			CreatedAt:   now,
			ExpireAt:    now.Add(7 * 24 * time.Hour),
		}
		s.db.Create(&b)
	}
	log.Printf("[robot] tenant=%d 投放 %d 个瓶子(池=%d/%d)", tenantID, n, len(pool), poolTarget)
}

// maybeReply 对最近的真人瓶子,按 reply_ratio 概率各回一条(每次 tick 限量,防刷屏)。
func (s *Service) maybeReply(tenantID int64, robots []model.User) {
	ratio := sysconfig.GetInt(tenantID, sysconfig.KeyRobotReplyRatio)
	if ratio <= 0 {
		return
	}
	var pool []model.RobotContent
	s.db.Where("tenant_id = ? AND type = ?", tenantID, "reply").Find(&pool)

	robotIDs := s.db.Model(&model.User{}).Select("user_id").Where("tenant_id = ? AND is_robot = ?", tenantID, true)
	var targets []model.Bottle
	s.db.Where("tenant_id = ? AND status = ? AND reply_count < ? AND user_id NOT IN (?)", tenantID, "active", 3, robotIDs).
		Order("created_at desc").Limit(5).Find(&targets)

	aiEnabled := sysconfig.GetBool(tenantID, sysconfig.KeyAIBotEnabled)
	now := time.Now()
	cnt := 0
	for _, b := range targets {
		if rand.Intn(100) >= ratio {
			continue
		}
		r := robots[rand.Intn(len(robots))]

		// 同瓶查重:已有回复文本不复用(跨机器人也不允许一字不差)
		var prior []string
		s.db.Model(&model.BottleReply{}).Where("bottle_id = ?", b.BottleID).Pluck("content", &prior)

		var text string
		if aiEnabled {
			persona := s.registry.Get(r.UserID)
			text = s.genAIText(tenantID, persona.AffectiveStyle, "reply", b.Content, prior)
		}
		if text == "" {
			if len(pool) == 0 {
				continue
			}
			text = strings.TrimSpace(pool[rand.Intn(len(pool))].Text)
		}
		if slices.Contains(prior, text) {
			continue // 静态池撞车或 LLM 罕见重复:放弃本条,宁缺毋滥
		}

		reply := model.BottleReply{
			ReplyID: idgen.Next(), TenantID: tenantID, BottleID: b.BottleID, UserID: r.UserID,
			Content: text, CreatedAt: now,
		}
		if err := s.db.Create(&reply).Error; err != nil {
			continue
		}
		s.db.Model(&model.Bottle{}).Where("bottle_id = ?", b.BottleID).
			Updates(map[string]interface{}{"reply_count": gorm.Expr("reply_count + 1"), "heat_score": gorm.Expr("heat_score + 3")})
		cnt++

		// 回信后引导加聊(总开关 + AI续聊开关 + 概率 + 每真人每日占位,防止同一真人被多条冷开场轰炸)
		if sender := s.getSender(); sender != nil &&
			sysconfig.GetBool(tenantID, sysconfig.KeyRobotProactive) &&
			sysconfig.GetBool(tenantID, sysconfig.KeyAIChatEnabled) {
			if r2 := sysconfig.GetInt(tenantID, sysconfig.KeyRobotBottle2ChatRatio); r2 > 0 && rand.Intn(100) < r2 {
				userCap := sysconfig.GetInt(tenantID, sysconfig.KeyOutreachUserDailyCap)
				if userCap <= 0 {
					userCap = 1
				}
				if ClaimDailyReach(tenantID, b.UserID, userCap) {
					if chatID, err := sender.EnsureRobotChat(tenantID, r.UserID, b.UserID); err == nil {
						sender.SendMessage(tenantID, r.UserID, chatID, randLine(bottleChatOpeners), "text")
					}
				}
			}
		}
	}
	if cnt > 0 {
		log.Printf("[robot] tenant=%d 回信 %d 条", tenantID, cnt)
	}
}

// genAIText 为 Path A/B 生成 AI 文本；失败返回 ""（调用方 fallback 静态池）。
// contentType: "bottle"(主动投放) or "reply"(回信)；msgCtx 为要回复的瓶子内容（回信时非空）。
// exclude: 不希望复用的文本(如该瓶已有回复),Tier 1/2 的 canned 结果命中其一时跳过。
func (s *Service) genAIText(tenantID int64, personaRole, contentType, msgCtx string, exclude []string) string {
	matcher := s.getMatcher(tenantID)

	// Tier 1：关键字规则（Path A/B 仅在有 msgCtx 时匹配）
	if msgCtx != "" {
		if reply, ruleID := matcher.Match(msgCtx, personaRole); reply != "" && !slices.Contains(exclude, reply) {
			matcher.IncrHit(ruleID)
			return reply
		}
	}

	// Tier 2：LLM 缓存(变体未满时按概率穿透,让变体池逐步积累)
	hashKey := msgCtx
	if hashKey == "" {
		hashKey = contentType
	}
	hash := cacheKey(hashKey, personaRole)
	variants := s.cache.GetVariants(tenantID, hash, personaRole)
	regen := len(variants) < maxVariants && rand.Intn(100) < cacheRegenProb
	if !regen {
		if picked := pickVariant(variants, exclude); picked != "" {
			s.cache.IncrHit(tenantID, hash, personaRole)
			return picked
		}
	}

	// Tier 3：LLM
	var userPrompt string
	switch contentType {
	default:
		userPrompt = "请对以下漂流瓶内容写一条简短、温暖的回应，30字以内，" +
			"自然地带一个小问题引导对方继续聊：\n" + msgCtx
	}
	bgCtx, cancel := gocontext.WithTimeout(gocontext.Background(), 15*time.Second)
	defer cancel()

	reply, err := s.llm.Chat(bgCtx, tenantID, s.buildPathABSystemPrompt(personaRole), nil, userPrompt)
	if err != nil {
		log.Printf("[robot] genAIText llm err: %v", err)
		return ""
	}
	if strings.TrimSpace(reply) == "" {
		return ""
	}
	if containsAIConfession(reply) {
		return ""
	}
	reply = sanitizeOutgoingReply(reply, msgCtx)
	s.cache.AddVariant(tenantID, hash, personaRole, hashKey, reply)
	return reply
}

func (s *Service) buildPathABSystemPrompt(personaRole string) string {
	labels := map[string]string{
		"warm_soft": "温柔体贴", "calm": "平静理性", "energetic": "活力开朗",
		"playful": "俏皮活泼", "dominant": "主导强势",
	}
	style := labels[personaRole]
	if style == "" {
		style = "自然随性"
	}
	return "你是一个" + style + "风格的用户，正在使用漂流瓶社交应用。用中文回复，语言自然真实，不超过80字。"
}
