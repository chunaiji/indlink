package pay

// Google Play Billing 的记账侧。
//
// 与 playauth.go 的分工：那边负责「怎么跟 Google 说话」（凭证、token、HTTP），
// 这边负责「说完之后怎么记账」。iap.go 把两件事放在一个文件里已经 367 行，
// 不必复制这个问题。

import "driftbottle/internal/model"

// pickPrice 选出该平台该地区的价格：精确命中优先，落不到则取该平台的 "*" 兜底。
//
// **绝不跨平台回退**：iOS 与 Android 的定价本就不同（两家的抽成都要在定价里吃掉），
// 拿 iOS 的价去校验 Play 的实付金额，结果必然是「金额不一致」而拒收所有正常购买。
func pickPrice(prices []model.CoinPackagePrice, platform, region string) *model.CoinPackagePrice {
	var fallback *model.CoinPackagePrice
	for i := range prices {
		p := &prices[i]
		if p.Platform != platform {
			continue
		}
		if p.Region == region {
			return p
		}
		if p.Region == "*" {
			fallback = p
		}
	}
	return fallback
}
