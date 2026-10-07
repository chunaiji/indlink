package pay

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/provider"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/apilog"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// Apple 内购(IAP)。
//
// 为什么 iOS 必须单独一套：App Store 3.1.1 规定 App 内消费的数字商品必须走 IAP，
// UPI / Card 等第三方渠道在 iOS 上会被直接拒审，引导去站外网页充值同样违规。
// 所以 iOS 与 Android 的充值是**两条链路**，只在入账那一步汇合。
//
// 幂等以 Apple 的 transactionId 为准：未 finish 的交易会在 App 冷启动时重新推给服务端，
// 不幂等就会一笔钱发两次币。

const (
	appStoreProdAPI    = "https://api.storekit.itunes.apple.com"
	appStoreSandboxAPI = "https://api.storekit-sandbox.itunes.apple.com"

	envSandbox = "Sandbox"
)

// iapHTTPClient 调 App Store Server API 用。超时给足：跨境请求。
var iapHTTPClient = &http.Client{Timeout: 10 * time.Second}

// iapTransaction JWS 解出来的交易信息(只取用得到的字段)。
type iapTransaction struct {
	TransactionID         string `json:"transactionId"`
	OriginalTransactionID string `json:"originalTransactionId"`
	ProductID             string `json:"productId"`
	BundleID              string `json:"bundleId"`
	Environment           string `json:"environment"`
	Type                  string `json:"type"`
	Quantity              int    `json:"quantity"`
	PurchaseDate          int64  `json:"purchaseDate"` // 毫秒
	RevocationDate        int64  `json:"revocationDate"`
	RevocationReason      *int   `json:"revocationReason"`
}

func (t iapTransaction) purchasedAt() time.Time {
	if t.PurchaseDate <= 0 {
		return time.Now()
	}
	return time.UnixMilli(t.PurchaseDate)
}

// appleNotification App Store Server Notifications V2 的外层负载。
type appleNotification struct {
	NotificationType string `json:"notificationType"`
	Subtype          string `json:"subtype"`
	Data             struct {
		BundleID              string `json:"bundleId"`
		Environment           string `json:"environment"`
		SignedTransactionInfo string `json:"signedTransactionInfo"`
	} `json:"data"`
}

// decodeJWSPayload 取 JWS 的 payload 段。
//
// 这里**不验签**：签名校验交给下面的 verifyWithApple —— 与其自己维护 Apple 根证书链，
// 不如带着我们的 API 私钥主动向苹果查一次，拿到的才是权威答案。
func decodeJWSPayload(jws string, out interface{}) error {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return errors.New("不是合法的 JWS")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// iapConfig 该租户的 App Store 配置(服务商页「iOS 内购」卡片),未启用返回 false。
func (s *Service) iapConfig(tenantID int64) (*provider.Resolved, bool) {
	if s.providers == nil {
		return nil, false
	}
	r, ok := s.providers.Get(tenantID, provider.KindPay, "apple")
	return r, ok && r.Enabled
}

// appStoreToken 生成调用 App Store Server API 的 ES256 JWT。
func (s *Service) appStoreToken(tenantID int64, bundleID string) (string, error) {
	row, ok := s.iapConfig(tenantID)
	if !ok {
		return "", errors.New("未配置 App Store Server API 凭证")
	}
	issuer, keyID, pemStr := row.Get("issuer_id"), row.Get("key_id"), row.Get("p8_key")
	if issuer == "" || keyID == "" || pemStr == "" {
		return "", errors.New("未配置 App Store Server API 凭证")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return "", errors.New(".p8 私钥格式不正确")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return "", errors.New(".p8 不是 EC 私钥")
	}

	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": issuer,
		"iat": now.Unix(),
		"exp": now.Add(30 * time.Minute).Unix(),
		"aud": "appstoreconnect-v1",
		"bid": bundleID,
	})
	tok.Header["kid"] = keyID
	return tok.SignedString(ecKey)
}

