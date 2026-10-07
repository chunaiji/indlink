package admin

import (
	"fmt"
	"strings"
	"time"

	"driftbottle/internal/chat"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/i18n"
	"driftbottle/internal/crypto"
	"driftbottle/internal/model"
	"driftbottle/internal/provider"
	"driftbottle/internal/push"
	"driftbottle/internal/robot"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/tenant"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/idgen"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Service struct {
	db        *gorm.DB
	credStore *tenant.Store
	chatSvc   *chat.Service
	pushSvc   *push.Service
	walletSvc *wallet.Service

	providers *provider.Store                                         // 服务商配置(支付/地图/内容安全)
	probe     func(tenantID int64, kind, prov string) (string, error) // 「测试连通」分派,main 注入
}

func New(db *gorm.DB, cs *tenant.Store) *Service { return &Service{db: db, credStore: cs} }

func (s *Service) SetChatService(svc *chat.Service)     { s.chatSvc = svc }
func (s *Service) SetPushService(svc *push.Service)     { s.pushSvc = svc }
func (s *Service) SetWalletService(svc *wallet.Service) { s.walletSvc = svc }

// AdjustCoins 后台给真实用户加币(delta>0)/扣币(delta<0),返回调整后余额。
// 机器人没有钱包语义,直接拒绝,免得流水表里混进假账。
func (s *Service) AdjustCoins(userID, delta int64, remark string) (int64, error) {
	if s.walletSvc == nil {
		return 0, fmt.Errorf("钱包服务未配置")
	}
	var u model.User
	if err := s.db.Select("user_id, tenant_id, is_robot").First(&u, "user_id = ?", userID).Error; err != nil {
		return 0, errs.New(errs.CodeBadRequest, "用户不存在")
	}
	if u.IsRobot {
		return 0, errs.New(errs.CodeBadRequest, "机器人账号不能调账")
	}
	return s.walletSvc.AdminAdjust(u.TenantID, userID, delta, remark)
}

// PushUser 后台直接向真实用户推送订阅消息。
func (s *Service) PushUser(userID int64, scene, f1, f2, f3, page string) error {
	if s.pushSvc == nil {
		return fmt.Errorf("推送服务未配置")
	}
	if page == "" {
		page = "/pages/ocean/ocean"
	}
	return s.pushSvc.SendDirect(userID, scene, f1, f2, f3, page)
}

// ---- 用户管理 ----

// UserRow 后台用户列表行：在 model.User 基础上补出 openid(C 端模型里被 json:"-" 隐藏)。
// openid 为空即视为机器人。
type UserRow struct {
	model.User
	OpenID         string `gorm:"-" json:"openid"`
	Tags           string `gorm:"-" json:"tags"`   // model.User.Tags 为 json:"-"(不下发 C 端),后台列表在此补出
	Balance        int64  `json:"balance"`         // 剩余金币(wallets.balance,无钱包行为 0)
	TotalRecharged int64  `json:"total_recharged"` // 总金币:历史累计获得(充值+签到+奖励等所有入账)
	Online         bool   `gorm:"-" json:"online"` // WS 是否在线(内存 Hub 快照)
	// App 账号体系(邮箱 / 第三方绑定)。model.User 里这几列对 C 端 json:"-",后台列表在此补出;
	// 第三方只下发"是否绑定",不下发 sub 本身(那是第三方的用户标识,后台也没有用途)。
	Email       string `gorm:"-" json:"email"`
	GoogleBound bool   `gorm:"-" json:"google_bound"`
	AppleBound  bool   `gorm:"-" json:"apple_bound"`
}

// googleFilter: "bound" 只看绑了 Google 的 / "unbound" 只看没绑的 / "" 不限。
func (s *Service) ListUsers(tenantID int64, keyword, status, robotFilter, tag, googleFilter string, page, size int) ([]UserRow, int64, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	if page < 1 {
		page = 1
	}
	// 过滤条件同时用于 count 与 list;list 侧 join 了 wallets,tenant_id 两表都有,统一加 users. 前缀
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if tenantID != 0 {
			q = q.Where("users.tenant_id = ?", tenantID)
		}
		switch robotFilter {
		case "robot":
			q = q.Where("users.is_robot = ?", true)
		case "human":
			q = q.Where("users.is_robot = ?", false)
		case "test":
			// 开发/测试登录账号:appid 为空时 openid = "wxdev_<code>"。
			q = q.Where("users.wx_openid LIKE ?", "wxdev%")
		}
		if keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("users.nickname LIKE ? OR users.city LIKE ? OR users.email LIKE ?", like, like, like)
		}
		// google_sub 未绑定时存 NULL(唯一索引要求,见 user/oauthlink.go),所以按 NULL 判。
		switch googleFilter {
		case "bound":
			q = q.Where("users.google_sub IS NOT NULL")
		case "unbound":
			q = q.Where("users.google_sub IS NULL")
		}
		if tag != "" {
			q = q.Where("users.tags LIKE ?", "%"+tag+"%")
		}
		switch status {
		case "banned":
			q = q.Where("users.status = ?", "banned")
		case "muted":
			q = q.Where("users.is_muted = ?", true)
		case "active":
			q = q.Where("users.status = ?", "active")
		case "deleted":
			q = q.Where("users.status = ?", "deleted")
		default: // 默认视图不展示软删除用户
			q = q.Where("users.status != ?", "deleted")
		}
		return q
	}

	var total int64
	applyFilters(s.db.Model(&model.User{})).Count(&total)

	// 在线集合(内存 Hub 快照),用于"在线优先"排序与 Online 标记
	var onlineIDs []int64
	if s.chatSvc != nil {
		onlineIDs = s.chatSvc.Hub().OnlineUserIDs()
	}

	// 排序优先级:真实用户(is_robot ASC) > 在线 > 最后活跃倒序 > 剩余金币倒序
	q := applyFilters(
		s.db.Table("users").
			Select("users.*, COALESCE(wallets.balance, 0) AS balance, COALESCE(wallets.total_recharged, 0) AS total_recharged").
			Joins("LEFT JOIN wallets ON wallets.user_id = users.user_id"),
	)
	q = q.Order("users.is_robot ASC")
	// 注意:GORM v2 的 Order() 只认 string / clause.OrderByColumn,clause.Expr 会被静默忽略,
	// 故用字符串拼接。在线 IDs 为纯数字,无注入风险。
	if len(onlineIDs) > 0 {
		idStrs := make([]string, len(onlineIDs))
		for i, id := range onlineIDs {
			idStrs[i] = fmt.Sprintf("%d", id)
		}
		q = q.Order("(users.user_id IN (" + strings.Join(idStrs, ",") + ")) DESC")
	}
	q = q.Order("users.last_active_at DESC").
		Order("COALESCE(wallets.balance, 0) DESC")

	var rows []UserRow
	err := q.Offset((page - 1) * size).Limit(size).Find(&rows).Error

	onlineSet := make(map[int64]bool, len(onlineIDs))
	for _, id := range onlineIDs {
		onlineSet[id] = true
	}
	for i := range rows {
		openid := rows[i].WxOpenID
		if openid == "" {
			openid = rows[i].AlipayUID
		}
		rows[i].OpenID = openid
		rows[i].Tags = rows[i].User.Tags
		rows[i].Online = onlineSet[rows[i].UserID]
		rows[i].Email = rows[i].User.Email
		rows[i].GoogleBound = rows[i].User.GoogleSub != nil
		rows[i].AppleBound = rows[i].User.AppleSub != nil
	}
	return rows, total, err
}

