import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';

/// 金币图标。全 App 只有这一处画币，尺寸随字号走。
class CoinIcon extends StatelessWidget {
  const CoinIcon({super.key, this.size = 14, this.onDark = false});

  final double size;

  /// 压在渐变头图/深色按钮上时，外圈描边换成半透明白。
  final bool onDark;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [c.coin, Color.lerp(c.coin, const Color(0xFFE08A1E), .55)!],
        ),
        border: Border.all(
          color: onDark
              ? Colors.white.withValues(alpha: .55)
              : const Color(0xFF8A5A00).withValues(alpha: .25),
          width: size >= 18 ? 1.2 : .8,
        ),
      ),
      alignment: Alignment.center,
      child: Container(
        width: size * .42,
        height: size * .42,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: const Color(0xFF8A5A00).withValues(alpha: .28),
        ),
      ),
    );
  }
}

/// 顶部栏的余额胶囊。点它进充值。
///
/// 渐变头图上是白底金字（`.coin`），浅色页面里换 sea 底 ink 字（`.coin.ink`）——
/// 两种底都保证对比度，不用半透明白叠在渐变上糊成一片。
class CoinChip extends StatelessWidget {
  const CoinChip({
    super.key,
    required this.coins,
    this.onTap,
    this.onHeader = true,
  });

  final int coins;
  final VoidCallback? onTap;
  final bool onHeader;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Semantics(
      button: onTap != null,
      label: '$coins',
      child: InkWell(
        onTap: onTap,
        borderRadius: Dim.brPill,
        // 原型 .coin：padding 2.5px/8/2.5px/3 → 高 30，左 4 右 8。
        child: Container(
          height: 30,
          padding: const EdgeInsets.fromLTRB(4, 0, Dim.s2, 0),
          decoration: BoxDecoration(
            color: onHeader ? Colors.white.withValues(alpha: .94) : c.sea,
            borderRadius: Dim.brPill,
            boxShadow: onHeader
                ? [
                    BoxShadow(
                      color: Colors.black.withValues(alpha: .16),
                      blurRadius: 4,
                      offset: const Offset(0, 1),
                    ),
                  ]
                : null,
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              const CoinIcon(size: 17),
              const SizedBox(width: 4),
              Text(
                '$coins',
                style: TextStyle(
                  fontSize: Dim.t3,
                  fontWeight: FontWeight.w800,
                  color: onHeader ? const Color(0xFF4A3200) : c.ink,
                  fontFeatures: const [FontFeature.tabularFigures()],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
