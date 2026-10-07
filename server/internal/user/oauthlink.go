package user

import (
	"errors"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// 第三方身份(Google / Apple)的登录决策与绑定/解绑。
//
// 与 appauth.go 的分工：appauth.go 只负责「这个 ID Token 是真的吗」，
// 本文件负责「验过之后该怎么办」——登录、建号、还是拒绝并提示去绑定。

// nilIfEmpty 空串转 nil。
//
// google_sub / apple_sub 带唯一索引，未绑定时必须存 NULL 而不是 ''：
// MySQL 的唯一索引允许多行 NULL，但不允许多行 ''。存 '' 的话，
// 第二个没绑 Google 的用户就插不进去了。
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// oauthDecision 第三方登录的三种去向。
type oauthDecision int

const (
	oauthLogin      oauthDecision = iota // sub 已存在，直接登录
	oauthCreate                          // 全新身份，建号
	oauthEmailTaken                      // 邮箱已被其他账号占用，拒绝建号并提示去绑定
)

// decideOAuthLogin 决定第三方登录该走哪条路。
//
// 产品决策是「不自动按邮箱合并」：自动合并等于把「Google 说这个邮箱属于他」
// 当成账号所有权证明，这条链路上任何疏漏都会变成账号接管。所以宁可多一步，
// 让用户自己用密码登录后再去绑定。
//
// emailVerified 为 false 时邮箱完全不参与判断——Apple 的 token 根本没有这个
// 字段(解析为 false)，而未经验证的邮箱谁都能声称拥有。
func decideOAuthLogin(subHit, emailVerified, emailHit bool) oauthDecision {
	if subHit {
		return oauthLogin
	}
	if emailVerified && emailHit {
		return oauthEmailTaken
	}
	return oauthCreate
}

// loginMethods 一个账号当前拥有的可登录方式。
//
// Password 单列一项而不是并进 Phone/Email：验证码时代注册的老账号有手机号
// 但没有密码，两者不是一回事。
type loginMethods struct {
	Phone    bool
	Password bool
	Google   bool
	Apple    bool
	Wechat   bool
	Alipay   bool
}

// remainingAfterUnbind 解绑 provider 之后还剩几种可登录方式。
// 返回 0 表示不能解绑——用户会被锁在门外，且没有任何入口能改回来。
func (m loginMethods) remainingAfterUnbind(provider string) int {
	switch provider {
	case "google":
		m.Google = false
	case "apple":
		m.Apple = false
	case "wechat":
		m.Wechat = false
	case "alipay":
		m.Alipay = false
	}
	n := 0
	for _, has := range []bool{m.Phone, m.Password, m.Google, m.Apple, m.Wechat, m.Alipay} {
		if has {
			n++
		}
	}
	return n
}

// loginOrCreateOAuth 第三方登录专用：在「查不到就建号」之间插入冲突判断。
//
// id.Email 为 ID Token 里的邮箱，emailVerified 为其 email_verified claim，
// 两者共同决定是否允许建号，见 decideOAuthLogin。
// 允许建号时转交 loginOrCreateApp——它会把 id.Email 一并写进 User.Email，
// 这正好补上了「Google 注册的账号没有邮箱」那个缺口。
func (s *Service) loginOrCreateOAuth(tenantID int64, id appIdentity, emailVerified bool) (*model.User, bool, error) {
	cond, val := id.where()

	var u model.User
	err := s.db.Where("tenant_id = ?", tenantID).First(&u, cond, val).Error
	subHit := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	emailHit := false
	if !subHit && id.Email != "" && emailVerified {
		emailHit = s.identityExists(tenantID, appIdentity{Email: id.Email})
	}

	if decideOAuthLogin(subHit, emailVerified, emailHit) == oauthEmailTaken {
		return nil, false, errs.New(errs.CodeOAuthEmailTaken,
			"该邮箱已注册，请用密码登录后在「账号与安全」里绑定")
	}
	// oauthLogin / oauthCreate 都交给既有逻辑：它本身就是「查得到就登录，查不到就建号」。
	return s.loginOrCreateApp(tenantID, id)
}

var oauthColumns = map[string]string{
	"google": "google_sub", "apple": "apple_sub", "wechat": "wx_openid", "alipay": "alipay_uid",
}

func isOAuthProvider(p string) bool { _, ok := oauthColumns[p]; return ok }

// subColumn provider 对应的列名。调用方保证 provider 已校验。
func subColumn(provider string) string { return oauthColumns[provider] }

// nullableSubColumn google_sub / apple_sub 是 *string 带唯一索引(空存 NULL);
// wx_openid / alipay_uid 是历史 string 列(空存 '')。写值与清空要按列类型分叉。
func nullableSubColumn(provider string) bool { return provider == "google" || provider == "apple" }

func bindValue(provider, sub string) interface{} {
	if nullableSubColumn(provider) {
		return nilIfEmpty(sub)
	}
	return sub
}

func unbindValue(provider string) interface{} {
	if nullableSubColumn(provider) {
		return nil
	}
	return ""
}

// BindOAuth 把已验签的第三方身份绑到当前登录用户上。
//
// 与登录的唯一区别是最后一步：登录按 sub 查用户，绑定按 sub 查「有没有别人先绑了」，
// 没有才写到当前用户上。验签那一步两者完全共用。
func (s *Service) BindOAuth(tenantID, userID int64, provider, sub string) error {
	if !isOAuthProvider(provider) || sub == "" {
		return errs.New(errs.CodeBadRequest, "不支持的绑定类型")
	}
	col := subColumn(provider)

	var other model.User
	// string 列的空串要排除,否则「没绑过的人」会被当成「已绑了空串的人」。
	err := s.db.Where("tenant_id = ?", tenantID).First(&other, col+" = ? AND "+col+" <> ''", sub).Error
	if err == nil {
		if other.UserID == userID {
			return nil // 幂等：已绑到自己身上
		}
		// 不透露是哪个账号——用户自己的账号自己知道，
		// 攻击者不该从这里问出关联关系。
		return errs.New(errs.CodeOAuthAlreadyBound, "该账号已被其他用户绑定")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Model(&model.User{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update(col, bindValue(provider, sub)).Error
}

// UnbindOAuth 解绑。解绑后必须仍有可登录方式，否则用户会把自己锁在门外。
func (s *Service) UnbindOAuth(tenantID, userID int64, provider string) error {
	if !isOAuthProvider(provider) {
		return errs.New(errs.CodeBadRequest, "不支持的绑定类型")
	}
	var u model.User
	if err := s.db.Where("tenant_id = ?", tenantID).First(&u, "user_id = ?", userID).Error; err != nil {
		return errs.New(errs.CodeNotFound, "用户不存在")
	}
	m := loginMethods{
		Phone:    u.Phone != "",
		Password: u.PasswordHash != "",
		Google:   u.GoogleSub != nil,
		Apple:    u.AppleSub != nil,
		Wechat:   u.WxOpenID != "",
		Alipay:   u.AlipayUID != "",
	}
	if m.remainingAfterUnbind(provider) == 0 {
		return errs.New(errs.CodeOAuthLastMethod, "这是你唯一的登录方式，请先绑定手机号或设置密码")
	}
	return s.db.Model(&model.User{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update(subColumn(provider), unbindValue(provider)).Error
}
