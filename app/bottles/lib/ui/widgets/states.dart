import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../core/network/api_exception.dart';
import 'buttons.dart';

/// 骨架块（`.skel`）。**骨架屏，不是转圈**——数据到位时不能发生布局跳动，
/// 所以骨架必须与真实内容同尺寸。
class Skeleton extends StatefulWidget {
  const Skeleton({
    super.key,
    this.width,
    this.height = 14,
    this.radius = Dim.r1,
    this.shape = BoxShape.rectangle,
  });

  const Skeleton.circle({super.key, required double size})
    : width = size,
      height = size,
      radius = 0,
      shape = BoxShape.circle;

  final double? width;
  final double height;
  final double radius;
  final BoxShape shape;

  @override
  State<Skeleton> createState() => _SkeletonState();
}

class _SkeletonState extends State<Skeleton>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1500),
  )..repeat(reverse: true);

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return FadeTransition(
      opacity: Tween<double>(begin: .5, end: 1).animate(_ctrl),
      child: Container(
        width: widget.width,
        height: widget.height,
        decoration: BoxDecoration(
          color: c.line2,
          shape: widget.shape,
          borderRadius: widget.shape == BoxShape.circle
              ? null
              : BorderRadius.circular(widget.radius),
        ),
      ),
    );
  }
}

/// 列表骨架：头像 + 两行文字，与 `ListRowItem` 同构。
class SkeletonRows extends StatelessWidget {
  const SkeletonRows({super.key, this.count = 5});

  final int count;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        for (var i = 0; i < count; i++)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: Dim.s2),
            child: Row(
              children: [
                const Skeleton.circle(size: 44),
                const SizedBox(width: Dim.s3),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Skeleton(width: 110 + (i % 3) * 30, height: 14),
                      const SizedBox(height: Dim.s2),
                      Skeleton(width: 180 + (i % 2) * 40, height: 12),
                    ],
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }
}

/// 空态（`.empty2`）。
///
/// **空态必须给下一步动作。**别画一张「暂无数据」灰图——它不告诉用户该干什么。
class EmptyState extends StatelessWidget {
  const EmptyState({
    super.key,
    required this.icon,
    required this.title,
    required this.body,
    this.ctaLabel,
    this.onCta,
    this.altLabel,
    this.onAlt,
    this.onDark = false,
  });

  final IconData icon;
  final String title;
  final String body;
  final String? ctaLabel;
  final VoidCallback? onCta;

  /// 第二条出路。只给一条会把人堵死在一个入口。
  final String? altLabel;
  final VoidCallback? onAlt;

  /// 压在海面上时整体转白。
  final bool onDark;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final titleColor = onDark ? Colors.white : c.ink;
    final bodyColor = onDark ? Colors.white.withValues(alpha: .85) : c.ink3;
    // 原型 .empty2：图标 38px → 57，h5 t5 mt s3，p t2 lh1.7 mt s1，按钮 mt s5 撑满。
    // 矮屏（SE 667）是弹性区收缩：图标缩到 40，不换布局（规范 §4.3）。
    final iconSize = Breakpoints.isShort(context) ? 40.0 : 57.0;

    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: Dim.s6),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Icon(
              icon,
              size: iconSize,
              color: onDark ? Colors.white.withValues(alpha: .88) : c.mist,
            ),
            const SizedBox(height: Dim.s3),
            Text(
              title,
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: Dim.t5,
                fontWeight: FontWeight.w800,
                height: 1.25,
                letterSpacing: -0.2,
                color: titleColor,
              ),
            ),
            const SizedBox(height: Dim.s1),
            Text(
              body,
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: Dim.t2, height: 1.7, color: bodyColor),
            ),
            if (ctaLabel != null) ...[
              const SizedBox(height: Dim.s5),
              AppButton(label: ctaLabel!, onTap: onCta),
            ],
            if (altLabel != null) ...[
              const SizedBox(height: Dim.s2),
              AppButton(
                label: altLabel!,
                kind: onDark ? BtnKind.glass : BtnKind.oauth,
                onTap: onAlt,
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// 失败态。加载超过 8s 也转到这里。
class FailureState extends StatelessWidget {
  const FailureState({
    super.key,
    required this.title,
    required this.body,
    required this.ctaLabel,
    this.onRetry,
    this.offline = false,
  });

  final String title;
  final String body;
  final String ctaLabel;
  final VoidCallback? onRetry;
  final bool offline;

  @override
  Widget build(BuildContext context) {
    return EmptyState(
      icon: offline ? Icons.wifi_off_rounded : Icons.cloud_off_rounded,
      title: title,
      body: body,
      ctaLabel: ctaLabel,
      onCta: onRetry,
    );
  }
}

/// <300ms 不显示任何加载态——闪一下比多等一会儿更烦。
class DelayedLoading extends StatefulWidget {
  const DelayedLoading({
    super.key,
    required this.child,
    this.delay = Motion.loadingDelay,
  });

  final Widget child;
  final Duration delay;

  @override
  State<DelayedLoading> createState() => _DelayedLoadingState();
}

class _DelayedLoadingState extends State<DelayedLoading> {
  bool _show = false;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _timer = Timer(widget.delay, () {
      if (mounted) setState(() => _show = true);
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedOpacity(
      opacity: _show ? 1 : 0,
      duration: Motion.skeletonFade,
      child: widget.child,
    );
  }
}

/// `AsyncValue` 的三态渲染，统一「骨架 → 内容交叉淡入」与失败兜底。
class AsyncView<T> extends StatelessWidget {
  const AsyncView({
    super.key,
    required this.value,
    required this.data,
    required this.skeleton,
    this.onRetry,
    this.errorTitle = '出了点问题',
    this.errorBody = '服务器没有回应，稍后再试一次',
    this.retryLabel = '重试',
  });

  final AsyncValue<T> value;
  final Widget Function(T data) data;
  final Widget skeleton;
  final VoidCallback? onRetry;
  final String errorTitle;
  final String errorBody;
  final String retryLabel;

  @override
  Widget build(BuildContext context) {
    return value.when(
      skipLoadingOnRefresh: true,
      // 刷新中不清空旧数据，否则用户会以为内容丢了。
      skipLoadingOnReload: true,
      data: data,
      loading: () => DelayedLoading(child: skeleton),
      // Z5：断网和「服务器没回应」是两回事，图标要分开，用户才知道该去查 Wi-Fi 还是等。
      error: (e, _) => FailureState(
        title: errorTitle,
        body: errorBody,
        ctaLabel: retryLabel,
        onRetry: onRetry,
        offline: e is ApiException && e.isNetwork,
      ),
    );
  }
}
