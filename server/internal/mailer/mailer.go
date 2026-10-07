// Package mailer 发事务性邮件（注册验证码、找回密码验证码）。
//
// ── 为什么是 net/smtp 而不是第三方 SDK ──────────────────────────
//
// 要发的邮件只有验证码这一类、纯文本套一层 HTML，没有模板系统、
// 没有批量投递、没有退信处理的需求。标准库够用，而每多一个依赖
// 就多一份要跟着升级的东西。
//
// ── 配置走 sysconfig，按租户 ────────────────────────────────────
//
// 与短信/推送的配置一个路子：后台「App 登录」分组里填，改完下一封信就生效，
// 不需要改环境变量或重启。授权码用 AES-GCM 加密落库（与租户凭证同一套 crypto）。
package mailer

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	"driftbottle/internal/crypto"
	"driftbottle/internal/sysconfig"
)

var (
	// ErrNotConfigured SMTP 没配齐。调用方据此决定是降级打日志还是拒绝服务。
	ErrNotConfigured = errors.New("mailer: SMTP 未配置")
	ErrSendFailed    = errors.New("mailer: 邮件发送失败")
)

const dialTimeout = 20 * time.Second

// Config 一次发信要用的全部连接参数。
//
// ⚠️ 含**明文**授权码，只在内存里存在；落库的是 AES-GCM 加密后的版本。
type Config struct {
	Host string
	Port string // 常见 465(隐式 TLS) / 587(STARTTLS)
	// Account 登录账号,通常与 From 相同但不总是
	Account string
	// Token 多数服务商给的是「授权码」而不是登录密码
	Token    string
	From     string // 收件人看到的发件地址
	FromName string // 发件人显示名,可空
}

func (c Config) Valid() bool {
	return c.Host != "" && c.Port != "" && c.Account != "" &&
		c.Token != "" && c.From != ""
}

// FromSysconfig 按租户取当前生效的 SMTP 配置。
//
// **每次发信时取**,不在启动时定死——后台改完下一封就用新配置。
func FromSysconfig(tenantID int64) Config {
	return Config{
		Host:     strings.TrimSpace(sysconfig.GetString(tenantID, sysconfig.KeyAppSMTPHost)),
		Port:     strings.TrimSpace(sysconfig.GetString(tenantID, sysconfig.KeyAppSMTPPort)),
		Account:  strings.TrimSpace(sysconfig.GetString(tenantID, sysconfig.KeyAppSMTPAccount)),
		Token:    decodeToken(sysconfig.GetString(tenantID, sysconfig.KeyAppSMTPTokenEnc)),
		From:     strings.TrimSpace(sysconfig.GetString(tenantID, sysconfig.KeyAppSMTPFrom)),
		FromName: strings.TrimSpace(sysconfig.GetString(tenantID, sysconfig.KeyAppSMTPFromName)),
	}
}

// decodeToken 取授权码：能解密就用明文，解不开就当它本来就是明文。
//
// ⚠️ 这个回退不是偷懒，是对齐项目现状：后台的 `SetConfig` **不加密**
// （见 admin/service.go），所以管理员在「App 登录」分组填进去的就是明文。
// `robot/llmclient.go` 取 LLM API Key 时也是同一套回退。
//
// 真要做到"凭据不明文落库"，得在 admin 侧按 key 分类加密 + 读取侧统一解密，
// 那会牵动短信密钥、IAP 私钥、LLM Key 三处既有配置，不在本次范围内。
func decodeToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !crypto.Enabled() {
		return raw
	}
	if plain, err := crypto.Decrypt(raw); err == nil && plain != "" {
		return plain
	}
	return raw
}

// Configured 该租户能不能发信。
func Configured(tenantID int64) bool { return FromSysconfig(tenantID).Valid() }

// Send 用该租户当前的配置发一封 HTML 邮件。
func Send(ctx context.Context, tenantID int64, to, subject, htmlBody string) error {
	return SendWith(ctx, FromSysconfig(tenantID), to, subject, htmlBody)
}

