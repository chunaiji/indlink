# L1 设计 — 企业级 UI 设计系统(前端)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-06-21 |
| 覆盖 | `client/src/uni.scss`(令牌)、`client/src/components/*`(组件)、全部页面样式 |
| 关联 | L2 `l2/2026-06-20-enterprise-ui-refactor.md` · `l2/2026-06-21-ocean-experience-redo.md` · L4 `l4/cross-platform-api-patterns.md`(MP-1/MP-2) · L3 `l3/2026-06-21-mp-tap-not-binding.md` · L0 `l0/architecture.md` |

---

## 一、设计方向

精致化"海洋社交"风:保留温暖产品调性,用**系统化一致性**(统一圆角、专业字阶、中性灰阶、克制阴影)而非更多装饰,达到企业级观感。唯一"放胆色"= 主 CTA 珊瑚。

## 二、设计令牌(`uni.scss`,全局注入)

```
中性灰阶(精致感来源):ink-900 #0C2A33 / 600 #4A6670 / 400 #8AA0A8 / 200 #D6E2E6 / 50 #F4F8F9
品牌色:珊瑚 #FF6B5B(唯一 CTA)· 浅海青 #4FB8D4 · 日光金 #FFC23C(金币)
语义色:success #2BB673 / warning #FFB020 / danger #F2543F
圆角阶梯:$r-xs 8 / $r-sm 12(控件) / $r-md 16 / $r-lg 24(卡片) / $r-img 16(内容图) / $r-avatar 24(头像) / $r-full 999
阴影:$shadow-card 0 2rpx 12rpx rgba(12,42,51,.06) · $shadow-pop 0 8rpx 32rpx rgba(12,42,51,.12)
间距:8 栅格 $sp-1..5(8/16/24/32/48);页边距统一 32rpx
```
兼容别名:旧变量名 `$ink/$ink-soft/$ink-faint/$line/$card/$ground` 映射到新灰阶/白卡,故页面用变量处自动获得新观感。

## 三、字体系统(`App.vue` 全局)

```
字体栈:-apple-system, "SF Pro Text", "PingFang SC", "HarmonyOS Sans SC", "Microsoft YaHei"
字阶(rpx):display 44/700 · h1 36/600 · h2 30/600 · body 28/400 · sub 26 · caption 22
原则:字重收敛(≤700,正文 400 强调 500/600);次要信息用灰阶分层而非加粗;大数字 tabular-nums 对齐。
```

## 四、组件库(`components/`,easycom 自动导入)

| 组件 | 用途 | 关键 props |
|---|---|---|
| `user-avatar` | 默认头像:有图显图,无图用**昵称首字 + 确定性渐变**(8 色,按名字 hash) | name / src / size(rpx) / shape(square·circle) |
| `empty-state` | 统一空状态:图标+标题+描述+可选按钮 | icon / title / desc / action(emit) |
| `sk-list` | 列表骨架屏(shimmer,reduce-motion 静止) | rows |

接入:user-avatar → city/expand/message/mine/detail/chat;empty-state → 同上列表/详情;sk-list → city/expand(loading 驱动)。

## 五、图片规范(细节)

所有内容图(瓶子图/动态图/聊天图/缩略图):统一 `$r-img` 16rpx + `mode="aspectFill"` + 1rpx 内描边 `rgba(12,42,51,.06)` + 占位底 `ink-50`,防白底图"融底"与变形。

## 六、约定

- 卡片 = 白底 + `$line` 1rpx 描边 + `$shadow-card` + `$r-lg`;CTA = 珊瑚渐变 + `$shadow-pop` + `$r-sm/$r-md`。
- 头像统一 `$r-avatar` 超椭圆;胶囊/圆点用 `$r-full`。
- 双主题:夜间复用同令牌,仅切背景与文字色(`page.theme-dark`)。

## 七、MP 端模板硬约束(L4 回灌,见 `l4/cross-platform-api-patterns.md` MP-1/MP-2)

- **事件一律调用表达式**:`@tap/@click/@change/@confirm/@longpress/@input` 必须写 `fn($event)`,**禁止裸方法名 `@tap="fn"`**(本 uni vue3 alpha 在 mp 端裸引用不触发;根因见 `l3/2026-06-21-mp-tap-not-binding.md`)。审阅模板时当 lint 红线。
- **emoji 只用老码点**(≤Emoji 6.0):🌊💬❤️🔒🎁🍾🎣 等;避免 Emoji 12+(🪙🪣)在微信旧 WebView 显示豆腐块;"币/价格"用纯文本或 CSS。
- **静态素材 ASCII 文件名**:`static/` 下图片用 `bottle-l.png` 这类名,勿用中文名(mp 端路径易失效)。

## 八、海洋页交互组件(2026-06-21,见 `l2/2026-06-21-ocean-experience-redo.md`)

- **信纸弹框**(捞瓶展示):叠纸 + 横线纸 + 正文 `scroll-view` 限高 `44vh`(长文必须可滚,头尾固定)。
- **写纸条投海弹框**(扔瓶):`curl`/`intoBottle`/`toss` 三段 CSS 关键帧;打捞动画用 `cast`/`ripple`/`rise`。动画进行中锁交互。
- 漂浮瓶子用真实照片(`bottle-l/r.png`)按奇偶交替朝向 + `bob` 浮动。

## 九、待办

- 写纸条弹框暂仅文字瓶;图片瓶待在弹框内补"贴图"入口(旧 throw 页已下线)。
- 灯塔/瓶子为 CSS+小图近似,等透明底 PNG 素材可拉满还原度。
- 夜间主题对新令牌的完整适配。
