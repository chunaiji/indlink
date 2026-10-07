package admin

import (
	"errors"
	"fmt"

	"driftbottle/internal/common/i18n"
	"driftbottle/internal/provider"
)

// 后台「服务商」三页(支付 / 地图 / 内容安全)的读写:schema 来自 provider 包,值来自 provider.Store。

type ProviderField struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Secret   bool     `json:"secret"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
	Value    string   `json:"value"` // 机密:"set" / ""
	Help     string   `json:"help,omitempty"`
}

type ProviderCard struct {
	Provider string          `json:"provider"`
	Label    string          `json:"label"`
	Platform string          `json:"platform"`
	DocURL   string          `json:"doc_url"`
	Enabled  bool            `json:"enabled"`
	Active   bool            `json:"active"`
	Complete bool            `json:"complete"`
	Missing  []string        `json:"missing"`
	Fields   []ProviderField `json:"fields"`
}

func (s *Service) SetProviderStore(ps *provider.Store) { s.providers = ps }

// SetProbe 注入「测试连通」分派(各域包各自实现,admin 不 import 它们)。
func (s *Service) SetProbe(fn func(tenantID int64, kind, prov string) (string, error)) { s.probe = fn }

// cardFor schema + 当前值 → 卡片。机密只告诉前端「设没设」;没有行时非机密字段回 schema 默认值。
func cardFor(d provider.Definition, row *provider.Resolved, lang i18n.Lang) ProviderCard {
	fields := map[string]string{}
	if row != nil {
		fields = row.Fields
	}
	c := ProviderCard{Provider: d.Provider, Label: pick(d.LabelZh, d.LabelEn, lang), Platform: d.Platform, DocURL: d.DocURL}
	if row != nil {
		c.Enabled, c.Active = row.Enabled, row.Active
	}
	c.Missing = provider.Missing(d, fields)
	if c.Missing == nil {
		c.Missing = []string{}
	}
	c.Complete = len(c.Missing) == 0
	for _, f := range d.Fields {
		v, has := fields[f.Key]
		if !has && row == nil {
			v = f.Default
		}
		if f.Secret && v != "" {
			v = "set"
		}
		c.Fields = append(c.Fields, ProviderField{Key: f.Key, Label: pick(f.LabelZh, f.LabelEn, lang), Type: f.Type, Secret: f.Secret,
			Required: f.Required, Options: f.Options, Value: v, Help: pick(f.HelpZh, f.HelpEn, lang)})
	}
	return c
}

var errNeedTenant = errors.New("请先选择租户")

// ListProviders 某领域对该租户可见的服务商卡片(按租户类型过滤,与 /config 同规则)。
func (s *Service) ListProviders(tenantID int64, kind string, lang i18n.Lang) ([]ProviderCard, error) {
	if tenantID == 0 {
		return nil, errNeedTenant
	}
	defs := provider.Definitions(kind)
	if len(defs) == 0 {
		return nil, fmt.Errorf("未知的服务商领域")
	}
	tenantType := s.tenantTypeOf(tenantID)
	out := make([]ProviderCard, 0, len(defs))
	for _, d := range defs {
		if !visibleFor(d.Platform, tenantType) {
			continue
		}
		var row *provider.Resolved
		if s.providers != nil {
			row, _ = s.providers.Get(tenantID, kind, d.Provider)
		}
		out = append(out, cardFor(d, row, lang))
	}
	return out, nil
}

type ProviderSaveReq struct {
	Enabled *bool             `json:"enabled"`
	Active  *bool             `json:"active"`
	Fields  map[string]string `json:"fields"`
}

func (s *Service) SaveProvider(tenantID int64, kind, prov string, req ProviderSaveReq) error {
	if tenantID == 0 {
		return errNeedTenant
	}
	if s.providers == nil {
		return fmt.Errorf("服务商存储未初始化")
	}
	if _, ok := provider.Find(kind, prov); !ok {
		return fmt.Errorf("未知的服务商")
	}
	// 支付宝内容安全依赖支付配置里的支付宝应用:没配就不让启用,免得开了个空壳
	if kind == provider.KindModeration && prov == "alipay" && req.Enabled != nil && *req.Enabled {
		if !s.providers.Usable(tenantID, provider.KindPay, "alipay") {
			return fmt.Errorf("请先在「支付」页配置并启用支付宝")
		}
	}
	return s.providers.Upsert(tenantID, kind, prov, provider.UpsertInput{Enabled: req.Enabled, Active: req.Active, Fields: req.Fields})
}

// ProbeProvider 测试连通。
func (s *Service) ProbeProvider(tenantID int64, kind, prov string) (string, error) {
	if tenantID == 0 {
		return "", errNeedTenant
	}
	if s.probe == nil {
		return "", fmt.Errorf("探活未配置")
	}
	return s.probe(tenantID, kind, prov)
}
