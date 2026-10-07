package pay

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/config"
	"driftbottle/internal/model"
	"driftbottle/internal/provider"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/tenant"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// OpenIDFunc 给定 userID 返回其平台身份(微信 openid / 支付宝 user_id)。
type OpenIDFunc func(userID int64) (string, error)

type Service struct {
	db           *gorm.DB
	wallet       *wallet.Service
	getOpenID    OpenIDFunc // 微信小程序 openid(JSAPI 下单要)
	getAlipayUID OpenIDFunc // 支付宝小程序 buyer_id(trade.create 要)
	cfg          *config.Config
	creds        *tenant.Store   // 登录凭证(支付不再读它,留给别处)
	providers    *provider.Store // 服务商配置:支付渠道的凭据全在这里

	mu          sync.RWMutex
	driverCache map[string]Driver // key "tenantID:channel"
	cacheVer    uint64            // 对应 providers.Version();不一致时整体清缓存

	syncMu   sync.Mutex
	lastSync map[string]time.Time // orderNo → 上次主动查单时间
}

func New(db *gorm.DB, w *wallet.Service, getOpenID, getAlipayUID OpenIDFunc, cfg *config.Config, creds *tenant.Store, providers *provider.Store) *Service {
	return &Service{
		db: db, wallet: w, getOpenID: getOpenID, getAlipayUID: getAlipayUID, cfg: cfg, creds: creds, providers: providers,
		driverCache: map[string]Driver{},
	}
}

// Orders 分页查询用户支付订单。
func (s *Service) Orders(userID int64, page, size int) ([]model.PayOrder, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	var list []model.PayOrder
	err := s.db.Where("user_id = ?", userID).
		Order("created_at desc").
		Offset((page - 1) * size).Limit(size).
		Find(&list).Error
	return list, err
}

// channelAliases 历史渠道名 → 规范名。订单表与已发布客户端里还有旧值,一律先折算。
var channelAliases = map[string]string{"wx": "wechat", "wx_app": "wechat", "alipay_app": "alipay", "ios": "apple", "gplay": "google_play"}

func canonicalChannel(p string) string {
	if c, ok := channelAliases[p]; ok {
		return c
	}
	return p
}

// appChannels App 端可在 /pay/order 选的渠道(IAP / Play 走各自的 verify 接口,不在这里)。
var appChannels = map[string]bool{"wechat": true, "alipay": true}

// resolveChannel 「登录平台 + 客户端选的渠道」→ 规范渠道名。
//
// App 平台必须选渠道(mock 开着时例外:联调走 mock,渠道参数忽略);
// 小程序平台**不许**传渠道——登录平台就是支付平台,传了说明客户端在乱来。
func resolveChannel(platform, channel string, mockOn bool) (string, error) {
	channel = canonicalChannel(channel)
	if platform == "app" {
		if mockOn {
			return "app", nil
		}
		if !appChannels[channel] {
			return "", errs.New(errs.CodeBadRequest, "请选择支付方式")
		}
		return channel, nil
	}
	if channel != "" {
		return "", errs.New(errs.CodeBadRequest, "不支持的支付方式")
	}
	return canonicalChannel(platform), nil
}

// ensureCacheVersion 服务商配置重载过就清空驱动缓存(后台改配置不用重启)。调用方持有 s.mu。
func (s *Service) ensureCacheVersion(v uint64) {
	if s.cacheVer != v {
		s.driverCache = map[string]Driver{}
		s.cacheVer = v
	}
}

// driverFor 按租户 + 渠道返回支付驱动。带缓存,缓存随服务商配置版本整体失效。
func (s *Service) driverFor(tenantID int64, channel string) (Driver, error) {
	channel = canonicalChannel(channel)
	// mock 渠道不走缓存:开关是租户级且可热更,缓存下来后台关掉开关仍会走 mock——一个「已关闭」的假渠道继续发币。
	if channel == "app" {
		return s.buildDriver(tenantID, channel)
	}

	key := fmt.Sprintf("%d:%s", tenantID, channel)
	var ver uint64
	if s.providers != nil {
		ver = s.providers.Version()
	}
	s.mu.Lock()
	s.ensureCacheVersion(ver)
	if d, ok := s.driverCache[key]; ok {
		s.mu.Unlock()
		return d, nil
	}
	s.mu.Unlock()

	d, err := s.buildDriver(tenantID, channel)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.ensureCacheVersion(ver)
	s.driverCache[key] = d
	s.mu.Unlock()
	return d, nil
}

