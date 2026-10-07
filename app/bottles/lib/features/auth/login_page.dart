import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/platform/oauth.dart';
import '../../core/providers.dart';
import '../../core/storage/prefs.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/brand_marks.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/sea_scene.dart';
import 'auth_controller.dart';
import 'oauth_conflict_page.dart';
import 'oauth_pending_overlay.dart';

/// A2 手机号 / 邮箱登录。
///
/// 类别：表单页（规范 §6）
/// 固定区：底部协议
/// 弹性区：头部留白（高 < 700 减半）；多出的高度按 1:2 分到头部上方与底部
/// 可滚动区：头部 + 表单 + 第三方登录 + 注册引导
/// 键盘：协议让位，两段弹性留白归零，表单区随 resizeToAvoidBottomInset 收缩后可滚
///
/// 两个渠道用顶部 Tab 切，下面的发送按钮与 A3 验证码页整屏复用——
/// 对用户来说这是同一件事，换的只是「码发到哪」。
///
/// 硬约束：一旦提供 Google 登录，iOS **必须**同时提供 Apple 登录（App Store 4.8）。
/// 反过来 Android 上**不该**出现 Apple 登录——4.8 只约束 iOS，而这个包在 Android
/// 上会退化成需要另配 Service ID 的网页流程，点了只会失败。故按平台显隐。
///
/// 两个按钮的尺寸 / 圆角 / 文案 / logo 都有强制品牌规范，最终要换成官方 SDK 的按钮组件，
/// 这里只占位置与布局。
class LoginPage extends ConsumerStatefulWidget {
  const LoginPage({super.key});

