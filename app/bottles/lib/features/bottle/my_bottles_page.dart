import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/bottle.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/states.dart';
import 'ocean_controller.dart';

/// B4 我的瓶子 · 轨迹。
///
/// 轨迹由 `MatchLog`（action=view/reply/like/skip）聚合，不是独立表；
/// 功能受 `bottle_trace_enabled` 控制，默认关。
/// 类别：列表页（规范 §6）
/// 固定区：NavBar、Tab 胶囊行
/// 弹性区：无
/// 可滚动区：瓶子卡片列表（展开行内嵌轨迹）
/// 键盘：无
class MyBottlesPage extends ConsumerStatefulWidget {
  const MyBottlesPage({super.key});

  @override
  ConsumerState<MyBottlesPage> createState() => _MyBottlesPageState();
}

class _MyBottlesPageState extends ConsumerState<MyBottlesPage> {
  BottleMineTab _tab = BottleMineTab.thrown;
  String? _expandedId;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final list = ref.watch(myBottlesProvider(_tab));

    return Scaffold(
      appBar: NavBar(title: l.bottleMineTitle),
      body: Column(
        children: [
          const SizedBox(height: Dim.s3),
          ChipBar(
            children: [
              for (final (tab, label) in <(BottleMineTab, String)>[
                (BottleMineTab.thrown, l.bottleTabThrown),
                (BottleMineTab.scooped, l.bottleTabScooped),
                (BottleMineTab.collected, l.bottleTabCollected),
              ])
                PillChip(
                  label: label,
                  selected: _tab == tab,
                  onTap: () => setState(() {
                    _tab = tab;
                    _expandedId = null;
                  }),
                ),
            ],
          ),
          const SizedBox(height: Dim.s3),
          Expanded(
            child: AsyncView(
              value: list,
              onRetry: () => ref.invalidate(myBottlesProvider(_tab)),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const Padding(
                padding: EdgeInsets.all(Dim.gutter),
                child: SkeletonRows(count: 4),
              ),
              data: (bottles) {
                if (bottles.isEmpty) {
                  // 三个 tab 各自的空态:此前都套用「还没扔过瓶子」,在「我捞的 /
                  // 已收藏」下文案是错的。
                  return switch (_tab) {
                    BottleMineTab.thrown => EmptyState(
                      icon: Icons.local_drink_outlined,
                      title: l.stateEmptyBottlesTitle,
                      body: l.stateEmptyBottlesBody,
                      ctaLabel: l.stateEmptyOceanCta,
                      onCta: () => context.push(Routes.bottleWrite),
                    ),
                    BottleMineTab.scooped => EmptyState(
                      icon: Icons.phishing_outlined,
                      title: l.stateEmptyScoopedTitle,
                      body: l.stateEmptyScoopedBody,
                      ctaLabel: l.stateEmptyChatsCta,
                      onCta: () => context.push(Routes.ocean),
                    ),
                    BottleMineTab.collected => EmptyState(
                      icon: Icons.favorite_border_rounded,
                      title: l.stateEmptyCollectedTitle,
                      body: l.stateEmptyCollectedBody,
                    ),
                  };
                }
                return ListView.separated(
                  padding: context.pagePadding(top: 0, bottom: Dim.s6),
                  itemCount: bottles.length,
                  separatorBuilder: (_, _) => const SizedBox(height: Dim.s3),
                  itemBuilder: (context, i) {
                    final b = bottles[i];
                    final expanded = _expandedId == b.id;
                    return Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        _BottleRowCard(
                          bottle: b,
                          onTap: () => context.push(Routes.bottleDetail(b.id)),
                          traceOpen: expanded,
                          onTrace: b.trace.isEmpty
                              ? null
                              : () => setState(
                                  () => _expandedId = expanded ? null : b.id,
                                ),
                        ),
                        if (expanded && b.trace.isNotEmpty) ...[
                          const SizedBox(height: Dim.s3),
                          _TraceCard(bottle: b),
                        ],
                      ],
                    );
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _BottleRowCard extends StatelessWidget {
  const _BottleRowCard({
    required this.bottle,
    required this.onTap,
    this.onTrace,
    this.traceOpen = false,
  });

  final Bottle bottle;
  final VoidCallback onTap;

  /// 展开 / 收起漂流轨迹。null = 该瓶没有轨迹(功能关或无节点),不显示入口。
  final VoidCallback? onTrace;
  final bool traceOpen;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final expired = bottle.status == BottleStatus.expired;

    return AppCard(
      onTap: onTap,
      dimmed: expired,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            bottle.content,
            maxLines: 3,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: Dim.t3, height: 1.6, color: c.ink),
          ),
          const SizedBox(height: Dim.s2),
          // 左边一组统计可换行，右边「轨迹 / 漂了 N 天」固定——窄屏 + 大字体下
          // 两边都不让步会把行撑爆（360 宽 × 1.2 实测溢出 135px）。
          Row(
            children: [
              Expanded(
                child: expired
                    ? Text(
                        l.bottleExpired,
                        style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                      )
                    : Wrap(
                        spacing: Dim.s3,
                        runSpacing: Dim.s1,
                        children: [
                          _Stat(
                            icon: Icons.visibility_outlined,
                            value: bottle.viewCount,
                          ),
                          _Stat(
                            icon: Icons.chat_bubble_outline_rounded,
                            value: bottle.replyCount,
                          ),
                          _Stat(
                            icon: Icons.favorite_border_rounded,
                            value: bottle.likeCount,
                          ),
                        ],
                      ),
              ),
              const SizedBox(width: Dim.s2),
              // 右侧也要能收缩换行，否则左边的 Expanded 会被挤到放不下一个统计项。
              Flexible(
                child: Wrap(
                  alignment: WrapAlignment.end,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  spacing: Dim.s3,
                  runSpacing: Dim.s1,
                  children: [
                    if (onTrace != null)
                      // 独立轨迹入口:点它展开轨迹,不冒泡到卡片的「点击跳详情」。
                      GestureDetector(
                        onTap: onTrace,
                        behavior: HitTestBehavior.opaque,
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(
                              traceOpen
                                  ? Icons.expand_less_rounded
                                  : Icons.timeline_rounded,
                              size: 14,
                              color: c.ink3,
                            ),
                            const SizedBox(width: 2),
                            Text(
                              l.bottleTraceShort,
                              style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                            ),
                          ],
                        ),
                      ),
                    Text(
                      driftedFor(context, bottle.createdAt),
                      style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _Stat extends StatelessWidget {
  const _Stat({required this.icon, required this.value});

  final IconData icon;
  final int value;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 13, color: c.ink3),
        const SizedBox(width: 3),
        Text(
          '$value',
          style: TextStyle(fontSize: Dim.t1, color: c.ink3),
        ),
      ],
    );
  }
}

class _TraceCard extends StatelessWidget {
  const _TraceCard({required this.bottle});

  final Bottle bottle;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: SectionLabel(l.bottleTraceTitle)),
              // 全 App 唯一值得截图外发的东西，不该只是一行小字。
              MiniButton(
                label: l.bottleMapTitle,
                kind: BtnKind.ghost,
                onTap: () => context.push(Routes.driftMap(bottle.id)),
              ),
            ],
          ),
          const SizedBox(height: Dim.s3),
          TimelineList(items: traceItems(context, bottle)),
          const SizedBox(height: Dim.s2),
          Text(
            l.bottleTraceSummary(bottle.viewCount, bottle.cityCount),
            style: TextStyle(fontSize: Dim.t1, color: c.ink3),
          ),
        ],
      ),
    );
  }
}

/// 轨迹节点 → (文案, 时间)。B4 与 B4s 共用这份映射。
List<(String, String)> traceItems(BuildContext context, Bottle bottle) {
  final l = L.of(context);
  return [
    for (final node in bottle.trace)
      (
        switch (node.kind) {
          TraceKind.thrown => l.bottleTraceThrown(node.city),
          TraceKind.seen => l.bottleTraceSeen(node.city, node.count),
          TraceKind.replied => l.bottleTraceReplied(node.city, node.count),
        },
        timeAgo(context, node.at),
      ),
  ];
}
