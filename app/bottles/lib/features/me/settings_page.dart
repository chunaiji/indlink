import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:share_plus/share_plus.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/logging/log_export.dart';
import '../../core/logging/log_record.dart';
import '../../core/logging/logger.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../location/location_controller.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';

/// 设置。
///
/// 合规硬要求：
/// * **账号删除入口必须在 App 内**（App Store 5.1.1(v)），且是真删除而非停用——
///   后端实现为匿名化 + 释放手机号标识，钱包流水与支付订单保留（已与自然人解绑）。
/// * UGC 类 App 需提供**开发者联系方式**（App Store 1.2）。
class SettingsPage extends ConsumerStatefulWidget {
  const SettingsPage({super.key});

  @override
  ConsumerState<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends ConsumerState<SettingsPage> {
  /// 段落之间空一行。
  static const lineGap = '\n\n';

  static const _supportEmail = 'support@ambertu.com';

  /// 「关于」连点计数，7 下打开日志查看页。
  int _aboutTaps = 0;

  late bool _locationEnabled = ref.read(prefsProvider).locationEnabled;

  Future<void> _open(BuildContext context, Uri uri) async {
    final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (!ok && context.mounted) {
      showToast(context, L.of(context).stateLoadFailed, error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final locale = ref.watch(localeProvider);
    final themeMode = ref.watch(themeModeProvider);
    final cfg = ref.watch(appConfigProvider);
    final support = cfg.supportText.trim();
    final supportImage = cfg.supportImage.trim();

    return Scaffold(
      appBar: NavBar(title: l.meSettings),
      body: ListView(
        padding: context.pagePadding(),
        children: [
          ListGroup(
            children: [
              ListRowItem(
                title: l.meLanguage,
                leadingEmoji: '🌐',
                trailingText: switch (locale?.languageCode) {
                  'zh' => '简体中文',
                  'en' => 'English',
                  _ => l.meLanguageFollowSystem,
                },
                onTap: () => _pickLocale(context, ref),
              ),
              ListRowItem(
                title: l.meTheme,
                leadingEmoji: '🌓',
                trailingText: switch (themeMode) {
                  ThemeMode.light => l.meThemeLight,
                  ThemeMode.dark => l.meThemeDark,
                  ThemeMode.system => l.meThemeFollowSystem,
                },
                onTap: () => _pickTheme(context, ref),
              ),
              // 定位总开关：关掉就不取不报；开回来立刻按已授权状态静默取一次。
              ListRowItem(
                title: l.meLocation,
                leadingEmoji: '📍',
                subtitle: l.meLocationHint,
                showChevron: false,
                trailing: Switch.adaptive(
                  key: const ValueKey('settings-location-switch'),
                  value: _locationEnabled,
                  onChanged: (v) async {
                    await ref.read(prefsProvider).setLocationEnabled(v);
                    if (!mounted) return;
                    setState(() => _locationEnabled = v);
                    if (v) {
                      await ref
                          .read(locationControllerProvider.notifier)
                          .refreshIfGranted();
                    }
                  },
                ),
              ),
            ],
          ),
          const SizedBox(height: Dim.s4),
          ListGroup(
            children: [
              // 正文走接口下发（后台「App 协议」分组），改文案不用发版。
              ListRowItem(
                title: l.accountSecurityTitle,
                leadingEmoji: '🔑',
                onTap: () => context.push(Routes.accountSecurity),
              ),
              ListRowItem(
                title: l.authTerms,
                leadingEmoji: '📄',
                onTap: () => context.push(Routes.legal('terms')),
              ),
              ListRowItem(
                title: l.authPrivacy,
                leadingEmoji: '🔒',
                onTap: () => context.push(Routes.legal('privacy')),
              ),
              // UGC 合规：开发者联系方式必须可达（App Store 1.2）。
              //
              // 后台配了客服文案就弹自己的（可带二维码），没配就还是邮件。
              // 这一行**没有开关**：能被配没了就等于能被配到下架。
              if (support.isEmpty)
                ListRowItem(
                  title: l.meContact,
                  leadingEmoji: '✉️',
                  trailingText: _supportEmail,
                  showChevron: false,
                  onTap: () => _open(
                    context,
                    Uri(
                      scheme: 'mailto',
                      path: _supportEmail,
                      query: 'subject=Drift feedback',
                    ),
                  ),
                )
              else
                ListRowItem(
                  title: l.meContact,
                  leadingEmoji: '✉️',
                  onTap: () => _showSupport(context, support, supportImage),
                ),
              ListRowItem(
                title: l.meExportLogs,
                leadingEmoji: '🧾',
                subtitle: l.meExportLogsHint,
                onTap: () => _exportLogs(context),
              ),
              ListRowItem(
                title: l.meAbout,
                leadingEmoji: 'ℹ️',
                trailingText: 'v1.0.0',
                showChevron: false,
                // 连点 7 次开发者面板。藏起来是因为里面的开关
                // （强制 debug 级别 + 记录请求体）不该被普通用户误触。
                onTap: () {
                  if (++_aboutTaps < 7) return;
                  _aboutTaps = 0;
                  context.push(Routes.logViewer);
                },
              ),
            ],
          ),
          const SizedBox(height: Dim.s6),
          AppButton(
            label: l.meLogout,
            kind: BtnKind.ghost,
            onTap: () async {
              final ok = await _confirm(
                context,
                title: l.meLogout,
                body: l.meLogoutConfirm,
                confirmLabel: l.meLogout,
              );
              if (ok) await ref.read(authProvider.notifier).signOut();
            },
          ),
          const SizedBox(height: Dim.s3),
          TextButton(
            onPressed: () async {
              final ok = await _confirm(
                context,
                title: l.meDeleteAccount,
                body: l.meDeleteAccountWarn,
                confirmLabel: l.meDeleteAccountConfirm,
                danger: true,
              );
              if (!ok) return;
              await ref.read(authProvider.notifier).deleteAccount();
            },
            child: Text(
              l.meDeleteAccount,
              style: TextStyle(fontSize: Dim.t2, color: c.warn),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _pickLocale(BuildContext context, WidgetRef ref) async {
    final l = L.of(context);
    await showAppSheet<void>(
      context,
      builder: (ctx) => Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SectionLabel(l.meLanguage),
          const SizedBox(height: Dim.s2),
          ListGroup(
            children: [
              ListRowItem(
                title: l.meLanguageFollowSystem,
                showChevron: false,
                onTap: () {
                  ref.read(localeProvider.notifier).set(null);
                  Navigator.of(ctx).pop();
                },
              ),
              ListRowItem(
                title: '简体中文',
                showChevron: false,
                onTap: () {
                  ref.read(localeProvider.notifier).set(const Locale('zh'));
                  Navigator.of(ctx).pop();
                },
              ),
              ListRowItem(
                title: 'English',
                showChevron: false,
                onTap: () {
                  ref.read(localeProvider.notifier).set(const Locale('en'));
                  Navigator.of(ctx).pop();
                },
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _pickTheme(BuildContext context, WidgetRef ref) async {
    final l = L.of(context);
    await showAppSheet<void>(
      context,
      builder: (ctx) => Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SectionLabel(l.meTheme),
          const SizedBox(height: Dim.s2),
          ListGroup(
            children: [
              for (final (mode, label) in [
                (ThemeMode.system, l.meThemeFollowSystem),
                (ThemeMode.light, l.meThemeLight),
                (ThemeMode.dark, l.meThemeDark),
              ])
                ListRowItem(
                  title: label,
                  showChevron: false,
                  onTap: () {
                    ref.read(themeModeProvider.notifier).set(mode);
                    Navigator.of(ctx).pop();
                  },
                ),
            ],
          ),
        ],
      ),
    );
  }

  /// 导出前先说清楚里面有什么。
  ///
  /// 这是日志离开设备的唯一路径，用户有权知道自己在发什么；
  /// 万一 body 开关是开着的，这也是最后一道提醒。
  Future<void> _exportLogs(BuildContext context) async {
    final l = L.of(context);
    final ok = await _confirm(
      context,
      title: l.meExportLogs,
      body: Log.config.logBody
          ? [l.meExportLogsConfirm, l.meExportLogsBodyWarn].join(lineGap)
          : l.meExportLogsConfirm,
      confirmLabel: l.meExportLogsGo,
    );
    if (!ok) return;
    try {
      final file = await buildExportFile();
      await SharePlus.instance.share(
        ShareParams(files: [XFile(file.path, mimeType: 'text/plain')]),
      );
    } catch (e) {
      Log.e(LogTag.sys, 'export logs failed', error: e);
      if (context.mounted) {
        showToast(context, l.stateLoadFailed, error: true);
      }
    }
  }

  /// 后台配的客服弹框：文案必有，图片（一般是二维码）可空。
  ///
  /// 图片加载失败只吞掉不占位——二维码没出来还能照着文案里的联系方式走，
  /// 弹一个碎图图标反而像是 App 坏了。
  Future<void> _showSupport(BuildContext context, String text, String image) {
    final l = L.of(context);
    return showAppModal<void>(
      context,
      builder: (ctx) => ModalCard(
        icon: Icons.support_agent_rounded,
        title: l.meContact,
        body: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            SelectableText(text, textAlign: TextAlign.center),
            if (image.isNotEmpty) ...[
              const SizedBox(height: Dim.s4),
              ClipRRect(
                borderRadius: Dim.brCard,
                child: CachedNetworkImage(
                  imageUrl: image,
                  width: 200,
                  fit: BoxFit.contain,
                  errorWidget: (_, _, _) => const SizedBox.shrink(),
                ),
              ),
            ],
          ],
        ),
        actions: [
          AppButton(
            label: l.commonClose,
            kind: BtnKind.ghost,
            onTap: () => Navigator.of(ctx).pop(),
          ),
        ],
      ),
    );
  }

  Future<bool> _confirm(
    BuildContext context, {
    required String title,
    required String body,
    required String confirmLabel,
    bool danger = false,
  }) async {
    final l = L.of(context);
    final result = await showAppModal<bool>(
      context,
      builder: (ctx) => ModalCard(
        icon: danger ? Icons.warning_amber_rounded : Icons.logout_rounded,
        title: title,
        body: Text(body),
        actions: [
          AppButton(
            label: confirmLabel,
            kind: danger ? BtnKind.danger : BtnKind.primary,
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
    return result ?? false;
  }
}
