// Package alipay 支付宝开放平台最小客户端:RSA2 签名 / 验签、网关调用、金额换算。
//
// 纯标准库,不引 SDK:我们只用 4 个接口(trade.app.pay / trade.create / trade.query /
// system.oauth.token),官方 SDK 几 MB 的依赖换不来什么。pay 与 user 两个包共用。
package alipay

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	GatewayProd    = "https://openapi.alipay.com/gateway.do"
	GatewaySandbox = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
)

// HTTPClient 可被测试替换。
var HTTPClient = &http.Client{Timeout: 10 * time.Second}

var cst = time.FixedZone("CST", 8*3600)

type Client struct {
	AppID   string
	Gateway string
	priv    *rsa.PrivateKey
	pub     *rsa.PublicKey // 支付宝公钥;空 = 只签不验(开发态)
}

// New 构造客户端。alipayPublicKey 可空。
func New(appID, privateKey, alipayPublicKey string) (*Client, error) {
	priv, err := ParsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("解析支付宝应用私钥失败: %w", err)
	}
	c := &Client{AppID: appID, Gateway: GatewayFor(appID), priv: priv}
	if strings.TrimSpace(alipayPublicKey) != "" {
		pub, err := ParsePublicKey(alipayPublicKey)
		if err != nil {
			return nil, fmt.Errorf("解析支付宝公钥失败: %w", err)
		}
		c.pub = pub
	}
	return c, nil
}

// GatewayFor 沙箱应用的 appid 以 9021 开头,走沙箱网关。
func GatewayFor(appID string) string {
	if strings.HasPrefix(appID, "9021") {
		return GatewaySandbox
	}
	return GatewayProd
}

var wsRe = regexp.MustCompile(`\s+`)

// normalizeKey 后台贴的密钥常是裸 base64(支付宝密钥工具默认输出);补上 PEM 头再解析。
func normalizeKey(s, header string) []byte {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "-----BEGIN") {
		return []byte(s)
	}
	s = wsRe.ReplaceAllString(s, "")
	var b strings.Builder
	b.WriteString("-----BEGIN " + header + "-----\n")
	for len(s) > 64 {
		b.WriteString(s[:64] + "\n")
		s = s[64:]
	}
	b.WriteString(s + "\n-----END " + header + "-----\n")
	return []byte(b.String())
}

// ParsePrivateKey 支持 PKCS#8 / PKCS#1,带或不带 PEM 头。
func ParsePrivateKey(s string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(normalizeKey(s, "PRIVATE KEY"))
	if block == nil {
		return nil, errors.New("无效的私钥")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("私钥不是 RSA 类型")
		}
		return rk, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// ParsePublicKey PKIX 公钥,带或不带 PEM 头。
func ParsePublicKey(s string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(normalizeKey(s, "PUBLIC KEY"))
	if block == nil {
		return nil, errors.New("无效的公钥")
	}
	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("公钥不是 RSA 类型")
	}
	return rk, nil
}

// SignContent 待签名串:按 key 升序 `k=v&k=v`,值**不**URL 编码,空值跳过,`sign` 永远剔除,
// exclude 里的键一并剔除(通知验签要剔 sign_type;请求签名不剔)。
func SignContent(params map[string]string, exclude ...string) string {
	skip := map[string]bool{"sign": true}
	for _, k := range exclude {
		skip[k] = true
	}
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if skip[k] || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	return b.String()
}

// Encode 把参数编成请求体 / orderStr:按 key 升序,值 URL 编码。
func Encode(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(params[k]))
	}
	return b.String()
}

