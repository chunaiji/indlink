package mailer

import (
	"fmt"
	"html/template"
	"strings"
)

// ── 验证码邮件的文案 ────────────────────────────────────────────
//
// 放在 mailer 包里,是因为它们是**邮件的呈现**,与登录注册的领域逻辑无关。
// user 只负责「该发一个码了」,长什么样是这里的事。
//
// ⚠️ 一律走 html/template 转义。验证码是自己生成的、邮箱是校验过的,
// 今天注不进东西——但「今天的输入是干净的」不是能长期依赖的前提,
// 而一封邮件里的 HTML 注入可以做成一个看起来来自本平台的钓鱼页。

var codeTpl = template.Must(template.New("code").Parse(`
<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
            max-width:480px;margin:0 auto;padding:32px 24px;color:#16323C">
  <div style="text-align:center;margin-bottom:24px">
    <div style="font-size:34px;line-height:1">🍾</div>
    <div style="font-size:18px;font-weight:800;letter-spacing:5px;margin-top:8px">DRIFT</div>
  </div>
  <p style="font-size:15px;line-height:1.6;margin:0 0 20px">{{.Lead}}</p>
  <div style="font-size:30px;font-weight:700;letter-spacing:6px;
              padding:16px 0;text-align:center;background:#F0F4F6;
              border-radius:12px;margin:0 0 20px;color:#12B5CE">{{.Code}}</div>
  <p style="font-size:13px;line-height:1.6;color:#6b7280;margin:0">
    验证码 {{.Minutes}} 分钟内有效。<br>
    <strong>如果这不是你本人的操作，请忽略这封邮件</strong> ——
    不需要做任何事，你的账号是安全的。
  </p>
  <p style="font-size:12px;color:#9ca3af;margin:24px 0 0;
            border-top:1px solid #e5e7eb;padding-top:16px">
    这封信由 DRIFT 自动发出，请勿直接回复。
  </p>
</div>`))

type codeData struct {
	Lead    string
	Code    string
	Minutes int
}

// CodeMail 按用途返回验证码邮件的标题与正文。
//
// purpose 取 user 包里的 register / reset / login。
// 三封信**刻意用同一个版式、同一种验证码**，而不是给重置发一条链接：
// 链接体验更顺，但手机上跨 App 跳转经常断（邮件 App 点开 → 系统浏览器
// → 与用户原本操作的那个不是同一个 → 会话对不上）。
// 而且一条能直接改密码的链接躺在收件箱里，比一个几分钟就失效的码危险。
func CodeMail(purpose, code string, ttlMinutes int) (subject, body string, err error) {
	var lead string
	switch purpose {
	case "register":
		subject, lead = "验证你的邮箱", "你正在注册 DRIFT 账号。请在注册页面填入下面的验证码："
	case "reset":
		subject, lead = "重置你的登录密码", "你正在重置 DRIFT 的登录密码。请在重置页面填入下面的验证码："
	default:
		subject, lead = "你的登录验证码", "你正在登录 DRIFT。请在登录页面填入下面的验证码："
	}
	body, err = render(codeData{Lead: lead, Code: code, Minutes: ttlMinutes})
	return subject, body, err
}

// TestMail 后台「发送测试邮件」发出的那一封。
//
// 内容刻意写得**像一封真的信**而不是 "test"：管理员要借它判断的不只是
// 「能不能发出去」，还有发件人显示名对不对、中文标题会不会乱码、会不会进垃圾箱——
// 一封写着 "test" 的信在垃圾箱判定上与真实邮件表现并不一样。
func TestMail() (subject, body string, err error) {
	body, err = render(codeData{
		Lead: "这是一封来自 DRIFT 的测试邮件。你能读到它，说明 SMTP 配置可用：" +
			"发件人、标题编码与投递都正常。下面是一个示例验证码，它不对应任何操作。",
		Code:    "000000",
		Minutes: 5,
	})
	return "SMTP 配置测试", body, err
}

func render(d codeData) (string, error) {
	var sb strings.Builder
	if err := codeTpl.Execute(&sb, d); err != nil {
		return "", fmt.Errorf("%w: 渲染邮件正文失败: %v", ErrSendFailed, err)
	}
	return sb.String(), nil
}
