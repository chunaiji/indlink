import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';
import 'avatar.dart';
import 'coin.dart';

/// 分组小标题（`.sec`）。
class SectionLabel extends StatelessWidget {
  const SectionLabel(this.text, {super.key, this.trailing});

  final String text;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Row(
      children: [
        Expanded(
          child: Text(
            text,
            style: TextStyle(
              fontSize: Dim.t2,
              fontWeight: FontWeight.w700,
              color: c.ink3,
            ),
          ),
        ),
        ?trailing,
      ],
    );
  }
}

/// 通用内容卡（`.mcd`）。
class AppCard extends StatelessWidget {
  const AppCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(Dim.s3),
    this.onTap,
    this.dimmed = false,
  });

  final Widget child;
  final EdgeInsets padding;
  final VoidCallback? onTap;

  /// 过期 / 失效内容整卡降透明度，而不是加一行灰字。
  final bool dimmed;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final card = Container(
      padding: padding,
      decoration: BoxDecoration(
        color: c.surface,
        borderRadius: Dim.brLargeCard,
        border: Border.all(color: c.line),
      ),
      child: child,
    );
    final body = onTap == null
        ? card
        : InkWell(onTap: onTap, borderRadius: Dim.brLargeCard, child: card);
    return dimmed ? Opacity(opacity: .5, child: body) : body;
  }
}

/// 信件卡（`.btlc`）—— 顶部一条品牌渐变，粉紫淡底。
/// 只用于「瓶子正文」这类需要被当成一封信来读的内容。
class BrandCard extends StatelessWidget {
  const BrandCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(Dim.s4),
  });

  final Widget child;
  final EdgeInsets padding;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        borderRadius: Dim.brLargeCard,
        border: Border.all(color: c.line2),
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            c.brand.withValues(alpha: .09),
            c.brand2.withValues(alpha: .11),
          ],
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Container(
            height: 3,
            decoration: BoxDecoration(
              gradient: LinearGradient(colors: c.brandGradient),
            ),
          ),
          Padding(padding: padding, child: child),
        ],
      ),
    );
  }
}

/// 设置项列表容器（`.lst`）。
class ListGroup extends StatelessWidget {
  const ListGroup({super.key, required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: c.surface,
        borderRadius: Dim.brLargeCard,
        border: Border.all(color: c.line),
      ),
      child: Column(
        children: [
          for (var i = 0; i < children.length; i++) ...[
            // 原型 .lst a::after：分割线左缩进 34px → 50pt，和图标右沿对齐。
            if (i > 0)
              Padding(
                padding: const EdgeInsets.only(left: 50),
                child: Divider(height: 1, color: c.line2),
              ),
            children[i],
          ],
        ],
      ),
    );
  }
}

/// 列表行（`.lst a`）。可点区域按 44pt 下限撑开。
class ListRowItem extends StatelessWidget {
  const ListRowItem({
    super.key,
    required this.title,
    this.leading,
    this.leadingEmoji,
    this.subtitle,
    this.trailingText,
    this.trailing,
    this.onTap,
    this.showChevron = true,
    this.titleColor,
    this.roomy = false,
  });

  /// 原型 `.lst.roomy`：35px → 52pt 的宽松行（v2「我的」页、账号与安全）。
  final bool roomy;

