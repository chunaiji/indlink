package wallet

import (
	"gorm.io/gorm/clause"
)

// lockForUpdate 返回 SELECT ... FOR UPDATE 子句,用于钱包行锁。
func lockForUpdate() clause.Locking {
	return clause.Locking{Strength: "UPDATE"}
}
