// 一次性:为多租户隔离测试插入两个租户 + 两条微信凭证(appA/appB)。用完可删。
package main

import (
	"log"
	"time"

	"driftbottle/internal/config"
	"driftbottle/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	tenants := []model.Tenant{
		{TenantID: 100, Name: "测试租户A", Status: "active", CreatedAt: time.Now()},
		{TenantID: 200, Name: "测试租户B", Status: "active", CreatedAt: time.Now()},
	}
	for _, t := range tenants {
		db.Where(model.Tenant{TenantID: t.TenantID}).FirstOrCreate(&t)
	}
	creds := []model.AppCredential{
		{ID: 1001, TenantID: 100, Platform: "wx", AppID: "appA", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: 1002, TenantID: 200, Platform: "wx", AppID: "appB", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	for _, c := range creds {
		db.Where(model.AppCredential{AppID: c.AppID, Platform: c.Platform}).FirstOrCreate(&c)
	}
	log.Println("多租户测试种子就绪:tenant 100(appA) / 200(appB)")
}
