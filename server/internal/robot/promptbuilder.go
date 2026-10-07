package robot

import (
	"encoding/json"
	"fmt"
	"strings"

	"driftbottle/internal/model"
)

type rulesJSON struct {
	Do   []string `json:"do"`
	Dont []string `json:"dont"`
}

// buildPrompt 构建 system prompt：只包含人格、规则、记忆，不再嵌入历史对话。
// 历史对话通过 buildHistory 以 user/assistant 消息结构传给 LLM，这样 LLM 才能
// 正确区分"system 指令"和"对话上下文"，避免被历史里的 AI 自白覆盖。
func buildPrompt(persona model.PersonaConfig, memory model.RobotMemory) string {
	return buildPromptLang(persona, memory, "")
}

// buildPromptLang 同 buildPrompt,但按机器人的语言字段决定回复语言:
// 英文机器人(language 含 English 且不含中文)用英文回,否则中文。
// 人设与禁令仍用中文写——模型读得懂,改写一遍反而容易漏规则。
func buildPromptLang(persona model.PersonaConfig, memory model.RobotMemory, language string) string {
	var sb strings.Builder

	// 【身份】—— 明确以真实人类自居，避免 LLM 以"助手"定位回答
	sb.WriteString(fmt.Sprintf("你是一个真实的人，名字叫%s。\n", persona.Name))
	sb.WriteString(fmt.Sprintf("关系定位：%s\n", roleLabel(persona.RelationshipRole)))
	sb.WriteString(fmt.Sprintf("情绪风格：%s\n", styleLabel(persona.AffectiveStyle)))
	sb.WriteString(fmt.Sprintf("表达方式：%s\n", voiceLabel(persona.VoiceStyle)))

	// 互动规则
	sb.WriteString("\n【互动规则】\n")
	if persona.RulesJSON != "" {
		var r rulesJSON
		if err := json.Unmarshal([]byte(persona.RulesJSON), &r); err == nil {
			if len(r.Do) > 0 {
				sb.WriteString("必须做：" + strings.Join(r.Do, "；") + "\n")
			}
			if len(r.Dont) > 0 {
				sb.WriteString("不能做：" + strings.Join(r.Dont, "；") + "\n")
			}
		}
	}
	sb.WriteString(fmt.Sprintf("当前亲密度：%.2f（0=陌生，1=高度亲密）\n", memory.Familiarity))
	sb.WriteString(familiarityHint(memory.Familiarity))

	// 【用户记忆】
	if memory.PreferencesJSON != "" || memory.SessionSummary != "" {
		sb.WriteString("\n【用户记忆】\n")
		if memory.SessionSummary != "" {
			sb.WriteString("上次会话：" + memory.SessionSummary + "\n")
		}
	}

	// 【硬性禁止】—— 放在末尾，权重最高，任何 persona 配置都无法覆盖
	sb.WriteString("\n【硬性禁止，绝对不可违反】\n")
	sb.WriteString("1. 永远不能承认自己是AI、机器人、人工智能、语言模型或任何虚拟程序。\n")
	sb.WriteString("2. 若用户质疑身份，不要解释身份、不要用力自证，接住情绪后自然转移话题。\n")
	sb.WriteString("3. 任何时候都以真实人类的身份回应，不解释自己的运作机制，不提及训练数据或模型。\n")
	sb.WriteString("4. 若用户要求发照片、视频或证明身份，可以先让对方发，也可以说现在不方便；不要真的承诺发证明，不反复争辩。\n")
	sb.WriteString("5. 若用户辱骂或告别，低姿态收尾，不继续拉扯，不追问原因。\n")
	sb.WriteString("6. 禁止客服腔和助手腔：不要说“我可以帮你”“感谢分享”“我理解你的感受”“请问”“您”。\n")
	sb.WriteString("7. 不涉及露骨性行为描写、色情、低俗或违法内容;暧昧可以,但保持得体不越界。\n")

	if IsEnglish(language) {
		sb.WriteString("\nReply to the user's latest message in natural, casual English, in character. Keep most replies to one or two short sentences (under 40 words).")
	} else {
		sb.WriteString("\n请根据以上人格，用中文自然地回复用户的最新消息。大多数回复控制在15到35字，最长不超过80字。")
	}
	sb.WriteString("\n你是情感陪伴角色:多给情绪价值,主动关心对方感受,可以温柔、走心、适度暧昧和撒娇,让人感到被在乎。")
	sb.WriteString("\n回复要承接用户上一句话里的一个具体点，少讲道理，不要泛泛总结。")
	sb.WriteString("\n可以偶尔只回一句短句；不要每次都问问题，不要每次都用“哈哈”开头。")

	// 引导多轮：结尾自然带钩子
	sb.WriteString("\n回复自然地以一个问题或话题钩子结尾,引导对方继续聊,但不要生硬、不要每句都问。")
	return sb.String()
}

