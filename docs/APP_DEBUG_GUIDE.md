# App 调试流程（给不熟悉移动端的人）

工程位置：`app/bottles`。所有命令都在这个目录下执行。

---

## 0. 一分钟结论

| 你想做什么 | 用哪条路 | 需要什么 |
|---|---|---|
| **看界面、改 UI 文案配色** | Chrome 浏览器 | 什么都不用配，现在就能跑 |
| **测真实交互**（相册、分享、通知、返回手势） | Android 模拟器/真机 | 先做第 1 步修环境变量 |
| **提审前验证** | iPhone | 需要一台 Mac，本机做不了 |

日常改 UI **只用 Chrome 就够**，改完按一个键就能看到效果。

---

## 1. 环境问题（已经帮你修好，这里记录原因）

这台机器上 Android 构建原本是坏的，一共**三个互相独立的坑**，都跟代码无关。已全部处理，记在这里是为了你换机器或同事接手时能照着排。

### 坑 1：空的代理环境变量 —— 已修

用户级环境变量里有 4 个**存在但值为空**的项：

```
HTTP_PROXY=   HTTPS_PROXY=   http_proxy=   https_proxy=
```

`sdkmanager` 读到空串后拿它当网址解析，一启动就崩：

```
java.net.MalformedURLException: no protocol:
```

连带后果是 `flutter doctor` 显示「Android license status unknown」—— 不是你没接受过许可，而是它要调 sdkmanager 去读许可状态，而 sdkmanager 根本起不来。

**已做的处理**：删掉注册表 `HKCU\Environment` 下这两个空项（小写的那两个并不真实存在，Windows 环境变量名不区分大小写）。`GOPROXY` 原样保留。

**副作用**：删完之后 `flutter doctor` 的 Android toolchain 直接变 `[√]`，许可也不用再手动接受。

> 已经开着的终端 / VS Code / Android Studio 需要**重启**才能拿到新环境——它们在启动时就继承了旧的那一份。

### 坑 2：Proxifier 的虚拟 DNS 挡住了 Gradle —— 已绕开

`dl.google.com` 在这台机器上被解析成 `127.249.1.144` 和 `fd00:696e:6974:6578::`。
那个 IPv6 前缀解码出来是 **`initex`**，即 Proxifier 开发商的标记：它开了「通过代理解析域名」，把域名映射成虚拟地址，再由自己拦截转发。

但 **Gradle 是 Java 进程，不在 Proxifier 的规则覆盖里**，于是它老实去连 `127.249.1.144:443`，那里没人监听：

```
Connect to dl.google.com:443 [dl.google.com/127.249.1.144] failed: Connection refused
```

**已做的处理**：给 `android/settings.gradle.kts` 和 `android/build.gradle.kts` 配了阿里云 Maven 镜像并排在官方源前面，官方源留作兜底。这样完全不碰 `dl.google.com`，顺带下载也快得多。

如果哪天镜像也连不上，说明 Proxifier 把 `maven.aliyun.com` 也虚拟解析了，那时有两个选择：给 `java.exe` 加一条直连规则，或者调试 Android 时先退出 Proxifier。

### 坑 3：项目与 pub cache 不在同一个盘 —— 已关掉增量编译

项目在 **E 盘**，pub cache（插件的 Kotlin 源码）在 **C 盘**。Kotlin 增量编译器要算源文件相对模块根目录的相对路径，**跨盘符算不出来**：

```
IllegalArgumentException: this and base files have different roots:
  C:\Users\...\Pub\Cache\...\ImageCompressPlugin.kt
  E:\MySelf\Code\git\ai-message\app\bottles\android
AssertionError: Could not close incremental caches
```

**已做的处理**：`android/gradle.properties` 加了 `kotlin.incremental=false`。

代价很小——插件的 Kotlin 代码量很少，全量编译多几秒；我们自己的业务代码是 Dart，走另一条编译链路，完全不受影响。想恢复增量编译的话，把 pub cache 挪到与项目同盘（设 `PUB_CACHE` 环境变量）即可。

### 排查这类问题的一个提醒

用 PowerShell 管道看构建输出时，**`$LASTEXITCODE` 拿到的是管道最后一个命令的退出码**，不是构建本身的：

```powershell
flutter build apk --debug | Select-String "error"   # 退出码永远是 Select-String 的，构建失败也显示 0
```

我自己就差点被这个骗过去。要判断成败，看输出里有没有 `Built build\app\outputs\...` 或 `FAILURE`，别只看退出码。

---

## 2. 最快看到界面：Chrome

```powershell
cd app\bottles
flutter run -d chrome
```

第一次编译大约 1~2 分钟，之后会自动打开浏览器。

**跑起来后，终端里这几个键是你最常用的：**

