package robot

import (
	"slices"
	"testing"
)

func TestPickVariant(t *testing.T) {
	// 空变体 → ""
	if got := pickVariant(nil, nil); got != "" {
		t.Fatalf("empty variants: got %q, want empty", got)
	}
	// 单变体无排除 → 返回它
	if got := pickVariant([]string{"a"}, nil); got != "a" {
		t.Fatalf("single: got %q, want a", got)
	}
	// 两变体排除一条 → 必返回另一条(确定性)
	if got := pickVariant([]string{"a", "b"}, []string{"a"}); got != "b" {
		t.Fatalf("exclude a: got %q, want b", got)
	}
	// 全部被排除 → ""
	if got := pickVariant([]string{"a", "b"}, []string{"a", "b"}); got != "" {
		t.Fatalf("all excluded: got %q, want empty", got)
	}
	// 多变体无排除 → 返回值必在变体集内
	vs := []string{"x", "y", "z"}
	for i := 0; i < 20; i++ {
		if got := pickVariant(vs, nil); !slices.Contains(vs, got) {
			t.Fatalf("multi: got %q not in variants", got)
		}
	}
}