  @override
  ConsumerState<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends ConsumerState<LoginPage> {
  final _controller = TextEditingController();
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();

  static const _dialCodes = ['+91', '+86', '+1', '+44', '+65'];

  /// 隐私授权弹框只在本页第一次满足条件时弹一次（避免每次 rebuild 重复弹）。
  bool _gateChecked = false;

  /// 「继续即表示同意…」前面的勾。没勾就点登录 / 第三方登录会先弹协议确认。
  /// 以前同意过（隐私弹框或这里）就默认勾上，不让人每次登录都点一遍。
  bool _agreed = false;

  /// 「记住密码」。默认开，关掉即清除已存的密码。
  late bool _rememberPassword = ref.read(prefsProvider).rememberPassword;

  @override
  void initState() {
    super.initState();
    _agreed = ref.read(prefsProvider).privacyAgreed;
    // 从注册/找回页退回来时，流程要复位成登录，否则「发送验证码」还带着上一条 purpose。
    // setFlow 会清密码，所以上次账号的回填放在它之后。
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (!mounted) return;
      ref.read(otpProvider.notifier).setFlow(AuthFlow.login);
      await _restoreLastLogin();
    });
  }

  /// 把上次登录的账号填回表单；开了「记住密码」就连密码一起填。
  ///
  /// 渠道只在后台同时开放手机 / 邮箱时才按记忆切——只开放一种时表单就是那一种，
  /// 硬切过去会出现一个后台已经关掉的输入框。
  Future<void> _restoreLastLogin() async {
    final prefs = ref.read(prefsProvider);
    final m = prefs.loginMemory;
    if (m.isEmpty) return;
    final n = ref.read(otpProvider.notifier);
    if (ref.read(appConfigProvider).loginChannelsBoth) {
      n.setChannel(m.channel == 'email' ? OtpChannel.email : OtpChannel.phone);
    }
    if (m.phone.isNotEmpty) {
      n.setDialCode(m.dialCode);
      n.setPhone(m.phone);
      _controller.text = m.phone;
    }
    if (m.email.isNotEmpty) {
      n.setEmail(m.email);
      _emailController.text = m.email;
    }
    if (m.method == 'code' &&
        ref.read(otpProvider).channel == OtpChannel.phone) {
      n.setMethod(LoginMethod.code);
    }
    if (!_rememberPassword) return;
    final pw = await ref.read(credentialStoreProvider).readPassword();
    if (!mounted || pw == null || pw.isEmpty) return;
    n.setPassword(pw);
    _passwordController.text = pw;
  }

  /// 登录成功（或验证码已发出）后记下这组账号；密码按开关决定存不存。
  Future<void> _rememberLogin({bool withPassword = false}) async {
    final otp = ref.read(otpProvider);
    final prefs = ref.read(prefsProvider);
    await prefs.setLoginMemory(
      LoginMemory(
        channel: otp.isEmail ? 'email' : 'phone',
        method: otp.method == LoginMethod.code ? 'code' : 'password',
        dialCode: otp.dialCode,
        phone: otp.phone,
        email: otp.email,
      ),
    );
    final store = ref.read(credentialStoreProvider);
    if (withPassword && _rememberPassword && otp.password.isNotEmpty) {
      await store.writePassword(otp.password);
    } else if (!_rememberPassword) {
      await store.clearPassword();
    }
  }

  Future<void> _toggleRemember(bool v) async {
    setState(() => _rememberPassword = v);
    await ref.read(prefsProvider).setRememberPassword(v);
    if (!v) await ref.read(credentialStoreProvider).clearPassword();
  }

  @override
  void dispose() {
    _controller.dispose();
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  /// 没勾协议就先弹确认；同意即勾上并记住，不同意什么都不发生。
  Future<bool> _ensureAgreed() async {
    if (_agreed) return true;
    final ok = await showAgreementModal(context);
    if (!mounted || ok != true) return false;
    setState(() => _agreed = true);
    await ref.read(prefsProvider).setPrivacyAgreed();
    return mounted;
  }

  /// 密码登录（主路径）。
  Future<void> _login() async {
    if (!await _ensureAgreed()) return;
    final ok = await ref.read(otpProvider.notifier).loginWithPassword();
    if (ok) await _rememberLogin(withPassword: true);
    if (!mounted || ok) return;
    final err = ref.read(otpProvider).error;
    if (err != null) showToast(context, err, error: true);
  }

  /// 手机号免密：发码后进验证码页，验完直接登录。
  Future<void> _sendOtpLogin() async {
    if (!await _ensureAgreed()) return;
    final n = ref.read(otpProvider.notifier);
    n.setFlow(AuthFlow.otpLogin);
    final ok = await n.sendCode();
    if (!mounted) return;
    if (ok) {
      // 验证码还没验，但账号已经确定——先记下，验证页成功后不会再回到这里。
      await _rememberLogin();
      if (!mounted) return;
      context.push(Routes.otp);
    } else {
      final err = ref.read(otpProvider).error;
      if (err != null) showToast(context, err, error: true);
    }
  }

  Future<void> _oauth(String provider) async {
    if (!await _ensureAgreed()) return;
    final ok = await ref.read(otpProvider.notifier).loginWithProvider(provider);
    if (!mounted || ok) return;
    // 「该邮箱已注册」不是 toast 能承载的——它要求用户做一件两步的事，
    // 走整屏 A2k。其余失败才用 toast；用户取消则两者都为空，静默返回。
    final conflict = ref.read(otpProvider).conflictEmail;
    if (conflict != null) {
      ref.read(otpProvider.notifier).clearConflict();
      await Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) =>
              OAuthConflictPage(message: conflict, provider: provider),
        ),
      );
      return;
    }
    final err = ref.read(otpProvider).error;
    if (err != null) showToast(context, err, error: true);
  }

  /// 首次登录前的隐私授权弹框（合规）。开关默认关，运营上线前打开；
  /// 不同意则退出 App —— 合规要求：不同意不能进入登录/主流程。
  Future<void> _showPrivacyGate() async {
    final l = L.of(context);
    // 原型 A1c：被动弹出的模态（scale 0.92→1，不从底部滑入），锁图标 + 标题 + 正文 +
    // 两个整宽按钮（同意 primary / 不同意 ghost）。不能预勾选、不能点遮罩关。
    final agreed = await showAppModal<bool>(
      context,
      dismissible: false,
      builder: (ctx) => PopScope(
        canPop: false,
        child: ModalCard(
          icon: Icons.lock_outline_rounded,
          title: l.privacyGateTitle,
          body: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(l.privacyGateBody),
              const SizedBox(height: Dim.s3),
              const AuthAgreement(),
            ],
          ),
          onClose: () => Navigator.of(ctx).pop(false),
          actions: [
            AppButton(
              label: l.privacyGateAgree,
              onTap: () => Navigator.of(ctx).pop(true),
            ),
            AppButton(
              label: l.privacyGateExit,
              kind: BtnKind.ghost,
              onTap: () => Navigator.of(ctx).pop(false),
            ),
          ],
        ),
      ),
    );
    if (!mounted) return;
    if (agreed == true) {
      await ref.read(prefsProvider).setPrivacyAgreed();
    } else {
      await SystemNavigator.pop();
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final otp = ref.watch(otpProvider);
    final channel = effectiveChannel(ref, otp.channel);

    // 隐私授权门（合规，开关默认关）：开着且未同意时，进登录页即弹一次。
    final cfg = ref.watch(appConfigProvider);
    if (!_gateChecked &&
        cfg.showPrivacyGate &&
        !ref.read(prefsProvider).privacyAgreed) {
      _gateChecked = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _showPrivacyGate();
      });
    }

    // 头部留白：原型 `.bd` 内边距 16 + 头部 margin 36px → 54pt，合计 70；
    // 矮屏（SE 667）减半。这是本页唯一按高度变化的量（规范 §4.3）。
    final headTop = Breakpoints.isShort(context) ? 35.0 : 70.0;
    // 键盘弹起时协议让位：它钉在屏底只是为了「不滚动也看得见」，
    // 键盘上来后这条理由不成立，占着 50pt 反而把表单挤出视野。
    final keyboardUp = MediaQuery.viewInsetsOf(context).bottom > 0;

    final page = Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            // 可滚动区：表单自上而下按原型节奏排布，超出一屏才滚。
            //
            // 原型按 762 高的框画的，851 以上的机器会多出 100～250pt。多出的高度
            // **按 1:2 分到头部上方与底部**，表单整体略靠上居中（视觉重心在
            // 黄金分割附近，iOS 系统登录页也是这个比例）。Pro Max（932）多出约 185pt，
            // 上方那份 62pt，不需要封顶。键盘弹起时可用高度骤减，两段留白自动归零，
            // 表单区退回顶部对齐再滚动——靠 SliverFillRemaining(hasScrollBody:false)：
            // 内容不足一屏时 Column 拿到整屏高度让 Spacer 分配，超出一屏时按内容高度滚。
            // ⚠️ 只能用 Spacer：Flexible 包一个有固定高度的 SizedBox 会把那段高度算进
            // Column 的固有高度，SliverFillRemaining 据此撑高后页面会平白多出一截可滚动区。
            Expanded(
              child: CustomScrollView(
                keyboardDismissBehavior:
                    ScrollViewKeyboardDismissBehavior.onDrag,
                slivers: [
                  SliverFillRemaining(
                    hasScrollBody: false,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: Dim.gutter,
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          SizedBox(height: headTop),
                          const Spacer(flex: 1),
                          // 头部整体居中：瓶子 + 欢迎语 + 副标（原型 A2）。
                          Column(
                            children: [
                              const BottleSprite(width: 72, tilt: -30),
                              const SizedBox(height: Dim.s2),
                              Text(
                                l.authLoginTitle,
                                style: TextStyle(
                                  fontSize: Dim.t6,
                                  fontWeight: FontWeight.w800,
                                  letterSpacing: -0.24,
                                  height: 1.25,
                                  color: c.ink,
                                ),
                              ),
                              const SizedBox(height: 3),
                              Text(
                                l.appTagline,
                                style: TextStyle(
                                  fontSize: Dim.t2,
                                  height: 1.5,
                                  color: c.ink3,
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: Dim.s4),
                          // 表单组：内部间距 8，组与组之间 16（原型 .bd gap s4 / 组内 gap s2）。
                          // 后台只开放一种标识时不出现切换，表单直接是那一种。
                          ChannelTabs(
                            current: channel,
                            onSelect: ref.read(otpProvider.notifier).setChannel,
                          ),
                          if (ChannelTabs.visible(ref))
                            const SizedBox(height: Dim.s2),
                          if (channel == OtpChannel.email)
                            EmailField(
                              controller: _emailController,
                              onChanged: ref
                                  .read(otpProvider.notifier)
                                  .setEmail,
                            )
                          else
                            PhoneField(
                              controller: _controller,
                              dialCode: otp.dialCode,
                              dialCodes: _dialCodes,
                              onDialCode: ref
                                  .read(otpProvider.notifier)
                                  .setDialCode,
                              onChanged: ref
                                  .read(otpProvider.notifier)
                                  .setPhone,
                            ),
                          // 验证码方式下不出现密码框——留着它等于让人以为两样都得填。
                          if (!otp.isCodeMethod) ...[
                            const SizedBox(height: Dim.s2),
                            PasswordField(
                              controller: _passwordController,
                              hint: l.authPasswordHint,
                              onChanged: ref
                                  .read(otpProvider.notifier)
                                  .setPassword,
                              onSubmitted: otp.canLogin
                                  ? (_) => _login()
                                  : null,
                            ),
                          ],
                          // 「验证码登录 / 密码登录」切换 + 「忘记密码」同一行（原型 A2：
                          // 两端都是 t1/700/ink3，不给切换链接上色）。切换只在手机号出现，
                          // 忘记密码只在密码方式出现。
                          const SizedBox(height: Dim.s2),
                          Row(
                            children: [
                              // 三样都可收缩：窄屏 + 大字体下谁也不许把别人顶出去。
                              if (channel == OtpChannel.phone) ...[
                                Flexible(
                                  child: _InlineLink(
                                    label: otp.isCodeMethod
                                        ? l.authTabPassword
                                        : l.authTabCode,
                                    onTap: () => ref
                                        .read(otpProvider.notifier)
                                        .setMethod(
                                          otp.isCodeMethod
                                              ? LoginMethod.password
                                              : LoginMethod.code,
                                        ),
                                  ),
                                ),
                                const SizedBox(width: Dim.s3),
                              ],
                              // 「记住密码」只在密码方式下出现：验证码登录没有密码可记。
                              if (!otp.isCodeMethod)
                                Flexible(
                                  child: _RememberToggle(
                                    checked: _rememberPassword,
                                    onChanged: _toggleRemember,
                                  ),
                                ),
                              const Spacer(),
                              if (!otp.isCodeMethod)
                                Flexible(
                                  child: _InlineLink(
                                    label: l.authForgotPassword,
                                    onTap: () {
                                      ref
                                          .read(otpProvider.notifier)
                                          .setFlow(AuthFlow.reset);
                                      context.push(Routes.forgotPassword);
                                    },
                                  ),
                                ),
                            ],
                          ),
                          const SizedBox(height: Dim.s4),
                          if (otp.isCodeMethod)
                            AppButton(
                              label: l.authSendCode,
                              loading: otp.busy,
                              onTap: otp.canSend ? _sendOtpLogin : null,
                            )
                          else
                            AppButton(
                              label: l.authSignIn,
                              loading: otp.busy,
                              onTap: otp.canLogin ? _login : null,
                            ),
                          // 四家全关时连「或」分隔线一起收起，
                          // 否则会留一条下面什么都没有的分隔线。
                          if (cfg.googleLogin ||
                              (showAppleSignIn && cfg.appleLogin)) ...[
                            const SizedBox(height: Dim.s4),
                            _OrDivider(label: l.commonOr),
                            const SizedBox(height: Dim.s4),
                          ],
                          if (cfg.googleLogin)
                            AppButton(
                              label: l.authContinueWithGoogle,
                              kind: BtnKind.oauth,
                              leading: const GoogleMark(size: 20),
                              onTap: () => _oauth('google'),
                            ),
                          // Apple 要过两道门：平台支持 + 后台启用。
                          if (showAppleSignIn && cfg.appleLogin) ...[
                            if (cfg.googleLogin)
                              const SizedBox(height: Dim.s2),
                            AppButton(
                              label: l.authContinueWithApple,
                              kind: BtnKind.oauth,
                              icon: Icons.apple_rounded,
                              onTap: () => _oauth('apple'),
                            ),
                          ],
                          // 注册引导放在第三方登录之后（原型 A2）。
                          const SizedBox(height: Dim.s4),
                          _RegisterHint(
                            onTap: () {
                              ref
                                  .read(otpProvider.notifier)
                                  .setFlow(AuthFlow.register);
                              context.push(Routes.register);
                            },
                          ),
                          const SizedBox(height: Dim.s4),
                          const Spacer(flex: 2),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
            // 固定区：协议钉在屏底（原型 `margin-top:auto`），多出的高度全落在
            // 它和「注册」之间——这正是原型在高屏上的样子，不是拉伸。
            if (!keyboardUp)
              Padding(
                padding: const EdgeInsets.fromLTRB(
                  Dim.gutter,
                  Dim.s3,
                  Dim.gutter,
                  Dim.s4,
                ),
                child: AuthAgreement(
                  fontSize: Dim.t2,
                  checked: _agreed,
                  onChanged: (v) => setState(() => _agreed = v),
                ),
              ),
          ],
        ),
      ),
    );
    // A2g / A2p：第三方登录进行中，整屏等待层盖在登录页上。
    final pending = otp.pendingProvider;
    if (pending == null) return page;
    return Stack(
      children: [
        page,
        OAuthPendingOverlay(provider: pending),
      ],
    );
  }
}

/// 表单下方的行内文字链接（「用验证码登录」「忘记密码？」）。
///
/// 文字本身只有 12pt 高，可点区域按 24pt 撑开，不然在小屏上很难点中。
/// 不再撑到 32：真机比对发现那会把这一行和上下的间距各拉大 8pt。
class _InlineLink extends StatelessWidget {
  const _InlineLink({required this.label, required this.onTap});

  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Container(
        constraints: const BoxConstraints(minHeight: Dim.s6),
        alignment: Alignment.centerLeft,
        child: Text(
          label,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(
            fontSize: Dim.t1,
            fontWeight: FontWeight.w700,
            height: 1.4,
            color: c.ink3,
          ),
        ),
      ),
    );
  }
}

