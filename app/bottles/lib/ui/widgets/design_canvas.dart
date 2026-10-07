import 'package:flutter/material.dart';

/// 装饰画布（规范 §5）：原型 375×762 坐标系，**按宽度等比缩放**，
/// 高度不足裁底、多余用 [extendBottom] 填满。
///
/// 画布只承载装饰（太阳、云、灯塔、瓶子、插画）。按钮、文字、头像这类功能控件
/// 不放进来，作它的兄弟节点，走 `Dim` 刻度与 SafeArea（铁律 5）。
class DesignCanvas extends StatelessWidget {
  const DesignCanvas({
    super.key,
    required this.children,
    this.designSize = const Size(375, 762),
    this.background,
    this.extendBottom,
  });

  /// 用 [CanvasPositioned] 按原型坐标定位的装饰元素。
  final List<Widget> children;

  /// 原型坐标系尺寸。海面这类只占页面一段的场景按该段在 375 宽框里的高度给。
  final Size designSize;

  /// 铺满整个可用区域的底（如海面渐变），不参与缩放。
  final Widget? background;

  /// 画布高度不够铺满时，画布底边以下的填充（如继续延伸的海水）。
  final Widget? extendBottom;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, box) {
        final scale = box.maxWidth / designSize.width;
        final canvasH = designSize.height * scale;
        return ClipRect(
          child: Stack(
            fit: StackFit.expand,
            children: [
              ?background,
              if (extendBottom != null && box.maxHeight > canvasH)
                Positioned(
                  left: 0,
                  right: 0,
                  top: canvasH,
                  bottom: 0,
                  child: extendBottom!,
                ),
              Positioned(
                left: 0,
                top: 0,
                width: box.maxWidth,
                height: canvasH,
                child: CanvasScope(
                  scale: scale,
                  child: Stack(clipBehavior: Clip.none, children: children),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// 把当前缩放系数传给子树：子元素需要按比例缩放自身尺寸时取。
class CanvasScope extends InheritedWidget {
  const CanvasScope({super.key, required this.scale, required super.child});

  final double scale;

  static CanvasScope of(BuildContext context) {
    final scope = context.dependOnInheritedWidgetOfExactType<CanvasScope>();
    assert(scope != null, 'CanvasPositioned must sit inside a DesignCanvas');
    return scope!;
  }

  @override
  bool updateShouldNotify(CanvasScope old) => old.scale != scale;
}

/// 原型坐标定位。给了 [width] / [height] 就按比例缩放尺寸；
/// 没给则整体 `Transform.scale`，让固定尺寸的装饰子节点跟着画布一起缩放。
class CanvasPositioned extends StatelessWidget {
  const CanvasPositioned({
    super.key,
    this.left,
    this.top,
    this.right,
    this.bottom,
    this.width,
    this.height,
    required this.child,
  });

  final double? left;
  final double? top;
  final double? right;
  final double? bottom;
  final double? width;
  final double? height;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final s = CanvasScope.of(context).scale;
    double? m(double? v) => v == null ? null : v * s;
    final sized = width != null || height != null;
    return Positioned(
      left: m(left),
      top: m(top),
      right: m(right),
      bottom: m(bottom),
      width: m(width),
      height: m(height),
      child: sized
          ? child
          : Transform.scale(scale: s, alignment: _anchor, child: child),
    );
  }

  /// 没给尺寸时围绕定位锚点缩放：靠右定位的元素以右边为锚，靠下的以底边为锚。
  Alignment get _anchor {
    final x = left != null ? -1.0 : (right != null ? 1.0 : 0.0);
    final y = top != null ? -1.0 : (bottom != null ? 1.0 : 0.0);
    return Alignment(x, y);
  }
}
