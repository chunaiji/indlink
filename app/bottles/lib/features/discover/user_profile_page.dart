import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/catalog.dart';
import '../../core/design/tokens.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/moment.dart';
import '../../domain/models/user.dart';
import '../../l10n/app_localizations.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../ui/flows/purchase_flows.dart';
import '../chat/chat_controller.dart';
import '../../ui/flows/gift_flows.dart';
import '../../ui/flows/safety_flows.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';
import '../moment/moment_controller.dart';
import 'discover_controller.dart';
import 'discover_page.dart' show startChatWith;

/// C3 用户主页。滑卡点进来，照片区做 Hero 共享元素放大。
///
/// 类别：详情页（规范 §6）
/// 固定区：底部「送礼 / 打招呼」
/// 弹性区：头图（原型 .hero 178px → 265；高 < 700 收到 200）
/// 可滚动区：资料 + 统计 + 动态预览
/// 键盘：无
class UserProfilePage extends ConsumerWidget {
  const UserProfilePage({super.key, required this.userId});

  final String userId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = L.of(context);
    final card = ref.watch(userCardProvider(userId));

    return Scaffold(
      body: AsyncView(
        value: card,
        onRetry: () => ref.invalidate(userCardProvider(userId)),
        errorTitle: l.stateErrorTitle,
        errorBody: l.stateErrorBody,
        retryLabel: l.commonRetry,
        skeleton: const Padding(
          padding: EdgeInsets.all(Dim.gutter),
          child: SkeletonRows(count: 5),
        ),
        data: (user) => _Body(user: user),
      ),
    );
  }
}

/// 在线 / 最近活跃标。
class _StatusPill extends StatelessWidget {
  const _StatusPill({required this.label, required this.live});

  final String label;
  final bool live;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: Dim.s2, vertical: 3),
      decoration: BoxDecoration(
        color: Colors.black.withValues(alpha: .35),
        borderRadius: Dim.brPill,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (live) ...[
            Container(
              width: 6,
              height: 6,
              decoration: const BoxDecoration(
                shape: BoxShape.circle,
                color: Color(0xFF35C77E),
              ),
            ),
            const SizedBox(width: 4),
          ],
          Text(
            label,
            style: const TextStyle(
              fontSize: Dim.t0,
              fontWeight: FontWeight.w700,
              color: Colors.white,
            ),
          ),
        ],
      ),
    );
  }
}

class _Body extends ConsumerWidget {
  const _Body({required this.user});

