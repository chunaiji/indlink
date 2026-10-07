package bootstrap

import (
	"log"
	"time"

	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// Migrate 建表(开发期用 AutoMigrate;生产可改用 migrations/ 下的 SQL)。
func Migrate(db *gorm.DB) error {
	// 先清理可能存在的重复会话，再 AutoMigrate(后者会给 chats 加唯一索引 uk_chat_pair)。
	if err := dedupeChats(db); err != nil {
		return err
	}
	if err := migrateConfigTenant(db); err != nil {
		return err
	}
	// 同理:AutoMigrate 会给 users 加 uk_tenant_google / uk_tenant_apple 唯一索引,
	// 历史遗留的空串必须先刷成 NULL,否则建索引失败。
	if err := nullifyEmptyOAuthSubs(db); err != nil {
		return err
	}
	return db.AutoMigrate(model.AllModels()...)
}

// nullifyEmptyOAuthSubs 把历史遗留的空串 google_sub / apple_sub 刷成 NULL。
//
// 与 dedupeChats 同理：AutoMigrate 会给这两列加唯一索引
// (uk_tenant_google / uk_tenant_apple)，而 MySQL 唯一索引不允许重复的 ''。
// 手机号注册的用户这两列都是 ''，不先清理，建索引会直接失败。
// 幂等；全新库(users 表尚不存在)或列尚未创建时直接跳过。
func nullifyEmptyOAuthSubs(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.User{}) {
		return nil
	}
	for _, col := range []string{"google_sub", "apple_sub"} {
		if !db.Migrator().HasColumn(&model.User{}, col) {
			continue
		}
		res := db.Exec("UPDATE users SET " + col + " = NULL WHERE " + col + " = ''")
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			log.Printf("[migrate] %s 空串置 NULL: %d 行", col, res.RowsAffected)
		}
	}
	return nil
}

// dedupeChats 合并同一 (tenant_id, user_a, user_b) 的重复会话：保留最小 chat_id，
// 把其余会话的消息重指到保留会话后删除之。历史无唯一约束时可能因并发产生重复，
// 不先清理会导致 AutoMigrate 创建唯一索引失败。全新库(chats 表尚不存在)直接跳过。
func dedupeChats(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Chat{}) {
		return nil
	}
	type group struct {
		TenantID int64
		UserA    int64
		UserB    int64
	}
	var dups []group
	if err := db.Model(&model.Chat{}).
		Select("tenant_id, user_a, user_b").
		Group("tenant_id, user_a, user_b").
		Having("COUNT(*) > 1").
		Scan(&dups).Error; err != nil {
		return err
	}
	for _, d := range dups {
		var ids []int64
		if err := db.Model(&model.Chat{}).
			Where("tenant_id = ? AND user_a = ? AND user_b = ?", d.TenantID, d.UserA, d.UserB).
			Order("chat_id asc").Pluck("chat_id", &ids).Error; err != nil {
			return err
		}
		if len(ids) < 2 {
			continue
		}
		keep, rest := ids[0], ids[1:]
		// 仅重指 message.chat_id；保留会话的 last_message/updated_at 可能短暂偏旧，
		// 下次 SendMessage 即自愈，无需在此回填。
		if err := db.Model(&model.Message{}).Where("chat_id IN ?", rest).
			Update("chat_id", keep).Error; err != nil {
			return err
		}
		if err := db.Where("chat_id IN ?", rest).Delete(&model.Chat{}).Error; err != nil {
			return err
		}
		log.Printf("[migrate] 合并重复会话 keep=%d merged=%v", keep, rest)
	}
	return nil
}

// migrateConfigTenant 把旧的 configs(主键 key) 升级为复合主键 (tenant_id, key)。
// 既有行 tenant_id 默认 0 → 自动成为全局默认。幂等：已有 tenant_id 列则跳过；全新库交给 AutoMigrate。
func migrateConfigTenant(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Config{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.Config{}, "tenant_id") {
		return nil
	}
	if err := db.Exec("ALTER TABLE configs ADD COLUMN tenant_id BIGINT NOT NULL DEFAULT 0").Error; err != nil {
		return err
	}
	return db.Exec("ALTER TABLE configs DROP PRIMARY KEY, ADD PRIMARY KEY (tenant_id, `key`)").Error
}

