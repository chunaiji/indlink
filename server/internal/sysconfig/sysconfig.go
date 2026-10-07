package sysconfig

import (
	"strconv"
	"sync"
	"time"

	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// 配置键集中定义(价格/开关都在这里,运营可在 config 表热调)。
const (
	KeyVerifyRequired = "verify_required"  // 发瓶/开聊是否强制真人认证 (0/1)
	KeyIOSRechargeOff = "ios_recharge_off" // iOS 是否隐藏充值入口 (0/1)
	KeyPriceChat      = "price_chat"       // 开聊消耗金币
	KeyPriceUnlock    = "price_unlock"     // 解锁一条回信消耗金币
	KeyRegReward      = "reg_reward_coins" // 注册奖励金币
	KeyFeedSize       = "feed_size"        // 捞瓶 feed 预生成条数

	// 机器人(#1,由管理台配置)
	KeyRobotEnabled    = "robot_enabled"        // 机器人总开关 (0/1)
	KeyRobotCount      = "robot_count"          // 机器人数量
	KeyRobotThrowPerHr = "robot_throw_per_hour" // 每小时投放瓶子数
	KeyRobotReplyRatio = "robot_reply_ratio"    // 机器人回信比例 (0~100)

	// 机器人内容/同城/主动引导(#robot-quality)
	KeyRobotBottlePoolTarget   = "robot_bottle_pool_target"   // AI 瓶子池目标条数
	KeyCityRobotTopM           = "city_robot_top_m"           // 同城前 M 个位置
	KeyCityRobotTopN           = "city_robot_top_n"           // 其中机器人数(0=关)
	KeyRobotLanguage           = "robot_language"             // 自动补齐机器人的默认语言 zh/en(后台新增可单独指定)
	KeyRobotProactive          = "robot_proactive_enabled"    // 主动引导总开关(b/c/d)
	KeyRobotBottle2ChatRatio   = "robot_bottle2chat_ratio"    // 回信后引导加聊概率(%)
	KeyRobotOutreachDailyCap   = "robot_outreach_daily_cap"   // 破冰每租户每日上限
	KeyRobotOutreachNewHours   = "robot_outreach_new_hours"   // 新用户窗口(小时)
	KeyRobotOutreachSilentDays = "robot_outreach_silent_days" // 沉默用户阈值(天)
	KeyRobotNudgeSilentMin     = "robot_nudge_silent_min"     // 会话沉默追问阈值(分)
	KeyRobotNudgeMax           = "robot_nudge_max_per_chat"   // 每会话最多追问次数

	// AI 引擎（OpenAI 兼容接口）
	KeyAIBotEnabled    = "ai_bot_enabled"     // AI 回复总开关，关闭则 fallback 静态池
	KeyAIChatEnabled   = "ai_chat_enabled"    // 聊天续接 AI 独立开关（路径 C）
	KeyLLMAPIEndpoint  = "llm_api_endpoint"   // LLM API 地址（OpenAI 兼容）
	KeyLLMAPIKey       = "llm_api_key"        // API Key（AES-GCM 加密存储）
	KeyLLMModel        = "llm_model"          // 模型名
	KeyLLMTemperature  = "llm_temperature"    // 实际温度 = value/10
	KeyLLMMaxTokens    = "llm_max_tokens"     // 单次回复 token 上限
	KeyAIReplyDelayMin = "ai_reply_delay_min" // 模拟打字最小延迟（ms）
	KeyAIReplyDelayMax = "ai_reply_delay_max" // 模拟打字最大延迟（ms）
	KeyLLMConcurrency  = "llm_concurrency"    // 同时进行的 LLM 调用上限

	// 精准匹配权重(#8,捞瓶/扩列打分)
	KeyMatchWTag    = "match_w_tag"    // 标签重合
	KeyMatchWCity   = "match_w_city"   // 同城
	KeyMatchWGender = "match_w_gender" // 取向/异性优先
	KeyMatchWFresh  = "match_w_fresh"  // 新鲜度
	KeyMatchWHeat   = "match_w_heat"   // 热度
	KeyMatchWRobot  = "match_w_robot"  // 机器人惩罚(正数,打分时相减)
	KeyMatchWRandom = "match_w_random" // 随机扰动
	KeyMatchWDist   = "match_w_dist"   // 距离衰减(需要双方都有经纬度)

	// 分享(#7)
	KeyShareTitle = "share_title" // 转发标题
	KeyShareImage = "share_image" // 转发卡片图(空=用默认截图)

	// 通知(#4)
	KeyNotifyWxTemplate = "notify_wx_template_id" // 微信订阅消息模板 ID(空=不下发)

	// 首页公告
	KeyNoticeTitle = "notice_title" // 公告标题
	KeyNoticeBody  = "notice_body"  // 公告正文(空=不弹)

	// Tab 可见性(1=显示 0=隐藏)
	KeyTabHome    = "tab_show_home"
	KeyTabCity    = "tab_show_city"
	KeyTabExpand  = "tab_show_expand"
	KeyTabMessage = "tab_show_message"
	KeyTabMine    = "tab_show_mine"
	KeyTabPrivacy = "tab_show_privacy"

	// 我的-其他功能项 可见性(1=显示 0=隐藏)
	KeyFnVerify     = "fn_show_verify"
	KeyFnAvatar     = "fn_show_avatar"
	KeyFnViewed     = "fn_show_viewed"
	KeyFnItems      = "fn_show_items"
	KeyFnCollection = "fn_show_collection"
	KeyFnWallet     = "fn_show_wallet"
	KeyFnOrders     = "fn_show_orders"
	KeyFnBlocklist  = "fn_show_blocklist"
	KeyFnContact    = "fn_show_contact"
	KeyFnSettings   = "fn_show_settings"
	KeyFnMoments    = "fn_show_moments"
	KeyFnRecharge   = "fn_show_recharge"  // 充值入口(钱包卡片内),默认隐藏
	KeyFnWalletLog  = "fn_show_walletlog" // 金币流水,默认隐藏

	// 我的页-完善资料跑马灯(资料不全时显示;文案含社交词,审核模式应关)
	KeyMineTipOn   = "mine_complete_tip_on"
	KeyMineTipText = "mine_complete_tip_text"

	// 推送
	KeyPushTplReply         = "push_tpl_reply"          // 回信订阅消息模板 ID
	KeyPushTplChat          = "push_tpl_chat"           // 聊天订阅消息模板 ID
	KeyPushTplActivity      = "push_tpl_activity"       // 活动预约提醒模板 ID
	KeyPushTplWorkRecommend = "push_tpl_work_recommend" // 新作品推荐提醒模板 ID
	KeyPushTplCheckin       = "push_tpl_checkin"        // 签到提醒模板 ID
	KeyPushWxAppID          = "push_wx_appid"           // 推送用 AppID(可与登录同一个)
	KeyPushWxSecret         = "push_wx_secret"          // 推送用 AppSecret
	KeyPushSubscribePrompt  = "push_subscribe_prompt"   // 是否弹框引导用户订阅消息模板 (0/1)

	// 小程序 WebSocket
	KeyWSURL = "ws_url" // 小程序 WebSocket 地址(空=使用客户端默认)

	// 每日次数限制(#5)
	KeyQuotaThrowDaily = "quota_throw_daily" // 每日免费扔瓶次数
	KeyQuotaScoopDaily = "quota_scoop_daily" // 每日免费捞瓶次数
	KeyQuotaThrowPack  = "quota_throw_pack"  // 扔瓶次数包:每个给多少次
	KeyQuotaScoopPack  = "quota_scoop_pack"  // 捞瓶次数包:每个给多少次

	// 首页「今日已有 N 条回应」计数(#3,按天 base + 每分钟净增)
	KeyHookBase      = "hook_base"        // 当日基数
	KeyHookAddPerMin = "hook_add_per_min" // 每分钟随机增加上限
	KeyHookSubPerMin = "hook_sub_per_min" // 每分钟随机减少上限
	// 「今日海况 · N 人在线」的展示数:与 hook_* 同一思路,是运营数不是真在线数。
	// online_base 为 0 时不下发(App 显示 0)。按小时曲线起伏 + 按分钟确定性抖动。
	KeyOnlineBase   = "online_base"   // 日间基数
	KeyOnlineJitter = "online_jitter" // 每分钟抖动幅度(±)
	// App 海面同时漂着几个瓶子(5~9,越界取边界)。
	KeyOceanBottleCount = "ocean_bottle_count"

	// 回信脱敏字符数(#5,前 N 字明文,其余 ****)
	KeyReplyMaskLen = "reply_mask_len"

	// UI 文案(运营可在管理台修改,登录时下发给小程序)
	KeyUITextAnonSender = "ui_text_anon_sender" // 匿名瓶发件人显示名
	KeyUITextAnonFriend = "ui_text_anon_friend" // 弹框/详情匿名用户称呼
	KeyUITextSomeFriend = "ui_text_some_friend" // 弹框/详情普通用户称呼
	KeyUITextNavTitle   = "ui_text_nav_title"   // 海洋页导航栏标题

	// 快捷回复(逗号分隔,登录时下发)
	KeyChatQuicks  = "chat_quicks"  // 聊天页快捷回复
	KeyReplyQuicks = "reply_quicks" // 回信页快捷回复

	KeyUITextChatBanner    = "ui_text_chat_banner"   // 聊天页顶部横幅文案
	KeyLowBalanceThreshold = "low_balance_threshold" // 余额低于此值(猛币)推系统消息;0=关
	KeyLowBalanceMsg       = "low_balance_msg"       // 余额不足系统消息文案
	KeyUITextQuotaTitle    = "ui_text_quota_title"   // 次数用完弹框标题
	KeyUITextQuotaMsg      = "ui_text_quota_msg"     // 次数用完弹框内容

	// 每日签到(#增长1)
	KeyCheckinEnabled     = "checkin_enabled"      // 签到总开关 (0/1)
	KeyCheckinCoins       = "checkin_coins"        // 每日签到发放金币 N(阶梯解析失败时的后备)
	KeyCheckinLadder      = "checkin_ladder"       // 7天阶梯币数,逗号分隔(连续第1~7天,第8天回到第1档)
	KeyCheckinMakeupLimit = "checkin_makeup_limit" // 每月看激励视频补签次数上限;0=关闭补签

	// 水印相机:逆地理编码(腾讯位置服务 key,lbs.qq.com 申请;空=水印降级显示经纬度)
	// 水印相机:品牌平铺水印样式(经 /features 下发)
	KeyWmTileText  = "wm_tile_text"  // 平铺文字
	KeyWmTileColor = "wm_tile_color" // 颜色(rgba,含透明度)
	KeyWmTileSize  = "wm_tile_size"  // 字号(px)

	// 微信内容安全(过审要求:msgSecCheck 文本同步 + mediaCheckAsync 图片异步)
	KeySecCallbackToken = "sec_callback_token" // 微信消息推送回调 Token(小程序后台"消息推送"里配同值)

	// 联系客服(我的页弹框,后台可配文案+图片,图片一般放客服微信二维码)
	KeyContactText  = "contact_text"
	KeyContactImage = "contact_image"

	// 首页关联小程序入口(最多 4 个,缩起胶囊点击展开;每个可配图标)
	KeyLinkMPEnabled = "link_mp_enabled" // 入口总开关(0/1)
	KeyLinkMPTitle   = "link_mp_title"   // 缩起胶囊文案
	KeyLinkMP1AppID  = "link_mp1_appid"
	KeyLinkMP1Title  = "link_mp1_title"
	KeyLinkMP1Icon   = "link_mp1_icon"
	KeyLinkMP1Path   = "link_mp1_path"
	KeyLinkMP2AppID  = "link_mp2_appid"
	KeyLinkMP2Title  = "link_mp2_title"
	KeyLinkMP2Icon   = "link_mp2_icon"
	KeyLinkMP2Path   = "link_mp2_path"
	KeyLinkMP3AppID  = "link_mp3_appid"
	KeyLinkMP3Title  = "link_mp3_title"
	KeyLinkMP3Icon   = "link_mp3_icon"
	KeyLinkMP3Path   = "link_mp3_path"
	KeyLinkMP4AppID  = "link_mp4_appid"
	KeyLinkMP4Title  = "link_mp4_title"
	KeyLinkMP4Icon   = "link_mp4_icon"
	KeyLinkMP4Path   = "link_mp4_path"

	// 留存四功能(2026-08):开关默认全关,工具形态不露出
	KeyBottleTraceEnabled = "bottle_trace_enabled" // 漂流轨迹开关(0/1)
	KeyNightBottleEnabled = "night_bottle_enabled" // 深夜瓶开关(0/1)
	KeyNightStart         = "night_start"          // 深夜场开始小时(0-23)
	KeyNightEnd           = "night_end"            // 深夜场结束小时(次日凌晨,0-23)
	KeyNightPushTitle     = "night_push_title"     // 深夜场开启推送文案
	KeyUserCardEnabled    = "user_card_enabled"    // 用户资料卡开关(0/1)
	KeyCharmRankEnabled   = "charm_rank_enabled"   // 魅力周榜开关(0/1)
	KeySquareEnabled      = "square_enabled"       // 动态广场开关(0/1):开=扩列tab变朋友圈,关=旧用户列表

	// 聊天扣费规则:开聊扣 price_chat(M)、每条扣 price_msg(N)、前 chat_free_msgs 条免费(L)。
	// 三个数全由后台配,服务端 chat.SendMessage 真扣;App 通过 /app-config 的 pricing 段拿来显示。
	KeyPriceMsg     = "price_msg"      // 每条消息消耗金币 N;0=关闭
	KeyChatFreeMsgs = "chat_free_msgs" // 每个会话里发送方前 L 条免费(按条收费开着才有意义);0=没有免费条数

	// 分享奖励(#增长3)
	KeyShareRewardEnabled    = "share_reward_enabled"     // 分享奖励总开关 (0/1)
	KeyShareRewardCoins      = "share_reward_coins"       // 单次分享奖励 M
	KeyShareRewardDailyLimit = "share_reward_daily_limit" // 每日最多领奖次数

	// 机器人主动触达(#增长5)
	KeyOutreachEnabled      = "outreach_enabled"        // 主动触达总开关 (0/1)
	KeyOutreachWindowStart  = "outreach_window_start"   // 触达时段开始小时 0-23(含)
	KeyOutreachWindowEnd    = "outreach_window_end"     // 触达时段结束小时 0-23(不含)
	KeyOutreachProbability  = "outreach_probability"    // 活跃用户抽样比例 N(%)
	KeyOutreachLookbackMin  = "outreach_lookback_min"   // 回溯活跃窗口 M(分钟)
	KeyOutreachTickMin      = "outreach_tick_min"       // 窗口内调度间隔(分钟)
	KeyOutreachUserDailyCap = "outreach_user_daily_cap" // 同一用户每日被触达上限

	// 流量主广告(#增长6):按类型配一个 unit id(同类共用),各广告位只开关。
	KeyAdEnabled     = "ad_enabled"           // 广告总开关 (0/1)
	KeyAdInterGapSec = "ad_inter_gap_sec"     // 插屏最小间隔(秒)
	KeyAdRewardCoins = "ad_reward_coin_coins" // 激励视频每次发放金币
	KeyAdRewardDaily = "ad_reward_coin_daily" // 激励视频每日领取上限

	// 各类型广告位 unit id(同类型所有广告位共用)
	KeyAdBannerUnit = "ad_banner_unit" // Banner 广告位 id
	KeyAdInterUnit  = "ad_inter_unit"  // 插屏广告位 id
	KeyAdRewardUnit = "ad_reward_unit" // 激励视频广告位 id
	KeyAdNativeUnit = "ad_native_unit" // 原生模板广告位 id

	// 各广告位开关(共用所属类型的 unit)
	KeyAdBannerOceanOn      = "ad_banner_ocean_on"
	KeyAdBannerCityOn       = "ad_banner_city_on"
	KeyAdBannerExpandOn     = "ad_banner_expand_on"
	KeyAdBannerMessageOn    = "ad_banner_message_on"
	KeyAdBannerMineOn       = "ad_banner_mine_on"
	KeyAdBannerDetailOn     = "ad_banner_detail_on"
	KeyAdBannerChatOn       = "ad_banner_chat_on"
	KeyAdBannerCollectionOn = "ad_banner_collection_on"
	KeyAdBannerOrdersOn     = "ad_banner_orders_on"
	KeyAdBannerWalletlogOn  = "ad_banner_walletlog_on"
	KeyAdBannerViewedOn     = "ad_banner_viewed_on"
	KeyAdBannerPrivacyOn    = "ad_banner_privacy_on"
	KeyAdInterScoopOn       = "ad_inter_scoop_on"
	KeyAdInterDetailOn      = "ad_inter_detail_on"
	KeyAdRewardCoinOn       = "ad_reward_coin_on"
	KeyAdGridMineOn         = "ad_grid_mine_on"

	// 全站页面图片覆盖(同城/首页/消息/充值/道具/流水/订单/收藏)总开关
	KeyPagesCoverOn    = "pages_cover_on"    // 覆盖开关 (0/1)
	KeyPagesCoverImage = "pages_cover_image" // 覆盖图 URL(空=用默认图)

	// ============ App 端(Flutter)============
	// 小程序不读这些键;新增键一律默认关/空,不影响线上小程序行为。

	// 手机号验证码登录
	KeyAppOTPTTL       = "app_otp_ttl"         // 验证码有效期(秒)
	KeyAppOTPResend    = "app_otp_resend"      // 同号码重发间隔(秒)
	KeyAppOTPDailyCap  = "app_otp_daily_cap"   // 单号码每日发送上限
	KeyAppOTPIPCap     = "app_otp_ip_hour_cap" // 单 IP 每小时发送上限
	KeyAppOTPDevCode   = "app_otp_dev_code"    // 开发态万能码(空=关闭;生产务必留空)
	KeyAppSMSProvider  = "app_sms_provider"    // 短信服务商(空=不真发,仅落日志)
	KeyAppSMSKey       = "app_sms_key"         // 服务商 Key
	KeyAppSMSSecretEnc = "app_sms_secret"      // 服务商密钥(AES-GCM 密文)
	KeyAppSMSTemplate  = "app_sms_template"    // 模板 ID(印度需先在 TRAI DLT 平台注册)
	KeyAppSMSSign      = "app_sms_sign"        // 短信签名

	// 邮箱验证码走 SMTP(net/smtp,不依赖第三方 SDK)。
	// 与短信共用 TTL/重发/频控那几个键,只有投递方式不同。
	// 这几项**配齐了才真发**,缺任意一项都只落日志(开发态)。
	KeyAppSMTPHost     = "app_smtp_host"      // SMTP 主机,如 smtp.gmail.com
	KeyAppSMTPPort     = "app_smtp_port"      // 465(隐式 TLS) 或 587(STARTTLS)
	KeyAppSMTPAccount  = "app_smtp_account"   // 登录账号
	KeyAppSMTPTokenEnc = "app_smtp_token"     // 授权码(AES-GCM 密文,非登录密码)
	KeyAppSMTPFrom     = "app_smtp_from"      // 发件地址
	KeyAppSMTPFromName = "app_smtp_from_name" // 发件人显示名,可空

	// 第三方登录

	// 登录 / 注册标识开关:手机号、邮箱各一个,默认都开;两个都关时 App 视为都开(不能把人锁在门外)。
	// 只影响 App 的展示(渠道切换、表单),服务端两条链路都保留。
	KeyAppLoginPhoneEnabled = "app_login_phone_enabled"
	KeyAppLoginEmailEnabled = "app_login_email_enabled"

	// App 第三方登录渠道开关(国内版:微信 / 支付宝)。默认开;/app-config 还要求该租户配了对应凭证才下发 true。

	// Apple 内购

	// App 模拟支付渠道(仅联调)
	KeyAppPayMockEnabled = "app_pay_mock_enabled" // 模拟支付渠道 (0/1),上线前必须关

	// 火花主动匹配(internal/spark)
	KeySparkEnabled      = "spark_enabled"        // 总开关 (0/1),默认关
	KeySparkIntervalMin  = "spark_interval_min"   // 每人下次匹配的最小间隔(分钟)
	KeySparkIntervalMax  = "spark_interval_max"   // 最大间隔(分钟)
	KeySparkRealRatio    = "spark_real_ratio"     // 配到真人的概率 A(%),其余配机器人
	KeySparkWindowStart  = "spark_window_start"   // 时段开始小时 0-23(含)
	KeySparkWindowEnd    = "spark_window_end"     // 时段结束小时 0-23(不含)
	KeySparkUserDailyCap = "spark_user_daily_cap" // 每人每日上限(与 outreach 共用计数)
	KeySparkTitle        = "spark_title"          // 弹窗标题,空=App 内置文案
	KeySparkTitleEN      = "spark_title_en"
	KeySparkText         = "spark_text" // 弹窗正文,支持 {nickname}
	KeySparkTextEN       = "spark_text_en"

	// 地理(Google Maps)
	KeyAppDiscoverMaxKM = "app_discover_max_km" // 发现页距离上限(km)

	// 发现页撤回:把刚左滑掉的人捞回来,扣币(付费功能,价格后台可调)。
	KeyAppRewindPrice = "app_rewind_price" // 撤回一次消耗金币

	// 发现页左滑跳过扣币(付费功能,默认关)。开关关时左滑免费,只落库去重。
	KeyAppDiscoverSkipChargeEnabled = "app_discover_skip_charge_enabled" // 左滑跳过是否扣币(0/1)
	KeyAppDiscoverSkipPrice         = "app_discover_skip_price"          // 左滑跳过一次消耗金币

	// 答题匹配(默认关)：答相同答案的人排序加分，替代死板的语言/兴趣维度。
	KeyAppDiscoverQuizEnabled = "app_discover_quiz_enabled" // 0/1
	KeyAppDiscoverQuiz        = "app_discover_quiz"         // 题库:每行 qkey|问题|逗号分隔选项

	// 完善资料的可选项。写死在包里意味着运营想加一门语言就得发版,
	// 所以做成后台可配,客户端启动时拉一次;留空则回退到客户端内置表。
	//
	// 语言:一行一个,用本族自称(endonym),不翻译。
	// 兴趣:一行一条 `key|中文|English`,key 入库、中英文只管显示。
	KeyAppProfileLanguages    = "app_profile_languages"
	KeyAppProfileInterests    = "app_profile_interests"
	KeyAppProfileMinInterests = "app_profile_min_interests" // 至少选几个兴趣才能完成引导
	KeyAppProfileMinAge       = "app_profile_min_age"       // 允许填写的年龄下限(合规:不得低于 18)
	KeyAppProfileMaxAge       = "app_profile_max_age"

	// 法务文本。走接口下发而不是打进包里——改一版文案要重新提审,代价太大。
	// 空值时客户端回退到内置的 H5 地址,保证不配也不会开天窗。
	KeyAppLegalTerms     = "app_legal_terms"      // 用户协议(中文)
	KeyAppLegalPrivacy   = "app_legal_privacy"    // 隐私政策(中文)
	KeyAppLegalTermsEN   = "app_legal_terms_en"   // 用户协议(英文,空=回退中文)
	KeyAppLegalPrivacyEN = "app_legal_privacy_en" // 隐私政策(英文,空=回退中文)

	// 推送(FCM / APNs)
	KeyAppPushEnabled = "app_push_enabled" // App 推送总开关 (0/1)

	// App 本地日志(客户端落在设备文件系统里,用户可导出发客服)
	// 设计见 docs/superpowers/specs/2026-09-19-app-logging-design.md
	KeyAppLogEnabled    = "app_log_enabled"     // 总开关 (0/1)
	KeyAppLogLevel      = "app_log_level"       // debug/info/warn/error
	KeyAppLogBody       = "app_log_body"        // 是否记录请求/响应体 (0/1)
	KeyAppLogMaxMB      = "app_log_max_mb"      // 本地日志总量上限(MB)
	KeyAppLogRetainDays = "app_log_retain_days" // 本地日志保留天数

	// —— App 界面与文案(经 /app-config 下发) ——
	//
	// 为什么不复用小程序那套 fn_show_* / ui_text_* :默认值是全局的,没法按端分叉。
	// fn_show_recharge 等默认 "0"(小程序过审要求默认隐藏),ui_text_nav_title 默认
	// "漂流瓶",contact_text 默认写着客服微信——App 现在这些入口常显、标题是「今晚的海」、
	// 客服是邮箱,复用就等于上线当天集体改样。这批键的默认值按「与 App 当前行为一致」定,
	// 配置上线是零行为变更。
	//
	// 文案一律默认空串 = 用 App 内置 ARB 文案。英文键空则回退中文键,中文键也空才回退内置。
	KeyAppUIAnonSender   = "app_ui_anon_sender"    // 匿名瓶发件人显示名
	KeyAppUIAnonSenderEN = "app_ui_anon_sender_en" //
	KeyAppUIOceanTitle   = "app_ui_ocean_title"    // 海洋页标题
	KeyAppUIOceanTitleEN = "app_ui_ocean_title_en" //
	KeyAppUIQuotaTitle   = "app_ui_quota_title"    // 次数用完弹框标题(支持占位符 {n} = 每日次数)
	KeyAppUIQuotaTitleEN = "app_ui_quota_title_en" //
	KeyAppUIQuotaBody    = "app_ui_quota_body"     // 次数用完弹框正文
	KeyAppUIQuotaBodyEN  = "app_ui_quota_body_en"  //

	// 客服。空则 App 回退到内置的 mailto:,所以不配也不会没有求助入口。
	KeyAppContactText   = "app_contact_text"
	KeyAppContactTextEN = "app_contact_text_en"
	KeyAppContactImage  = "app_contact_image" // 图片不分语种

	// App 启动页图片(最多 5 张)。空=用 App 内置启动页。App 拉到 /app-config 后静默
	// 下载进本地缓存,下一次冷启动才显示——启动页不能等网络。
	KeyAppSplashImage1 = "app_splash_image_1"
	KeyAppSplashImage2 = "app_splash_image_2"
	KeyAppSplashImage3 = "app_splash_image_3"
	KeyAppSplashImage4 = "app_splash_image_4"
	KeyAppSplashImage5 = "app_splash_image_5"

	// 「我的」入口显隐。默认全 "1" —— App 今天就是全显,默认值改成 0 等于静默砍功能。
	//
	// 刻意不给「设置」和「客服」开关:设置页装着退出登录/切语言/协议;
	// 客服是 App Store 1.2 对 UGC 应用的硬要求(开发者联系方式必须可达),
	// 配没了能直接导致下架。这两个不该是配置能关掉的东西。
	KeyAppFnItems     = "app_fn_items"      // 道具
	KeyAppFnWallet    = "app_fn_wallet"     // 余额卡片
	KeyAppFnRecharge  = "app_fn_recharge"   // 充值按钮
	KeyAppFnWalletLog = "app_fn_wallet_log" // 金币流水
	KeyAppFnBlocklist = "app_fn_blocklist"  // 黑名单

	// 首次登录前隐私授权弹框(合规,默认关;上线前由运营打开)。
	KeyAppPrivacyGateEnabled = "app_privacy_gate_enabled"
)

// 默认值:DB 未配置时回退。
var defaults = map[string]string{
	KeyVerifyRequired: "0",
	KeyIOSRechargeOff: "1",
	KeyPriceChat:      "5",
	KeyPriceUnlock:    "2",
	KeyRegReward:      "50",
	KeyFeedSize:       "50",

	// App 端:全部默认关/空,不影响线上小程序
	KeyAppOTPTTL:       "300",
	KeyAppOTPResend:    "60",
	KeyAppOTPDailyCap:  "10",
	KeyAppOTPIPCap:     "20",
	KeyAppOTPDevCode:   "",
	KeyAppSMSProvider:  "",
	KeyAppSMSKey:       "",
	KeyAppSMSSecretEnc: "",
	KeyAppSMSTemplate:  "",
	KeyAppSMSSign:      "",

	KeyAppSMTPHost:     "",
	KeyAppSMTPPort:     "465",
	KeyAppSMTPAccount:  "",
	KeyAppSMTPTokenEnc: "",
	KeyAppSMTPFrom:     "",
	KeyAppSMTPFromName: "DRIFT",


	KeyAppLoginPhoneEnabled: "1",
	KeyAppLoginEmailEnabled: "1",


	// 火花主动匹配:默认全关,文案空串=App 用内置 ARB
	KeySparkEnabled:      "0",
	KeySparkIntervalMin:  "20",
	KeySparkIntervalMax:  "60",
	KeySparkRealRatio:    "30",
	KeySparkWindowStart:  "10",
	KeySparkWindowEnd:    "23",
	KeySparkUserDailyCap: "3",
	KeySparkTitle:        "",
	KeySparkTitleEN:      "",
	KeySparkText:         "",
	KeySparkTextEN:       "",

	KeyAppPayMockEnabled: "0", // 联调时后台打开;上线前务必关掉,否则任何人都能凭空发币

	KeyAppLogEnabled: "1",
	KeyAppLogLevel:   "info",
	// 默认不记 body:请求体里有密码、验证码、ID Token,
	// 而日志会被用户导出发给客服——那条路径不可控。排查时临时打开,用完关掉。
	KeyAppLogBody:       "0",
	KeyAppLogMaxMB:      "100",
	KeyAppLogRetainDays: "31",

	KeyAppDiscoverMaxKM: "100",
	KeyAppRewindPrice:   "10",

	// 左滑跳过默认免费(开关默认关),开时默认 1 币。
	KeyAppDiscoverSkipChargeEnabled: "0",
	KeyAppDiscoverSkipPrice:         "1",

	// 答题匹配默认开(App 端专属键、原型 C2 主设计、内置 3 道题；运营可在后台增删改或关闭)。
	// App 与小程序两个租户互不影响,这里开只影响 App 的发现筛选,不碰小程序工具形态。
	KeyAppDiscoverQuizEnabled: "1",
	KeyAppDiscoverQuiz: "weekend|周末更想怎么过？|宅家充电,出门探索,看心情\n" +
		"night|理想的夜晚是|热闹派对,安静小酌,早睡星人\n" +
		"reply|回消息的节奏|秒回选手,随缘上线,看对象",

	// 留空 = 用客户端内置表,不强制运营先去配一遍才能注册。
	KeyAppProfileLanguages:    "",
	KeyAppProfileInterests:    "",
	KeyAppProfileMinInterests: "3",
	KeyAppProfileMinAge:       "18",
	KeyAppProfileMaxAge:       "60",

	KeyAppLegalTerms:     "",
	KeyAppLegalPrivacy:   "",
	KeyAppLegalTermsEN:   "",
	KeyAppLegalPrivacyEN: "",

	KeyAppPushEnabled: "0",

	// 全空 = 用 App 内置 ARB 文案。默认给中文串会让每个 App 租户一上线就被
	// 覆盖成运营没写过的文案,空串才是「没配过」的诚实表达。
	KeyAppUIAnonSender:   "",
	KeyAppUIAnonSenderEN: "",
	KeyAppUIOceanTitle:   "",
	KeyAppUIOceanTitleEN: "",
	KeyAppUIQuotaTitle:   "",
	KeyAppUIQuotaTitleEN: "",
	KeyAppUIQuotaBody:    "",
	KeyAppUIQuotaBodyEN:  "",

	KeyAppContactText:   "",
	KeyAppContactTextEN: "",
	KeyAppContactImage:  "",
	KeyAppSplashImage1:  "",
	KeyAppSplashImage2:  "",
	KeyAppSplashImage3:  "",
	KeyAppSplashImage4:  "",
	KeyAppSplashImage5:  "",

	// 全 "1":这是 App 今天的行为。默认 0 等于发版即砍功能。
	KeyAppFnItems:     "1",
	KeyAppFnWallet:    "1",
	KeyAppFnRecharge:  "1",
	KeyAppFnWalletLog: "1",
	KeyAppFnBlocklist: "1",

	// 隐私授权弹框默认关(新功能,合规上线前打开)。
	KeyAppPrivacyGateEnabled: "0",

	KeyRobotEnabled:    "0",
	KeyRobotCount:      "20",
	KeyRobotThrowPerHr: "10",
	KeyRobotReplyRatio: "30",

	KeyRobotBottlePoolTarget:   "80",
	KeyCityRobotTopM:           "10",
	KeyCityRobotTopN:           "3",
	KeyRobotLanguage:           "zh",
	KeyRobotProactive:          "0",
	KeyRobotBottle2ChatRatio:   "20",
	KeyRobotOutreachDailyCap:   "20",
	KeyRobotOutreachNewHours:   "24",
	KeyRobotOutreachSilentDays: "7",
	KeyRobotNudgeSilentMin:     "30",
	KeyRobotNudgeMax:           "1",

	KeyAIBotEnabled:    "0",
	KeyAIChatEnabled:   "0",
	KeyLLMAPIEndpoint:  "",
	KeyLLMAPIKey:       "",
	KeyLLMModel:        "gpt-4o-mini",
	KeyLLMTemperature:  "8",
	KeyLLMMaxTokens:    "200",
	KeyAIReplyDelayMin: "1500",
	KeyAIReplyDelayMax: "4000",
	KeyLLMConcurrency:  "10",

	KeyMatchWTag:    "40",
	KeyMatchWCity:   "25",
	KeyMatchWGender: "15",
	KeyMatchWFresh:  "10",
	KeyMatchWHeat:   "10",
	KeyMatchWRobot:  "5",
	KeyMatchWRandom: "15",
	KeyMatchWDist:   "15",

	KeyShareTitle: "我在漂流瓶捞到一句话,你也来看看~",
	KeyShareImage: "",

	KeyNotifyWxTemplate: "",

	KeyNoticeTitle: "温馨提示",
	KeyNoticeBody:  "请文明交流，不要发布违法、色情信息，共同维护健康的社区环境。违规内容将被删除，情节严重者封号处理。",

	KeyTabHome:    "1",
	KeyTabCity:    "1",
	KeyTabExpand:  "1",
	KeyTabMessage: "1",
	KeyTabMine:    "1",
	KeyTabPrivacy: "0",

	KeyFnVerify:     "1",
	KeyFnAvatar:     "1",
	KeyFnViewed:     "1",
	KeyFnItems:      "0", // 道具商城默认隐藏,后台开启
	KeyFnCollection: "1",
	KeyFnWallet:     "1",
	KeyFnOrders:     "0", // 我的订单默认隐藏,后台开启
	KeyFnBlocklist:  "1",
	KeyFnContact:    "1",
	KeyFnSettings:   "1",
	KeyFnMoments:    "1",
	KeyFnRecharge:   "0", // 充值入口默认隐藏,后台开启
	KeyFnWalletLog:  "0", // 金币流水默认隐藏,后台开启

	KeyPushTplReply:         "",
	KeyPushTplChat:          "",
	KeyPushTplActivity:      "pCcmAdWN2BrNao4JB_Zx0WYyqY6Ip4r-dtnER321nbs",
	KeyPushTplWorkRecommend: "1QTc2A0zT5RUm0Khm6EGv33UPEGCWdkBwDu02steWn4",
	KeyPushTplCheckin:       "jDkqqnbLivP1FxZsIWXdmnuofZKCyHiuEMyvIEEjKrE",
	KeyPushWxAppID:          "",
	KeyPushWxSecret:         "",
	KeyPushSubscribePrompt:  "0",

	KeyWSURL: "",

	KeyQuotaThrowDaily: "10",
	KeyQuotaScoopDaily: "20",
	KeyQuotaThrowPack:  "10",
	KeyQuotaScoopPack:  "20",

	KeyHookBase:         "56000",
	KeyHookAddPerMin:    "8",
	KeyHookSubPerMin:    "3",
	KeyOnlineBase:       "260",
	KeyOnlineJitter:     "12",
	KeyOceanBottleCount: "5",

	KeyReplyMaskLen: "3",

	KeyUITextAnonSender: "匿名漂流瓶",
	KeyUITextAnonFriend: "海上的朋友",
	KeyUITextSomeFriend: "某位朋友",
	KeyUITextNavTitle:   "漂流瓶",

	KeyChatQuicks:  "我懂你,我也经历过,说说细节?,继续聊",
	KeyReplyQuicks: "我懂你,我也经历过,说说细节?",

	KeyUITextChatBanner:    "缘起一只漂流瓶 · 友善聊天",
	KeyLowBalanceThreshold: "10",
	KeyLowBalanceMsg:       "余额不足啦,充值后就能继续畅聊咯~",
	KeyUITextQuotaTitle:    "次数已用完",
	KeyUITextQuotaMsg:      "可前往道具商城购买次数包继续,或明天免费次数恢复。",

	KeyCheckinEnabled:     "0",
	KeyCheckinCoins:       "10",
	KeyCheckinLadder:      "5,5,10,10,15,15,30",
	KeyCheckinMakeupLimit: "2",

	KeyMineTipOn:   "1",
	KeyMineTipText: "✨ 完善资料，更容易被同城的人看到 ›",

	KeyWmTileText:  "小纸条",
	KeyWmTileColor: "rgba(255,255,255,0.16)",
	KeyWmTileSize:  "26",

	KeySecCallbackToken: "",

	KeyContactText:  "客服微信:chunj008\n工作时间 10:00-22:00",
	KeyContactImage: "",

	KeyLinkMPEnabled: "0",
	KeyLinkMPTitle:   "更多好玩",
	KeyLinkMP1AppID:  "", KeyLinkMP1Title: "", KeyLinkMP1Icon: "", KeyLinkMP1Path: "",
	KeyLinkMP2AppID: "", KeyLinkMP2Title: "", KeyLinkMP2Icon: "", KeyLinkMP2Path: "",
	KeyLinkMP3AppID: "", KeyLinkMP3Title: "", KeyLinkMP3Icon: "", KeyLinkMP3Path: "",
	KeyLinkMP4AppID: "", KeyLinkMP4Title: "", KeyLinkMP4Icon: "", KeyLinkMP4Path: "",

	KeyBottleTraceEnabled: "0",
	KeyNightBottleEnabled: "0",
	KeyNightStart:         "22",
	KeyNightEnd:           "2",
	KeyNightPushTitle:     "深夜瓶已开启,今晚的心事有人接住",
	KeyUserCardEnabled:    "0",
	KeyCharmRankEnabled:   "0",
	KeySquareEnabled:      "0",

	KeyPriceMsg:     "0",
	KeyChatFreeMsgs: "0",

	KeyShareRewardEnabled:    "0",
	KeyShareRewardCoins:      "5",
	KeyShareRewardDailyLimit: "3",

	KeyOutreachEnabled:      "0",
	KeyOutreachWindowStart:  "19",
	KeyOutreachWindowEnd:    "22",
	KeyOutreachProbability:  "10",
	KeyOutreachLookbackMin:  "30",
	KeyOutreachTickMin:      "30",
	KeyOutreachUserDailyCap: "1",

	KeyAdEnabled:     "0",
	KeyAdInterGapSec: "180",
	KeyAdRewardCoins: "5",
	KeyAdRewardDaily: "5",

	KeyAdBannerUnit: "",
	KeyAdInterUnit:  "",
	KeyAdRewardUnit: "",
	KeyAdNativeUnit: "",

	KeyAdBannerOceanOn:      "0",
	KeyAdBannerCityOn:       "0",
	KeyAdBannerExpandOn:     "0",
	KeyAdBannerMessageOn:    "0",
	KeyAdBannerMineOn:       "0",
	KeyAdBannerDetailOn:     "0",
	KeyAdBannerChatOn:       "0",
	KeyAdBannerCollectionOn: "0",
	KeyAdBannerOrdersOn:     "0",
	KeyAdBannerWalletlogOn:  "0",
	KeyAdBannerViewedOn:     "0",
	KeyAdBannerPrivacyOn:    "0",
	KeyAdInterScoopOn:       "0",
	KeyAdInterDetailOn:      "0",
	KeyAdRewardCoinOn:       "0",
	KeyAdGridMineOn:         "0",

	KeyPagesCoverOn:    "0", // 默认关:显示原内容
	KeyPagesCoverImage: "",
}

var (
	mu     sync.RWMutex
	cached map[int64]map[string]string // tenantID → key → value
	db     *gorm.DB
)

// Init 注入 DB 并做一次全量加载,同时将未入库的默认值写入 DB(确保 UTF-8 正确存储)。
func Init(database *gorm.DB) error {
	db = database
	if err := reload(); err != nil {
		return err
	}
	seedDefaults()
	return nil
}

// seedDefaults 将 defaults 中尚未存入 DB 的键以正确编码写入 tenant 0(全局默认)。
func seedDefaults() {
	mu.RLock()
	loaded := make(map[string]bool, len(cached[0]))
	for k := range cached[0] {
		loaded[k] = true
	}
	mu.RUnlock()

	for k, v := range defaults {
		if !loaded[k] {
			_ = Set(0, k, v)
		}
	}
}

func reload() error {
	var rows []model.Config
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	m := make(map[int64]map[string]string)
	for _, r := range rows {
		if m[r.TenantID] == nil {
			m[r.TenantID] = map[string]string{}
		}
		m[r.TenantID][r.Key] = r.Value
	}
	mu.Lock()
	cached = m
	mu.Unlock()
	return nil
}

// Reload 供后台改配置后手动刷新(V1 也可定时刷新)。
func Reload() error { return reload() }

// get 三级回退:租户值(非空) → 全局 tenant 0(非空) → 代码默认。
func get(tenantID int64, key string) string {
	mu.RLock()
	if tm, ok := cached[tenantID]; ok {
		if v, ok := tm[key]; ok && v != "" {
			mu.RUnlock()
			return v
		}
	}
	if gm, ok := cached[0]; ok {
		if v, ok := gm[key]; ok && v != "" {
			mu.RUnlock()
			return v
		}
	}
	mu.RUnlock()
	return defaults[key]
}

// GetString 返回字符串值(含三级回退),供后台展示当前配置。
func GetString(tenantID int64, key string) string { return get(tenantID, key) }

func GetInt(tenantID int64, key string) int {
	n, _ := strconv.Atoi(get(tenantID, key))
	return n
}

func GetInt64(tenantID int64, key string) int64 {
	n, _ := strconv.ParseInt(get(tenantID, key), 10, 64)
	return n
}

func GetBool(tenantID int64, key string) bool {
	return get(tenantID, key) == "1"
}

// Set 写入指定租户的配置并更新缓存。
func Set(tenantID int64, key, value string) error {
	cfg := model.Config{TenantID: tenantID, Key: key, Value: value, UpdatedAt: time.Now()}
	if err := db.Save(&cfg).Error; err != nil {
		return err
	}
	mu.Lock()
	if cached == nil {
		cached = map[int64]map[string]string{}
	}
	if cached[tenantID] == nil {
		cached[tenantID] = map[string]string{}
	}
	cached[tenantID][key] = value
	mu.Unlock()
	return nil
}