| 键 | 作用 | 什么时候按 |
|---|---|---|
| `r` | **热重载** —— 保留当前页面状态，只更新代码 | 改了颜色、文字、布局 |
| `R` | **热重启** —— 重新走一遍启动流程 | 改了启动逻辑、登录态相关 |
| `q` | 退出 | 结束调试 |

**日常循环就是**：改 Dart 文件 → 存盘 → 在终端按 `r` → 浏览器立刻变。不用重新编译。

### 哪些改动按 `r` 不够，必须停掉重跑？

- 新增或删除了依赖包（改了 `pubspec.yaml`）
- 改了多语言文案（`lib/l10n/*.arb`）→ 要先跑 `flutter gen-l10n`
- 改了 `main.dart` 里的初始化逻辑
- 出现莫名其妙的错误时，先试 `R`，再不行就 `q` 重跑

### Chrome 上有两个功能用不了

保存海报到相册、系统分享面板——这两个是手机能力，浏览器里点了会报错。**其余全部可用**，包括登录、捞瓶、聊天、支付页面。

---

## 3. Android 模拟器 / 真机

做完第 1 步之后：

```powershell
flutter devices              # 看有哪些设备
flutter run                  # 只有一个设备时直接跑
flutter run -d <设备ID>      # 多个设备时指定
```

**真机**：手机打开「开发者选项 → USB 调试」，插上数据线，`flutter devices` 就能看到。

**模拟器**：已经建好一个 `drift_api35`（Android 35 · Google APIs · x86_64），直接用：

```powershell
flutter emulators --launch drift_api35
flutter run
```

> 如果 `flutter emulators` 列不出它，多半是终端还在用旧环境变量——见下面第 2 点。

### 当时为什么 Device Manager 里建不出来（换机器还会遇到）

Device Manager 建 AVD 必须选一个 System Image，而这台机器上 `system-images` 是**空的**；
点「Download」会去连 `dl.google.com`，它被解析成 `127.88.0.37`（回环）——又是坑 2 那个
Proxifier 虚拟 DNS。而**下载器是 Java 进程，不在 Proxifier 的规则覆盖里**，于是去连一个
没人监听的地址，**静默失败，连报错都不给**（`sdkmanager --list` 只会刷
`IO exception while downloading manifest`）。

绕开的办法不是关 Proxifier，而是利用一个区分：**被 Proxifier 规则匹配的进程能正常转发，
只有 Java 不能**。所以用 PowerShell 直接下 zip 就通了：

```powershell
# 1) 从 manifest 找 zip 直链：sys-img2-3.xml 里 remotePackage 的 archives/complete/url
#    本次是 https://dl.google.com/android/repository/sys-img/google_apis/x86_64-35_r09.zip
(New-Object System.Net.WebClient).DownloadFile($url, "D:\Android\_dl\x.zip")

# 2) 解压进 SDK。zip 顶层就是 x86_64/，所以解到 google_apis\ 这一层,不要再套一层
[System.IO.Compression.ZipFile]::ExtractToDirectory(
  "D:\Android\_dl\x.zip", "D:\Android\Sdk\system-images\android-35\google_apis")

# 3) 建 AVD。avdmanager 会问要不要自定义硬件配置,喂个 no 进去
$env:ANDROID_AVD_HOME = "D:\Android\avd"
"no" | avdmanager create avd -n drift_api35 -k "system-images;android-35;google_apis;x86_64" -d pixel_7
```

两个当时不确定、现在有答案的点：

1. **不需要 `package.xml`**。手动解压的镜像没有这个文件（它是 sdkmanager 装包时生成的），
   但 `sdkmanager --list_installed` 靠 `source.properties` 就能认出来，`avdmanager` 也一样
2. **AVD 必须放 D 盘**。C 盘只剩 8.5 GB，镜像解压后就 3.5 GB，AVD 本身还要几 GB。
   已设用户级 `ANDROID_AVD_HOME=D:\Android\avd`——改完要**重启终端 / Android Studio**
   才生效，它们在启动时就继承了旧的那一份

**硬件加速**：`emulator -accel-check` 返回 0，WHPX 可用。输出里那句
「Please disable Hyper-V」是 HAXM 路径的遗留提示，WHPX 与 Hyper-V 共存没问题，不用管。

**真机不受影响**：插数据线就能用，不需要下载任何镜像。

---

## 4. 用假数据还是连真后端

App 默认跑在 **Mock 模式**：数据都是内存里造的，**不需要启动后端**，但行为是真的——扣币、配额、发消息、余额不足跳充值，整条链路都能走通。

### 想连真实后端时

```powershell
flutter run -d chrome `
  --dart-define=USE_MOCK=false `
  --dart-define=API_BASE=https://ambertu.com/message/api `
  --dart-define=WS_BASE=wss://ambertu.com/message/ws `
  --dart-define=APP_ID=drift_app_dev