// MessageRow 后台消息列表行(含发送者昵称/openid)。
type MessageRow struct {
	MessageID      int64     `json:"message_id,string"`
	ChatID         int64     `json:"chat_id,string"`
	SenderID       int64     `json:"sender_id,string"`
	SenderNickname string    `json:"sender_nickname"`
	SenderOpenID   string    `json:"sender_openid"`
	IsRobot        bool      `json:"is_robot"`
	Content        string    `json:"content"`
	Type           string    `json:"type"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListMessages 后台消息列表，按发送者 用户ID / 昵称 / openid / 机器人 过滤。
// robotFilter: "robot"=仅机器人 "human"=仅真实用户 ""=不限。
func (s *Service) ListMessages(tenantID, userID int64, nickname, openid, robotFilter string, page, size int) ([]MessageRow, int64, error) {
	if size <= 0 || size > 100 {
		size = 20
	}
	if page < 1 {
		page = 1
	}
	where := "1=1"
	args := []interface{}{}
	if tenantID > 0 {
		where += " AND m.tenant_id = ?"
		args = append(args, tenantID)
	}
	if userID > 0 {
		where += " AND m.sender_id = ?"
		args = append(args, userID)
	}
	if nickname != "" {
		where += " AND u.nickname LIKE ?"
		args = append(args, "%"+nickname+"%")
	}
	if openid != "" {
		where += " AND (u.wx_openid LIKE ? OR u.alipay_uid LIKE ?)"
		args = append(args, "%"+openid+"%", "%"+openid+"%")
	}
	switch robotFilter {
	case "robot":
		where += " AND u.is_robot = 1"
	case "human":
		where += " AND u.is_robot = 0"
	}

	var total int64
	countSQL := `SELECT COUNT(*) FROM messages m JOIN users u ON u.user_id = m.sender_id WHERE ` + where
	if err := s.db.Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	listSQL := `SELECT m.message_id, m.chat_id, m.sender_id,
		       u.nickname AS sender_nickname,
		       COALESCE(NULLIF(u.wx_openid, ''), u.alipay_uid) AS sender_openid,
		       u.is_robot AS is_robot,
		       m.content, m.type, m.created_at
		FROM messages m
		JOIN users u ON u.user_id = m.sender_id
		WHERE ` + where + `
		ORDER BY m.created_at DESC
		LIMIT ? OFFSET ?`
	listArgs := append(append([]interface{}{}, args...), size, (page-1)*size)

	var rows []MessageRow
	if err := s.db.Raw(listSQL, listArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) BanUser(userID int64, ban bool) error {
	status := "active"
	if ban {
		status = "banned"
	}
	return s.db.Model(&model.User{}).Where("user_id = ?", userID).
		Update("status", status).Error
}

func (s *Service) MuteUser(userID int64, mute bool) error {
	return s.db.Model(&model.User{}).Where("user_id = ?", userID).
		Update("is_muted", mute).Error
}

// UpdateUserTags 全量覆盖用户标签(逗号分隔,已清洗)。
func (s *Service) UpdateUserTags(userID int64, raw string) error {
	tags, err := sanitizeTags(raw)
	if err != nil {
		return err
	}
	return s.db.Model(&model.User{}).Where("user_id = ?", userID).
		Update("tags", tags).Error
}

func (s *Service) StartRobotChat(tenantID, userID, botUserID int64) (int64, error) {
	// 找或建聊天
	var existing model.Chat
	err := s.db.Where(
		"tenant_id = ? AND ((user_a = ? AND user_b = ?) OR (user_a = ? AND user_b = ?))",
		tenantID, userID, botUserID, botUserID, userID,
	).First(&existing).Error
	var chatID int64
	if err == nil {
		chatID = existing.ChatID
	} else {
		// 免费建会话（机器人不扣币）
		c := model.Chat{
			ChatID:   idgen.Next(),
			TenantID: tenantID,
			UserA:    userID,
			UserB:    botUserID,
		}
		if err := s.db.Create(&c).Error; err != nil {
			return 0, err
		}
		chatID = c.ChatID
	}

	// 机器人发开场白
	if s.chatSvc != nil {
		var bot model.User
		if s.db.First(&bot, "user_id = ?", botUserID).Error == nil {
			greeting := fmt.Sprintf("嗨～我是%s，很高兴认识你 😊", bot.Nickname)
			_, _ = s.chatSvc.SendMessage(tenantID, botUserID, chatID, greeting, "text")
		}
	}
	// 触发路径 C，让 AI 后续接管对话
	robot.EnqueueBotReply(robot.BotJob{
		TenantID: tenantID, ChatID: chatID,
		BotUserID: botUserID, UserID: userID,
		UserMsg: "你好",
	})
	return chatID, nil
}

func (s *Service) defaultTenantID() int64 {
	var t model.Tenant
	s.db.Where("status = ?", "active").First(&t)
	return t.TenantID
}

// TenantRow 租户列表行(附各平台登录 AppID,来自凭证表)。
type TenantRow struct {
	model.Tenant
	WxAppID     string `gorm:"-" json:"wx_appid"`
	AlipayAppID string `gorm:"-" json:"alipay_appid"`
	AppAppID    string `gorm:"-" json:"app_appid"` // platform=app 行:App 端 APP_ID
}

func (s *Service) ListTenants() ([]TenantRow, error) {
	var tenants []model.Tenant
	if err := s.db.Order("tenant_id asc").Find(&tenants).Error; err != nil {
		return nil, err
	}
	var creds []model.AppCredential
	s.db.Select("tenant_id, platform, app_id").Where("status = ?", "active").Find(&creds)
	wx := map[int64]string{}
	ali := map[int64]string{}
	app := map[int64]string{}
	for _, cr := range creds {
		switch cr.Platform {
		case "wx":
			wx[cr.TenantID] = cr.AppID
		case "alipay":
			ali[cr.TenantID] = cr.AppID
		case "app":
			app[cr.TenantID] = cr.AppID
		}
	}
	out := make([]TenantRow, len(tenants))
	for i, t := range tenants {
		out[i] = TenantRow{Tenant: t, WxAppID: wx[t.TenantID], AlipayAppID: ali[t.TenantID], AppAppID: app[t.TenantID]}
	}
	return out, nil
}

// CreateTenant 新建租户并绑定一条登录凭证:小程序是 wx 行(appid+secret 加密),App 是 app 行(仅 appid)。
// 用户用该 appid 登录即解析到此租户;支付等其余字段后续在「凭证管理」补。
func (s *Service) CreateTenant(tenantType, name, appid, secret string) (model.Tenant, error) {
	name = strings.TrimSpace(name)
	appid = strings.TrimSpace(appid)
	secret = strings.TrimSpace(secret)
	tenantType = normalizeTenantType(tenantType)
	if err := validateTenantInput(tenantType, name, appid, secret); err != nil {
		return model.Tenant{}, err
	}

	now := time.Now()
	t := model.Tenant{TenantID: idgen.Next(), Name: name, Status: "active", Type: tenantType, CreatedAt: now}

	// 没填 appid 到此为止,也不必 reload 凭证缓存——这次压根没往里加东西。
	if !wantsLoginCredential(tenantType, appid) {
		if err := s.db.Create(&t).Error; err != nil {
			return model.Tenant{}, err
		}
		return t, nil
	}

	platform := loginCredentialPlatform(tenantType)
	// appid 全局唯一(uk_plat_appid),先查重给出友好提示
	var n int64
	s.db.Model(&model.AppCredential{}).Where("platform = ? AND app_id = ?", platform, appid).Count(&n)
	if n > 0 {
		return model.Tenant{}, fmt.Errorf("该 AppID 已被其他租户占用")
	}
	cred := model.AppCredential{
		ID: idgen.Next(), TenantID: t.TenantID, Platform: platform, AppID: appid,
		Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	if secret != "" { // app 行没有 secret(它只做 appid→租户映射)
		enc, err := crypto.Encrypt(secret)
		if err != nil {
			return model.Tenant{}, err
		}
		cred.SecretEnc = enc
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&t).Error; err != nil {
			return err
		}
		return tx.Create(&cred).Error
	}); err != nil {
		return model.Tenant{}, err
	}
	if s.credStore != nil {
		_ = s.credStore.Reload() // 让新凭证立即生效
	}
	return t, nil
}

// UpdateTenant 编辑租户:名称/类型/状态必改;appid 有值则更新登录凭证行(小程序 wx / App app;secret 留空保持不变)。
func (s *Service) UpdateTenant(tenantID int64, tenantType, name, status, appid, secret string) error {
	name = strings.TrimSpace(name)
	appid = strings.TrimSpace(appid)
	secret = strings.TrimSpace(secret)
	tenantType = normalizeTenantType(tenantType)
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("租户名不能为空")
	}
	if err := validateTenantStatus(status); err != nil {
		return err
	}
	if err := s.db.Model(&model.Tenant{}).Where("tenant_id = ?", tenantID).
		Updates(map[string]interface{}{"name": name, "status": status, "type": tenantType}).Error; err != nil {
		return err
	}
	if appid != "" {
		platform := loginCredentialPlatform(tenantType)
		// appid 全局唯一:排除本租户后查重
		var n int64
		s.db.Model(&model.AppCredential{}).
			Where("platform = ? AND app_id = ? AND tenant_id <> ?", platform, appid, tenantID).Count(&n)
		if n > 0 {
			return fmt.Errorf("该 AppID 已被其他租户占用")
		}
		upd := map[string]interface{}{"app_id": appid, "updated_at": time.Now()}
		if secret != "" {
			enc, err := crypto.Encrypt(secret)
			if err != nil {
				return err
			}
			upd["secret_enc"] = enc
		}
		var cred model.AppCredential
		err := s.db.Where("tenant_id = ? AND platform = ?", tenantID, platform).First(&cred).Error
		if err == nil {
			if e := s.db.Model(&model.AppCredential{}).Where("id = ?", cred.ID).Updates(upd).Error; e != nil {
				return e
			}
		} else {
			// 无登录凭证则新建。小程序行此时 secret 必填;app 行没有 secret。
			if platform == "wx" && secret == "" {
				return fmt.Errorf("该租户尚无 wx 凭证,首次配置需同时填写 Secret")
			}
			row := model.AppCredential{
				ID: idgen.Next(), TenantID: tenantID, Platform: platform, AppID: appid,
				Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			if secret != "" {
				enc, _ := crypto.Encrypt(secret)
				row.SecretEnc = enc
			}
			if e := s.db.Create(&row).Error; e != nil {
				return e
			}
		}
		if s.credStore != nil {
			_ = s.credStore.Reload()
		}
	}
	return nil
}

// Login 校验用户名密码,成功返回账号(并更新最后登录时间)。
func (s *Service) Login(username, password string) (*model.AdminUser, error) {
	var a model.AdminUser
	if err := s.db.Where("username = ? AND status = ?", username, "active").First(&a).Error; err != nil {
		return nil, errInvalid
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PwdHash), []byte(password)) != nil {
		return nil, errInvalid
	}
	s.db.Model(&a).Update("last_login", time.Now())
	return &a, nil
}

// ChangePassword 修改指定管理员密码。
func (s *Service) ChangePassword(adminID int64, oldPwd, newPwd string) error {
	var a model.AdminUser
	if err := s.db.First(&a, adminID).Error; err != nil {
		return errInvalid
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PwdHash), []byte(oldPwd)) != nil {
		return errInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Model(&a).Update("pwd_hash", string(hash)).Error
}

// tenantTypeOf 查租户形态。tenantID=0 是「全部租户」(在配全局默认),返回空串。
//
// 空串在 visibleFor 里意味着「全都显示」—— 配全局默认时必须看得见全部,
// 否则小程序那半边的默认值就再也改不了了。
func (s *Service) tenantTypeOf(tenantID int64) string {
	if tenantID == 0 {
		return ""
	}
	var t model.Tenant
	if err := s.db.Select("type").First(&t, "tenant_id = ?", tenantID).Error; err != nil {
		return "" // 查不到就别过滤,宁可多显示也不要让人找不到配置项
	}
	return t.Type
}

// Config 返回该租户可见的白名单配置项 + 当前值,标签按 lang 解析。
//
// 按租户形态过滤:小程序租户看不到 App 日志,App 租户也看不到微信流量主。
// 全量配置对单个租户来说有一半是噪音。
func (s *Service) Config(tenantID int64, lang i18n.Lang) []ConfigField {
	tenantType := s.tenantTypeOf(tenantID)
	out := make([]ConfigField, 0, len(configMeta))
	for _, m := range configMeta {
		if !visibleFor(effectivePlatform(m, groupMeta[m.Group]), tenantType) {
			continue
		}
		out = append(out, resolveField(m, sysconfig.GetString(tenantID, m.Key), lang))
	}
	return out
}

// SetConfig 写入单个配置(仅白名单键),并热刷新。
// SetConfig 写入单个配置。clear=true 时强制清空(机密项唯一的清除途径)。
//
// 机密项写空默认被丢弃:前端显示的是掩码不是真值,一次无意的聚焦触发 blur
// 就会提交空串。要真清掉得走 clear,那是个明确的动作。
func (s *Service) SetConfig(tenantID int64, key, value string, clear bool) error {
	if !allowedKey(key) {
		return errBadKey
	}
	if !clear && shouldSkipWrite(isSecretKey(key), value) {
		return nil
	}
	if clear {
		value = ""
	}
	if err := sysconfig.Set(tenantID, key, value); err != nil {
		return err
	}
	_ = sysconfig.Reload()
	return nil
}

// Stats 后台仪表盘概览。
type Stats struct {
	Users        int64   `json:"users"`
	Bottles      int64   `json:"bottles"`
	BottlesToday int64   `json:"bottles_today"`
	Replies      int64   `json:"replies"`
	Chats        int64   `json:"chats"`
	Orders       int64   `json:"orders"`        // 已支付订单数
	RechargeYuan float64 `json:"recharge_yuan"` // 累计充值金额(元,仅已支付)
	Robots       int64   `json:"robots"`
}

// Stats 各计数跟随租户口径(tenantID=0 才是全租户汇总),与概览 KPI 一致。
func (s *Service) Stats(tenantID int64) Stats {
	var st Stats
	dayStart := time.Now().Truncate(24 * time.Hour)
	scope(s.db.Model(&model.User{}), tenantID).Count(&st.Users)
	scope(s.db.Model(&model.Bottle{}), tenantID).Count(&st.Bottles)
	scope(s.db.Model(&model.Bottle{}).Where("created_at >= ?", dayStart), tenantID).Count(&st.BottlesToday)
	scope(s.db.Model(&model.BottleReply{}), tenantID).Count(&st.Replies)
	scope(s.db.Model(&model.Chat{}), tenantID).Count(&st.Chats)
	// 充值订单只算已支付;累计充值金额同口径。
	scope(s.db.Model(&model.PayOrder{}).Where("status = 'paid'"), tenantID).Count(&st.Orders)
	var fen int64
	scope(s.db.Model(&model.PayOrder{}).Where("status = 'paid'"), tenantID).Select("COALESCE(SUM(price_fen),0)").Scan(&fen)
	st.RechargeYuan = round2(float64(fen) / 100)
	st.Robots = sysconfig.GetInt64(tenantID, sysconfig.KeyRobotCount)
	return st
}

func (s *Service) ListRobotContent(tenantID int64, typ string) ([]model.RobotContent, error) {
	q := s.db.Model(&model.RobotContent{})
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	var list []model.RobotContent
	return list, q.Order("id desc").Find(&list).Error
}

func (s *Service) CreateRobotContent(tenantID int64, typ, text, tags string, weight int) error {
	if weight <= 0 {
		weight = 1
	}
	return s.db.Create(&model.RobotContent{
		TenantID: tenantID, Type: typ, Text: text, Tags: tags, Weight: weight, CreatedAt: time.Now(),
	}).Error
}

func (s *Service) UpdateRobotContent(id int64, text, tags string, weight int) error {
	updates := map[string]interface{}{"text": text, "tags": tags}
	if weight > 0 {
		updates["weight"] = weight
	}
	return s.db.Model(&model.RobotContent{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Service) DeleteRobotContent(id int64) error {
	return s.db.Delete(&model.RobotContent{}, id).Error
}

// credPlatforms 凭证行允许的平台:小程序 wx / alipay、App 租户映射 app。
//
// App 的第三方登录(原 wx_app / alipay_app)已迁至「服务商 → 登录」页,
// 不再从这里维护;已有的行保留但不可编辑,确认稳定后由人工清理。
var credPlatforms = map[string]bool{"wx": true, "alipay": true, "app": true}

func validCredPlatform(p string) bool { return credPlatforms[p] }

// CredRow 脱敏后的凭证行。
type CredRow struct {
	ID                int64  `json:"id,string"`
	TenantID          int64  `json:"tenant_id,string"`
	TenantName        string `json:"tenant_name"`
	Platform          string `json:"platform"`
	AppID             string `json:"appid"`
	MchID             string `json:"mch_id"`
	PaySerialNo       string `json:"pay_serial_no"`
	PayPlatformSerial string `json:"pay_platform_serial"`
	NotifyURL         string `json:"notify_url"`
	AliPublicKey      string `json:"ali_pub_key"`
	SecretSet         bool   `json:"secret_set"`
	PayAPIv3KeySet    bool   `json:"pay_apiv3_key_set"`
	PayPrivKeySet     bool   `json:"pay_priv_key_set"`
	PayPlatformKeySet bool   `json:"pay_platform_key_set"`
	AliPrivKeySet     bool   `json:"ali_priv_key_set"`
}

func (s *Service) ListCredentials() ([]CredRow, error) {
	var rows []model.AppCredential
	if err := s.db.Order("tenant_id asc, platform asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	var tenants []model.Tenant
	s.db.Select("tenant_id, name").Find(&tenants)
	names := make(map[int64]string, len(tenants))
	for _, t := range tenants {
		names[t.TenantID] = t.Name
	}
	out := make([]CredRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, CredRow{
			ID: r.ID, TenantID: r.TenantID, TenantName: names[r.TenantID], Platform: r.Platform, AppID: r.AppID,
			MchID: r.MchID, PaySerialNo: r.PaySerialNo, PayPlatformSerial: r.PayPlatformSerial,
			NotifyURL: r.NotifyURL, AliPublicKey: r.AlipayPublicKey,
			SecretSet:         r.SecretEnc != "",
			PayAPIv3KeySet:    r.PayAPIv3KeyEnc != "",
			PayPrivKeySet:     r.PayPrivateKeyEnc != "",
			PayPlatformKeySet: r.PayPlatformKey != "",
			AliPrivKeySet:     r.AlipayPrivateKeyEnc != "",
		})
	}
	return out, nil
}

// CreateCredReq 给租户新建一行凭证(密钥之后用 UpdateCredential 填)。
type CreateCredReq struct {
	TenantID int64  `json:"tenant_id,string" binding:"required"`
	Platform string `json:"platform" binding:"required"`
	AppID    string `json:"appid" binding:"required"`
}

// CreateCredential 新建凭证行。(platform, appid) 全局唯一;建完刷新 credstore 让登录 / 支付立刻能查到。
func (s *Service) CreateCredential(req CreateCredReq) error {
	if !validCredPlatform(req.Platform) {
		return fmt.Errorf("不支持的平台")
	}
	appid := strings.TrimSpace(req.AppID)
	var n int64
	s.db.Model(&model.Tenant{}).Where("tenant_id = ?", req.TenantID).Count(&n)
	if n == 0 {
		return fmt.Errorf("租户不存在")
	}
	s.db.Model(&model.AppCredential{}).Where("platform = ? AND app_id = ?", req.Platform, appid).Count(&n)
	if n > 0 {
		return fmt.Errorf("该 AppID 已被占用")
	}
	now := time.Now()
	if err := s.db.Create(&model.AppCredential{
		ID: idgen.Next(), TenantID: req.TenantID, Platform: req.Platform, AppID: appid,
		Status: "active", CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		return err
	}
	if s.credStore != nil {
		return s.credStore.Reload()
	}
	return nil
}

type UpdateCredReq struct {
	AppID             string `json:"appid"`
	MchID             string `json:"mch_id"`
	PaySerialNo       string `json:"pay_serial_no"`
	PayPlatformSerial string `json:"pay_platform_serial"`
	NotifyURL         string `json:"notify_url"`
	Secret            string `json:"secret"`
	PayAPIv3Key       string `json:"pay_apiv3_key"`
	PayPrivateKey     string `json:"pay_private_key"`
	PayPlatformKey    string `json:"pay_platform_key"`
	AliPrivateKey     string `json:"ali_private_key"`
	AliPublicKey      string `json:"ali_pub_key"`
}

func (s *Service) UpdateCredential(id int64, req UpdateCredReq) error {
	var row model.AppCredential
	if err := s.db.First(&row, id).Error; err != nil {
		return err
	}
	updates := map[string]interface{}{}
	set := func(key, val string) {
		if val != "" {
			updates[key] = val
		}
	}
	enc := func(key, val string) error {
		if val == "" {
			return nil
		}
		e, err := crypto.Encrypt(val)
		if err != nil {
			return err
		}
		updates[key] = e
		return nil
	}
	set("app_id", req.AppID)
	set("mch_id", req.MchID)
	set("pay_serial_no", req.PaySerialNo)
	set("pay_platform_serial", req.PayPlatformSerial)
	set("pay_platform_key", req.PayPlatformKey)
	set("notify_url", req.NotifyURL)
	set("alipay_public_key", req.AliPublicKey)
	if err := enc("secret_enc", req.Secret); err != nil {
		return err
	}
	if err := enc("pay_apiv3_key_enc", req.PayAPIv3Key); err != nil {
		return err
	}
	if err := enc("pay_private_key_enc", req.PayPrivateKey); err != nil {
		return err
	}
	if err := enc("alipay_private_key_enc", req.AliPrivateKey); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	if err := s.db.Model(&model.AppCredential{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	if s.credStore != nil {
		return s.credStore.Reload()
	}
	return nil
}

// PersonaRow is the list/edit view for persona configs.
type PersonaRow struct {
	PersonaID        int64  `json:"persona_id,string"`
	TenantID         int64  `json:"tenant_id,string"`
	Name             string `json:"name"`
	RelationshipRole string `json:"relationship_role"`
	AffectiveStyle   string `json:"affective_style"`
	VoiceStyle       string `json:"voice_style"`
	RulesJSON        string `json:"rules_json"`
	Status           string `json:"status"`
}

func (s *Service) ListPersonas(tenantID int64) ([]model.PersonaConfig, error) {
	var list []model.PersonaConfig
	q := s.db.Model(&model.PersonaConfig{})
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	return list, q.Order("persona_id desc").Find(&list).Error
}

func (s *Service) CreatePersona(req PersonaRow) error {
	now := time.Now()
	return s.db.Create(&model.PersonaConfig{
		PersonaID:        idgen.Next(),
		TenantID:         req.TenantID,
		Name:             req.Name,
		RelationshipRole: req.RelationshipRole,
		AffectiveStyle:   req.AffectiveStyle,
		VoiceStyle:       req.VoiceStyle,
		RulesJSON:        req.RulesJSON,
		Status:           req.Status,
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error
}

// PersonaTemplateCount 返回模板租户(默认租户)可复制的人格数,给批量弹窗设上限。
// isTemplateTenant=true 表示当前就站在模板租户上,此时不能批量复制。
func (s *Service) PersonaTemplateCount(dstTenantID int64) (available int64, isTemplateTenant bool) {
	src := s.defaultTenantID()
	s.db.Model(&model.PersonaConfig{}).
		Where("tenant_id = ? AND bot_user_id IS NULL", src).Count(&available)
	return available, src == dstTenantID
}

// BatchCreatePersonas 从默认租户的人格模板批量复制到目标租户。
// 取模板前 count 个(与列表页同序 persona_id desc),跳过目标租户已存在的同名人格,幂等。
func (s *Service) BatchCreatePersonas(dstTenantID int64, count int) (created, skipped int, err error) {
	if dstTenantID <= 0 {
		return 0, 0, fmt.Errorf("请先在右上角选择具体租户")
	}
	if count <= 0 {
		return 0, 0, fmt.Errorf("数量必须大于 0")
	}
	srcTenantID := s.defaultTenantID()
	if srcTenantID == dstTenantID {
		return 0, 0, fmt.Errorf("当前已是模板租户,请切换到其他租户")
	}

	var templates []model.PersonaConfig
	if err = s.db.Where("tenant_id = ? AND bot_user_id IS NULL", srcTenantID).
		Order("persona_id desc").Limit(count).Find(&templates).Error; err != nil {
		return 0, 0, err
	}
	if len(templates) == 0 {
		return 0, 0, fmt.Errorf("模板租户暂无可复制的人格")
	}

	var names []string
	if err = s.db.Model(&model.PersonaConfig{}).
		Where("tenant_id = ?", dstTenantID).Pluck("name", &names).Error; err != nil {
		return 0, 0, err
	}
	exists := make(map[string]bool, len(names))
	for _, n := range names {
		exists[n] = true
	}

	now := time.Now()
	rows := make([]model.PersonaConfig, 0, len(templates))
	for _, t := range templates {
		if exists[t.Name] {
			skipped++
			continue
		}
		// BotUserID 留 nil:复制出来的仍是模板,待「机器人档案」页手动绑定
		rows = append(rows, model.PersonaConfig{
			PersonaID:        idgen.Next(),
			TenantID:         dstTenantID,
			Name:             t.Name,
			RelationshipRole: t.RelationshipRole,
			AffectiveStyle:   t.AffectiveStyle,
			VoiceStyle:       t.VoiceStyle,
			RulesJSON:        t.RulesJSON,
			Status:           t.Status,
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}
	if len(rows) > 0 {
		if err = s.db.Create(&rows).Error; err != nil {
			return 0, 0, err
		}
	}
	return len(rows), skipped, nil
}

func (s *Service) UpdatePersona(id int64, req PersonaRow) error {
	updates := map[string]interface{}{
		"name":              req.Name,
		"relationship_role": req.RelationshipRole,
		"affective_style":   req.AffectiveStyle,
		"voice_style":       req.VoiceStyle,
		"rules_json":        req.RulesJSON,
		"updated_at":        time.Now(),
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	return s.db.Model(&model.PersonaConfig{}).Where("persona_id = ?", id).Updates(updates).Error
}

func (s *Service) DeletePersona(id int64) error {
	return s.db.Model(&model.PersonaConfig{}).Where("persona_id = ?", id).
		Updates(map[string]interface{}{"status": "paused", "updated_at": time.Now()}).Error
}

// RobotProfileRow is the robot user list/edit view.
type RobotProfileRow struct {
	UserID      int64  `json:"user_id,string"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	Gender      int8   `json:"gender"`
	Age         int    `json:"age"`
	City        string `json:"city"`
	Language    string `json:"language"`  // 逗号分隔,取值同 App 语言选项("中文"/"English")
	Interests   string `json:"interests"` // 逗号分隔的兴趣 key
	Bio         string `json:"bio"`
	Status      string `json:"status"`
	PersonaID   *int64 `json:"persona_id"`
	PersonaName string `json:"persona_name"`
}