/// 密码输入框。注册、登录、重设三处共用，所以是公开的。
///
/// 默认隐藏字符 + 右侧眼睛切换：印度用户大量在小键盘上输入长密码，
/// 不给可见切换会显著拉高输错率。
class PasswordField extends StatefulWidget {
  const PasswordField({
    super.key,
    required this.controller,
    required this.hint,
    required this.onChanged,
    this.onSubmitted,
    this.helper,
  });

  final TextEditingController controller;
  final String hint;
  final ValueChanged<String> onChanged;
  final ValueChanged<String>? onSubmitted;

  /// 设密码场景下的规则说明（「至少 8 位」）。
  final String? helper;

  @override
  State<PasswordField> createState() => _PasswordFieldState();
}

class _PasswordFieldState extends State<PasswordField> {
  bool _hidden = true;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Container(
          height: 50,
          padding: const EdgeInsets.only(left: Dim.s3, right: Dim.s2),
          decoration: BoxDecoration(
            color: c.surface,
            borderRadius: Dim.brCard,
            border: Border.all(color: c.line),
          ),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  controller: widget.controller,
                  onChanged: widget.onChanged,
                  onSubmitted: widget.onSubmitted,
                  obscureText: _hidden,
                  autocorrect: false,
                  enableSuggestions: false,
                  textCapitalization: TextCapitalization.none,
                  style: TextStyle(fontSize: Dim.t3, color: c.ink),
                  decoration: InputDecoration(
                    isDense: true,
                    filled: false,
                    border: InputBorder.none,
                    enabledBorder: InputBorder.none,
                    focusedBorder: InputBorder.none,
                    contentPadding: EdgeInsets.zero,
                    hintText: widget.hint,
                  ),
                ),
              ),
              GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: () => setState(() => _hidden = !_hidden),
                child: Padding(
                  padding: const EdgeInsets.all(Dim.s2),
                  child: Icon(
                    _hidden
                        ? Icons.visibility_off_rounded
                        : Icons.visibility_rounded,
                    size: 20,
                    color: c.ink3,
                  ),
                ),
              ),
            ],
          ),
        ),
        if (widget.helper != null) ...[
          const SizedBox(height: Dim.s1),
          Text(
            widget.helper!,
            style: TextStyle(fontSize: Dim.t1, height: 1.5, color: c.ink3),
          ),
        ],
      ],
    );
  }
}