func (s *Service) buildDriver(tenantID int64, channel string) (Driver, error) {
	// 联调用的假渠道。默认关;关着时 App 平台仍然走原逻辑,即「请选择支付方式」。
	if channel == "app" && sysconfig.GetBool(tenantID, sysconfig.KeyAppPayMockEnabled) {
		return mockDriver{}, nil
	}

	if s.providers != nil {
		if row, ok := s.providers.Get(tenantID, provider.KindPay, channel); ok && s.providers.Usable(tenantID, provider.KindPay, channel) {
			// 交易类型由租户类型定:App 租户走 APP 支付,小程序租户走 JSAPI / trade.create。
			tt := ""
			if s.providers.TenantType(tenantID) == "app" {
				tt = "APP"
			}
			switch channel {
			case "wechat":
				return NewWxDriver(WxCreds{
					AppID: row.Get("app_id"), MchID: row.Get("mch_id"), APIv3Key: row.Get("apiv3_key"), SerialNo: row.Get("serial_no"),
					PrivateKeyPEM: row.Get("private_key"), PlatformPubPEM: row.Get("platform_key"), PlatformSerial: row.Get("platform_serial"),
					NotifyURL: row.Get("notify_url"), TradeType: tt, TenantID: tenantID,
				}), nil
			case "alipay":
				return NewAlipayDriver(AliCreds{
					AppID: row.Get("app_id"), PrivateKeyPEM: row.Get("private_key"), PublicKey: row.Get("alipay_public_key"),
					NotifyURL: row.Get("notify_url"), PID: row.Get("pid"), TradeType: tt, Sandbox: row.Bool("sandbox"), TenantID: tenantID,
				}), nil
			}
		}
		if s.cfg.MultiTenant {
			return nil, errs.New(errs.CodeBadRequest, "该租户未配置该支付渠道")
		}
	}
	// 单租户 .env 回落(本地开发)
	switch channel {
	case "wechat":
		c := WxCreds{
			AppID: s.cfg.WX.AppID, MchID: s.cfg.WX.MchID, APIv3Key: s.cfg.WX.PayAPIv3Key,
			SerialNo: s.cfg.WX.PaySerialNo, PlatformSerial: s.cfg.WX.PayPlatformSerial, NotifyURL: s.cfg.WX.NotifyURL,
		}
		if s.cfg.WX.PayPrivateKeyPath != "" {
			if data, err := readFile(s.cfg.WX.PayPrivateKeyPath); err == nil {
				c.PrivateKeyPEM = string(data)
			}
		}
		if s.cfg.WX.PayPlatformKeyPath != "" {
			if data, err := readFile(s.cfg.WX.PayPlatformKeyPath); err == nil {
				c.PlatformPubPEM = string(data)
			}
		}
		return NewWxDriver(c), nil
	case "alipay":
		return NewAlipayDriver(AliCreds{AppID: s.cfg.Alipay.AppID, NotifyURL: s.cfg.Alipay.NotifyURL}), nil
	}
	return nil, errs.New(errs.CodeBadRequest, "不支持的支付平台")
}

func (s *Service) Packages() ([]model.CoinPackage, error) {
	var list []model.CoinPackage
	err := s.db.Where("status = ?", "active").Order("sort asc").Find(&list).Error
	return list, err
}

