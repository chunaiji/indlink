import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';

const _maskColor = Color(0x94081420);

/// 主动呼出的底部弹层（礼物面板 / 举报面板）。
///
/// 进场有弹性、出场快 100ms——关闭要利落，打开可以有弹性。
/// 下拉超过 40% 或点遮罩关闭。
Future<T?> showAppSheet<T>(
  BuildContext context, {
  required WidgetBuilder builder,
  bool dismissible = true,
}) {
  return showModalBottomSheet<T>(
    context: context,
    isScrollControlled: true,
    isDismissible: dismissible,
    enableDrag: dismissible,
    barrierColor: _maskColor,
    backgroundColor: Colors.transparent,
    sheetAnimationStyle: AnimationStyle(
      duration: Motion.sheetIn,
      curve: Motion.spring,
      reverseDuration: Motion.sheetOut,
      reverseCurve: Motion.exit,
    ),
    builder: (ctx) => SheetShell(child: builder(ctx)),
  );
}

/// 弹层外壳（原型 `.sheet`）：r5 顶圆角、抓手 45×4.5、padding s2 s4 s4。
///
/// 高度由内容决定，但封顶屏高 85%（规范 §6），超出即内部滚动；
/// 底部 = s4 + 安全区 + 键盘高度（相加不取大，理由见 BottomActionBar），调用方不要再包 SafeArea。
class SheetShell extends StatelessWidget {
  const SheetShell({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final inset = MediaQuery.viewPaddingOf(context).bottom;
    final keyboard = MediaQuery.viewInsetsOf(context).bottom;
    return Container(
      constraints: BoxConstraints(
        maxHeight: MediaQuery.sizeOf(context).height * .85,
      ),
      decoration: BoxDecoration(
        color: c.surface,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(Dim.r5)),
        boxShadow: Shadows.sheet(),
      ),
      padding: EdgeInsets.fromLTRB(
        Dim.gutter,
        Dim.s2,
        Dim.gutter,
        Dim.s4 + inset + keyboard,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Center(
            child: Container(
              width: 45,
              height: 4.5,
              margin: const EdgeInsets.only(bottom: Dim.s3),
              decoration: BoxDecoration(
                color: c.line,
                borderRadius: Dim.brPill,
              ),
            ),
          ),
          Flexible(child: SingleChildScrollView(child: child)),
        ],
      ),
    );
  }
}

/// 被动弹出的模态（次数用尽 / 余额不足）。
///
/// **不要从底部滑入**——会和主动呼出的弹层混淆。这里是 scale 0.92→1 + 淡入。
Future<T?> showAppModal<T>(
  BuildContext context, {
  required WidgetBuilder builder,
  bool dismissible = true,
}) {
  return showGeneralDialog<T>(
    context: context,
    barrierDismissible: dismissible,
    barrierLabel: MaterialLocalizations.of(context).modalBarrierDismissLabel,
    barrierColor: _maskColor,
    transitionDuration: Motion.modal,
    pageBuilder: (ctx, _, _) => Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: Dim.gutter),
        child: builder(ctx),
      ),
    ),
    transitionBuilder: (ctx, anim, _, child) {
      final curved = CurvedAnimation(parent: anim, curve: Motion.spring);
      return FadeTransition(
        opacity: anim,
        child: ScaleTransition(
          scale: Tween<double>(begin: 0.92, end: 1).animate(curved),
          child: child,
        ),
      );
    },
  );
}

/// 模态卡片骨架（`.modal`）：图标 + 标题 + 正文 + 动作区。
///
/// **必须可关闭**（右上角 ✕ + 点遮罩）——它是转化入口，不是死路。
class ModalCard extends StatelessWidget {
  const ModalCard({
    super.key,
    required this.title,
    required this.body,
    required this.actions,
    this.icon,
    this.iconWidget,
    this.onClose,
  });

  final String title;
  final Widget body;
  final List<Widget> actions;
  final IconData? icon;
  final Widget? iconWidget;
  final VoidCallback? onClose;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    // 原型 .modal：r5、0 18 44 .36 投影；.head 居中 + 金色渐变顶、图标 45 gold、
    // h4 t5、p t2 ink2 lh1.65；.body padding 0 s4 s4 gap s2；✕ 20px → 30 圆。
    return Material(
      color: c.surface,
      borderRadius: Dim.brModal,
      clipBehavior: Clip.antiAlias,
      child: DecoratedBox(
        decoration: BoxDecoration(boxShadow: Shadows.modal()),
        child: Stack(
          children: [
            Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Container(
                  padding: const EdgeInsets.fromLTRB(
                    Dim.s4,
                    Dim.s5,
                    Dim.s4,
                    Dim.s3,
                  ),
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      begin: Alignment.topCenter,
                      end: Alignment.bottomCenter,
                      colors: [
                        c.coin.withValues(alpha: .26),
                        c.coin.withValues(alpha: 0),
                      ],
                    ),
                  ),
                  child: Column(
                    children: [
                      if (iconWidget != null)
                        iconWidget!
                      else if (icon != null)
                        Icon(icon, size: 45, color: c.gold),
                      if (iconWidget != null || icon != null)
                        const SizedBox(height: Dim.s2),
                      Text(
                        title,
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: Dim.t5,
                          fontWeight: FontWeight.w800,
                          color: c.ink,
                          height: 1.25,
                          letterSpacing: -0.2,
                        ),
                      ),
                      const SizedBox(height: Dim.s1),
                      DefaultTextStyle(
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: Dim.t2,
                          height: 1.65,
                          color: c.ink2,
                        ),
                        child: body,
                      ),
                    ],
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.fromLTRB(
                    Dim.s4,
                    Dim.s2,
                    Dim.s4,
                    Dim.s4,
                  ),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      for (var i = 0; i < actions.length; i++) ...[
                        if (i > 0) const SizedBox(height: Dim.s2),
                        actions[i],
                      ],
                    ],
                  ),
                ),
              ],
            ),
            Positioned(
              right: Dim.s3,
              top: Dim.s3,
              child: InkResponse(
                onTap: onClose ?? () => Navigator.of(context).maybePop(),
                radius: 22,
                child: Container(
                  key: const ValueKey('modal-close'),
                  width: 30,
                  height: 30,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: c.ink.withValues(alpha: .09),
                  ),
                  child: Icon(Icons.close_rounded, size: 18, color: c.ink3),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 轻提示。成功与失败共用一个出口，避免各页面各写一套。
void showToast(BuildContext context, String message, {bool error = false}) {
  final c = context.c;
  final messenger = ScaffoldMessenger.maybeOf(context);
  if (messenger == null) return;
  messenger
    ..hideCurrentSnackBar()
    ..showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: error ? c.warn : c.ink,
        duration: const Duration(milliseconds: 2200),
        margin: const EdgeInsets.all(Dim.gutter),
      ),
    );
}
