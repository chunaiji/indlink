package user

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/provider"
	"driftbottle/internal/tenant"

	"github.com/golang-jwt/jwt/v5"
)

// Google / Apple 第三方登录。
//
// 两家签发的都是标准 RS256 JWT，校验方式一致：按 kid 从各自 JWKS 取公钥验签，
// 再核对 iss / aud / exp。所以这里只写一套验证器，两边共用。
//
// ⚠️ 合规联动：一旦提供 Google 登录，iOS 端**必须**同时提供 Apple 登录
// （App Store 审核指南 4.8），否则直接拒审。

const (
	googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"
	appleJWKSURL  = "https://appleid.apple.com/auth/keys"

	appleIssuer = "https://appleid.apple.com"
)

// googleIssuers Google 两种写法都合法，历史原因。
var googleIssuers = map[string]bool{
	"https://accounts.google.com": true,
	"accounts.google.com":         true,
}

var (
	googleJWKS = &jwksCache{url: googleJWKSURL}
	appleJWKS  = &jwksCache{url: appleJWKSURL}
)

// jwksHTTPClient 专拉 Google / Apple 的 JWKS 公钥。
//
// 国内服务器直连 googleapis.com / appleid.apple.com 会超时(验签拿不到公钥,
// 表现为「登录校验失败」)。配了 JWKS_PROXY(如首尔中转的 http 代理)就走它出海;
// 没配则直连(海外部署 / 本地开发)。⚠️ 只有 JWKS 走代理:微信 / 腾讯等国内外呼
// 仍用 oauth.go 的 httpClient 直连,不受影响。
var jwksHTTPClient = &http.Client{
	Timeout: 8 * time.Second,
	Transport: &http.Transport{
		// Proxy 放回调里按需读:本 var 在 main() 之前初始化,而 JWKS_PROXY 由
		// main() 里的 godotenv.Load() 加载——初始化那刻读会拿到空。回调在每次
		// 请求时执行,那时 .env 已加载,能读到最新值。
		Proxy: func(*http.Request) (*url.URL, error) {
			p := os.Getenv("JWKS_PROXY")
			if p == "" {
				return nil, nil
			}
			return url.Parse(p)
		},
	},
}

// jwksCache 公钥缓存。两家都会轮换密钥，所以带 TTL；
// 遇到未知 kid 时强制刷新一次再判失败（轮换瞬间不至于全员登录失败）。
type jwksCache struct {
	url string

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

const jwksTTL = 6 * time.Hour

func (c *jwksCache) publicKey(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	k, ok := c.keys[kid]
	fresh := time.Since(c.fetchedAt) < jwksTTL
	c.mu.RUnlock()
	if ok && fresh {
		return k, nil
	}
	if err := c.refresh(); err != nil {
		// 刷新失败但手上还有旧键，先用旧的，别让登录整体挂掉。
		if ok {
			return k, nil
		}
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if k, ok := c.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("未知的签名密钥 kid=%s", kid)
}

func (c *jwksCache) refresh() error {
	resp, err := jwksHTTPClient.Get(c.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("拉取 JWKS 失败: %d", resp.StatusCode)
	}
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nb, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eb, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nb),
			E: int(new(big.Int).SetBytes(eb).Int64()),
		}
	}
	if len(keys) == 0 {
		return fmt.Errorf("JWKS 为空")
	}
	c.mu.Lock()
	c.keys, c.fetchedAt = keys, time.Now()
	c.mu.Unlock()
	return nil
}

