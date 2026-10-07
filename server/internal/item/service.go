package item

import (
	"strconv"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/quota"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// Service 增值道具/礼物:列表 + 购买/赠送(消耗金币)。
type Service struct {
	db  *gorm.DB
	wlt *wallet.Service
}

func New(db *gorm.DB, wlt *wallet.Service) *Service { return &Service{db: db, wlt: wlt} }

func (s *Service) List() ([]model.Item, error) {
	var list []model.Item
	err := s.db.Where("status = ?", "active").Order("sort asc").Find(&list).Error
	return list, err
}

// MyItems 返回用户每种道具的持有数量。
func (s *Service) MyItems(userID int64) (map[string]int64, error) {
	type row struct {
		ItemID int64
		Cnt    int64
	}
	var rows []row
	// 只统计自购库存(target_id=0);送给别人的不计入持有
	err := s.db.Model(&model.ItemOrder{}).
		Select("item_id, count(*) as cnt").
		Where("user_id = ? AND target_id = 0", userID).
		Group("item_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(rows))
	for _, r := range rows {
		m[strconv.FormatInt(r.ItemID, 10)] = r.Cnt
	}
	return m, nil
}

// OrderRow 道具流水一行(App「道具」页的记录区)。
//
// kind: buy(买进背包) / sent(送出) / received(收到)。送出一件记一行,库存抵扣的也记——
// 用户要看的是"我的礼物去哪了",不只是钱。
type OrderRow struct {
	ID        int64     `json:"id,string"`
	Kind      string    `json:"kind"`
	ItemID    int64     `json:"item_id,string"`
	ItemName  string    `json:"item_name"`
	ItemIcon  string    `json:"item_icon"`
	Coins     int64     `json:"coins"`
	PeerID    int64     `json:"peer_id,string"`
	PeerName  string    `json:"peer_name"`
	CreatedAt time.Time `json:"created_at"`
}

// Orders 我的道具流水:买进(user=我,target=0)、送出(user=我,target≠0)、收到(target=我)。
func (s *Service) Orders(tenantID, userID int64, page, size int) ([]OrderRow, error) {
	if size <= 0 || size > 100 {
		size = 50
	}
	if page < 1 {
		page = 1
	}
	var rows []OrderRow
	err := s.db.Raw(`
		SELECT o.id,
		       CASE WHEN o.user_id = ? AND o.target_id = 0 THEN 'buy'
		            WHEN o.user_id = ? THEN 'sent' ELSE 'received' END AS kind,
		       o.item_id, COALESCE(i.name,'') AS item_name, COALESCE(i.icon,'') AS item_icon,
		       o.coins,
		       CASE WHEN o.user_id = ? THEN o.target_id ELSE o.user_id END AS peer_id,
		       COALESCE(u.nickname,'') AS peer_name,
		       o.created_at
		FROM item_orders o
		LEFT JOIN items i ON i.item_id = o.item_id
		LEFT JOIN users u ON u.user_id = CASE WHEN o.user_id = ? THEN o.target_id ELSE o.user_id END
		WHERE o.tenant_id = ? AND (o.user_id = ? OR o.target_id = ?)
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT ? OFFSET ?`,
		userID, userID, userID, userID, tenantID, userID, userID, size, (page-1)*size).Scan(&rows).Error
	if rows == nil {
		rows = []OrderRow{}
	}
	return rows, err
}

// Buy 购买/赠送道具:用金币扣费,记录 item_order。targetID 为 0 表示自用。
func (s *Service) Buy(tenantID, userID, itemID, targetID int64) error {
	var it model.Item
	if err := s.db.First(&it, "item_id = ? AND status = ?", itemID, "active").Error; err != nil {
		return errs.New(errs.CodeBadRequest, "道具不存在")
	}
	bizNo := "item:" + strconv.FormatInt(idgen.Next(), 10)
	if err := s.wlt.Debit(tenantID, userID, it.PriceCoin, wallet.SceneGift, bizNo, func(tx *gorm.DB) error {
		return tx.Create(&model.ItemOrder{
			ID: idgen.Next(), TenantID: tenantID, UserID: userID, ItemID: itemID, TargetID: targetID,
			Coins: it.PriceCoin, CreatedAt: time.Now(),
		}).Error
	}); err != nil {
		return err
	}
	// 次数包道具:购买后充值对应的扔/捞次数(#5 方案 B)
	switch it.Type {
	case "quota_throw":
		quota.AddPack(tenantID, userID, quota.ActionThrow, sysconfig.GetInt(tenantID, sysconfig.KeyQuotaThrowPack))
	case "quota_scoop":
		quota.AddPack(tenantID, userID, quota.ActionScoop, sysconfig.GetInt(tenantID, sysconfig.KeyQuotaScoopPack))
	}
	return nil
}
