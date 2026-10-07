import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';
import 'coin.dart';

/// 按钮语义四级 —— 这是产品规则，不是审美选择（原型 §0）。
///
/// * [primary] 海蓝实心：主线**免费**动作（捞瓶 / 抛瓶 / 登录 / 签到领币）
/// * [pay] 粉实心：**要花金币或真钱**（解锁 / 打招呼 / 送礼 / 充值）
/// * [ghost] 灰描边：次要动作
/// * [payGhost] 粉描边：花币的次要动作，保证「粉 = 要付费」在任何层级都成立
/// * [danger] 警示实心：破坏性（举报并拉黑 / 注销账号）
///
/// 渐变只用于品牌面（头图 / Tab 选中块 / 进度条），**按钮一律实色**。
///
/// * [oauth] 白底描边、15pt 正文字重：第三方登录（原型 `.oauth`）。
///   它比 [ghost] 轻一档——登录页上它排在主按钮下面，字号和主按钮一样大会抢戏。
enum BtnKind { primary, pay, ghost, payGhost, danger, glass, oauth }

class AppButton extends StatelessWidget {
  const AppButton({
    super.key,
    required this.label,
    this.kind = BtnKind.primary,
    this.onTap,
    this.coins,
    this.icon,
    this.leading,
    this.subLabel,
    this.expand = true,
    this.loading = false,
    this.height = 50,
  });

  final String label;
  final BtnKind kind;
  final VoidCallback? onTap;

  /// 非空时在文案后面跟一枚币 + 数字，价格永远和动作在同一个按钮里。
  final int? coins;

  final IconData? icon;

  /// 自绘的前导图标（品牌标）；给了它就不用 [icon]。
  final Widget? leading;

  /// 按钮内的第二行小字（「今日还剩 3 次」这类）。
  final String? subLabel;

  final bool expand;
  final bool loading;
  final double height;

  bool get _enabled => onTap != null && !loading;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final (bg, fg, border) = switch (kind) {
      BtnKind.primary => (c.aqua, Colors.white, null),
      BtnKind.pay => (c.brand, Colors.white, null),
      BtnKind.danger => (c.warn, Colors.white, null),
      BtnKind.ghost => (Colors.transparent, c.ink2, c.line),
      BtnKind.payGhost => (
        Colors.transparent,
        c.brand,
        c.brand.withValues(alpha: .45),
      ),
      BtnKind.glass => (
        Colors.white.withValues(alpha: .22),
        Colors.white,
        Colors.white.withValues(alpha: .5),
      ),
      BtnKind.oauth => (c.surface, c.ink, c.line),
    };

    // 原型 `.cta` 字号 t4/800，`.oauth` 是 t3/700：第三方登录按钮比主按钮轻一档。
    final (labelSize, labelWeight) = switch (kind) {
      BtnKind.oauth => (Dim.t3, FontWeight.w700),
      _ => (Dim.t4, FontWeight.w800),
    };

    // 实心按钮带同色投影，描边按钮不带——否则次要动作看着比主动作还重。
    // 原型 .cta：海蓝 .38 / 粉 .40 / 警示 .34，几何走 Shadows.glow。
    final (glowColor, glowAlpha) = switch (kind) {
      BtnKind.primary => (c.aqua, .38),
      BtnKind.pay => (c.brand, .40),
      BtnKind.danger => (c.warn, .34),
      _ => (null, 0.0),
    };