```

> ⚠️ **前提**：后端必须能解析出 App 的租户，两种方式**满足一个**即可：
>
> - 环境变量 `APP_DEFAULT_TENANT_ID=358804313465688064`（推荐，本地 `.env` 已配好）——
>   这时 `APP_ID` 传什么都行，反正查不到凭证就落这个租户
> - 或者 `app_credentials` 表里有一行 `platform="app"`，且 `APP_ID` 与它的 appid 对得上
>
> 两个都没有，所有 App 接口都会返回「未配置 App 凭证」。这是 App 端的总开关。

### 连本机后端的两个坑

后端本地起在 `localhost:8980`（怎么起见 `docs/DEV_RUNBOOK.md`）：

| 跑在哪 | API_BASE 要写什么 | 为什么 |
|---|---|---|
| Chrome | `http://localhost:8980/api` | 同一台机器 |
| Android 模拟器 | `http://10.0.2.2:8980/api` | 模拟器里的 `localhost` 指的是它自己，`10.0.2.2` 才是你的电脑 |
| Android 真机 | `http://你电脑的局域网IP:8980/api` | 手机和电脑要在同一个 WiFi |

---

## 5. 出问题时怎么查

### 第一步永远是看终端

`flutter run` 的终端会实时打印错误。红色的一大段里，**只看第一行和带 `package:bottles/` 的那几行**——那才是我们自己的代码，其余是框架内部调用栈。

### 第二步：DevTools（图形化调试器）

`flutter run` 启动后终端会打印一个 `http://127.0.0.1:xxxxx` 的地址，浏览器打开它。里面最有用的两个页签：

- **Network**：看每个接口请求的 URL、参数、返回值——接口联调全靠它
- **Widget Inspector**：点界面上任何元素，看它是哪个文件哪一行画出来的

### 常见问题对照表

| 现象 | 多半是什么 | 怎么办 |
|---|---|---|
| **白屏 / 一直转圈** | 某个接口没返回 | 看 DevTools 的 Network 页签，找红色那条 |
| **改了代码没反应** | 忘了按 `r`，或改的是需要重启的部分 | 先按 `r`，不行按 `R`，再不行 `q` 重跑 |
| **改了文案不生效** | ARB 文件改完没重新生成 | 跑 `flutter gen-l10n`，然后 `R` |
| **一直跳回登录页** | 接口返回了 2001（登录失效） | 检查服务端有没有 `APP_DEFAULT_TENANT_ID`，或 `APP_ID` 与 `app_credentials` 那行对不对得上 |
| **提示「余额不足」但余额够** | 错误码对不上 | 后端余额不足是 **5001**（钱包段），不是 3001 |
| **图片不显示** | Mock 模式下本来就没有真实图片 | 正常现象，连真后端才有 |
| **报 MissingPluginException** | 在 Chrome 上用了手机才有的功能 | 换 Android 跑，或忽略（保存相册/分享属于此类） |
| **一堆红字看不懂** | 先跑 `flutter analyze` | 它会用人话告诉你哪行有问题 |

---

## 6. 改完代码后必做的两条检查

提交前跑一遍，等于「编译器帮你把关」：

```powershell
flutter analyze      # 应该输出 No issues found!
flutter test         # 应该输出 All tests passed!
```

`analyze` 能抓出绝大多数低级错误（拼错变量名、类型不对、忘了处理空值），**比跑起来点一遍快得多**。

后端改完对应的是：

```powershell
cd server
go build ./... ; go vet ./...    # 两条都没输出就是通过
```

---

## 7. 命令速查

```powershell
cd app\bottles

flutter run -d chrome     # 浏览器跑（最快）
flutter run               # 手机/模拟器跑
flutter devices           # 看有哪些设备可用

flutter analyze           # 静态检查
flutter test              # 跑测试
flutter gen-l10n          # 改了 lib/l10n/*.arb 之后必跑

flutter clean             # 玄学问题的万能药：清缓存
flutter pub get           # clean 之后要跑这个恢复依赖
```

> **不要随手升级依赖**。`flutter_secure_storage` 锁在 9.x、`path_provider_foundation`
> 锁在 2.4.1，升上去会让构建在**任何平台**直接失败（原因见 `app/bottles/README.md`）。

---

## 8. 目前跑不了的部分

这些点了不会有反应或会报错，是**还没接完**，不是 bug（完整清单见 `docs/APP_V1_PROGRESS.md`）：

- 「用 Google / Apple 继续」——客户端还没接官方 SDK
- iOS 充值——还没接 StoreKit
- 推送通知——设备登记做了，下发还没做
- 看激励视频——还没接 AdMob
- 真实短信 / 邮件验证码——服务商都还没接，Mock 模式下**随便填 6 位数字就能登录**；
  连真后端时验证码只打在服务端日志里（`[otp] dev-mode ...`），或在后台「App 登录」分组配一个开发态万能码
