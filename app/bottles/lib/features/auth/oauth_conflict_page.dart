import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../core/design/tokens.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/headers.dart';

/// A2k 第三方登录撞上「该邮箱已注册」。
///
/// 类别：场景页（规范 §6）
/// 固定区：NavBar、底部两个按钮（.foot，gap s2）
/// 弹性区：居中的图标 + 标题 + 正文（.empty2 版式）
/// 可滚动区：无
/// 键盘：无
///
/// 为什么要一整屏而不是一个 toast：这一屏要让用户做一件**有两步**的事——
/// 先用密码登录，再去「账号与安全」绑定。一个两秒后消失的提示传达不了这个，
/// 用户只会反复点 Google 按钮然后放弃。
///
/// 为什么不自动按邮箱合并：那等于把「Google 说这个邮箱属于他」当成账号所有权
/// 证明。即使校验了 `email_verified`，这条链路上任何疏漏都会变成账号接管。
/// 产品选择让用户自己确认，多一步换确定性。
class OAuthConflictPage extends StatelessWidget {
  const OAuthConflictPage({super.key, this.message, this.provider = 'google'});

  /// 服务端给的提示语（含邮箱）。为空时用内置文案兜底，不开天窗。
  final String? message;

  /// 'google' / 'apple'，决定标题与「换一个账号」的文案。
  final String provider;

  String get _brand => provider == 'apple' ? 'Apple' : 'Google';

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    return Scaffold(
      appBar: NavBar(title: l.authOauthNav(_brand)),
      body: SafeArea(
        bottom: false,
        child: Column(
          children: [
            Expanded(
              child: Center(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: Dim.s6),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      // 原型：56×56 粉紫渐变圆角块作图标底。
                      Center(
                        child: Container(
                          width: 56,
                          height: 56,
                          decoration: const BoxDecoration(
                            borderRadius: Dim.brCard,
                            gradient: LinearGradient(
                              begin: Alignment.topLeft,
                              end: Alignment.bottomRight,
                              colors: [
                                Color(0xFFFFD9E2),
                                Color(0xFFEAD8FF),
                                Color(0xFFCDE9F4),
                              ],
                              stops: [0, .54, 1],
                            ),
                          ),
                          child: Icon(
                            Icons.alternate_email_rounded,
                            size: 26,
                            color: c.ink2,
                          ),
                        ),
                      ),
                      const SizedBox(height: Dim.s4),
                      Text(
                        l.authConflictTitle,
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: Dim.t5,
                          fontWeight: FontWeight.w800,
                          height: 1.25,
                          letterSpacing: -0.2,
                          color: c.ink,
                        ),
                      ),
                      const SizedBox(height: Dim.s2),
                      if (message?.isNotEmpty == true)
                        Text(
                          message!,
                          textAlign: TextAlign.center,
                          style: TextStyle(
                            fontSize: Dim.t2,
                            height: 1.7,
                            color: c.ink2,
                          ),
                        ),
                      Text(
                        l.authConflictBody,
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: Dim.t2,
                          height: 1.7,
                          color: c.ink3,
                        ),
                      ),
                      const SizedBox(height: Dim.s5),
                      NoticeBanner(
                        icon: Icons.info_outline_rounded,
                        text: l.authConflictHint,
                      ),
                    ],
                  ),
                ),
              ),
            ),
            BottomActionBar(
              children: [
                AppButton(
                  label: l.authUsePassword,
                  onTap: () => context.pop(),
                ),
                AppButton(
                  label: l.authSwitchAccount(_brand),
                  kind: BtnKind.oauth,
                  onTap: () => context.pop(provider),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