    final content = loading
        ? SizedBox(
            width: 18,
            height: 18,
            child: CircularProgressIndicator(strokeWidth: 2, color: fg),
          )
        : Column(
            mainAxisSize: MainAxisSize.min,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Row(
                mainAxisSize: MainAxisSize.min,
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  if (leading != null) ...[
                    leading!,
                    const SizedBox(width: 8),
                  ] else if (icon != null) ...[
                    Icon(icon, size: 17, color: fg),
                    const SizedBox(width: 6),
                  ],
                  Flexible(
                    child: Text(
                      label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: labelSize,
                        fontWeight: labelWeight,
                        color: fg,
                        height: 1.2,
                        letterSpacing: -0.1,
                      ),
                    ),
                  ),
                  if (coins != null) ...[
                    const SizedBox(width: 6),
                    CoinIcon(size: 16, onDark: glowColor != null),
                    const SizedBox(width: 3),
                    Text(
                      '$coins',
                      style: TextStyle(
                        fontSize: Dim.t4,
                        fontWeight: FontWeight.w800,
                        color: fg,
                        height: 1.2,
                      ),
                    ),
                  ],
                ],
              ),
              if (subLabel != null)
                Padding(
                  padding: const EdgeInsets.only(top: 1),
                  child: Text(
                    subLabel!,
                    style: TextStyle(
                      fontSize: Dim.t0,
                      fontWeight: FontWeight.w500,
                      color: fg.withValues(alpha: .82),
                      height: 1.2,
                    ),
                  ),
                ),
            ],
          );

    return Opacity(
      opacity: _enabled ? 1 : .45,
      child: DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: Dim.brPill,
          boxShadow: glowColor == null || !_enabled
              ? null
              : Shadows.glow(glowColor, alpha: glowAlpha),
        ),
        child: Material(
          color: bg,
          borderRadius: Dim.brPill,
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            onTap: _enabled ? onTap : null,
            child: Container(
              height: height,
              width: expand ? double.infinity : null,
              padding: EdgeInsets.symmetric(
                horizontal: expand ? Dim.s4 : Dim.s5,
              ),
              decoration: border == null
                  ? null
                  : BoxDecoration(
                      borderRadius: Dim.brPill,
                      border: Border.all(color: border),
                    ),
              alignment: Alignment.center,
              child: content,
            ),
          ),
        ),
      ),
    );
  }
}

/// 卡片右上 / 列表行尾的小胶囊按钮（「开启」「＋ 关注」「充值」）。
class MiniButton extends StatelessWidget {
  const MiniButton({
    super.key,
    required this.label,
    this.onTap,
    this.kind = BtnKind.primary,
    this.coins,
    this.height = Dim.tap,
  });

  final String label;
  final VoidCallback? onTap;
  final BtnKind kind;
  final int? coins;

  /// 原型 .jn 是 44；钱包卡上的「充值」（.wallet .wpay）是 32px → 48。
  final double height;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final (bg, fg, border) = switch (kind) {
      BtnKind.pay => (c.brand, Colors.white, null),
      BtnKind.ghost => (c.surface, c.ink2, c.line),
      BtnKind.payGhost => (c.surface, c.brand, c.brand.withValues(alpha: .45)),
      _ => (c.aqua, Colors.white, null),
    };
    // 原型 .jn：0 3px 10px，粉 .38 / 海蓝 .34；描边版无投影。
    final glow = switch (kind) {
      BtnKind.pay => Shadows.glowSmall(c.brand, alpha: .38),
      BtnKind.ghost || BtnKind.payGhost => null,
      _ => Shadows.glowSmall(c.aqua),
    };