func (s *Service) ListRobotProfiles(tenantID int64) ([]RobotProfileRow, error) {
	var rows []RobotProfileRow
	sql := `SELECT u.user_id, u.nickname, u.avatar, u.gender, u.age, u.city, u.language, u.interests, u.bio, u.status,
			       u.persona_id, COALESCE(p.name,'') AS persona_name
			FROM users u
			LEFT JOIN persona_configs p ON p.persona_id = u.persona_id
			WHERE u.is_robot = 1`
	var args []interface{}
	if tenantID > 0 {
		sql += " AND u.tenant_id = ?"
		args = append(args, tenantID)
	}
	sql += " ORDER BY u.user_id DESC"
	return rows, s.db.Raw(sql, args...).Scan(&rows).Error
}

// RobotProfileUpdate 后台可改的机器人资料;nil = 不改。
type RobotProfileUpdate struct {
	PersonaID *int64
	Status    string
	Nickname  *string
	Gender    *int8
	Age       *int
	City      *string
	Language  *string
	Interests *string
	Bio       *string
}

func (s *Service) UpdateRobotProfile(userID int64, in RobotProfileUpdate) error {
	updates := map[string]interface{}{}
	if in.PersonaID != nil {
		updates["persona_id"] = *in.PersonaID
	}
	if in.Status != "" {
		updates["status"] = in.Status
	}
	if in.Nickname != nil {
		if n := strings.TrimSpace(*in.Nickname); n != "" {
			updates["nickname"] = n
		}
	}
	if in.Gender != nil {
		updates["gender"] = *in.Gender
	}
	if in.Age != nil && *in.Age >= 18 && *in.Age <= 80 {
		updates["age"] = *in.Age
	}
	if in.City != nil {
		updates["city"] = strings.TrimSpace(*in.City)
	}
	if in.Language != nil {
		updates["language"] = strings.TrimSpace(*in.Language)
	}
	if in.Interests != nil {
		updates["interests"] = strings.TrimSpace(*in.Interests)
	}
	if in.Bio != nil {
		updates["bio"] = strings.TrimSpace(*in.Bio)
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.Model(&model.User{}).Where("user_id = ? AND is_robot = 1", userID).Updates(updates).Error
}

// CreateRobots 后台按语言批量新增机器人(昵称/城市/兴趣/简介随语言生成),返回新建的行。
func (s *Service) CreateRobots(tenantID int64, count int, lang string, gender int8, city string) ([]model.User, error) {
	if count < 1 || count > 50 {
		return nil, errs.New(errs.CodeBadRequest, "数量须在 1~50 之间")
	}
	out := make([]model.User, 0, count)
	for i := 0; i < count; i++ {
		u := robot.GenerateRobotUser(tenantID, robot.GenOptions{Lang: lang, Gender: gender, City: city})
		if err := s.db.Create(&u).Error; err != nil {
			return out, err
		}
		out = append(out, u)
	}
	return out, nil
}

func (s *Service) ListKeywordRules(tenantID int64) ([]model.RobotKeywordRule, error) {
	var list []model.RobotKeywordRule
	q := s.db.Model(&model.RobotKeywordRule{})
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	return list, q.Order("priority asc, rule_id desc").Find(&list).Error
}

func (s *Service) CreateKeywordRule(req model.RobotKeywordRule) error {
	now := time.Now()
	req.RuleID = idgen.Next()
	req.CreatedAt = now
	req.UpdatedAt = now
	return s.db.Create(&req).Error
}

func (s *Service) UpdateKeywordRule(id int64, req model.RobotKeywordRule) error {
	updates := map[string]interface{}{
		"match_type":     req.MatchType,
		"keywords_json":  req.KeywordsJSON,
		"responses_json": req.ResponsesJSON,
		"priority":       req.Priority,
		"updated_at":     time.Now(),
	}
	return s.db.Model(&model.RobotKeywordRule{}).Where("rule_id = ?", id).Updates(updates).Error
}

func (s *Service) DeleteKeywordRule(id int64) error {
	return s.db.Delete(&model.RobotKeywordRule{}, id).Error
}

func (s *Service) ToggleKeywordRule(id int64, status string) error {
	return s.db.Model(&model.RobotKeywordRule{}).Where("rule_id = ?", id).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()}).Error
}