// idTokenClaims 只取我们用得到的字段。sub 是稳定的用户唯一标识。
type idTokenClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	// EmailVerified Google 会给；Apple 不给(其 token 无此字段,解析为 false)。
	// 只有它为 true 时,邮箱才可用于「该邮箱已注册」的冲突判断——
	// 拿未经验证的邮箱去匹配已有账号,等于谁都能声称自己拥有那个邮箱。
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// splitClientIDs 把后台那一格拆成多个 client ID。
//
// 同一个 Google 项目下,Android 与 iOS 拿到的 aud 本来就不是同一个值:
// Android 配了 serverClientId 之后 aud = Web client ID,iOS 走 Info.plist
// 那条路 aud = iOS client ID。只认单个值,两个平台必然有一个登录不了。
//
// 单值配置照常工作,所以这是向后兼容的加法。
func splitClientIDs(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// primaryClientID 取第一个 —— 约定它是 **Web** client ID。
//
// /app-config 把它下发给 Android 当 serverClientId(没有它 Android 拿不到
// ID token,登录请求根本发不出去)。所以顺序有语义,别排序也别去重打乱。
func primaryClientID(raw string) string {
	ids := splitClientIDs(raw)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// verifyIDTokenAny 逐个 audience 试,任一通过即可。
//
// 逐个重解析而不是解出来再比对 aud:jwt 库的 audience 校验是它自己那套
// (含 RFC 规定的多值 aud 处理),自己比对等于把这段规则重写一遍。
// audiences 最多两三个,重解析的代价可以忽略。
func verifyIDTokenAny(cache *jwksCache, token string, audiences []string) (*idTokenClaims, error) {
	var lastErr error
	for _, aud := range audiences {
		claims, err := verifyIDToken(cache, token, aud)
		if err == nil {
			return claims, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("未配置任何 client ID")
	}
	return nil, lastErr
}

// verifyIDToken 验签并校验 aud / exp，返回 claims。iss 由调用方按各自规则核对。
func verifyIDToken(cache *jwksCache, token, audience string) (*idTokenClaims, error) {
	var claims idTokenClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("缺少 kid")
		}
		return cache.publicKey(kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("缺少 sub")
	}
	return &claims, nil
}

// verifyOAuthSub 验签并返回 sub / email / emailVerified / 租户。
//
// 登录与绑定共用：两者的验签规则完全一致，区别只在验完之后做什么——
// 登录按 sub 查用户，绑定把 sub 写到当前登录用户上。
func (s *Service) verifyOAuthSub(appid, provider, credential string) (sub, email string, emailVerified bool, tenantID int64, err error) {
	tenantID, err = s.resolveAppTenant(appid)
	if err != nil {
		return "", "", false, 0, err
	}
	switch provider {
	case "google":
		clientIDs := splitClientIDs(s.ssoField(tenantID, "google", "client_id"))
		if len(clientIDs) == 0 {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "未配置 Google Client ID")
		}
		claims, e := verifyIDTokenAny(googleJWKS, credential, clientIDs)
		if e != nil {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "Google 登录校验失败")
		}
		if !googleIssuers[claims.Issuer] {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "Google 令牌签发方不正确")
		}
		return claims.Subject, normalizeEmail(claims.Email), claims.EmailVerified, tenantID, nil
	case "apple":
		bundleIDs := splitClientIDs(s.ssoField(tenantID, "apple", "bundle_id"))
		if len(bundleIDs) == 0 {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "未配置 Apple Bundle ID")
		}
		claims, e := verifyIDTokenAny(appleJWKS, credential, bundleIDs)
		if e != nil {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "Apple 登录校验失败")
		}
		if claims.Issuer != appleIssuer {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "Apple 令牌签发方不正确")
		}
		// Apple 不给 email_verified，恒 false。
		return claims.Subject, normalizeEmail(claims.Email), false, tenantID, nil
	case "wechat":
		res, e := s.wechatIdentity(tenantID, credential)
		if e != nil {
			return "", "", false, 0, e
		}
		return res.OpenID, "", false, tenantID, nil
	case "alipay":
		res, e := s.alipayIdentity(tenantID, credential)
		if e != nil {
			return "", "", false, 0, e
		}
		return res.OpenID, "", false, tenantID, nil
	}
	return "", "", false, 0, errs.New(errs.CodeBadRequest, "不支持的登录方式")
}

// LoginWithGoogle 校验 Google ID Token 并登录/注册。
func (s *Service) LoginWithGoogle(appid, idToken string) (*model.User, bool, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, false, err
	}
	clientIDs := splitClientIDs(s.ssoField(tenantID, "google", "client_id"))
	if len(clientIDs) == 0 {
		return nil, false, errs.New(errs.CodeLoginFailed, "未配置 Google Client ID")
	}
	claims, err := verifyIDTokenAny(googleJWKS, idToken, clientIDs)
	if err != nil {
		return nil, false, errs.New(errs.CodeLoginFailed, "Google 登录校验失败")
	}
	if !googleIssuers[claims.Issuer] {
		return nil, false, errs.New(errs.CodeLoginFailed, "Google 令牌签发方不正确")
	}
	// 带上 email：不带的话 Google 注册出来的账号会没有邮箱，
	// 既做不了「该邮箱已注册」的冲突判断，用户日后也无从找回账号。
	return s.loginOrCreateOAuth(tenantID, appIdentity{
		GoogleSub: claims.Subject,
		Email:     normalizeEmail(claims.Email),
	}, claims.EmailVerified)
}

