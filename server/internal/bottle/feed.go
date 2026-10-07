package bottle

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"
	"driftbottle/pkg/geodist"

	"gorm.io/gorm"
)

// FeedService 实现"捞瓶"的 Redis 预生成列表 + 去重。
// 设计原则(对齐文档):feed 走缓存,不每次实时查库;命中空时再生成。
type FeedService struct {
	db *gorm.DB
}

func NewFeedService(db *gorm.DB) *FeedService { return &FeedService{db: db} }

const feedBatch = 5 // 每次捞返回条数

// Filter 捞瓶筛选(#7):漂向(同城/全国)+ 作者性别。空=不限。
type Filter struct {
	Scope  string // local / national / ""
	Gender int8   // 0 不限 / 1 男 / 2 女
}

func (ft Filter) sig() string { return fmt.Sprintf("%s-%d", ft.Scope, ft.Gender) }

func feedKey(tenantID, userID int64, sig string) string {
	return fmt.Sprintf("user_feed:%d:%d:%s", tenantID, userID, sig)
}

// Next 返回一批瓶子:先从 Redis feed 取 ID;空则重建后再取。按租户 + 筛选条件隔离缓存。
// key 带昼夜维度:夜场切换后旧池立即失效,深夜瓶不漏到白天(反之亦然)。
func (f *FeedService) Next(tenantID, userID int64, city string, tags []string, ft Filter) ([]model.Bottle, error) {
	ctx := context.Background()

	// 一次取齐浏览者画像:性别/城市供打分,经纬度供距离打分与缓存分桶。
	// 原本 rebuild 里也查一次,合并到这里,查询数不变。
	var viewer model.User
	f.db.Select("user_id, gender, city, lat, lng").First(&viewer, "user_id = ?", userID)
	if city == "" {
		city = viewer.City
	}

	sig := ft.sig()
	if InNightWindow(tenantID) {
		sig += "-n"
	}
	// 位置分桶进缓存键:跨城后立刻换桶、立刻重建,而不必等 10 分钟 TTL 自然过期。
	// 市内移动不换桶,不浪费一次全量重建。
	sig += "-" + geodist.Bucket(viewer.Lat, viewer.Lng)
	key := feedKey(tenantID, userID, sig)

	ids, _ := cache.RDB.LRange(ctx, key, 0, feedBatch-1).Result()
	if len(ids) == 0 {
		if err := f.rebuild(ctx, tenantID, viewer, city, tags, ft); err != nil {
			return nil, err
		}
		ids, _ = cache.RDB.LRange(ctx, key, 0, feedBatch-1).Result()
	}
	if len(ids) == 0 {
		return []model.Bottle{}, nil
	}
	// 弹出已取出的 ID
	cache.RDB.LTrim(ctx, key, int64(len(ids)), -1)

	bottleIDs := make([]int64, 0, len(ids))
	for _, s := range ids {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			bottleIDs = append(bottleIDs, v)
		}
	}
	var list []model.Bottle
	if err := f.db.Where("tenant_id = ? AND bottle_id IN ? AND status = ?", tenantID, bottleIDs, "active").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// distanceScore 距离衰减打分：越近分越高，上限 w，下限 0。
//
// 任一方没有定位就返回 0——**不计分，也不惩罚**。这一点很重要：
//   - 存量瓶子的经纬度是 0,若按「距离极远」处理,老瓶子会被永久压到池底
//   - 浏览者拒绝定位时,整个距离维度消失,其余维度照常工作(与 Z6 的降级原则一致)
//
// 衰减用 1/(1+d/halfKM)：在 halfKM 处得半分，无拐点、不会为负，
// 比线性截断更适合「同城很近」与「隔壁城市」之间的平滑过渡。
func distanceScore(viewerLat, viewerLng, bLat, bLng, w float64) float64 {
	if !geodist.HasFix(viewerLat, viewerLng) || !geodist.HasFix(bLat, bLng) {
		return 0
	}
	const halfKM = 25.0
	d := geodist.KM(viewerLat, viewerLng, bLat, bLng)
	return w / (1 + d/halfKM)
}

