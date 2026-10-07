package robot

import (
	"log"
	"time"

	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// Start 启动后台调度:每 intervalMin 分钟为每个租户跑一次 Tick。
// 总开关在 sysconfig(robot_enabled),关闭时 Tick 内部直接返回,调度器空转无副作用。
func Start(svc *Service, db *gorm.DB, defaultTenant int64, multiTenant bool) {
	const intervalMin = 1.0
	tenants := func() []int64 {
		if multiTenant {
			var ids []int64
			db.Model(&model.Tenant{}).Pluck("tenant_id", &ids)
			if len(ids) > 0 {
				return ids
			}
		}
		return []int64{defaultTenant}
	}
	runOnce := func() {
		for _, tid := range tenants() {
			func(t int64) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[robot] tenant=%d tick panic: %v", t, r)
					}
				}()
				svc.Tick(t, intervalMin)
			}(tid)
		}
	}
	go func() {
		time.Sleep(10 * time.Second) // 启动后稍等,首跑一次
		// 启动时按性别重刷一次头像（幂等,仅修正历史混用）
		for _, tid := range tenants() {
			svc.ReassignAvatarsByGender(tid)
		}
		runOnce()
		t := time.NewTicker(time.Duration(intervalMin*60) * time.Second)
		defer t.Stop()
		for range t.C {
			runOnce()
		}
	}()
	log.Printf("[robot] 调度器已启动(每 %.0f 分钟一次;总开关 robot_enabled)", intervalMin)
}
