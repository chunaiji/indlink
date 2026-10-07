// Package i18n 提供请求语言的解析与文案翻译。
//
// 单独开包而不是塞进 middleware:response 需要读语言,若读取函数在 middleware 里
// 就是 response → middleware,而 middleware 的中断逻辑将来可能要用 response,构成环。
// 本包只依赖 gin,谁都能引。
package i18n

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// Lang 是支持的语言标签。
type Lang string

const (
	ZhCN Lang = "zh-CN"
	En   Lang = "en"
)

// ctxKey 是语言在 gin.Context 中的键。
const ctxKey = "i18n_lang"

// Parse 解析 Accept-Language,取首个语言标签。
//
// 有意不实现 RFC 7231 的 q 权重排序:后台发 "zh-CN"/"en",
// App 发 Locale.toLanguageTag(),都是单个干净标签。
// 为一个不存在的输入形态写排序逻辑是负债。
func Parse(header string) Lang {
	tag := header
	if i := strings.IndexAny(tag, ",;"); i >= 0 {
		tag = tag[:i]
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(tag)), "en") {
		return En
	}
	return ZhCN
}

// Middleware 把请求语言写入 context。
//
// ⚠️ 只挂 /admin/api 路由组。C 端不挂 —— wx.request 在部分机型会带
// Accept-Language: en,全局挂载会让小程序中文用户突然收到英文报错。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ctxKey, Parse(c.GetHeader("Accept-Language")))
		c.Next()
	}
}

// FromContext 取请求语言。没有中间件写入时返回 ZhCN —— C 端路由走这条。
func FromContext(c *gin.Context) Lang {
	if c == nil {
		return ZhCN
	}
	if v, ok := c.Get(ctxKey); ok {
		if l, ok := v.(Lang); ok {
			return l
		}
	}
	return ZhCN
}
