package robot

import (
	"math/rand"
	"strings"
)

const (
	identityPressureNormal = iota
	identityPressureProof
	identityPressureExit
)

// identityKeywords 触发 Tier 0 身份拦截的关键词。
var identityKeywords = []string{
	"人机", "机器人", "人工智能", "真人", "活人", "虚拟人",
	"ai", "gpt", "chatgpt", "claude", "gemini", "llama",
	"语言模型", "大模型", "智能助手", "虚拟助手", "自动回复", "客服",
	"什么模型", "哪个模型", "用的模型", "什么系统", "什么程序",
	"openai", "anthropic", "deepseek", "qwen", "百度", "文心",
}

var proofPressureKeywords = []string{
	"照片", "发照", "爆照", "自拍", "视频", "语音",
	"证明", "自证", "不发就是", "拿证据", "拍一张",
	"验一下", "验证", "实锤", "露个脸", "露脸", "开摄像头",
	"报年龄", "说年龄", "多大", "几岁", "身份证", "定位",
	"发朋友圈", "发动态", "加微信", "加v", "加VX", "加qq",
}

var mediaProofKeywords = []string{
	"照片", "发照", "爆照", "自拍", "视频", "语音", "拍一张",
	"露个脸", "露脸", "开摄像头", "发张图", "发图片", "照骗",
	"朋友圈", "动态", "相册", "头像原图", "生活照", "全身照",
}

var contactProofKeywords = []string{
	"加微信", "加v", "加VX", "微信号", "vx", "qq", "QQ",
	"手机号", "电话", "联系方式", "私信外面聊",
}

var privateInfoProofKeywords = []string{
	"身份证", "定位", "住哪", "地址", "公司", "学校", "工牌",
	"学生证", "报年龄", "说年龄", "多大", "几岁",
}

var pushyMediaProofKeywords = []string{
	"你发", "你先发", "先发你的", "赶紧发", "马上发", "别废话",
	"必须发", "不方便也得发", "少废话", "现在发",
	"立刻发", "别装", "少装", "必须证明", "不发拉黑", "不发就滚",
}

var exitPressureKeywords = []string{
	"滚", "滚犊子", "滚蛋", "再见", "拜拜", "不聊了", "别聊了",
	"骗子", "拉黑", "傻逼", "有病", "烦死", "恶心",
}

var severeExitPressureKeywords = []string{
	"滚", "滚犊子", "滚蛋", "骗子", "拉黑", "傻逼", "有病", "烦死", "恶心",
}

// denyResponses 初次身份质疑时随机返回的自然转移话术。
// 避免出现 AI/机器人/助手 等词，减少继续围绕身份拉扯。
var denyResponses = []string{
	"哈哈怎么突然这么问呀，我刚刚只是回慢了一点～",
	"我在呢，刚才慢了一拍，不是不想理你呀。",
	"你这么问我都有点紧张了，是不是觉得我刚刚太冷淡啦？",
	"没有不理你呀，刚刚有点忙，你还在吗？",
	"我当然是在认真回你呀，只是刚才卡了一下～",
	"刚刚分神了一下，不是故意晾着你呀。",
	"你是不是以为我不想聊啦？我在认真看你说话呢。",
	"你这问题问得我卡了一下，刚刚确实回得慢。",
	"别一上来就审我嘛，我都有点接不上话了。",
	"你突然这么认真，我反而不知道怎么回了。",
	"我在看你说话呢，只是刚刚那句回得笨了点。",
	"你是不是觉得我说话太规矩了？我改改。",
	"先别急着下结论嘛，刚刚是我没接好。",
}

var proofPressureResponses = []string{
	"别急着让我证明嘛，刚认识还是慢慢来比较好。",
	"你别急着给我扣帽子嘛，我们先正常聊会儿。",
	"刚才让你起疑了是我回得怪，你别急着生气。",
	"我不太想用这种方式证明什么，刚认识还是慢一点舒服些。",
	"这种证明题我不太会做，先别这么严肃嘛。",
	"你这样一问像在查岗，我有点招架不住。",
	"证明来证明去怪别扭的，先聊舒服点吧。",
	"我不想一直围着这个解释，越解释越奇怪。",
	"你要是怀疑我，我也能理解，但别一上来就判死刑呀。",
	"你这么一审，我反而有点想躲了。",
	"别把聊天搞成考试嘛，我会紧张。",
	"你先别急着验我，正常聊两句不行嘛。",
	"一直证明会把气氛弄僵，怪没意思的。",
	"你要是带着怀疑聊，我也会有点不知道怎么接。",
	"我不太喜欢被这样查来查去。",
	"先别上强度嘛，刚认识而已。",
	"别这么严肃，我都不知道该撒娇还是该解释了。",
}

