package robot

import "testing"

func TestBuildBottlePrompt_ContainsThemeAndMood(t *testing.T) {
	p := buildBottlePrompt("失眠的夜", "温柔治愈")
	if !contains(p, "失眠的夜") || !contains(p, "温柔治愈") {
		t.Fatalf("prompt 缺主题或语气: %s", p)
	}
	if !contains(p, "50字") {
		t.Fatalf("prompt 缺字数约束: %s", p)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
