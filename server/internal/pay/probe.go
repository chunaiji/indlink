package pay

import (
	"errors"
	"strconv"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/provider"
)

// Probe 后台「测试连通」。微信 / 支付宝查一个不存在的单号:签名、证书、网关任一错都会在这里暴露;
// 查不到单(Paid=false, err=nil)就是通。Apple 只验私钥能签出 JWT;Play 验服务账号能换到 token。
func (s *Service) Probe(tenantID int64, prov string) (string, error) {
	switch prov {
	case "wechat", "alipay":
		d, err := s.buildDriver(tenantID, prov)
		if err != nil {
			return "", err
		}
		q, ok := d.(Querier)
		if !ok {
			return "", errors.New("该渠道不支持查单")
		}
		if _, err := q.Query(&model.PayOrder{OrderNo: "PROBE" + strconv.FormatInt(time.Now().Unix(), 10)}); err != nil {
			return "", err
		}
		return "签名与网关正常", nil
	case "apple":
		row, ok := s.iapConfig(tenantID)
		if !ok {
			return "", errors.New("未启用")
		}
		if _, err := s.appStoreToken(tenantID, row.Get("bundle_id")); err != nil {
			return "", err
		}
		return ".p8 私钥可用", nil
	case "google_play":
		if s.providers == nil {
			return "", errors.New("未配置")
		}
		row, ok := s.providers.Get(tenantID, provider.KindPay, "google_play")
		if !ok {
			return "", errors.New("未配置")
		}
		if _, err := playToken(row.Get("service_account_json"), time.Now(), ""); err != nil {
			return "", err
		}
		return "服务账号可用", nil
	}
	return "", errors.New("未知渠道")
}
