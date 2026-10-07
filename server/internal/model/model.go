package model

import "time"

// ============ 租户 / 凭证(SaaS 多租户)============

// Tenant 租户(一个产品/客户)。
// 租户形态。决定后台要不要向它索取微信凭证,也是将来按端选能力(如内容审核)的依据。
const (
	TenantTypeMiniProgram = "miniprogram"
	TenantTypeApp         = "app"
)

type Tenant struct {
	TenantID int64  `gorm:"primaryKey" json:"tenant_id,string"`
	Name     string `gorm:"size:64" json:"name"`
	Status   string `gorm:"size:16;default:active" json:"status"`
	// Type 小程序 / App。
	//
	// `not null + default` 是有意的:AutoMigrate 加这一列时 MySQL 会把存量行
	// 一并回填成 miniprogram,不需要迁移脚本——而现状正是"除 App 测试租户外全是小程序"。
	Type      string    `gorm:"size:16;not null;default:miniprogram" json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// AppCredential 一行 = 一个小程序应用(微信或支付宝)的凭证。
// 敏感字段(*_enc)以 AES-GCM 加密存储。
type AppCredential struct {
	ID                  int64     `gorm:"primaryKey" json:"id,string"`
	TenantID            int64     `gorm:"index" json:"tenant_id,string"`
	Platform            string    `gorm:"size:16;uniqueIndex:uk_plat_appid" json:"platform"` // wx/alipay/app/wx_app/alipay_app;从 size:8 加宽,alipay_app 10 字符会被静默截断
	AppID               string    `gorm:"size:64;uniqueIndex:uk_plat_appid" json:"appid"`
	SecretEnc           string    `gorm:"size:512" json:"-"` // 登录密钥(密文)
	MchID               string    `gorm:"size:32" json:"mch_id"`
	PayAPIv3KeyEnc      string    `gorm:"size:512" json:"-"`
	PaySerialNo         string    `gorm:"size:128" json:"pay_serial_no"`
	PayPrivateKeyEnc    string    `gorm:"type:text" json:"-"`
	PayPlatformKey      string    `gorm:"type:text" json:"-"`      // 平台公钥(公开)
	PayPlatformSerial   string    `gorm:"size:128" json:"pay_platform_serial"`
	AlipayPrivateKeyEnc string    `gorm:"type:text" json:"-"`
	AlipayPublicKey     string    `gorm:"type:text" json:"-"`      // 公开
	NotifyURL           string    `gorm:"size:255" json:"notify_url"`
	Status              string    `gorm:"size:16;default:active" json:"status"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ProviderConfig 一租户 × 一领域(支付/地图/内容安全) × 一服务商 一行。
//
// 字段整包加密存 JSON:机密与非机密混在一个 JSON 里,省得两列对账;
// LookupA/B 是 schema 指定的两个字段的明文投影,给回调反查租户用(微信 mch_id+platform_serial、支付宝 app_id)。
type ProviderConfig struct {
	ID        int64     `gorm:"primaryKey" json:"id,string"`
	TenantID  int64     `gorm:"uniqueIndex:uk_tenant_kind_provider,priority:1;index" json:"tenant_id,string"`
	Kind      string    `gorm:"size:16;uniqueIndex:uk_tenant_kind_provider,priority:2" json:"kind"`
	Provider  string    `gorm:"size:16;uniqueIndex:uk_tenant_kind_provider,priority:3" json:"provider"`
	Enabled   bool      `json:"enabled"`
	Active    bool      `gorm:"index" json:"active"` // 单选领域「当前用谁」;支付领域恒 false
	FieldsEnc string    `gorm:"type:text" json:"-"`
	LookupA   string    `gorm:"size:128;index" json:"-"`
	LookupB   string    `gorm:"size:128;index" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============ 用户 / 关系 ============

// TagPaidUser 充值成功后自动打上的系统标签。
const TagPaidUser = "付费用户"

// User 用户表(双平台身份合一)。
type User struct {
	UserID int64 `gorm:"primaryKey" json:"user_id,string"`
	// TenantID 带上 google_sub / apple_sub 两个唯一索引的首位:
	// 第三方身份的唯一性是**租户内**唯一,不是全局唯一。
	TenantID       int64     `gorm:"index;uniqueIndex:uk_tenant_google,priority:1;uniqueIndex:uk_tenant_apple,priority:1" json:"tenant_id,string"`
	WxOpenID       string    `gorm:"column:wx_openid;size:64;index" json:"-"`
	AlipayUID      string    `gorm:"column:alipay_uid;size:64;index" json:"-"`
	UnionID        string    `gorm:"column:union_id;size:64;index" json:"-"`
	Nickname       string    `gorm:"size:32" json:"nickname"`
	Avatar         string    `gorm:"size:255" json:"avatar"`
	Bio            string    `gorm:"size:200" json:"bio"`
	Gender         int8      `json:"gender"` // 0未知 1男 2女
	Age            int       `json:"age"`
	Birthday       string    `gorm:"size:10" json:"birthday"` // YYYY-MM-DD;App 资料页填,服务端据此算 Age
	City           string    `gorm:"size:32" json:"city"`
	IsVerified     bool      `json:"is_verified"`
	IsRobot        bool      `gorm:"index" json:"is_robot"`
	IsMuted        bool      `gorm:"default:false" json:"is_muted"`
	PersonaID      *int64    `gorm:"index" json:"persona_id,string"`
	AnonymousLevel int8      `gorm:"default:1" json:"anonymous_level"`
	Charm          int64     `gorm:"default:0" json:"charm"` // 魅力值:收到礼物累加(+礼物金币数)
	Tags           string    `gorm:"size:255" json:"-"` // 逗号分隔,仅后台管理用(含系统自动标签"付费用户"),不下发 C 端
	Status         string    `gorm:"size:16;default:active" json:"status"` // active/frozen/banned/deleted
	CreatedAt      time.Time `json:"created_at"`
	LastActiveAt   time.Time `json:"last_active_at"`
	LastLoginAt    time.Time `json:"last_login_at"`

	// ---- App 端新增(小程序不使用,AutoMigrate 自动加列)----

	// Phone E.164 格式(+91xxxxxxxxxx)。账号删除时置空以释放,可被重新注册。
	Phone     string `gorm:"size:24;index" json:"-"`
	Email     string `gorm:"size:128;index" json:"-"` // 统一小写存储,与 Phone 同为可释放标识

	// PasswordHash bcrypt 密文,永不下发(json:"-")。
	// 空 = 该账号没设过密码(验证码注册的老用户),只能走验证码登录或「忘记密码」补设。
	PasswordHash string `gorm:"size:72" json:"-"`
	// GoogleSub / AppleSub 第三方身份标识。
	//
	// 必须可空 + 唯一：没有唯一约束时,两个账号可以绑同一个 Google 身份,
	// 之后按 sub 查用户会返回不确定的那一个——绑定按钮连点两次即可触发。
	// 未绑定存 NULL 而非 '':MySQL 唯一索引允许多行 NULL,不允许多行 ''。
	// 赋值一律走 user.nilIfEmpty;注销时也必须置 NULL(见 user/account.go)。
	GoogleSub *string `gorm:"size:64;uniqueIndex:uk_tenant_google,priority:2" json:"-"`
	AppleSub  *string `gorm:"size:64;uniqueIndex:uk_tenant_apple,priority:2" json:"-"`

	// Language/Interests 逗号分隔,发现页按它们算匹配度。
	Language  string `gorm:"size:64" json:"language"`
	Interests string `gorm:"size:255" json:"interests"`
	// 答题匹配的答案:紧凑串 qkey:optIndex 逗号分隔(如 "weekend:0,night:1")。AutoMigrate 自动加列。
	QuizAnswers string `gorm:"size:255" json:"quiz_answers"`

	// Lat/Lng 最后一次上报的位置。复合索引供发现页做 bounding box 预筛,
	// 直接 WHERE 算距离会全表扫。
	Lat float64 `gorm:"index:idx_user_geo,priority:1" json:"-"`
	Lng float64 `gorm:"index:idx_user_geo,priority:2" json:"-"`

	// DeletedAt 账号删除时间。
	// ⚠️ 必须是 *time.Time 而非 gorm.DeletedAt——后者会开启 GORM 全局软删除,
	// 静默改变现有所有查询的行为。账号是否有效一律以 Status 判断。
	DeletedAt *time.Time `json:"-"`
}

// Relation 关系表(喜欢/看过/关系强度)。
type Relation struct {
	RelationID        int64     `gorm:"primaryKey" json:"relation_id,string"`
	TenantID          int64     `gorm:"index" json:"tenant_id,string"`
	UserA             int64     `gorm:"index:idx_a" json:"user_a,string"`
	UserB             int64     `gorm:"index:idx_b" json:"user_b,string"`
	Type              string    `gorm:"size:16" json:"type"` // like/viewed/friend
	StrengthScore     int       `json:"strength_score"`      // 0~100
	Stage             string    `gorm:"size:16;default:stranger" json:"stage"`
	InteractionCount  int       `json:"interaction_count"`
	LastInteractionAt time.Time `json:"last_interaction_at"`
	CreatedAt         time.Time `json:"created_at"`
}

// Block 拉黑表。
type Block struct {
	ID        int64     `gorm:"primaryKey" json:"id,string"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	UserID    int64     `gorm:"index:idx_blocker" json:"user_id,string"`
	TargetID  int64     `gorm:"index" json:"target_id,string"`
	CreatedAt time.Time `json:"created_at"`
}

// ============ 漂流瓶 ============

// Bottle 漂流瓶。
type Bottle struct {
	BottleID    int64     `gorm:"primaryKey" json:"bottle_id,string"`
	TenantID    int64     `gorm:"index:idx_tenant_status" json:"tenant_id,string"`
	UserID      int64     `gorm:"index" json:"user_id,string"`
	Content     string    `gorm:"type:text" json:"content"`
	ContentType string    `gorm:"size:16;default:text" json:"content_type"` // text/audio/image
	MediaURL    string    `gorm:"size:255" json:"media_url"`
	Tags        string    `gorm:"size:128" json:"tags"` // 逗号分隔,V1 简化
	IsAnonymous bool      `gorm:"default:true" json:"is_anonymous"`
	Scope       string    `gorm:"size:16;default:national" json:"scope"` // local/national
	City        string    `gorm:"size:32" json:"city"`
	// Lat/Lng 发瓶地点。⚠️ 存量瓶子为 0,用 geodist.HasFix 判空后再算距离——
	// (0,0) 是几内亚湾一个真实坐标,不判空老瓶子会被算成在西非。
	// 与 User.Lat/Lng 一样标 json:"-",对外只给 distance_km。
	Lat float64 `gorm:"index:idx_bottle_geo,priority:1" json:"-"`
	Lng float64 `gorm:"index:idx_bottle_geo,priority:2" json:"-"`
	// PlaceName 短地名("Bandra West"),展示用。空=发瓶时没带地点。
	PlaceName string `gorm:"size:64" json:"place_name,omitempty"`
	Status      string    `gorm:"size:16;default:active;index:idx_status_exp" json:"status"` // active/expired/deleted
	HeatScore   int       `gorm:"index" json:"heat_score"`
	ReplyCount  int       `json:"reply_count"`
	LikeCount   int       `json:"like_count"`
	CreatedAt   time.Time `json:"created_at"`
	ExpireAt    time.Time `gorm:"index:idx_status_exp" json:"expire_at"`
	// 非持久:展示用作者信息(#6,不管匿名都显示性别/年龄)
	AuthorGender int8 `gorm:"-" json:"author_gender"`
	AuthorAge    int  `gorm:"-" json:"author_age"`
}

// ReplyUnlock 回信解锁记录(#4:任何人付费即可看,按 viewer 维度记录)。
type ReplyUnlock struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	ReplyID   int64     `gorm:"index:idx_reply_viewer" json:"reply_id,string"`
	ViewerID  int64     `gorm:"index:idx_reply_viewer" json:"viewer_id,string"`
	CreatedAt time.Time `json:"created_at"`
}

