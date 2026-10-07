# 企业级 UI 改造 + 组件化

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-20 |
| 状态 | COMPLETED(微信端构建通过) |
| 触发 | 用户反馈"现状 UI 不够企业级",要求统一圆角/字体等细节 |
| 关联 | L1 `l1/ui-design-system.md`(设计系统)· L0 `l0/architecture.md` |
| 实施 | commit `8ebd6a5`(前序令牌改造随同) |

---

## 一、范围

精致化现有社交风,令牌系统 + 全部 9 页面 + 新增 3 个复用组件。详细令牌/组件规范见 `l1/ui-design-system.md`。

## 二、令牌系统(uni.scss / App.vue)

- 新增中性灰阶 ink-900~50、圆角阶梯 `$r-*`、柔阴影 `$shadow-card/pop`、间距 8 栅格。
- 旧变量名做兼容别名 → 页面用变量处**自动**获得精致灰阶/白卡/统一描边(最大杠杆)。
- App.vue 页面底色 `#F4F8F9`,字体栈加 SF/PingFang,行高 1.5。

## 三、全部页面改造

- 圆角统一:卡片 `$r-lg`、控件 `$r-sm`、头像 `$r-avatar`、内容图 `$r-img`(批量 perl 替换 28/32→24、22→12、18→16)。
- 阴影全部令牌化:4 张卡片 `$shadow-card`、4 个 CTA `$shadow-pop`(取代彩色重投影)。
- 字重收敛 800→700;图片统一 `$r-img` + 内描边 + 占位底。

## 四、新增组件(easycom)

| 组件 | 说明 |
|---|---|
| `user-avatar` | 默认头像:昵称首字 + 确定性渐变,替代随机 emoji;有 src 显图 |
| `empty-state` | 统一空状态(图标/标题/描述/按钮),场景化文案 |
| `sk-list` | 列表骨架屏 shimmer,reduce-motion 静止 |

接入:头像 → city/expand/message/mine/detail/chat;空状态 → 列表/详情;骨架 → city/expand(loading 驱动)。

## 五、验证

- `npm run build:mp-weixin` → DONE(组件 easycom 自动导入,全部编译通过)。
- 圆角令牌覆盖率:卡片/头像/按钮/图片基本全令牌化;剩余字面值为胶囊(999)/圆形(50%)/小 chip(本应独立)。

## 六、工程注意

- 误入的 `*.vue.bak`(perl 备份)已删除并加入 `.gitignore`。
- 提交前已核验无 `.env`/`node_modules`/`dist`/`uploads` 入库。

## 七、待办

- 真人头像上传(我的页改头像)
- 夜间主题对新令牌完整适配
- 空状态可替换为统一插画素材(目前 emoji 图标)
