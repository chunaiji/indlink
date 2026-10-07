import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/chat.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/coin.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/states.dart';
import 'chat_controller.dart';

/// D1 会话列表。
/// 类别：列表页（规范 §6）
/// 固定区：头图（标题 + 金币 + 通知）、压在头图下沿的筛选 chips
/// 弹性区：无
/// 可滚动区：会话列表（下拉刷新）
/// 键盘：无
class ChatListPage extends ConsumerWidget {
  const ChatListPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = L.of(context);
    final conversations = ref.watch(conversationsProvider);
    final filter = ref.watch(chatFilterProvider);
    final wallet = ref.watch(walletProvider).value;
    final unread = ref.watch(unreadProvider);

    return Scaffold(
      body: Column(
        children: [
          GradientHeader(
            slim: true,
            topRow: Row(
              children: [
                Expanded(
                  child: Text(
                    l.chatTitle,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: Dim.t5,
                      fontWeight: FontWeight.w800,
                      color: Colors.white,
                      letterSpacing: -0.2,
                      height: 1.25,
                    ),
                  ),
                ),
                const SizedBox(width: Dim.s2),
                CoinChip(
                  coins: wallet?.coins ?? 0,
                  onTap: () => context.push(Routes.recharge),
                ),
                HeaderIconButton(
                  icon: Icons.notifications_none_rounded,
                  onTap: () => context.push(Routes.notifications),
                ),
              ],
            ),
          ),
          // 原型 .chs.raise 其实没有负边距：胶囊行在头图下方留 s2，不压头图。
          ChipBar(
            padding: const EdgeInsets.fromLTRB(
              Dim.gutter,
              Dim.s2,
              Dim.gutter,
              Dim.s1,
            ),
            children: [
              PillChip(
                label: l.commonAll,
                selected: filter == 'all',
                onTap: () => ref.read(chatFilterProvider.notifier).set('all'),
              ),
              PillChip(
                label: l.chatTabUnread(unread),
                tone: ChipTone.pay,
                selected: filter == 'unread',
                onTap: () =>
                    ref.read(chatFilterProvider.notifier).set('unread'),
              ),
              PillChip(
                label: l.chatTabBottleFriends,
                selected: filter == 'bottle',
                onTap: () =>
                    ref.read(chatFilterProvider.notifier).set('bottle'),
              ),
            ],
          ),
          Expanded(
            child: AsyncView(
              value: conversations,
              onRetry: () => ref.invalidate(conversationsProvider),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const Padding(
                padding: EdgeInsets.symmetric(horizontal: Dim.gutter),
                child: SkeletonRows(count: 6),
              ),
              data: (all) {
                final list = switch (filter) {
                  'unread' => all.where((c) => c.unread > 0).toList(),
                  'bottle' => all.where((c) => c.fromBottle).toList(),
                  _ => all,
                };

                if (list.isEmpty) {
                  // 空态给两条路：主按钮回瓶子主线，次按钮去发现页。
                  return EmptyState(
                    icon: Icons.forum_outlined,
                    title: l.stateEmptyChatsTitle,
                    body: l.stateEmptyChatsBody,
                    ctaLabel: l.stateEmptyChatsCta,
                    onCta: () => context.go(Routes.ocean),
                    altLabel: l.stateEmptyChatsAlt,
                    onAlt: () => context.go(Routes.discover),
                  );
                }

                return RefreshIndicator(
                  onRefresh: () =>
                      ref.read(conversationsProvider.notifier).refresh(),
                  // separated 而不是 builder：会话行本身没有边框也没有底色，
                  // 挨在一起时读不出「这是几条」，尤其头像都是圆的时候。
                  child: ListView.separated(
                    padding: const EdgeInsets.fromLTRB(
                      Dim.gutter,
                      Dim.s2,
                      Dim.gutter,
                      Dim.s5,
                    ),
                    itemCount: list.length,
                    // 分隔线从头像右侧起，让头像列在视觉上连成一条——
                    // 通栏切到底会把列表切得很碎。
                    separatorBuilder: (context, _) => Padding(
                      padding: const EdgeInsets.only(
                        left: ConversationRow.dividerIndent,
                      ),
                      child: Divider(height: 1, color: context.c.line2),
                    ),
                    itemBuilder: (context, i) => _conversationRow(
                      context,
                      list[i],
                      () => context.push(Routes.chatRoom(list[i].id)),
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

Widget _conversationRow(
  BuildContext context,
  Conversation conversation,
  VoidCallback onTap,
) {
  final c = context.c;
  final l = L.of(context);
  final peer = conversation.peer;
  final preview = switch (conversation.lastType) {
    MessageType.image => l.chatImageMessage,
    MessageType.gift => l.chatGiftSentBy(conversation.lastText),
    _ => conversation.lastText,
  };
  return ConversationRow(
    leading: AvatarRing.of(peer, size: 44),
    title: peer.deleted ? l.chatDeletedUser : peer.nickname,
    subtitle: peer.deleted ? l.chatDeletedUserHint : preview,
    subtitleColor: conversation.lastType == MessageType.gift ? c.gold : null,
    time: compactAgo(context, conversation.lastAt),
    unread: conversation.unread,
    dimmed: peer.deleted,
    onTap: onTap,
  );
}
