package user

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"driftbottle/pkg/apilog"
)

// oauthResult 登录换取的平台身份。
type oauthResult struct {
	OpenID   string
	UnionID  string
	Nickname string // 微信 App 登录 sns/userinfo 给的,建号初值;其它渠道为空
	Avatar   string
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

// wxCode2Session 用微信 appid/secret + 登录 code 换 openid/unionid。
// 缺 appid/secret(开发态)用 code 派生稳定 openid,便于本地/接口测试。
func wxCode2Session(appid, secret, code string) (*oauthResult, error) {
	if appid == "" || secret == "" {
		return &oauthResult{OpenID: "wxdev_" + code}, nil
	}
	u := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		url.QueryEscape(appid), url.QueryEscape(secret), url.QueryEscape(code))
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if r.ErrCode != 0 || r.OpenID == "" {
		return nil, fmt.Errorf("微信登录失败:%s", r.ErrMsg)
	}
	return &oauthResult{OpenID: r.OpenID, UnionID: r.UnionID}, nil
}

// wxAppCode2Token 微信开放平台(移动应用)登录:code → access_token/openid/unionid,再顺手取昵称头像。
// userinfo 失败不阻断登录——资料只是建号时的初值。
func wxAppCode2Token(tenantID int64, appid, secret, code string) (*oauthResult, error) {
	if appid == "" || secret == "" {
		return &oauthResult{OpenID: "wxappdev_" + code}, nil
	}
	u := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		url.QueryEscape(appid), url.QueryEscape(secret), url.QueryEscape(code))
	var tok struct {
		AccessToken string `json:"access_token"`
		OpenID      string `json:"openid"`
		UnionID     string `json:"unionid"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	body, err := wxGetJSON(u, &tok)
	// 日志里不能出现 secret / access_token:只记错误码与 openid,不记 URL 与原文。
	logBody := fmt.Sprintf("errcode=%d errmsg=%s openid=%s", tok.ErrCode, tok.ErrMsg, tok.OpenID)
	if err != nil {
		logBody = err.Error() + " " + body[:min(len(body), 200)]
	}
	apilog.Record(tenantID, "wx_oauth2_access_token", "code 换 token", tok.ErrCode, logBody, err == nil && tok.ErrCode == 0)
	if err != nil {
		return nil, err
	}
	if tok.ErrCode != 0 || tok.OpenID == "" {
		return nil, fmt.Errorf("微信登录失败:%s", tok.ErrMsg)
	}
	res := &oauthResult{OpenID: tok.OpenID, UnionID: tok.UnionID}
	var info struct {
		Nickname   string `json:"nickname"`
		HeadImgURL string `json:"headimgurl"`
		ErrCode    int    `json:"errcode"`
	}
	infoURL := fmt.Sprintf("https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s&lang=zh_CN",
		url.QueryEscape(tok.AccessToken), url.QueryEscape(tok.OpenID))
	if _, err := wxGetJSON(infoURL, &info); err == nil && info.ErrCode == 0 {
		res.Nickname, res.Avatar = info.Nickname, info.HeadImgURL
		apilog.Record(tenantID, "wx_sns_userinfo", "取昵称头像", 0, "nickname="+info.Nickname, true)
	}
	return res, nil
}

// wxGetJSON GET 并解析 JSON,返回原文供排错。
func wxGetJSON(u string, out interface{}) (string, error) {
	resp, err := httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), json.Unmarshal(b, out)
}
