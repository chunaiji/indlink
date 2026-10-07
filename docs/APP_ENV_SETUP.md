# Flutter 开发环境配置（Drift App）

> 这份文档分两层，别混着看：
>
> - **§1 硬约束**——代码本身要求什么。两台机器必须一致，不满足就构建不出来。
> - **§1.2 实测记录**——每台机器各自跑通时的实际配置。**允许不一样**，
>   只要都满足 §1.1。不要把某一台的偶然选择当成通用要求（这份文档以前就是
>   那样写的，导致第二台机器照着去装 JBR 17，而它的 Android Studio 自带的是 25）。

---

## 1. 版本要求

### 1.1 硬约束（代码决定的，不可协商）

| 约束 | 值 | 来源 | 不满足的后果 |
|---|---|---|---|
| **Dart SDK** | `^3.13.0` | `pubspec.yaml` 的 `environment` | `flutter pub get` 直接拒绝解析 |
| **Flutter** | `≥ 3.47.0` | 上一条推导（3.47.0 才带 Dart 3.13.0） | 同上 |
| Flutter **不得低于** 3.47 | — | §4.5 两个依赖锁在「3.47 已移除 `Architecture.arm64e`」这个前提上 | native-assets hook 报错，任何平台都构建失败 |
| **AGP** | `9.1.0` | `android/settings.gradle.kts` | — |
| **Kotlin** | `2.4.0` | 同上 | — |
| **Gradle** | `9.3.1` | `gradle-wrapper.properties`（wrapper 自动下载，无需手装） | — |
| **Java/Kotlin 字节码目标** | `17` | `android/app/build.gradle.kts` 的 `VERSION_17` / `JVM_17` | — |
| **Android NDK** | `28.2.13676358` | `ndkVersion = flutter.ndkVersion` | AGP 找不到会尝试联网装 |
| **CMake** | `3.22.1` | 依赖里的 `:jni:` 模块要做原生构建 | `:jni:configureCMakeDebug` 失败，见 §4.7 |

**字节码目标 17 ≠ 必须用 JDK 17。** 这是以前这份文档写错的地方，见 §4.1。

**产物的 SDK 级别**（从 release APK 反查，最权威）：

```
compileSdk 36 · targetSdk 36 · minSdk 24
ABI: arm64-v8a, armeabi-v7a, x86_64
applicationId / namespace: com.ambertu.bottles
Java/Kotlin 字节码目标: 17
```

### 1.2 两台机器的实测记录

| 组件 | 机器 A（2026-09-18） | 机器 B（2026-09-19） |
|---|---|---|
| 操作系统 | Windows 11 专业版 25H2 (26200) | Windows 10 专业版 21H2 (19044) |
| CPU | — | i7-4700MQ（Haswell 四核，2013） |
| 内存 / 提交上限 | 31.8 G / 35.7 G（**吃紧**，见 §4.8） | 15.9 G / 53.9 G（页面文件 22+16 G，宽裕） |
| **Flutter** | `3.47.0`（framework `4cf2416426`） | `3.47.5`（framework `6a19cca564`） |
| **Dart** | `3.13.0` | `3.13.4` |
| Flutter SDK 路径 | `D:\flutter_windows_3.47.0-stable\flutter` | `I:\flutter_windows_3.47.5-stable\flutter` |
| **Gradle 实际用的 JDK** | Android Studio 自带 **JBR 17.0.10** | **JDK 21.0.12.1**（独立安装） |
| Android Studio | （版本未记录），自带 JBR 17 | 2026.1.4，自带 **JBR 25** |
| Android SDK Platform | 36（另有 30/34/35） | 36、37.0（构建时自动补装了 34、35） |
| build-tools | 36.0.0（另有 34.0.0） | 36.0.0 |
| NDK | 28.2.13676358 | 28.2.13676358（手动装，见 §4.2） |
| CMake | （未记录，应已有） | 3.22.1（手动装，见 §4.7） |
| cmdline-tools | latest | **13114758（19.0）**，见 §4.6 |
| Emulator | 34.1.19.0 | 37.1.11.0 |
| 模拟器加速 | （未记录） | **AEHD 2.2**（Hyper-V/WHPX 均关闭） |
| 系统镜像 | android-35 | android-36 google_apis_playstore x86_64 |
| 国内镜像环境变量 | 设了 `FLUTTER_STORAGE_BASE_URL` / `PUB_HOSTED_URL` | **没设，pub.dev 直连正常** |
| 项目盘 / pub cache 盘 | E: / C:（跨盘） | J: / C:（跨盘，同样需要 §4.9 的设置） |
| 验证产物 | `flutter build apk --release` ~57 MB | `flutter build apk --debug` 165.5 MB |