// MatchLog 捞瓶行为 / 推荐打分记录。
type MatchLog struct {
	MatchID    int64     `gorm:"primaryKey" json:"match_id,string"`
	TenantID   int64     `gorm:"index" json:"tenant_id,string"`
	BottleID   int64     `gorm:"index" json:"bottle_id,string"`
	ViewerID   int64     `gorm:"index:idx_viewer" json:"viewer_id,string"`
	Action     string    `gorm:"size:16" json:"action"` // view/like/reply/skip
	Score      int       `json:"score"`
	MatchType  string    `gorm:"size:16" json:"match_type"` // strong/weak/random
	CreatedAt  time.Time `gorm:"index:idx_viewer" json:"created_at"`
}

// BottleReply 对瓶子的回应(默认仅作者可见,可花金币解锁)。
type BottleReply struct {
	ReplyID    int64     `gorm:"primaryKey" json:"reply_id,string"`
	TenantID   int64     `gorm:"index" json:"tenant_id,string"`
	BottleID   int64     `gorm:"index" json:"bottle_id,string"`
	UserID     int64     `gorm:"index" json:"user_id,string"`
	Content    string    `gorm:"type:text" json:"content"`
	IsUnlocked bool      `json:"is_unlocked"` // 瓶主是否已解锁查看
	CreatedAt  time.Time `json:"created_at"`
}

