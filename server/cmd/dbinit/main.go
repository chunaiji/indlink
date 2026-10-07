// 一次性:确保开发库存在 + 校验 MySQL/Redis 连通。凭证来自 .env(不硬编码)。
package main

import (
	"context"
	"log"
	"strings"
	"time"

	"driftbottle/internal/config"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	// 从 .env 的 DSN 派生「无库 DSN」与库名,不硬编码任何凭证。
	// DSN 形如 user:pass@tcp(host:port)/dbname?params
	dsn := cfg.MySQLDSN
	slash := strings.Index(dsn, ")/")
	q := strings.Index(dsn, "?")
	if slash < 0 || q < 0 || q < slash {
		log.Fatalf("MYSQL_DSN 格式无法解析: %s", dsn)
	}
	dbName := dsn[slash+2 : q]
	noDBDSN := dsn[:slash+2] + dsn[q:]

	db, err := gorm.Open(mysql.Open(noDBDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("MySQL 连接失败: %v", err)
	}
	// 确保库存在(非破坏性)。需要重建时手动 DROP。
	if err := db.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
		log.Fatalf("建库失败: %v", err)
	}
	log.Printf("MySQL OK,数据库 %s 就绪", dbName)

	// 校验目标库可连
	d2, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接 %s 失败: %v", dbName, err)
	}
	sqlDB, _ := d2.DB()
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("ping %s 失败: %v", dbName, err)
	}
	log.Printf("%s 可连", dbName)

	// 校验 Redis
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			log.Fatalf("Redis URL 解析失败: %v", err)
		}
		rdb := redis.NewClient(opt)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Fatalf("Redis ping 失败: %v", err)
		}
		log.Printf("Redis OK (db=%d)", opt.DB)
	}

	log.Println("=== 开发环境连通性校验全部通过 ===")
}
