import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/network/api_exception.dart';
import '../../core/platform/oauth.dart';
import '../../core/providers.dart';
import '../../domain/models/user.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../auth/auth_controller.dart';

/// A6 账号与安全（含 A6b 绑定被占用）。
///
/// 类别：列表页（规范 §6）
/// 固定区：NavBar
/// 弹性区：无
/// 可滚动区：两个分组 + 提示条
/// 键盘：无
///
/// 合规：设置页内必须有**账号删除**入口（App Store 5.1.1(v)）。
/// 至少保留一种登录方式：只剩一种时解绑入口置灰，否则用户会把自己锁在门外。
class AccountSecurityPage extends ConsumerStatefulWidget {
  const AccountSecurityPage({super.key});

  @override
  ConsumerState<AccountSecurityPage> createState() =>
      _AccountSecurityPageState();
}

class _AccountSecurityPageState extends ConsumerState<AccountSecurityPage> {
  UserProfile? _me;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final me = await ref.read(authRepoProvider).fetchProfile();
      if (mounted) setState(() => _me = me);
    } catch (_) {
      // 读失败就保持上一份；这一页没有资料也能看，只是绑定状态显示为未知。
    }
  }

  String _brand(String provider) => provider == 'apple' ? 'Apple' : 'Google';

  Future<void> _bind(String provider) async {
    if (_busy) return;
    setState(() => _busy = true);
    // 「换一个账号」的重试要等 finally 把 _busy 放开之后再发起，
    // 否则递归进来的 _bind 会被第一行的守卫原地吞掉（widget test 抓到过）。
    var retry = false;
    try {
      final client = ref.read(oauthClientProvider);
      final idToken = provider == 'apple'
          ? await client.appleIdToken()
          : await client.googleIdToken();
      await ref.read(authRepoProvider).bindOAuth(provider, idToken);
      await _load();
      if (mounted) _showBound();
    } on OAuthCancelled {
      // 用户取消，静默返回。
    } on ApiException catch (e) {
      if (!mounted) return;
      if (e.code == ErrCode.oauthAlreadyBound) {
        retry = await _showTaken(provider, e.message);
      } else {
        showToast(context, e.message, error: true);
      }
    } catch (_) {
      if (mounted) {
        showToast(context, L.of(context).accountBindFailed, error: true);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    if (retry && mounted) await _bind(provider);
  }

  Future<void> _unbind(String provider) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await ref.read(authRepoProvider).unbindOAuth(provider);
      await _load();
    } on ApiException catch (e) {
      if (mounted) showToast(context, e.message, error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _showBound() {
    final l = L.of(context);
    showAppModal<void>(
      context,
      builder: (ctx) => ModalCard(
        icon: Icons.check_circle_outline_rounded,
        title: l.accountBoundTitle,
        body: Text(l.accountBoundBody),
        actions: [
          AppButton(label: l.commonGotIt, onTap: () => Navigator.of(ctx).pop()),
        ],
      ),
    );
  }

  /// A6b：被占用。返回 true 表示用户选了「换一个账号」。
  Future<bool> _showTaken(String provider, String? message) async {
    final l = L.of(context);
    final brand = _brand(provider);
    final retry = await showAppModal<bool>(
      context,
      builder: (ctx) => ModalCard(
        icon: Icons.block_rounded,
        title: l.accountTakenTitle(brand),
        body: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              message?.isNotEmpty == true
                  ? message!
                  : l.accountTakenBody(brand),
            ),
            const SizedBox(height: Dim.s3),
            NoticeBanner(
              icon: Icons.info_outline_rounded,
              text: l.accountTakenHint,
            ),
          ],
        ),
        actions: [
          AppButton(
            label: l.commonGotIt,
            onTap: () => Navigator.of(ctx).pop(false),
          ),
          AppButton(
            label: l.authSwitchAccount(brand),
            kind: BtnKind.oauth,
            onTap: () => Navigator.of(ctx).pop(true),
          ),
        ],
      ),
    );
    return retry ?? false;
  }

  Future<void> _confirmDelete() async {
    final l = L.of(context);
    final ok = await showAppModal<bool>(
      context,
      builder: (ctx) => ModalCard(
        icon: Icons.delete_outline_rounded,
        title: l.accountDeleteTitle,
        body: Text(l.accountDeleteBody),
        actions: [
          AppButton(
            label: l.accountDeleteConfirm,
            kind: BtnKind.danger,
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
    if (ok == true) {
      await ref.read(authProvider.notifier).deleteAccount();
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final me = _me;
    final phone = me?.maskedPhone;
    final email = me?.maskedEmail;
    // 只剩一种登录方式时，那一项的解绑置灰。
    final ways = [
      phone?.isNotEmpty == true,
      email?.isNotEmpty == true,
      me?.googleBound == true,
      me?.appleBound == true,
    ].where((b) => b).length;
    final canUnbind = ways > 1;

    return Scaffold(
      appBar: NavBar(title: l.accountSecurityTitle),
      body: ListView(
        padding: context.pagePadding(),
        children: [
          SectionLabel(l.accountLoginMethods),
          const SizedBox(height: Dim.s2),
          // 原型 .lst.roomy：行高 52，标题 + 副标，右侧状态文字；不放图标。
          ListGroup(
            children: [
              ListRowItem(
                title: l.accountPhone,
                subtitle: phone?.isNotEmpty == true ? phone : l.accountNotBound,
                trailingText: phone?.isNotEmpty == true ? l.accountBound : null,
                showChevron: false,
                roomy: true,
              ),
              ListRowItem(
                title: l.accountEmail,
                subtitle: email?.isNotEmpty == true ? email : l.accountNotBound,
                trailingText: email?.isNotEmpty == true ? l.accountBound : null,
                showChevron: false,
                roomy: true,
              ),
              _OAuthRow(
                title: 'Google',
                bound: me?.googleBound == true,
                busy: _busy || me == null,
                canUnbind: canUnbind,
                onBind: () => _bind('google'),
                onUnbind: () => _unbind('google'),
              ),
              if (showAppleSignIn)
                _OAuthRow(
                  title: 'Apple',
                  bound: me?.appleBound == true,
                  busy: _busy || me == null,
                  canUnbind: canUnbind,
                  onBind: () => _bind('apple'),
                  onUnbind: () => _unbind('apple'),
                )
              else
                ListRowItem(
                  title: 'Apple',
                  subtitle: l.accountAppleIosOnly,
                  trailingText: '—',
                  showChevron: false,
                  roomy: true,
                ),
            ],
          ),
          const SizedBox(height: Dim.s4),
          SectionLabel(l.accountSecuritySection),
          const SizedBox(height: Dim.s2),
          ListGroup(
            children: [
              ListRowItem(
                title: l.accountChangePassword,
                roomy: true,
                onTap: () {
                  ref.read(otpProvider.notifier).setFlow(AuthFlow.reset);
                  context.push(Routes.forgotPassword);
                },
              ),
              ListRowItem(
                title: l.accountDelete,
                titleColor: c.warn,
                roomy: true,
                onTap: _confirmDelete,
              ),
            ],
          ),
          const SizedBox(height: Dim.s4),
          NoticeBanner(
            icon: Icons.info_outline_rounded,
            text: l.accountKeepOneHint,
          ),
        ],
      ),
    );
  }
}

/// 第三方账号行：右侧是「绑定」（brand 色）或「解绑」（置灰规则见页面注释）。
class _OAuthRow extends StatelessWidget {
  const _OAuthRow({
    required this.title,
    required this.bound,
    required this.busy,
    required this.canUnbind,
    required this.onBind,
    required this.onUnbind,
  });

  final String title;
  final bool bound;
  final bool busy;
  final bool canUnbind;
  final VoidCallback onBind;
  final VoidCallback onUnbind;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final enabled = !busy && (!bound || canUnbind);
    return ListRowItem(
      title: title,
      subtitle: bound ? l.accountBound : l.accountNotBound,
      showChevron: false,
      roomy: true,
      onTap: enabled ? (bound ? onUnbind : onBind) : null,
      trailing: Text(
        bound ? l.accountUnbind : l.accountBind,
        style: TextStyle(
          fontSize: Dim.t2,
          fontWeight: FontWeight.w700,
          height: 1.4,
          color: !enabled
              ? c.ink3.withValues(alpha: .5)
              : (bound ? c.ink3 : c.brand),
        ),
      ),
    );
  }
}
