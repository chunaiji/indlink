import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/moment.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/coin.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/states.dart';
import 'moment_controller.dart';

/// F1 动态广场。
/// 类别：列表页（规范 §6）
/// 固定区：头图（标题 + 金币）、Tab 胶囊行、右下角 FAB
/// 弹性区：无
/// 可滚动区：动态卡片列表（下拉刷新）
/// 键盘：无
class MomentFeedPage extends ConsumerWidget {
  const MomentFeedPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = L.of(context);
    final feed = ref.watch(momentFeedProvider);
    final tab = ref.watch(momentTabProvider);
    final wallet = ref.watch(walletProvider).value;

    return Scaffold(
      // 发动态是本页主操作，用右下角 FAB 最显眼；发动态免费 → 海蓝(aqua)，不用粉色。
      // 原型 .fab：40px → 60 圆，aqua，0 6 16 .5 投影（Shadows.glow 几何）。
      floatingActionButton: DecoratedBox(
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          boxShadow: Shadows.glow(context.c.aqua, alpha: .5),
        ),
        child: SizedBox(
          width: 60,
          height: 60,
          child: FloatingActionButton(
            onPressed: () => context.push(Routes.momentPost),
            backgroundColor: context.c.aqua,
            foregroundColor: Colors.white,
            elevation: 0,
            highlightElevation: 0,
            shape: const CircleBorder(),
            child: const Icon(Icons.edit_outlined, size: 26),
          ),
        ),
      ),
      body: Column(
        children: [
          GradientHeader(
            slim: true,
            topRow: Row(
              children: [
                Expanded(
                  child: Text(
                    l.momentTitle,
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
                // 头部只留金币 pill。发动态改为右下角 FAB（本页主操作应最显眼）。
                CoinChip(
                  coins: wallet?.coins ?? 0,
                  onTap: () => context.push(Routes.recharge),
                ),
              ],
            ),
          ),
          // Tab 独立成一行，不再用负位移叠在渐变头上（长文案/多语言下会错位）。
          Padding(
            padding: const EdgeInsets.only(top: Dim.s2, bottom: Dim.s1),
            child: ChipBar(
              children: [
                for (final (key, label) in [
                  ('recommend', l.momentTabRecommend),
                  ('following', l.momentTabFollowing),
                  ('city', l.momentTabCity),
                ])
                  PillChip(
                    label: label,
                    tone: key == 'following' ? ChipTone.pay : ChipTone.plain,
                    selected: tab == key,
                    onTap: () => ref.read(momentTabProvider.notifier).set(key),
                  ),
              ],
            ),
          ),
          Expanded(
            child: AsyncView(
              value: feed,
              onRetry: () => ref.invalidate(momentFeedProvider),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const Padding(
                padding: EdgeInsets.all(Dim.gutter),
                child: SkeletonRows(count: 4),
              ),
              data: (list) {
                if (list.isEmpty) {
                  return EmptyState(
                    icon: Icons.auto_awesome_outlined,
                    title: l.stateEmptyMomentsTitle,
                    body: l.stateEmptyMomentsBody,
                    ctaLabel: l.stateEmptyMomentsCta,
                    onCta: () => context.push(Routes.momentPost),
                  );
                }
                return RefreshIndicator(
                  onRefresh: () =>
                      ref.read(momentFeedProvider.notifier).refresh(),
                  child: ListView.separated(
                    padding: const EdgeInsets.fromLTRB(
                      Dim.gutter,
                      Dim.s2,
                      Dim.gutter,
                      Dim.s5,
                    ),
                    itemCount: list.length,
                    separatorBuilder: (_, _) => const SizedBox(height: Dim.s3),
                    itemBuilder: (context, i) => MomentCard(
                      moment: list[i],
                      onTap: () =>
                          context.push(Routes.momentDetail(list[i].id)),
                      onLike: () => ref
                          .read(momentFeedProvider.notifier)
                          .toggleLike(list[i].id),
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

/// 动态卡片。F1 与个人主页共用。
class MomentCard extends StatelessWidget {
  const MomentCard({
    super.key,
    required this.moment,
    this.onTap,
    this.onLike,
    this.onGift,
  });

  final Moment moment;
  final VoidCallback? onTap;
  final VoidCallback? onLike;
  final VoidCallback? onGift;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    return AppCard(
      onTap: onTap,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              GestureDetector(
                onTap: () => context.push(Routes.userProfile(moment.author.id)),
                child: AvatarRing.of(moment.author, size: 44),
              ),
              const SizedBox(width: Dim.s2),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      moment.author.nickname,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: Dim.t3,
                        fontWeight: FontWeight.w700,
                        height: 1.4,
                        color: c.ink,
                      ),
                    ),
                    const SizedBox(height: 1),
                    Text(
                      [
                        timeAgo(context, moment.createdAt),
                        if (moment.city != null) moment.city!,
                      ].join(' · '),
                      style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: Dim.s2),
          Text(
            moment.text,
            style: TextStyle(fontSize: Dim.t3, height: 1.6, color: c.ink),
          ),
          if (moment.images.isNotEmpty) ...[
            const SizedBox(height: Dim.s3),
            _ImageGrid(images: moment.images),
          ],
          const SizedBox(height: Dim.s3),
          Row(
            children: [
              _Action(
                icon: moment.liked
                    ? Icons.favorite_rounded
                    : Icons.favorite_border_rounded,
                label: '${moment.likeCount}',
                color: moment.liked ? c.brand : c.ink3,
                onTap: onLike,
              ),
              const SizedBox(width: Dim.s5),
              _Action(
                icon: Icons.chat_bubble_outline_rounded,
                label: '${moment.commentCount}',
                color: c.ink3,
                onTap: onTap,
              ),
              const SizedBox(width: Dim.s5),
              // 原型 .bar5 三项同色 ink3；粉色在卡片上只留给「漂了 N 天」这类标记。
              _Action(
                icon: Icons.card_giftcard_rounded,
                label: l.momentGift,
                color: c.ink3,
                onTap: onGift ?? onTap,
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _ImageGrid extends StatelessWidget {
  const _ImageGrid({required this.images});

  final List<String> images;

  /// 原型 .mcd .pics gap 3px → 4.5。
  static const _gap = 4.5;

  @override
  Widget build(BuildContext context) {
    final count = images.length.clamp(1, 9);
    // 原型 .pics 是一行 flex：2～3 张并排各占等宽；4 张起按 3 列换行（九宫格）。
    // 之前 ≤4 张走 2 列，三张图就成了两张半屏大图 + 一张，真机上比原型大一倍。
    final cols = count == 1 ? 1 : (count <= 3 ? count : 3);
    final cacheWidth =
        (MediaQuery.sizeOf(context).width /
                cols *
                MediaQuery.devicePixelRatioOf(context))
            .round();

    Widget tile(int i) => ClipRRect(
      key: ValueKey('moment-pic-$i'),
      borderRadius: Dim.brTag,
      child: images[i].isEmpty
          ? ColoredBox(color: context.c.sea)
          : CachedNetworkImage(
              imageUrl: images[i],
              fit: BoxFit.cover,
              memCacheWidth: cacheWidth,
              placeholder: (_, _) => ColoredBox(color: context.c.sea),
              errorWidget: (_, _, _) => ColoredBox(color: context.c.sea),
            ),
    );

    if (count == 1) return AspectRatio(aspectRatio: 16 / 10, child: tile(0));

    final rows = <Widget>[];
    for (var start = 0; start < count; start += cols) {
      final cells = <Widget>[];
      for (var j = 0; j < cols; j++) {
        if (j > 0) cells.add(const SizedBox(width: _gap));
        final i = start + j;
        cells.add(
          Expanded(
            child: i < count
                ? AspectRatio(aspectRatio: 1, child: tile(i))
                : const SizedBox.shrink(),
          ),
        );
      }
      if (rows.isNotEmpty) rows.add(const SizedBox(height: _gap));
      rows.add(Row(children: cells));
    }
    return Column(children: rows);
  }
}

class _Action extends StatelessWidget {
  const _Action({
    required this.icon,
    required this.label,
    required this.color,
    this.onTap,
  });

  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: color),
          const SizedBox(width: 4),
          Text(
            label,
            style: TextStyle(fontSize: Dim.t2, color: color, height: 1.4),
          ),
        ],
      ),
    );
  }
}
