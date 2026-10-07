package admin

import (
	"testing"

	"driftbottle/internal/common/i18n"
)

func TestConfigMetaIsComplete(t *testing.T) {
	if len(configMeta) == 0 {
		t.Fatal("configMeta 为空")
	}
	seen := map[string]bool{}
	for i, f := range configMeta {
		if f.Key == "" {
			t.Errorf("configMeta[%d] Key 为空", i)
			continue
		}
		if seen[f.Key] {
			t.Errorf("配置键重复: %s", f.Key)
		}
		seen[f.Key] = true

		if f.LabelZh == "" {
			t.Errorf("%s 缺中文标签", f.Key)
		}
		if f.LabelEn == "" {
			t.Errorf("%s 缺英文标签", f.Key)
		}
		if f.Group == "" {
			t.Errorf("%s 缺分组", f.Key)
		} else if _, ok := groupMeta[f.Group]; !ok {
			t.Errorf("%s 的分组 %q 不在 groupMeta 中", f.Key, f.Group)
		}
		switch f.Type {
		case "bool", "int", "text", "textarea", "image":
		default:
			t.Errorf("%s 的类型 %q 非法", f.Key, f.Type)
		}
	}
}

// 分组 code 必须是稳定标识,不能含中文 —— 前端拿它做逻辑判断。
func TestGroupCodesAreStable(t *testing.T) {
	for code, g := range groupMeta {
		for _, r := range code {
			if r > 127 {
				t.Errorf("分组 code %q 含非 ASCII 字符,前端逻辑依赖它,必须稳定", code)
				break
			}
		}
		if g.LabelZh == "" {
			t.Errorf("分组 %s 缺中文名", code)
		}
		if g.LabelEn == "" {
			t.Errorf("分组 %s 缺英文名", code)
		}
	}
}

// 每个分组都必须归属一个分区。
//
// 这条是整个拆分方案的安全绳:前端按分区渲染 Tab,一个没有分区的分组
// **不会报错,它会从界面上彻底消失**——而配置项消失是没人会立刻发现的那种坏法。
func TestEveryGroupHasASection(t *testing.T) {
	for code, g := range groupMeta {
		if g.Section == "" {
			t.Errorf("分组 %s 没有归属分区,它会在后台界面上消失", code)
			continue
		}
		if _, ok := sectionLabels[g.Section]; !ok {
			t.Errorf("分组 %s 的分区 %q 未在 sectionLabels 中定义", code, g.Section)
		}
	}
}

// 分区也要有双语名,否则英文后台会露出分区 code。
func TestSectionsAreBilingual(t *testing.T) {
	if len(sectionLabels) == 0 {
		t.Fatal("sectionLabels 为空")
	}
	for code, labels := range sectionLabels {
		if labels[i18n.ZhCN] == "" {
			t.Errorf("分区 %s 缺中文名", code)
		}
		if labels[i18n.En] == "" {
			t.Errorf("分区 %s 缺英文名", code)
		}
	}
}

// 空分区会在后台渲染出一个点进去什么都没有的 Tab。
func TestNoEmptySections(t *testing.T) {
	used := map[string]bool{}
	for _, f := range configMeta {
		if g, ok := groupMeta[f.Group]; ok {
			used[g.Section] = true
		}
	}
	for code := range sectionLabels {
		if !used[code] {
			t.Errorf("分区 %s 下没有任何配置项,会渲染出一个空 Tab", code)
		}
	}
}

// 无用分组会随着配置项删改悄悄堆积,下一个人无从判断还作不作数。
func TestNoOrphanGroups(t *testing.T) {
	used := map[string]bool{}
	for _, f := range configMeta {
		used[f.Group] = true
	}
	for code := range groupMeta {
		if !used[code] {
			t.Errorf("分组 %s 已无配置项使用,请删除", code)
		}
	}
}

func TestResolveFieldPicksLanguage(t *testing.T) {
	m := configFieldMeta{
		Key: "x", LabelZh: "通用开关", LabelEn: "General switch",
		Group: GroupCommon, Type: "bool",
	}

	zh := resolveField(m, "1", i18n.ZhCN)
	if zh.Label != "通用开关" {
		t.Errorf("中文标签 = %q", zh.Label)
	}
	if zh.GroupLabel != "通用" {
		t.Errorf("中文分组名 = %q", zh.GroupLabel)
	}

	en := resolveField(m, "1", i18n.En)
	if en.Label != "General switch" {
		t.Errorf("英文标签 = %q", en.Label)
	}
	if en.GroupLabel != "General" {
		t.Errorf("英文分组名 = %q", en.GroupLabel)
	}

	// Group 是稳定 code,不随语言变 —— 前端逻辑依赖这一点。
	if zh.Group != en.Group || zh.Group != GroupCommon {
		t.Errorf("Group code 随语言变了: zh=%q en=%q", zh.Group, en.Group)
	}
	if en.Value != "1" {
		t.Errorf("Value = %q, 期望 1", en.Value)
	}
}
