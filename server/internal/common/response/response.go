package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"driftbottle/internal/common/i18n"
)

// 统一响应结构:{ code, msg, data }
// code == 0 表示成功,其余为业务错误码。
type Body struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Body{Code: 0, Msg: "ok", Data: data})
}

// Fail 返回业务错误(HTTP 200,前端按 code 判断)。
//
// msg 传中文原文;按请求语言在此处统一翻译 —— 97 处调用点因此一行不改。
// 未挂语言中间件的路由(全部 C 端路由)恒取 ZhCN,原样下发。
func Fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Body{Code: code, Msg: i18n.T(i18n.FromContext(c), msg)})
}

// FailErr 返回「<前缀>: <底层错误>」形式的业务错误。
//
// 前缀按请求语言翻译,错误详情原样附上(多为 driver / GORM 的英文原文)。
//
// 单独开这个函数而不是在调用点写 Fail(c, code, "更新失败:"+err.Error()):
// 拼出来的整串永远命中不了消息表,会**静默**漏翻。
func FailErr(c *gin.Context, code int, zhPrefix string, err error) {
	msg := i18n.T(i18n.FromContext(c), zhPrefix)
	if err != nil {
		msg += ": " + err.Error()
	}
	c.JSON(http.StatusOK, Body{Code: code, Msg: msg})
}

// Abort 用于中间件中断(带 HTTP 状态码)。
func Abort(c *gin.Context, status, code int, msg string) {
	c.AbortWithStatusJSON(status, Body{Code: code, Msg: i18n.T(i18n.FromContext(c), msg)})
}
