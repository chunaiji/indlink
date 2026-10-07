import 'dart:math';

import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';

/// 滑卡组。
///
/// 判定阈值走**位移与速度双通道**：只看位移则快速轻扫失效，只看速度则慢速长拖失效。
/// 跟手 1:1，旋转 = 位移 × 0.06°（上限 12°）；飞出 easeOutCubic 280ms，回弹带弹性。
class SwipeDeck extends StatefulWidget {
  const SwipeDeck({
    super.key,
    required this.itemCount,
    required this.builder,
    required this.onSwipe,
    this.confirmPass,
    this.stackCount = 3,
  });

  final int itemCount;

  /// progress 为 -1（左滑到底）~ 1（右滑到底），卡片据此显示印章。
  final Widget Function(BuildContext context, int index, double progress)
  builder;

  final void Function(bool liked) onSwipe;

  /// 左滑（跳过）前置确认。返回 true 才飞出、触发 onSwipe(false)；false 则弹回。
  /// null = 不拦截，左滑直接飞出（免费跳过）。用于付费跳过的二次确认。
  final Future<bool> Function()? confirmPass;

  final int stackCount;

  @override
  State<SwipeDeck> createState() => SwipeDeckState();
}

class SwipeDeckState extends State<SwipeDeck>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl = AnimationController(
    vsync: this,
    duration: Motion.cardFly,
  );

  double _dragX = 0;
  double _dragY = 0;
  double _flyFrom = 0;
  bool _flyingOut = false;
  bool _flyLiked = false;

  static const _velocityThreshold = 800.0;
  static const _distanceRatio = 0.25;

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  /// 供外部按钮（✕ / ♥）触发同一套飞出动画，保证两条路径手感一致。
  void swipe(bool liked) {
    if (_flyingOut || widget.itemCount == 0) return;
    if (!liked && widget.confirmPass != null) {
      _confirmThenFly();
    } else {
      _flyOut(liked);
    }
  }

  /// 付费跳过：卡片停在当前位置等二次确认，确认通过才飞出扣费、取消则弹回。
  Future<void> _confirmThenFly() async {
    final ok = await widget.confirmPass!();
    if (!mounted || _flyingOut) return;
    if (ok) {
      _flyOut(false);
    } else if (_dragX != 0 || _dragY != 0) {
      _springBack();
    }
  }

  void _flyOut(bool liked) {
    final width = context.size?.width ?? 320;
    setState(() {
      _flyingOut = true;
      _flyLiked = liked;
      _flyFrom = _dragX;
    });
    _ctrl
      ..reset()
      ..duration = Motion.cardFly
      ..forward().then((_) {
        if (!mounted) return;
        widget.onSwipe(liked);
        setState(() {
          _flyingOut = false;
          _dragX = 0;
          _dragY = 0;
        });
      });
    _target = liked ? width * 1.6 : -width * 1.6;
  }

  double _target = 0;

  void _springBack() {
    final from = _dragX;
    final fromY = _dragY;
    _ctrl
      ..reset()
      ..duration = const Duration(milliseconds: 380);
    final anim = CurvedAnimation(parent: _ctrl, curve: Curves.elasticOut);
    void listener() {
      setState(() {
        _dragX = from * (1 - anim.value);
        _dragY = fromY * (1 - anim.value);
      });
    }

    anim.addListener(listener);
    _ctrl.forward().then((_) {
      anim.removeListener(listener);
      if (!mounted) return;
      setState(() {
        _dragX = 0;
        _dragY = 0;
      });
    });
  }

  @override
  Widget build(BuildContext context) {
    if (widget.itemCount == 0) return const SizedBox.shrink();

    return LayoutBuilder(
      builder: (context, box) {
        final width = box.maxWidth;
        final progress = (_dragX / (width * _distanceRatio)).clamp(-1.0, 1.0);
        final visible = min(widget.stackCount, widget.itemCount);

        return Stack(
          alignment: Alignment.center,
          children: [
            // 后面的卡：缩放 + 下移，制造牌堆的厚度
            for (var i = visible - 1; i >= 1; i--)
              // 原型 .swc.b1 / .b2：translateY 11px / 22px → 16 / 33，scale .945 / .89。
              Transform.translate(
                offset: Offset(0, i * 16.5),
                child: Transform.scale(
                  scale: 1 - i * 0.055,
                  child: Opacity(
                    opacity: i == 1 ? .62 : .34,
                    child: widget.builder(context, i, 0),
                  ),
                ),
              ),
            _TopCard(
              dragX: _dragX,
              dragY: _dragY,
              flyingOut: _flyingOut,
              controller: _ctrl,
              flyFrom: _flyFrom,
              flyTarget: _target,
              child: widget.builder(
                context,
                0,
                _flyingOut ? (_flyLiked ? 1.0 : -1.0) : progress,
              ),
              onPanUpdate: (d) {
                if (_flyingOut) return;
                setState(() {
                  _dragX += d.delta.dx;
                  _dragY += d.delta.dy * .4;
                });
              },
              onPanEnd: (d) {
                if (_flyingOut) return;
                final v = d.velocity.pixelsPerSecond.dx;
                final passed =
                    _dragX.abs() > width * _distanceRatio ||
                    v.abs() > _velocityThreshold;
                if (!passed) {
                  _springBack();
                  return;
                }
                final liked = _dragX > 0 || v > 0;
                if (!liked && widget.confirmPass != null) {
                  // 付费跳过：卡片停在手势位置，确认通过再飞出，取消则弹回。
                  _confirmThenFly();
                } else {
                  _flyOut(liked);
                }
              },
            ),
          ],
        );
      },
    );
  }
}