// Seed 写入初始数据:默认租户、充值档位、道具、默认配置。幂等。
func Seed(db *gorm.DB, defaultTenantID int64) error {
	// 默认租户(单租户/迁移基准)
	var tCount int64
	db.Model(&model.Tenant{}).Where("tenant_id = ?", defaultTenantID).Count(&tCount)
	if tCount == 0 {
		if err := db.Create(&model.Tenant{
			TenantID: defaultTenantID, Name: "默认租户", Status: "active", CreatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
	}

	// 充值档位
	var pkgCount int64
	db.Model(&model.CoinPackage{}).Count(&pkgCount)
	if pkgCount == 0 {
		pkgs := []model.CoinPackage{
			{PackageID: 1, Name: "尝鲜", Coins: 60, BonusCoins: 0, PriceFen: 600, Status: "active", Sort: 1},
			{PackageID: 2, Name: "小海浪", Coins: 120, BonusCoins: 0, PriceFen: 1200, Status: "active", Sort: 2},
			{PackageID: 3, Name: "常用", Coins: 300, BonusCoins: 20, PriceFen: 3000, Status: "active", Sort: 3},
			{PackageID: 4, Name: "实惠", Coins: 680, BonusCoins: 88, PriceFen: 6800, Status: "active", Sort: 4},
			{PackageID: 5, Name: "大礼包", Coins: 1280, BonusCoins: 200, PriceFen: 12800, Status: "active", Sort: 5},
			{PackageID: 6, Name: "最划算", Coins: 3280, BonusCoins: 800, PriceFen: 32800, Status: "active", Sort: 6},
		}
		if err := db.Create(&pkgs).Error; err != nil {
			return err
		}
	}

	// 道具/礼物
	var itemCount int64
	db.Model(&model.Item{}).Count(&itemCount)
	if itemCount == 0 {
		items := []model.Item{
			{ItemID: 1, Name: "超级曝光", Type: "boost", PriceCoin: 30, Status: "active", Sort: 1},
			{ItemID: 2, Name: "瓶子置顶", Type: "top", PriceCoin: 20, Status: "active", Sort: 2},
			{ItemID: 3, Name: "超级喜欢", Type: "superlike", PriceCoin: 10, Status: "active", Sort: 3},
			{ItemID: 4, Name: "贝壳礼物", Type: "gift", PriceCoin: 8, Status: "active", Sort: 4},
			{ItemID: 5, Name: "灯塔礼物", Type: "gift", PriceCoin: 66, Status: "active", Sort: 5},
		}
		if err := db.Create(&items).Error; err != nil {
			return err
		}
	}
	// 次数包道具(#5):对已有库也补齐(FirstOrCreate 幂等)
	for _, it := range []model.Item{
		{ItemID: 6, Name: "扔瓶次数包", Type: "quota_throw", PriceCoin: 20, Status: "active", Sort: 6},
		{ItemID: 7, Name: "捞瓶次数包", Type: "quota_scoop", PriceCoin: 20, Status: "active", Sort: 7},
	} {
		db.Where(model.Item{ItemID: it.ItemID}).
			Attrs(model.Item{Name: it.Name, Type: it.Type, PriceCoin: it.PriceCoin, Status: it.Status, Sort: it.Sort}).
			FirstOrCreate(&model.Item{ItemID: it.ItemID})
	}

	// 默认配置(全局默认写 tenant_id=0;复合主键下零值 TenantID 不能进 WHERE,需显式反引号 SQL)
	defaults := []model.Config{
		{TenantID: 0, Key: "verify_required", Value: "0", Remark: "发瓶/开聊是否强制真人认证"},
		{TenantID: 0, Key: "ios_recharge_off", Value: "1", Remark: "iOS 是否隐藏充值入口"},
		{TenantID: 0, Key: "price_chat", Value: "5", Remark: "开聊消耗金币"},
		{TenantID: 0, Key: "price_unlock", Value: "2", Remark: "解锁回信消耗金币"},
		{TenantID: 0, Key: "reg_reward_coins", Value: "50", Remark: "注册奖励金币"},
		{TenantID: 0, Key: "feed_size", Value: "50", Remark: "捞瓶 feed 预生成条数"},
	}
	for _, c := range defaults {
		c.UpdatedAt = time.Now()
		// 已存在则跳过,不覆盖运营改过的值
		db.Where("tenant_id = 0 AND `key` = ?", c.Key).
			Attrs(model.Config{Value: c.Value, Remark: c.Remark, UpdatedAt: c.UpdatedAt}).
			FirstOrCreate(&c)
	}
	return nil
}
