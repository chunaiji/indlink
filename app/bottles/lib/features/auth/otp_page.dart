import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/design/tokens.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import 'auth_controller.dart';

/// A3 验证码。
///
/// 验证码存 Redis 带 TTL；同号码 / 同 IP / 同设备三维频控属反作弊模块范畴，
/// 客户端这侧只做「60 秒内不可重发」的最外层拦截。
class OtpPage extends ConsumerStatefulWidget {
  const OtpPage({super.key});

  @override
  ConsumerState<OtpPage> createState() => _OtpPageState();
}

class _OtpPageState extends ConsumerState<OtpPage> {
  static const _length = 6;

  final _controller = TextEditingController();
  final _focus = FocusNode();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _focus.requestFocus());
  }

  @override
  void dispose() {
    _controller.dispose();
    _focus.dispose();
    super.dispose();
  }

  Future<void> _verify() async {
    final ok = await ref.read(otpProvider.notifier).verify(_controller.text);
    if (!mounted) return;
    if (!ok) {
      final err = ref.read(otpProvider).error;
      showToast(context, err ?? L.of(context).authInvalidCode, error: true);
    }
    // 成功后由 router 的 redirect 守卫决定去引导页还是主 Tab。
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final otp = ref.watch(otpProvider);
    final filled = _controller.text.length;

    return Scaffold(
      appBar: NavBar(title: otp.isEmail ? l.authVerifyEmail : l.authVerifyPhone),
      // 底部由 BottomActionBar 自己读安全区，这里只避让顶部。
      body: SafeArea(
        bottom: false,
        child: Column(
          children: [
            Expanded(
              child: ListView(
                // 原型 A3：.bd 内边距 16 + 标题 margin-top 12 = 28。
                padding: const EdgeInsets.fromLTRB(
                  Dim.gutter,
                  Dim.s4 + Dim.s3,
                  Dim.gutter,
                  Dim.s4,
                ),
                children: [
                  Text(l.authEnterCode, style: Theme.of(context).textTheme.headlineLarge),
                  const SizedBox(height: Dim.s2),
                  Row(
                    children: [
                      Text(
                        '${l.authSentTo} ',
                        style: TextStyle(fontSize: Dim.t2, color: c.ink3),
                      ),
                      // 邮箱比号码长，挤不下时让它先让位给「改」的入口。
                      Flexible(
                        child: Text(
                          otp.target,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: Dim.t2,
                            fontWeight: FontWeight.w700,
                            color: c.ink,
                          ),
                        ),
                      ),
                      const SizedBox(width: Dim.s2),
                      GestureDetector(
                        onTap: () => context.pop(),
                        child: Text(
                          otp.isEmail ? l.authChangeEmail : l.authChangeNumber,
                          style: TextStyle(
                            fontSize: Dim.t2,
                            fontWeight: FontWeight.w700,
                            color: c.brand,
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: Dim.s5),
                  _OtpBoxes(
                    length: _length,
                    controller: _controller,
                    focusNode: _focus,
                    onChanged: (v) {
                      setState(() {});
                      if (v.length == _length) _verify();
                    },
                  ),
                  const SizedBox(height: Dim.s4),
                  GestureDetector(
                    onTap: otp.secondsLeft == 0
                        ? () => ref.read(otpProvider.notifier).sendCode()
                        : null,
                    child: Text(
                      otp.secondsLeft > 0
                          ? l.authResendIn(otp.secondsLeft)
                          : l.authResend,
                      style: TextStyle(
                        fontSize: Dim.t2,
                        fontWeight: otp.secondsLeft > 0
                            ? FontWeight.w400
                            : FontWeight.w700,
                        color: otp.secondsLeft > 0 ? c.ink3 : c.brand,
                      ),
                    ),
                  ),
                  const SizedBox(height: Dim.s4),
                  NoticeBanner(
                    text: otp.isEmail ? l.authOtpWarningEmail : l.authOtpWarning,
                  ),
                ],
              ),
            ),
            BottomActionBar(
              child: AppButton(
                label: l.authVerifyAndLogin,
                loading: otp.busy,
                onTap: filled == _length && !otp.busy ? _verify : null,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 6 格验证码。用一个透明 TextField 承接输入，格子只负责显示——
/// 这样系统短信自动填充、粘贴、退格全都白拿。
class _OtpBoxes extends StatelessWidget {
  const _OtpBoxes({
    required this.length,
    required this.controller,
    required this.focusNode,
    required this.onChanged,
  });

  final int length;
  final TextEditingController controller;
  final FocusNode focusNode;
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
                  height: 56,
                  decoration: BoxDecoration(
                    color: c.surface,
                    borderRadius: Dim.brButton,
                    // 原型 `.otp div`：当前格描边 brand，已填格 ink3，空格 line。
                    border: Border.all(
                      color: i == text.length && focusNode.hasFocus
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
              focusNode: focusNode,
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
