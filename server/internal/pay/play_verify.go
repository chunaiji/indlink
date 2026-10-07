package pay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/provider"
	"driftbottle/pkg/apilog"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// Google Play 结算服务端校验。
//
// 客户端(in_app_purchase)拿到 purchaseToken 后调 /pay/play/verify;服务端用服务账号向
// androidpublisher 核实,入账后再 acknowledge——不 acknowledge 的购买 3 天后会被 Google 自动退款。
// 幂等键是 Play 的 orderId:App 冷启动会把未 consume 的购买重新推上来。

var playAPIBase = "https://androidpublisher.googleapis.com"
var playHTTPClient = &http.Client{Timeout: 10 * time.Second}

type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// playToken 服务账号 JSON → access_token(JWT bearer 换取)。tokenURLOverride 仅测试用。
func playToken(saJSON string, now time.Time, tokenURLOverride string) (string, error) {
	var sa serviceAccount
	if err := json.Unmarshal([]byte(saJSON), &sa); err != nil {
		return "", fmt.Errorf("服务账号 JSON 不合法: %w", err)
	}
	if sa.TokenURI == "" {
		sa.TokenURI = "https://oauth2.googleapis.com/token"
	}
	if tokenURLOverride != "" {
		sa.TokenURI = tokenURLOverride
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(sa.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("服务账号私钥不合法: %w", err)
	}
	claims := jwt.MapClaims{
		"iss":   sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/androidpublisher",
		"aud":   sa.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	assertion, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	resp, err := playHTTPClient.PostForm(sa.TokenURI, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("Google 换 token 失败: %s", string(body))
	}
	return tok.AccessToken, nil
}

type playProduct struct {
	OrderID              string `json:"orderId"`
	PurchaseState        int    `json:"purchaseState"` // 0 已购买 1 已取消 2 待处理
	ConsumptionState     int    `json:"consumptionState"`
	AcknowledgementState int    `json:"acknowledgementState"`
	PurchaseTimeMillis   string `json:"purchaseTimeMillis"`
	RegionCode           string `json:"regionCode"`
}

func (p playProduct) purchasedAt() time.Time {
	ms, _ := strconv.ParseInt(p.PurchaseTimeMillis, 10, 64)
	if ms <= 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}

// playDecision 能不能入账。
func playDecision(p playProduct) error {
	if p.OrderID == "" {
		return errs.New(errs.CodePaySignError, "Google 未返回订单号")
	}
	switch p.PurchaseState {
	case 0:
		return nil
	case 1:
		return errs.New(errs.CodeForbidden, "该购买已取消或退款")
	default:
		return errs.New(errs.CodeForbidden, "该购买仍在处理中")
	}
}

// VerifyPlay 核实 purchaseToken 并入账(幂等)。
func (s *Service) VerifyPlay(tenantID, userID int64, productID, purchaseToken string) (*model.PlayPurchase, error) {
	if s.providers == nil || !s.providers.Usable(tenantID, provider.KindPay, "google_play") {
		return nil, errs.New(errs.CodeForbidden, "Google Play 结算未开启")
	}
	row, _ := s.providers.Get(tenantID, provider.KindPay, "google_play")
	pkgName := row.Get("package_name")
	token, err := playToken(row.Get("service_account_json"), time.Now(), "")
	if err != nil {
		apilog.Record(tenantID, "play_token", pkgName, 0, err.Error(), false)
		return nil, errs.New(errs.CodeServerError, "Google 凭证无效")
	}
	u := fmt.Sprintf("%s/androidpublisher/v3/applications/%s/purchases/products/%s/tokens/%s",
		playAPIBase, url.PathEscape(pkgName), url.PathEscape(productID), url.PathEscape(purchaseToken))
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := playHTTPClient.Do(req)
	if err != nil {
		return nil, errs.New(errs.CodeServerError, "Google 查询失败")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	apilog.Record(tenantID, "play_verify", productID, resp.StatusCode, string(body), resp.StatusCode == 200)
	if resp.StatusCode != 200 {
		return nil, errs.New(errs.CodePaySignError, "购买凭证无效")
	}
	var p playProduct
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, errs.New(errs.CodeServerError, "Google 响应不合法")
	}
	if err := playDecision(p); err != nil {
		return nil, err
	}
	var pkg model.CoinPackage
	if err := s.db.First(&pkg, "play_product_id = ? AND status = ?", productID, "active").Error; err != nil {
		return nil, errs.New(errs.CodeBadRequest, "未知的商品:"+productID)
	}
	var prices []model.CoinPackagePrice
	s.db.Where("package_id = ?", pkg.PackageID).Find(&prices)
	price := pickPrice(prices, "gplay", p.RegionCode)
	coins := pkg.Coins + pkg.BonusCoins

	var saved model.PlayPurchase
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var exist model.PlayPurchase
		if e := tx.First(&exist, "order_id = ?", p.OrderID).Error; e == nil {
			saved = exist
			return nil
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		order := model.PayOrder{OrderNo: genOrderNo(), TenantID: tenantID, UserID: userID, PackageID: pkg.PackageID,
			Platform: "google_play", Coins: coins, Status: "paid", PlatformTxn: p.OrderID, CreatedAt: time.Now()}
		if price != nil {
			order.PriceMinor, order.Currency = price.Amount, price.Currency
		}
		paidAt := p.purchasedAt()
		order.PaidAt = &paidAt
		if e := tx.Create(&order).Error; e != nil {
			return e
		}
		saved = model.PlayPurchase{TenantID: tenantID, UserID: userID, OrderID: p.OrderID, PurchaseToken: purchaseToken,
			ProductID: productID, OrderNo: order.OrderNo, Coins: coins, State: "purchased", Currency: order.Currency,
			AmountMinor: order.PriceMinor, CreatedAt: time.Now()}
		if e := tx.Create(&saved).Error; e != nil {
			return e
		}
		return creditPaidOrder(tx, tenantID, userID, coins, order.OrderNo)
	})
	if err != nil {
		return nil, err
	}
	// acknowledge:失败只记日志,下次冷启动客户端会再推一次,幂等保证不重复发币
	if p.AcknowledgementState == 0 {
		ackReq, _ := http.NewRequest(http.MethodPost, u+":acknowledge", bytes.NewReader([]byte(`{}`)))
		ackReq.Header.Set("Authorization", "Bearer "+token)
		ackReq.Header.Set("Content-Type", "application/json")
		if r2, e := playHTTPClient.Do(ackReq); e == nil {
			r2.Body.Close()
			if r2.StatusCode < 300 {
				now := time.Now()
				s.db.Model(&model.PlayPurchase{}).Where("order_id = ?", p.OrderID).Update("acked_at", &now)
			}
			apilog.Record(tenantID, "play_ack", p.OrderID, r2.StatusCode, "", r2.StatusCode < 300)
		}
	}
	return &saved, nil
}
