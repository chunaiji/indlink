# L1 设计 — 微信小程序权限 & 隐私声明

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-06-24 |
| 覆盖模块 | `client/src/manifest.json` · `client/src/App.vue` · `client/src/pages/profile-edit/` · `client/src/utils/` |
| 关联 | L1 `l1/user-auth.md` · L1 `l1/chat-ws.md` · L2 `l2/2026-06-24-mp-permissions-and-ui-polish.md` |

---

## 一、manifest.json 已声明权限

```json
"permission": {
  "scope.userLocation": {
    "desc": "用于同城漂流瓶与附近推荐"
  }
},
"requiredPrivateInfos": ["getLocation"],
"__usePrivacyCheck__": true
```

| 声明项 | 对应 API | 用途 |
|---|---|---|
| `scope.userLocation` | `uni.getLocation` | 同城匹配与附近推荐 |
| `requiredPrivateInfos: getLocation` | `uni.getLocation` | 精确位置(微信新要求) |
| `__usePrivacyCheck__: true` | 全局 | 开启隐私授权框架 |

> ⚠️ `chooseAvatar` **不能**放入 `requiredPrivateInfos`，该字段只接受地理位置相关 API。`chooseAvatar` 的隐私授权通过 `wx.onNeedPrivacyAuthorization` 全局回调处理，无需在此声明。

---

## 二、微信 MP 后台「用户隐私保护指引」待声明

路径：**公众平台 → 小程序 → 设置 → 隐私相关 → 用户隐私保护指引**

| 数据类型 | 用途 | 对应 API | 状态 |
|---|---|---|---|
| 用户头像 | 设置个人资料头像 | `open-type="chooseAvatar"` | ⚠️ 需后台勾选 |
| 用户昵称 | 设置个人资料昵称 | `<input type="nickname">` | ⚠️ 需后台勾选 |
| 精确地理位置 | 同城匹配、附近推荐 | `uni.getLocation` | ⚠️ 需后台勾选 |
| 相册（仅读取） | 上传头像、发送图片消息 | `uni.chooseImage` | ⚠️ 需后台勾选 |

> **关键约束**：后台未配置以上项目时，对应 API 在 Android 真机上静默失败（不弹错误、不触发回调）。这是微信 2023 年 9 月起强制执行的隐私框架行为。

---

## 三、无需 manifest 声明的能力

| 功能 | API | 说明 |
|---|---|---|
| 相册 / 拍照 | `uni.chooseImage` | 系统弹框自动授权，无需 scope 声明 |
| 订阅消息 | `wx.requestSubscribeMessage` | 用户主动授权，无需 scope |
| 微信支付 | `uni.requestPayment` | 通过微信支付能力开通，无需 scope |
| 短振动 | `uni.vibrateShort` | 无需任何授权 |
| WebSocket | `wx.connectSocket` | 无需权限声明 |

---

## 四、各权限触发时机与代码位置

### 地理位置

- **触发**：进入「同城」Tab 或「海洋」页
- **代码**：`client/src/utils/platform.js` → `getLocation()`
- **行为**：首次使用弹出系统授权对话框

### 微信头像

- **触发**：「完善资料」页点击头像区域
- **代码**：`client/src/pages/profile-edit/profile-edit.vue` → `onChooseAvatar()`
- **关键逻辑**：
  - 选微信头像 → 返回 `https://thirdwx.qlogo.cn/...` CDN URL → **直接存储，不上传**
  - 选相册图片 → 返回本地临时路径 → `uni.uploadFile` 上传至 `/api/upload`
  - 模拟器 Windows 下不可用（虚拟文件系统 bug，只能真机测试）

### 微信昵称

- **触发**：「完善资料」页昵称输入框
- **代码**：`client/src/pages/profile-edit/profile-edit.vue` `<input type="nickname">`
- **行为**：弹出微信原生昵称选取键盘

### 相册 / 拍照

- **触发**：聊天页发送图片、完善资料选头像（非微信头像）
- **代码**：`client/src/utils/upload.js` → `chooseAndUploadImage()`
- **上传接口**：`POST /api/upload`

### 订阅消息

- **触发**：首次进入「海洋」页（管理后台 `push_subscribe_prompt=1` 时）
- **代码**：`client/src/pages/ocean/ocean.vue` → `requestPushSubscription()`
- **模板**：

| 场景 | 模板 ID |
|---|---|
| 签到提醒 | `jDkqqnbLivP1FxZsIWXdmnuofZKCyHiuEMyvIEEjKrE` |
| 新作品推荐 | `1QTc2A0zT5RUm0Khm6EGv33UPEGCWdkBwDu02steWn4` |
| 活动预约 | `pCcmAdWN2BrNao4JB_Zx0WYyqY6Ip4r-dtnER321nbs` |

- **约束**：`wx.requestSubscribeMessage` 必须在用户 tap 手势同步调用链中执行，不能在 async 回调或定时器中调用

---

## 五、隐私授权框架（2023 年 9 月起强制）

`App.vue` `onLaunch` 注册全局回调，拦截所有涉及隐私 API 的首次调用：

```js
// #ifdef MP-WEIXIN
wx.onNeedPrivacyAuthorization((resolve) => {
  uni.showModal({
    title: '用户隐私保护提示',
    content: '...',
    success(res) {
      if (res.confirm) {
        wx.requirePrivacyAuthorize({
          success: () => resolve({ event: 'agree', buttonId: 'agree-btn' }),
          fail: () => resolve({ event: 'disagree' })
        })
      } else {
        resolve({ event: 'disagree' })
      }
    }
  })
})
// #endif
```

**故障排查**：真机日志出现 `ENOENT: stat '.../privacy/scopeState.txt'` 或 `errno:101` → 说明隐私回调未触发或未授权，检查 MP 后台「用户隐私保护指引」是否配置完整。