class _RegisterHint extends StatelessWidget {
  const _RegisterHint({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Text.rich(
        TextSpan(
          style: TextStyle(fontSize: Dim.t2, color: c.ink3),
          children: [
            TextSpan(text: '${l.authNoAccount} '),
            TextSpan(
              text: l.authSignUp,
              style: TextStyle(fontWeight: FontWeight.w800, color: c.brand),
            ),
          ],
        ),
        textAlign: TextAlign.center,
      ),
    );
  }
}

/// 后台开关决定生效的渠道：当前选的那个被关掉时回落到开着的那个
/// （两个都关 = 都开，见 [AppRemoteConfig.phoneLoginOn]）。
///
/// 渲染用返回值；状态也要跟上——提交时控制器按 state.channel 选接口，
/// 光改渲染会把空手机号发出去。改 provider 放到帧后，build 里不能改。
OtpChannel effectiveChannel(WidgetRef ref, OtpChannel current) {
  final cfg = ref.watch(appConfigProvider);
  var next = current;
  if (current == OtpChannel.phone && !cfg.phoneLoginOn) next = OtpChannel.email;
  if (current == OtpChannel.email && !cfg.emailLoginOn) next = OtpChannel.phone;
  if (next != current) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (ref.context.mounted) ref.read(otpProvider.notifier).setChannel(next);
    });
  }
  return next;
}

