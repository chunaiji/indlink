package robot

import (
	"context"
	"log"
	"math/rand"
	"slices"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"

	"gorm.io/gorm"
)

const botQueueSize = 1000

// BotJob 一条待处理的机器人回复任务。
type BotJob struct {
	TenantID   int64
	ChatID     int64
	BotUserID  int64
	UserID     int64  // 真人用户（消息接收者）
	UserMsg    string // 用户发送的原始消息
}

// MessageSender 解耦 chat.Service.SendMessage，避免 import cycle。
type MessageSender interface {
	SendMessage(tenantID, senderID, chatID int64, content, msgType string) (*model.Message, error)
	HistoryRecent(chatID int64, limit int) ([]model.Message, error)
	EnsureRobotChat(tenantID, botUserID, userID int64) (int64, error)
}

// BotWorkerPool 管理 BotReplyQueue 和 Worker goroutine 池。
type BotWorkerPool struct {
	queue    chan BotJob
	db       *gorm.DB
	registry *BotRegistry
	memory   *MemoryStore
	matcher  *KeywordMatcher
	cache    *ReplyCache
	llm      *LLMClient
	sender   MessageSender
}

var globalPool *BotWorkerPool

// InitWorkerPool 在 main 启动后调用，注入 chat.Service（通过 MessageSender 接口）。
func InitWorkerPool(db *gorm.DB, sender MessageSender, registry *BotRegistry, memory *MemoryStore, matcher *KeywordMatcher, cache *ReplyCache, llm *LLMClient, defaultTenant int64) {
	globalPool = &BotWorkerPool{
		queue:    make(chan BotJob, botQueueSize),
		db:       db,
		registry: registry,
		memory:   memory,
		matcher:  matcher,
		cache:    cache,
		llm:      llm,
		sender:   sender,
	}

	concurrency := sysconfig.GetInt(defaultTenant, sysconfig.KeyLLMConcurrency)
	if concurrency <= 0 {
		concurrency = 10
	}
	for i := 0; i < concurrency; i++ {
		go globalPool.worker()
	}
	log.Printf("[robot/worker] pool started, workers=%d queue=%d", concurrency, botQueueSize)
}

// EnqueueBotReply 投递任务到队列；队列满则丢弃，不阻塞 HTTP。
func EnqueueBotReply(job BotJob) {
	if globalPool == nil {
		return
	}
	select {
	case globalPool.queue <- job:
	default:
		log.Printf("[robot/worker] queue full, dropped job chatID=%d", job.ChatID)
	}
}

// TestLLM 用于管理台测试 LLM 连接；pool 未初始化时直接构造临时 client。
func TestLLM(ctx context.Context, tenantID int64, prompt string) (string, error) {
	llm := globalPool.llm
	if llm == nil {
		llm = &LLMClient{sem: make(chan struct{}, 1)}
	}
	return llm.Chat(ctx, tenantID, "你是助手", nil, prompt)
}

func (p *BotWorkerPool) worker() {
	for job := range p.queue {
		p.handle(job)
	}
}

