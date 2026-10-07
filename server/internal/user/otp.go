package user

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/mailer"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/cache"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// PlatformApp App 端登录平台标识。
//
// App 没有小程序那样的 code2session，只需要用 appid 定位租户——复用 app_credentials
// 表加一行 platform="app" 即可，租户隔离/凭证管理/后台「租户凭证」页全部自动复用。
const PlatformApp = "app"

var phoneRe = regexp.MustCompile(`^[0-9]{6,15}$`)

// emailRe 只做形状校验（有 @、点分域名、无空白）。真正的可达性由「能不能收到码」验证。
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s.]+(\.[^@\s.]+)+$`)

// normalizeEmail 统一小写去空白。
// 邮箱大小写不敏感，不归一会让同一个人注册出两个账号。
func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// 验证码用途。**必须与验证码绑定**，否则「注册」场景发出的码能拿去重置别人的密码——
// 攻击者只要知道受害者邮箱，就能以注册名义要一个码、再用它走重置流程。
const (
	PurposeLogin    = "login"    // 手机号免密登录：不存在则建号
	PurposeRegister = "register" // 注册：标识已存在要拒
	PurposeReset    = "reset"    // 找回密码：标识不存在要拒
)

func validPurpose(p string) bool {
	return p == PurposeLogin || p == PurposeRegister || p == PurposeReset
}

// otpKeys Redis 键。验证码只存 Redis 带 TTL，不落库。
//
// id 是「标识」——E.164 手机号或邮箱地址。
//
// **码键带 purpose，闸门键不带**：这是有意的不对称。
//   - 码按用途隔离 → 注册的码换不成重置的码
//   - 频控按标识合并 → 同一个人换用途/换渠道都绕不开限额
func otpCodeKey(id, purpose string) string { return "otp:code:" + purpose + ":" + id }
func otpFailKey(id, purpose string) string { return "otp:fail:" + purpose + ":" + id }
func otpCooldownKey(id string) string      { return "otp:cd:" + id }
func otpDayKey(id string) string {
	return fmt.Sprintf("otp:day:%s:%s", id, time.Now().UTC().Format("20060102"))
}
func otpIPKey(ip string) string {
	return fmt.Sprintf("otp:ip:%s:%s", ip, time.Now().UTC().Format("2006010215"))
}

// 连续输错这么多次就作废当前验证码，必须重新获取。
const otpMaxFail = 5

// e164 把区号与号码拼成 E.164（+919876543210）。
func e164(dialCode, phone string) string {
	d := strings.TrimSpace(dialCode)
	if !strings.HasPrefix(d, "+") {
		d = "+" + d
	}
	return d + strings.TrimSpace(phone)
}

// resolveAppTenant 解析 App 端租户。
//
// ⚠️ App 使用**独立 tenant_id**（与小程序隔离）：两端用户互不可见，
// App 冷启动是空池子，内容需要靠机器人填充。
//
// 解析顺序：凭证表（platform=app + appid）→ APP_DEFAULT_TENANT_ID → 单租户默认值。
// 中间这一档让 App 端不必维护 app_credentials 行：App 只有一个包，
// 租户是部署期常量，没有小程序那种「一份服务跑 N 个 appid」的需求。
func (s *Service) resolveAppTenant(appid string) (int64, error) {
	if s.creds != nil {
		if r, ok := s.creds.ByAppID(PlatformApp, appid); ok {
			return r.TenantID, nil
		}
	}
	if s.cfg.AppDefaultTenantID != 0 {
		return s.cfg.AppDefaultTenantID, nil
	}
	if s.cfg.MultiTenant {
		return 0, errs.New(errs.CodeLoginFailed, "未配置 App 凭证：app_credentials 需要一行 platform=app，或设 APP_DEFAULT_TENANT_ID")
	}
	// 单租户开发态：回落默认租户，避免未配凭证时无法联调。
	log.Printf("[app] 未找到 appid=%q 的 App 凭证，回落默认租户 %d", appid, s.cfg.DefaultTenantID)
	return s.cfg.DefaultTenantID, nil
}

// SendOTP 发送手机验证码。
func (s *Service) SendOTP(appid, dialCode, phone, ip, purpose string) error {
	if !phoneRe.MatchString(strings.TrimSpace(phone)) {
		return errs.New(errs.CodeBadRequest, "手机号格式不正确")
	}
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return err
	}
	num := e164(dialCode, phone)
	if err := s.guardPurpose(tenantID, appIdentity{Phone: num}, purpose); err != nil {
		return err
	}
	return s.issueOTP(tenantID, num, ip, purpose, func(code string) error {
		return s.sendSMS(tenantID, num, code)
	})
}

// SendEmailOTP 发送邮箱验证码。
//
// 与手机号共用 issueOTP 的全部闸门——邮件虽然不像短信那样按条计费，
// 但发信域名的信誉是会被滥用打坏的，一样得挡住批量注册。
func (s *Service) SendEmailOTP(appid, email, ip, purpose string) error {
	addr := normalizeEmail(email)
	if !emailRe.MatchString(addr) {
		return errs.New(errs.CodeBadRequest, "邮箱格式不正确")
	}
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return err
	}
	if err := s.guardPurpose(tenantID, appIdentity{Email: addr}, purpose); err != nil {
		return err
	}
	return s.issueOTP(tenantID, addr, ip, purpose, func(code string) error {
		return s.sendEmail(tenantID, addr, code, purpose)
	})
}

// guardPurpose 按用途校验标识是否处于正确状态。
//
// ⚠️ 这里的报错会**暴露某个标识是否已注册**（枚举风险）。这是权衡后的选择：
// 不告诉用户「这个号已经注册过了」，注册页就只能在提交时才失败，体验很差。
// 真正要防的是撞库，那靠 issueOTP 的三维频控挡。
func (s *Service) guardPurpose(tenantID int64, id appIdentity, purpose string) error {
	switch purpose {
	case PurposeRegister:
		if s.identityExists(tenantID, id) {
			return errs.New(errs.CodeBadRequest, "该账号已注册，请直接登录")
		}
	case PurposeReset:
		if !s.identityExists(tenantID, id) {
			return errs.New(errs.CodeNotFound, "该账号尚未注册")
		}
	}
	return nil
}

func (s *Service) identityExists(tenantID int64, id appIdentity) bool {
	cond, val := id.where()
	var n int64
	s.db.Model(&model.User{}).
		Where("tenant_id = ?", tenantID).Where(cond, val).
		Where("status <> ?", "deleted").Count(&n)
	return n > 0
}

// issueOTP 发码的公共流程：三维频控 → 生成 → 存 Redis → 交给 deliver 真发。
//
// 三维频控：同标识重发冷却、同标识每日上限、同 IP 每小时上限。
// 验证码是薅羊毛重灾区，这三道闸缺一不可。
// deliver 失败要把码和冷却一起删掉——否则用户没收到却被冷却挡住，只能干等。
func (s *Service) issueOTP(tenantID int64, id, ip, purpose string, deliver func(code string) error) error {
	if !validPurpose(purpose) {
		return errs.New(errs.CodeBadRequest, "参数错误")
	}
	ctx := context.Background()
	rdb := cache.RDB

	// 1) 重发冷却
	resend := sysconfig.GetInt(tenantID, sysconfig.KeyAppOTPResend)
	if resend <= 0 {
		resend = 60
	}
	if n, err := rdb.Exists(ctx, otpCooldownKey(id)).Result(); err == nil && n > 0 {
		ttl, _ := rdb.TTL(ctx, otpCooldownKey(id)).Result()
		return errs.New(errs.CodeRateLimited, fmt.Sprintf("请 %d 秒后再试", int(ttl.Seconds())+1))
	}

	// 2) 单标识日限
	if cap := sysconfig.GetInt(tenantID, sysconfig.KeyAppOTPDailyCap); cap > 0 {
		cnt, err := rdb.Incr(ctx, otpDayKey(id)).Result()
		if err == nil {
			if cnt == 1 {
				rdb.Expire(ctx, otpDayKey(id), 24*time.Hour)
			}
			if cnt > int64(cap) {
				return errs.New(errs.CodeRateLimited, "今日获取次数已达上限")
			}
		}
	}

	// 3) 单 IP 小时限：挡批量注册
	if ip != "" {
		if cap := sysconfig.GetInt(tenantID, sysconfig.KeyAppOTPIPCap); cap > 0 {
			cnt, err := rdb.Incr(ctx, otpIPKey(ip)).Result()
			if err == nil {
				if cnt == 1 {
					rdb.Expire(ctx, otpIPKey(ip), time.Hour)
				}
				if cnt > int64(cap) {
					return errs.New(errs.CodeRateLimited, "操作太频繁，请稍后再试")
				}
			}
		}
	}

	code := genOTPCode()
	ttl := sysconfig.GetInt(tenantID, sysconfig.KeyAppOTPTTL)
	if ttl <= 0 {
		ttl = 300
	}
	if err := rdb.Set(ctx, otpCodeKey(id, purpose), code, time.Duration(ttl)*time.Second).Err(); err != nil {
		return errs.New(errs.CodeServerError, "验证码发送失败")
	}
	rdb.Del(ctx, otpFailKey(id, purpose))
	rdb.Set(ctx, otpCooldownKey(id), "1", time.Duration(resend)*time.Second)

	if err := deliver(code); err != nil {
		rdb.Del(ctx, otpCodeKey(id, purpose), otpCooldownKey(id))
		return errs.New(errs.CodeServerError, "验证码发送失败")
	}
	return nil
}

// genOTPCode 6 位数字，用 crypto/rand —— 验证码是登录凭据，不能用可预测的伪随机。
func genOTPCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// sendSMS 发短信。
//
// provider 为空时只打日志：开发期不真发，避免烧钱。
// ⚠️ 印度市场发短信前必须先在 TRAI DLT 平台注册实体与模板，那是开户前置，不是代码工作。
func (s *Service) sendSMS(tenantID int64, to, code string) error {
	provider := sysconfig.GetString(tenantID, sysconfig.KeyAppSMSProvider)
	if provider == "" {
		log.Printf("[otp] dev-mode to=%s code=%s（未配短信服务商，未真实发送）", to, code)
		return nil
	}
	// TODO(real): 按 provider 分支调服务商 API。
	// Key 取 sysconfig.KeyAppSMSKey，密钥取 KeyAppSMSSecretEnc 后 crypto.Decrypt，
	// 模板取 KeyAppSMSTemplate、签名取 KeyAppSMSSign；
	// 出站请求走 pkg/apilog 落库，便于在后台「接口日志」页排查「到底发没发」。
	log.Printf("[otp] provider=%s 尚未接入，验证码未真实发送 to=%s", provider, to)
	return nil
}

// sendEmail 发验证码邮件（SMTP）。
//
// SMTP 没配齐时**只打日志不报错**：开发期没人想为了点一下注册按钮先去配邮箱。
// ⚠️ 上线前应改成返回错误——一个「发不出邮件但假装发了」的注册流程，
// 会让用户对着收件箱等到放弃，而日志里什么都没有。
//
// ⚠️ 另一个开户前置：发信域名要配 SPF / DKIM / DMARC，否则验证码直接进垃圾箱。
// 那是域名配置工作，不是代码工作。
func (s *Service) sendEmail(tenantID int64, to, code, purpose string) error {
	ttl := sysconfig.GetInt(tenantID, sysconfig.KeyAppOTPTTL)
	if ttl <= 0 {
		ttl = 300
	}
	subject, _, _ := mailer.CodeMail(purpose, code, ttl/60)

	if !mailer.Configured(tenantID) {
		log.Printf("[otp] dev-mode email to=%s code=%s purpose=%s（SMTP 未配齐，未真实发送）", to, code, purpose)
		// 一样落库。后台「邮件记录」页要回答的正是「为什么我没收到信」——
		// 没有这条记录，未配置与发失败在后台看起来一模一样（都是没有）。
		s.logEmail(tenantID, to, purpose, code, subject, "skipped", "SMTP 未配齐")
		return nil
	}

	subject, body, err := mailer.CodeMail(purpose, code, ttl/60)
	if err != nil {
		log.Printf("[otp] 渲染邮件失败 to=%s: %v", to, err)
		s.logEmail(tenantID, to, purpose, code, subject, "failed", err.Error())
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := mailer.Send(ctx, tenantID, to, subject, body); err != nil {
		// 带上错误原文:SMTP 的失败原因(认证/连接/被拒)差别很大,
		// 吞掉它会让排查只能靠猜。
		log.Printf("[otp] 邮件发送失败 to=%s purpose=%s: %v", to, purpose, err)
		s.logEmail(tenantID, to, purpose, code, subject, "failed", err.Error())
		return err
	}
	log.Printf("[otp] 邮件已发送 to=%s purpose=%s", to, purpose)
	s.logEmail(tenantID, to, purpose, code, subject, "sent", "")
	return nil
}

// logEmail 异步记一条邮件发送记录。
//
// 异步 + 静默失败:日志功能不该拖慢发信,更不该因为写库失败把注册流程带崩。
// 与 pkg/apilog 的取舍一致。
func (s *Service) logEmail(tenantID int64, to, purpose, code, subject, status, errMsg string) {
	if s.db == nil {
		return
	}
	go func() {
		_ = s.db.Create(&model.EmailLog{
			TenantID: tenantID, To: to, Purpose: purpose, Code: code,
			Subject: truncRunes(subject, 128), Status: status,
			Error: truncRunes(errMsg, 500), CreatedAt: time.Now(),
		}).Error
	}()
}

// truncRunes 按字符截断,防爆列(SMTP 的错误原文可能很长)。
func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// VerifyOTP 校验手机验证码并登录/注册。
//
// 返回结构与现有 /api/auth/login 对齐（user + is_new），下游鉴权中间件零改动。
func (s *Service) VerifyOTP(appid, dialCode, phone, code string) (*model.User, bool, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, false, err
	}
	num := e164(dialCode, phone)
	if err := s.consumeOTP(tenantID, num, code, PurposeLogin); err != nil {
		return nil, false, err
	}
	return s.loginOrCreateApp(tenantID, appIdentity{Phone: num})
}

// VerifyEmailOTP 校验邮箱验证码并登录/注册。
func (s *Service) VerifyEmailOTP(appid, email, code string) (*model.User, bool, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, false, err
	}
	addr := normalizeEmail(email)
	if !emailRe.MatchString(addr) {
		return nil, false, errs.New(errs.CodeBadRequest, "邮箱格式不正确")
	}
	if err := s.consumeOTP(tenantID, addr, code, PurposeLogin); err != nil {
		return nil, false, err
	}
	return s.loginOrCreateApp(tenantID, appIdentity{Email: addr})
}

// consumeOTP 校验并消费验证码，通过后码立即作废（一码一次）。
//
// purpose 必须与发码时一致——这是「注册的码不能拿去重置密码」的落点。
func (s *Service) consumeOTP(tenantID int64, id, code, purpose string) error {
	// 开发态万能码：仅当后台显式配置了非空值才生效，生产必须留空。
	devCode := sysconfig.GetString(tenantID, sysconfig.KeyAppOTPDevCode)
	if devCode != "" && code == devCode {
		return nil
	}

	ctx := context.Background()
	rdb := cache.RDB

	want, err := rdb.Get(ctx, otpCodeKey(id, purpose)).Result()
	if err != nil || want == "" {
		return errs.New(errs.CodeLoginFailed, "验证码已过期，请重新获取")
	}
	if want != code {
		// 连续输错到上限就作废验证码，挡暴力枚举。
		if n, err := rdb.Incr(ctx, otpFailKey(id, purpose)).Result(); err == nil {
			if n == 1 {
				rdb.Expire(ctx, otpFailKey(id, purpose), 15*time.Minute)
			}
			if n >= otpMaxFail {
				rdb.Del(ctx, otpCodeKey(id, purpose))
			}
		}
		return errs.New(errs.CodeLoginFailed, "验证码不正确")
	}
	rdb.Del(ctx, otpCodeKey(id, purpose), otpFailKey(id, purpose))
	return nil
}

// appIdentity App 端的四种身份来源，四选一非空。
type appIdentity struct {
	Phone     string
	Email     string
	GoogleSub string
	AppleSub  string
	WxOpenID  string // 微信开放平台 openid(App 登录)
	AlipayUID string // 支付宝 user_id
	UnionID   string // 微信 unionid,建号时顺带存,不参与查找
	// Nickname / Avatar 第三方给的资料,仅建号时用;空则用默认昵称。
	Nickname string
	Avatar   string
}

func (id appIdentity) where() (string, string) {
	switch {
	// ⚠️ sub 必须排在 email 之前。
	//
	// 第三方登录会**同时**带上 sub 和 email(email 供建号与「该邮箱已注册」的
	// 冲突判断使用)。若按 email 查，就等于「按邮箱自动合并账号」——
	// 那正是产品明确否决的行为(见 oauthlink.go 的 decideOAuthLogin)。
	// sub 也确实是比邮箱更强的标识：它由签发方保证唯一且不可转移。
	case id.GoogleSub != "":
		return "google_sub = ?", id.GoogleSub
	case id.AppleSub != "":
		return "apple_sub = ?", id.AppleSub
	case id.WxOpenID != "":
		return "wx_openid = ?", id.WxOpenID
	case id.AlipayUID != "":
		return "alipay_uid = ?", id.AlipayUID
	case id.Phone != "":
		return "phone = ?", id.Phone
	default:
		return "email = ?", id.Email
	}
}

// loginOrCreateApp 按 (tenant_id, 身份) 找用户，没有就建。
//
// 唯一性在应用层查重，不加 DB 复合唯一索引——与现有 openid 的处理方式一致，
// 规避空值唯一冲突（大量小程序用户的 phone 字段为空）。
func (s *Service) loginOrCreateApp(tenantID int64, id appIdentity) (*model.User, bool, error) {
	cond, val := id.where()

	var u model.User
	err := s.db.Where("tenant_id = ?", tenantID).First(&u, cond, val).Error
	if err == nil {
		switch u.Status {
		case "banned":
			return nil, false, errs.New(errs.CodeForbidden, "账号已被封禁，如有疑问请联系客服")
		case "deleted":
			// 正常不会走到：删除时标识已释放。兜底防止脏数据把人放进来。
			return nil, false, errs.New(errs.CodeForbidden, "账号已注销")
		}
		now := time.Now()
		s.db.Model(&u).Updates(map[string]interface{}{"last_active_at": now, "last_login_at": now})
		return &u, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	nickname := fmt.Sprintf("用户%06d", idgen.Next()%1000000)
	if strings.TrimSpace(id.Nickname) != "" {
		nickname = truncateRunes(strings.TrimSpace(id.Nickname), 32)
	}
	u = model.User{
		UserID:         idgen.Next(),
		TenantID:       tenantID,
		Nickname:       nickname,
		Avatar:         id.Avatar,
		AnonymousLevel: 1,
		Status:         "active",
		Phone:          id.Phone,
		Email:          id.Email,
		// 空串转 NULL:这两列带唯一索引,存 '' 会让第二个未绑定的用户插不进去。
		GoogleSub:      nilIfEmpty(id.GoogleSub),
		AppleSub:       nilIfEmpty(id.AppleSub),
		// 这三列是历史 string 列(小程序登录在用),空就存空串。
		WxOpenID:       id.WxOpenID,
		AlipayUID:      id.AlipayUID,
		UnionID:        id.UnionID,
		CreatedAt:      time.Now(),
		LastActiveAt:   time.Now(),
		LastLoginAt:    time.Now(),
	}
	if err := s.db.Create(&u).Error; err != nil {
		return nil, false, err
	}
	if reward := sysconfig.GetInt64(tenantID, sysconfig.KeyRegReward); reward > 0 {
		_ = s.wallet.Credit(tenantID, u.UserID, reward, wallet.SceneReward, fmt.Sprintf("register:%d", u.UserID))
	}
	return &u, true, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
