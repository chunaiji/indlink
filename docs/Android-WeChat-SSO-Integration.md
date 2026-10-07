# Android 集成微信 SSO（微信登录）指南

> 目标：在 Android App 中集成微信登录，通过微信授权获取临时 `code`，由业务后端完成身份校验并签发本系统的登录 Token。
>
> 适用：原生 Android（Java）项目。Flutter 项目也可以复用 Android 原生集成部分，但还需要通过 MethodChannel 等方式桥接到 Flutter。

## 一、整体登录流程

1. 用户在 Android App 点击「微信登录」。
2. App 调用微信 Android SDK，拉起微信客户端。
3. 用户在微信中确认或取消授权。
4. 微信通过 `WXEntryActivity` 回调 App。
5. 授权成功时，App 从 `SendAuth.Resp` 获取一次性授权码 `code`。
6. App 将 `code` 发送给自己的后端。
7. 后端使用服务端保存的微信 `AppID`、`AppSecret` 向微信兑换身份信息。
8. 后端根据 `openid`（或符合业务条件的 `unionid`）查询或创建本地用户。
9. 后端签发本系统的 JWT / Session Token，App 保存业务登录态并进入首页。

**重要：**
- 微信 `code` 是一次性授权凭证，不是业务登录 Token。
- 微信 `access_token` 与本系统的 JWT / Session Token 是不同的凭证。
- `AppSecret` 必须只保存在后端，不能写入 Android APK。
- 不要仅凭 SDK 调用成功就判定用户已登录。

## 二、准备微信开放平台配置