两台唯一真正冲突的地方是 **Gradle 用的 JDK**，而那恰恰不是硬约束——见 §4.1。

---

## 2. 安装步骤

### 2.1 Flutter SDK

解压到**不含空格、不含中文**的路径。本机是：

```
D:\flutter_windows_3.47.0-stable\flutter
```

把 `<解压路径>\flutter\bin` 加进 `Path`。

> 装指定版本而不是最新版：`flutter upgrade` 会跳到新版，而本项目对
> Flutter 3.47 有硬依赖（见 §5 的两个依赖锁定）。装完**不要** upgrade。

### 2.2 国内镜像（视网络情况，不是必做）

> **2026-09-19 修订**：原文写的是「必做，否则大概率卡住」。机器 B 上**没设这两个
> 变量**，`flutter pub get`（118 个依赖）和 `flutter --version` 的 pub upgrade
> 都直连 pub.dev 正常完成。所以这条按需，不是硬性。

先直接试一次 `flutter pub get`，卡住再设。要设的话必须是**用户级永久变量**，
不能只在当前终端 set：

```powershell
setx FLUTTER_STORAGE_BASE_URL "https://storage.flutter-io.cn"
setx PUB_HOSTED_URL "https://pub.flutter-io.cn"
```

> 真正容易卡的不是 pub，而是 **Android SDK 组件**和 **Flutter SDK 本体**这些大文件，
> 那两个变量管不到。见 §4.2。

Gradle 侧的镜像已经写进仓库了（`android/settings.gradle.kts` 里阿里云的三个仓库
排在 `google()` 前面），不用另配。

### 2.3 Android SDK

用 Android Studio 的 SDK Manager 装。路径两台机器不同，各自记清楚：

```
机器 A：D:\Android\Sdk
机器 B：C:\Users\<用户名>\AppData\Local\Android\Sdk   （Studio 默认位置）
```

勾选：

- SDK Platforms → **Android 16 (API 36)**
- SDK Tools → **Android SDK Build-Tools 36.0.0**
- SDK Tools → **Android SDK Command-line Tools** —— ⚠️ 别直接点「latest」，见 §4.10
- SDK Tools → **Android SDK Platform-Tools**
- SDK Tools → **NDK 28.2.13676358**
- SDK Tools → **CMake 3.22.1** —— 漏了会挂在 `:jni:`，见 §4.7
- （要跑模拟器再加）SDK Tools → Android Emulator + 一个系统镜像

装完设环境变量（用户级），Gradle 和 Flutter 都靠它们找 SDK：

```powershell
setx ANDROID_HOME     "<你的 SDK 路径>"
setx ANDROID_SDK_ROOT "<你的 SDK 路径>"
setx JAVA_HOME        "<你的 JDK 路径>"      # ≥17；注意会覆盖系统级的旧 JDK，见 §4.1
flutter config --android-sdk "<你的 SDK 路径>" --jdk-dir "<你的 JDK 路径>"
```

然后接受许可：

```powershell
flutter doctor --android-licenses
```

> 这一步如果只回一句 `The --licenses option is no longer needed.`，
> 说明装到了新版 cmdline-tools，**许可证不会被接受**，照 §4.10 处理。

### 2.4 验证

