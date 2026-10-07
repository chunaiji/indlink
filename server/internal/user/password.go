package user

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/idgen"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 密码长度下限。
//
// 只卡长度不卡复杂度：印度用户大量在手机小键盘上输入，强制大小写+符号会显著掉注册转化，
// 而 8 位起 + bcrypt + 登录频控已经能挡住绝大多数在线撞库。
const minPasswordLen = 8

// bcryptCost 10 ≈ 60ms/次。再高会让登录明显变慢,再低则降低离线爆破成本。
const bcryptCost = 10

func hashPassword(raw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(raw), bcryptCost)
	if err != nil {
		return "", errs.New(errs.CodeServerError, "密码处理失败")
	}
	return string(b), nil
}

func checkPasswordLen(raw string) error {
	// 按 rune 计数:一个中文字符也算一位,不能让 utf8 字节数把人误伤。
	if len([]rune(raw)) < minPasswordLen {
		return errs.New(errs.CodeBadRequest, fmt.Sprintf("密码至少 %d 位", minPasswordLen))
	}
	if len(raw) > 72 {
		// bcrypt 硬上限 72 字节,超出部分会被静默截断——与其悄悄截断不如直接拒。
		return errs.New(errs.CodeBadRequest, "密码过长")
	}
	return nil
}

// identityOf 把请求里的三个字段收敛成一个身份。邮箱优先，其次手机号。
func identityOf(dialCode, phone, email string) (appIdentity, error) {
	if addr := normalizeEmail(email); addr != "" {
		if !emailRe.MatchString(addr) {
			return appIdentity{}, errs.New(errs.CodeBadRequest, "邮箱格式不正确")
		}
		return appIdentity{Email: addr}, nil
	}
	if p := strings.TrimSpace(phone); p != "" {
		if !phoneRe.MatchString(p) {
			return appIdentity{}, errs.New(errs.CodeBadRequest, "手机号格式不正确")
		}
		return appIdentity{Phone: e164(dialCode, p)}, nil
	}
	return appIdentity{}, errs.New(errs.CodeBadRequest, "手机号与邮箱至少填一个")
}

// idValue 取身份的字面值，用作验证码的 Redis 键。
func idValue(id appIdentity) string {
	if id.Email != "" {
		return id.Email
	}
	return id.Phone
}

// Register 注册：验证码 + 密码一次性完成建号。
//
// 与 VerifyOTP（手机号免密登录）的区别是它**只建号不登录已有账号**——
// 标识已存在直接拒，避免「注册」变成一个能改别人密码的入口。
func (s *Service) Register(appid, dialCode, phone, email, code, password string) (*model.User, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, err
	}
	id, err := identityOf(dialCode, phone, email)
	if err != nil {
		return nil, err
	}
	if err := checkPasswordLen(password); err != nil {
		return nil, err
	}
	// 再查一次:发码到提交之间可能已经被别人注册走(竞态)。
	if s.identityExists(tenantID, id) {
		return nil, errs.New(errs.CodeBadRequest, "该账号已注册，请直接登录")
	}
	if err := s.consumeOTP(tenantID, idValue(id), code, PurposeRegister); err != nil {
		return nil, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	u := model.User{
		UserID:         idgen.Next(),
		TenantID:       tenantID,
		Nickname:       fmt.Sprintf("用户%06d", idgen.Next()%1000000),
		AnonymousLevel: 1,
		Status:         "active",
		Phone:          id.Phone,
		Email:          id.Email,
		PasswordHash:   hash,
		CreatedAt:      now,
		LastActiveAt:   now,
		LastLoginAt:    now,
	}
	if err := s.db.Create(&u).Error; err != nil {
		return nil, err
	}
	if reward := sysconfig.GetInt64(tenantID, sysconfig.KeyRegReward); reward > 0 {
		_ = s.wallet.Credit(tenantID, u.UserID, reward, wallet.SceneReward, fmt.Sprintf("register:%d", u.UserID))
	}
	return &u, nil
}

// LoginWithPassword 密码登录。
//
// ⚠️ 账号不存在与密码错误**返回同一句话**：分开说等于给撞库送一个用户名枚举接口。
func (s *Service) LoginWithPassword(appid, dialCode, phone, email, password string) (*model.User, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, err
	}
	id, err := identityOf(dialCode, phone, email)
	if err != nil {
		return nil, err
	}
	if password == "" {
		return nil, errs.New(errs.CodeLoginFailed, "账号或密码不正确")
	}

	cond, val := id.where()
	var u model.User
	err = s.db.Where("tenant_id = ?", tenantID).First(&u, cond, val).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeLoginFailed, "账号或密码不正确")
		}
		return nil, err
	}
	switch u.Status {
	case "banned":
		return nil, errs.New(errs.CodeForbidden, "账号已被封禁，如有疑问请联系客服")
	case "deleted":
		return nil, errs.New(errs.CodeLoginFailed, "账号或密码不正确")
	}
	if u.PasswordHash == "" {
		// 验证码时代注册的老账号,还没设过密码。明确引导,别让人反复试密码。
		return nil, errs.New(errs.CodeLoginFailed, "该账号还未设置密码，请用「忘记密码」设置一次")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, errs.New(errs.CodeLoginFailed, "账号或密码不正确")
	}

	now := time.Now()
	s.db.Model(&u).Updates(map[string]interface{}{"last_active_at": now, "last_login_at": now})
	return &u, nil
}

// ResetPassword 忘记密码：验证码校验通过后改密。
//
// 也是老账号（验证码时代注册、没有密码）补设密码的唯一入口。
func (s *Service) ResetPassword(appid, dialCode, phone, email, code, password string) (*model.User, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, err
	}
	id, err := identityOf(dialCode, phone, email)
	if err != nil {
		return nil, err
	}
	if err := checkPasswordLen(password); err != nil {
		return nil, err
	}

	cond, val := id.where()
	var u model.User
	if err := s.db.Where("tenant_id = ?", tenantID).First(&u, cond, val).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "该账号尚未注册")
		}
		return nil, err
	}
	if u.Status == "banned" {
		return nil, errs.New(errs.CodeForbidden, "账号已被封禁，如有疑问请联系客服")
	}
	if u.Status == "deleted" {
		return nil, errs.New(errs.CodeNotFound, "该账号尚未注册")
	}
	if err := s.consumeOTP(tenantID, idValue(id), code, PurposeReset); err != nil {
		return nil, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := s.db.Model(&u).Updates(map[string]interface{}{
		"password_hash":  hash,
		"last_active_at": now,
		"last_login_at":  now,
	}).Error; err != nil {
		return nil, err
	}
	u.PasswordHash = hash
	return &u, nil
}