  final String title;
  final Widget? leading;
  final String? leadingEmoji;
  final String? subtitle;
  final String? trailingText;
  final Widget? trailing;
  final VoidCallback? onTap;
  final bool showChevron;
  final Color? titleColor;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return InkWell(
      onTap: onTap,
      // 原型 .lst a：min-height 30px → 44pt，padding 0 s4；内容高时自然撑开。
      child: Container(
        constraints: BoxConstraints(minHeight: roomy ? 52 : Dim.tap),
        padding: const EdgeInsets.symmetric(
          horizontal: Dim.s4,
          vertical: Dim.s1,
        ),
        child: Row(
          children: [
            if (leading != null)
              Padding(
                padding: const EdgeInsets.only(right: Dim.s3),
                child: leading,
              )
            else if (leadingEmoji != null)
              Padding(
                padding: const EdgeInsets.only(right: Dim.s3),
                child: SizedBox(
                  width: 24,
                  child: Text(
                    leadingEmoji!,
                    style: const TextStyle(fontSize: 18),
                    textAlign: TextAlign.center,
                  ),
                ),
              ),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    title,
                    style: TextStyle(
                      fontSize: Dim.t3,
                      fontWeight: FontWeight.w600,
                      color: titleColor ?? c.ink,
                    ),
                  ),
                  if (subtitle != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 1),
                      child: Text(
                        subtitle!,
                        style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                      ),
                    ),
                ],
              ),
            ),
            if (trailingText != null)
              Text(
                trailingText!,
                style: TextStyle(fontSize: Dim.t2, color: c.ink3),
              ),
            ?trailing,
            if (showChevron)
              Padding(
                padding: const EdgeInsets.only(left: 2),
                child: Icon(Icons.chevron_right, size: 18, color: c.ink3),
              ),
          ],
        ),
      ),
    );
  }
}

/// 提示条（`.warn`）。金色系，用于说明与风险提示，不是错误。
///
/// 图标走线性图标（原型 §0：框内不用 emoji），13px → 19pt，gold 色。
class NoticeBanner extends StatelessWidget {
  const NoticeBanner({
    super.key,
    required this.text,
    this.icon = Icons.warning_amber_rounded,
    this.richText,
  });

  final String text;
  final IconData icon;
  final Widget? richText;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      padding: const EdgeInsets.all(Dim.s3),
      decoration: BoxDecoration(
        color: c.coin.withValues(alpha: .14),
        borderRadius: Dim.brCard,
        border: Border.all(color: c.gold.withValues(alpha: .3)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 1),
            child: Icon(icon, size: 19, color: c.gold),
          ),
          const SizedBox(width: Dim.s2),
          Expanded(
            child:
                richText ??
                Text(
                  text,
                  style: TextStyle(
                    fontSize: Dim.t1,
                    height: 1.6,
                    color: c.ink2,
                  ),
                ),
          ),
        ],
      ),
    );
  }
}

/// 细进度条（`.bar4` / `.prog`）。
class ThinProgressBar extends StatelessWidget {
  const ThinProgressBar({
    super.key,
    required this.value,
    this.width,
    this.height = 4,
    this.color,
    this.gradient = false,
  });

  final double value;
  final double? width;
  final double height;
  final Color? color;

