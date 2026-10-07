# 小程序权限补全 & UI 细节打磨

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-24 |
| 状态 | COMPLETED + DEPLOYED |
| 触发 | 真机头像选取静默失败；聊天/列表头像方形；价格硬编码 |
| 关联 | L1 `l1/mp-permissions.md` · L1 `l1/user-auth.md` · L1 `l1/robot.md` |

---

## 一、微信隐私授权框架（chooseAvatar 真机失败）

**根本原因**：Android 真机日志 `ENOENT: stat '.../privacy/scopeState.txt'` + `errno:101`，微信 2023 年 9 月起强制隐私框架，`chooseAvatar` 未获授权时静默失败，`onChooseAvatar` 事件不触发。

**修复**：
1. `manifest.json` 加 `"__usePrivacyCheck__": true`；`requiredPrivateInfos` 只保留 `["getLocation"]`（`chooseAvatar` 不能放此字段，微信仅支持位置类 API）
2. `App.vue` `onLaunch` 注册 `wx.onNeedPrivacyAuthorization` 全局回调
3. ⚠️ **待办**：MP 后台「用户隐私保护指引」需手动勾选：用户头像、用户昵称、精确地理位置、相册

**chooseAvatar URL 处理**：
- WeChat CDN URL（`https://thirdwx.qlogo.cn/...`）→ 直接存储，不上传（避免 downloadFile 域名白名单问题）
- 本地临时路径 → `uni.uploadFile` 上传服务器

---

## 二、UI 圆形头像

| 页面 | 文件 | 修改 |
|---|---|---|
| 聊天室消息气泡 | `pages/chat/chat.vue` | `shape="circle"` + `.av` CSS `border-radius: 10rpx → 50%` |
| 聊天列表行 | `pages/message/message.vue` | `shape="circle"` |
| 我的瓶子回复 | `pages/mybottles/mybottles.vue` | emoji 占位换 `<user-avatar>` 显示真实 `r.avatar` / `r.nickname` |

---

## 三、回信弹框键盘适配

**问题**：`detail.vue` 回信底部弹框被键盘遮住。

**修复**：
- `reply-box` 改为 `position: absolute; bottom: 0`
- textarea 加 `adjust-position="false"` + `@focus` 监听键盘高度
- 动态绑定 `:style="keyboardH ? { bottom: keyboardH + 'px' } : {}"`

---

## 四、同城开聊按钮心跳动画

`city.vue` `.chatbtn` 加 `@keyframes heartbeat`：
- 双跳节奏（12% 主跳 scale 1.1 → 24% 回落 → 36% 余跳 scale 1.08 → 50% 回落 → 静止）
- `.free`（已开聊）`animation: none`

---

## 五、price_chat 从后端配置下发

**问题**：`city.vue` `chatPrice: 5` 硬编码，管理台改配置不生效。

**修复**：
- `user/handler.go` login 响应加 `"price_chat": sysconfig.GetInt(KeyPriceChat)`
- `user/handler.go` profile 响应改为 `gin.H{ user, price_chat }`（兼容 silentLogin 走 fetchProfile 路径）
- `store/user.js` state 加 `chatPrice: 5`，login/fetchProfile 时同步更新
- `city.vue` `chatPrice` 改为 computed 从 store 读取

---

## 六、数据库空头像补全

SSH 执行一次性脚本，对 60 个空头像用户随机分配 `uploads/robot/` 中的头像 URL（`https://ambertu.com/message/static/robot/xxx.jpg`）。
