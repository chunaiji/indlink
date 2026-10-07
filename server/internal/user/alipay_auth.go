package user

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"driftbottle/internal/common/alipay"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/provider"
	"driftbottle/pkg/apilog"
)

// alipayClient 优先用服务商页「支付宝」卡片的密钥(登录与支付同一应用、同一把私钥);
// 没配过再回落 app_credentials 旧行(迁移期)。都没有返回 (nil, nil) 让调用方走开发态 / 报未配置。
func (s *Service) alipayClient(tenantID int64, platform string) (*alipay.Client, error) {
	if s.providers != nil {
		if row, ok := s.providers.Get(tenantID, provider.KindPay, "alipay"); ok && row.Get("private_key") != "" {
			cli, err := alipay.New(row.Get("app_id"), row.Get("private_key"), row.Get("alipay_public_key"))
			if err != nil {
				return nil, err
			}
			if row.Bool("sandbox") {
				cli.Gateway = alipay.GatewaySandbox
			}
			return cli, nil
		}
	}
	if s.creds == nil {
		return nil, nil
	}
	r, ok := s.creds.ByTenantPlatform(tenantID, platform)
	if !ok || r.AlipayPrivateKeyPEM == "" {
		return nil, nil
	}
	return alipay.New(r.AppID, r.AlipayPrivateKeyPEM, r.AlipayPublicKey)
}

// AlipayClient 导出给内容安全模块复用(支付宝内容检测用同一个应用)。
func (s *Service) AlipayClient(tenantID int64) (*alipay.Client, error) {
	return s.alipayClient(tenantID, "alipay_app")
}

// alipayPID 授权登录要的商户 PID,来自「服务商 → 登录 → 支付宝登录」卡片。
// 不再回落旧凭证行:迁移已经把值搬过来了,留回落只会让「新页面改了不生效」。
func alipayPID(row *provider.Resolved, _ string) string {
	if row == nil {
		return ""
	}
	return row.Get("pid")
}

// alipayAuthInfo 服务端签发「App 支付宝登录」的授权串(alipay.open.auth.sdk.code.get),客户端原样交给 SDK。
// 应用私钥不出服务端,所以必须在这里签。
func alipayAuthInfo(cli *alipay.Client, pid string) (string, error) {
	if pid == "" {
		return "", errs.New(errs.CodeLoginFailed, "未配置支付宝 PID")
	}
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	p := map[string]string{
		"apiname":    "com.alipay.account.auth",
		"app_id":     cli.AppID,
		"app_name":   "mc",
		"auth_type":  "AUTHACCOUNT",
		"biz_type":   "openservice",
		"method":     "alipay.open.auth.sdk.code.get",
		"pid":        pid,
		"product_id": "APP_FAST_LOGIN",
		"scope":      "kuaijie",
		"sign_type":  "RSA2",
		"target_id":  hex.EncodeToString(buf),
	}
	sign, err := cli.Sign(p)
	if err != nil {
		return "", err
	}
	p["sign"] = sign
	return alipay.Encode(p), nil
}

// parseAuthInfoParams 把 authInfo 还原成参数表(测试与排错用)。
func parseAuthInfoParams(authInfo string) map[string]string {
	out := map[string]string{}
	vals, err := url.ParseQuery(authInfo)
	if err != nil {
		return out
	}
	for k := range vals {
		out[k] = vals.Get(k)
	}
	return out
}

// alipayCode2UID auth_code → user_id(alipay.system.oauth.token)。业务参数在公共层,不走 biz_content。
func alipayCode2UID(tenantID int64, cli *alipay.Client, code string) (*oauthResult, error) {
	node, err := cli.Execute("alipay.system.oauth.token", nil, map[string]string{
		"grant_type": "authorization_code", "code": code,
	})
	const detail = "换 user_id"
	if err != nil {
		apilog.Record(tenantID, "alipay_oauth_token", detail, 0, err.Error(), false)
		return nil, fmt.Errorf("支付宝登录失败:%w", err)
	}
	var r struct {
		UserID string `json:"user_id"`
		OpenID string `json:"open_id"`
	}
	_ = json.Unmarshal(node, &r)
	apilog.Record(tenantID, "alipay_oauth_token", detail, 0, redactToken(string(node)), true)
	uid := r.UserID
	if uid == "" {
		uid = r.OpenID
	}
	if uid == "" {
		return nil, errors.New("支付宝未返回 user_id")
	}
	return &oauthResult{OpenID: uid}, nil
}

// redactToken 日志里抹掉 access_token / refresh_token 的值。
func redactToken(s string) string {
	for _, k := range []string{"access_token", "refresh_token"} {
		marker := `"` + k + `":"`
		i := strings.Index(s, marker)
		if i < 0 {
			continue
		}
		start := i + len(marker)
		j := strings.Index(s[start:], `"`)
		if j < 0 {
			continue
		}
		s = s[:start] + "***" + s[start+j:]
	}
	return s
}
