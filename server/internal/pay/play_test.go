package pay

import (
	"testing"

	"driftbottle/internal/model"
)

// pickPrice 从多地区价目里选出该平台该地区的价格。
//
// 「后台为准 + 留多市场结构」这条决策的落点。三条规则：
//   - 精确命中 (platform, region) 优先
//   - 命中不到就落该平台的 "*" 兜底行
//   - 平台不匹配的行**绝不能选**——iOS 与 Android 定价本就不同
//     (两家抽成都要在定价里吃掉),选错等于按错误的价格校验实付金额
func TestPickPrice(t *testing.T) {
	prices := []model.CoinPackagePrice{
		{PackageID: 3, Platform: "gplay", Region: "IN", Currency: "INR", Amount: 19900},
		{PackageID: 3, Platform: "gplay", Region: "*", Currency: "USD", Amount: 299},
		{PackageID: 3, Platform: "ios", Region: "IN", Currency: "INR", Amount: 24900},
	}

	t.Run("精确命中地区", func(t *testing.T) {
		got := pickPrice(prices, "gplay", "IN")
		if got == nil || got.Amount != 19900 || got.Currency != "INR" {
			t.Fatalf("应命中 gplay/IN, 实际 %+v", got)
		}
	})

	t.Run("未命中地区时落 * 兜底", func(t *testing.T) {
		got := pickPrice(prices, "gplay", "US")
		if got == nil || got.Amount != 299 || got.Currency != "USD" {
			t.Fatalf("应落 gplay/*, 实际 %+v", got)
		}
	})

	t.Run("不同平台同地区必须各取各的", func(t *testing.T) {
		g := pickPrice(prices, "gplay", "IN")
		i := pickPrice(prices, "ios", "IN")
		if g == nil || i == nil {
			t.Fatal("两个平台都应命中")
		}
		if g.Amount == i.Amount {
			t.Errorf("两端定价本就不同,不应取到同一个值: %d", g.Amount)
		}
	})

	t.Run("平台没有任何价目时返回 nil，不要回退到别的平台", func(t *testing.T) {
		if got := pickPrice(prices, "wx", "IN"); got != nil {
			t.Errorf("wx 没有价目应返回 nil, 实际 %+v", got)
		}
	})

	t.Run("空价目表返回 nil", func(t *testing.T) {
		if got := pickPrice(nil, "gplay", "IN"); got != nil {
			t.Errorf("应返回 nil, 实际 %+v", got)
		}
	})
}