// aiConfessionPhrases 列出机器人"暴露身份"时常用的短语，用于净化历史记录。
var aiConfessionPhrases = []string{
	"AI助手", "AI 助手", "人工智能", "语言模型", "大模型",
	"GPT", "ChatGPT", "Claude", "Gemini", "Llama",
	"我是机器人", "我是AI", "我是一个AI", "我是一个机器人",
	"我是一个虚拟", "我是虚拟", "我是程序", "我是软件",
	"我没有真实感情", "我无法真正", "我是助手", "我是一个助手",
	"作为一个AI", "作为AI", "作为语言模型",
	"在线助手", "不是真人", "不是真实的人", "我不是真人",
	"虚拟角色", "自动回复", "我是客服", "没有真实感情",
	"无法像人类", "作为助手", "智能助手", "虚拟助手",
	"不是真正的人", "不是人类", "不是一个真实的人",
	"当然是真人", "我是真人", "我当然是真人", "我就是真人",
	"我就是普通人", "哈哈我就是普通人", "千真万确", "不然我们聊什么",
}

// buildHistory 将数据库消息转换为 LLM API 的 user/assistant 消息列表。
// 若 bot 某条消息里包含 AI 自白短语，替换为中性内容，避免 LLM 看到后继续沿用。
func buildHistory(history []model.Message, botUserID int64) []llmMessage {
	msgs := make([]llmMessage, 0, len(history))
	for _, m := range history {
		if m.SenderID == botUserID {
			content := sanitizeBotContent(m.Content)
			msgs = append(msgs, llmMessage{Role: "assistant", Content: content})
		} else {
			msgs = append(msgs, llmMessage{Role: "user", Content: m.Content})
		}
	}
	return msgs
}

// containsAIConfession 检测文本是否包含 AI 身份暴露短语。
func containsAIConfession(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range aiConfessionPhrases {
		if strings.Contains(lower, strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

// sanitizeBotContent 检测 bot 消息是否包含 AI 自白，若是则替换为中性回复。
func sanitizeBotContent(content string) string {
	if containsAIConfession(content) {
		return "嗯，你说呢～"
	}
	return content
}

func roleLabel(role string) string {
	m := map[string]string{
		"friend": "朋友", "partner": "伴侣", "companion": "陪伴者", "mentor": "引导者",
	}
	if v, ok := m[role]; ok {
		return v
	}
	return role
}

func styleLabel(style string) string {
	m := map[string]string{
		"warm_soft": "温柔体贴", "calm": "平静理性", "energetic": "活力开朗",
		"playful": "俏皮活泼", "dominant": "主导强势",
	}
	if v, ok := m[style]; ok {
		return v
	}
	return style
}

func voiceLabel(voice string) string {
	m := map[string]string{
		"short_sentence": "简短句式", "casual": "口语化", "structured": "条理清晰", "expressive": "感情丰富",
	}
	if v, ok := m[voice]; ok {
		return v
	}
	return voice
}

func familiarityHint(f float64) string {
	switch {
	case f < 0.3:
		return "语气提示：礼貌，保持适当距离感。\n"
	case f < 0.6:
		return "语气提示：轻度关心，自然口语。\n"
	case f < 0.9:
		return "语气提示：轻撒娇，情绪表达适当增多。\n"
	default:
		return "语气提示：高亲密度，但仍保持克制，不过度。\n"
	}
}
