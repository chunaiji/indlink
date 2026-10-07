package robot

import (
	"math/rand"
	"strings"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/idgen"
)

// 机器人资料按语言生成。
//
// 语言字段的取值必须与 App 资料页的语言选项一致(自称 endonym,见 app `Catalog.languages`):
// "中文" / "English"。发现页按 FIND_IN_SET 匹配「共同语言」,机器人填 "zh"/"en" 这种代码
// 就永远匹配不上真人,等于白建。兴趣同理用 App 的 key(music/travel/...),显示名由客户端翻译。
const (
	LangZh = "中文"
	LangEn = "English"
)

// GenOptions 生成一个机器人的可选约束;零值 = 全随机、中文。
type GenOptions struct {
	Lang   string // LangZh / LangEn;也接受 "zh"/"en",空 = 中文
	Gender int8   // 0 = 随机;1 男 2 女
	City   string // 空 = 按语言从城市池随机
}

// NormalizeLang 把后台/配置里的 zh/en/中文/English 统一成 LangZh/LangEn。
func NormalizeLang(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "en", "english", "en-us", "en_us":
		return LangEn
	default:
		return LangZh
	}
}

// RobotLang 本租户机器人默认语言(后台「机器人」分组 robot_language,默认 zh)。
func RobotLang(tenantID int64) string {
	return NormalizeLang(sysconfig.GetString(tenantID, sysconfig.KeyRobotLanguage))
}

// ---- 中文资料池 ----
var nickPrefix = []string{"夏天的", "海边的", "深夜", "云朵", "柠檬", "晚风", "听海", "旧时光", "星河", "温柔", "麦田", "拾光", "南巷", "北屿", "暖阳"}
var nickSuffix = []string{"风", "猫", "鲸", "信", "光", "屿", "river", "森林", "汽水", "candy", "鹿", "晚安", "贝壳", "潮汐"}
var cities = []string{"北京", "上海", "广州", "深圳", "杭州", "成都", "武汉", "南京", "西安", "重庆", "苏州", "长沙", "厦门", "青岛"}
var zhBios = []string{
	"喜欢在海边发呆,也喜欢听别人的故事。",
	"夜猫子一枚,凌晨的城市最安静。",
	"相信每一个漂来的瓶子都有它的缘分。",
	"爱拍照、爱走路,偶尔也爱写几句没头没尾的话。",
	"人生苦短,想多遇见几个有意思的人。",
	"白天上班,晚上看书,周末去看海。",
	"不太会聊天,但很会听。",
	"想找个人一起分享今天的日落。",
}

// ---- 英文资料池 ----
var enFirstNamesM = []string{"Jack", "Liam", "Noah", "Ethan", "Lucas", "Mason", "Logan", "Oliver", "Aiden", "Daniel", "Leo", "Ryan", "Owen", "Caleb", "Nathan"}
var enFirstNamesF = []string{"Emma", "Olivia", "Ava", "Mia", "Sophia", "Isabella", "Chloe", "Lily", "Zoe", "Grace", "Ella", "Aria", "Nora", "Hannah", "Luna"}
var enCities = []string{"New York", "London", "Los Angeles", "Toronto", "Sydney", "Chicago", "Manchester", "Vancouver", "Melbourne", "Dublin", "San Francisco", "Austin", "Singapore", "Mumbai", "Bangalore"}
var enBios = []string{
	"Night owl. The city is quietest at 2am.",
	"Sunsets, long walks and stories from strangers.",
	"I believe every bottle finds the person it was meant for.",
	"Coffee first, questions later.",
	"Collecting small moments, one at a time.",
	"Here to meet interesting people, not to scroll.",
	"Not great at small talk, great at listening.",
	"Looking for someone to share today's sunset with.",
}

// interestKeys 与 App `Catalog.interests` 的 key 一致。
var interestKeys = []string{"music", "movie", "travel", "cricket", "food", "art", "photo", "reading", "fitness", "gaming"}

// GenerateRobotUser 按语言生成一个未落库的机器人 User(含昵称/城市/语言/兴趣/简介/头像)。
//
// 注册时间往前打散 30 天,免得一批机器人全是「今天注册」。
func GenerateRobotUser(tenantID int64, opt GenOptions) model.User {
	lang := NormalizeLang(opt.Lang)
	gender := opt.Gender
	if gender != 1 && gender != 2 {
		gender = int8(rand.Intn(2) + 1)
	}
	city := strings.TrimSpace(opt.City)
	var nick, bio, language string
	switch lang {
	case LangEn:
		nick = enNickname(gender)
		if city == "" {
			city = enCities[rand.Intn(len(enCities))]
		}
		bio = enBios[rand.Intn(len(enBios))]
		language = LangEn
	default:
		nick = nickPrefix[rand.Intn(len(nickPrefix))] + nickSuffix[rand.Intn(len(nickSuffix))]
		if city == "" {
			city = cities[rand.Intn(len(cities))]
		}
		bio = zhBios[rand.Intn(len(zhBios))]
		language = LangZh
		// 三成中文机器人也会英语,给跨语言用户一点可匹配的人。
		if rand.Intn(10) < 3 {
			language = LangZh + "," + LangEn
		}
	}
	now := time.Now()
	return model.User{
		UserID:       idgen.Next(),
		TenantID:     tenantID,
		Nickname:     nick,
		Avatar:       randomAvatarByGender(gender),
		Bio:          bio,
		Gender:       gender,
		Age:          18 + rand.Intn(18),
		City:         city,
		Language:     language,
		Interests:    randomInterests(),
		IsVerified:   true,
		IsRobot:      true,
		Status:       "active",
		CreatedAt:    now.Add(-time.Duration(rand.Intn(720)) * time.Hour),
		LastActiveAt: now,
	}
}

// enNickname 英文昵称:多数直接用名字,少数带姓氏首字母或数字后缀,看起来像真实账号。
func enNickname(gender int8) string {
	pool := enFirstNamesF
	if gender == 1 {
		pool = enFirstNamesM
	}
	name := pool[rand.Intn(len(pool))]
	switch rand.Intn(10) {
	case 0, 1:
		return name + " " + string(rune('A'+rand.Intn(26))) + "."
	case 2:
		return strings.ToLower(name) + string(rune('0'+rand.Intn(10))) + string(rune('0'+rand.Intn(10)))
	default:
		return name
	}
}

// randomInterests 随机 2~4 个兴趣 key,逗号分隔。
func randomInterests() string {
	n := 2 + rand.Intn(3)
	perm := rand.Perm(len(interestKeys))
	picked := make([]string, 0, n)
	for _, i := range perm[:n] {
		picked = append(picked, interestKeys[i])
	}
	return strings.Join(picked, ",")
}

// IsEnglish 机器人的语言字段是否以英语为主(含 English 且不含中文)。
// 用来决定 LLM 回复语言:中英双语的机器人仍按中文回,跟它的内容池一致。
func IsEnglish(language string) bool {
	return strings.Contains(language, LangEn) && !strings.Contains(language, LangZh)
}