// ===== 充值档位 =====

func (s *Service) ListPackages() ([]model.CoinPackage, error) {
	var list []model.CoinPackage
	err := s.db.Order("sort asc, package_id asc").Find(&list).Error
	return list, err
}

// CreatePackage 用小自增 ID(max+1),与种子一致;避免雪花大 ID 在前端 JS 丢精度、破坏下单。
func (s *Service) CreatePackage(req model.CoinPackage) error {
	var maxID int64
	s.db.Model(&model.CoinPackage{}).Select("COALESCE(MAX(package_id),0)").Scan(&maxID)
	req.PackageID = maxID + 1
	if req.Status == "" {
		req.Status = "active"
	}
	return s.db.Create(&req).Error
}

func (s *Service) UpdatePackage(id int64, req model.CoinPackage) error {
	return s.db.Model(&model.CoinPackage{}).Where("package_id = ?", id).Updates(map[string]interface{}{
		"name":        req.Name,
		"coins":       req.Coins,
		"bonus_coins": req.BonusCoins,
		"price_fen":   req.PriceFen,
		"sort":        req.Sort,
		"status":      req.Status,
		// 商品 ID 之前漏在白名单外,导致后台改了也写不进去 ——
		// 而 iap.go 正是按 ios_product_id 反查档位的,空着就永远入账失败。
		"ios_product_id":  req.IOSProductID,
		"play_product_id": req.PlayProductID,
	}).Error
}

