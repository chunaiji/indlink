# App 全页面对齐原型 + 小米 9 真机四轮返工

> 2026-10-01 ～ 2026-10-04 · L2 · 关联 [l1/ui-design-system](../l1/ui-design-system.md)
> 原始文档:`docs/superpowers/plans/2026-10-01-app-prototype-alignment.md`(含返工清单)·
> `docs/superpowers/specs/2026-09-30-app-layout-adaptation-design.md`(布局适配规范)·
> `docs/prototype/v2-screens.html`(原型 53 屏)
> 状态:**COMPLETED**(Phase 0–8 全部落地;返工清单仅剩「补差折扣」一项未立项)

把 `app/bottles` 从「看起来像原型」做成「按 pt 量得出来像原型」。核心不是逐页改样式,而是先造
比对工具与共享组件,再让每页只做登记。

## 1. Phase 0:让「像不像」可被测量

| 工具 | 作用 |
|---|---|
| `tool/proto_render.py` | 用无头浏览器把原型 HTML 按 393×851 渲染成 PNG,得到**真实 pt 尺寸**而不是肉眼估 |
| `tool/compare.py` | 原型图与真机截图并排输出 `效果图/compare_<屏>.png`,偏差 > 4pt 的记进返工清单 |
| `test/layout/matrix.dart` | 设备矩阵 helper,全部页面跑一遍窄屏/短屏,溢出即失败 |

> 踩坑:`proto_render` 在 Edge 上会挂住(`da637b9`),渲染器要显式指定通道。

## 2. Phase 1:共享组件先对齐,页面只做登记

阴影/渐变先收进 tokens(`94d5bf4`),再逐个把按钮(`.jn/.ab/.hi` 三套度量)、导航栏/头图/底部操作栏、
Tab 栏、列表行/四宫格/横幅、标签与分段/金币/头像环、弹层与模态(统一走 `showAppSheet`)、
状态视图对到原型。新增组件:`Shadows` `HeaderStats` `QuadGrid` `SelectField` `RadioRow`
`StatusTag` `OrderIdChip` `PackageTile` `CheckInStrip` `MessageBubble`。

**顺序是关键**:组件层改动会同时影响所有页面,所以 Phase 0→1 一口气做完才让用户装机,否则反复看。

## 3. Phase 2:`DesignCanvas` 与海面

装饰性场景(海面、灯塔、瓶子)不按 flex 布局,改为**锚在原型画布坐标系**上等比缩放 —— 新增
`DesignCanvas`(`d7cf102`),海面是它的首个案例(`3bacc6e`)。后续夜/黄昏态、瓶子美术都挂在这套坐标上。

## 4. 真机四轮返工(小米 9)

工具能测出 pt 差,测不出「挡住了」「点不到」「贴边了」。四轮装机反馈共修 20 余项,其中**沉淀成规则**的:

| # | 现象 | 根因 | 修法 |
|---|---|---|---|
| 1 | 六个二级页都带着 Tab 栏 | 子路由在 `StatefulShellRoute` 分支里渲染 | 子路由一律 `parentNavigatorKey: _rootKey`,`router_test.dart` 守 |
| 2 | 胶囊标签撑成整宽 | `Container(alignment:)` 在 `Wrap` 里会撑满 | `Center(widthFactor: 1)` 收缩包裹 |
| 3 | 「发布」「全部已读」没贴右沿 | NavBar 把 actions 放进 `Flexible`,Row 先均分 | actions 按内容定宽(上限 35%/45%)+ `FittedBox`,**标题是唯一让步的** |
| 4 | 底栏骑在手势条上 / 列表最后一项贴屏底 | 底部内边距用 `max(安全区, s4)` | 一律 `s4 + 安全区`;列表页用 `context.pagePadding()` |
| 5 | 我的页钱包卡盖住关注/粉丝/魅力 | `Positioned(bottom: 0)` 让上探量随卡高变 | 占位 + `bottom: 39`,上探恒为 39 |

后两条已回灌进布局适配规范(`f1d9993`)。

## 5. 连带修掉的功能缺陷

真机轮次里暴露、属于业务而非样式的,一并修了:改密路由在已登录时黑屏、注册问生日而非年龄
(最小年龄取 `app_profile_min_age`,不写死)、爱心按钮可点赞(只有滑动才翻卡)、钱包/额度/背包
provider 按登录用户重建(换账号看到上一个人的背包)。

## 6. 验证

`flutter analyze` 干净;`flutter test` 设备矩阵覆盖全部页面(`fc773dc`)。
`test/widget_test.dart` 的「App boots into the login screen」为既有 flaky,与本轮无关。