/// 「手机号 / 邮箱」二选一。滑块底在选中项下面，宽度各占一半。
/// 后台只开放一种时整个切换不渲染（[visible]），表单直接是那一种。
class ChannelTabs extends ConsumerWidget {
  const ChannelTabs({super.key, required this.current, required this.onSelect});

  final OtpChannel current;
  final ValueChanged<OtpChannel> onSelect;

  static bool visible(WidgetRef ref) =>
      ref.watch(appConfigProvider).loginChannelsBoth;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!visible(ref)) return const SizedBox.shrink();
    final l = L.of(context);
    // 原型 .seg.aq：登录页的渠道切换用海蓝——粉色在全 App 的语义是「要花币」。
    return SegmentedRow(
      labels: [l.authTabPhone, l.authTabEmail],
      index: current == OtpChannel.phone ? 0 : 1,
      accent: context.c.aqua,
      onChanged: (i) => onSelect(i == 0 ? OtpChannel.phone : OtpChannel.email),
    );
  }
}

class EmailField extends StatelessWidget {
  const EmailField({
    super.key,
    required this.controller,
    required this.onChanged,
    this.onSubmitted,
  });

  final TextEditingController controller;
  final ValueChanged<String> onChanged;
  final ValueChanged<String>? onSubmitted;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: Dim.s3),
      decoration: BoxDecoration(
        color: c.surface,
        borderRadius: Dim.brCard,
        border: Border.all(color: c.line),
      ),
      alignment: Alignment.centerLeft,
      child: TextField(
        controller: controller,
        onChanged: onChanged,
        onSubmitted: onSubmitted,
        keyboardType: TextInputType.emailAddress,
        autofillHints: const [AutofillHints.email],
        autocorrect: false,
        // 邮箱地址永远不该被首字母大写——键盘默认会这么干。
        textCapitalization: TextCapitalization.none,
        style: TextStyle(fontSize: Dim.t3, color: c.ink),
        decoration: InputDecoration(
          isDense: true,
          filled: false,
          border: InputBorder.none,
          enabledBorder: InputBorder.none,
          focusedBorder: InputBorder.none,
          contentPadding: EdgeInsets.zero,
          hintText: L.of(context).authEmailHint,
        ),
      ),
    );
  }
}