// ============ 聊天 ============

// Chat 会话。
type Chat struct {
	ChatID        int64     `gorm:"primaryKey" json:"chat_id,string"`
	TenantID      int64     `gorm:"uniqueIndex:uk_chat_pair,priority:1" json:"tenant_id,string"`
	UserA         int64     `gorm:"index:idx_ua;uniqueIndex:uk_chat_pair,priority:2" json:"user_a,string"`
	UserB         int64     `gorm:"index:idx_ub;uniqueIndex:uk_chat_pair,priority:3" json:"user_b,string"`
	SourceBottle  int64     `json:"source_bottle_id,string"`
	RelationStage string    `gorm:"size:16;default:stranger" json:"relation_stage"`
	LastMessage   string    `gorm:"size:255" json:"last_message"`
	// LastType 最后一条消息的类型(text/image/gift/system)。
	// 没有它,图片消息在会话列表里会显示成一串原始 URL——LastMessage 存的就是 content。
	LastType  string    `gorm:"size:16;default:text" json:"last_type"`
	UpdatedAt time.Time `gorm:"index" json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Message 消息。
type Message struct {
	MessageID  int64     `gorm:"primaryKey" json:"message_id,string"`
	TenantID   int64     `gorm:"index" json:"tenant_id,string"`
	ChatID     int64     `gorm:"index:idx_chat_time" json:"chat_id,string"`
	SenderID   int64     `json:"sender_id,string"`
	Content    string    `gorm:"type:text" json:"content"`
	Type       string    `gorm:"size:16;default:text" json:"type"` // text/audio/image/gift/system
	ReadStatus bool      `json:"read_status"`
	CreatedAt  time.Time `gorm:"index:idx_chat_time" json:"created_at"`
}

// ============ 钱包 / 支付 / 道具 ============

// Wallet 金币钱包(一人一行)。
type Wallet struct {
	UserID         int64     `gorm:"primaryKey" json:"user_id,string"`
	TenantID       int64     `gorm:"index" json:"tenant_id,string"`
	Balance        int64     `gorm:"default:0" json:"balance"` // 金币余额
	TotalRecharged int64     `gorm:"default:0" json:"total_recharged"`
	TotalSpent     int64     `gorm:"default:0" json:"total_spent"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WalletTxn 钱包流水(可对账)。
type WalletTxn struct {
	TxnID        int64     `gorm:"primaryKey" json:"txn_id,string"`
	TenantID     int64     `gorm:"index" json:"tenant_id,string"`
	UserID       int64     `gorm:"index:idx_user_time" json:"user_id,string"`
	Direction    string    `gorm:"size:8" json:"direction"` // credit/debit
	Coins        int64     `json:"coins"`
	Scene        string    `gorm:"size:24" json:"scene"` // recharge/chat/unlock/gift/reward
	BizNo        string    `gorm:"size:64;index" json:"biz_no"`
	BalanceAfter int64     `json:"balance_after"`
	Remark       string    `gorm:"size:128" json:"remark"` // 后台人工调账的备注,业务流水为空
	CreatedAt    time.Time `gorm:"index:idx_user_time" json:"created_at"`
}

// CoinPackage 充值档位。
type CoinPackage struct {
	PackageID  int64  `gorm:"primaryKey" json:"package_id"`
	Name       string `gorm:"size:32" json:"name"`
	Coins      int64  `json:"coins"`
	BonusCoins int64  `json:"bonus_coins"`
	PriceFen   int64  `json:"price_fen"` // 单位:分
	Status     string `gorm:"size:16;default:active" json:"status"`
	Sort       int    `json:"sort"`

	// IOSProductID App Store Connect 里配置的商品 ID。
	// iOS 的价格由商店本地化下发,不用这里的 PriceFen；两端定价本就不同
	// (苹果 30% 抽成要在定价里吃掉),所以档位必须能按平台区分。
	IOSProductID string `gorm:"size:64;index" json:"ios_product_id"`

	// PlayProductID Google Play Console 里配置的商品 ID。与 IOSProductID 对称。
	// 商品 ID 是**全局唯一、多国共用一个**的,随国家变的只是价格——
	// 所以它属于档位,价格属于 CoinPackagePrice。
	PlayProductID string `gorm:"size:64;index" json:"play_product_id"`
}

// CoinPackagePrice 档位的多平台多地区定价。
//
// 为什么要 platform 维度:上面 IOSProductID 的注释写过「两端定价本就不同
// (苹果 30% 抽成要在定价里吃掉)」。为什么要 region 维度:印度先上,
// 但结构要能容纳多市场。Region 用 ISO 国家码,"*" 为兜底行。
//
// CoinPackage.PriceFen 保留为兜底,等这张表有了 "*" 行之后可以废弃——
// 不直接删是给存量数据和后台页面一个过渡期。
type CoinPackagePrice struct {
	ID        int64  `gorm:"primaryKey;autoIncrement" json:"id,string"`
	PackageID int64  `gorm:"uniqueIndex:uk_pkg_plat_region,priority:1" json:"package_id"`
	Platform  string `gorm:"size:8;uniqueIndex:uk_pkg_plat_region,priority:2" json:"platform"` // gplay/ios
	Region    string `gorm:"size:8;uniqueIndex:uk_pkg_plat_region,priority:3" json:"region"`   // IN/US/*
	Currency  string `gorm:"size:8" json:"currency"`
	Amount    int64  `json:"amount"` // 最小货币单位
}

// PlayPurchase Google Play 购买记录,对称 IAPTransaction。
type PlayPurchase struct {
	ID       int64 `gorm:"primaryKey;autoIncrement" json:"id,string"`
	TenantID int64 `gorm:"index" json:"tenant_id,string"`
	UserID   int64 `gorm:"index" json:"user_id,string"`
	// OrderID Play 的 GPA.xxxx,幂等键。
	// App 冷启动时 queryPurchasesAsync 会把未 consume 的交易重新推上来,
	// 不幂等就会一笔钱发两次币。
	OrderID string `gorm:"size:64;uniqueIndex" json:"order_id"`
	// PurchaseToken 不下发:凭它可向 Google 查询该笔购买的全部详情。
	PurchaseToken string     `gorm:"size:512;index" json:"-"`
	ProductID     string     `gorm:"size:64" json:"product_id"`
	OrderNo       string     `gorm:"size:32;index" json:"order_no"` // 关联 PayOrder
	Coins         int64      `json:"coins"`
	State         string     `gorm:"size:16" json:"state"` // pending/purchased/refunded
	Currency      string     `gorm:"size:8" json:"currency"`
	AmountMinor   int64      `json:"amount_minor"` // 留档对账用
	AckedAt       *time.Time `json:"acked_at"`
	RefundedAt    *time.Time `json:"refunded_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// PayOrder 充值订单(回调幂等的依据)。
type PayOrder struct {
	OrderNo      string     `gorm:"primaryKey;size:32" json:"order_no"`
	TenantID     int64      `gorm:"index" json:"tenant_id,string"`
	UserID       int64      `gorm:"index:idx_user_status" json:"user_id,string"`
	PackageID int64 `json:"package_id"`
	// Platform 从 size:8 加宽:塞 google_play(11 字符) 会被**静默截断**。
	// 实际用短码 gplay,加宽是为了以后再加渠道时不用再改一次。
	Platform string `gorm:"size:16" json:"platform"` // wx/alipay/ios/gplay
	// PriceMinor 原名 PriceFen。改名是因为语义不再是「分」——
	// Play / App Store 结算的是各国本地货币,单位是该货币的最小单位
	// (INR 的 paise、USD 的 cent)。留着 Fen 这个名字,下一个人会理所当然地
	// 按人民币去算,而且**错得很安静**。
	//
	// ⚠️ 列名与 JSON 名都**保留 price_fen**,只改 Go 字段名:
	//   - 列名:改要写数据迁移,而列名不会误导读代码的人
	//   - JSON 名:线上微信小程序的订单页(client/src/pages/orders/orders.vue)
	//     直接读 o.price_fen。改了它,小程序订单列表立刻显示 ¥NaN,
	//     而小程序发版要过微信审核,无法与后端同步上线。
	//
	// 真正误导人的是 Go 侧的字段名,那个已经改掉了。JSON 名要改,
	// 得等一个能与小程序发版协同的窗口,或先并行下发两个字段过渡。
	PriceMinor int64 `gorm:"column:price_fen" json:"price_fen"`
	// Currency ISO 货币码。空 = CNY(改名之前的历史数据)。
	Currency string `gorm:"size:8" json:"currency"`
	Coins        int64      `json:"coins"` // 含赠送,支付成功后入账的总金币
	Status       string     `gorm:"size:16;default:pending;index:idx_user_status" json:"status"` // pending/paid/failed/refunded
	PlatformTxn  string     `gorm:"column:platform_txn_id;size:64" json:"platform_txn_id"`
	CreatedAt    time.Time  `json:"created_at"`
	PaidAt       *time.Time `json:"paid_at"`
}

// Item 增值道具/礼物目录。
type Item struct {
	ItemID    int64  `gorm:"primaryKey" json:"item_id"`
	Name      string `gorm:"size:32" json:"name"`
	// Type boost/top/superlike/gift/quota_throw/quota_scoop。
	// 后两个是次数包,买下后由 item.Buy 调 quota.AddPack 发次数,不是实物道具。
	Type string `gorm:"size:16" json:"type"`
	Icon      string `gorm:"size:255" json:"icon"`
	PriceCoin int64  `json:"price_coin"` // 金币价格
	Status    string `gorm:"size:16;default:active" json:"status"`
	Sort      int    `json:"sort"`
}

// ItemOrder 道具购买/赠送记录。
type ItemOrder struct {
	ID        int64     `gorm:"primaryKey" json:"id,string"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	UserID    int64     `gorm:"index" json:"user_id,string"`
	ItemID    int64     `json:"item_id"`
	TargetID  int64     `json:"target_id,string"` // 赠送对象(可空)
	Coins     int64     `json:"coins"`
	CreatedAt time.Time `json:"created_at"`
}

// ============ 风控 / 配置 ============

// ApiCallLog 外部接口调用日志(微信内容安全/订阅消息/逆地理等),供后台排查"有没有调、调了返回什么"。
type ApiCallLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	Kind      string    `gorm:"size:32;index" json:"kind"` // msg_sec_check/media_check_async/media_check_callback/subscribe_send/access_token/geo_regeo
	Detail    string    `gorm:"size:512" json:"detail"`    // 请求摘要(内容截断/URL/openid 等)
	RespCode  int       `json:"resp_code"`                 // 微信 errcode 或 HTTP 状态
	RespBody  string    `gorm:"size:512" json:"resp_body"` // 响应摘要(截断)
	OK        bool      `json:"ok"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// WxMediaCheck 微信图片异步检测记录(media_check_async):回调按 trace_id 回填结果,risky 删文件。
type WxMediaCheck struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	UserID    int64     `json:"user_id,string"`
	TraceID   string    `gorm:"size:128;uniqueIndex" json:"trace_id"`
	URL       string    `gorm:"size:255" json:"url"`
	Status    string    `gorm:"size:16;default:pending" json:"status"` // pending/pass/risky
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Report 举报表。
type Report struct {
	ReportID   int64     `gorm:"primaryKey" json:"report_id,string"`
	TenantID   int64     `gorm:"index" json:"tenant_id,string"`
	ReporterID int64     `gorm:"index" json:"reporter_id,string"`
	TargetID   int64     `json:"target_id,string"`
	TargetType string    `gorm:"size:16" json:"target_type"` // bottle/user/message
	Reason     string    `gorm:"size:128" json:"reason"`
	Status     string    `gorm:"size:16;default:pending" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Config 后台配置(价格/开关,运营热调)。tenant_id=0 为全局默认,非0为租户覆盖。
type Config struct {
	TenantID  int64     `gorm:"primaryKey" json:"tenant_id,string"`
	Key       string    `gorm:"primaryKey;size:48" json:"key"`
	// longtext:用户协议 / 隐私政策正文(中英文各一份)走这张表,255 存不下一段话;
	// AutoMigrate 会把已有列改成 longtext,老数据不动。
	Value     string    `gorm:"type:longtext" json:"value"`
	Remark    string    `gorm:"size:128" json:"remark"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Notification 站内通知(#4):别人回信/点赞等互动提醒。
type Notification struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	UserID    int64     `gorm:"index:idx_user_read" json:"user_id,string"` // 接收者
	Type      string    `gorm:"size:16" json:"type"`                       // reply/like/system
	RefID     int64     `json:"ref_id,string"`                             // 关联瓶子/资源 ID
	Title     string    `gorm:"size:64" json:"title"`
	Body      string    `gorm:"size:255" json:"body"`
	Read      bool      `gorm:"column:is_read;index:idx_user_read" json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// ============ AI 人格化对话引擎 ============

// PersonaConfig 人格配置模板；多个机器人可共用同一模板。
type PersonaConfig struct {
	PersonaID        int64     `gorm:"primaryKey" json:"persona_id,string"`
	TenantID         int64     `gorm:"index" json:"tenant_id,string"`
	BotUserID        *int64    `gorm:"index" json:"bot_user_id,string"` // NULL=模板，非 NULL=绑定具体机器人
	Name             string    `gorm:"size:64" json:"name"`
	RelationshipRole string    `gorm:"size:32" json:"relationship_role"` // friend/partner/companion/mentor
	AffectiveStyle   string    `gorm:"size:32" json:"affective_style"`   // warm_soft/calm/energetic/playful/dominant
	VoiceStyle       string    `gorm:"size:32" json:"voice_style"`       // short_sentence/casual/structured/expressive
	RulesJSON        string    `gorm:"type:text" json:"rules_json"`      // {"do":["..."],"dont":["..."]}
	Status           string    `gorm:"size:16;default:active" json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RobotMemory 用户×机器人关系记忆；同一用户对不同机器人独立记录。
type RobotMemory struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id,string"`
	TenantID        int64     `gorm:"index" json:"tenant_id,string"`
	UserID          int64     `gorm:"uniqueIndex:uk_user_bot" json:"user_id,string"`
	BotUserID       int64     `gorm:"uniqueIndex:uk_user_bot" json:"bot_user_id,string"`
	Familiarity     float64   `gorm:"default:0.3" json:"familiarity"` // 0~1，驱动语气亲密度
	PreferencesJSON string    `gorm:"type:text" json:"preferences_json"`
	SessionSummary  string    `gorm:"type:text" json:"session_summary"` // 上次会话压缩摘要
	UpdatedAt       time.Time `json:"updated_at"`
}

// RobotKeywordRule Tier1 关键字规则，Admin 预设，命中直接返回，不调 LLM。
type RobotKeywordRule struct {
	RuleID        int64     `gorm:"primaryKey" json:"rule_id,string"`
	TenantID      int64     `gorm:"index:idx_tenant_status" json:"tenant_id,string"`
	Priority      int       `gorm:"default:0" json:"priority"`
	MatchType     string    `gorm:"size:16" json:"match_type"`    // contains/exact/prefix
	KeywordsJSON  string    `gorm:"type:text" json:"keywords_json"`  // ["你好","hi","hello"]
	ResponsesJSON string    `gorm:"type:text" json:"responses_json"` // {"warm_soft":["变体A"],"default":["通用"]}
	HitCount      int       `gorm:"default:0" json:"hit_count"`
	Status        string    `gorm:"size:16;default:active;index:idx_tenant_status" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RobotReplyCache Tier2 LLM 自积累缓存；每次 LLM 生成后异步写入。
type RobotReplyCache struct {
	CacheID        int64     `gorm:"primaryKey" json:"cache_id,string"`
	TenantID       int64     `gorm:"uniqueIndex:uk_hash_role" json:"tenant_id,string"`
	PersonaRole    string    `gorm:"size:32;uniqueIndex:uk_hash_role" json:"persona_role"`
	QuestionHash   string    `gorm:"size:16;uniqueIndex:uk_hash_role" json:"question_hash"` // SHA256[:8] hex，16 chars
	QuestionSample string    `gorm:"size:200" json:"question_sample"`
	ResponsesJSON  string    `gorm:"type:text" json:"responses_json"` // 最多 5 条变体
	HitCount       int       `gorm:"index" json:"hit_count"`
	Source         string    `gorm:"size:16;default:ai_generated" json:"source"` // ai_generated/manual
	Status         string    `gorm:"size:16;default:active" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RobotContent 机器人内容池(#1):投放瓶子/回信时随机取用。
type RobotContent struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	Type      string    `gorm:"size:16;index" json:"type"` // bottle / reply
	Text      string    `gorm:"type:text" json:"text"`
	Tags      string    `gorm:"size:128" json:"tags"` // 逗号分隔
	Weight    int       `gorm:"default:1" json:"weight"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminUser 管理后台账号(与 C 端 User 完全隔离)。
type AdminUser struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string    `gorm:"size:32;uniqueIndex" json:"username"`
	PwdHash   string    `gorm:"size:100" json:"-"`
	Role      string    `gorm:"size:16;default:admin" json:"role"` // admin/super
	Status    string    `gorm:"size:16;default:active" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	LastLogin time.Time `json:"last_login"`
}

// Collection 我的收藏(瓶子等)。
type Collection struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id,string"`
	TenantID   int64     `gorm:"index" json:"tenant_id,string"`
	UserID     int64     `gorm:"index:idx_user_target" json:"user_id,string"`
	TargetType string    `gorm:"size:16;index:idx_user_target" json:"target_type"` // bottle
	TargetID   int64     `gorm:"index:idx_user_target" json:"target_id,string"`
	CreatedAt  time.Time `json:"created_at"`
}

// CheckinLog 每日签到记录(唯一索引保证一人一天一次，资金可对账)。
type CheckinLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	UserID    int64     `gorm:"uniqueIndex:uk_user_date" json:"user_id,string"`
	Date      string    `gorm:"size:8;uniqueIndex:uk_user_date" json:"date"` // 20060102
	Coins     int64     `json:"coins"`
	CreatedAt time.Time `json:"created_at"`
}

// PushSubscription 用户微信订阅消息授权记录。
type PushSubscription struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id,string"`
	TenantID   int64     `gorm:"index" json:"tenant_id,string"`
	UserID     int64     `gorm:"index:idx_push_user_scene" json:"user_id,string"`
	OpenID     string    `gorm:"size:64" json:"open_id"`
	TemplateID string    `gorm:"size:128" json:"template_id"`
	Scene      string    `gorm:"size:32;index:idx_push_user_scene" json:"scene"` // reply/chat/system
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Moment 用户动态(V1:文字，public=公开/self=仅自己可见)。
type Moment struct {
	MomentID     int64     `gorm:"primaryKey" json:"moment_id,string"`
	TenantID     int64     `gorm:"index" json:"tenant_id,string"`
	UserID       int64     `gorm:"index" json:"user_id,string"`
	Content      string    `gorm:"type:text" json:"content"`
	Images       string    `gorm:"size:2048" json:"images"` // JSON 数组存图 URL(≤9),空=纯文字
	Visible      string    `gorm:"size:10;default:public" json:"visible"`
	// 位置。Moment 原本一个位置字段都没有,四个一起加。
	// 口径与 Bottle 一致:存精确经纬度供算距离,对外只给 city / place_name。
	City      string  `gorm:"size:32" json:"city,omitempty"`
	Lat       float64 `gorm:"index:idx_moment_geo,priority:1" json:"-"`
	Lng       float64 `gorm:"index:idx_moment_geo,priority:2" json:"-"`
	PlaceName string  `gorm:"size:64" json:"place_name,omitempty"`
	LikeCount    int       `gorm:"default:0" json:"like_count"`
	CommentCount int       `gorm:"default:0" json:"comment_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// MomentComment 动态评论(单层,可回复某人)。type=gift 时 content 为 JSON{name,icon,coins}。
type MomentComment struct {
	CommentID     int64     `gorm:"primaryKey" json:"comment_id,string"`
	TenantID      int64     `gorm:"index" json:"tenant_id,string"`
	MomentID      int64     `gorm:"index" json:"moment_id,string"`
	UserID        int64     `json:"user_id,string"`
	ReplyToUserID int64     `json:"reply_to_user_id,string"` // 回复某人;0=直接评论动态
	Type          string    `gorm:"size:16;default:text" json:"type"` // text/gift
	Content       string    `gorm:"size:512" json:"content"`
	CreatedAt     time.Time `json:"created_at"`
}

// MomentLike 动态点赞(唯一索引防重复)。
type MomentLike struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	MomentID  int64     `gorm:"uniqueIndex:uk_moment_user" json:"moment_id,string"`
	UserID    int64     `gorm:"uniqueIndex:uk_moment_user" json:"user_id,string"`
	CreatedAt time.Time `json:"created_at"`
}

// AllModels 供 AutoMigrate 使用。
// ============ App 端(推送设备 / Apple 内购)============

// DeviceToken App 推送设备令牌。
//
// 微信订阅消息那套(模板 ID + PushSubscription.OpenID)在 App 端完全不可用,
// 这是 FCM / APNs 的落点。Token 唯一:同一台设备换账号登录时把绑定转移过去,
// 否则退出登录的人还会收到新用户的消息。
type DeviceToken struct {
	ID        int64     `gorm:"primaryKey" json:"id,string"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	UserID    int64     `gorm:"index" json:"user_id,string"`
	Platform  string    `gorm:"size:16" json:"platform"` // android/ios
	Provider  string    `gorm:"size:16" json:"provider"` // fcm/apns
	Token     string    `gorm:"size:255;uniqueIndex" json:"token"`
	Status    string    `gorm:"size:16;default:active" json:"status"` // active/revoked
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EmailLog 邮件发送记录。后台「邮件记录」页靠它回答两个问题:
// 「到底发没发出去」和「验证码是多少」。
//
// ⚠️ **Code 存的是明文验证码。** 这是产品要求(后台要能直接看到),
// 代价必须说清楚:任何有后台权限的人都能拿到任意账号的注册/重置验证码,
// 等于能接管那个账号。缓解只有两条——保留期短(7 天,与接口日志一致)、
// 后台账号本身要管好。真要消除这个风险,就得改成只记「发了没」不记码。
type EmailLog struct {
	ID       int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID int64  `gorm:"index" json:"tenant_id,string"`
	To       string `gorm:"size:128;index" json:"to"`
	Purpose  string `gorm:"size:16;index" json:"purpose"` // register/reset/login
	Code     string `gorm:"size:16" json:"code"`          // ⚠️ 明文
	Subject  string `gorm:"size:128" json:"subject"`
	// Status sent=真发出去了 / failed=发送失败(看 Error) / skipped=SMTP 没配,只落了日志
	Status    string    `gorm:"size:16;index" json:"status"`
	Error     string    `gorm:"size:512" json:"error"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// IAPTransaction Apple 内购交易。
//
// 主键用 Apple 的 transaction_id —— 这是**入账幂等的唯一依据**:
// 未 finish 的交易会在 App 冷启动时重新推给服务端,不幂等就会一笔钱发两次币。
type IAPTransaction struct {
	TransactionID         string     `gorm:"primaryKey;size:64" json:"transaction_id"`
	OriginalTransactionID string     `gorm:"size:64;index" json:"original_transaction_id"`
	TenantID              int64      `gorm:"index" json:"tenant_id,string"`
	UserID                int64      `gorm:"index" json:"user_id,string"`
	ProductID             string     `gorm:"size:64" json:"product_id"`
	OrderNo               string     `gorm:"size:32;index" json:"order_no"` // 关联 PayOrder
	Coins                 int64      `json:"coins"`
	Environment           string     `gorm:"size:16" json:"environment"`           // Production/Sandbox
	Status                string     `gorm:"size:16;default:paid" json:"status"`   // paid/refunded/revoked
	PurchasedAt           time.Time  `json:"purchased_at"`
	RefundedAt            *time.Time `json:"refunded_at"`
	CreatedAt             time.Time  `json:"created_at"`
}

func AllModels() []interface{} {
	return []interface{}{
		&Tenant{}, &AppCredential{}, &ProviderConfig{},
		&User{}, &Relation{}, &Block{},
		&Bottle{}, &MatchLog{}, &BottleReply{},
		&Chat{}, &Message{},
		&Wallet{}, &WalletTxn{}, &CoinPackage{}, &PayOrder{},
		&Item{}, &ItemOrder{},
		&Report{}, &Config{}, &AdminUser{}, &RobotContent{}, &Notification{}, &Collection{}, &CheckinLog{}, &WxMediaCheck{}, &ApiCallLog{},
		&PushSubscription{}, &ReplyUnlock{},
		&DeviceToken{}, &IAPTransaction{}, &EmailLog{},
		&CoinPackagePrice{}, &PlayPurchase{},
		&PersonaConfig{}, &RobotMemory{}, &RobotKeywordRule{}, &RobotReplyCache{},
		&Moment{}, &MomentLike{}, &MomentComment{},
	}
}
