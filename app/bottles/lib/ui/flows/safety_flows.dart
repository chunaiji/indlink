import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../domain/models/chat.dart';
import '../../features/chat/chat_controller.dart';
import '../../l10n/app_localizations.dart';
import '../widgets/buttons.dart';
import '../widgets/chips.dart';
import '../widgets/overlays.dart';

/// G3 举报与拉黑。
///
/// 合规硬要求（App Store 1.2）：举报入口必须出现在**聊天、瓶子详情、动态、用户主页**四处，
/// 缺一处都可能被拒审。所以它是一个共享 flow，而不是某个页面的私有实现。
///
/// 返回 true 表示对方已被拉黑，调用方应退出当前会话/页面。
Future<bool> showReportSheet(
  BuildContext context,
  WidgetRef ref, {
  required String userId,
  required String userName,
}) async {
  final result = await showAppSheet<bool>(
    context,
    builder: (ctx) =>
        _ReportSheet(userId: userId, userName: userName, ref: ref),
  );
  return result ?? false;
}

class _ReportSheet extends StatefulWidget {
  const _ReportSheet({
    required this.userId,
    required this.userName,
    required this.ref,
  });

  final String userId;
  final String userName;
  final WidgetRef ref;

  @override
  State<_ReportSheet> createState() => _ReportSheetState();
}

class _ReportSheetState extends State<_ReportSheet> {
  ReportReason _reason = ReportReason.spam;
  bool _busy = false;

  Future<void> _submit({required bool report}) async {
    setState(() => _busy = true);
    final repo = widget.ref.read(chatRepoProvider);
    final message = report
        ? L.of(context).safetyReported
        : L.of(context).safetyBlocked;
    final navigator = Navigator.of(context);
    final messenger = ScaffoldMessenger.of(context);
    final barColor = context.c.ink;

    if (report) await repo.report(widget.userId, _reason);
    await repo.block(widget.userId);
    widget.ref.invalidate(conversationsProvider);

    if (!mounted) return;
    navigator.pop(true);
    messenger
      ..hideCurrentSnackBar()
      ..showSnackBar(
        SnackBar(
          content: Text(message),
          backgroundColor: barColor,
          margin: const EdgeInsets.all(Dim.gutter),
        ),
      );
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final reasons = <(ReportReason, String)>[
      (ReportReason.porn, l.safetyReasonPorn),
      (ReportReason.spam, l.safetyReasonSpam),
      (ReportReason.harassment, l.safetyReasonHarass),
      (ReportReason.minor, l.safetyReasonMinor),
    ];

    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l.safetyReportUser(widget.userName),
          style: TextStyle(
            fontSize: Dim.t3,
            fontWeight: FontWeight.w800,
            color: c.ink,
          ),
        ),
        const SizedBox(height: Dim.s3),
        // 原型 .pay 单选行：50 高，选中 brand 描边 + 实心圆点。
        for (final (reason, label) in reasons) ...[
          RadioRow(
            label: label,
            selected: _reason == reason,
            onTap: () => setState(() => _reason = reason),
          ),
          const SizedBox(height: Dim.s2),
        ],
        const SizedBox(height: Dim.s2),
        Row(
          children: [
            Expanded(
              child: AppButton(
                label: l.safetyBlockOnly,
                kind: BtnKind.ghost,
                onTap: _busy ? null : () => _submit(report: false),
              ),
            ),
            const SizedBox(width: Dim.s2),
            Expanded(
              child: AppButton(
                label: l.safetyReportAndBlock,
                kind: BtnKind.danger,
                loading: _busy,
                onTap: _busy ? null : () => _submit(report: true),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

/// 右上角 ⋯ 菜单：举报入口常驻，不藏在二级页。
class SafetyMenuButton extends ConsumerWidget {
  const SafetyMenuButton({
    super.key,
    required this.userId,
    required this.userName,
    this.onBlocked,
    this.circle = false,
  });

  final String userId;
  final String userName;
  final VoidCallback? onBlocked;

  /// 压在头图上时用 36 白圆（[IconCircleButton]），和左侧的 ← 成对；
  /// 导航栏里仍是普通图标按钮。
  final bool circle;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    Future<void> open() async {
      final blocked = await showReportSheet(
        context,
        ref,
        userId: userId,
        userName: userName,
      );
      if (blocked) onBlocked?.call();
    }

    if (circle) {
      return IconCircleButton(
        icon: Icons.more_horiz_rounded,
        onTap: open,
        tooltip: l.safetyReportUser(userName),
      );
    }
    return IconButton(
      icon: Icon(Icons.more_horiz_rounded, color: c.ink),
      onPressed: open,
      tooltip: l.safetyReportUser(userName),
    );
  }
}
