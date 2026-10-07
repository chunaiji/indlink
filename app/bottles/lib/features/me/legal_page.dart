import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';

/// 法务文本。正文走 `GET /api/legal/:doc`，改一版文案不用重新提审。
///
/// 免鉴权——登录页的「同意用户协议」在拿到 token 之前就要能点开。
final legalDocProvider =
    FutureProvider.family<String, ({String doc, String lang})>((ref, args) {
      return ref.watch(authRepoProvider).legalDoc(args.doc, lang: args.lang);
    });

/// 用户协议 / 隐私政策。[doc] = `terms` / `privacy`。
class LegalPage extends ConsumerWidget {
  const LegalPage({super.key, required this.doc});

  final String doc;

  /// 后台没配正文时的兜底。合规页面不能开天窗——宁可跳浏览器看 H5，
  /// 也不能给用户一张白纸。
  static const _fallbackUrl = {
    'terms': 'https://ambertu.com/drift/terms',
    'privacy': 'https://ambertu.com/drift/privacy',
  };

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = L.of(context);
    final lang = Localizations.localeOf(context).languageCode;
    final key = (doc: doc, lang: lang);
    final content = ref.watch(legalDocProvider(key));

    return Scaffold(
      appBar: NavBar(title: doc == 'privacy' ? l.authPrivacy : l.authTerms),
      body: AsyncView(
        value: content,
        onRetry: () => ref.invalidate(legalDocProvider(key)),
        errorTitle: l.stateErrorTitle,
        errorBody: l.stateErrorBody,
        retryLabel: l.commonRetry,
        skeleton: const Padding(
          padding: EdgeInsets.all(Dim.gutter),
          child: SkeletonRows(count: 8),
        ),
        data: (text) {
          if (text.trim().isEmpty) {
            return EmptyState(
              icon: Icons.description_outlined,
              title: l.legalNotReadyTitle,
              body: l.legalNotReadyBody,
              ctaLabel: l.legalOpenInBrowser,
              onCta: () => _openFallback(context),
            );
          }
          return ListView(
            padding: context.pagePadding(bottom: Dim.s6),
            children: [
              // 可选中：合规文本经常要被复制去比对，禁选只会被当成心虚。
              SelectableText(
                text,
                style: TextStyle(
                  fontSize: Dim.t3,
                  height: 1.7,
                  color: context.c.ink2,
                ),
              ),
            ],
          );
        },
      ),
    );
  }

  Future<void> _openFallback(BuildContext context) async {
    final uri = Uri.parse(_fallbackUrl[doc] ?? _fallbackUrl['terms']!);
    final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (!ok && context.mounted) {
      showToast(context, L.of(context).stateLoadFailed, error: true);
    }
  }
}
