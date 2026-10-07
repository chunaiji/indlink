package wallet

import (
	"errors"
	"strconv"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// 消费场景常量。
const (
	SceneRecharge = "recharge"
	SceneChat     = "chat"
	SceneUnlock   = "unlock"
	SceneGift     = "gift"
	SceneReward   = "reward"
	SceneCheckin  = "checkin"
	SceneMsg      = "msg"
	SceneShare    = "share"
	SceneAdReward = "ad_reward"
	// SceneRewind 发现页撤回:把刚左滑掉的人捞回来。
	SceneRewind = "rewind"
	// SceneDiscoverSkip 发现页左滑跳过扣币(付费功能,默认关)。
	SceneDiscoverSkip = "discover_skip"
	// SceneRefund 退款扣回。苹果侧用户可以直接向 Apple 申请退款,
	// 此时币可能已经花掉——允许记负账,由人工跟进,不能装作没发生。
	SceneRefund = "refund"
	// SceneAdmin 后台人工调账(加币/扣币),流水 Remark 记原因。
	SceneAdmin = "admin"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Balance 返回金币余额(无钱包则视为 0)。
func (s *Service) Balance(userID int64) (int64, error) {
	var w model.Wallet
	err := s.db.Select("balance").First(&w, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return w.Balance, nil
}

// EnsureWalletTx 在事务内确保钱包行存在并加行锁,返回当前余额。供 pay 回调复用。
func EnsureWalletTx(tx *gorm.DB, tenantID, userID int64) (int64, error) {
	var w model.Wallet
	// 行锁:SELECT ... FOR UPDATE
	err := tx.Clauses(lockForUpdate()).First(&w, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		w = model.Wallet{UserID: userID, TenantID: tenantID, Balance: 0, UpdatedAt: time.Now()}
		if err := tx.Create(&w).Error; err != nil {
			return 0, err
		}
		if err := tx.Clauses(lockForUpdate()).First(&w, "user_id = ?", userID).Error; err != nil {
			return 0, err
		}
	} else if err != nil {
		return 0, err
	}
	return w.Balance, nil
}

// CreditTx 在已有事务内加币 + 写流水(供 pay 回调复用,避免嵌套事务)。
func CreditTx(tx *gorm.DB, tenantID, userID, coins int64, scene, bizNo string) error {
	bal, err := EnsureWalletTx(tx, tenantID, userID)
	if err != nil {
		return err
	}
	newBal := bal + coins
	if err := tx.Model(&model.Wallet{}).Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"balance":         newBal,
			"total_recharged": gorm.Expr("total_recharged + ?", coins),
			"updated_at":      time.Now(),
		}).Error; err != nil {
		return err
	}
	return tx.Create(&model.WalletTxn{
		TxnID: idgen.Next(), TenantID: tenantID, UserID: userID, Direction: "credit",
		Coins: coins, Scene: scene, BizNo: bizNo, BalanceAfter: newBal, CreatedAt: time.Now(),
	}).Error
}

// Credit 加金币(充值入账 / 注册奖励)。
func (s *Service) Credit(tenantID, userID, coins int64, scene, bizNo string) error {
	if coins <= 0 {
		return errs.New(errs.CodeBadRequest, "金额非法")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return CreditTx(tx, tenantID, userID, coins, scene, bizNo)
	})
}

// Debit 扣金币(开聊/解锁/送礼)。同事务执行 biz 回调,保证"扣费+业务"原子。
func (s *Service) Debit(tenantID, userID, coins int64, scene, bizNo string, biz func(tx *gorm.DB) error) error {
	if coins < 0 {
		return errs.New(errs.CodeBadRequest, "金额非法")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		bal, err := EnsureWalletTx(tx, tenantID, userID)
		if err != nil {
			return err
		}
		if bal < coins {
			return errs.ErrInsufficient
		}
		newBal := bal - coins
		if coins > 0 {
			if err := tx.Model(&model.Wallet{}).Where("user_id = ?", userID).
				Updates(map[string]interface{}{
					"balance":     newBal,
					"total_spent": gorm.Expr("total_spent + ?", coins),
					"updated_at":  time.Now(),
				}).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.WalletTxn{
				TxnID: idgen.Next(), TenantID: tenantID, UserID: userID, Direction: "debit",
				Coins: coins, Scene: scene, BizNo: bizNo, BalanceAfter: newBal, CreatedAt: time.Now(),
			}).Error; err != nil {
				return err
			}
		}
		if biz != nil {
			return biz(tx)
		}
		return nil
	})
}

// AdminAdjust 后台人工调账:delta>0 加币、<0 扣币,返回调整后余额。
//
// 只动 balance,不计入 total_recharged / total_spent——那两个口径是「用户真实充值/消费」,
// 运营补偿或纠错混进去会把付费统计污染掉。扣币不允许扣成负数:
// 退款那条负账是外部事实不得不记,人工操作没有这个理由。
func (s *Service) AdminAdjust(tenantID, userID, delta int64, remark string) (int64, error) {
	if delta == 0 {
		return 0, errs.New(errs.CodeBadRequest, "金额非法")
	}
	var newBal int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		bal, err := EnsureWalletTx(tx, tenantID, userID)
		if err != nil {
			return err
		}
		newBal = bal + delta
		if newBal < 0 {
			return errs.ErrInsufficient
		}
		if err := tx.Model(&model.Wallet{}).Where("user_id = ?", userID).
			Updates(map[string]interface{}{"balance": newBal, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		txn := &model.WalletTxn{
			TxnID: idgen.Next(), TenantID: tenantID, UserID: userID,
			Scene: SceneAdmin, BalanceAfter: newBal, Remark: remark, CreatedAt: time.Now(),
		}
		txn.BizNo = "admin:" + strconv.FormatInt(txn.TxnID, 10)
		if delta > 0 {
			txn.Direction, txn.Coins = "credit", delta
		} else {
			txn.Direction, txn.Coins = "debit", -delta
		}
		return tx.Create(txn).Error
	})
	return newBal, err
}

// Txns 分页查询流水。
func (s *Service) Txns(userID int64, page, size int) ([]model.WalletTxn, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	var list []model.WalletTxn
	err := s.db.Where("user_id = ?", userID).
		Order("created_at desc").
		Offset((page - 1) * size).Limit(size).
		Find(&list).Error
	return list, err
}
