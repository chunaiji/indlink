import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';

/// 九宫格（原型 `.g9`）：3 列、gap s2、r2、正方形。
///
/// 只管排布：每格内容由 [itemBuilder] 给（本地图、网络图、上传遮罩都由调用方画），
/// 末尾按需追加一个虚线「+」空位。编辑态可以显示序号角标（`.num`，14px → 21）。
class PhotoGrid extends StatelessWidget {
  const PhotoGrid({
    super.key,
    required this.count,
    required this.itemBuilder,
    this.onAdd,
    this.max = 9,
    this.numbered = false,
  });

  final int count;
  final IndexedWidgetBuilder itemBuilder;
  final VoidCallback? onAdd;
  final int max;
  final bool numbered;

  static const int _columns = 3;

  @override
  Widget build(BuildContext context) {
    final showAdd = onAdd != null && count < max;
    final total = count + (showAdd ? 1 : 0);
    if (total == 0) return const SizedBox.shrink();
    // 不用 GridView(shrinkWrap)：它不支持固有尺寸，放进 SliverFillRemaining /
    // IntrinsicHeight 里会直接抛错。3 列 Row + AspectRatio 手排，固有高度可算。
    final rows = (total / _columns).ceil();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (var r = 0; r < rows; r++) ...[
          if (r > 0) const SizedBox(height: Dim.s2),
          Row(
            children: [
              for (var col = 0; col < _columns; col++) ...[
                if (col > 0) const SizedBox(width: Dim.s2),
                Expanded(
                  child: AspectRatio(
                    aspectRatio: 1,
                    child: _cell(context, r * _columns + col, total),
                  ),
                ),
              ],
            ],
          ),
        ],
      ],
    );
  }

  Widget _cell(BuildContext context, int i, int total) {
    final c = context.c;
    if (i >= total) return const SizedBox.shrink();
    if (i == count) {
      return GestureDetector(
        onTap: onAdd,
        child: CustomPaint(
          painter: _DashedRRectPainter(color: c.line, radius: Dim.r2),
          child: Center(
            child: Icon(Icons.add_rounded, size: 30, color: c.ink3),
          ),
        ),
      );
    }
    return ClipRRect(
      borderRadius: Dim.brButton,
      child: Stack(
        fit: StackFit.expand,
        children: [
          itemBuilder(context, i),
          if (numbered)
            Positioned(
              right: 4,
              top: 4,
              child: Container(
                width: 21,
                height: 21,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: c.brand,
                  shape: BoxShape.circle,
                ),
                child: Text(
                  '${i + 1}',
                  style: const TextStyle(
                    fontSize: Dim.t0,
                    fontWeight: FontWeight.w700,
                    color: Colors.white,
                    height: 1,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

/// 格子上的半透明遮罩（原型 `.g9 .ovl`）：上传中 / 失败时盖在图上，白字 t1/700。
class PhotoTileOverlay extends StatelessWidget {
  const PhotoTileOverlay({super.key, required this.label, this.onTap});

  final String label;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: ColoredBox(
        color: const Color(0xFF081420).withValues(alpha: .52),
        child: Center(
          child: Text(
            label,
            textAlign: TextAlign.center,
            style: const TextStyle(
              fontSize: Dim.t1,
              fontWeight: FontWeight.w700,
              color: Colors.white,
              height: 1.4,
            ),
          ),
        ),
      ),
    );
  }
}

/// 虚线圆角框（Flutter 没有 dashed border）。
class _DashedRRectPainter extends CustomPainter {
  const _DashedRRectPainter({required this.color, required this.radius});

  final Color color;
  final double radius;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1;
    final rrect = RRect.fromRectAndRadius(
      (Offset.zero & size).deflate(.5),
      Radius.circular(radius),
    );
    final path = Path()..addRRect(rrect);
    const dash = 5.0;
    const gap = 4.0;
    for (final metric in path.computeMetrics()) {
      var d = 0.0;
      while (d < metric.length) {
        canvas.drawPath(metric.extractPath(d, d + dash), paint);
        d += dash + gap;
      }
    }
  }

  @override
  bool shouldRepaint(covariant _DashedRRectPainter old) =>
      old.color != color || old.radius != radius;
}