```powershell
flutter doctor -v
```

Android toolchain 这一项必须是 `[√]`，且能看到
`Platform android-36, build-tools 36.0.0` 与 `All Android licenses accepted.`。

---

## 3. 项目首次构建

```powershell
cd <仓库>\app\bottles
flutter pub get
flutter analyze          # 应输出 No issues found!
flutter build apk --release
```

`android/local.properties` **不进仓库**（`.gitignore` 里），首次 `flutter pub get`
会自动生成。如果没生成，手写两行，注意反斜杠要转义：

```properties
# 机器 A
sdk.dir=D:\\Android\\Sdk
flutter.sdk=D:\\flutter_windows_3.47.0-stable\\flutter

# 机器 B
sdk.dir=C:\\Users\\Administrator\\AppData\\Local\\Android\\Sdk
flutter.sdk=I:\\flutter_windows_3.47.5-stable\\flutter
```

---

## 4. ⚠️ 已经踩过的坑

这一节是这份文档真正的价值所在——上面那些版本号照抄就行，下面这几条不看会重新踩一遍。

### 4.1 JDK：字节码目标 17，但**不要求** JVM 也是 17

> **2026-09-19 更正。** 这一条原本写的是「`build.gradle.kts` 写死了
> `JavaVersion.VERSION_17` / `JVM_17`，**用 21 会编译失败**」——**这个结论是错的**，
> 在机器 B 上不成立。下面是更正后的说明。

`compileOptions` 的 `sourceCompatibility` / `targetCompatibility` 和 Kotlin 的
`jvmTarget` 指定的是**产出的字节码版本**，不是「跑 Gradle 的 JVM 版本」。
用 JDK 21 编译出 17 字节码是完全正常的用法。

机器 B 的实测：`JAVA_HOME` = JDK 21.0.12.1，`flutter config --jdk-dir` 也指向它，
Kotlin 编译、Java 编译、`assembleDebug` **全部通过**，产出 `app-debug.apk`。

机器 A 之所以用 JBR 17，只是因为它的 Android Studio 恰好自带 17 且 Flutter
默认优先选 Studio 的 JBR——那是**默认行为的结果，不是代码的要求**。

**照抄机器 A 的做法在新机器上反而会出事**：机器 B 的 Android Studio 2026.1.4
自带的是 **JBR 25**，机器上根本没有 JDK 17。

实际要求：**JDK 17 或更高即可**（已验证 21 可用）。显式指定的方法：

```powershell
flutter config --jdk-dir="<JDK 路径>"
flutter doctor -v          # 看 "Java binary at:" 确认实际用的是哪个
```

注意 `JAVA_HOME` 有系统级和用户级两层，用户级会覆盖系统级。机器 B 的系统级
`JAVA_HOME` 指向一个古老的 JDK 8，是在用户级设 JDK 21 盖掉的。

### 4.2 代理软件会拦住 Java 的下载（比原先记的更严重）

本机装了 Proxifier，它**只代理 Java 进程**，结果是：

- Android Studio / `sdkmanager` 下载系统镜像卡死、或者下到一半**包损坏**
- 但 PowerShell / `curl` 直接下同一个地址是通的

机器 B 上实测到的三种表现，**一种比一种难认**：

1. `sdkmanager` 下系统镜像，到 66% 报 `Error on ZipFile unknown archive`
   ——看着像 Google 的包坏了，其实是传输被截断
2. `sdkmanager` 进程挂着不动，`Get-NetTCPConnection` 看只有一条 `::1` 本地连接，
   没有任何外网连接——**完全静默**
3. **最阴的一种**：Gradle 构建跑到 `:jni:configureCMakeDebug[arm64-v8a]` 失败，
   报 `Failed to install the following SDK components: cmake;3.22.1`。
   报错位置在原生构建任务上，看起来像 NDK/CMake 配置问题，
   翻到栈顶才看到 `java.net.ConnectException: Connection timed out`。见 §4.7。

