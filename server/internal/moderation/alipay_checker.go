package moderation

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/pkg/apilog"
	"driftbottle/pkg/idgen"
)

// alipayChecker 支付宝内容安全(alipay.security.risk.content.sync.detect)。
//
// 接口形状照 CH.Super.Project 里那份生产实现(AliPayApplication.ContentDetect)抄来:
// 文本与图片同一个接口,靠 content_type 区分;凭据就是支付配置里那把支付宝应用私钥,
// 不需要第二套密钥。
//
// ⚠️ 两个前提得在支付宝侧确认,代码里解决不了:
//  1. 参考实现的 channel 是 "tinyapp-eco-open"(小程序生态渠道)。移动应用能否开通该服务,
//     要问支付宝;开不了的话这家服务商对 App 租户就是摆设。
//  2. 入参要 open_id(支付宝用户标识)。用微信 / 手机号登录的用户没有,这类用户直接跳过
//     在线检测——与微信那条「非微信用户跳过」同一个处理。
type alipayChecker struct{ s *Service }

const (
	alipayDetectMethod = "alipay.security.risk.content.sync.detect"
	// 默认检测项:敏感 / 色情 / 违禁 / 暴恐 / 谩骂。后台可改。
	alipayDefaultProducts = "TJ_POLITICS_MC,TJ_PORN_MC,TJ_ILLEGAL_MC,TJ_TERRORISM_MC,TJ_ABUSES_MC"
	alipayDefaultChannel  = "tinyapp-eco-open"
)

// alipayVerdict 解析 suggestion。
//
// pass 放行;review 是「交人工复核」,也放行——把不确定的判定直接拦下来,
// 代价是正常用户发不出东西,这比漏掉一条更伤。其余一律拦。
// suggestion 为空说明响应不是我们认识的形状,报错交调用方按「故障放行」处理。
func alipayVerdict(node []byte) (bool, error) {
	var r struct {
		Suggestion string `json:"suggestion"`
		Labels     []struct {
			Label string `json:"label"`
			Rate  string `json:"rate"`
		} `json:"detect_check_labels"`
	}
	if err := json.Unmarshal(node, &r); err != nil {
		return false, err
	}
	switch r.Suggestion {
	case "":
		return false, errors.New("支付宝未返回 suggestion")
	case "pass", "review":
		return false, nil
	default:
		return true, nil
	}
}

// alipayDetectBiz 组装 biz_content。data_list 是数组(支付宝支持一次送检多条)。
func alipayDetectBiz(fields map[string]string, items []string, contentType, openID, requestID string) map[string]string {
	products := fields["products"]
	if products == "" {
		products = alipayDefaultProducts
	}
	channel := fields["channel"]
	if channel == "" {
		channel = alipayDefaultChannel
	}
	list, _ := json.Marshal(items)
	return map[string]string{
		"data_list":    string(list),
		"content_type": contentType,
		"open_id":      openID,
		"channel":      channel,
		"request_id":   requestID,
		"products":     products,
		"tenants":      fields["tenants"],
	}
}

// alipayOpenID 取用户的支付宝标识;微信 / 手机号登录的用户为空。
func (s *Service) alipayOpenID(userID int64) string {
	var u model.User
	if err := s.db.Select("alipay_uid").First(&u, "user_id = ?", userID).Error; err != nil {
		return ""
	}
	return u.AlipayUID
}

// detect 送一次检。返回 (是否违规, 错误)。
func (a *alipayChecker) detect(tenantID int64, fields map[string]string, items []string, contentType, openID, kind string) (bool, error) {
	if a.s.alipayClient == nil {
		return false, errors.New("未注入支付宝客户端")
	}
	cli, err := a.s.alipayClient(tenantID)
	if err != nil || cli == nil {
		return false, errors.New("未配置支付宝应用(到服务商 → 支付页配置)")
	}
	reqID := strconv.FormatInt(idgen.Next(), 10)
	biz := alipayDetectBiz(fields, items, contentType, openID, reqID)
	detail := fmt.Sprintf("%s len=%d req=%s", contentType, len(items), reqID)
	node, err := cli.Execute(alipayDetectMethod, biz, nil)
	if err != nil {
		apilog.Record(tenantID, "alipay_content_"+kind, detail, 0, err.Error(), false)
		return false, err
	}
	blocked, err := alipayVerdict(node)
	apilog.Record(tenantID, "alipay_content_"+kind, detail, 0, string(node), err == nil)
	return blocked, err
}

// fieldsOf 当前生效的支付宝审核行字段(取不到就给空表,detect 里会用内置默认)。
func (a *alipayChecker) fieldsOf(tenantID int64) map[string]string {
	if _, row, ok := a.s.checkerFor(tenantID); ok && row != nil {
		return row.Fields
	}
	return map[string]string{}
}

// Probe 真的送检一句固定文案,错误原样抛出(与 CheckText 的「故障放行」相反)。
// 探活没有真实用户,open_id 用配置里的 tenants 兜一下——拿不到正确结果不要紧,
// 要的是「签名、网关、服务开通」这三件事能不能跑通。
func (a *alipayChecker) Probe(tenantID int64) error {
	fields := a.fieldsOf(tenantID)
	_, err := a.detect(tenantID, fields, []string{"今天天气不错"}, "TEXT", fields["tenants"], "probe")
	return err
}

func (a *alipayChecker) CheckText(tenantID, userID int64, scene int, text string) error {
	openID := a.s.alipayOpenID(userID)
	if openID == "" {
		return nil // 非支付宝用户:跳过在线检测,本地词库已兜底
	}
	blocked, err := a.detect(tenantID, a.fieldsOf(tenantID), []string{text}, "TEXT", openID, "text")
	if err != nil {
		log.Printf("[seccheck] alipay text err tenant=%d: %v", tenantID, err)
		return nil // API 故障放行,本地词库已兜底(与微信同策略)
	}
	if blocked {
		return errs.ErrContentBlock
	}
	return nil
}

// CheckImageAsync 支付宝是同步接口;在 goroutine 里跑,违规直接删文件。
func (a *alipayChecker) CheckImageAsync(tenantID, userID int64, mediaURL string) {
	openID := a.s.alipayOpenID(userID)
	if openID == "" {
		return
	}
	fields := a.fieldsOf(tenantID)
	go func() {
		blocked, err := a.detect(tenantID, fields, []string{mediaURL}, "PICTURE", openID, "image")
		if err != nil {
			log.Printf("[seccheck] alipay image err tenant=%d: %v", tenantID, err)
			return
		}
		if blocked {
			a.s.removeUploadedFile(mediaURL)
			log.Printf("[seccheck] alipay image rejected tenant=%d user=%d url=%s", tenantID, userID, mediaURL)
		}
	}()
}
