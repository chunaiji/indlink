package errs

// 业务错误码集中定义。区段:
// 1xxx 通用 / 2xxx 用户鉴权 / 3xxx 漂流瓶 / 4xxx 聊天 / 5xxx 钱包支付
const (
	CodeOK          = 0
	CodeBadRequest  = 1001
	CodeServerError = 1002
	CodeRateLimited = 1003
	CodeForbidden   = 1004
	CodeNotFound    = 1005

	CodeUnauthorized = 2001
	CodeLoginFailed  = 2002
	CodeNeedVerify   = 2003 // 需要真人认证(开关开启时)

	CodeOAuthEmailTaken   = 2004 // 第三方登录:该邮箱已注册,需先登录再绑定
	CodeOAuthAlreadyBound = 2005 // 绑定:该第三方账号已被其他用户绑定
	CodeOAuthLastMethod   = 2006 // 解绑:解绑后将没有任何可登录方式

	CodeBottleExpired = 3001
	CodeContentBlock  = 3002 // 命中敏感词/审核未过
	CodeQuotaExceeded = 3003 // 今日扔/捞次数已用完(无免费额度且无次数包)

	CodeChatBlocked = 4001 // 被拉黑

	CodeInsufficient     = 5001 // 金币余额不足
	CodeOrderNotFound    = 5002
	CodePaySignError     = 5003
	CodeDuplicate        = 5004
	CodeItemInsufficient = 5005 // 道具/礼物库存不足(送礼是库存制,需先购买入库)
)

type BizError struct {
	Code int
	Msg  string
}

func (e *BizError) Error() string { return e.Msg }

func New(code int, msg string) *BizError {
	return &BizError{Code: code, Msg: msg}
}

// 常用预置错误
var (
	ErrInsufficient = New(CodeInsufficient, "金币余额不足")
	ErrNeedVerify   = New(CodeNeedVerify, "请先完成真人认证")
	ErrChatBlocked  = New(CodeChatBlocked, "对方已将你拉黑或你已拉黑对方")
	ErrContentBlock = New(CodeContentBlock, "内容包含违规信息,请修改后再试")
)