// verifyWithApple 主动向 App Store Server API 核实交易。
//
// 返回 nil 表示「未配置凭证、跳过核实」，由调用方按开关决定是否放行。
func (s *Service) verifyWithApple(tenantID int64, txnID, env, bundleID string) (*iapTransaction, error) {
	token, err := s.appStoreToken(tenantID, bundleID)
	if err != nil {
		return nil, nil // 未配凭证
	}
	base := appStoreProdAPI
	if env == envSandbox {
		base = appStoreSandboxAPI
	}
	req, err := http.NewRequest("GET", base+"/inApps/v1/transactions/"+txnID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := iapHTTPClient.Do(req)
	if err != nil {
		apilog.Record(tenantID, "iap_verify", txnID, -1, err.Error(), false)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		apilog.Record(tenantID, "iap_verify", txnID, resp.StatusCode, "http error", false)
		return nil, fmt.Errorf("App Store 返回 %d", resp.StatusCode)
	}
	var body struct {
		SignedTransactionInfo string `json:"signedTransactionInfo"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	var t iapTransaction
	if err := decodeJWSPayload(body.SignedTransactionInfo, &t); err != nil {
		return nil, err
	}
	apilog.Record(tenantID, "iap_verify", txnID, 200, t.ProductID, true)
	return &t, nil
}

// VerifyIAP 校验客户端提交的交易并入账。
//
// ⚠️ 客户端必须**等本接口返回成功后才 finish** 这笔 StoreKit transaction。
// 提前 finish = 苹果认为交付完成，用户钱扣了币没到，且再也拿不到这笔交易。
func (s *Service) VerifyIAP(tenantID, userID int64, signedTransaction string) (*model.IAPTransaction, error) {
	row, ok := s.iapConfig(tenantID)
	if !ok {
		return nil, errs.New(errs.CodeForbidden, "内购未开启")
	}
	bundleID := row.Get("bundle_id")
	if bundleID == "" {
		return nil, errs.New(errs.CodeServerError, "未配置 Apple Bundle ID")
	}
	sandboxOK := row.Bool("sandbox")

	var claimed iapTransaction
	if err := decodeJWSPayload(signedTransaction, &claimed); err != nil {
		return nil, errs.New(errs.CodeBadRequest, "交易凭证格式不正确")
	}
	if claimed.TransactionID == "" {
		return nil, errs.New(errs.CodeBadRequest, "交易凭证缺少 transactionId")
	}

	// 以苹果的答复为准；拿不到就用客户端提交的内容，但仅限沙盒开关打开时。
	txn := &claimed
	verified, err := s.verifyWithApple(tenantID, claimed.TransactionID, claimed.Environment, bundleID)
	if err != nil {
		return nil, errs.New(errs.CodePaySignError, "交易校验失败")
	}
	if verified != nil {
		txn = verified
	} else if !sandboxOK {
		// 没配 API 凭证又不允许沙盒 —— 不能凭客户端一面之词发币。
		return nil, errs.New(errs.CodeServerError, "未配置 App Store Server API 凭证")
	}

	if txn.BundleID != "" && txn.BundleID != bundleID {
		return nil, errs.New(errs.CodePaySignError, "交易不属于本应用")
	}
	if txn.Environment == envSandbox && !sandboxOK {
		return nil, errs.New(errs.CodeForbidden, "沙盒交易不予入账")
	}
	if txn.RevocationDate > 0 {
		return nil, errs.New(errs.CodeForbidden, "该交易已被撤销")
	}

	// 商品 → 档位。iOS 档位单独配（30% 抽成要在定价里吃掉，两端价格本就不同）。
	var pkg model.CoinPackage
	if err := s.db.First(&pkg, "ios_product_id = ? AND status = ?", txn.ProductID, "active").Error; err != nil {
		return nil, errs.New(errs.CodeBadRequest, "未知的内购商品:"+txn.ProductID)
	}

	qty := int64(txn.Quantity)
	if qty <= 0 {
		qty = 1
	}
	coins := (pkg.Coins + pkg.BonusCoins) * qty

	var saved model.IAPTransaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 幂等：同一 transactionId 只入账一次
		var exist model.IAPTransaction
		err := tx.First(&exist, "transaction_id = ?", txn.TransactionID).Error
		if err == nil {
			saved = exist
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		order := model.PayOrder{
			OrderNo:     genOrderNo(),
			TenantID:    tenantID,
			UserID:      userID,
			PackageID:   pkg.PackageID,
			Platform:    "apple",
			PriceMinor:  pkg.PriceFen, // 记录用；真实收款金额以 App Store 结算为准
			Currency:    "CNY",        // iOS 档位价目迁到 CoinPackagePrice 之前先记 CNY
			Coins:       coins,
			Status:      "paid",
			PlatformTxn: txn.TransactionID,
			CreatedAt:   time.Now(),
		}
		paidAt := txn.purchasedAt()
		order.PaidAt = &paidAt
		if err := tx.Create(&order).Error; err != nil {
			return err
		}

		saved = model.IAPTransaction{
			TransactionID:         txn.TransactionID,
			OriginalTransactionID: txn.OriginalTransactionID,
			TenantID:              tenantID,
			UserID:                userID,
			ProductID:             txn.ProductID,
			OrderNo:               order.OrderNo,
			Coins:                 coins,
			Environment:           txn.Environment,
			Status:                "paid",
			PurchasedAt:           paidAt,
			CreatedAt:             time.Now(),
		}
		if err := tx.Create(&saved).Error; err != nil {
			return err
		}

		return creditPaidOrder(tx, tenantID, userID, coins, order.OrderNo)
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

// HandleAppleNotification 处理 App Store Server Notifications V2。
//
// **这是感知退款的唯一途径**：用户直接向苹果申请退款时，App 端收不到任何同步信号。
// 没有它，退款发生后金币已经发出去甚至花掉，账永远对不平。
func (s *Service) HandleAppleNotification(signedPayload string) error {
	var note appleNotification
	if err := decodeJWSPayload(signedPayload, &note); err != nil {
		return errs.New(errs.CodeBadRequest, "通知格式不正确")
	}
	if note.Data.SignedTransactionInfo == "" {
		return nil // 与交易无关的通知(如订阅续期),忽略
	}
	var txn iapTransaction
	if err := decodeJWSPayload(note.Data.SignedTransactionInfo, &txn); err != nil {
		return errs.New(errs.CodeBadRequest, "交易信息不正确")
	}

	switch note.NotificationType {
	case "REFUND", "REVOKE":
		return s.revokeIAP(txn.TransactionID, strings.ToLower(note.NotificationType))
	default:
		log.Printf("[iap] 收到通知 %s/%s txn=%s，未做处理",
			note.NotificationType, note.Subtype, txn.TransactionID)
		return nil
	}
}

// revokeIAP 退款/撤销：扣回已发放的金币。
//
// 允许扣成负数——币可能已经花掉了。装作没发生只会让账永远对不平；
// 记负账 + 冻结账号，交人工跟进，这是唯一诚实的处理。
func (s *Service) revokeIAP(transactionID, status string) error {
	if transactionID == "" {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var rec model.IAPTransaction
		if err := tx.First(&rec, "transaction_id = ?", transactionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // 没入过账,无需扣回
			}
			return err
		}
		if rec.Status != "paid" {
			return nil // 幂等
		}

		now := time.Now()
		if err := tx.Model(&model.IAPTransaction{}).
			Where("transaction_id = ? AND status = ?", transactionID, "paid").
			Updates(map[string]interface{}{"status": status, "refunded_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.PayOrder{}).
			Where("order_no = ?", rec.OrderNo).
			Update("status", "refunded").Error; err != nil {
			return err
		}
		// 负数入账即扣回
		if err := wallet.CreditTx(tx, rec.TenantID, rec.UserID, -rec.Coins,
			wallet.SceneRefund, rec.OrderNo); err != nil {
			return err
		}
		// 退款后余额为负说明币已花掉,冻结账号交人工处理
		var w model.Wallet
		if err := tx.Select("balance").First(&w, "user_id = ?", rec.UserID).Error; err == nil && w.Balance < 0 {
			log.Printf("[iap] 退款后余额为负 uid=%d balance=%d，已冻结账号", rec.UserID, w.Balance)
			if err := tx.Model(&model.User{}).Where("user_id = ?", rec.UserID).
				Update("status", "frozen").Error; err != nil {
				return err
			}
		}
		return nil
	})
}