  final UserProfile user;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);

    final heroHeight = Breakpoints.isShort(context) ? 200.0 : 265.0;

    final tags = <Widget>[
      if (user.distanceKm != null)
        TagChip(
          label:
              '${user.brief.city ?? ''} · ${user.distanceKm!.toStringAsFixed(1)} km',
          tone: TagTone.pay,
          icon: Icons.place_outlined,
        ),
      for (final lang in user.languages) TagChip(label: lang),
      for (final key in user.interests)
        TagChip(label: Catalog.interest(context, key), tone: TagTone.purple),
    ];

    return Column(
      children: [
        SizedBox(
          height: heroHeight,
          child: Stack(
            children: [
              Positioned.fill(
                child: Hero(
                  tag: 'user-photo-${user.id}',
                  child: UserPhoto(user: user.brief, emojiSize: 120),
                ),
              ),
              // 原型 .hero .bar：← 与 ··· 成对，都用 .iconb 白圆（36）压在照片上。
              SafeArea(
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: Dim.s3,
                    vertical: Dim.s2,
                  ),
                  child: Row(
                    children: [
                      IconCircleButton(
                        icon: Icons.arrow_back_ios_new_rounded,
                        onTap: () => Navigator.of(context).maybePop(),
                      ),
                      const Spacer(),
                      // 合规：举报 / 拉黑入口常驻右上角，不可藏在二级页。
                      SafetyMenuButton(
                        userId: user.id,
                        userName: user.nickname,
                        circle: true,
                        onBlocked: () => Navigator.of(context).maybePop(),
                      ),
                    ],
                  ),
                ),
              ),
              // 原型 .hero .live：在线状态贴照片左下（bottom 11px → 16）。
              if (user.brief.presumedOnline)
                Positioned(
                  left: Dim.s3,
                  bottom: Dim.s4,
                  child: _StatusPill(label: l.commonOnline, live: true),
                )
              else if (user.lastActiveAt != null)
                Positioned(
                  left: Dim.s3,
                  bottom: Dim.s4,
                  child: _StatusPill(
                    label: l.commonActiveAgo(
                      timeAgo(context, user.lastActiveAt!),
                    ),
                    live: false,
                  ),
                ),
            ],
          ),
        ),
        Expanded(
          child: ListView(
            padding: const EdgeInsets.all(Dim.gutter),
            children: [
              // 名字 t6/800 可收缩，右侧魅力值金色胶囊（原型 .charm）。
              Row(
                children: [
                  Expanded(
                    child: Text(
                      '${user.nickname}, ${user.brief.age ?? 0}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: Dim.t6,
                        fontWeight: FontWeight.w800,
                        color: c.ink,
                        letterSpacing: -0.24,
                        height: 1.25,
                      ),
                    ),
                  ),
                  const SizedBox(width: Dim.s2),
                  TagChip(
                    label: '${user.charm}',
                    tone: TagTone.gold,
                    icon: Icons.auto_awesome_rounded,
                  ),
                ],
              ),
              // 下面每一块都「有内容才占位」：真机上一个空串 bio 曾让名字和统计之间
              // 空出 61pt（空 Text 也有一行高），资料越空的人页面越难看。
              if (tags.isNotEmpty) ...[
                const SizedBox(height: Dim.s3),
                Wrap(spacing: Dim.s1, runSpacing: Dim.s1, children: tags),
              ],
              if (user.bio?.isNotEmpty == true) ...[
                const SizedBox(height: Dim.s3),
                Text(
                  user.bio!,
                  style: TextStyle(
                    fontSize: Dim.t2,
                    height: 1.65,
                    color: c.ink2,
                  ),
                ),
              ],
              // 共同点：与 TA 选了相同答案的答题项（对齐原型 C2/C3 答题匹配）。
              if (user.commonPoint != null && user.commonPoint!.isNotEmpty) ...[
                const SizedBox(height: Dim.s3),
                _CommonPointChip(answer: user.commonPoint!),
              ],
              const SizedBox(height: Dim.s4),
              StatsRow(
                items: [
                  ('${user.bottleCount}', l.discoverStatBottles),
                  ('${user.momentCount}', l.discoverStatMoments),
                  (
                    switch (user.relationStage) {
                      RelationStage.familiar => l.relationStageFamiliar,
                      RelationStage.known => l.relationStageKnown,
                      RelationStage.stranger => l.relationStageStranger,
                    },
                    l.discoverStatRelation,
                  ),
                ],
              ),
              const SizedBox(height: Dim.s4),
              _UserMoments(userId: user.id, total: user.momentCount),
            ],
          ),
        ),
        // 原型 C3 的两枚按钮落在 .bd 末尾，没有 .foot 那条线：送礼（粉描边 76px → 113）
        // + 打招呼（粉实心，占满）。flat 去掉分割线，仍钉在底部保证长资料也够得着。
        BottomActionBar(
          flat: true,
          child: Row(
            children: [
              SizedBox(
                width: 113,
                child: AppButton(
                  label: l.momentGift,
                  kind: BtnKind.payGhost,
                  onTap: () => _giftFromProfile(context, ref, user),
                ),
              ),
              const SizedBox(width: Dim.s3),
              Expanded(
                child: HiButton(
                  label: l.discoverSayHi,
                  coins: ref.watch(appConfigProvider).chatStartPrice,
                  onTap: () => startChatWith(context, ref, user),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// C3「TA 的动态」预览列表：点任一条进 F3 动态详情。空 / 加载中则不显示。
class _UserMoments extends ConsumerWidget {
  const _UserMoments({required this.userId, required this.total});

  final String userId;

  /// TA 的动态总数（服务端下发），用于「共 N 条」；拉到的预览列表只取前几条。
  final int total;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final moments = ref.watch(userMomentsProvider(userId));

    return moments.maybeWhen(
      data: (list) {
        if (list.isEmpty) return const SizedBox.shrink();
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(
                  l.profileTheirMoments,
                  style: TextStyle(
                    fontSize: Dim.t1,
                    fontWeight: FontWeight.w700,
                    color: c.ink3,
                    letterSpacing: .3,
                  ),
                ),
                const Spacer(),
                Text(
                  l.profileMomentCount(total > 0 ? total : list.length),
                  style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                ),
              ],
            ),
            const SizedBox(height: Dim.s2),
            for (final m in list.take(3)) _MomentRow(moment: m),
          ],
        );
      },
      orElse: () => const SizedBox.shrink(),
    );
  }
}

