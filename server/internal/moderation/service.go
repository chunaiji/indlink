package moderation

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"driftbottle/internal/common/alipay"
	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/provider"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// Service 负责内容审核(敏感词、防引流、微信内容安全 API)与举报/拉黑。
// 文本 UGC 场景一律调 CheckUGC(本地词库 + 微信 msgSecCheck);图片走 CheckImageAsync,见 wxcheck.go。
type Service struct {
	db        *gorm.DB
	sensitive []string
	// 微信内容安全依赖(SetWxSource 注入;未注入时仅本地词库)
	tokenFn    func(tenantID int64) (string, error)
	uploadDir  string
	publicBase string

	// 服务商配置(后台「服务商 → 内容安全」单选)与支付宝客户端工厂(复用支付配置里的支付宝应用)
	providers    *provider.Store
	alipayClient func(tenantID int64) (*alipay.Client, error)
}

// Checker 一家内容安全服务商。
//
// Probe 与 CheckText 是**两件事**:CheckText 对 API 故障一律放行(本地词库已兜底,
// 不能因为外部接口抖一下就挡住用户发帖);Probe 要把错误原样抛出来,否则后台
// 「测试连通」在 appid 填错时也亮绿灯,运营照着绿灯上线,审核那天才发现没开。
type Checker interface {
	CheckText(tenantID, userID int64, scene int, text string) error
	CheckImageAsync(tenantID, userID int64, mediaURL string)
	Probe(tenantID int64) error
}

// SetProviders 注入服务商配置与支付宝客户端工厂(main 装配)。
func (s *Service) SetProviders(ps *provider.Store, alipayClient func(int64) (*alipay.Client, error)) {
	s.providers, s.alipayClient = ps, alipayClient
}

// checkerFor 当前生效的服务商。没有生效行 = 只走本地词库。
func (s *Service) checkerFor(tenantID int64) (Checker, *provider.Resolved, bool) {
	if s.providers == nil {
		return nil, nil, false
	}
	row, ok := s.providers.Active(tenantID, provider.KindModeration)
	if !ok {
		return nil, nil, false
	}
	switch row.Provider {
	case "wechat":
		return &wechatChecker{s: s}, row, true
	case "alipay":
		return &alipayChecker{s: s}, row, true
	}
	return nil, nil, false
}

// CheckUGC 统一 UGC 文本审核:本地词库/防引流 → (服务商开着文本检测时)在线检测。
// 所有用户发布文本的场景都应调用本方法而非 CheckText。
func (s *Service) CheckUGC(tenantID, userID int64, scene int, text string) error {
	if err := s.CheckText(text); err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	ck, row, ok := s.checkerFor(tenantID)
	if !ok || !row.Bool("text_on") {
		return nil
	}
	return ck.CheckText(tenantID, userID, scene, text)
}

// CheckImageAsync C 端上传成功后调;服务商开着图片检测时交给它。
func (s *Service) CheckImageAsync(tenantID, userID int64, mediaURL string) {
	if mediaURL == "" {
		return
	}
	ck, row, ok := s.checkerFor(tenantID)
	if !ok || !row.Bool("image_on") {
		return
	}
	ck.CheckImageAsync(tenantID, userID, mediaURL)
}

// ProbeText 后台「测试连通」:真的打一次服务商,失败把原始错误透出来。
func (s *Service) ProbeText(tenantID int64) error {
	ck, _, ok := s.checkerFor(tenantID)
	if !ok {
		return errors.New("没有生效的内容安全服务商")
	}
	return ck.Probe(tenantID)
}

// 防引流:屏蔽微信号/QQ/手机号等外部联系方式。
var (
	rePhone  = regexp.MustCompile(`1[3-9]\d{9}`)
	reQQ     = regexp.MustCompile(`(?i)(qq|扣扣|企鹅)[^\d]{0,4}\d{5,12}`)
	reWechat = regexp.MustCompile(`(?i)(微信|weixin|wechat|vx|v信|薇信)[^\w]{0,4}[a-zA-Z0-9_-]{5,}`)
)

func New(db *gorm.DB) *Service {
	return &Service{
		db: db,
		// V1 占位词表,生产从配置/词库加载
		sensitive: []string{"赌博", "代孕", "枪支", "毒品", "诈骗"},
	}
}

// CheckText 返回 nil 表示通过;命中违规返回 errs.ErrContentBlock。
func (s *Service) CheckText(text string) error {
	low := strings.ToLower(text)
	for _, w := range s.sensitive {
		if strings.Contains(low, strings.ToLower(w)) {
			return errs.ErrContentBlock
		}
	}
	if rePhone.MatchString(text) || reQQ.MatchString(text) || reWechat.MatchString(text) {
		return errs.ErrContentBlock
	}
	return nil
}

// Report 提交举报。
func (s *Service) Report(tenantID, reporterID, targetID int64, targetType, reason string) error {
	return s.db.Create(&model.Report{
		ReportID: idgen.Next(), TenantID: tenantID, ReporterID: reporterID, TargetID: targetID,
		TargetType: targetType, Reason: reason, Status: "pending", CreatedAt: time.Now(),
	}).Error
}

// Block 拉黑(幂等)。
func (s *Service) Block(tenantID, userID, targetID int64) error {
	var cnt int64
	s.db.Model(&model.Block{}).Where("user_id = ? AND target_id = ?", userID, targetID).Count(&cnt)
	if cnt > 0 {
		return nil
	}
	return s.db.Create(&model.Block{
		ID: idgen.Next(), TenantID: tenantID, UserID: userID, TargetID: targetID, CreatedAt: time.Now(),
	}).Error
}

// Blocked 返回当前用户拉黑的用户(精简卡片)。
type BlockedCard struct {
	UserID   int64  `json:"user_id,string"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	City     string `json:"city"`
}

func (s *Service) Blocked(userID int64) ([]BlockedCard, error) {
	var ids []int64
	if err := s.db.Model(&model.Block{}).Where("user_id = ?", userID).Pluck("target_id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []BlockedCard{}, nil
	}
	var users []model.User
	if err := s.db.Where("user_id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]BlockedCard, 0, len(users))
	for _, u := range users {
		out = append(out, BlockedCard{UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar, City: u.City})
	}
	return out, nil
}

// Unblock 解除拉黑。
func (s *Service) Unblock(userID, targetID int64) error {
	return s.db.Where("user_id = ? AND target_id = ?", userID, targetID).Delete(&model.Block{}).Error
}

// IsBlocked 判断 a、b 之间是否存在任一方向的拉黑。
func (s *Service) IsBlocked(a, b int64) bool {
	var cnt int64
	s.db.Model(&model.Block{}).
		Where("(user_id = ? AND target_id = ?) OR (user_id = ? AND target_id = ?)", a, b, b, a).
		Count(&cnt)
	return cnt > 0
}
