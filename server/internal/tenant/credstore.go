package tenant

import (
	"fmt"
	"sync"
	"sync/atomic"

	"driftbottle/internal/crypto"
	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// Resolved 解密后的租户凭证,业务层直接使用。
type Resolved struct {
	TenantID            int64
	Platform            string
	AppID               string
	Secret              string // 登录密钥(已解密)
	MchID               string
	PayAPIv3Key         string
	PaySerialNo         string
	PayPrivateKeyPEM    string // 已解密 PEM
	PayPlatformKeyPEM   string
	PayPlatformSerial   string
	AlipayPrivateKeyPEM string
	AlipayPublicKey     string
	NotifyURL           string
}

// Store 凭证缓存:按 (platform,appid)、(tenantID,platform)、PayPlatformSerial 三索引。
type Store struct {
	db       *gorm.DB
	mu       sync.RWMutex
	byAppID  map[string]*Resolved
	byTenant map[string]*Resolved
	bySerial map[string]*Resolved // key = platform:PayPlatformSerial,供回调自动识别租户
	ver      atomic.Uint64
}

func New(db *gorm.DB) *Store {
	return &Store{db: db, byAppID: map[string]*Resolved{}, byTenant: map[string]*Resolved{}, bySerial: map[string]*Resolved{}}
}

func keyAppID(platform, appid string) string      { return platform + ":" + appid }
func keyTenant(tid int64, platform string) string { return fmt.Sprintf("%d:%s", tid, platform) }
func keySerial(platform, serial string) string    { return platform + ":" + serial }

// Reload 全量加载 active 凭证并解密缓存。
func (s *Store) Reload() error {
	var rows []model.AppCredential
	if err := s.db.Where("status = ?", "active").Find(&rows).Error; err != nil {
		return err
	}
	resolved := make([]*Resolved, 0, len(rows))
	for i := range rows {
		r, err := decrypt(&rows[i])
		if err != nil {
			return fmt.Errorf("解密凭证失败(appid=%s): %w", rows[i].AppID, err)
		}
		resolved = append(resolved, r)
	}
	s.swap(buildIndex(resolved))
	return nil
}

// buildIndex 纯函数:把解密后的行建成三张索引。拆出来是为了能在没有 DB 的测试里断言索引键。
func buildIndex(rows []*Resolved) (byAppID, byTenant, bySerial map[string]*Resolved) {
	byAppID = make(map[string]*Resolved, len(rows))
	byTenant = make(map[string]*Resolved, len(rows))
	bySerial = make(map[string]*Resolved, len(rows))
	for _, r := range rows {
		byAppID[keyAppID(r.Platform, r.AppID)] = r
		byTenant[keyTenant(r.TenantID, r.Platform)] = r
		if r.PayPlatformSerial != "" {
			// 同一商户号可同时服务 wx(小程序)与 wx_app(App),平台证书序列号相同——键必须带平台。
			bySerial[keySerial(r.Platform, r.PayPlatformSerial)] = r
		}
	}
	return
}

// swap 原子替换三张索引并把版本号 +1。读到不同版本的调用方(pay 的驱动缓存)据此整体失效。
func (s *Store) swap(a, t, ser map[string]*Resolved) {
	s.mu.Lock()
	s.byAppID, s.byTenant, s.bySerial = a, t, ser
	s.mu.Unlock()
	s.ver.Add(1)
}

// Version 每次 Reload 自增。
func (s *Store) Version() uint64 { return s.ver.Load() }

func decrypt(c *model.AppCredential) (*Resolved, error) {
	dec := func(enc string) (string, error) {
		if enc == "" {
			return "", nil
		}
		return crypto.Decrypt(enc)
	}
	secret, err := dec(c.SecretEnc)
	if err != nil {
		return nil, err
	}
	apiv3, err := dec(c.PayAPIv3KeyEnc)
	if err != nil {
		return nil, err
	}
	wxPriv, err := dec(c.PayPrivateKeyEnc)
	if err != nil {
		return nil, err
	}
	aliPriv, err := dec(c.AlipayPrivateKeyEnc)
	if err != nil {
		return nil, err
	}
	return &Resolved{
		TenantID: c.TenantID, Platform: c.Platform, AppID: c.AppID, Secret: secret,
		MchID: c.MchID, PayAPIv3Key: apiv3, PaySerialNo: c.PaySerialNo,
		PayPrivateKeyPEM: wxPriv, PayPlatformKeyPEM: c.PayPlatformKey, PayPlatformSerial: c.PayPlatformSerial,
		AlipayPrivateKeyPEM: aliPriv, AlipayPublicKey: c.AlipayPublicKey, NotifyURL: c.NotifyURL,
	}, nil
}

// ByPlatformSerial 按 平台 + 微信平台公钥 ID 反查租户(回调免 tenantId 用)。
func (s *Store) ByPlatformSerial(platform, serial string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.bySerial[keySerial(platform, serial)]
	return r, ok
}

// ByAppID 按平台 + appid 解析(登录用)。
func (s *Store) ByAppID(platform, appid string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byAppID[keyAppID(platform, appid)]
	return r, ok
}

// ByTenantPlatform 按租户 + 平台解析(支付下单/回调用)。
func (s *Store) ByTenantPlatform(tid int64, platform string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byTenant[keyTenant(tid, platform)]
	return r, ok
}

// All 全部解密行(迁移用)。
func (s *Store) All() []*Resolved {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Resolved, 0, len(s.byAppID))
	for _, r := range s.byAppID {
		out = append(out, r)
	}
	return out
}
