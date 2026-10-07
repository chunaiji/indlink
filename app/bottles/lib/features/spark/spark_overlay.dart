import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../domain/models/spark.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/overlays.dart';

/// 同一时刻只允许一个火花弹窗。
///
/// 第二条直接丢弃而不是排队：排队意味着用户关掉一个又冒出一个，
/// 要连点两次才能回到他原本在看的页面。
bool _sparkOpen = false;

Future<void> showSparkDialog(BuildContext context, SparkEvent e) async {
  if (_sparkOpen) return;
  _sparkOpen = true;
  try {
    await showDialog<void>(
      context: context,
      barrierDismissible: true,
      builder: (_) => SparkDialog(event: e),
    );
  } finally {
    _sparkOpen = false;
  }
}

class SparkDialog extends ConsumerStatefulWidget {
  const SparkDialog({super.key, required this.event});

  final SparkEvent event;

  @override
  ConsumerState<SparkDialog> createState() => _SparkDialogState();
}

class _SparkDialogState extends ConsumerState<SparkDialog> {
  bool _busy = false;

  Future<void> _go() async {
    final e = widget.event;
    // 机器人配对：会话已建好、开场白也发了，直接跳。
    if (e.hasChat) {
      Navigator.of(context).pop();
      context.push(Routes.chatRoom(e.chatId!));
      return;
    }
    // 真人配对：先换一个免费会话。服务端会校验这对确实被匹配过。
    setState(() => _busy = true);
    try {
      final chatId = await ref.read(sparkRepoProvider).accept(e.peerId);
      if (!mounted) return;
      Navigator.of(context).pop();
      context.push(Routes.chatRoom(chatId));
    } catch (_) {
      if (!mounted) return;
      setState(() => _busy = false);
      showToast(context, L.of(context).sparkExpired, error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final e = widget.event;
    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(Dim.r4)),
      child: Padding(
        padding: const EdgeInsets.all(Dim.s5),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            CircleAvatar(
              radius: 36,
              backgroundImage: e.avatar.isEmpty ? null : NetworkImage(e.avatar),
              child: e.avatar.isEmpty ? Text(e.nickname.characters.take(1).toString()) : null,
            ),
            const SizedBox(height: Dim.s4),
            Text(
              e.title,
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: Dim.t4, fontWeight: FontWeight.w800),
            ),
            const SizedBox(height: Dim.s2),
            Text(e.body, textAlign: TextAlign.center),
            const SizedBox(height: Dim.s5),
            AppButton(label: l.sparkGoChat, loading: _busy, onTap: _busy ? null : _go),
            const SizedBox(height: Dim.s2),
            AppButton(
              label: l.sparkLater,
              kind: BtnKind.ghost,
              onTap: _busy ? null : () => Navigator.of(context).pop(),
            ),
          ],
        ),
      ),
    );
  }
}