> 注意第 3 种发生时，Android SDK Platform 34 和 35 是**装成功了**的——
> 所以"Java 下载全废"这个判断也不准，它是**部分成功**，更容易误导。

**根治办法**（二选一）：

- 在 Proxifier 里给 `java.exe` 加 Direct 规则（长期有效，推荐）
- 下载 SDK 组件时临时退出 Proxifier

**绕过办法**（不动 Proxifier，机器 B 用的就是这个）：
用 `curl.exe` 直连把包下下来，校验哈希后手动解压进 SDK 目录。
每个包的官方 URL、大小和 SHA-1 都能从 Google 的仓库清单里读到：

```powershell
# 主清单：platforms / build-tools / ndk / cmake
[xml]$x = (Invoke-WebRequest "https://dl.google.com/android/repository/repository2-3.xml" -UseBasicParsing).Content
$p = $x.DocumentElement.SelectNodes("remotePackage") | ? { $_.GetAttribute('path') -eq 'ndk;28.2.13676358' }
$p.SelectNodes("archives/archive") | % { $_.SelectSingleNode("complete/url").InnerText }

# 系统镜像在各自的子清单，例如：
# https://dl.google.com/android/repository/sys-img/google_apis_playstore/sys-img2-3.xml
```

下载用 `curl.exe -L -C - --retry 20 --retry-all-errors`（`-C -` 断点续传，
大文件被重置时能接上），下完**务必用官方 SHA-1 校验**再解压。

国内可用的镜像（机器 B 实测，字节数与 Google 官方完全一致）：

```
Flutter SDK    https://mirrors.cloud.tencent.com/flutter/flutter_infra_release/releases/...
Android SDK    https://mirrors.cloud.tencent.com/AndroidSDK/<包名>.zip
系统镜像        https://mirrors.cloud.tencent.com/AndroidSDK/sys-img/<tag>/<包名>.zip
```

> 直连 `storage.googleapis.com` / `dl.google.com` 下**大文件**（>500 MB）在机器 B 上
> 会稳定地被重置，报 `Received an unexpected EOF or 0 bytes from the transport stream`；
> 小文件（~150 MB）没问题。走上面的镜像 + 断点续传可以绕开。

### 4.3 模拟器黑屏 —— Impeller 的着色器在模拟器上链不动

模拟器（GLES）上 Impeller 的 `FramebufferBlend` pipeline 链接失败，表现是**整屏纯黑、
没有任何报错**。

已在 `android/app/src/debug/AndroidManifest.xml` 里**只对 debug 关掉 Impeller**：

```xml
<meta-data android:name="io.flutter.embedding.android.EnableImpeller" android:value="false" />
```

release 仍然用 Impeller（真机没问题）。**不要把这个 meta-data 挪到 main manifest**，
那样 release 也会退回 Skia。

### 4.4 `-qt-hide-window`：模拟器启动了但看不见窗口

Android Studio 的「Launch in a tool window」选项会给 emulator 加 `-qt-hide-window`
参数，于是 `flutter devices` 能看到设备、但屏幕不显示。
在 Android Studio 的 Settings → Tools → Emulator 里关掉那个选项。

### 4.5 两个依赖必须锁版本

`pubspec.yaml` 里这两条不是随手写的，**升上去会直接构建失败**：

```yaml
# 锁在 9.x：10+ 的 flutter_secure_storage_darwin → objective_c 的
# native-assets hook 引用了 Flutter 3.47 已移除的 Architecture.arm64e
flutter_secure_storage: ^9.2.4

dependency_overrides:
  # 同一个原因：2.6.0 起走 objective_c 9.6.1 的 hook
  path_provider_foundation: 2.4.1
```

失败现象是 `flutter test` / `flutter build` 在任何平台都直接挂掉，报的是
native-assets 相关的错，**跟你改的代码没关系**。

### 4.6 原生通道：`drift/haptics`

`android/app/src/main/kotlin/com/ambertu/bottles/MainActivity.kt` 注册了一个
MethodChannel 直接驱动 `Vibrator`（`AndroidManifest.xml` 里配套声明了
`android.permission.VIBRATE`）。

