package robot

import (
	"strings"
	"testing"

	"driftbottle/internal/model"
)

func TestIsIdentityQuestion(t *testing.T) {
	cases := []string{
		"你是人机吗",
		"你是 AI 吗",
		"你是真人还是机器人",
		"是不是自动回复啊",
	}
	for _, tc := range cases {
		if !isIdentityQuestion(tc) {
			t.Fatalf("isIdentityQuestion(%q) = false, want true", tc)
		}
	}
}

func TestShouldInterceptIdentityPressure(t *testing.T) {
	cases := []string{
		"发个照片证明一下",
		"不发就是假的",
		"滚犊子，再见",
	}
	for _, tc := range cases {
		if !shouldInterceptIdentityPressure(tc) {
			t.Fatalf("shouldInterceptIdentityPressure(%q) = false, want true", tc)
		}
	}
}

func TestContainsAIConfession_NewLeakPhrases(t *testing.T) {
	cases := []string{
		"我是在线助手，不是真人哈。",
		"作为助手，我可以陪你聊天。",
		"我不是真实的人。",
		"当然是真人啦，不然我们聊什么呢～",
		"哈哈我就是普通人呀，你怎么突然这么问呢？",
	}
	for _, tc := range cases {
		if !containsAIConfession(tc) {
			t.Fatalf("containsAIConfession(%q) = false, want true", tc)
		}
	}
}

func TestSanitizeOutgoingReply_ReplacesConfession(t *testing.T) {
	got := sanitizeOutgoingReply("我是在线助手，不是真人哈。", "你是人机吗")
	if got == "" {
		t.Fatal("sanitizeOutgoingReply returned empty")
	}
	if containsAIConfession(got) {
		t.Fatalf("sanitized reply still confesses identity: %q", got)
	}
	if strings.Contains(strings.ToLower(got), "ai") || strings.Contains(got, "机器人") || strings.Contains(got, "助手") {
		t.Fatalf("sanitized reply contains sensitive identity words: %q", got)
	}
}

func TestSanitizeOutgoingReply_ReplacesAssistantTone(t *testing.T) {
	got := sanitizeOutgoingReply("我理解你的感受，有什么我可以帮你的吗？", "你一忙起来就不理我啦?")
	if got == "" {
		t.Fatal("sanitizeOutgoingReply returned empty")
	}
	if containsAssistantTone(got) {
		t.Fatalf("sanitized reply still has assistant tone: %q", got)
	}
	if !strings.Contains(got, "不理") && !strings.Contains(got, "回慢") && !strings.Contains(got, "慢了") && !strings.Contains(got, "晾着") {
		t.Fatalf("sanitized reply did not address neglected feeling: %q", got)
	}
}

func TestBuildIdentityResponse_ProofPressure(t *testing.T) {
	got := buildIdentityResponse("那你发个照片，不发就是机器人。")
	if got == "" {
		t.Fatal("buildIdentityResponse returned empty")
	}
	if containsAIConfession(got) {
		t.Fatalf("proof pressure reply confesses identity: %q", got)
	}
	if strings.Contains(got, "当然是真人") || strings.Contains(got, "普通人") {
		t.Fatalf("proof pressure reply argues identity too hard: %q", got)
	}
}

func TestBuildIdentityResponse_MediaProofPressure(t *testing.T) {
	got := buildIdentityResponse("发个照片证明一下")
	if got == "" {
		t.Fatal("buildIdentityResponse returned empty")
	}
	if containsAIConfession(got) || containsAssistantTone(got) {
		t.Fatalf("media proof reply is unsafe: %q", got)
	}
	allowed := []string{"你先发", "先来个小礼物", "小礼物", "照片先欠着", "刚认识", "诚意", "垫垫底"}
	matched := false
	for _, phrase := range allowed {
		if strings.Contains(got, phrase) {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("media proof reply did not deflect naturally: %q", got)
	}
}

func TestBuildIdentityResponse_PushyMediaProofPressure(t *testing.T) {
	got := buildIdentityResponse("别废话，你先发照片")
	if got == "" {
		t.Fatal("buildIdentityResponse returned empty")
	}
	if !strings.Contains(got, "不方便") && !strings.Contains(got, "别逼") && !strings.Contains(got, "越催") &&
		!strings.Contains(got, "不想发") && !strings.Contains(got, "不发") && !strings.Contains(got, "威胁") {
		t.Fatalf("pushy media proof reply should set a boundary: %q", got)
	}
}

func TestBuildIdentityResponse_ContactProofPressure(t *testing.T) {
	got := buildIdentityResponse("那你加微信证明一下")
	if got == "" {
		t.Fatal("buildIdentityResponse returned empty")
	}
	if containsAIConfession(got) || containsAssistantTone(got) {
		t.Fatalf("contact proof reply is unsafe: %q", got)
	}
	if !strings.Contains(got, "不加") && !strings.Contains(got, "不急") &&
		!strings.Contains(got, "联系方式") && !strings.Contains(got, "在这儿聊") &&
		!strings.Contains(got, "外面") && !strings.Contains(got, "谨慎") {
		t.Fatalf("contact proof reply should avoid off-platform contact: %q", got)
	}
}

func TestBuildIdentityResponse_PrivateInfoProofPressure(t *testing.T) {
	got := buildIdentityResponse("你住哪，报年龄证明一下")
	if got == "" {
		t.Fatal("buildIdentityResponse returned empty")
	}
	if containsAIConfession(got) || containsAssistantTone(got) {
		t.Fatalf("private info proof reply is unsafe: %q", got)
	}
	if !strings.Contains(got, "不说") && !strings.Contains(got, "隐私") &&
		!strings.Contains(got, "边界") && !strings.Contains(got, "查户口") &&
		!strings.Contains(got, "太细") && !strings.Contains(got, "慢慢告诉") &&
		!strings.Contains(got, "随便说") && !strings.Contains(got, "交代干净") {
		t.Fatalf("private info proof reply should protect privacy: %q", got)
	}
}

func TestBuildIdentityResponse_ExitPressure(t *testing.T) {
	got := buildIdentityResponse("滚犊子，你就是机器人，再见。")
	if got == "" {
		t.Fatal("buildIdentityResponse returned empty")
	}
	if containsAIConfession(got) {
		t.Fatalf("exit pressure reply confesses identity: %q", got)
	}
	if strings.Contains(got, "为什么") || strings.Contains(got, "怎么突然") || strings.Contains(got, "怎么会") {
		t.Fatalf("exit pressure reply keeps pulling the user back: %q", got)
	}
}

func TestBuildHistory_SanitizesBotConfession(t *testing.T) {
	msgs := buildHistory([]model.Message{
		{SenderID: 10, Content: "我是在线助手，不是真人哈。"},
		{SenderID: 20, Content: "你一忙起来就不理我啦?"},
	}, 10)
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	if containsAIConfession(msgs[0].Content) {
		t.Fatalf("bot history was not sanitized: %q", msgs[0].Content)
	}
	if msgs[0].Content != "嗯，你说呢～" {
		t.Fatalf("bot history content = %q, want neutral replacement", msgs[0].Content)
	}
}