进入 [微信开放平台](https://open.weixin.qq.com/)，创建或打开移动应用，并确认应用已满足微信登录能力的开通、审核要求。

需要准备：

| 配置项 | 用途 |
|---|---|
| AppID | Android 注册微信 SDK、发起授权 |
| AppSecret | 后端向微信兑换授权信息 |
| Android 包名 | 必须与实际应用的 `applicationId` 对应 |
| Android 应用签名 | 用于校验应用身份 |

注意事项：

1. 包名必须与实际构建和安装的应用一致。
2. Debug 和 Release 的签名可能不同，需要分别核对。
3. 如果 APK 由应用商店重新签名，应核对最终安装包实际使用的证书。
4. `AppSecret` 应保存在后端环境变量或密钥管理系统中。
5. SDK 版本和开放平台要求可能变化，正式接入前请查看微信官方文档。

参考：
- [微信开放平台](https://open.weixin.qq.com/)
- [微信移动应用登录开发指南](https://wdk-docs.github.io/wxopen-docs/mobile/login/guide.html)
- [Android 接入指南](https://wdk-docs.github.io/wxopen-docs/mobile/guide/android.html)

## 三、Android 工程接入微信 SDK

以下使用 Java 示例，假设应用包名为 `com.example.myapp`。

### 3.1 添加 SDK 依赖

在 `app/build.gradle` 中添加：

```gradle
dependencies {
    // 示例版本；正式使用前请核对微信官方当前版本及项目兼容性
    implementation 'com.tencent.mm.opensdk:wechat-sdk-android:6.8.0'
}
```

如果项目使用 Kotlin DSL（`build.gradle.kts`），写法为：

```kotlin
dependencies {
    implementation("com.tencent.mm.opensdk:wechat-sdk-android:6.8.0")
}
```

### 3.2 创建微信 SDK 管理类

新建 `WechatManager.java`：

```java
package com.example.myapp;

import android.content.Context;

import com.tencent.mm.opensdk.openapi.IWXAPI;
import com.tencent.mm.opensdk.openapi.WXAPIFactory;

public final class WechatManager {

    public static final String APP_ID = "wx你的AppID";

    private static IWXAPI api;

    private WechatManager() {}

    public static void init(Context context) {
        if (api == null) {
            api = WXAPIFactory.createWXAPI(
                    context.getApplicationContext(),
                    APP_ID,
                    true
            );

            api.registerApp(APP_ID);
        }
    }

    public static IWXAPI getApi() {
        return api;
    }
}
```

将 `wx你的AppID` 替换为微信开放平台提供的真实 AppID。

### 3.3 在 Application 中初始化

新建 `MyApplication.java`：

```java
package com.example.myapp;

import android.app.Application;

public class MyApplication extends Application {

    @Override
    public void onCreate() {
        super.onCreate();
        WechatManager.init(this);
    }
}
```

在 `AndroidManifest.xml` 的 `<application>` 节点声明：

```xml
<application
    android:name=".MyApplication"
    ... >
</application>
```

如果项目已经有自己的 `Application` 类，应在现有类中初始化，不要重复声明多个 Application。

### 3.4 配置 AndroidManifest.xml

在 `manifest` 节点下添加网络权限：

```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
```

在 `<application>` 内注册微信回调 Activity：

```xml
<activity
    android:name=".wxapi.WXEntryActivity"
    android:exported="true"
    android:launchMode="singleTop"
    android:theme="@android:style/Theme.Translucent.NoTitleBar" />
```

确保类路径和包名一致。示例工程结构：

```text
app/src/main/
├── AndroidManifest.xml
└── java/com/example/myapp/
    ├── MyApplication.java
    ├── WechatManager.java
    ├── MainActivity.java
    └── wxapi/
        └── WXEntryActivity.java
```

注意：
- `WXEntryActivity` 必须位于应用包名下的 `wxapi` 子包中。
- 如果 `namespace` 与 `applicationId` 不同，应核对最终 APK 包名、回调类完整路径以及微信开放平台配置。
- `android:exported` 应按项目的 Android 版本和组件配置要求设置。

## 四、发起微信登录

在登录页面中添加以下方法：

```java
import com.tencent.mm.opensdk.modelmsg.SendAuth;
import com.tencent.mm.opensdk.openapi.IWXAPI;

private void loginWithWechat() {
    IWXAPI api = WechatManager.getApi();

    if (api == null || !api.isWXAppInstalled()) {
        // 提示用户未安装微信，并提供其他登录方式
        return;
    }

    SendAuth.Req req = new SendAuth.Req();
    req.scope = "snsapi_userinfo";
    req.state = "login_" + System.currentTimeMillis();

    boolean success = api.sendReq(req);

    if (!success) {
        // 提示发起微信授权失败
    }
}
```

说明：

- `scope` 表示申请的授权范围，应符合移动应用已获准使用的权限。
- `state` 用于关联授权请求和回调。生产项目应使用不可预测的随机值，并校验回调中的 `state`，不要只依赖时间戳。
- `sendReq()` 返回成功只表示请求已提交，不代表用户已经授权或登录成功。
- 实际项目应避免重复点击造成多个并发授权请求。

## 五、接收微信回调并获取 code

新建 `wxapi/WXEntryActivity.java`：

```java
package com.example.myapp.wxapi;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;

import com.example.myapp.WechatManager;
import com.tencent.mm.opensdk.modelbase.BaseReq;
import com.tencent.mm.opensdk.modelbase.BaseResp;
import com.tencent.mm.opensdk.modelmsg.SendAuth;
import com.tencent.mm.opensdk.openapi.IWXAPI;
import com.tencent.mm.opensdk.openapi.IWXAPIEventHandler;

public class WXEntryActivity extends Activity
        implements IWXAPIEventHandler {

    private IWXAPI api;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        WechatManager.init(getApplicationContext());
        api = WechatManager.getApi();

        if (api == null ||
                !api.handleIntent(getIntent(), this)) {
            finish();
        }
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);

        if (api != null) {
            api.handleIntent(intent, this);
        } else {
            finish();
        }
    }

    @Override
    public void onReq(BaseReq req) {
        // 微信向 App 发来的请求。
        // 如果暂时只实现登录，可按项目需要暂不处理。
    }

    @Override
    public void onResp(BaseResp resp) {
        try {
            if (resp instanceof SendAuth.Resp) {
                SendAuth.Resp authResp = (SendAuth.Resp) resp;

                if (authResp.errCode == BaseResp.ErrCode.ERR_OK) {
                    String code = authResp.code;
                    String state = authResp.state;

                    // TODO:
                    // 1. 校验 state 是否与当前登录请求匹配
                    // 2. 将 code 交给登录协调器 / 状态管理层
                    // 3. 调用业务后端的微信登录接口

                } else if (resp.errCode ==
                        BaseResp.ErrCode.ERR_USER_CANCEL) {
                    // 用户取消授权
                } else if (resp.errCode ==
                        BaseResp.ErrCode.ERR_AUTH_DENIED) {
                    // 用户拒绝授权
                } else {
                    // 其他授权错误
                }
            }
        } finally {
            finish();
        }
    }
}
```

以上是回调处理骨架，`TODO` 部分必须接入项目的登录流程。

生产环境建议：
- 通过生命周期安全的登录协调器、事件机制或状态管理层传递回调结果。
- 不要仅依赖某个 Activity 的静态字段保存登录结果。
- 处理授权取消、拒绝、超时、重复回调和页面销毁。
- 在将 `code` 交给后端之前，验证 `state` 与本次发起的登录请求匹配。

## 六、调用自己的后端完成 SSO

### 6.1 Android 请求后端

拿到 `code` 后，调用自己的业务接口，例如：

```http
POST /api/auth/wechat/android
Content-Type: application/json
```

请求体示例：

```json
{
  "code": "微信返回的一次性授权码",
  "state": "本次授权请求的状态值"
}
```

这是业务接口示例，不是微信官方接口。请求应通过 HTTPS 发送。

### 6.2 后端兑换微信授权信息

后端从安全配置中读取 `AppID` 和 `AppSecret`，再请求微信授权码兑换接口：

```http
GET https://api.weixin.qq.com/sns/oauth2/access_token
    ?appid=APPID
    &secret=APPSECRET
    &code=CODE
    &grant_type=authorization_code
```

实际请求时，应将参数正确 URL 编码并组成合法 URL。

后端应检查微信返回的错误字段和必要参数，不要只检查 HTTP 状态码。授权码具有一次性和时效性，失败时通常需要客户端重新发起授权。

### 6.3 后端用户匹配与签发 Token

建议后端按以下步骤处理：

1. 验证请求参数，并校验业务登录流程所需的状态。
2. 使用服务端保存的 `AppID`、`AppSecret` 和 `code` 请求微信接口。
3. 校验微信响应，获取 `openid` 等身份信息。
4. 按业务规则查询本地用户；如果允许微信新用户注册，则创建或关联用户。
5. 根据产品需要处理 `unionid`。只有在微信确实返回且符合应用体系要求时，才使用它进行跨应用账号关联。
6. 签发本系统自己的 JWT / Session Token。
7. 返回业务用户信息和 Token。

示例响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "accessToken": "your-app-jwt",
    "userId": "10001",
    "isNewUser": false
  }
}
```

上面是业务后端的响应格式示例，不是微信接口原始响应。

**安全要求：**
- `AppSecret` 只能存在于后端。
- 不要把微信 `access_token` 直接当成本系统的登录 Token 返回给 App。
- 不要在日志中记录 `AppSecret`、完整令牌或其他敏感凭证。
- 对账号绑定、首次注册和已有账号合并制定明确规则，避免错误关联账号。
- 对登录接口设置合理的超时、错误处理、限流和审计日志。

## 七、获取 Android 签名并核对平台配置

### 7.1 查看 Debug 签名

Windows 命令行执行：

```bat
keytool -list -v ^
  -keystore "%USERPROFILE%\.android\debug.keystore" ^
  -alias androiddebugkey ^
  -storepass android ^
  -keypass android
```

查看输出中的 `SHA1` 等证书指纹信息。

### 7.2 查看 Release 签名

将路径和 alias 替换为实际值：

```bash
keytool -list -v -keystore your-release.keystore -alias your-alias
```

如果 APK 最终由应用商店签名或重新签名，需要核对最终安装包的签名证书。

### 7.3 联调时重点核对

- AppID 是否正确。
- Android 包名是否与实际构建应用一致。
- 开放平台登记的签名是否匹配实际安装包。
- 应用审核状态和微信登录能力是否已开通。
- Debug 与 Release 是否使用不同的签名配置。

如果 Debug 能登录而 Release 失败，应优先排查包名、签名和最终 APK 的构建配置。

## 八、常见问题排查

| 问题 | 优先检查 |
|---|---|
| 无法拉起微信 | 微信是否安装、AppID 是否正确、SDK 初始化和注册是否成功 |
| 微信授权后没有回调 | `WXEntryActivity` 包名和路径、Manifest 注册、`handleIntent()` 是否执行 |
| 回调报错或参数为空 | SDK 版本、回调类型、授权结果及错误码 |
| Debug 正常，Release 失败 | Release 包名、签名证书、开放平台配置 |
| 后端兑换 code 失败 | code 是否过期或重复使用、AppID 与 AppSecret 是否匹配、微信响应中的错误信息 |
| 用户已授权但 App 未登录 | 后端请求是否发出、用户匹配逻辑、业务 Token 是否成功返回和保存 |
| 用户取消后界面卡住 | 取消回调、登录 loading 状态清理、超时和生命周期处理 |
| 多次点击导致异常 | 登录请求互斥、重复授权保护、重复回调处理 |

## 九、上线前测试清单

- [ ] 已安装微信时可以正常拉起授权。
- [ ] 用户同意授权后，App 能收到 `code`。
- [ ] 用户取消授权时能正常返回登录页面。
- [ ] 用户拒绝授权时有明确提示。
- [ ] 未安装微信时提供其他登录方式。
- [ ] 后端能正确兑换 `code` 并完成本地用户匹配。
- [ ] 后端能返回本系统的业务 Token。
- [ ] 过期或重复使用的 `code` 会被正确拒绝。
- [ ] `state` 会与本次授权请求进行匹配校验。
- [ ] Debug 和 Release 包都完成真机测试。
- [ ] App 重启后能正确恢复或刷新业务登录态。
- [ ] `AppSecret` 未被打包进 APK。
- [ ] 日志没有泄露敏感凭证。
- [ ] 登录、取消、拒绝、网络异常和后端异常均已测试。

## 十、建议实施顺序

1. 完成微信开放平台移动应用配置。
2. 添加微信 SDK 并初始化。
3. 验证能否拉起微信客户端。
4. 实现 `WXEntryActivity` 并确认能收到 `code`。
5. 实现后端兑换和本地用户匹配。
6. 完成业务 Token 的保存、刷新和退出登录。
7. 测试 Debug、Release、未安装微信、取消授权和网络异常场景。

## 参考资料

- [微信开放平台](https://open.weixin.qq.com/)
- [微信移动应用登录开发指南](https://wdk-docs.github.io/wxopen-docs/mobile/login/guide.html)
- [微信 Android 接入指南](https://wdk-docs.github.io/wxopen-docs/mobile/guide/android.html)

> 说明：本文中的 SDK 版本、类名和代码为集成示例。正式上线前，请以微信官方最新文档和 SDK 为准，并结合项目的 Android Gradle Plugin、Android 版本、包名、签名和后端接口进行验证。