class _TopCard extends StatelessWidget {
  const _TopCard({
    required this.dragX,
    required this.dragY,
    required this.flyingOut,
    required this.controller,
    required this.flyFrom,
    required this.flyTarget,
    required this.child,
    required this.onPanUpdate,
    required this.onPanEnd,
  });

  final double dragX;
  final double dragY;
  final bool flyingOut;
  final AnimationController controller;
  final double flyFrom;
  final double flyTarget;
  final Widget child;
  final GestureDragUpdateCallback onPanUpdate;
  final GestureDragEndCallback onPanEnd;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onPanUpdate: onPanUpdate,
      onPanEnd: onPanEnd,
      child: AnimatedBuilder(
        animation: controller,
        builder: (context, inner) {
          var x = dragX;
          var opacity = 1.0;
          if (flyingOut) {
            final t = Motion.emphasized.transform(controller.value);
            x = flyFrom + (flyTarget - flyFrom) * t;
            opacity = 1 - t * .6;
          }
          final angle = (x * 0.06).clamp(-12.0, 12.0) * pi / 180;
          return Transform.translate(
            offset: Offset(x, dragY),
            child: Transform.rotate(
              angle: angle,
              child: Opacity(opacity: opacity, child: inner),
            ),
          );
        },
        child: child,
      ),
    );
  }
}

/// 拖拽时的印章。
///
/// opacity 随位移线性映射，在阈值处到满不透明——让用户**松手前就知道结果**，
/// 这是滑卡最关键的反馈。
class SwipeStamp extends StatelessWidget {
  const SwipeStamp({
    super.key,
    required this.progress,
    required this.likeLabel,
    required this.passLabel,
  });

  final double progress;
  final String likeLabel;
  final String passLabel;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final liked = progress > 0;
    final opacity = progress.abs().clamp(0.0, 1.0);
    if (opacity < 0.02) return const SizedBox.shrink();

    // 原型 .stamp：rotate -14°、3px → 4 描边 #2FBF77、r2、t6/800 字距 .05em，满态 .92。
    const like = Color(0xFF2FBF77);
    final tint = liked ? like : c.ink2;
    return Opacity(
      opacity: opacity * .92,
      child: Transform.rotate(
        angle: (liked ? -14 : 14) * pi / 180,
        child: Container(
          padding: const EdgeInsets.symmetric(
            horizontal: Dim.s3,
            vertical: Dim.s1,
          ),
          decoration: BoxDecoration(
            borderRadius: Dim.brButton,
            border: Border.all(color: tint, width: 4),
          ),
          child: Text(
            liked ? likeLabel : passLabel,
            style: TextStyle(
              fontSize: Dim.t6,
              fontWeight: FontWeight.w800,
              color: tint,
              letterSpacing: 1.2,
              height: 1.2,
            ),
          ),
        ),
      ),
    );
  }
}
