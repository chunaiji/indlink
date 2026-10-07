package user

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/config"
	"driftbottle/internal/model"
	"driftbottle/internal/provider"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/tenant"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/cache"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

type Service struct {
	db    *gorm.DB
	wallet *wallet.Service
	cfg   *config.Config
	creds *tenant.Store
	// CheckProfileText 资料文本审核回调(昵称/签名,main 注入 moderation.CheckUGC;nil 时跳过)
	CheckProfileText func(tenantID, userID int64, text string) error
	providers *provider.Store
}

func New(db *gorm.DB, w *wallet.Service, cfg *config.Config, creds *tenant.Store) *Service {
	return &Service{db: db, wallet: w, cfg: cfg, creds: creds}
}

// SetProviders 注入服务商配置(支付宝登录从「支付 → 支付宝」卡片取密钥)。
func (s *Service) SetProviders(ps *provider.Store) { s.providers = ps }

// resolveTenant 根据请求 appid 解析租户与登录凭证。
// 多租户开:查 credstore;关:默认租户 + .env 凭证。
func (s *Service) resolveLogin(platform, appid string) (tenantID int64, wxAppID, wxSecret, aliAppID string, err error) {
	if s.cfg.MultiTenant {
		if platform == "wx" && appid == "" {
			appid = s.cfg.DefaultWxAppID
		}
		r, ok := s.creds.ByAppID(platform, appid)
		if !ok {
			return 0, "", "", "", errs.New(errs.CodeLoginFailed, "未知的小程序 appid,请检查租户凭证配置")
		}
		switch platform {
		case "wx":
			return r.TenantID, r.AppID, r.Secret, "", nil
		case "alipay":
			return r.TenantID, "", "", r.AppID, nil
		}
		return 0, "", "", "", errs.New(errs.CodeBadRequest, "不支持的平台")
	}
	// 单租户:默认租户 + .env
	return s.cfg.DefaultTenantID, s.cfg.WX.AppID, s.cfg.WX.Secret, s.cfg.Alipay.AppID, nil
}

// Login 平台登录:解析租户 → 换 openid → 找/建用户(按 tenant+openid)→ 新用户发奖励。
func (s *Service) Login(platform, appid, code string) (*model.User, bool, error) {
	tenantID, wxAppID, wxSecret, aliAppID, err := s.resolveLogin(platform, appid)
	if err != nil {
		return nil, false, err
	}

	var oauth *oauthResult
	switch platform {
	case "wx":
		oauth, err = wxCode2Session(wxAppID, wxSecret, code)
	case "alipay":
		oauth, err = s.alipayMiniLogin(tenantID, aliAppID, code)
	default:
		return nil, false, errs.New(errs.CodeBadRequest, "不支持的平台")
	}
	if err != nil {
		return nil, false, errs.New(errs.CodeLoginFailed, err.Error())
	}

	// 按 (tenant_id, openid) 查找
	var u model.User
	q := s.db.Where("tenant_id = ?", tenantID)
	if platform == "wx" {
		err = q.First(&u, "wx_openid = ?", oauth.OpenID).Error
	} else {
		err = q.First(&u, "alipay_uid = ?", oauth.OpenID).Error
	}

	if err == nil {
		if u.Status == "banned" {
			return nil, false, errs.New(errs.CodeForbidden, "账号已被封禁，如有疑问请联系客服")
		}
		now := time.Now()
		s.db.Model(&u).Updates(map[string]interface{}{"last_active_at": now, "last_login_at": now})
		return &u, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	// 新建用户(带 tenant_id)
	u = model.User{
		UserID:         idgen.Next(),
		TenantID:       tenantID,
		Nickname:       fmt.Sprintf("用户%06d", idgen.Next()%1000000),
		AnonymousLevel: 1,
		Status:         "active",
		CreatedAt:      time.Now(),
		LastActiveAt:   time.Now(),
		LastLoginAt:    time.Now(),
	}
	if platform == "wx" {
		u.WxOpenID = oauth.OpenID
		u.UnionID = oauth.UnionID
	} else {
		u.AlipayUID = oauth.OpenID
	}
	if err := s.db.Create(&u).Error; err != nil {
		return nil, false, err
	}

	if reward := sysconfig.GetInt64(tenantID, sysconfig.KeyRegReward); reward > 0 {
		_ = s.wallet.Credit(tenantID, u.UserID, reward, wallet.SceneReward, fmt.Sprintf("register:%d", u.UserID))
	}
	return &u, true, nil
}

func (s *Service) Profile(userID int64) (*model.User, error) {
	var u model.User
	if err := s.db.First(&u, "user_id = ?", userID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "用户不存在")
	}
	return &u, nil
}