class PhoneField extends StatelessWidget {
  const PhoneField({
    super.key,
    required this.controller,
    required this.dialCode,
    required this.dialCodes,
    required this.onDialCode,
    required this.onChanged,
    this.onSubmitted,
  });

  final TextEditingController controller;
  final String dialCode;
  final List<String> dialCodes;
  final ValueChanged<String> onDialCode;
  final ValueChanged<String> onChanged;
  final ValueChanged<String>? onSubmitted;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      height: 50,
      padding: const EdgeInsets.only(left: Dim.s3, right: Dim.s3),
      decoration: BoxDecoration(
        color: c.surface,
        borderRadius: Dim.brCard,
        border: Border.all(color: c.line),
      ),
      child: Row(
        children: [
          PopupMenuButton<String>(
            initialValue: dialCode,
            onSelected: onDialCode,
            position: PopupMenuPosition.under,
            itemBuilder: (_) => [
              for (final code in dialCodes)
                PopupMenuItem(value: code, child: Text(code)),
            ],
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  dialCode,
                  style: TextStyle(
                    fontSize: Dim.t3,
                    fontWeight: FontWeight.w700,
                    color: c.ink,
                  ),
                ),
                Icon(Icons.arrow_drop_down_rounded, size: 18, color: c.ink3),
              ],
            ),
          ),
          Container(
            width: 1,
            height: 20,
            margin: const EdgeInsets.symmetric(horizontal: Dim.s3),
            color: c.line,
          ),
          Expanded(
            child: TextField(
              controller: controller,
              onChanged: onChanged,
              onSubmitted: onSubmitted,
              keyboardType: TextInputType.phone,
              autofillHints: const [AutofillHints.telephoneNumber],
              inputFormatters: [
                FilteringTextInputFormatter.digitsOnly,
                LengthLimitingTextInputFormatter(15),
              ],
              style: TextStyle(fontSize: Dim.t3, color: c.ink),
              decoration: InputDecoration(
                isDense: true,
                filled: false,
                border: InputBorder.none,
                enabledBorder: InputBorder.none,
                focusedBorder: InputBorder.none,
                contentPadding: EdgeInsets.zero,
                hintText: L.of(context).authPhoneHint,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _OrDivider extends StatelessWidget {
  const _OrDivider({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Row(
      children: [
        Expanded(child: Divider(color: c.line)),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: Dim.s3),
          child: Text(
            label,
            style: TextStyle(fontSize: Dim.t1, color: c.ink3),
          ),
        ),
        Expanded(child: Divider(color: c.line)),
      ],
    );
  }
}

/// 「登录即代表同意…」。
///
/// 两个链接**必须真的能点开**——长得像链接却点不动，合规上等于没提供，
/// 提审会被直接打回。正文走接口下发，登录前也能取（`/api/legal/:doc` 免鉴权）。
class AuthAgreement extends StatefulWidget {
  const AuthAgreement({
    super.key,
    this.fontSize = Dim.t1,
    this.checked,
    this.onChanged,
  });

  /// 登录页钉在屏底的那条用 11pt（原型 A2 `--t0`），注册页与弹框里用 12pt。
  final double fontSize;

  /// 给了就在文字前面画一个勾选圆点；null = 纯文案（弹框里那种）。
  final bool? checked;
  final ValueChanged<bool>? onChanged;

  @override
  State<AuthAgreement> createState() => _AuthAgreementState();
}

class _AuthAgreementState extends State<AuthAgreement> {
  // recognizer 持有手势订阅，随 State 一起释放；建在 build 里会泄漏。
  late final _terms = TapGestureRecognizer()
    ..onTap = () => context.push(Routes.legal('terms'));
  late final _privacy = TapGestureRecognizer()
    ..onTap = () => context.push(Routes.legal('privacy'));

  @override
  void dispose() {
    _terms.dispose();
    _privacy.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final link = TextStyle(
      fontSize: widget.fontSize,
      fontWeight: FontWeight.w700,
      color: c.brand,
    );
    final text = Text.rich(
      TextSpan(
        style: TextStyle(fontSize: widget.fontSize, color: c.ink3, height: 1.7),
        children: [
          TextSpan(text: '${l.authAgreementPrefix} '),
          TextSpan(text: l.authTerms, style: link, recognizer: _terms),
          TextSpan(text: ' ${l.authAnd} '),
          TextSpan(text: l.authPrivacy, style: link, recognizer: _privacy),
        ],
      ),
      textAlign: TextAlign.center,
    );
    final checked = widget.checked;
    if (checked == null) return text;
    // 勾与文字是**一组**：整体居中、紧挨着，点文字（非链接部分）也能勾。
    // 以前勾钉在最左、文字居中，两者隔着半个屏幕，看起来像两个不相干的东西。
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => widget.onChanged?.call(!checked),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          GestureDetector(
            key: const ValueKey('auth-agree-check'),
            behavior: HitTestBehavior.opaque,
            onTap: () => widget.onChanged?.call(!checked),
            child: Padding(
              padding: const EdgeInsets.only(right: Dim.s2),
              child: _CheckDot(checked: checked),
            ),
          ),
          Flexible(child: Text.rich(text.textSpan!, textAlign: TextAlign.left)),
        ],
      ),
    );
  }
}

