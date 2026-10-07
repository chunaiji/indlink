// 微信内容安全接入(过审要求):
//   - 文本:msgSecCheck v2 同步检测,所有 UGC 发布场景生效(CheckUGC)
//   - 图片:mediaCheckAsync 异步检测,C 端上传统一入口触发;违规回调后删除文件
//
// 开关在后台「服务商 → 内容安全」页(text_on / image_on),关闭则完全跳过。
// API 故障/openid 不可用(支付宝用户、2小时未访问)时放行并记日志——本地词库仍兜底,不阻断业务。
package moderation

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/apilog"
)

// 微信 msgSecCheck 场景值。
const (
	SceneProfile = 1 // 资料(昵称/签名)
	SceneComment = 2 // 评论(回信/聊天/动态评论)
	SceneSocial  = 4 // 社交日志(瓶子/动态)
)

// SetWxSource 注入微信 API 依赖(main 装配):token 取自 push 模块的按租户缓存;
// uploadDir/publicBaseURL 用于回调命中违规时把图片 URL 映射回磁盘路径删除。
func (s *Service) SetWxSource(tokenFn func(int64) (string, error), uploadDir, publicBaseURL string) {
	s.tokenFn = tokenFn
	s.uploadDir = uploadDir
	s.publicBase = strings.TrimRight(publicBaseURL, "/")
}

// wechatChecker 微信内容安全(msgSecCheck / mediaCheckAsync)。开关判断在 Service.CheckUGC / CheckImageAsync。
type wechatChecker struct{ s *Service }

func (w *wechatChecker) CheckText(tenantID, userID int64, scene int, text string) error {
	return w.s.wxMsgSecCheck(tenantID, userID, scene, text)
}

func (w *wechatChecker) CheckImageAsync(tenantID, userID int64, mediaURL string) {
	w.s.wxMediaCheckAsync(tenantID, userID, mediaURL)
}

// Probe 探 access_token 而不是 msgSecCheck:后者要真实 openid,拿不到就在发 HTTP 前返回,
// 探不出任何东西。能取到 token 就说明该租户的 appid/secret 可用,内容安全这条链路是通的。
func (w *wechatChecker) Probe(tenantID int64) error {
	if w.s.tokenFn == nil {
		return errors.New("未接入微信 access_token 来源")
	}
	if _, err := w.s.tokenFn(tenantID); err != nil {
		return fmt.Errorf("取 access_token 失败: %w", err)
	}
	return nil
}