Flutter 自带的 `HapticFeedback` 走 `View.performHapticFeedback`，**系统「触感反馈」
开关一关就完全静默**，捞瓶仪式的震动会整个失效，所以才自己接了原生。
换机器时这部分不用配置，但**改 Android 侧代码时别把这个 channel 弄丢**。

### 4.7 CMake 3.22.1 必须预装，否则构建挂在 `:jni:`

依赖里的 `:jni:` 模块要做原生构建，需要 **CMake 3.22.1**。AGP 会尝试自动装，
但那是 Java 发起的下载，在有代理的机器上必失败（§4.2 第 3 种表现）。

失败长这样，**看起来完全不像网络问题**：

```
Execution failed for task ':jni:configureCMakeDebug[arm64-v8a]'.
> com.android.builder.sdk.InstallFailedException: Failed to install the following SDK components:
      cmake;3.22.1 CMake 3.22.1
```

手动装（只有 15 MB）：

```powershell
curl.exe -L -o cmake.zip "https://dl.google.com/android/repository/cmake-3.22.1-windows.zip"
# 官方 SHA1: 292778f32a7d5183e1c49c7897b870653f2d2c1b
tar.exe -xf cmake.zip -C "$env:ANDROID_HOME\cmake\3.22.1"
```

注意这个 zip **顶层没有包装目录**，解出来直接就是 `bin/ doc/ share/ source.properties`，
要解到 `cmake\3.22.1\` 里面，不要再套一层。装对了的标志：
`$env:ANDROID_HOME\cmake\3.22.1\bin\cmake.exe` 和 `ninja.exe` 都在，
且 `source.properties` 里是 `Pkg.Path = cmake;3.22.1`。

### 4.8 Gradle 堆内存：`-Xmx2G` 是刻意调小的，别改回模板值

`android/gradle.properties` 里把 Flutter 模板默认的 `-Xmx8G -XX:MaxMetaspaceSize=4G`
改成了 `-Xmx2G -XX:MaxMetaspaceSize=512m`。原因写在文件注释里：机器 A 的
**Windows 提交上限**只剩 3.2 G，JVM 一次性提交 8G+4G 虚拟地址空间会 mmap 失败，
daemon 直接崩。

机器 B 的提交上限宽裕（剩 24.6 G），但物理内存只有 15.9 G、可用 4.7 G，
**2 G 堆同样是合适的**。两台机器出于不同原因都该保持这个值，不要改回去。

### 4.9 `kotlin.incremental=false`：只要项目盘和 pub cache 盘不同就必须保持

同样写在 `gradle.properties` 注释里：Kotlin 增量编译器要算源文件相对模块根的
相对路径，跨盘符算不出来，抛 `this and base files have different roots`。

机器 A 是 E:（项目）/ C:（pub cache），机器 B 是 J: / C:，**两台都跨盘**，
所以这条对两台都适用。想恢复增量编译，得设 `PUB_CACHE` 把缓存挪到项目同盘。

### 4.10 新版 cmdline-tools 废弃了 `--licenses`，会让 doctor 卡在「license unknown」

Google 最新的 cmdline-tools（16111833 / 20.0）把 `sdkmanager` 换成了新的
Android CLI，`sdkmanager --licenses` 只回一句
`Warning: The --licenses option is no longer needed.`。

而 Flutter 的许可证校验（`android_workflow.dart` 的 `licensesAccepted`）是靠
**跑 `sdkmanager --licenses` 并在输出里找 `All SDK package licenses accepted.`**
来判断的，匹配不到就返回 `unknown`，于是：

- `flutter doctor` 永远报 `X Android license status unknown`
- `flutter doctor --android-licenses` 也失效，没法接受许可证

机器 B 的解法：把**带经典 sdkmanager 的 13114758（19.0）装成
`cmdline-tools\latest`**，新版挪到 `cmdline-tools\20.0` 备用。之后
`flutter doctor --android-licenses` 正常工作，7 个许可证全部接受，doctor 转绿。

副作用（可接受）：旧版 cmdline-tools 看不懂新版 Studio 装的 SDK XML v4，
`avdmanager list target` 会有一句
`This version only understands SDK XML versions up to 3` 的告警，
并把 `platforms;android-37.0` 显示成 `android-0`。**不影响构建和建 AVD。**

### 4.11 模拟器加速：AEHD 而不是 HAXM

HAXM 已被 Google 弃用，新版 emulator 不再支持；Windows 上现在是两条路：
**WHPX**（要开 Hyper-V 平台，会影响 VMware/VirtualBox，且要重启）或
**AEHD**（独立内核驱动，不碰 Hyper-V）。

机器 B 用的是 AEHD，**不需要重启**：

```powershell
# AEHD 已从 Android SDK 仓库下架，转到 GitHub 发布
# https://github.com/google/android-emulator-hypervisor-driver/releases  (v2.2)
# 解压后用管理员权限跑（silent_install_safe.bat 不带自提权，适合已提权的终端）
.\silent_install_safe.bat
sc query aehd                              # 应为 RUNNING
& "$env:ANDROID_HOME\emulator\emulator.exe" -accel-check   # 应为 "AEHD (version 2.2) is installed and usable."
```

装之前建议验一下签名：`aehd.Sys` 应由 **Google LLC** 签名，
`aehd.cat` 由 **Microsoft Windows Hardware Compatibility Publisher** 签名。

另外 `avdmanager create avd` 生成的 `config.ini` 有两个默认值要改：

```ini
hw.gpu.enabled=yes            # 默认是 no，软件渲染慢到不可用
disk.dataPartition.size=6G    # 默认 2G，装 Play 商店镜像偏小
# 并删掉 disk.dataPartition.path=<temp> 这一行，否则数据分区不持久化
```

---

## 5. iOS（本机不可用）

Windows 上 `flutter build ipa` **连子命令都不存在**——Flutter 只在 macOS 注册 iOS
构建命令。`ios/` 工程是齐的（Bundle ID `com.ambertu.bottles`、部署目标 15.0、
相册权限文案已写），但要出包必须有 macOS + Xcode。

另外 iOS 没有 Android 那种 debug keystore：要把包发给测试，只能走 TestFlight 或
ad-hoc（注册每台设备 UDID），两条都需要 **Apple 开发者账号（$99/年）**。

详见 `docs/APP_DEBUG_GUIDE.md`。

---

## 6. 快速自检清单

新机器配完，逐条对一遍：

```powershell
flutter --version                 # Flutter ≥ 3.47.0，Dart ≥ 3.13.0（不必是 3.47.0 整）
flutter doctor -v                 # 7 项全 [√]；重点看 Android toolchain 和 licenses accepted
                                  #   顺便确认 "Java binary at:" 指向的 JDK ≥ 17
cd app\bottles
flutter pub get                   # 不报 native-assets 错误
flutter analyze                   # No issues found!
flutter build apk --debug         # 能出 app-debug.apk 就算通
```

最后一条能过，环境就是对的。`--release` 也能过更好，但 debug 已经覆盖了
Kotlin 编译、Java 编译、CMake 原生构建、Dart AOT 这几个真正会卡的环节。

**两台机器的实测耗时**（供判断「是慢还是卡住」）：

| 阶段 | 机器 B 实测 |
|---|---|
| 首次 `flutter build apk --debug`（要下 Gradle 9.3.1 + 全部依赖） | **30 分钟**（i7-4700MQ） |
| 依赖缓存后再次构建 | **145 秒** |

> 首次构建期间 `flutter` 会把 Gradle 输出缓冲到结束才吐，**日志长时间是空的属于正常**，
> 不要据此判断卡死。想确认它在动，看 `.gradle\caches` 最近是否有文件写入，
> 或看 Gradle daemon 进程的 CPU 累计时间。