type UpdateProfileInput struct {
	Nickname *string
	Avatar   *string
	Bio      *string
	Gender   *int8
	Age      *int
	Birthday *string // YYYY-MM-DD;设了就按它重算 Age
	City     *string

	// ---- App 端字段(小程序不传)----
	Language  *string  // 逗号分隔
	Interests *string  // 逗号分隔
	Lat       *float64 // 与 Lng 成对提交才写库
	Lng       *float64
}

// ContentCounts 我扔过的瓶子数 / 发过的动态数。App 自己的资料页靠它显示「我的瓶子 N 个」——
// 以前只有发现页的别人卡片算这两个数,自己的资料恒为 0(真机反馈)。
func (s *Service) ContentCounts(tenantID, userID int64) (bottles, moments int64) {
	s.db.Model(&model.Bottle{}).Where("tenant_id = ? AND user_id = ?", tenantID, userID).Count(&bottles)
	s.db.Model(&model.Moment{}).Where("tenant_id = ? AND user_id = ?", tenantID, userID).Count(&moments)
	return
}

// RelationCounts 关注数(我关注的人) 与粉丝数(关注我的人)。
// 关注复用 Relation.Type=like，没有独立 follow 表（与 moment 的 relationFollow 同源）。
func (s *Service) RelationCounts(tenantID, userID int64) (following, followers int64) {
	s.db.Model(&model.Relation{}).
		Where("tenant_id = ? AND user_a = ? AND type = ?", tenantID, userID, "like").
		Count(&following)
	s.db.Model(&model.Relation{}).
		Where("tenant_id = ? AND user_b = ? AND type = ?", tenantID, userID, "like").
		Count(&followers)
	return
}

func (s *Service) UpdateProfile(userID int64, in UpdateProfileInput) error {
	var cur model.User
	s.db.Select("tenant_id, gender").First(&cur, "user_id = ?", userID)
	// 昵称/签名走内容审核(资料场景)
	if s.CheckProfileText != nil {
		for _, t := range []*string{in.Nickname, in.Bio} {
			if t != nil && *t != "" {
				if err := s.CheckProfileText(cur.TenantID, userID, *t); err != nil {
					return err
				}
			}
		}
	}
	upd := map[string]interface{}{}
	if in.Nickname != nil {
		upd["nickname"] = *in.Nickname
	}
	if in.Avatar != nil {
		upd["avatar"] = *in.Avatar
	}
	if in.Bio != nil {
		upd["bio"] = *in.Bio
	}
	if in.Gender != nil {
		// 性别只能定一次:完善资料时选定,之后不可改(匹配 / 筛选都依赖它)。
		// 同值重发放行;0=未知不算定过。
		if cur.Gender != 0 && *in.Gender != cur.Gender {
			return errs.New(errs.CodeBadRequest, "性别设置后不能再修改")
		}
		upd["gender"] = *in.Gender
	}
	if in.Age != nil {
		upd["age"] = *in.Age
	}
	if in.Birthday != nil {
		if *in.Birthday == "" {
			upd["birthday"] = ""
		} else {
			d, err := time.Parse("2006-01-02", *in.Birthday)
			if err != nil {
				return errs.New(errs.CodeBadRequest, "出生日期格式不对")
			}
			upd["birthday"] = *in.Birthday
			// 年龄由生日推出,后者优先于客户端直接传的 age。
			upd["age"] = ageOf(d, time.Now())
		}
	}
	if in.City != nil {
		upd["city"] = *in.City
	}
	if in.Language != nil {
		upd["language"] = *in.Language
	}
	if in.Interests != nil {
		upd["interests"] = *in.Interests
	}
	// 位置成对更新：只有一个坐标的记录没有意义，还会污染距离筛选。
	if in.Lat != nil && in.Lng != nil {
		upd["lat"], upd["lng"] = *in.Lat, *in.Lng
		// 顺带把城市反查出来。
		//
		// 没有它,App 用户的 users.city 恒为空(只有在资料页手填才有值),
		// 而漂流轨迹的 seen/replied 节点在聚合时会被「城市为空就跳过」直接丢掉
		// ——「在 XXX 捞起瓶子」因此永远不显示。同城筛选同样受益。
		//
		// in.City 非空时以用户手填为准,不覆盖。
		if in.City == nil {
			// 连 tenant_id 一起取:UpdateProfile 的签名里没有租户,
			// 为了一次反查去改所有调用方不值当。
			var cur struct {
				TenantID int64
				City     string
				Lat, Lng float64
			}
			s.db.Model(&model.User{}).Select("tenant_id", "city", "lat", "lng").
				Where("user_id = ?", userID).Scan(&cur)
			if shouldRefreshCity(cur.City, cur.Lat, cur.Lng, *in.Lat, *in.Lng) {
				if city := s.cityAt(cur.TenantID, *in.Lat, *in.Lng); city != "" {
					upd["city"] = city
				}
			}
		}
	}
	if len(upd) == 0 {
		return nil
	}
	return s.db.Model(&model.User{}).Where("user_id = ?", userID).Updates(upd).Error
}

