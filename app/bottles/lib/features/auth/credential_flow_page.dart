import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import 'auth_controller.dart';
import 'login_page.dart';

/// 注册与找回密码共用这一屏（原型 A2r）。
///
/// 类别：表单页（规范 §6）
/// 固定区：导航栏、底部协议（仅注册）
/// 弹性区：无
/// 可滚动区：表单本体
/// 键盘：协议让位，表单区随 resizeToAvoidBottomInset 收缩后可滚
///
/// 两条流程的形状完全一样——**标识 → 验证码 → 设密码**，差别只有三处：
/// 标题、提交按钮文案、以及提交时调哪个接口。为它们各写一个页面，
/// 等于把同一套「发码倒计时 + 验证码校验 + 密码规则」的逻辑抄两遍，
/// 改一处漏一处是迟早的事。
class CredentialFlowPage extends ConsumerStatefulWidget {
  const CredentialFlowPage({super.key, required this.flow});

  /// 只接受 [AuthFlow.register] 与 [AuthFlow.reset]。
  final AuthFlow flow;

  @override
  ConsumerState<CredentialFlowPage> createState() => _CredentialFlowPageState();
}

class _CredentialFlowPageState extends ConsumerState<CredentialFlowPage> {
  final _phoneController = TextEditingController();
  final _emailController = TextEditingController();
  final _codeController = TextEditingController();
  final _passwordController = TextEditingController();

  static const _dialCodes = ['+91', '+86', '+1', '+44', '+65'];
  static const _codeLength = 6;

  /// 验证码发出去之前不显示「验证码 + 密码」——一上来就摊开四个空框会劝退人。
  bool _codeSent = false;

  bool get _isRegister => widget.flow == AuthFlow.register;

  /// 注册才要勾协议；找回密码的人已经是用户了。
  bool _agreed = false;

  Future<bool> _ensureAgreed() async {
    if (!_isRegister || _agreed) return true;
    final ok = await showAgreementModal(context);
    if (!mounted || ok != true) return false;
    setState(() => _agreed = true);
    await ref.read(prefsProvider).setPrivacyAgreed();
    return mounted;
  }

