package main

import (
	"fmt"
	"log"
	"time"

	"driftbottle/internal/ad"
	"driftbottle/internal/admin"
	"driftbottle/internal/bootstrap"
	"driftbottle/internal/bottle"
	"driftbottle/internal/chat"
	"driftbottle/internal/checkin"
	"driftbottle/internal/collection"
	"driftbottle/internal/common/jwtutil"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/ratelimit"
	"driftbottle/internal/config"
	"driftbottle/internal/crypto"
	"driftbottle/internal/discover"
	"driftbottle/internal/geo"
	"driftbottle/internal/item"
	"driftbottle/internal/match"
	"driftbottle/internal/model"
	"driftbottle/internal/moderation"
	"driftbottle/internal/moment"
	"driftbottle/internal/notify"
	"driftbottle/internal/pay"
	"driftbottle/internal/provider"
	"driftbottle/internal/push"
	"driftbottle/internal/rank"
	"driftbottle/internal/relation"
	"driftbottle/internal/robot"
	"driftbottle/internal/share"
	"driftbottle/internal/spark"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/tenant"
	"driftbottle/internal/upload"
	"driftbottle/internal/user"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/apilog"
	"driftbottle/pkg/cache"
	"driftbottle/pkg/database"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	jwtutil.Setup(cfg.JWTSecret, cfg.JWTExpireHours)

	// 基础设施
	if err := database.Init(cfg.MySQLDSN); err != nil {
		log.Fatalf("mysql 连接失败: %v", err)
	}
	if cfg.RedisURL != "" {
		if err := cache.InitURL(cfg.RedisURL); err != nil {
			log.Fatalf("redis 连接失败(URL): %v", err)
		}
	} else {
		if err := cache.Init(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB); err != nil {
			log.Fatalf("redis 连接失败: %v", err)
		}
	}
	if err := bootstrap.Migrate(database.DB); err != nil {
		log.Fatalf("建表失败: %v", err)
	}
	if err := bootstrap.Seed(database.DB, cfg.DefaultTenantID); err != nil {
		log.Fatalf("初始化数据失败: %v", err)
	}
	if err := sysconfig.Init(database.DB); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	// 管理后台:注入签名密钥 + 播种默认管理员
	admin.Setup(cfg.JWTSecret)
	if err := admin.Init(database.DB); err != nil {
		log.Fatalf("初始化管理员失败: %v", err)
	}

	db := database.DB

	// 凭证加密主密钥 + 租户凭证缓存
	crypto.Setup(cfg.CredMasterKey)
	credStore := tenant.New(db)
	// 无条件 Reload:provider.Migrate 要从这里读旧支付凭证,空着跑一遍会什么都搬不到,
	// 却照样把 provider_migrated 置 1——之后再开多租户也不会重跑,商户密钥得手工重敲。
	if err := credStore.Reload(); err != nil {
		if cfg.MultiTenant {
			log.Fatalf("加载租户凭证失败: %v", err)
		}
		log.Printf("[tenant] 凭证加载失败(单租户模式,忽略): %v", err)
	}
	if cfg.MultiTenant {
		log.Printf("多租户模式已启用")
	}

	// ---- 装配各模块服务(模块化单体:包边界即模块边界)----
	walletSvc := wallet.New(db)
	// 服务商配置(支付 / 地图 / 内容安全)+ 一次性把旧凭证、旧 sysconfig 键搬进来
	providerStore := provider.New(db)
	if err := providerStore.Reload(); err != nil {
		log.Fatalf("加载服务商配置失败: %v", err)
	}
	if err := provider.Migrate(db, credStore.All(), providerStore); err != nil {
		log.Fatalf("迁移服务商配置失败: %v", err)
	}

	// 第三方登录配置迁移。独立标记 sso_migrated:provider_migrated 线上早已是 "1",
	// 复用它这段永远不会跑。
	if err := provider.MigrateSSO(db, credStore.All(), providerStore); err != nil {
		log.Printf("[provider/sso] 迁移失败(不阻断启动): %v", err)
	}

	userSvc := user.New(db, walletSvc, cfg, credStore)
	userSvc.SetProviders(providerStore)
	middleware.OnAuthenticated = func(tenantID, userID int64) { userSvc.TouchActive(tenantID, userID) }
	// 账号注销后旧 JWT 立即失效(查 Redis,单次往返)
	middleware.IsRevoked = userSvc.IsRevoked
	modSvc := moderation.New(db)
	modSvc.SetProviders(providerStore, userSvc.AlipayClient) // 内容安全服务商 + 支付宝客户端(复用支付配置)
	paySvc := pay.New(db, walletSvc, userSvc.GetWxOpenID, userSvc.GetAlipayUID, cfg, credStore, providerStore)
	bottleSvc := bottle.New(db, modSvc, walletSvc, userSvc)
	matchSvc := match.New(db)
	relationSvc := relation.New(db)
	itemSvc := item.New(db, walletSvc)
	hub := chat.NewHub()
	chatSvc := chat.New(db, walletSvc, modSvc, userSvc, hub)

	// 通知(#4):站内信 + 在线 WS 推送;回信成功回调通知瓶主
	notifySvc := notify.New(db, hub)
	bottleSvc.OnReplied = notifySvc.OnBottleReplied
	bottleSvc.OnScooped = notifySvc.OnBottleScooped

	// 机器人调度(总开关 sysconfig.robot_enabled,默认关闭)
	robot.InitAvatarPool(cfg.UploadDir, cfg.PublicBaseURL)
	robotSvc := robot.New(db, cfg.DefaultTenantID)
	robot.Start(robotSvc, db, cfg.DefaultTenantID, cfg.MultiTenant)
	// 注入 bot_worker pool（路径 C 聊天续接）
	robot.InitWorkerPool(db, chatSvc, robotSvc.Registry(), robotSvc.Memory(), robotSvc.GetMatcher(cfg.DefaultTenantID), robotSvc.Cache(), robotSvc.LLM(), cfg.DefaultTenantID)
	robotSvc.SetSender(chatSvc) // 供主动引导(破冰/沉默追问)发消息
	robot.StartOutreachCron(robotSvc, chatSvc, cfg.DefaultTenantID)
	chatSvc.SetBotEnqueue(func(tenantID, chatID, botUserID, userID int64, userMsg string) {
		robot.EnqueueBotReply(robot.BotJob{
			TenantID: tenantID, ChatID: chatID, BotUserID: botUserID,
			UserID: userID, UserMsg: userMsg,
		})
	})

	// ---- 路由 ----
	if cfg.Env != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(middleware.CORS())

	auth := middleware.Auth()
	api := r.Group("/api")

	// 限流中间件
	throwLimit := ratelimit.PerUser("throw", 10, time.Minute) // 发瓶 10次/分
	replyLimit := ratelimit.PerUser("reply", 30, time.Minute) // 回信 30次/分

	user.NewHandler(userSvc).Register(api, auth)
	wallet.NewHandler(walletSvc).Register(api, auth)
	pay.NewHandler(paySvc).Register(api, auth)
	bottle.NewHandler(bottleSvc).Register(api, auth, throwLimit, replyLimit)
	match.NewHandler(matchSvc).Register(api, auth)
	// 发现页「按语言/兴趣/地理找人」:候选集是用户池,与 match 的瓶子池是两回事
	discover.NewHandler(discover.New(db, walletSvc)).Register(api, auth)
	relation.NewHandler(relationSvc).Register(api, auth)
	item.NewHandler(itemSvc).Register(api, auth)
	moderation.NewHandler(modSvc).Register(api, auth)
	notify.NewHandler(notifySvc).Register(api, auth)
	collectionSvc := collection.New(db)
	collection.NewHandler(collectionSvc).Register(api, auth)
	momentSvc := moment.New(db, modSvc, walletSvc)
	momentSvc.OnCommented = func(tenantID, authorID, momentID, commenterID int64) {
		notifySvc.Create(tenantID, authorID, "moment_comment", momentID, "收到新评论 💬", "有人评论了你的动态,去看看吧")
	}
	momentSvc.OnGifted = func(tenantID, authorID, momentID, senderID int64, itemName string) {
		notifySvc.Create(tenantID, authorID, "moment_gift", momentID, "收到礼物 🎁", "有人在你的动态送出「"+itemName+"」,魅力值+了")
	}
	moment.NewHandler(momentSvc).Register(api, auth, replyLimit)
	chatHandler := chat.NewHandler(chatSvc)
	chatHandler.Register(api, auth)
	chatHandler.RegisterWS(r, "/ws") // WebSocket:/ws?token=xxx

	pushSvc := push.New(db)
	pushSvc.SetCredStore(credStore) // access_token 凭证与登录同源,多租户免重复配置
	push.NewHandler(pushSvc, cfg.DefaultTenantID).Register(api, auth)
	// 主动匹配(火花):在线真人定时被配一个人,App 弹窗;总开关 spark_enabled 默认关
	sparkDeps := spark.Deps{
		DB:         db,
		Online:     hub.OnlineUserIDs,
		Push:       func(userID int64, payload any) { hub.PushTo(userID, payload) },
		EnsureChat: chatSvc.EnsureFreeChat,
		SendAs:     chatSvc.SendMessage,
		Opening:    robotSvc.Opening,
		ClaimDaily: robot.ClaimDailyReach,
		IsBlocked:  modSvc.IsBlocked,
	}
	sparkSvc := spark.New(sparkDeps)
	spark.NewHandler(sparkSvc).Register(api, auth)
	spark.Start(sparkDeps, cfg.DefaultTenantID)
	push.StartCheckinCron(pushSvc)
	push.StartNightCron(pushSvc)
	checkin.NewHandler(checkin.New(db)).Register(api, auth)
	rank.NewHandler(rank.New(db)).Register(api, auth)
	geo.NewHandler(providerStore).Register(api, auth)
	share.NewHandler(share.New(db, walletSvc)).Register(api, auth)
	ad.NewHandler(ad.New(db, walletSvc)).Register(api, auth)
	chatSvc.SetOnMsgPush(func(tenantID, recipientID int64) {
		pushSvc.NotifyNewMessage(tenantID, recipientID)
	})
	sysconfig.NewHandler(cfg.DefaultTenantID, cfg.AppDefaultTenantID).
		WithCredLookup(func(tid int64, platform string) (string, bool) {
			r, ok := credStore.ByTenantPlatform(tid, platform)
			if !ok {
				return "", false
			}
			return r.AppID, true
		}).
		WithPayUsable(func(tid int64, p string) bool { return providerStore.Usable(tid, provider.KindPay, p) }).
		WithMapProvider(func(tid int64) string {
			if r, ok := providerStore.Active(tid, provider.KindMap); ok {
				return r.Provider
			}
			return ""
		}).
		WithSSOUsable(func(tid int64, p string) bool { return providerStore.Usable(tid, provider.KindSSO, p) }).
		WithSSOField(func(tid int64, p, k string) string {
			row, ok := providerStore.Get(tid, provider.KindSSO, p)
			if !ok {
				return ""
			}
			return row.Get(k)
		}).
		Register(api)

	// 管理后台 API(/admin/api/*,独立鉴权;Vue SPA 由 admin/ 单独构建部署)
	adminSvc := admin.New(db, credStore)
	adminSvc.SetProviderStore(providerStore)
	adminSvc.SetProbe(func(tid int64, kind, prov string) (string, error) {
		switch kind {
		case provider.KindPay:
			return paySvc.Probe(tid, prov)
		case provider.KindMap:
			row, ok := providerStore.Get(tid, provider.KindMap, prov)
			if !ok {
				return "", fmt.Errorf("未配置")
			}
			g, ok := geo.GeocoderFor(row)
			if !ok {
				return "", fmt.Errorf("未知服务商")
			}
			res, err := g.Regeo("39.9087", "116.3975", "zh-CN")
			if err != nil {
				return "", err
			}
			return res.Address, nil
		case provider.KindModeration:
			if err := modSvc.ProbeText(tid); err != nil {
				return "", err
			}
			return "接口可用", nil
		case provider.KindSSO:
			// SSO 没有「调一个接口就能验」的探活:微信要用户授权码、
			// Google / Apple 验的是客户端传来的 token。只回报配置是否齐全。
			if providerStore.Usable(tid, provider.KindSSO, prov) {
				return "配置齐全;实际可用性要用 App 真实登录一次验证", nil
			}
			return "", fmt.Errorf("配置不齐全")
		}
		return "", fmt.Errorf("未知领域")
	})
	adminSvc.SetChatService(chatSvc)
	adminSvc.SetPushService(pushSvc)
	adminSvc.SetWalletService(walletSvc)
	adminHandler := admin.NewHandler(adminSvc)
	adminHandler.SetUpload(cfg.UploadDir, cfg.PublicBaseURL)
	adminHandler.Register(r)

	// 本地图片上传 + 静态服务(联调临时方案)
	uploadHandler := upload.NewHandler(cfg.UploadDir, cfg.PublicBaseURL)
	uploadHandler.OnUploaded = modSvc.CheckImageAsync // 图片内容安全(mediaCheckAsync,开关控制)
	uploadHandler.Register(api, auth)
	r.Static(upload.StaticMount(), cfg.UploadDir) // GET /static/<day>/<file>

	// 外部接口调用日志(后台「接口日志」页数据源;保留 7 天)
	apilog.Init(db)
	apilog.CleanupBefore(7 * 24 * time.Hour)

	// 邮件记录同样只留 7 天——那张表存着**明文验证码**,
	// 留得越久,后台一旦被攻破能一次性拿到的账号就越多。
	go func() {
		db.Where("created_at < ?", time.Now().Add(-7*24*time.Hour)).Delete(&model.EmailLog{})
	}()

	// 微信内容安全装配:token 复用 push 缓存;资料文本(昵称/签名)走资料场景检测
	modSvc.SetWxSource(pushSvc.AccessToken, cfg.UploadDir, cfg.PublicBaseURL)
	userSvc.CheckProfileText = func(tenantID, userID int64, text string) error {
		return modSvc.CheckUGC(tenantID, userID, moderation.SceneProfile, text)
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	addr := ":" + cfg.Port
	log.Printf("漂流瓶后端启动于 %s (env=%s)", addr, cfg.Env)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