// LoginWithApple 校验 Apple identityToken 并登录/注册。
//
// 注意 Apple 只在**首次授权**时回传昵称与邮箱，且不在 identityToken 里——
// 客户端要把它们随首次登录一并提交，错过就再也拿不到。
func (s *Service) LoginWithApple(appid, identityToken string) (*model.User, bool, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, false, err
	}
	bundleIDs := splitClientIDs(s.ssoField(tenantID, "apple", "bundle_id"))
	if len(bundleIDs) == 0 {
		return nil, false, errs.New(errs.CodeLoginFailed, "未配置 Apple Bundle ID")
	}
	claims, err := verifyIDTokenAny(appleJWKS, identityToken, bundleIDs)
	if err != nil {
		return nil, false, errs.New(errs.CodeLoginFailed, "Apple 登录校验失败")
	}
	if claims.Issuer != appleIssuer {
		return nil, false, errs.New(errs.CodeLoginFailed, "Apple 令牌签发方不正确")
	}
	// emailVerified 恒传 false：Apple 的 identityToken 没有 email_verified，
	// 且用户可能选了「隐藏我的邮箱」，拿到的是 @privaterelay.appleid.com 中继地址——
	// 能收信，但不是用户的真实邮箱，不可用于冲突判断。
	return s.loginOrCreateOAuth(tenantID, appIdentity{
		AppleSub: claims.Subject,
		Email:    normalizeEmail(claims.Email),
	}, false)
}

// ---------------- 微信 / 支付宝(国内版 App) ----------------

// appCreds 取 App 第三方登录凭证(wx_app / alipay_app)。
func (s *Service) appCreds(tenantID int64, platform, human string) (*tenant.Resolved, error) {
	if s.creds != nil {
		if r, ok := s.creds.ByTenantPlatform(tenantID, platform); ok {
			return r, nil
		}
	}
	return nil, errs.New(errs.CodeLoginFailed, "未配置"+human+"登录")
}

// ssoField 取「服务商 → 登录」卡片的某个字段。没接 store、没建卡片、没填值
// 都返回空串——调用方据此报「未配置」,不要在这里 panic。
func (s *Service) ssoField(tenantID int64, prov, key string) string {
	if s.providers == nil {
		return ""
	}
	row, ok := s.providers.Get(tenantID, provider.KindSSO, prov)
	if !ok {
		return ""
	}
	return row.Get(key)
}

// wechatIdentity code → 身份。凭证来自「服务商 → 登录 → 微信登录」卡片。
func (s *Service) wechatIdentity(tenantID int64, code string) (*oauthResult, error) {
	appID := s.ssoField(tenantID, "wechat", "app_id")
	secret := s.ssoField(tenantID, "wechat", "app_secret")
	if appID == "" || secret == "" {
		return nil, errs.New(errs.CodeLoginFailed, "未配置微信登录")
	}
	res, err := wxAppCode2Token(tenantID, appID, secret, code)
	if err != nil {
		return nil, errs.New(errs.CodeLoginFailed, err.Error())
	}
	return res, nil
}

// alipayIdentity auth_code → 身份。密钥来自服务商页「支付宝」卡片(回落旧凭证行)。
func (s *Service) alipayIdentity(tenantID int64, authCode string) (*oauthResult, error) {
	cli, err := s.alipayClient(tenantID, "alipay_app")
	if err != nil {
		return nil, errs.New(errs.CodeLoginFailed, "支付宝凭证无效:"+err.Error())
	}
	if cli == nil {
		return nil, errs.New(errs.CodeLoginFailed, "未配置支付宝登录")
	}
	res, err := alipayCode2UID(tenantID, cli, authCode)
	if err != nil {
		return nil, errs.New(errs.CodeLoginFailed, err.Error())
	}
	return res, nil
}

// LoginWithWechat 微信开放平台登录 / 建号。
func (s *Service) LoginWithWechat(appid, code string) (*model.User, bool, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, false, err
	}
	res, err := s.wechatIdentity(tenantID, code)
	if err != nil {
		return nil, false, err
	}
	return s.loginOrCreateOAuth(tenantID, appIdentity{
		WxOpenID: res.OpenID, UnionID: res.UnionID, Nickname: res.Nickname, Avatar: res.Avatar,
	}, false)
}

// LoginWithAlipay 支付宝登录 / 建号。
func (s *Service) LoginWithAlipay(appid, authCode string) (*model.User, bool, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, false, err
	}
	res, err := s.alipayIdentity(tenantID, authCode)
	if err != nil {
		return nil, false, err
	}
	return s.loginOrCreateOAuth(tenantID, appIdentity{AlipayUID: res.OpenID}, false)
}

// AlipayAuthInfo 给客户端 SDK 的授权串。
func (s *Service) AlipayAuthInfo(appid string) (string, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return "", err
	}
	cli, err := s.alipayClient(tenantID, "alipay_app")
	if err != nil || cli == nil {
		return "", errs.New(errs.CodeLoginFailed, "未配置支付宝登录")
	}
	// PID 来自「服务商 → 登录 → 支付宝登录」卡片;密钥仍在支付卡片上(alipayClient 取)。
	var row *provider.Resolved
	if s.providers != nil {
		row, _ = s.providers.Get(tenantID, provider.KindSSO, "alipay")
	}
	return alipayAuthInfo(cli, alipayPID(row, ""))
}
