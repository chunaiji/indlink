import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../domain/models/engagement.dart';
import '../../domain/models/user.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/states.dart';
import 'me_controller.dart';

/// E1 关系。
///
/// 进度条取 `Relation.StrengthScore`，文案取 `Relation.Stage`。
/// 强度分由「互动次数 × 时间衰减」算出，30 天不互动会回落。
/// 类别：列表页（规范 §6）
/// 固定区：NavBar、Tab 胶囊行
/// 弹性区：无
/// 可滚动区：关系列表 + 说明条
/// 键盘：无
class RelationsPage extends ConsumerStatefulWidget {
  const RelationsPage({super.key});

  @override
  ConsumerState<RelationsPage> createState() => _RelationsPageState();
}

class _RelationsPageState extends ConsumerState<RelationsPage> {
  String _tab = 'liked-me';

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final list = ref.watch(relationsProvider(_tab));

    return Scaffold(
      appBar: NavBar(title: l.relationTitle),
      body: Column(
        children: [
          const SizedBox(height: Dim.s3),
          ChipBar(
            children: [
              for (final (key, label) in [
                ('liked-me', l.relationTabLikedMe(list.value?.length ?? 0)),
                ('i-liked', l.relationTabILiked(list.value?.length ?? 0)),
                ('viewed', l.relationTabViewedMe),
              ])
                PillChip(
                  label: label,
                  selected: _tab == key,
                  onTap: () => setState(() => _tab = key),
                ),
            ],
          ),
          const SizedBox(height: Dim.s3),
          Expanded(
            child: AsyncView(
              value: list,
              onRetry: () => ref.invalidate(relationsProvider(_tab)),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const Padding(
                padding: EdgeInsets.symmetric(horizontal: Dim.gutter),
                child: SkeletonRows(count: 5),
              ),
              data: (items) => ListView(
                padding: context.pagePadding(top: 0, bottom: Dim.s5),
                children: [
                  for (final item in items)
                    _RelationRow(
                      item: item,
                      onTap: () =>
                          context.push(Routes.userProfile(item.user.id)),
                    ),
                  const SizedBox(height: Dim.s3),
                  NoticeBanner(
                    icon: Icons.lightbulb_outline_rounded,
                    text: l.relationStrengthHint,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _RelationRow extends StatelessWidget {
  const _RelationRow({required this.item, required this.onTap});

  final RelationItem item;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final stageLabel = switch (item.stage) {
      RelationStage.familiar => l.relationStageFamiliar,
      RelationStage.known => l.relationStageKnown,
      RelationStage.stranger => l.relationStageStranger,
    };

    // 复用会话行版式（.rws），右侧换成关系强度条 + 数值。
    return ConversationRow(
      leading: AvatarRing.of(item.user, size: 44),
      title: item.user.nickname,
      subtitle:
          '${l.relationInteractions(item.interactionCount)} · $stageLabel',
      subtitleColor: c.ink2,
      onTap: onTap,
      trailing: Column(
        crossAxisAlignment: CrossAxisAlignment.end,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          ThinProgressBar(
            value: item.strength / 100,
            width: 60,
            height: 4,
            color: item.strength >= 60 ? c.brand : c.mist,
          ),
          const SizedBox(height: 3),
          Text(
            '${item.strength}',
            style: TextStyle(fontSize: Dim.t1, color: c.ink3, height: 1.4),
          ),
        ],
      ),
    );
  }
}