func (s *Service) wxMsgSecCheck(tenantID, userID int64, scene int, content string) error {
	openid := s.wxOpenID(userID)
	if openid == "" || s.tokenFn == nil {
		return nil // 非微信用户/未接 token:跳过在线检测
	}
	token, err := s.tokenFn(tenantID)
	if err != nil {
		log.Printf("[seccheck] token err tenant=%d: %v", tenantID, err)
		apilog.Record(tenantID, "msg_sec_check", "获取 access_token 失败(检查该租户推送分组的 appid/secret)", 0, err.Error(), false)
		return nil
	}
	// 微信限制 content ≤ 2500 字,超长截断分段意义不大,取前 2500
	rs := []rune(content)
	if len(rs) > 2500 {
		content = string(rs[:2500])
	}
	body, _ := json.Marshal(map[string]interface{}{
		"content": content, "version": 2, "scene": scene, "openid": openid,
	})
	resp, err := http.Post("https://api.weixin.qq.com/wxa/msg_sec_check?access_token="+token,
		"application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[seccheck] msg_sec_check http err: %v", err)
		return nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var r struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
		Result  struct {
			Suggest string `json:"suggest"`
			Label   int    `json:"label"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil
	}
	apilog.Record(tenantID, "msg_sec_check",
		fmt.Sprintf("scene=%d user=%d content=%s", scene, userID, content),
		r.Errcode, "suggest="+r.Result.Suggest+" "+r.Errmsg, r.Errcode == 0)
	if r.Errcode != 0 {
		// 61010=openid 超 2 小时未访问等:放行,本地词库已兜底
		log.Printf("[seccheck] msg_sec_check errcode=%d msg=%s", r.Errcode, r.Errmsg)
		return nil
	}
	if r.Result.Suggest == "risky" {
		log.Printf("[seccheck] text risky tenant=%d user=%d scene=%d label=%d", tenantID, userID, scene, r.Result.Label)
		return errs.ErrContentBlock
	}
	return nil
}

// wxMediaCheckAsync 提交图片异步检测(非微信用户 / 未接 token 跳过)。
// 结果经微信消息推送回调 HandleWxMediaEvent,risky 时删除磁盘文件。
func (s *Service) wxMediaCheckAsync(tenantID, userID int64, mediaURL string) {
	if mediaURL == "" || s.tokenFn == nil {
		return
	}
	openid := s.wxOpenID(userID)
	if openid == "" {
		return
	}
	go func() {
		token, err := s.tokenFn(tenantID)
		if err != nil {
			log.Printf("[seccheck] token err tenant=%d: %v", tenantID, err)
			apilog.Record(tenantID, "media_check_async", "获取 access_token 失败(检查该租户推送分组的 appid/secret) url="+mediaURL, 0, err.Error(), false)
			return
		}
		body, _ := json.Marshal(map[string]interface{}{
			"media_url": mediaURL, "media_type": 2, "version": 2, "scene": SceneSocial, "openid": openid,
		})
		resp, err := http.Post("https://api.weixin.qq.com/wxa/media_check_async?access_token="+token,
			"application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("[seccheck] media_check_async http err: %v", err)
			return
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var r struct {
			Errcode int    `json:"errcode"`
			Errmsg  string `json:"errmsg"`
			TraceID string `json:"trace_id"`
		}
		ok := json.Unmarshal(raw, &r) == nil && r.Errcode == 0 && r.TraceID != ""
		apilog.Record(tenantID, "media_check_async",
			fmt.Sprintf("user=%d url=%s", userID, mediaURL),
			r.Errcode, "trace_id="+r.TraceID+" "+r.Errmsg, ok)
		if !ok {
			log.Printf("[seccheck] media_check_async errcode=%d msg=%s", r.Errcode, r.Errmsg)
			return
		}
		s.db.Create(&model.WxMediaCheck{
			TenantID: tenantID, UserID: userID, TraceID: r.TraceID, URL: mediaURL,
			Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		})
	}()
}

// VerifyWxSignature 校验微信消息推送签名(GET 验证与 POST 事件共用)。
// URL 带 ?tenant=<id> 定位租户,Token 取该租户 sysconfig。
func (s *Service) VerifyWxSignature(tenantID int64, signature, timestamp, nonce string) bool {
	token := sysconfig.GetString(tenantID, sysconfig.KeySecCallbackToken)
	if token == "" {
		return false
	}
	arr := []string{token, timestamp, nonce}
	sort.Strings(arr)
	h := sha1.Sum([]byte(strings.Join(arr, "")))
	return hex.EncodeToString(h[:]) == signature
}

// wxMediaEvent 微信 media_check 异步结果事件(消息推送 JSON 明文模式)。
type wxMediaEvent struct {
	Event   string `json:"Event"`
	TraceID string `json:"trace_id"`
	Result  struct {
		Suggest string `json:"suggest"`
		Label   int    `json:"label"`
	} `json:"result"`
}

// HandleWxMediaEvent 处理 wxa_media_check 回调:回填状态;risky → 删除磁盘文件(引用处裂图兜底)。
func (s *Service) HandleWxMediaEvent(raw []byte) {
	var ev wxMediaEvent
	if json.Unmarshal(raw, &ev) != nil || ev.Event != "wxa_media_check" || ev.TraceID == "" {
		return
	}
	var rec model.WxMediaCheck
	if err := s.db.First(&rec, "trace_id = ?", ev.TraceID).Error; err != nil {
		apilog.Record(0, "media_check_callback", "trace_id="+ev.TraceID+"(未匹配记录)", 0, "suggest="+ev.Result.Suggest, false)
		return
	}
	apilog.Record(rec.TenantID, "media_check_callback",
		fmt.Sprintf("trace_id=%s url=%s", ev.TraceID, rec.URL), 0, "suggest="+ev.Result.Suggest, true)
	status := "pass"
	if ev.Result.Suggest == "risky" {
		status = "risky"
		s.removeUploadedFile(rec.URL)
		log.Printf("[seccheck] media risky tenant=%d user=%d url=%s label=%d", rec.TenantID, rec.UserID, rec.URL, ev.Result.Label)
	}
	s.db.Model(&model.WxMediaCheck{}).Where("trace_id = ?", ev.TraceID).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()})
}

// removeUploadedFile 把公网 URL 映射回上传目录路径并删除(仅允许删 /static 下的文件,防目录穿越)。
func (s *Service) removeUploadedFile(rawURL string) {
	if s.uploadDir == "" {
		return
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	p := u.Path // 形如 /message/static/20260810/xxx.jpg 或 /static/20260810/xxx.jpg
	idx := strings.Index(p, "/static/")
	if idx < 0 {
		return
	}
	rel := strings.TrimPrefix(p[idx:], "/static/")
	rel = filepath.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, "..") {
		return
	}
	full := filepath.Join(s.uploadDir, rel)
	if err := os.Remove(full); err != nil {
		log.Printf("[seccheck] remove file err %s: %v", full, err)
	}
}

// wxOpenID 取用户微信 openid;支付宝用户为空(跳过微信在线检测)。
func (s *Service) wxOpenID(userID int64) string {
	var u model.User
	if err := s.db.Select("wx_openid").First(&u, "user_id = ?", userID).Error; err != nil {
		return ""
	}
	return u.WxOpenID
}
