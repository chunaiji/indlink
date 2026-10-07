package pay

import (
	"errors"
	"log"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// orderScope 单笔查单的查询条件。
//
// 抽成纯函数是为了能在没有数据库基建的情况下断言 user_id 一定在 WHERE 里
// (仓库里 27 个 _test.go 中 gorm.Open 出现 0 次,没有 DB 测试基建)。
func orderScope(userID int64, orderNo string) (string, []interface{}) {
	return "order_no = ? AND user_id = ?", []interface{}{orderNo, userID}
}

// Order 按单号查单笔订单。
//
// ⚠️ user_id 必须进 WHERE:只按 order_no 查,这就成了一个能遍历他人订单的接口。
// 订单号能不能被猜到,不该成为安全前提。
//
// 「不存在」与「不属于你」返回同一个错误:区分开来等于提供一个订单号存在性探测接口。
func (s *Service) Order(userID int64, orderNo string) (*model.PayOrder, error) {
	where, args := orderScope(userID, orderNo)
	var order model.PayOrder
	if err := s.db.Where(where, args...).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeOrderNotFound, "订单不存在")
		}
		return nil, err
	}
	return &order, nil
}

// needsSync pending 且下单超过 5 秒才向渠道问:5 秒内用户多半还在收银台里,问了也是「未支付」。
func needsSync(o *model.PayOrder, now time.Time) bool {
	return o.Status == "pending" && now.Sub(o.CreatedAt) > 5*time.Second
}

// takeSyncSlot 同一订单 10 秒内只放行一次。
func (s *Service) takeSyncSlot(orderNo string, now time.Time) bool {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	if s.lastSync == nil {
		s.lastSync = map[string]time.Time{}
	}
	if last, ok := s.lastSync[orderNo]; ok && now.Sub(last) < 10*time.Second {
		return false
	}
	s.lastSync[orderNo] = now
	if len(s.lastSync) > 10000 { // 顺手清老条目,别让 map 一直长
		for k, t := range s.lastSync {
			if now.Sub(t) > time.Minute {
				delete(s.lastSync, k)
			}
		}
	}
	return true
}

// SyncIfStale 回调丢了的补偿:向渠道查一次,支付了就走**唯一的入账口** HandleCallback,再把最新订单读回来。
// 任何错误只打日志,不影响查单响应——客户端会继续轮询。
func (s *Service) SyncIfStale(order *model.PayOrder) *model.PayOrder {
	now := time.Now()
	if order == nil || !needsSync(order, now) || !s.takeSyncSlot(order.OrderNo, now) {
		return order
	}
	d, err := s.driverFor(order.TenantID, canonicalChannel(order.Platform))
	if err != nil {
		return order
	}
	q, ok := d.(Querier)
	if !ok {
		return order
	}
	res, err := q.Query(order)
	if err != nil {
		log.Printf("[pay] 主动查单失败 order=%s: %v", order.OrderNo, err)
		return order
	}
	if res == nil || !res.Paid {
		return order
	}
	if err := s.HandleCallback(order.Platform, res); err != nil {
		log.Printf("[pay] 主动查单入账失败 order=%s: %v", order.OrderNo, err)
		return order
	}
	fresh, err := s.Order(order.UserID, order.OrderNo)
	if err != nil {
		return order
	}
	return fresh
}