func (s *Service) DeletePackage(id int64) error {
	return s.db.Delete(&model.CoinPackage{}, "package_id = ?", id).Error
}

// ===== 道具/礼物 =====

func (s *Service) ListItems() ([]model.Item, error) {
	var list []model.Item
	err := s.db.Order("sort asc, item_id asc").Find(&list).Error
	return list, err
}

// CreateItem 小自增 ID(max+1),避免雪花大 ID 在前端 JS 丢精度、破坏购买。
func (s *Service) CreateItem(req model.Item) error {
	var maxID int64
	s.db.Model(&model.Item{}).Select("COALESCE(MAX(item_id),0)").Scan(&maxID)
	req.ItemID = maxID + 1
	if req.Status == "" {
		req.Status = "active"
	}
	return s.db.Create(&req).Error
}

func (s *Service) UpdateItem(id int64, req model.Item) error {
	return s.db.Model(&model.Item{}).Where("item_id = ?", id).Updates(map[string]interface{}{
		"name":       req.Name,
		"type":       req.Type,
		"icon":       req.Icon,
		"price_coin": req.PriceCoin,
		"sort":       req.Sort,
		"status":     req.Status,
	}).Error
}

func (s *Service) DeleteItem(id int64) error {
	return s.db.Delete(&model.Item{}, "item_id = ?", id).Error
}

