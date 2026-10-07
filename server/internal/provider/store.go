package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"driftbottle/internal/crypto"
	"driftbottle/internal/model"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// Resolved 解密后的一行。
type Resolved struct {
	TenantID int64
	Kind     string
	Provider string
	Enabled  bool
	Active   bool
	Fields   map[string]string
}

func (r *Resolved) Get(k string) string { return r.Fields[k] }

// Bool "1" / "true"(忽略大小写)为真。
func (r *Resolved) Bool(k string) bool {
	v := strings.ToLower(strings.TrimSpace(r.Fields[k]))
	return v == "1" || v == "true"
}

type Store struct {
	db          *gorm.DB
	mu          sync.RWMutex
	byKey       map[string]*Resolved
	byLookup    map[string]*Resolved // nil 值 = 歧义占位
	tenantTypes map[int64]string
	ver         atomic.Uint64
}

func New(db *gorm.DB) *Store {
	return &Store{db: db, byKey: map[string]*Resolved{}, byLookup: map[string]*Resolved{}, tenantTypes: map[int64]string{}}
}

func keyOf(tid int64, kind, provider string) string {
	return fmt.Sprintf("%d:%s:%s", tid, kind, provider)
}

func lookupKey(kind, provider, a, b string) string { return kind + ":" + provider + ":" + a + "|" + b }

// encodeFields / decodeFields 整包加密;未配主密钥(本地开发)时明文 JSON。
func encodeFields(fields map[string]string) (string, error) {
	b, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	if !crypto.Enabled() {
		return string(b), nil
	}
	return crypto.Encrypt(string(b))
}

func decodeFields(enc string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(enc) == "" {
		return out, nil
	}
	raw := enc
	if !strings.HasPrefix(strings.TrimSpace(enc), "{") {
		plain, err := crypto.Decrypt(enc)
		if err != nil {
			return nil, err
		}
		raw = plain
	}
	return out, json.Unmarshal([]byte(raw), &out)
}

// lookupValues 把 schema 指定的两个字段投影成索引值。
func lookupValues(d Definition, fields map[string]string) (a, b string) {
	if d.LookupA != "" {
		a = strings.TrimSpace(fields[d.LookupA])
	}
	if d.LookupB != "" {
		b = strings.TrimSpace(fields[d.LookupB])
	}
	return
}

// buildIndex 建主键索引与反查索引。反查登记 (a,b)、(a,)、(,b) 三种键;
// 同一个键被两行命中 → 置 nil 当歧义占位(调用方拿到 ok=true && nil 就知道要再核对)。
func buildIndex(rows []*Resolved) (byKey, byLookup map[string]*Resolved) {
	byKey = make(map[string]*Resolved, len(rows))
	byLookup = map[string]*Resolved{}
	put := func(k string, r *Resolved) {
		if prev, seen := byLookup[k]; seen && prev != r {
			byLookup[k] = nil
			return
		}
		byLookup[k] = r
	}
	for _, r := range rows {
		byKey[keyOf(r.TenantID, r.Kind, r.Provider)] = r
		d, ok := Find(r.Kind, r.Provider)
		if !ok {
			continue
		}
		a, b := lookupValues(d, r.Fields)
		if a != "" && b != "" {
			put(lookupKey(r.Kind, r.Provider, a, b), r)
		}
		if a != "" {
			put(lookupKey(r.Kind, r.Provider, a, ""), r)
		}
		if b != "" {
			put(lookupKey(r.Kind, r.Provider, "", b), r)
		}
	}
	return
}

// Reload 全量加载并解密;顺带把租户类型表装进来(支付按租户类型定交易类型)。
func (s *Store) Reload() error {
	var rows []model.ProviderConfig
	if err := s.db.Find(&rows).Error; err != nil {
		return err
	}
	resolved := make([]*Resolved, 0, len(rows))
	for i := range rows {
		f, err := decodeFields(rows[i].FieldsEnc)
		if err != nil {
			return fmt.Errorf("解密服务商配置失败(tenant=%d %s/%s): %w", rows[i].TenantID, rows[i].Kind, rows[i].Provider, err)
		}
		resolved = append(resolved, &Resolved{TenantID: rows[i].TenantID, Kind: rows[i].Kind, Provider: rows[i].Provider,
			Enabled: rows[i].Enabled, Active: rows[i].Active, Fields: f})
	}
	var tenants []model.Tenant
	s.db.Select("tenant_id, type").Find(&tenants)
	types := make(map[int64]string, len(tenants))
	for _, t := range tenants {
		types[t.TenantID] = t.Type
	}
	byKey, byLookup := buildIndex(resolved)
	s.mu.Lock()
	s.byKey, s.byLookup, s.tenantTypes = byKey, byLookup, types
	s.mu.Unlock()
	s.ver.Add(1)
	return nil
}

func (s *Store) Version() uint64 { return s.ver.Load() }

func (s *Store) Get(tid int64, kind, provider string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byKey[keyOf(tid, kind, provider)]
	return r, ok
}