func (c *Client) signContent(content string) (string, error) {
	h := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.priv, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Sign 对参数做 RSA2 签名(请求用:sign_type 参与)。
func (c *Client) Sign(params map[string]string) (string, error) {
	return c.signContent(SignContent(params))
}

func (c *Client) verify(content, signB64 string) error {
	if c.pub == nil {
		return errors.New("未配置支付宝公钥,无法验签")
	}
	sig, err := base64.StdEncoding.DecodeString(signB64)
	if err != nil {
		return fmt.Errorf("签名不是合法 base64: %w", err)
	}
	h := sha256.Sum256([]byte(content))
	return rsa.VerifyPKCS1v15(c.pub, crypto.SHA256, h[:], sig)
}

// Verify 异步通知验签:剔 sign 与 sign_type。
func (c *Client) Verify(params map[string]string) error {
	return c.verify(SignContent(params, "sign_type"), params["sign"])
}

// SignRaw / VerifyRaw 对任意字符串签名 / 验签(网关响应节点、其它包的测试)。
func (c *Client) SignRaw(content string) (string, error)  { return c.signContent(content) }
func (c *Client) VerifyRaw(content, signB64 string) error { return c.verify(content, signB64) }

// BuildRequest 组装 method 的完整签名参数(含 sign)。bizContent 为 nil 时不带 biz_content
// (system.oauth.token 的业务参数在公共层,而不是 biz_content 里)。
func (c *Client) BuildRequest(method string, bizContent any, extra map[string]string) (map[string]string, error) {
	p := map[string]string{
		"app_id": c.AppID, "method": method, "format": "JSON", "charset": "utf-8",
		"sign_type": "RSA2", "timestamp": time.Now().In(cst).Format("2006-01-02 15:04:05"), "version": "1.0",
	}
	for k, v := range extra {
		if v != "" {
			p[k] = v
		}
	}
	if bizContent != nil {
		b, err := json.Marshal(bizContent)
		if err != nil {
			return nil, err
		}
		p["biz_content"] = string(b)
	}
	sign, err := c.Sign(p)
	if err != nil {
		return nil, err
	}
	p["sign"] = sign
	return p, nil
}

// Execute 调网关,返回 <method>_response 节点(已验签、已判业务码)。
func (c *Client) Execute(method string, bizContent any, extra map[string]string) (json.RawMessage, error) {
	params, err := c.BuildRequest(method, bizContent, extra)
	if err != nil {
		return nil, err
	}
	resp, err := HTTPClient.Post(c.Gateway, "application/x-www-form-urlencoded;charset=utf-8", strings.NewReader(Encode(params)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return c.ParseResponse(method, body)
}

// ParseResponse 解析网关 JSON:验签对象是响应节点的**原始子串**(json.RawMessage 原样保留)。
//
// 成功响应不一定有 code(system.oauth.token 就没有),所以「有 code 且不是 10000」才算失败。
func (c *Client) ParseResponse(method string, body []byte) (json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, fmt.Errorf("支付宝响应不是 JSON: %w", err)
	}
	key := strings.ReplaceAll(method, ".", "_") + "_response"
	node, ok := top[key]
	if !ok {
		node, ok = top["error_response"]
		if !ok {
			return nil, fmt.Errorf("支付宝响应缺少 %s: %s", key, string(body))
		}
	}
	var sign string
	_ = json.Unmarshal(top["sign"], &sign)
	if c.pub != nil && sign != "" {
		if err := c.verify(string(node), sign); err != nil {
			return nil, fmt.Errorf("支付宝响应验签失败: %w", err)
		}
	} else if c.pub != nil {
		return nil, errors.New("支付宝响应缺少 sign")
	}
	var st struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		SubCode string `json:"sub_code"`
		SubMsg  string `json:"sub_msg"`
	}
	_ = json.Unmarshal(node, &st)
	if st.Code != "" && st.Code != "10000" {
		return nil, fmt.Errorf("支付宝 %s 失败: %s %s %s %s", method, st.Code, st.Msg, st.SubCode, st.SubMsg)
	}
	return node, nil
}

// FenToYuan 分 → "12.00"。
func FenToYuan(fen int64) string { return fmt.Sprintf("%d.%02d", fen/100, fen%100) }

var yuanRe = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)

// YuanToFen "12.00" / "12" / "0.1" → 分。格式不对一律报错——支付宝通知里的金额要拿去和订单比对,
// 解析失败当 0 会让金额校验形同虚设。
func YuanToFen(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if !yuanRe.MatchString(s) {
		return 0, fmt.Errorf("金额格式不正确: %q", s)
	}
	parts := strings.SplitN(s, ".", 2)
	whole, _ := strconv.ParseInt(parts[0], 10, 64)
	var frac int64
	if len(parts) == 2 {
		f := parts[1]
		if len(f) == 1 {
			f += "0"
		}
		frac, _ = strconv.ParseInt(f, 10, 64)
	}
	return whole*100 + frac, nil
}
