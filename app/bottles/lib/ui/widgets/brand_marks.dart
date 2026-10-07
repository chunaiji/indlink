import 'dart:math' as math;

import 'package:flutter/material.dart';

/// Google 的四色「G」。自绘而不是放图：登录按钮上只要 20pt，矢量在任何 DPR 下都清楚，
/// 也不用为一个标进一张图资源。几何按官方标志近似：四段弧 + 右侧横杠。
class GoogleMark extends StatelessWidget {
  const GoogleMark({super.key, this.size = 20});

  final double size;

  @override
  Widget build(BuildContext context) =>
      CustomPaint(size: Size.square(size), painter: const _GooglePainter());
}

class _GooglePainter extends CustomPainter {
  const _GooglePainter();

  static const _blue = Color(0xFF4285F4);
  static const _green = Color(0xFF34A853);
  static const _yellow = Color(0xFFFBBC05);
  static const _red = Color(0xFFEA4335);

  @override
  void paint(Canvas canvas, Size size) {
    final stroke = size.width * .2;
    final rect = Rect.fromLTWH(
      stroke / 2,
      stroke / 2,
      size.width - stroke,
      size.height - stroke,
    );
    final paint = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = stroke
      ..strokeCap = StrokeCap.butt;
    double rad(double deg) => deg * math.pi / 180;
    // 屏幕坐标顺时针：0° 在右。右下蓝、下绿、左黄、上红；右上留口给横杠进来。
    canvas.drawArc(rect, rad(0), rad(45), false, paint..color = _blue);
    canvas.drawArc(rect, rad(45), rad(90), false, paint..color = _green);
    canvas.drawArc(rect, rad(135), rad(90), false, paint..color = _yellow);
    canvas.drawArc(rect, rad(225), rad(90), false, paint..color = _red);
    // 横杠：从圆心到右沿，和蓝弧接上。
    final c = size.center(Offset.zero);
    canvas.drawRect(
      Rect.fromLTRB(c.dx, c.dy - stroke / 2, size.width, c.dy + stroke / 2),
      Paint()..color = _blue,
    );
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