// rebuild 生成候选池并按分值排序写入 Redis。
// 精准匹配(#8):score = wTag·标签重合 + wCity·同城 + wDist·距离 + wGender·异性 + wFresh·新鲜度
//
//   - wHeat·热度 − wRobot·机器人 + wRandom·随机扰动;权重走 sysconfig(运营可调)。
//
// 候选已排除:自己、已看过(match_log)、被自己拉黑的作者。
// viewer 由 Next 加载后传入(含 lat/lng),此处不再重复查库。
func (f *FeedService) rebuild(ctx context.Context, tenantID int64, viewer model.User, city string, tags []string, ft Filter) error {
	userID := viewer.UserID
	size := int(sysconfig.GetInt64(tenantID, sysconfig.KeyFeedSize))
	if size <= 0 {
		size = 50
	}
	tagSet := f.viewerTags(userID, tags)

	// 候选:本租户、active、未过期、非自己、未看过、未被拉黑
	var candidates []model.Bottle
	seen := f.db.Model(&model.MatchLog{}).Select("bottle_id").Where("viewer_id = ?", userID)
	blocked := f.db.Model(&model.Block{}).Select("target_id").Where("user_id = ?", userID)
	night := InNightWindow(tenantID) // 深夜瓶只在夜场时段可被捞;夜场内置顶优先
	q := f.db.Where("tenant_id = ? AND status = ? AND expire_at > ? AND user_id <> ?", tenantID, "active", time.Now(), userID).
		Where("bottle_id NOT IN (?)", seen).
		Where("user_id NOT IN (?)", blocked)
	if !night {
		q = q.Where("tags NOT LIKE ?", "%night%")
	}
	// #7 筛选:同城(硬过滤当前城市)/ 作者性别
	if ft.Scope == "local" && city != "" {
		q = q.Where("city = ?", city)
	}
	if ft.Gender > 0 {
		genderSub := f.db.Model(&model.User{}).Select("user_id").Where("gender = ?", ft.Gender)
		q = q.Where("user_id IN (?)", genderSub)
	}
	q = q.Order("heat_score desc, created_at desc").Limit(size * 3) // 多取一些,打分后截断
	if err := q.Find(&candidates).Error; err != nil {
		return err
	}
	// 兜底：若无未过期瓶子，忽略 expire_at 再查一次（已扔但过期的瓶子优先于空海面）
	if len(candidates) == 0 {
		qFallback := f.db.Where("tenant_id = ? AND status = ? AND user_id <> ?", tenantID, "active", userID).
			Where("bottle_id NOT IN (?)", seen).
			Where("user_id NOT IN (?)", blocked)
		if !night {
			qFallback = qFallback.Where("tags NOT LIKE ?", "%night%")
		}
		if ft.Scope == "local" && city != "" {
			qFallback = qFallback.Where("city = ?", city)
		}
		if ft.Gender > 0 {
			genderSub := f.db.Model(&model.User{}).Select("user_id").Where("gender = ?", ft.Gender)
			qFallback = qFallback.Where("user_id IN (?)", genderSub)
		}
		qFallback = qFallback.Order("heat_score desc, created_at desc").Limit(size * 3)
		if err := qFallback.Find(&candidates).Error; err != nil {
			return err
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	// 批量取候选作者的 性别 / 是否机器人(避免 N+1)
	authors := f.authorMeta(candidates)

	// 权重(运营在管理台可调)
	wTag := float64(sysconfig.GetInt(tenantID, sysconfig.KeyMatchWTag))
	wCity := float64(sysconfig.GetInt(tenantID, sysconfig.KeyMatchWCity))
	wGender := float64(sysconfig.GetInt(tenantID, sysconfig.KeyMatchWGender))
	wFresh := float64(sysconfig.GetInt(tenantID, sysconfig.KeyMatchWFresh))
	wHeat := float64(sysconfig.GetInt(tenantID, sysconfig.KeyMatchWHeat))
	wRobot := float64(sysconfig.GetInt(tenantID, sysconfig.KeyMatchWRobot))
	wRandom := float64(sysconfig.GetInt(tenantID, sysconfig.KeyMatchWRandom))
	wDist := float64(sysconfig.GetInt(tenantID, sysconfig.KeyMatchWDist))

	type scored struct {
		id    int64
		score float64
	}
	maxHeat := 1
	for _, b := range candidates {
		if b.HeatScore > maxHeat {
			maxHeat = b.HeatScore
		}
	}
	now := time.Now()

	arr := make([]scored, 0, len(candidates))
	for i, b := range candidates {
		var s float64
		// 标签重合度(命中数 / 浏览者标签数,0~1)
		if len(tagSet) > 0 {
			hit := 0
			for _, bt := range strings.Split(b.Tags, ",") {
				if tagSet[strings.TrimSpace(strings.TrimPrefix(bt, "#"))] {
					hit++
				}
			}
			if hit > 0 {
				s += wTag * float64(hit) / float64(len(tagSet))
			}
		}
		// 同城
		if city != "" && b.City == city {
			s += wCity
		}
		// 距离衰减:双方都有经纬度才生效,否则不计分也不惩罚
		s += distanceScore(viewer.Lat, viewer.Lng, b.Lat, b.Lng, wDist)
		// 异性优先(双方性别已知且相异)
		a := authors[b.UserID]
		if viewer.Gender > 0 && a.gender > 0 && a.gender != viewer.Gender {
			s += wGender
		}
		// 新鲜度:7 天内线性衰减(0~1)
		ageH := now.Sub(b.CreatedAt).Hours()
		fresh := 1 - ageH/168
		if fresh < 0 {
			fresh = 0
		}
		s += wFresh * fresh
		// 热度归一
		s += wHeat * float64(b.HeatScore) / float64(maxHeat)
		// 机器人惩罚
		if a.isRobot {
			s -= wRobot
		}
		// 夜场时段深夜瓶置顶(boost 远大于各权重和,保证排最前)
		if night && strings.Contains(b.Tags, "night") {
			s += 10000
		}
		// 随机扰动(用雪花位+索引,避免依赖 rand/Date,且对同一用户稳定到下次刷新)
		s += wRandom * float64((b.BottleID^int64(i)*2654435761)&0xffff) / 65535.0
		arr = append(arr, scored{id: b.BottleID, score: s})
	}
	// 插入排序按 score 降序(候选量受 size*3 限制,成本可控)
	for i := 1; i < len(arr); i++ {
		for j := i; j > 0 && arr[j].score > arr[j-1].score; j-- {
			arr[j], arr[j-1] = arr[j-1], arr[j]
		}
	}

	// ⚠️ 必须和 Next 里算 key 的方式**完全一致**(含位置分桶),
	// 否则写入的 key 与读取的 key 对不上,缓存永远落空、每次都全量重建。
	sig := ft.sig()
	if night {
		sig += "-n"
	}
	sig += "-" + geodist.Bucket(viewer.Lat, viewer.Lng)
	key := feedKey(tenantID, userID, sig)
	pipe := cache.RDB.Pipeline()
	pipe.Del(ctx, key)
	vals := make([]interface{}, 0, len(arr))
	for k, sc := range arr {
		if k >= size {
			break
		}
		vals = append(vals, strconv.FormatInt(sc.id, 10))
	}
	if len(vals) > 0 {
		pipe.RPush(ctx, key, vals...)
		pipe.Expire(ctx, key, 10*time.Minute) // feed 10 分钟过期,定期刷新
	}
	_, err := pipe.Exec(ctx)
	return err
}

// viewerTags 组合浏览者兴趣标签:入参标签 + 自己最近发过的瓶子标签。
func (f *FeedService) viewerTags(userID int64, tags []string) map[string]bool {
	set := map[string]bool{}
	for _, t := range tags {
		if t = strings.TrimSpace(strings.TrimPrefix(t, "#")); t != "" {
			set[t] = true
		}
	}
	var mine []string
	f.db.Model(&model.Bottle{}).Where("user_id = ? AND tags <> ''", userID).
		Order("created_at desc").Limit(20).Pluck("tags", &mine)
	for _, row := range mine {
		for _, t := range strings.Split(row, ",") {
			if t = strings.TrimSpace(strings.TrimPrefix(t, "#")); t != "" {
				set[t] = true
			}
		}
	}
	return set
}

type authorInfo struct {
	gender  int8
	isRobot bool
}

// authorMeta 批量取候选作者的性别/是否机器人,避免逐条查询。
func (f *FeedService) authorMeta(candidates []model.Bottle) map[int64]authorInfo {
	idset := map[int64]bool{}
	for _, b := range candidates {
		idset[b.UserID] = true
	}
	ids := make([]int64, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	out := map[int64]authorInfo{}
	if len(ids) == 0 {
		return out
	}
	type row struct {
		UserID  int64
		Gender  int8
		IsRobot bool
	}
	var rows []row
	f.db.Model(&model.User{}).Select("user_id, gender, is_robot").Where("user_id IN ?", ids).Scan(&rows)
	for _, r := range rows {
		out[r.UserID] = authorInfo{gender: r.Gender, isRobot: r.IsRobot}
	}
	return out
}
