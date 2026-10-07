# mp-weixin 端 `@tap="方法名"` 裸引用不触发(全站按钮点击无响应)

| 字段 | 值 |
|---|---|
| KFO 层级 | L3 — 复盘分析层 |
| 日期 | 2026-06-21 |
| 触发 | 联调时大量按钮"点击没反应":我的(真人认证/换头像/客服)、海洋(捞/扔/我的瓶子)、聊天(发送/发图)、详情(打招呼/回信)等 |
| 数据来源 | 微信开发者工具 Console 埋点(`[App]`/`[mine]`/`[test]`)+ 编译产物 wxml/wxss/js 逐层比对 |
| 关联 | L2 `l2/2026-06-21-ocean-experience-redo.md` · L4 `l4/cross-platform-api-patterns.md`(模式 MP-1) · L1 `l1/ui-design-system.md` |

---

## 一、症状

跨多页面、多按钮点击无任何反应。native tabBar 正常,但 `@tap` 绑定的方法处理器普遍不触发。多轮"改 inline 表达式→方法""清缓存""硬化启动流程"均无效,用户反复确认仍坏。

## 二、定位过程(埋点 → 编译比对 → 决定性区分)

1. **加 console 埋点**:`App.onLaunch` ✅、`mine.onLoad` ✅、`mine.onShow profile=…` ✅ → **页面正常加载、登录成功**,但点客服/换头像**无任何 tap 日志** → 事件根本没派发到方法。
2. **查编译产物**:`mine.wxml` 绑定为 `bindtap="{{n}}"`,`mine.js` 里 `n: common_vendor.o(($event)=>…)` 包裹齐全;无遮挡层、版本(7 个 @dcloudio 包)完全一致 → **代码/编译/版本全对**,排除遮挡与版本错配。
3. **决定性证据**:用户实际点击中,**只有 `soon('浏览记录')`、`soon('我的收藏')` 触发了**(带参数的调用表达式),而 `goVerify`/`changeAvatar`/`contact`(裸方法名)全部不触发。

## 三、根因

**这个 uni-app vue3 alpha(`3.0.0-alpha-1000920260615733`)在 mp-weixin 端,模板事件写成裸方法名 `@tap="contact"` 时,编译出的绑定运行时不调用;写成调用表达式 `@tap="soon('x')"` / `@tap="contact($event)"` 才正常触发。**

- 关键认知:这与"inline 表达式不稳"的早期猜测**相反** —— 真相是**调用表达式 OK、裸引用坏**。之前把 `@tap="showReply=true"` 改成裸 `@tap="openReply"` 反而把"打招呼/回信"改坏了,正是这个根因。
- 编译后两者结构最终都是 `common_vendor.o(($event)=>$options.foo(...))`,但裸引用那条在该 alpha 的 mp 运行时不派发(无法靠肉眼从产物区分,只能靠"哪个能点"的实测区分)。

## 四、修复

全项目 36 处裸事件统一改为调用表达式:`@tap="fn"` → `@tap="fn($event)"`(`@click/@change/@confirm/@longpress/@input` 同理,`$event` 兼容需事件参数的 `@change` 等)。

```
perl -0pi -e 's/\@(tap|click|longpress|change|confirm|input)="([A-Za-z_][A-Za-z0-9_]*)"/\@$1="$2(\$event)"/g'
```

验证:产物里 `contact` 等已与可触发的 `soon` 同为 `o(($event)=>$options.x($event))`;实测所有按钮恢复响应。提炼为 L4 模式 MP-1(强制 lint 规则)。

## 五、附带运维教训

同期 `/api/block/list` 报 404:根因是**后端跑的是旧二进制**(`go run` 不热重载),新路由没重启不生效。改 Go 代码后必须重启服务。已登记到 L4。

## 六、复发记录(2026-06-25)

新建的 `pages/privacy/privacy.vue`(相机/水印页)再次写成裸 `@tap="pickAdd"`/`@tap="pickRemove"`,同一根因复发,点击全程无响应,排查绕了多轮(误判 `<button>` 元素、tab-bar 重渲染打断手势、跳转动画挂起 JS)。最终靠**全套触摸埋点 + 编译产物 `privacy.js` 比对**(`g: o(($event)=>$options.pickAdd)` 缺括号 vs `f: o(($event)=>$options.doSave(...))`)锁定,加括号修复。

教训补充:① 该 alpha 的 perl 批量替换只覆盖了当时存量,**新写的代码会再犯**,需在 review/lint 卡点;② 排查此类"hover 有反应、@tap 无反应"问题应**第一时间看 `dist/.../xxx.js` 的 `o(...)` 绑定**,而非盯源码猜。详见 `l2/2026-06-25-camera-page-and-ui-polish.md`。