// Verify 提交真人/实名认证(V1 简化)。
func (s *Service) Verify(userID int64) error {
	return s.db.Model(&model.User{}).Where("user_id = ?", userID).Update("is_verified", true).Error
}

// GetAlipayUID 支付宝小程序 trade.create 的 buyer_id。
func (s *Service) GetAlipayUID(userID int64) (string, error) {
	var u model.User
	if err := s.db.Select("alipay_uid").First(&u, "user_id = ?", userID).Error; err != nil {
		return "", err
	}
	return u.AlipayUID, nil
}

// alipayMiniLogin 小程序支付宝登录:有凭证走网关,无凭证开发态派生。
func (s *Service) alipayMiniLogin(tenantID int64, appid, code string) (*oauthResult, error) {
	cli, err := s.alipayClient(tenantID, "alipay")
	if err != nil {
		return nil, err
	}
	if cli == nil || appid == "" {
		return &oauthResult{OpenID: "alidev_" + code}, nil
	}
	return alipayCode2UID(tenantID, cli, code)
}

// GetWxOpenID 供 pay 模块下单使用。
func (s *Service) GetWxOpenID(userID int64) (string, error) {
	var u model.User
	if err := s.db.Select("wx_openid").First(&u, "user_id = ?", userID).Error; err != nil {
		return "", err
	}
	return u.WxOpenID, nil
}

// EnsureVerifiedIfRequired 开关开启时校验认证。
func (s *Service) EnsureVerifiedIfRequired(tenantID, userID int64) error {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyVerifyRequired) {
		return nil
	}
	var u model.User
	if err := s.db.Select("is_verified").First(&u, "user_id = ?", userID).Error; err != nil {
		return err
	}
	if !u.IsVerified {
		return errs.ErrNeedVerify
	}
	return nil
}

// TouchActive 节流刷新最近活跃时间：每个用户最多每 60s 写一次 last_active_at(#增长4/5)。
// SetNX 同步判定(单次 Redis 往返，廉价)；仅在命中去抖窗口边界(约每 60s 一次)时才异步落库，
// 避免每请求都 spawn goroutine + 写库(FF4)。
func (s *Service) TouchActive(tenantID, userID int64) {
	if userID == 0 {
		return
	}
	ctx := context.Background()
	key := fmt.Sprintf("active:%d", userID)
	// SetNX 成功(此前不存在)才落库，TTL 60s 作为去抖窗口
	ok, err := cache.RDB.SetNX(ctx, key, "1", 60*time.Second).Result()
	if err != nil {
		log.Printf("[user] TouchActive redis err uid=%d: %v", userID, err)
		return
	}
	if !ok {
		return
	}
	// 仅此处(约每 60s/人)异步写库，不阻塞请求
	go func() {
		if e := s.db.Model(&model.User{}).Where("user_id = ?", userID).
			Update("last_active_at", time.Now()).Error; e != nil {
			log.Printf("[user] TouchActive db err uid=%d: %v", userID, e)
		}
	}()
}

// ageOf 按生日算周岁:生日当天算满。
func ageOf(birth, now time.Time) int {
	age := now.Year() - birth.Year()
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		age--
	}
	if age < 0 {
		return 0
	}
	return age
}