func (p *BotWorkerPool) handle(job BotJob) {
	if !sysconfig.GetBool(job.TenantID, sysconfig.KeyAIChatEnabled) {
		return
	}

	persona := p.registry.Get(job.BotUserID)
	personaRole := persona.AffectiveStyle // 以 affective_style 作为缓存 key 的 role 维度

	// ── Tier 0：身份识别硬拦截（最高优先级，不走缓存也不走 LLM）──────
	if shouldInterceptIdentityPressure(job.UserMsg) {
		p.deliver(job, buildIdentityResponse(job.UserMsg))
		return
	}

	// 防复读:该机器人在该会话最近 5 条消息,canned 回复(规则/缓存)与其任一相同则不用。
	recent := p.lastBotMessages(job.ChatID, job.BotUserID, 5)

	// ── Tier 1：关键字规则 ─────────────────────────────────────
	if reply, ruleID := p.matcher.Match(job.UserMsg, personaRole); reply != "" && !slices.Contains(recent, reply) {
		p.matcher.IncrHit(ruleID)
		p.deliver(job, reply)
		return
	}

	// ── Tier 2：LLM 缓存(变体未满时按概率穿透,让变体池逐步积累) ────
	hash := cacheKey(job.UserMsg, personaRole)
	variants := p.cache.GetVariants(job.TenantID, hash, personaRole)
	regen := len(variants) < maxVariants && rand.Intn(100) < cacheRegenProb
	if !regen {
		if picked := pickVariant(variants, recent); picked != "" {
			p.cache.IncrHit(job.TenantID, hash, personaRole)
			p.deliver(job, picked)
			return
		}
	}

	// ── Tier 3：LLM 调用 ──────────────────────────────────────
	memory := p.memory.GetOrCreate(job.TenantID, job.UserID, job.BotUserID)
	history, _ := p.sender.HistoryRecent(job.ChatID, 6)

	systemPrompt := buildPromptLang(persona, memory, p.botLanguage(job.BotUserID))
	historyMsgs := buildHistory(history, job.BotUserID)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	reply, err := p.llm.Chat(ctx, job.TenantID, systemPrompt, historyMsgs, job.UserMsg)
	if err != nil {
		log.Printf("[robot/worker] llm err chatID=%d: %v, fallback to static pool", job.ChatID, err)
		reply = p.fallbackStatic(job.TenantID)
		if reply == "" {
			return // 静态池也空，不发
		}
		p.deliver(job, sanitizeOutgoingReply(reply, job.UserMsg))
		return
	}

	if containsAIConfession(reply) {
		log.Printf("[robot/worker] ai confession detected chatID=%d, replacing", job.ChatID)
		p.deliver(job, buildIdentityResponse(job.UserMsg))
		return
	}
	reply = sanitizeOutgoingReply(reply, job.UserMsg)

	// 异步写缓存
	p.cache.AddVariant(job.TenantID, hash, personaRole, job.UserMsg, reply)

	p.deliver(job, reply)

	// 异步更新记忆（无 session summary，简化 V1）
	p.memory.UpdateAsync(job.TenantID, job.UserID, job.BotUserID, "")
}

// deliver 模拟打字延迟后发送机器人回复。
func (p *BotWorkerPool) deliver(job BotJob, text string) {
	text = sanitizeOutgoingReply(text, job.UserMsg)
	delayMin := sysconfig.GetInt(job.TenantID, sysconfig.KeyAIReplyDelayMin)
	delayMax := sysconfig.GetInt(job.TenantID, sysconfig.KeyAIReplyDelayMax)
	if delayMin <= 0 {
		delayMin = 1500
	}
	if delayMax <= delayMin {
		delayMax = delayMin + 2500
	}
	delay := time.Duration(delayMin+rand.Intn(delayMax-delayMin)) * time.Millisecond
	time.Sleep(delay)

	if _, err := p.sender.SendMessage(job.TenantID, job.BotUserID, job.ChatID, text, "text"); err != nil {
		log.Printf("[robot/worker] send err chatID=%d: %v", job.ChatID, err)
	}
}

// lastBotMessages 返回该会话最近 n 条由该机器人发送的消息内容(用于防复读)。
func (p *BotWorkerPool) lastBotMessages(chatID, botUserID int64, n int) []string {
	var contents []string
	p.db.Model(&model.Message{}).Where("chat_id = ? AND sender_id = ?", chatID, botUserID).
		Order("created_at desc").Limit(n).Pluck("content", &contents)
	return contents
}

// fallbackStatic 从静态内容池随机取一条 reply 类型的文本。
func (p *BotWorkerPool) fallbackStatic(tenantID int64) string {
	var pool []model.RobotContent
	p.db.Where("tenant_id = ? AND type = ?", tenantID, "reply").Find(&pool)
	if len(pool) == 0 {
		return ""
	}
	return pool[rand.Intn(len(pool))].Text
}

// botLanguage 机器人资料里的语言字段(决定 LLM 回复语言);查不到按中文。
func (p *BotWorkerPool) botLanguage(botUserID int64) string {
	var u model.User
	if p.db == nil || p.db.Select("language").First(&u, "user_id = ?", botUserID).Error != nil {
		return ""
	}
	return u.Language
}