// CreateOrder 创建充值订单并向平台预下单。价格以服务端档位为准。
// channel 只对 App 平台有意义(wechat / alipay),见 resolveChannel。
func (s *Service) CreateOrder(tenantID, userID, packageID int64, platform, channel string) (orderNo string, payParams map[string]interface{}, err error) {
	mockOn := platform == "app" && sysconfig.GetBool(tenantID, sysconfig.KeyAppPayMockEnabled)
	driverPlatform, err := resolveChannel(platform, channel, mockOn)
	if err != nil {
		return "", nil, err
	}
	d, err := s.driverFor(tenantID, driverPlatform)
	if err != nil {
		return "", nil, err
	}
	var pkg model.CoinPackage
	if err := s.db.First(&pkg, "package_id = ? AND status = ?", packageID, "active").Error; err != nil {
		return "", nil, errs.New(errs.CodeBadRequest, "充值档位不存在")
	}

	order := model.PayOrder{
		OrderNo:    genOrderNo(),
		TenantID:   tenantID,
		UserID:     userID,
		PackageID:  pkg.PackageID,
		Platform:   driverPlatform,
		PriceMinor: pkg.PriceFen,
		Currency:   "CNY", // 微信/支付宝只收人民币
		Coins:      pkg.Coins + pkg.BonusCoins,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	if err := s.db.Create(&order).Error; err != nil {
		return "", nil, err
	}

	// 小程序下单要买家身份(JSAPI openid / trade.create buyer_id);App 渠道不需要。
	var payerID string
	switch {
	case driverPlatform == "wechat" && platform == "wx" && s.getOpenID != nil:
		payerID, _ = s.getOpenID(userID)
	case driverPlatform == "alipay" && platform == "alipay" && s.getAlipayUID != nil:
		payerID, _ = s.getAlipayUID(userID)
	}
	params, err := d.Prepay(&order, payerID)
	if err != nil {
		return "", nil, err
	}
	return order.OrderNo, params, nil
}

// creditPaidOrder 入账 + 首充打「付费用户」标签。所有渠道共用,这是唯一的发币口。
func creditPaidOrder(tx *gorm.DB, tenantID, userID, coins int64, orderNo string) error {
	if err := wallet.CreditTx(tx, tenantID, userID, coins, wallet.SceneRecharge, orderNo); err != nil {
		return err
	}
	// MySQL 方言;已含则不追加,幂等
	return tx.Exec(
		"UPDATE users SET tags = IF(tags = '' OR tags IS NULL, ?, CONCAT(tags, ',', ?)) "+
			"WHERE user_id = ? AND (tags IS NULL OR tags NOT LIKE ?)",
		model.TagPaidUser, model.TagPaidUser, userID, "%"+model.TagPaidUser+"%",
	).Error
}

// HandleCallback 处理平台异步回调:验签 -> 幂等 -> 金额校验 -> 入账。
// 加币只在此发生,且对同一 order_no 幂等。
func (s *Service) HandleCallback(platform string, res *CallbackResult) error {
	if res == nil || res.OrderNo == "" {
		return errs.New(errs.CodePaySignError, "回调内容非法")
	}
	var order model.PayOrder
	if err := s.db.First(&order, "order_no = ?", res.OrderNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.New(errs.CodeOrderNotFound, "订单不存在")
		}
		return err
	}
	if order.Status == "paid" {
		return nil
	}
	if !res.Paid {
		return nil
	}
	if res.AmountFen > 0 && res.AmountFen != order.PriceMinor {
		return errs.New(errs.CodePaySignError, "回调金额与订单不一致")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		ret := tx.Model(&model.PayOrder{}).
			Where("order_no = ? AND status = ?", order.OrderNo, "pending").
			Updates(map[string]interface{}{
				"status": "paid", "platform_txn_id": res.TxnID, "paid_at": time.Now(),
			})
		if ret.Error != nil {
			return ret.Error
		}
		if ret.RowsAffected == 0 {
			return nil // 已被其它回调处理,幂等
		}
		return creditPaidOrder(tx, order.TenantID, order.UserID, order.Coins, order.OrderNo)
	})
}

// DriverForTenant 供 handler 取回调验签 driver + 应答报文(带 tenantId 的旧回调路径)。
func (s *Service) DriverForTenant(tenantID int64, platform string) (Driver, bool) {
	d, err := s.driverFor(tenantID, platform)
	if err != nil {
		return nil, false
	}
	return d, true
}

// wxCallbackTenants 一个平台公钥 ID 对应哪些租户,按租户 ID 升序(确定性)。
//
// 三种情形:唯一命中 → 只给那一家;同 serial 多租户(同一商户号服务两个租户)→ 全给;
// serial 不认识(微信轮换了平台公钥)→ 退回全部微信租户。调用方逐个验签,**过了的那家才是真的**。
// 绝不猜默认租户:猜错会用错 APIv3 密钥解密,回调 400、微信重试 24 小时后放弃,用户付了钱拿不到币。
func wxCallbackTenants(rows []*provider.Resolved, serial string) []int64 {
	var exact, all []int64
	for _, r := range rows {
		if r.Kind != provider.KindPay || r.Provider != "wechat" || !r.Enabled {
			continue
		}
		all = append(all, r.TenantID)
		if serial != "" && r.Get("platform_serial") == serial {
			exact = append(exact, r.TenantID)
		}
	}
	out := exact
	if len(out) == 0 {
		out = all
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// WxCallbackDrivers 微信回调的候选驱动,按 wxCallbackTenants 的顺序。
func (s *Service) WxCallbackDrivers(serial string) []Driver {
	var out []Driver
	if s.providers != nil {
		for _, tid := range wxCallbackTenants(s.providers.All(), serial) {
			if d, err := s.driverFor(tid, "wechat"); err == nil {
				out = append(out, d)
			}
		}
	}
	if len(out) == 0 {
		// 单租户 .env 部署:没有服务商行,回落默认租户
		if d, err := s.driverFor(s.cfg.DefaultTenantID, "wechat"); err == nil {
			out = append(out, d)
		}
	}
	return out
}

// DriverByAlipayAppID 支付宝回调:表单 app_id → 租户。
func (s *Service) DriverByAlipayAppID(appid string) (Driver, bool) {
	if s.providers != nil && appid != "" {
		if r, ok := s.providers.ByLookup(provider.KindPay, "alipay", appid, ""); ok {
			d, err := s.driverFor(r.TenantID, "alipay")
			return d, err == nil
		}
	}
	d, err := s.driverFor(s.cfg.DefaultTenantID, "alipay")
	return d, err == nil
}

func genOrderNo() string {
	return fmt.Sprintf("%s%d", time.Now().Format("20060102150405"), idgen.Next()%100000)
}