/// C3「共同点」小胶囊：海蓝底 + 海蓝字（aqua = 免费/亲和维度，非付费粉色）。
class _CommonPointChip extends StatelessWidget {
  const _CommonPointChip({required this.answer});

  final String answer;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Align(
      alignment: Alignment.centerLeft,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: Dim.s2 + 2,
          vertical: 4,
        ),
        decoration: BoxDecoration(
          color: c.aqua.withValues(alpha: .13),
          borderRadius: Dim.brPill,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.favorite_rounded, size: 12, color: c.aqua),
            const SizedBox(width: 4),
            Flexible(
              child: Text(
                L.of(context).profileCommonPoint(answer),
                style: TextStyle(
                  fontSize: Dim.t1,
                  fontWeight: FontWeight.w700,
                  color: c.aqua,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 单条动态预览行：缩略图 + 摘要 + ♡ 数，点进详情。
class _MomentRow extends StatelessWidget {
  const _MomentRow({required this.moment});

  final Moment moment;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final img = moment.images.isNotEmpty ? moment.images.first : null;

    return InkWell(
      onTap: () => context.push(Routes.momentDetail(moment.id)),
      borderRadius: Dim.brCard,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: Dim.s2),
        child: Row(
          children: [
            ClipRRect(
              borderRadius: Dim.brButton,
              child: SizedBox(
                width: 57,
                height: 57,
                child: (img == null || img.isEmpty)
                    ? ColoredBox(color: c.sea)
                    : CachedNetworkImage(
                        imageUrl: img,
                        fit: BoxFit.cover,
                        placeholder: (_, _) => ColoredBox(color: c.sea),
                        errorWidget: (_, _, _) => ColoredBox(color: c.sea),
                      ),
              ),
            ),
            const SizedBox(width: Dim.s3),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    moment.text,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: Dim.t2, color: c.ink),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    '${timeAgo(context, moment.createdAt)} · ♡ ${moment.likeCount}',
                    style: TextStyle(fontSize: Dim.t0, color: c.ink3),
                  ),
                ],
              ),
            ),
            Icon(Icons.chevron_right_rounded, size: 18, color: c.ink3),
          ],
        ),
      ),
    );
  }
}

/// C3 送礼：礼物只能送进会话（服务端没有「送给某个人」的接口），
/// 先找和 TA 的现有会话；还没聊过就提示先打招呼——之前这里只弹个「已送出」，什么都没发。
Future<void> _giftFromProfile(
  BuildContext context,
  WidgetRef ref,
  UserProfile user,
) async {
  final l = L.of(context);
  final convs = await ref.read(conversationsProvider.future);
  if (!context.mounted) return;
  final conv = convs.where((c) => c.peer.id == user.id).firstOrNull;
  if (conv == null) {
    showToast(context, l.giftNeedChatFirst);
    return;
  }
  final picked = await showGiftSheet(context, ref);
  if (picked == null || !context.mounted) return;
  try {
    await ref.read(chatRepoProvider).sendGift(conv.id, picked.$1, picked.$2);
    ref.invalidate(userCardProvider(user.id));
    await ref.read(walletProvider.notifier).refresh();
    if (context.mounted) showToast(context, l.giftSentToast(picked.$1.name));
  } on ApiException catch (e) {
    if (!context.mounted) return;
    if (e.isItemInsufficient) {
      await showItemInsufficientModal(context, giftName: picked.$1.name);
    } else if (e.isInsufficientBalance) {
      await showInsufficientModal(
        context,
        ref,
        itemName: picked.$1.name,
        need: picked.$1.coins * picked.$2,
        have: ref.read(walletProvider).value?.coins ?? 0,
      );
    } else {
      showToast(context, e.message, error: true);
    }
  }
}