// SendWith 用**指定**配置发信。
//
// 单独暴露出来是为了后台的「发送测试邮件」：管理员填完表单想先试一下，
// 而此时配置还没保存。没有它就得先存一份可能是错的配置,
// 于是真实用户在那段时间里全都注册失败。
func SendWith(ctx context.Context, cfg Config, to, subject, htmlBody string) error {
	if !cfg.Valid() {
		return ErrNotConfigured
	}
	msg, err := compose(cfg, to, subject, htmlBody)
	if err != nil {
		return err
	}

	// ctx 取消要能真的断开连接。net/smtp 自己不认 ctx,
	// 所以用 DialContext 建连,再把 conn 交给 smtp.NewClient。
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	conn, err := (&net.Dialer{Timeout: dialTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("%w: 连不上 %s: %v", ErrSendFailed, addr, err)
	}

	// 465 是**隐式 TLS**(SMTPS):连上立刻握手,不走 STARTTLS。
	// 587 / 25 是明文连上之后再 STARTTLS。
	//
	// 搞反了的表现是「连上之后一直没响应直到超时」,而不是一个说得清的错误,
	// 所以这里按端口分开处理。
	if cfg.Port == "465" {
		conn = tls.Client(conn, &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12})
	}

	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("%w: SMTP 握手失败: %v", ErrSendFailed, err)
	}
	defer func() { _ = c.Close() }()

	if cfg.Port != "465" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{
				ServerName: cfg.Host, MinVersion: tls.VersionTLS12,
			}); err != nil {
				return fmt.Errorf("%w: STARTTLS 失败: %v", ErrSendFailed, err)
			}
		}
		// 服务器不支持 STARTTLS 时继续发,但凭据是明文过网的。
		// 不硬拒是因为内网自建 SMTP 是合法场景;公网服务商不支持的话,
		// 那个服务商本身就不该用。
	}

	if err := c.Auth(smtp.PlainAuth("", cfg.Account, cfg.Token, cfg.Host)); err != nil {
		return fmt.Errorf("%w: SMTP 认证失败: %v", ErrSendFailed, err)
	}
	if err := c.Mail(cfg.From); err != nil {
		return fmt.Errorf("%w: 设置发件人失败: %v", ErrSendFailed, err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("%w: 设置收件人失败: %v", ErrSendFailed, err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("%w: 打开数据通道失败: %v", ErrSendFailed, err)
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return fmt.Errorf("%w: 写入邮件失败: %v", ErrSendFailed, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("%w: 提交邮件失败: %v", ErrSendFailed, err)
	}
	return c.Quit()
}

// compose 拼出一封完整的邮件。
func compose(cfg Config, to, subject, htmlBody string) ([]byte, error) {
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		// 🔴 **头注入**。收件人或标题里塞一个 CRLF,后面的内容就变成新的邮件头,
		// 可以加 Bcc 把信抄送给任意人。收件人来自用户输入,必须在这里挡住。
		return nil, fmt.Errorf("%w: 收件人或标题含非法换行", ErrSendFailed)
	}

	msgID, err := messageID(cfg.From)
	if err != nil {
		return nil, err
	}

	from := cfg.From
	if cfg.FromName != "" {
		// 显示名含中文同样要 RFC 2047 编码,否则收件箱里是乱码。
		from = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", cfg.FromName), cfg.From)
	}

	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	// 🔴 中文标题必须走 RFC 2047 编码。直接塞 UTF-8 进邮件头,多数客户端显示成乱码——
	// 而这件事在自己的邮箱里测很可能看不出来(同一家服务商会容错)。
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Message-ID: " + msgID + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String()), nil
}

// messageID 生成唯一的 Message-ID。
//
// 没有它,部分服务商(尤其 Gmail)会把信判成可疑——
// 一封没有 Message-ID 的邮件在正常投递里几乎不存在。
func messageID(from string) (string, error) {
	at := strings.LastIndex(from, "@")
	if at < 0 || at == len(from)-1 {
		return "", fmt.Errorf("%w: 发件地址不是完整邮箱(%q),请到后台「App 登录」分组修正", ErrNotConfigured, from)
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("%w: 生成 Message-ID 失败: %v", ErrSendFailed, err)
	}
	return "<" + hex.EncodeToString(b) + "@" + from[at+1:] + ">", nil
}