/// 「记住密码」小勾：与协议勾同一套圆点，12pt 文字，整行可点。
class _RememberToggle extends StatelessWidget {
  const _RememberToggle({required this.checked, required this.onChanged});

  final bool checked;
  final ValueChanged<bool> onChanged;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      key: const ValueKey('auth-remember-password'),
      behavior: HitTestBehavior.opaque,
      onTap: () => onChanged(!checked),
      child: Container(
        constraints: const BoxConstraints(minHeight: Dim.s6),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            _CheckDot(checked: checked, size: 16),
            const SizedBox(width: 6),
            Flexible(
              child: Text(
                L.of(context).authRememberPassword,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: Dim.t1,
                  fontWeight: FontWeight.w700,
                  height: 1.4,
                  color: c.ink3,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 勾选圆点：默认 20pt，勾上海蓝实心 + 白勾，没勾 line 描边。
class _CheckDot extends StatelessWidget {
  const _CheckDot({required this.checked, this.size = 20});

  final bool checked;
  final double size;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return AnimatedContainer(
      duration: const Duration(milliseconds: 120),
      width: size,
      height: size,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: checked ? c.aqua : Colors.transparent,
        border: Border.all(color: checked ? c.aqua : c.line, width: 1.5),
      ),
      child: checked
          ? Icon(Icons.check_rounded, size: size * .66, color: Colors.white)
          : null,
    );
  }
}

/// 没勾协议就点了登录：弹一次确认（原型 A1c 的被动模态），同意返回 true。
Future<bool?> showAgreementModal(BuildContext context) {
  final l = L.of(context);
  return showAppModal<bool>(
    context,
    builder: (ctx) => ModalCard(
      icon: Icons.description_outlined,
      title: l.authAgreeTitle,
      body: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(l.authAgreeBody),
          const SizedBox(height: Dim.s3),
          const AuthAgreement(),
        ],
      ),
      onClose: () => Navigator.of(ctx).pop(false),
      actions: [
        AppButton(
          label: l.authAgreeAndContinue,
          onTap: () => Navigator.of(ctx).pop(true),
        ),
        AppButton(
          label: l.commonCancel,
          kind: BtnKind.ghost,
          onTap: () => Navigator.of(ctx).pop(false),
        ),
      ],
    ),
  );
}
