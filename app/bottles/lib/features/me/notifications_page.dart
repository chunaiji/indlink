import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/engagement.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/states.dart';
import 'me_controller.dart';

/// G2 通知中心。
///
/// 站内通知完全复用后端 `notify`；**推送另起炉灶**：现有 `push` 绑死微信订阅消息
/// （模板 ID、`PushSubscription.OpenID`、access_token 同源），App 端全部作废，
/// FCM + APNs 需要新建设备 token 表与证书管理。
///
/// 权限弹窗的时机：别放在启动时，放在用户**首次发出瓶子之后**，授权率高得多。
/// 类别：列表页（规范 §6）
/// 固定区：NavBar（全部已读）
/// 弹性区：无
/// 可滚动区：开启推送卡 + 通知行
/// 键盘：无
class NotificationsPage extends ConsumerWidget {
  const NotificationsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final list = ref.watch(notificationsProvider);
    final pushAsked = ref.watch(prefsProvider).pushAsked;

    return Scaffold(
      appBar: NavBar(
        title: l.notifyTitle,
        actions: [
          TextButton(
            onPressed: () =>
                ref.read(notificationsProvider.notifier).markAllRead(),
            child: Text(
              l.notifyMarkAllRead,
              style: TextStyle(
                fontSize: Dim.t2,
                fontWeight: FontWeight.w700,
                color: c.brand,
              ),
            ),
          ),
        ],
      ),
      body: AsyncView(
        value: list,
        onRetry: () => ref.invalidate(notificationsProvider),
        errorTitle: l.stateErrorTitle,
        errorBody: l.stateErrorBody,
        retryLabel: l.commonRetry,
        skeleton: const Padding(
          padding: EdgeInsets.all(Dim.gutter),
          child: SkeletonRows(count: 5),
        ),
        data: (items) => ListView(
          padding: context.pagePadding(),
          children: [
            if (!pushAsked) ...[
              BrandCard(
                padding: const EdgeInsets.all(Dim.s3),
                child: Row(
                  children: [
                    Icon(
                      Icons.notifications_active_outlined,
                      size: 26,
                      color: c.brand,
                    ),
                    const SizedBox(width: Dim.s3),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            l.notifyEnablePush,
                            style: TextStyle(
                              fontSize: Dim.t3,
                              fontWeight: FontWeight.w700,
                              color: c.ink,
                            ),
                          ),
                          const SizedBox(height: 1),
                          Text(
                            l.notifyEnablePushHint,
                            style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                          ),
                        ],
                      ),
                    ),
                    MiniButton(
                      label: l.notifyEnable,
                      onTap: () async {
                        // 设备 token 由 FCM / APNs SDK 取得后交给后端登记。
                        await ref.read(prefsProvider).setPushAsked();
                        await ref
                            .read(notifyRepoProvider)
                            .registerDevice('pending-device-token');
                      },
                    ),
                  ],
                ),
              ),
              const SizedBox(height: Dim.s4),
            ],
            if (items.isEmpty)
              Padding(
                padding: const EdgeInsets.only(top: Dim.s6),
                child: EmptyState(
                  icon: Icons.notifications_none_rounded,
                  title: l.notifyEmpty,
                  body: l.stateEmptyMomentsBody,
                ),
              )
            else
              for (final n in items) _NotifyRow(item: n),
          ],
        ),
      ),
    );
  }
}

class _NotifyRow extends StatelessWidget {
  const _NotifyRow({required this.item});

  final NotificationItem item;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    // 框内不用 emoji（原型 §0）：按类型给线性图标 + 底色圆。
    final (icon, bg, fg) = switch (item.kind) {
      NotifyKind.reply => (Icons.mail_outline_rounded, c.brand, Colors.white),
      NotifyKind.gift => (Icons.card_giftcard_rounded, c.coin, c.ink),
      NotifyKind.like => (Icons.favorite_border_rounded, c.sea, c.ink2),
      NotifyKind.comment => (
        Icons.chat_bubble_outline_rounded,
        c.aqua,
        Colors.white,
      ),
      NotifyKind.system => (Icons.campaign_outlined, c.mist, Colors.white),
    };

    return ConversationRow(
      leading: Container(
        width: 44,
        height: 44,
        decoration: BoxDecoration(color: bg, shape: BoxShape.circle),
        alignment: Alignment.center,
        child: Icon(icon, size: 22, color: fg),
      ),
      title: item.title,
      subtitle: item.body,
      subtitleColor: c.ink2,
      time: timeAgo(context, item.at),
      dimmed: item.read,
    );
  }
}