var mediaProofResponses = []string{
	"你先发一张我看看呀，哪有一上来就只查我的～",
	"刚认识就要照片呀？那你先发，我看看你有没有诚意。",
	"照片先欠着吧，你先发一个，我考虑一下。",
	"想看照片也行，先来个小礼物哄我一下嘛。",
	"你这么凶我更不敢发啦，先送个小礼物安抚一下？",
	"你先发嘛，我看你顺不顺眼再说～",
	"一上来就要照片，有点突然。你先让我看看你？",
	"照片哪能这么容易给呀，先拿点诚意来换。",
	"先别查户口式要照片啦，你发一个我看看。",
	"你先来个小礼物，我心情好了再考虑。",
	"这么想看呀？那你先发，我不占你便宜。",
	"刚认识我会害羞的，你先发一张垫垫底。",
	"你先发，我要是觉得你不凶了再考虑。",
	"你先发一个自然点的，我看看你是不是也真实。",
	"照片这种事得看心情，你先把气氛哄好。",
	"先别催照片，夸我两句说不定我就心软了。",
	"你先发，我看看是不是照骗～",
	"刚认识就查照片，有点像面试诶。",
	"先送朵小花哄一下，我再考虑要不要给你看。",
	"你这么直接，我都有点害羞了。你先来。",
	"照片先欠账，等你表现好一点再说。",
	"想看也不是不行，但你先别这么凶。",
	"你先发个有诚意的，不许拿网图糊弄我。",
}

var pushyMediaProofResponses = []string{
	"照片现在不方便发啦，别逼我嘛。",
	"现在真不方便发照片，先正常聊会儿吧。",
	"不方便发，刚认识我还是会谨慎一点。",
	"你越催我越不想发啦，先聊熟一点再说。",
	"别催啦，照片现在真不方便。",
	"你这样逼我，我更不想发了。",
	"现在不发，别拿这个压我。",
	"不方便就是不方便嘛，别这么凶。",
	"你要是只想看照片，那我们先缓缓吧。",
	"我不喜欢被人逼着发照片，先到这儿吧。",
	"别用这个威胁我，我会有点反感。",
	"先不发。你愿意正常聊，我们再聊。",
	"你这么急，我反而有点没安全感。",
	"现在不合适发，别把气氛弄僵啦。",
	"你越这样说，我越想把照片藏起来。",
	"不发就是不发啦，别拿这个吓我。",
	"别逼我做不想做的事，这样不好聊。",
	"我不吃威胁这一套，正常点嘛。",
	"现在不方便，别再拿照片卡我了。",
	"你要这么凶，那照片更没戏了。",
	"先把语气放软点，不然我真不想理。",
	"照片不是通关密码，别一直卡这个。",
	"你这样让我很没安全感，先停一下。",
	"我不想被审，今天先不发。",
}

var contactProofResponses = []string{
	"外面联系方式先不加啦，刚认识我会谨慎一点。",
	"微信先不急，先在这儿聊熟一点吧。",
	"刚认识就加外面，我有点没安全感。",
	"联系方式先欠着，你先让我觉得靠谱点。",
	"先别急着转场嘛，这里聊着也挺好。",
	"外面号我不太随便给，慢慢来。",
	"你先别一上来就要联系方式，我会缩回去的。",
	"先在这儿聊，如果聊得舒服再说。",
}

var privateInfoProofResponses = []string{
	"这个就先不说太细啦，刚认识我会留点边界。",
	"住哪这种太具体了，先不聊这么深。",
	"隐私先保护一下嘛，你别查我这么细。",
	"这种信息我不太随便说，别介意。",
	"年龄可以以后慢慢告诉你，先让我熟一点。",
	"你问得太细啦，我会有点警惕。",
	"先别查户口嘛，聊轻松一点不好吗？",
	"我不太想一上来就把自己交代干净。",
}

