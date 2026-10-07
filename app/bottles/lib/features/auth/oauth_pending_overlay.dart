import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/headers.dart';
import 'auth_controller.dart';

/// A2g / A2p 第三方登录进行中的整屏等待层。
///
/// 类别：场景页（规范 §6）
/// 固定区：NavBar（✕ 取消）、底部「取消」
/// 弹性区：居中的转圈 + 文案（原型 .pst 版式）
/// 可滚动区：无
/// 键盘：无
///
/// 盖在登录页上而不是 push 一个路由：SDK 的账号选择器会把 App 切到后台，
/// 回来时如果是独立路由会多一次转场闪动。
class OAuthPendingOverlay extends ConsumerWidget {
  const OAuthPendingOverlay({super.key, required this.provider});

  /// 'google' / 'apple'
  final String provider;

  String get _brand => provider == 'apple' ? 'Apple' : 'Google';

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final cancel = ref.read(otpProvider.notifier).cancelProvider;
    final reduceMotion = MediaQuery.disableAnimationsOf(context);

    return Scaffold(
      appBar: NavBar(
        title: l.authOauthNav(_brand),
        closeIcon: true,
        onBack: cancel,
      ),
      body: SafeArea(
        bottom: false,
        child: Column(
          children: [
            Expanded(
              child: Center(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: Dim.s5),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      // 原型 .spin：30px → 45 圆环，2.5px → 3 描边，line 底 aqua 顶。
                      SizedBox(
                        width: 45,
                        height: 45,
                        child: reduceMotion
                            ? DecoratedBox(
                                decoration: BoxDecoration(
                                  shape: BoxShape.circle,
                                  border: Border.all(color: c.aqua, width: 3),
                                ),
                              )
                            : CircularProgressIndicator(
                                strokeWidth: 3,
                                color: c.aqua,
                                backgroundColor: c.line,
                              ),
                      ),
                      const SizedBox(height: Dim.s4),
                      Text(
                        l.authOauthPendingTitle(_brand),
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: Dim.t5,
                          fontWeight: FontWeight.w800,
                          height: 1.25,
                          color: c.ink,
                        ),
                      ),
                      const SizedBox(height: Dim.s2),
                      Text(
                        l.authOauthPendingBody,
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: Dim.t2,
                          height: 1.65,
                          color: c.ink2,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
            BottomActionBar(
              child: AppButton(
                label: l.commonCancel,
                kind: BtnKind.oauth,
                onTap: cancel,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
