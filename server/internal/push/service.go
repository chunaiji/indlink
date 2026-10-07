package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/tenant"
	"driftbottle/pkg/apilog"

	"gorm.io/gorm"
)

// tokenEntry 单个租户的 access_token 及其过期时间。
type tokenEntry struct {
	token string
	exp   time.Time
}

// Service 微信订阅消息推送服务。
type Service struct {
	db        *gorm.DB
	mu        sync.Mutex
	tokens    map[int64]tokenEntry // tenantID → access_token 缓存
	sentToday sync.Map             // key: "tenantID:userID:YYYYMMDD" → struct{}，新消息推送日内去重
	creds     *tenant.Store        // 租户凭证(与登录同源);appid/secret 优先取这里,sysconfig 仅后备
}

func New(db *gorm.DB) *Service { return &Service{db: db, tokens: map[int64]tokenEntry{}} }

// SetCredStore 注入租户凭证缓存(main 装配):多租户各自的 appid/secret 与登录同源,无需重复配置。
func (s *Service) SetCredStore(cs *tenant.Store) { s.creds = cs }

// Subscribe 记录/更新用户对某场景的订阅授权。
func (s *Service) Subscribe(tenantID, userID int64, openID, templateID, scene string) error {
	now := time.Now()
	sub := model.PushSubscription{
		TenantID: tenantID, UserID: userID, OpenID: openID,
		TemplateID: templateID, Scene: scene, CreatedAt: now, UpdatedAt: now,
	}
	return s.db.
		Where(model.PushSubscription{TenantID: tenantID, UserID: userID, Scene: scene}).
		Assign(map[string]interface{}{"open_id": openID, "template_id": templateID, "updated_at": now}).
		FirstOrCreate(&sub).Error
}

// TplInfo 模板 ID 及其对应的场景标识，供前端订阅时建立映射。
type TplInfo struct {
	ID    string `json:"id"`
	Scene string `json:"scene"`
}

// GetTemplates 返回当前已配置的模板列表，含 id 和 scene。
func (s *Service) GetTemplates(tenantID int64) []TplInfo {
	pairs := []struct{ key, scene string }{
		{sysconfig.KeyPushTplReply, "reply"},
		{sysconfig.KeyPushTplChat, "chat"},
		{sysconfig.KeyPushTplActivity, "activity"},
		{sysconfig.KeyPushTplWorkRecommend, "work_recommend"},
		{sysconfig.KeyPushTplCheckin, "checkin"},
	}
	var list []TplInfo
	for _, p := range pairs {
		if id := sysconfig.GetString(tenantID, p.key); id != "" {
			list = append(list, TplInfo{ID: id, Scene: p.scene})
		}
	}
	return list
}

// SubscribeByUser 记录订阅，openID 由服务端从用户表中自取。
func (s *Service) SubscribeByUser(tenantID, userID int64, templateID, scene string) error {
	var u model.User
	if err := s.db.Select("wx_openid").First(&u, "user_id = ?", userID).Error; err != nil {
		return err
	}
	if u.WxOpenID == "" {
		return nil // 尚未绑定 openID，静默跳过
	}
	return s.Subscribe(tenantID, userID, u.WxOpenID, templateID, scene)
}

// NotifyNewMessage 有人给用户发消息时触发"新作品推荐提醒"，当天内只推一次。
func (s *Service) NotifyNewMessage(tenantID, userID int64) {
	today := time.Now().Format("20060102")
	key := fmt.Sprintf("%d:%d:%s", tenantID, userID, today)
	if _, loaded := s.sentToday.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	s.SendWorkRecommend(tenantID, userID, "新消息提醒", "有人给你发来了消息", "点击打开查看", "/pages/chat/index")
}

// SendActivity 活动预约提醒推送。
// 微信模板字段名请对照模板 pCcmAdWN2BrNao4JB_Zx0WYyqY6Ip4r-dtnER321nbs 的实际定义。
func (s *Service) SendActivity(tenantID, userID int64, nickname, activityDesc, publisher, page string) {
	s.Send(tenantID, userID, "activity", page, map[string]MsgValue{
		"thing1": {Value: nickname},
		"thing2": {Value: activityDesc},
		"thing3": {Value: publisher},
	})
}

// SendWorkRecommend 新作品推荐提醒推送。
// 微信模板字段名请对照模板 1QTc2A0zT5RUm0Khm6EGv33UPEGCWdkBwDu02steWn4 的实际定义。
func (s *Service) SendWorkRecommend(tenantID, userID int64, author, reason, tip, page string) {
	s.Send(tenantID, userID, "work_recommend", page, map[string]MsgValue{
		"thing1": {Value: author},
		"thing2": {Value: reason},
		"thing3": {Value: tip},
	})
}

// SendCheckin 签到提醒推送。
// 微信模板字段名请对照模板 jDkqqnbLivP1FxZsIWXdmnuofZKCyHiuEMyvIEEjKrE 的实际定义。
func (s *Service) SendCheckin(tenantID, userID int64, method, status, datetime, page string) {
	s.Send(tenantID, userID, "checkin", page, map[string]MsgValue{
		"thing1":     {Value: method},
		"thing2":     {Value: status},
		"date_time1": {Value: datetime},
	})
}