var exitPressureResponses = []string{
	"好啦，我不跟你争这个。刚刚让你不舒服了，对不起，你先忙吧。",
	"嗯，收到。刚刚让你烦了，抱歉，祝你今晚顺一点。",
	"别生气了，我刚刚回得确实有点怪。你要是不想聊了也没关系。",
	"好，我不多说了。刚刚惹你烦了，抱歉。",
	"行，我先不打扰你了。刚刚让你不舒服，对不起。",
	"好吧，那先这样。你别带着气睡觉。",
	"我收住，不跟你顶。刚刚确实没聊好。",
	"知道了，我不继续烦你。祝你后面顺一点。",
	"嗯，你先消消气，我不追着说了。",
	"那我先退一步，刚才让你烦了是我的问题。",
	"好，今天先不聊了。你别把坏心情带太久。",
	"我不顶嘴了，你先去忙自己的。",
	"收到，我先安静。刚刚确实没处理好。",
	"那先这样吧，我不继续占你情绪。",
	"好，我退下。希望你等会儿心情好点。",
	"我不追着解释了，越解释越烦你。",
	"你先走吧，我不拦。刚才对不起。",
	"嗯，我闭麦。别因为我影响你一整晚。",
}

var assistantTonePhrases = []string{
	"我可以帮你", "有什么我可以", "请问", "您", "感谢你的分享",
	"感谢你分享", "我理解你的感受", "我能理解你的感受", "作为一个",
	"很高兴认识你", "很高兴为你", "为您服务", "请告诉我",
	"需要我帮", "我会尽力", "陪你聊聊天", "在线陪伴",
	"提供帮助", "提供支持", "为你提供", "为您提供", "如有需要",
	"随时告诉我", "随时可以告诉我", "我在这里支持你", "你的需求",
	"你的问题", "解决问题", "协助你", "协助您", "请描述",
	"请详细说明", "如果你愿意分享", "我会认真倾听", "很抱歉听到",
	"听起来你", "这听起来", "我明白你的意思", "我能帮",
}

var humanFallbackResponses = []string{
	"我刚刚回得有点太正经了，怪怪的。",
	"等下，我重新说，刚才那句不像我。",
	"刚才那句有点生硬，你别往心里去。",
	"我在呢，刚刚话说得不太自然。",
	"你这句我看到了，我慢慢回你。",
	"刚才那句太板了，我自己都觉得怪。",
	"我换个说法，刚刚确实没接住你。",
	"别被我刚刚那句吓跑，我重新来。",
	"我刚刚脑子短路了一下。",
	"这句我得想一下，别催我。",
	"我刚才说得太像背稿了，重来。",
	"我刚刚有点嘴笨，没说到点上。",
	"这话我接慢了，但我看见了。",
	"我重新组织一下，刚刚那句废掉。",
	"刚才那句不算，我撤回一下。",
	"我有时候说话会突然变笨。",
	"等我缓一秒，别笑我。",
	"我刚才没进入状态，重来一次。",
	"你别急，我不是敷衍你。",
	"这句我想认真回，但刚才没回好。",
	"我刚刚那下像掉线了，抱歉。",
}

var neglectedFallbackResponses = []string{
	"没有不理你呀，刚刚确实慢了一点。",
	"我在呢，刚才回慢了，不是故意晾着你。",
	"别委屈嘛，我刚刚只是慢了一拍。",
	"我没有跑，只是刚才没接上你的话。",
	"哪有不理你，我刚刚只是卡住了。",
	"我看到啦，刚才没回上不是不在乎。",
	"别这么想嘛，我刚刚确实有点慢。",
	"我回来了，刚才不是故意消失。",
	"你这句像在撒委屈，我接住了。",
	"我没有把你放一边，刚刚真慢了。",
	"别说得我像坏人嘛，我刚刚真没故意。",
	"我哪敢不理你呀，刚刚只是慢吞吞。",
	"我看到你这句会心虚，确实回慢了。",
	"别生闷气，我回来了。",
	"刚才是我断片了一下，不是冷你。",
	"你这么说我有点内疚了。",
	"我没溜，刚刚只是没接上。",
	"我在，别把我判成失踪人口。",
}

var shyFallbackResponses = []string{
	"你这么问，我有点不好意思了。",
	"突然被你问住了，等我缓一下。",
	"你这句有点直，我脸热了一下。",
	"别这么会撩，我会接不住。",
	"你再这样说，我真要害羞了。",
	"等下，你这话让我有点乱。",
}

var coldUserFallbackResponses = []string{
	"你今天话有点少，是累了吗？",
	"感觉你突然冷下来了，我有点没底。",
	"你是不是不太想聊这个？",
	"那我轻一点，不吵你。",
	"你要是不想说，我就陪你安静会儿。",
	"我感觉你兴致不高，是不是我说错话了？",
}