  /// 品牌渐变只用在「引导进度」这种品牌面上。
  final bool gradient;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return SizedBox(
      width: width,
      height: height,
      child: ClipRRect(
        borderRadius: Dim.brPill,
        child: Stack(
          children: [
            Positioned.fill(child: ColoredBox(color: c.line)),
            FractionallySizedBox(
              widthFactor: value.clamp(0, 1),
              child: Container(
                decoration: BoxDecoration(
                  color: gradient ? null : (color ?? c.brand),
                  gradient: gradient
                      ? LinearGradient(colors: c.brandGradient)
                      : null,
                  borderRadius: Dim.brPill,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 三格统计条（`.stats`）。
class StatsRow extends StatelessWidget {
  const StatsRow({super.key, required this.items});

  /// (数值, 说明)
  final List<(String, String)> items;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      decoration: BoxDecoration(
        color: c.surface,
        borderRadius: Dim.brLargeCard,
        border: Border.all(color: c.line),
      ),
      child: Row(
        children: [
          for (var i = 0; i < items.length; i++)
            Expanded(
              child: Container(
                padding: const EdgeInsets.symmetric(vertical: Dim.s3),
                decoration: BoxDecoration(
                  border: i == items.length - 1
                      ? null
                      : Border(right: BorderSide(color: c.line2)),
                ),
                child: Column(
                  children: [
                    Text(
                      items[i].$1,
                      style: TextStyle(
                        fontSize: Dim.t5,
                        fontWeight: FontWeight.w800,
                        color: c.ink,
                      ),
                    ),
                    const SizedBox(height: 1),
                    Text(
                      items[i].$2,
                      style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}

/// 漂流轨迹时间线（`.tl`）。
///
/// B4 与 B4s 用的是同一个组件，只换配色——同一份数据、同一个组件，两种密度。
class TimelineList extends StatelessWidget {
  const TimelineList({super.key, required this.items, this.onDark = false});

  /// (正文, 时间)
  final List<(String, String)> items;
  final bool onDark;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final line = onDark ? Colors.white.withValues(alpha: .28) : c.line;
    final dotColor = onDark ? Colors.white : c.brand;
    final textColor = onDark ? Colors.white : c.ink;
    final timeColor = onDark ? Colors.white.withValues(alpha: .6) : c.ink3;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 原型 .tl div：行最小高 26px → 39pt，点 6px → 9，竖线 1.5。
        for (var i = 0; i < items.length; i++)
          IntrinsicHeight(
            key: ValueKey('timeline-row-$i'),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SizedBox(
                  width: 16,
                  child: Column(
                    children: [
                      Container(
                        width: 1.5,
                        height: 15,
                        color: i == 0 ? Colors.transparent : line,
                      ),
                      Container(
                        width: 9,
                        height: 9,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: i == items.length - 1 ? dotColor : line,
                        ),
                      ),
                      Expanded(
                        child: Container(
                          width: 1.5,
                          color: i == items.length - 1
                              ? Colors.transparent
                              : line,
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: Dim.s2),
                Expanded(
                  child: Container(
                    constraints: const BoxConstraints(minHeight: 39),
                    padding: const EdgeInsets.only(bottom: Dim.s2),
                    alignment: Alignment.centerLeft,
                    child: Row(
                      children: [
                        Expanded(
                          child: Text(
                            items[i].$1,
                            style: TextStyle(
                              fontSize: Dim.t2,
                              color: textColor,
                              height: 1.4,
                            ),
                          ),
                        ),
                        const SizedBox(width: Dim.s2),
                        Text(
                          items[i].$2,
                          style: TextStyle(fontSize: Dim.t1, color: timeColor),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }
}

/// 四宫格入口的一项。
class QuadItem {
  const QuadItem({
    required this.icon,
    required this.label,
    required this.onTap,
    this.badge,
  });

  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final int? badge;
}

/// 四宫格入口（原型 `.quad`）。[roomy] 是 v2「我的」页的高级版：62pt 格、surface 底、投影。
class QuadGrid extends StatelessWidget {
  const QuadGrid({super.key, required this.items, this.roomy = false});

  final List<QuadItem> items;
  final bool roomy;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (var i = 0; i < items.length; i++) ...[
          if (i > 0) SizedBox(width: roomy ? Dim.s3 : Dim.s2),
          Expanded(
            child: QuadTile(item: items[i], roomy: roomy, index: i),
          ),
        ],
      ],
    );
  }
}

class QuadTile extends StatelessWidget {
  const QuadTile({
    super.key,
    required this.item,
    required this.roomy,
    required this.index,
  });

  final QuadItem item;
  final bool roomy;
  final int index;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    // 原型 .quad div b：32px → 48，r3，sea 底；.roomy：42px → 62，r4，surface + 描边 + 投影。
    final size = roomy ? 62.0 : 48.0;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: item.onTap,
      child: Column(
        children: [
          Stack(
            clipBehavior: Clip.none,
            children: [
              Container(
                key: ValueKey('quad-icon-$index'),
                width: size,
                height: size,
                decoration: BoxDecoration(
                  color: roomy ? c.surface : c.sea,
                  borderRadius: roomy ? Dim.brLargeCard : Dim.brCard,
                  border: roomy ? Border.all(color: c.line2) : null,
                  boxShadow: roomy ? Shadows.card(c) : null,
                ),
                child: Icon(item.icon, size: roomy ? 30 : 22, color: c.ink2),
              ),
              if ((item.badge ?? 0) > 0)
                Positioned(
                  right: -6,
                  top: -6,
                  child: UnreadBadge(count: item.badge!),
                ),
            ],
          ),
          const SizedBox(height: Dim.s1),
          Text(
            item.label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontSize: roomy ? Dim.t2 : Dim.t1,
              fontWeight: FontWeight.w600,
              color: c.ink2,
              height: 1.4,
            ),
          ),
        ],
      ),
    );
  }
}

/// 会话 / 关系 / 通知共用的行（原型 `.rws a`）：最小高 44px → 65、padding s2 0、
/// 头像 44、标题 t3/700、副文 t2 ink3 单行省略；右侧默认是时间 t1 + 未读 pill，
/// 也可以整体换成 [trailing]。分割线由列表的 separator 画（左缩进 57）。
class ConversationRow extends StatelessWidget {
  const ConversationRow({
    super.key,
    required this.leading,
    required this.title,
    required this.subtitle,
    this.time,
    this.unread = 0,
    this.subtitleColor,
    this.trailing,
    this.onTap,
    this.dimmed = false,
  });

  final Widget leading;
  final String title;
  final String subtitle;
  final String? time;
  final int unread;
  final Color? subtitleColor;
  final Widget? trailing;
  final VoidCallback? onTap;

  /// 对方已注销等失效态：整行压淡。
  final bool dimmed;

  /// 分割线左缩进：原型 38px → 57，和头像右沿对齐。
  static const double dividerIndent = 57;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Opacity(
      opacity: dimmed ? .55 : 1,
      child: InkWell(
        onTap: onTap,
        child: Container(
          constraints: const BoxConstraints(minHeight: 65),
          padding: const EdgeInsets.symmetric(vertical: Dim.s2),
          child: Row(
            children: [
              leading,
              const SizedBox(width: Dim.s3),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text(
                      title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: Dim.t3,
                        fontWeight: FontWeight.w700,
                        height: 1.4,
                        color: c.ink,
                      ),
                    ),
                    const SizedBox(height: 1),
                    Text(
                      subtitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: Dim.t2,
                        height: 1.4,
                        color: subtitleColor ?? c.ink3,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: Dim.s2),
              trailing ??
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      if (time != null)
                        Text(
                          time!,
                          style: TextStyle(
                            fontSize: Dim.t1,
                            height: 1.4,
                            color: c.ink3,
                          ),
                        ),
                      if (unread > 0) ...[
                        const SizedBox(height: Dim.s1),
                        UnreadBadge(count: unread),
                      ],
                    ],
                  ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 充值档位格（原型 `.pkg`）：padding s3 0 居中、1.5 描边 r3；选中 brand 描边 + 粉 .06 底；
/// 币数 t5/800 带币图标，价格 t2 ink3（原价划掉 .7），角标 `em` 浮在顶边中央。
///
/// 价格行套 FittedBox：360 宽 × 1.2 字体下「¥59.9 ¥79」不能换行（Review Focus 5）。
class PackageTile extends StatelessWidget {
  const PackageTile({
    super.key,
    required this.coins,
    required this.price,
    required this.selected,
    required this.onTap,
    this.original,
    this.badge,
  });

  final int coins;
  final String price;
  final String? original;
  final String? badge;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          // 铺满格宽：Stack 给的是松约束，不写宽度卡片会按内容缩成一小块，角标也跟着跑偏。
          Container(
            width: double.infinity,
            padding: const EdgeInsets.symmetric(
              vertical: Dim.s3,
              horizontal: Dim.s2,
            ),
            decoration: BoxDecoration(
              color: selected ? c.brand.withValues(alpha: .06) : c.surface,
              borderRadius: Dim.brCard,
              border: Border.all(
                color: selected ? c.brand : c.line,
                width: 1.5,
              ),
            ),
            // 两行内容整体按格高缩放：360 宽 + 1.2 字体下格子只有 80 高，
            // 原样排会差 1px 溢出；缩 2% 肉眼看不出，溢出条却很显眼。
            child: FittedBox(
              fit: BoxFit.scaleDown,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  FittedBox(
                    fit: BoxFit.scaleDown,
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const CoinIcon(size: 17),
                        const SizedBox(width: 3),
                        Text(
                          '$coins',
                          style: TextStyle(
                            fontSize: Dim.t5,
                            fontWeight: FontWeight.w800,
                            color: c.ink,
                            height: 1.25,
                            fontFeatures: const [FontFeature.tabularFigures()],
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 2),
                  FittedBox(
                    fit: BoxFit.scaleDown,
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          price,
                          style: TextStyle(
                            fontSize: Dim.t2,
                            color: c.ink3,
                            height: 1.4,
                          ),
                        ),
                        if (original != null) ...[
                          const SizedBox(width: 3),
                          Text(
                            original!,
                            style: TextStyle(
                              fontSize: Dim.t2,
                              color: c.ink3.withValues(alpha: .7),
                              height: 1.4,
                              decoration: TextDecoration.lineThrough,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
          if (badge != null)
            Positioned(
              top: -10,
              left: 0,
              right: 0,
              child: Center(
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: Dim.s2,
                    vertical: 1,
                  ),
                  decoration: BoxDecoration(
                    color: c.brand,
                    borderRadius: Dim.brPill,
                  ),
                  child: Text(
                    badge!,
                    maxLines: 1,
                    style: const TextStyle(
                      fontSize: Dim.t0,
                      fontWeight: FontWeight.w800,
                      color: Colors.white,
                      height: 1.4,
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

/// 签到进度格（原型 `.ck`）：gap 3px → 4.5、最小高 32px → 48、r2、sea 底 t0/700 ink3；
/// 已签 ok 绿 .2 + ✓；今天 aqua 白字 + 投影；未来 line 描边。
class CheckInStrip extends StatelessWidget {
  const CheckInStrip({
    super.key,
    required this.cycle,
    required this.streak,
    required this.checkedToday,
    required this.todayReward,
  });

  final int cycle;
  final int streak;
  final bool checkedToday;
  final int todayReward;

  static const _ok = Color(0xFF2FBF77);

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final todayIndex = checkedToday ? streak - 1 : streak;
    return Row(
      children: [
        for (var i = 0; i < cycle; i++) ...[
          if (i > 0) const SizedBox(width: 4.5),
          Expanded(
            child: Builder(
              builder: (context) {
                final done = i < streak;
                final now = i == todayIndex && !checkedToday;
                return Container(
                  constraints: const BoxConstraints(minHeight: 48),
                  alignment: Alignment.center,
                  decoration: BoxDecoration(
                    color: done
                        ? _ok.withValues(alpha: .2)
                        : now
                        ? c.aqua
                        : Colors.transparent,
                    borderRadius: Dim.brButton,
                    border: done || now ? null : Border.all(color: c.line),
                    boxShadow: now
                        ? Shadows.glowSmall(c.aqua, alpha: .38)
                        : null,
                  ),
                  child: done
                      ? const Icon(Icons.check_rounded, size: 18, color: _ok)
                      : Text(
                          now ? '+$todayReward' : '${i + 1}',
                          style: TextStyle(
                            fontSize: Dim.t0,
                            fontWeight: FontWeight.w700,
                            color: now ? Colors.white : c.ink3,
                            height: 1.4,
                          ),
                        ),
                );
              },
            ),
          ),
        ],
      ],
    );
  }
}