// Active 单选领域当前生效的那家:enabled && active。
//
// ⚠️ 必须**确定性**返回:map 迭代是随机的,脏数据(DBA 改库 / 部分备份恢复)留下两行 active 时,
// 随机挑一行会让同一进程内一半请求走腾讯、一半走 Google——结果漂移,是最难查的那种故障。
// 按 provider 名排序取第一个,并打日志让运维看得见。
func (s *Store) Active(tid int64, kind string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var hits []*Resolved
	for _, r := range s.byKey {
		if r.TenantID == tid && r.Kind == kind && r.Enabled && r.Active {
			hits = append(hits, r)
		}
	}
	if len(hits) == 0 {
		return nil, false
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Provider < hits[j].Provider })
	if len(hits) > 1 {
		names := make([]string, len(hits))
		for i, r := range hits {
			names[i] = r.Provider
		}
		log.Printf("[provider] tenant=%d kind=%s 有多行 active(%s),已取 %s;请到后台只保留一家",
			tid, kind, strings.Join(names, ","), hits[0].Provider)
	}
	return hits[0], true
}

// Usable enabled && 必填齐全(含被依赖卡片的必填)。/app-config 与下单前置判断都用它。
func (s *Store) Usable(tid int64, kind, provider string) bool {
	r, ok := s.Get(tid, kind, provider)
	if !ok || !r.Enabled {
		return false
	}
	d, ok := Find(kind, provider)
	if !ok {
		return false
	}
	var depFields map[string]string
	if d.DependsOn != nil {
		if dr, ok := s.Get(tid, d.DependsOn.Kind, d.DependsOn.Provider); ok {
			depFields = dr.Fields
		}
	}
	return len(MissingWith(d, r.Fields, depFields)) == 0
}

// ByLookup 回调反查。a / b 可只给一个;命中歧义或未命中都返回 false。
func (s *Store) ByLookup(kind, provider, a, b string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byLookup[lookupKey(kind, provider, strings.TrimSpace(a), strings.TrimSpace(b))]
	return r, ok && r != nil
}

func (s *Store) TenantType(tid int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tenantTypes[tid]
}

func (s *Store) All() []*Resolved {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Resolved, 0, len(s.byKey))
	for _, r := range s.byKey {
		out = append(out, r)
	}
	return out
}

type UpsertInput struct {
	Enabled *bool
	Active  *bool
	Fields  map[string]string
}

// mergeFields 旧值 + 本次写入 → 新值。机密写空保持,非机密写空清空,未知键丢弃。
func mergeFields(d Definition, old, in map[string]string) map[string]string {
	out := make(map[string]string, len(d.Fields))
	for _, f := range d.Fields {
		v, given := in[f.Key]
		switch {
		case !given:
			out[f.Key] = old[f.Key]
		case f.Secret && strings.TrimSpace(v) == "":
			out[f.Key] = old[f.Key]
		default:
			out[f.Key] = strings.TrimSpace(v)
		}
	}
	return out
}

var ErrUnknownProvider = errors.New("未知的服务商")

// Upsert 合并写一行;单选领域 active=true 时事务内把同领域其它行置 false;写完 Reload。
func (s *Store) Upsert(tid int64, kind, provider string, in UpsertInput) error {
	d, ok := Find(kind, provider)
	if !ok {
		return ErrUnknownProvider
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var row model.ProviderConfig
		err := tx.Where("tenant_id = ? AND kind = ? AND provider = ?", tid, kind, provider).First(&row).Error
		isNew := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !isNew {
			return err
		}
		old := map[string]string{}
		if !isNew {
			if old, err = decodeFields(row.FieldsEnc); err != nil {
				return err
			}
		} else {
			for _, f := range d.Fields {
				if f.Default != "" {
					old[f.Key] = f.Default
				}
			}
		}
		fields := mergeFields(d, old, in.Fields)
		enc, err := encodeFields(fields)
		if err != nil {
			return err
		}
		a, b := lookupValues(d, fields)
		now := time.Now()
		if isNew {
			row = model.ProviderConfig{ID: idgen.Next(), TenantID: tid, Kind: kind, Provider: provider, CreatedAt: now}
		}
		if in.Enabled != nil {
			row.Enabled = *in.Enabled
		}
		if in.Active != nil && SingleActive(kind) {
			row.Active = *in.Active
		}
		row.FieldsEnc, row.LookupA, row.LookupB, row.UpdatedAt = enc, a, b, now
		if row.Active && SingleActive(kind) {
			// 单选:先把同领域其它行清掉再保存本行,任何时刻至多一行 active。
			if err := tx.Model(&model.ProviderConfig{}).
				Where("tenant_id = ? AND kind = ? AND provider <> ?", tid, kind, provider).
				Update("active", false).Error; err != nil {
				return err
			}
		}
		if isNew {
			return tx.Create(&row).Error
		}
		return tx.Save(&row).Error
	})
	if err != nil {
		return err
	}
	return s.Reload()
}