  @override
  void initState() {
    super.initState();
    _agreed = ref.read(prefsProvider).privacyAgreed;
    // 进页面就把流程钉死，sendCode 的 purpose 全靠它。
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) ref.read(otpProvider.notifier).setFlow(widget.flow);
    });
  }

  @override
  void dispose() {
    _phoneController.dispose();
    _emailController.dispose();
    _codeController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _sendCode() async {
    if (!await _ensureAgreed()) return;
    final ok = await ref.read(otpProvider.notifier).sendCode();
    if (!mounted) return;
    if (ok) {
      setState(() => _codeSent = true);
    } else {
      // 「该账号已注册」「该账号尚未注册」都从这里出来——
      // 服务端按 purpose 判的，文案已经是给用户看的话。
      final err = ref.read(otpProvider).error;
      if (err != null) showToast(context, err, error: true);
    }
  }

  Future<void> _submit() async {
    final l = L.of(context);
    if (!await _ensureAgreed()) return;
    final n = ref.read(otpProvider.notifier);
    final code = _codeController.text;
    final ok = _isRegister
        ? await n.register(code)
        : await n.resetPassword(code);
    if (!mounted) return;
    if (ok) {
      // 注册:登录态已写入，router 的 redirect 守卫会把人带去引导页。
      // 登录态下改密码:守卫不会动，自己退回「账号与安全」。
      if (!_isRegister && Navigator.of(context).canPop()) {
        showToast(context, l.authPasswordUpdated);
        Navigator.of(context).pop();
      }
      return;
    }
    final err = ref.read(otpProvider).error;
    if (err != null) showToast(context, err, error: true);
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final otp = ref.watch(otpProvider);
    final channel = effectiveChannel(ref, otp.channel);
    final codeFilled = _codeController.text.length == _codeLength;

    final keyboardUp = MediaQuery.viewInsetsOf(context).bottom > 0;

    return Scaffold(
      appBar: NavBar(
        title: _isRegister ? l.authRegisterTitle : l.authResetTitle,
      ),
      body: SafeArea(
        child: Column(
          children: [
            // 可滚动区：原型 .bd gap s3，全程 12pt 等距，没有大小节奏。
            Expanded(
              child: ListView(
                keyboardDismissBehavior:
                    ScrollViewKeyboardDismissBehavior.onDrag,
                padding: const EdgeInsets.fromLTRB(
                  Dim.gutter,
                  Dim.s3,
                  Dim.gutter,
                  Dim.s4,
                ),
                children: [
                  // 注册可以自由切手机/邮箱；找回密码时标识是「要找回哪个账号」，同样要能切。
                  ChannelTabs(
                    current: channel,
                    onSelect: (v) {
                      ref.read(otpProvider.notifier).setChannel(v);
                      // 换了渠道，之前那个码就不对应了。
                      setState(() => _codeSent = false);
                    },
                  ),
                  if (ChannelTabs.visible(ref)) const SizedBox(height: Dim.s3),
                  if (channel == OtpChannel.email)
                    EmailField(
                      controller: _emailController,
                      onChanged: ref.read(otpProvider.notifier).setEmail,
                    )
                  else
                    PhoneField(
                      controller: _phoneController,
                      dialCode: otp.dialCode,
                      dialCodes: _dialCodes,
                      onDialCode: ref.read(otpProvider.notifier).setDialCode,
                      onChanged: ref.read(otpProvider.notifier).setPhone,
                    ),
                  const SizedBox(height: Dim.s3),

                  if (!_codeSent)
                    AppButton(
                      label: l.authSendCode,
                      loading: otp.busy,
                      onTap: otp.canSend ? _sendCode : null,
                    )
                  else ...[
                    _CodeField(
                      controller: _codeController,
                      length: _codeLength,
                      onChanged: (_) => setState(() {}),
                    ),
                    const SizedBox(height: Dim.s3),
                    GestureDetector(
                      behavior: HitTestBehavior.opaque,
                      onTap: otp.secondsLeft == 0 ? _sendCode : null,
                      child: Text(
                        otp.secondsLeft > 0
                            ? l.authResendIn(otp.secondsLeft)
                            : l.authResend,
                        style: TextStyle(
                          fontSize: Dim.t2,
                          height: 1.5,
                          fontWeight: otp.secondsLeft > 0
                              ? FontWeight.w400
                              : FontWeight.w700,
                          color: otp.secondsLeft > 0 ? c.ink3 : c.brand,
                        ),
                      ),
                    ),
                    const SizedBox(height: Dim.s3),
                    PasswordField(
                      controller: _passwordController,
                      hint: _isRegister
                          ? l.authPasswordHint
                          : l.authNewPasswordHint,
                      helper: otp.password.isEmpty || otp.passwordValid
                          ? l.authSetPasswordHint
                          : l.authPasswordTooShort,
                      onChanged: ref.read(otpProvider.notifier).setPassword,
                    ),
                    const SizedBox(height: Dim.s3),
                    AppButton(
                      label: _isRegister ? l.authRegisterDone : l.authResetDone,
                      loading: otp.busy,
                      onTap: codeFilled && otp.canSubmitPassword
                          ? _submit
                          : null,
                    ),
                  ],
                ],
              ),
            ),
            // 固定区：协议钉在屏底（原型 `margin-top:auto`），键盘弹起时让位。
            if (_isRegister && !keyboardUp)
              Padding(
                padding: const EdgeInsets.fromLTRB(
                  Dim.gutter,
                  Dim.s3,
                  Dim.gutter,
                  Dim.s4,
                ),
                child: AuthAgreement(
                  checked: _agreed,
                  onChanged: (v) => setState(() => _agreed = v),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// 6 格验证码。与 A3 同款：一个透明 TextField 承接输入，格子只负责显示，
/// 这样系统短信自动填充、粘贴、退格全都白拿。
class _CodeField extends StatelessWidget {
  const _CodeField({
    required this.controller,
    required this.length,
    required this.onChanged,
  });

  final TextEditingController controller;
  final int length;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final text = controller.text;

    return Stack(
      children: [
        Row(
          children: [
            for (var i = 0; i < length; i++) ...[
              if (i > 0) const SizedBox(width: Dim.s2),
              Expanded(
                child: Container(
                  // 原型 `.otp div`：38px → 56pt；当前格描边用 brand，已填格 ink3。
                  height: 56,
                  decoration: BoxDecoration(
                    color: c.surface,
                    borderRadius: Dim.brButton,
                    border: Border.all(
                      color: i == text.length
                          ? c.brand
                          : i < text.length
                          ? c.ink3
                          : c.line,
                      width: 1.5,
                    ),
                  ),
                  alignment: Alignment.center,
                  child: Text(
                    i < text.length ? text[i] : '',
                    style: TextStyle(
                      fontSize: Dim.t5,
                      fontWeight: FontWeight.w800,
                      height: 1.2,
                      color: c.ink,
                    ),
                  ),
                ),
              ),
            ],
          ],
        ),
        Positioned.fill(
          child: Opacity(
            opacity: 0,
            child: TextField(
              controller: controller,
              onChanged: onChanged,
              keyboardType: TextInputType.number,
              autofillHints: const [AutofillHints.oneTimeCode],
              inputFormatters: [
                FilteringTextInputFormatter.digitsOnly,
                LengthLimitingTextInputFormatter(length),
              ],
              showCursor: false,
              decoration: const InputDecoration(border: InputBorder.none),
            ),
          ),
        ),
      ],
    );
  }
}
