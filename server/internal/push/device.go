package push

import (
	"errors"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// App 推送设备令牌登记（FCM / APNs）。
//
// 微信订阅消息那一套（模板 ID + PushSubscription.OpenID + access_token 同源）
// 在 App 端完全不可用，这是另起的一条链路。本文件只负责**登记设备**，
// 真正的下发（FCM HTTP v1 / APNs）待接入，见 §开发进度。

// RegisterDevice 登记/更新设备令牌。
//
// token 全局唯一：同一台设备换账号登录时把绑定**转移**给新用户，
// 否则退出登录的人还会继续收到新用户的消息通知——这是最典型的串号事故。
func (s *Service) RegisterDevice(tenantID, userID int64, platform, provider, token string) error {
	if token == "" {
		return errs.New(errs.CodeBadRequest, "设备令牌为空")
	}
	if provider == "" {
		// 未显式指定时按系统推断：iOS 走 APNs，其余走 FCM。
		if platform == "ios" {
			provider = "apns"
		} else {
			provider = "fcm"
		}
	}

	var row model.DeviceToken
	err := s.db.First(&row, "token = ?", token).Error
	if err == nil {
		return s.db.Model(&model.DeviceToken{}).
			Where("token = ?", token).
			Updates(map[string]interface{}{
				"tenant_id":  tenantID,
				"user_id":    userID,
				"platform":   platform,
				"provider":   provider,
				"status":     "active",
				"updated_at": time.Now(),
			}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Create(&model.DeviceToken{
		ID:        idgen.Next(),
		TenantID:  tenantID,
		UserID:    userID,
		Platform:  platform,
		Provider:  provider,
		Token:     token,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}).Error
}

// UnregisterDevice 退出登录时解绑（客户端主动调用；不调用也会在下次登录时被转移）。
func (s *Service) UnregisterDevice(userID int64, token string) error {
	return s.db.Model(&model.DeviceToken{}).
		Where("token = ? AND user_id = ?", token, userID).
		Updates(map[string]interface{}{"status": "revoked", "updated_at": time.Now()}).Error
}

// ActiveDevices 取某用户的有效设备（下发推送时用）。
func (s *Service) ActiveDevices(tenantID, userID int64) ([]model.DeviceToken, error) {
	var list []model.DeviceToken
	err := s.db.Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, "active").
		Find(&list).Error
	return list, err
}
