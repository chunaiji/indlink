package user

import (
	"context"
	"fmt"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/pkg/cache"

	"gorm.io/gorm"
)

// 账号删除。
//
// App Store 5.1.1(v) 要求 App 内提供**真实删除**而非停用。实现为
// 「匿名化 + 释放标识」，在单事务内完成：
//
//   - 清空 PII，手机号/第三方标识置空 → 同一手机号可以重新注册
//   - 瓶子 status=deleted、动态 visible=deleted → 不再出现在任何 feed
//   - **保留**钱包流水与支付订单：财务对账与审计需要，且它们此刻已与自然人解绑
//
// 会话不删：对方看到的是「已注销用户」且会话只读（原型 D1 画了这个状态），
// 直接删会让对方的聊天记录凭空消失。

func revokedKey(userID int64) string { return fmt.Sprintf("revoked:%d", userID) }

// DeleteAccount 注销账号。
func (s *Service) DeleteAccount(userID int64) error {
	var u model.User
	if err := s.db.First(&u, "user_id = ?", userID).Error; err != nil {
		return errs.New(errs.CodeNotFound, "用户不存在")
	}
	if u.Status == "deleted" {
		return nil // 幂等
	}

	now := time.Now()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
			"status":     "deleted",
			"deleted_at": now,
			// PII 清空；标识置空即释放，可被重新注册
			"nickname":   "已注销用户",
			"avatar":     "",
			"bio":        "",
			"phone":      "",
			"email":      "",
			// 置 NULL 而非 '':这两列带唯一索引(uk_tenant_google / uk_tenant_apple),
			// 写 '' 会导致同租户下第二个账号注销时撞唯一键而失败。
			"google_sub": nil,
			"apple_sub":  nil,
			"wx_openid":  "",
			"alipay_uid": "",
			"union_id":   "",
			"language":   "",
			"interests":  "",
			"lat":        0,
			"lng":        0,
			"tags":       "",
		}).Error; err != nil {
			return err
		}
		// 瓶子下架（bottle 的查询按 status 过滤）
		if err := tx.Model(&model.Bottle{}).
			Where("user_id = ?", userID).
			Update("status", "deleted").Error; err != nil {
			return err
		}
		// 动态下架（feed 只查 visible='public'）
		if err := tx.Model(&model.Moment{}).
			Where("user_id = ?", userID).
			Update("visible", "deleted").Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 旧 JWT 立即失效：写吊销标记，TTL 覆盖到 token 自然过期为止。
	ttl := time.Duration(s.cfg.JWTExpireHours) * time.Hour
	if ttl <= 0 {
		ttl = 72 * time.Hour
	}
	cache.RDB.Set(context.Background(), revokedKey(userID), "1", ttl)
	return nil
}

// IsRevoked 供鉴权中间件判断 token 是否已被吊销。
//
// 只查 Redis（单次往返，约 0.1ms），不查库——鉴权在每个请求上都会跑，
// 这里多一次 DB 查询会拖垮整个服务。
func (s *Service) IsRevoked(userID int64) bool {
	if userID == 0 || cache.RDB == nil {
		return false
	}
	n, err := cache.RDB.Exists(context.Background(), revokedKey(userID)).Result()
	return err == nil && n > 0
}
