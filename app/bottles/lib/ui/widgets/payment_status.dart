import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';
import 'chips.dart';
import 'coin.dart';

/// 支付状态三态（原型 `.pst`）：成功 / 失败 / 等待共用一套居中版式，只换图标环的颜色。
///
/// 占据页面的弹性区（调用方放进 Expanded），按钮由页面放进 BottomActionBar。
/// 图标环 52px → 77；标题 t5/800；正文 t2 ink2 lh1.65；金额 `.amt2` 26px → 39 gold；
/// 订单号用 [OrderIdChip]。等待态的转圈响应「减弱动态效果」，退成静态圆环。
enum PaymentStatusKind { ok, error, pending }

class PaymentStatusView extends StatelessWidget {
  const PaymentStatusView({
    super.key,
    required this.kind,
    required this.title,
    required this.body,
    this.amount,
    this.meta,
    this.orderNo,
    this.icon,
  });

  final PaymentStatusKind kind;
  final String title;
  final String body;

  /// 「+300」这类到账金额，带币图标，gold 色。
  final String? amount;

  /// 「余额 248 → 548」「已等待 12 秒」这类一行小字。
  final String? meta;

  /// 需要报给客服的订单号。
  final String? orderNo;

  /// 覆盖默认图标（成功 ✓ / 失败 ✕）。
  final IconData? icon;

  static const _ok = Color(0xFF2FBF77);

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final reduceMotion = MediaQuery.disableAnimationsOf(context);

    final ring = switch (kind) {
      PaymentStatusKind.ok => _Ring(
        bg: _ok.withValues(alpha: .14),
        border: _ok.withValues(alpha: .45),
        child: Icon(icon ?? Icons.check_rounded, size: 34, color: _ok),
      ),
      PaymentStatusKind.error => _Ring(
        bg: c.warn.withValues(alpha: .12),
        border: c.warn.withValues(alpha: .45),
        child: Icon(icon ?? Icons.close_rounded, size: 34, color: c.warn),
      ),
      PaymentStatusKind.pending => _Ring(
        bg: Colors.transparent,
        border: Colors.transparent,
        // 原型 .spin：30px → 45 圆环，2.5px → 3，line 底 aqua 顶。
        child: SizedBox(
          width: 45,
          height: 45,
          child: reduceMotion
              ? DecoratedBox(
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    border: Border.all(color: c.aqua, width: 3),
                  ),
                )
              : CircularProgressIndicator(
                  strokeWidth: 3,
                  color: c.aqua,
                  backgroundColor: c.line,
                ),
        ),
      ),
    };

    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: Dim.s5),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ring,
            const SizedBox(height: Dim.s4),
            if (amount != null) ...[
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const CoinIcon(size: 30),
                  const SizedBox(width: Dim.s2),
                  Text(
                    amount!,
                    style: TextStyle(
                      fontSize: 39,
                      fontWeight: FontWeight.w800,
                      color: c.gold,
                      height: 1.1,
                      fontFeatures: const [FontFeature.tabularFigures()],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: Dim.s2),
            ],
            Text(
              title,
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: Dim.t5,
                fontWeight: FontWeight.w800,
                height: 1.25,
                letterSpacing: -0.2,
                color: c.ink,
              ),
            ),
            const SizedBox(height: Dim.s2),
            Text(
              body,
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: Dim.t2, height: 1.65, color: c.ink2),
            ),
            if (meta != null) ...[
              const SizedBox(height: Dim.s2),
              Text(
                meta!,
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: Dim.t1, height: 1.5, color: c.ink3),
              ),
            ],
            if (orderNo != null) ...[
              const SizedBox(height: Dim.s3),
              OrderIdChip(text: orderNo!),
            ],
          ],
        ),
      ),
    );
  }
}

class _Ring extends StatelessWidget {
  const _Ring({required this.bg, required this.border, required this.child});

  final Color bg;
  final Color border;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const ValueKey('pst-ring'),
      width: 77,
      height: 77,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: bg,
        border: Border.all(color: border, width: 2),
      ),
      child: child,
    );
  }
}