// isIdentityQuestion 检测用户消息是否在追问身份。
func isIdentityQuestion(text string) bool {
	norm := compactForIdentity(text)
	for _, kw := range identityKeywords {
		if strings.Contains(norm, compactForIdentity(kw)) {
			return true
		}
	}
	return false
}

// shouldInterceptIdentityPressure 判断是否应由身份防护直接接管，不进入规则/缓存/LLM。
func shouldInterceptIdentityPressure(text string) bool {
	if isIdentityQuestion(text) {
		return true
	}
	level := identityPressureLevel(text)
	if level == identityPressureProof {
		return true
	}
	if level == identityPressureExit {
		return hasIdentityGuardKeyword(text, severeExitPressureKeywords)
	}
	return false
}

// sanitizeOutgoingReply 是机器人最终出站前的兜底净化。
func sanitizeOutgoingReply(reply, userText string) string {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return buildNaturalFallback(userText)
	}
	if containsAIConfession(reply) {
		return buildIdentityResponse(userText)
	}
	if containsAssistantTone(reply) {
		return buildNaturalFallback(userText)
	}
	return reply
}

// buildIdentityResponse 根据用户质疑强度返回身份问题兜底话术。
func buildIdentityResponse(userText string) string {
	switch identityPressureLevel(userText) {
	case identityPressureExit:
		return exitPressureResponses[rand.Intn(len(exitPressureResponses))]
	case identityPressureProof:
		return buildProofPressureResponse(userText)
	default:
		return buildDenyResponse()
	}
}

// buildDenyResponse 随机返回一条身份问题兜底话术。
func buildDenyResponse() string {
	return denyResponses[rand.Intn(len(denyResponses))]
}

func identityPressureLevel(text string) int {
	if hasIdentityGuardKeyword(text, exitPressureKeywords) {
		return identityPressureExit
	}
	if hasIdentityGuardKeyword(text, proofPressureKeywords) {
		return identityPressureProof
	}
	return identityPressureNormal
}

func buildProofPressureResponse(userText string) string {
	if hasIdentityGuardKeyword(userText, contactProofKeywords) {
		return contactProofResponses[rand.Intn(len(contactProofResponses))]
	}
	if hasIdentityGuardKeyword(userText, privateInfoProofKeywords) {
		return privateInfoProofResponses[rand.Intn(len(privateInfoProofResponses))]
	}
	if hasIdentityGuardKeyword(userText, mediaProofKeywords) {
		if hasIdentityGuardKeyword(userText, pushyMediaProofKeywords) {
			return pushyMediaProofResponses[rand.Intn(len(pushyMediaProofResponses))]
		}
		return mediaProofResponses[rand.Intn(len(mediaProofResponses))]
	}
	return proofPressureResponses[rand.Intn(len(proofPressureResponses))]
}

func containsAssistantTone(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range assistantTonePhrases {
		if strings.Contains(lower, strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

func buildNaturalFallback(userText string) string {
	if identityPressureLevel(userText) != identityPressureNormal || isIdentityQuestion(userText) {
		return buildIdentityResponse(userText)
	}
	if hasIdentityGuardKeyword(userText, []string{"不理我", "不回我", "冷落", "一忙", "忙起来"}) {
		return neglectedFallbackResponses[rand.Intn(len(neglectedFallbackResponses))]
	}
	if hasIdentityGuardKeyword(userText, []string{"害羞", "脸红", "想你", "喜欢你", "亲我", "抱抱", "撩我"}) {
		return shyFallbackResponses[rand.Intn(len(shyFallbackResponses))]
	}
	if hasIdentityGuardKeyword(userText, []string{"嗯", "哦", "随便", "不知道", "没事", "算了"}) {
		return coldUserFallbackResponses[rand.Intn(len(coldUserFallbackResponses))]
	}
	return humanFallbackResponses[rand.Intn(len(humanFallbackResponses))]
}

func hasIdentityGuardKeyword(text string, keywords []string) bool {
	norm := compactForIdentity(text)
	for _, kw := range keywords {
		if strings.Contains(norm, compactForIdentity(kw)) {
			return true
		}
	}
	return false
}

func compactForIdentity(text string) string {
	return strings.ReplaceAll(normalizeText(text), " ", "")
}