func (s *Service) ListReplyCache(tenantID int64, personaRole string, page, size int) ([]model.RobotReplyCache, int64, error) {
	if size > 50 {
		size = 50
	}
	q := s.db.Model(&model.RobotReplyCache{})
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if personaRole != "" {
		q = q.Where("persona_role = ?", personaRole)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.RobotReplyCache
	offset := (page - 1) * size
	err := q.Order("hit_count desc").Offset(offset).Limit(size).Find(&list).Error
	return list, total, err
}

func (s *Service) UpdateReplyCacheVariants(id int64, responsesJSON string) error {
	return s.db.Model(&model.RobotReplyCache{}).Where("cache_id = ?", id).
		Updates(map[string]interface{}{
			"responses_json": responsesJSON,
			"source":         "manual",
			"updated_at":     time.Now(),
		}).Error
}

func (s *Service) ToggleReplyCache(id int64, status string) error {
	return s.db.Model(&model.RobotReplyCache{}).Where("cache_id = ?", id).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()}).Error
}

func (s *Service) DeleteReplyCache(id int64) error {
	return s.db.Delete(&model.RobotReplyCache{}, id).Error
}

// ---- 机器人对话（管理员手动回复）----

type RobotChatItem struct {
	ChatID       int64     `json:"chat_id,string"`
	BotUserID    int64     `json:"bot_user_id,string"`
	BotNickname  string    `json:"bot_nickname"`
	UserID       int64     `json:"user_id,string"`
	UserNickname string    `json:"user_nickname"`
	LastMessage  string    `json:"last_message"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *Service) ListRobotChats(tenantID int64, page, size int) ([]RobotChatItem, int64, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	if page < 1 {
		page = 1
	}
	type raw struct {
		ChatID  int64
		UserA   int64
		UserB   int64
		NickA   string
		RobotA  bool
		NickB   string
		RobotB  bool
		LastMsg string
		Updated time.Time
	}

	// 构建可选的 tenant 过滤条件
	tenantClause := ""
	filterArgs := []interface{}{}
	if tenantID > 0 {
		tenantClause = " AND c.tenant_id = ?"
		filterArgs = append(filterArgs, tenantID)
	}

	listSQL := `SELECT c.chat_id, c.user_a, c.user_b, c.updated_at AS updated,
		       ua.nickname AS nick_a, ua.is_robot AS robot_a,
		       ub.nickname AS nick_b, ub.is_robot AS robot_b,
		       (SELECT content FROM messages WHERE chat_id = c.chat_id ORDER BY created_at DESC LIMIT 1) AS last_msg
		FROM chats c
		JOIN users ua ON ua.user_id = c.user_a
		JOIN users ub ON ub.user_id = c.user_b
		WHERE (ua.is_robot = 1 OR ub.is_robot = 1)` + tenantClause + `
		ORDER BY c.updated_at DESC
		LIMIT ? OFFSET ?`
	listArgs := append(append([]interface{}{}, filterArgs...), size, (page-1)*size)

	var rows []raw
	if err := s.db.Raw(listSQL, listArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	countSQL := `SELECT COUNT(*) FROM chats c
		JOIN users ua ON ua.user_id = c.user_a
		JOIN users ub ON ub.user_id = c.user_b
		WHERE (ua.is_robot = 1 OR ub.is_robot = 1)` + tenantClause
	var total int64
	s.db.Raw(countSQL, filterArgs...).Scan(&total)

	items := make([]RobotChatItem, 0, len(rows))
	for _, r := range rows {
		item := RobotChatItem{ChatID: r.ChatID, LastMessage: r.LastMsg, UpdatedAt: r.Updated}
		if r.RobotA {
			item.BotUserID, item.BotNickname = r.UserA, r.NickA
			item.UserID, item.UserNickname = r.UserB, r.NickB
		} else {
			item.BotUserID, item.BotNickname = r.UserB, r.NickB
			item.UserID, item.UserNickname = r.UserA, r.NickA
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (s *Service) RobotChatMessages(chatID int64, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var msgs []model.Message
	err := s.db.Where("chat_id = ?", chatID).
		Order("created_at asc").Limit(limit).Find(&msgs).Error
	return msgs, err
}

// ---- 会话式消息浏览(全部会话,非仅机器人) ----

type ChatListItem struct {
	ChatID  int64     `json:"chat_id,string"`
	UserA   int64     `json:"user_a,string"`
	UserB   int64     `json:"user_b,string"`
	NickA   string    `json:"nick_a"`
	RobotA  bool      `json:"robot_a"`
	NickB   string    `json:"nick_b"`
	RobotB  bool      `json:"robot_b"`
	LastMsg string    `json:"last_msg"`
	Updated time.Time `json:"updated_at"`
}

// ListChats 全部会话列表,按更新时间倒序;keyword 匹配任一方昵称。
func (s *Service) ListChats(tenantID int64, keyword string, page, size int) ([]ChatListItem, int64, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	if page < 1 {
		page = 1
	}
	where := "1=1"
	args := []interface{}{}
	if tenantID > 0 {
		where += " AND c.tenant_id = ?"
		args = append(args, tenantID)
	}
	if keyword != "" {
		where += " AND (ua.nickname LIKE ? OR ub.nickname LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	countSQL := `SELECT COUNT(*) FROM chats c
		JOIN users ua ON ua.user_id = c.user_a
		JOIN users ub ON ub.user_id = c.user_b
		WHERE ` + where
	if err := s.db.Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	listSQL := `SELECT c.chat_id, c.user_a, c.user_b, c.updated_at AS updated,
		       ua.nickname AS nick_a, ua.is_robot AS robot_a,
		       ub.nickname AS nick_b, ub.is_robot AS robot_b,
		       (SELECT content FROM messages WHERE chat_id = c.chat_id ORDER BY created_at DESC LIMIT 1) AS last_msg
		FROM chats c
		JOIN users ua ON ua.user_id = c.user_a
		JOIN users ub ON ub.user_id = c.user_b
		WHERE ` + where + `
		ORDER BY c.updated_at DESC
		LIMIT ? OFFSET ?`
	listArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	var rows []ChatListItem
	if err := s.db.Raw(listSQL, listArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

type ChatMsg struct {
	MessageID int64     `json:"message_id,string"`
	SenderID  int64     `json:"sender_id,string"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatDetail struct {
	ChatID   int64     `json:"chat_id,string"`
	UserA    int64     `json:"user_a,string"`
	UserB    int64     `json:"user_b,string"`
	NickA    string    `json:"nick_a"`
	NickB    string    `json:"nick_b"`
	RobotA   bool      `json:"robot_a"`
	RobotB   bool      `json:"robot_b"`
	Messages []ChatMsg `json:"messages"`
}

// GetChatDetail 会话双方信息 + 最近 limit 条消息(升序),前端按 sender_id 分左右。
func (s *Service) GetChatDetail(chatID int64, limit int) (*ChatDetail, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var ch model.Chat
	if err := s.db.First(&ch, "chat_id = ?", chatID).Error; err != nil {
		return nil, err
	}
	d := &ChatDetail{ChatID: ch.ChatID, UserA: ch.UserA, UserB: ch.UserB}
	var ua, ub model.User
	if s.db.Select("nickname, is_robot").First(&ua, "user_id = ?", ch.UserA).Error == nil {
		d.NickA, d.RobotA = ua.Nickname, ua.IsRobot
	}
	if s.db.Select("nickname, is_robot").First(&ub, "user_id = ?", ch.UserB).Error == nil {
		d.NickB, d.RobotB = ub.Nickname, ub.IsRobot
	}
	// 取最近 limit 条,再正序展示。
	var msgs []model.Message
	if err := s.db.Where("chat_id = ?", chatID).
		Order("created_at desc").Limit(limit).Find(&msgs).Error; err != nil {
		return nil, err
	}
	d.Messages = make([]ChatMsg, len(msgs))
	for i, m := range msgs {
		// 倒序取出后翻转为正序
		d.Messages[len(msgs)-1-i] = ChatMsg{
			MessageID: m.MessageID, SenderID: m.SenderID,
			Content: m.Content, Type: m.Type, CreatedAt: m.CreatedAt,
		}
	}
	return d, nil
}

func (s *Service) SendAsRobot(chatID int64, content string) error {
	if s.chatSvc == nil {
		return nil
	}
	var ch model.Chat
	if err := s.db.First(&ch, "chat_id = ?", chatID).Error; err != nil {
		return err
	}
	var botUser model.User
	if err := s.db.First(&botUser, "user_id = ? AND is_robot = ?", ch.UserA, true).Error; err != nil {
		if err := s.db.First(&botUser, "user_id = ? AND is_robot = ?", ch.UserB, true).Error; err != nil {
			return err
		}
	}
	_, err := s.chatSvc.SendMessage(ch.TenantID, botUser.UserID, chatID, content, "text")
	return err
}

// GetUserTenant 返回用户所属的租户 ID。
func (s *Service) GetUserTenant(userID int64) int64 {
	var u model.User
	if s.db.Select("tenant_id").First(&u, "user_id = ?", userID).Error != nil {
		return 0
	}
	return u.TenantID
}

// ---- 订单流水 ----

type OrderRow struct {
	OrderNo  string `json:"order_no"`
	TenantID int64  `json:"tenant_id,string"`
	UserID   int64  `json:"user_id,string"`
	Nickname string `json:"nickname"`
	Platform string `json:"platform"`
	// PriceFen 对应 model.PayOrder.PriceMinor（列名仍是 price_fen）。
	// ⚠️ 这里**刻意不跟随改名**：本结构由裸 SQL Scan 填充，GORM 按列名约定
	// 映射字段，改成 PriceMinor 会去找 price_minor 列，找不到就**静默填 0**
	// ——后台订单列表金额全变 0 且不报错。收益很小，风险是静默的，不值得。
	PriceFen    int64      `json:"price_fen"`
	Coins       int64      `json:"coins"`
	Status      string     `json:"status"`
	PlatformTxn string     `json:"platform_txn_id"`
	CreatedAt   time.Time  `json:"created_at"`
	PaidAt      *time.Time `json:"paid_at"`
}

func (s *Service) ListPayOrders(tenantID int64, status, userKw string, page, size int) ([]OrderRow, int64, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	if page < 1 {
		page = 1
	}
	where := "1=1"
	args := []interface{}{}
	if tenantID > 0 {
		where += " AND o.tenant_id = ?"
		args = append(args, tenantID)
	}
	if status != "" {
		where += " AND o.status = ?"
		args = append(args, status)
	}
	if userKw != "" {
		where += " AND (CAST(o.user_id AS CHAR) LIKE ? OR u.nickname LIKE ?)"
		args = append(args, "%"+userKw+"%", "%"+userKw+"%")
	}
	var total int64
	s.db.Raw("SELECT COUNT(*) FROM pay_orders o LEFT JOIN users u ON u.user_id = o.user_id WHERE "+where, args...).Scan(&total)

	listArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	var rows []OrderRow
	err := s.db.Raw(`
		SELECT o.order_no, o.tenant_id, o.user_id, COALESCE(u.nickname,'') AS nickname,
		       o.platform, o.price_fen, o.coins, o.status,
		       o.platform_txn_id, o.created_at, o.paid_at
		FROM pay_orders o
		LEFT JOIN users u ON u.user_id = o.user_id
		WHERE `+where+`
		ORDER BY o.created_at DESC
		LIMIT ? OFFSET ?`, listArgs...).Scan(&rows).Error
	return rows, total, err
}

// ---- 金币流水 ----

type TxnRow struct {
	TxnID        int64     `json:"txn_id,string"`
	TenantID     int64     `json:"tenant_id,string"`
	UserID       int64     `json:"user_id,string"`
	Nickname     string    `json:"nickname"`
	Direction    string    `json:"direction"`
	Coins        int64     `json:"coins"`
	Scene        string    `json:"scene"`
	BizNo        string    `json:"biz_no"`
	BalanceAfter int64     `json:"balance_after"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Service) ListWalletTxns(tenantID int64, direction, scene, userKw, start, end string, page, size int) ([]TxnRow, int64, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	if page < 1 {
		page = 1
	}
	where := "1=1"
	args := []interface{}{}
	if tenantID > 0 {
		where += " AND t.tenant_id = ?"
		args = append(args, tenantID)
	}
	if direction != "" {
		where += " AND t.direction = ?"
		args = append(args, direction)
	}
	if scene != "" {
		where += " AND t.scene = ?"
		args = append(args, scene)
	}
	if userKw != "" {
		where += " AND (CAST(t.user_id AS CHAR) LIKE ? OR u.nickname LIKE ?)"
		args = append(args, "%"+userKw+"%", "%"+userKw+"%")
	}
	if start != "" {
		where += " AND t.created_at >= ?"
		args = append(args, start+" 00:00:00")
	}
	if end != "" {
		where += " AND t.created_at <= ?"
		args = append(args, end+" 23:59:59")
	}
	var total int64
	s.db.Raw("SELECT COUNT(*) FROM wallet_txns t LEFT JOIN users u ON u.user_id = t.user_id WHERE "+where, args...).Scan(&total)

	listArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	var rows []TxnRow
	err := s.db.Raw(`
		SELECT t.txn_id, t.tenant_id, t.user_id, COALESCE(u.nickname,'') AS nickname,
		       t.direction, t.coins, t.scene, t.biz_no, t.balance_after, t.remark, t.created_at
		FROM wallet_txns t
		LEFT JOIN users u ON u.user_id = t.user_id
		WHERE `+where+`
		ORDER BY t.created_at DESC
		LIMIT ? OFFSET ?`, listArgs...).Scan(&rows).Error
	return rows, total, err
}