// SendDirect 管理员后台直接向用户推送，不经过 subscription 记录，同步返回发送结果。
func (s *Service) SendDirect(userID int64, scene, f1, f2, f3, page string) error {
	var u model.User
	if err := s.db.Select("wx_openid, tenant_id").First(&u, "user_id = ?", userID).Error; err != nil {
		return err
	}
	if u.WxOpenID == "" {
		return fmt.Errorf("该用户未绑定微信 openid，无法推送")
	}
	sceneToKey := map[string]string{
		"reply":          sysconfig.KeyPushTplReply,
		"chat":           sysconfig.KeyPushTplChat,
		"activity":       sysconfig.KeyPushTplActivity,
		"work_recommend": sysconfig.KeyPushTplWorkRecommend,
		"checkin":        sysconfig.KeyPushTplCheckin,
	}
	key, ok := sceneToKey[scene]
	if !ok {
		return fmt.Errorf("未知场景: %s", scene)
	}
	tmplID := sysconfig.GetString(u.TenantID, key)
	if tmplID == "" {
		return fmt.Errorf("场景 %s 尚未在系统配置中填写模板 ID", scene)
	}
	data := map[string]MsgValue{
		"thing1": {Value: f1},
		"thing2": {Value: f2},
	}
	if scene == "checkin" {
		data["date_time1"] = MsgValue{Value: f3}
	} else {
		data["thing3"] = MsgValue{Value: f3}
	}
	token, err := s.getAccessToken(u.TenantID)
	if err != nil {
		return fmt.Errorf("获取 access_token 失败: %w", err)
	}
	payload := map[string]interface{}{
		"touser":      u.WxOpenID,
		"template_id": tmplID,
		"page":        page,
		"data":        data,
	}
	b, _ := json.Marshal(payload)
	apiURL := "https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=" + token
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(b)) //nolint:noctx
	if err != nil {
		return fmt.Errorf("请求微信接口失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败: %s", string(body))
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("微信返回错误 %d: %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

// Send 向指定租户下某用户的某场景发送订阅消息(异步,失败静默)。
func (s *Service) Send(tenantID, userID int64, scene, page string, data map[string]MsgValue) {
	var subs []model.PushSubscription
	s.db.Where("tenant_id = ? AND user_id = ? AND scene = ?", tenantID, userID, scene).Find(&subs)
	for _, sub := range subs {
		go s.sendOne(tenantID, sub.OpenID, sub.TemplateID, page, data)
	}
}

// MsgValue 模板消息单个字段的值。
type MsgValue struct {
	Value string `json:"value"`
}

// AccessToken 对外暴露租户 access_token(带缓存),供内容安全等其他微信 API 复用。
func (s *Service) AccessToken(tenantID int64) (string, error) { return s.getAccessToken(tenantID) }

// --- 内部: access_token 管理 + 实际发送 ---

func (s *Service) getAccessToken(tenantID int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.tokens[tenantID]; ok && time.Now().Before(e.exp) {
		return e.token, nil
	}
	// 优先:租户凭证表(app_credentials,登录同源,后台「租户凭证」页维护);后备:sysconfig 推送分组
	appid := sysconfig.GetString(tenantID, sysconfig.KeyPushWxAppID)
	secret := sysconfig.GetString(tenantID, sysconfig.KeyPushWxSecret)
	if s.creds != nil {
		if r, ok := s.creds.ByTenantPlatform(tenantID, "wx"); ok && r.AppID != "" && r.Secret != "" {
			appid, secret = r.AppID, r.Secret
		}
	}
	if appid == "" || secret == "" {
		return "", fmt.Errorf("push: appid/secret 未配置(租户凭证页或推送分组二选一)")
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s", appid, secret)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.AccessToken == "" {
		return "", fmt.Errorf("push: 获取 access_token 失败: %s", string(body))
	}
	if s.tokens == nil {
		s.tokens = map[int64]tokenEntry{}
	}
	e := tokenEntry{token: result.AccessToken, exp: time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)}
	s.tokens[tenantID] = e
	return e.token, nil
}

func (s *Service) sendOne(tenantID int64, openID, tmplID, page string, data map[string]MsgValue) {
	token, err := s.getAccessToken(tenantID)
	if err != nil {
		return
	}
	payload := map[string]interface{}{
		"touser":      openID,
		"template_id": tmplID,
		"page":        page,
		"data":        data,
	}
	b, _ := json.Marshal(payload)
	url := "https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=" + token
	resp, err := http.Post(url, "application/json", bytes.NewReader(b)) //nolint:noctx
	if err != nil {
		apilog.Record(tenantID, "subscribe_send", "openid="+openID+" tmpl="+tmplID, 0, err.Error(), false)
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var r struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	_ = json.Unmarshal(raw, &r)
	apilog.Record(tenantID, "subscribe_send", "openid="+openID+" tmpl="+tmplID, r.Errcode, r.Errmsg, r.Errcode == 0)
}
