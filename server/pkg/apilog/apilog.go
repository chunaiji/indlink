// Package apilog 外部接口调用日志:异步落库,供管理后台「接口日志」页排查外呼行为。
// 只记对第三方(微信内容安全/订阅消息/逆地理等)的调用,不记 C 端业务请求。
package apilog

import (
	"time"

	"driftbottle/internal/model"

	"gorm.io/gorm"
)

var db *gorm.DB

// Init 注入 DB(main 启动时调用一次);未初始化时 Record 为空操作。
func Init(d *gorm.DB) { db = d }

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// Record 异步写一条外呼日志(截断防爆列;失败静默,日志功能不能影响业务)。
func Record(tenantID int64, kind, detail string, respCode int, respBody string, ok bool) {
	if db == nil {
		return
	}
	go func() {
		_ = db.Create(&model.ApiCallLog{
			TenantID: tenantID, Kind: kind,
			Detail: trunc(detail, 500), RespCode: respCode, RespBody: trunc(respBody, 500),
			OK: ok, CreatedAt: time.Now(),
		}).Error
	}()
}

// CleanupBefore 删除 d 之前的日志(main 启动时清理一次,防表膨胀)。
func CleanupBefore(d time.Duration) {
	if db == nil {
		return
	}
	go func() {
		db.Where("created_at < ?", time.Now().Add(-d)).Delete(&model.ApiCallLog{})
	}()
}