    return DecoratedBox(
      decoration: BoxDecoration(borderRadius: Dim.brPill, boxShadow: glow),
      child: Material(
        color: bg,
        borderRadius: Dim.brPill,
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          child: Container(
            height: height,
            // 最窄 1.5 倍高：两个字的「充值」按内容只比高度宽一点，48 高就成了个圆
            // （原型 .wpay 是 65×48 的胶囊）。
            constraints: BoxConstraints(minWidth: height * 1.5),
            padding: const EdgeInsets.symmetric(horizontal: Dim.s3),
            decoration: border == null
                ? null
                : BoxDecoration(
                    borderRadius: Dim.brPill,
                    border: Border.all(color: border),
                  ),
            alignment: Alignment.center,
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                // 原型 .jn 是 t4/800：它只有 44 高，字小了会像个标签而不是按钮。
                Text(
                  label,
                  style: TextStyle(
                    fontSize: Dim.t4,
                    fontWeight: FontWeight.w800,
                    color: fg,
                    height: 1.2,
                  ),
                ),
                if (coins != null) ...[
                  const SizedBox(width: 4),
                  CoinIcon(size: 15, onDark: glow != null),
                  const SizedBox(width: 2),
                  Text(
                    '$coins',
                    style: TextStyle(
                      fontSize: Dim.t2,
                      fontWeight: FontWeight.w800,
                      color: fg,
                      height: 1.2,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// 圆形图标按钮，可点区域按 44pt 下限撑开。
class RoundIconButton extends StatelessWidget {
  const RoundIconButton({
    super.key,
    required this.icon,
    this.onTap,
    this.color,
    this.background,
    this.size = 20,
    this.tooltip,
  });

  final IconData icon;
  final VoidCallback? onTap;
  final Color? color;
  final Color? background;
  final double size;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final btn = InkResponse(
      onTap: onTap,
      radius: 24,
      child: Container(
        width: Dim.tap,
        height: Dim.tap,
        alignment: Alignment.center,
        child: Container(
          width: background == null ? null : 30,
          height: background == null ? null : 30,
          decoration: background == null
              ? null
              : BoxDecoration(color: background, shape: BoxShape.circle),
          alignment: Alignment.center,
          child: Icon(icon, size: size, color: color ?? context.c.ink),
        ),
      ),
    );
    return tooltip == null ? btn : Tooltip(message: tooltip!, child: btn);
  }
}

/// 滑卡底部的动作圆钮（原型 `.ab i`）：50pt 圆、page 底、line 描边、图标 24。
///
/// [liked] 是「喜欢」的点亮态（`.ab.lk`）：粉底白图标加一圈同色小投影。
/// 它和 [RoundIconButton] 不是一回事：后者是输入栏里 44 的透明点击区。
class ActionCircleButton extends StatelessWidget {
  const ActionCircleButton({
    super.key,
    required this.icon,
    this.onTap,
    this.liked = false,
    this.tooltip,
  });

  final IconData icon;
  final VoidCallback? onTap;
  final bool liked;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final btn = InkResponse(
      onTap: onTap,
      radius: 28,
      child: Container(
        width: 50,
        height: 50,
        decoration: BoxDecoration(
          color: liked ? c.brand : c.page,
          shape: BoxShape.circle,
          border: liked ? null : Border.all(color: c.line),
          boxShadow: liked ? Shadows.glowSmall(c.brand, alpha: .4) : null,
        ),
        alignment: Alignment.center,
        child: Icon(icon, size: 24, color: liked ? Colors.white : c.ink2),
      ),
    );
    return tooltip == null ? btn : Tooltip(message: tooltip!, child: btn);
  }
}

/// 滑卡底部「打招呼」（原型 `.hi`）：占满剩余宽、50 高、粉底、t4/800，价格跟在文案后。
///
/// 粉色在全 App 只有一个含义：要花金币——这一枚永远是 [BtnKind.pay]。
class HiButton extends StatelessWidget {
  const HiButton({
    super.key,
    required this.label,
    required this.coins,
    this.onTap,
    this.loading = false,
  });

  final String label;
  final int coins;
  final VoidCallback? onTap;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    return AppButton(
      label: label,
      coins: coins,
      kind: BtnKind.pay,
      onTap: onTap,
      loading: loading,
    );
  }
}

/// 压在照片 / 地图上的圆形图标按钮（原型 `.iconb`）：24px → 36 白圆（.94）+ 卡片投影，
/// 图标 18 ink2。资料页头图的 ← / ⋯ 和地图选点的「回到我」都是它——
/// 压在真实照片上时半透明玻璃圆和裸图标都看不清，实心白圆是唯一稳的选择。
class IconCircleButton extends StatelessWidget {
  const IconCircleButton({
    super.key,
    required this.icon,
    this.onTap,
    this.tooltip,
  });

  final IconData icon;
  final VoidCallback? onTap;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final button = InkResponse(
      onTap: onTap,
      radius: 24,
      child: Container(
        width: 36,
        height: 36,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: c.surface.withValues(alpha: .94),
          boxShadow: Shadows.card(c),
        ),
        child: Icon(icon, size: 18, color: c.ink2),
      ),
    );
    if (tooltip == null) return button;
    return Tooltip(message: tooltip!, child: button);
  }
}
