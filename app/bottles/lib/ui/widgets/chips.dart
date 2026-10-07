import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';

/// 筛选胶囊的语义变体。
///
/// 选中态用 **brand2 紫**而不是粉——粉色在这套设计里被「要花金币」独占，
/// 筛选选中不是付费动作，不能借用它。
enum ChipTone {
  /// 普通项
  plain,

  /// 花币项 / 强调项：粉字粉边
  pay,

  /// 语言项：海蓝字蓝边
  aqua,
}

class PillChip extends StatelessWidget {
  const PillChip({
    super.key,
    required this.label,
    this.selected = false,
    this.tone = ChipTone.plain,
    this.onTap,
  });

  final String label;
  final bool selected;
  final ChipTone tone;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final (fg, border) = switch (tone) {
      ChipTone.pay => (c.brand, c.brand.withValues(alpha: .45)),
      ChipTone.aqua => (c.aqua, c.aqua.withValues(alpha: .45)),
      ChipTone.plain => (c.ink2, c.line),
    };

    return Semantics(
      selected: selected,
      button: true,
      // 原型 .chs .ch：padding 4px/s4 → 高 36、最小宽 65；选中 brand2 + 0 3 9 投影。
      child: DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: Dim.brPill,
          boxShadow: selected ? Shadows.glowSmall(c.brand2, alpha: .36) : null,
        ),
        child: Material(
          color: selected ? c.brand2 : c.surface,
          borderRadius: Dim.brPill,
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            onTap: onTap,
            child: Container(
              height: 36,
              constraints: const BoxConstraints(minWidth: 65),
              padding: const EdgeInsets.symmetric(horizontal: Dim.s6),
              decoration: BoxDecoration(
                borderRadius: Dim.brPill,
                border: Border.all(
                  color: selected ? c.brand2 : border,
                  width: 1.5,
                ),
              ),
              // 不用 Container.alignment：它在有界约束（Wrap / Column）里会撑满整行，
              // 真机上标签区就成了一排整宽胶囊。Center(widthFactor: 1) 才按内容收缩。
              child: Center(
                widthFactor: 1,
                child: Text(
                  label,
                  maxLines: 1,
                  style: TextStyle(
                    fontSize: Dim.t3,
                    fontWeight: FontWeight.w700,
                    height: 1.2,
                    color: selected ? Colors.white : fg,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 横向滚动的筛选条（原型 `.chs`）。
///
/// 原型 `.chs.raise` 只是 `margin: s2 0 0`，**没有负边距**——胶囊行在头图下方，不压头图。
/// 早先这里有个上提 18 的 `raise`，真机上三枚 Tab 胶囊骑在渐变头图的下沿，已去掉。
class ChipBar extends StatelessWidget {
  const ChipBar({super.key, required this.children, this.padding});

  final List<Widget> children;

  /// 四边内边距；高度 = 36 + 上下内边距，默认只有左右 gutter。
  final EdgeInsets? padding;

  @override
  Widget build(BuildContext context) {
    final pad = padding ?? const EdgeInsets.symmetric(horizontal: Dim.gutter);
    final bar = SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      padding: pad,
      child: Row(
        children: [
          for (var i = 0; i < children.length; i++) ...[
            if (i > 0) const SizedBox(width: Dim.s2),
            children[i],
          ],
        ],
      ),
    );
    // 宽度必须铺满：横向 ScrollView 在 Column 里会按内容收缩，
    // 然后被 Column 默认的 center 对齐居中——真机上三枚 Tab 胶囊就跑到了屏幕中间。
    return SizedBox(
      height: 36 + pad.vertical,
      width: double.infinity,
      child: bar,
    );
  }
}

/// 资料页 / 卡片上的小标签（距离、语言、兴趣）。
class TagChip extends StatelessWidget {
  const TagChip({
    super.key,
    required this.label,
    this.tone = TagTone.plain,
    this.icon,
  });

  final String label;
  final TagTone tone;
  final IconData? icon;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final (bg, fg) = switch (tone) {
      TagTone.pay => (c.brand.withValues(alpha: .14), c.brand),
      TagTone.purple => (c.brand2.withValues(alpha: .15), c.brand2),
      TagTone.gold => (c.coin.withValues(alpha: .2), c.gold),
      TagTone.plain => (c.sea, c.ink2),
    };

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: Dim.s2, vertical: Dim.s1),
      decoration: BoxDecoration(color: bg, borderRadius: Dim.brTag),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(icon, size: 11, color: fg),
            const SizedBox(width: 3),
          ],
          Text(
            label,
            style: TextStyle(
              fontSize: Dim.t1,
              fontWeight: FontWeight.w700,
              color: fg,
            ),
          ),
        ],
      ),
    );
  }
}

enum TagTone { plain, pay, purple, gold }

/// 分段控件（原型 `.seg`）：30px → 44 高，page 底、line2 描边，
/// **选中态铺满整格**（不是带内边距的浮起药丸）。
///
/// 默认选中色 brand；登录页的渠道切换传 [accent] = aqua——粉色在全 App 的语义是
/// 「要花币」，登录不该沾。
class SegmentedRow extends StatelessWidget {
  const SegmentedRow({
    super.key,
    required this.labels,
    required this.index,
    required this.onChanged,
    this.accent,
  });

  final List<String> labels;
  final int index;
  final ValueChanged<int> onChanged;
  final Color? accent;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final on = accent ?? c.brand;
    // 描边画在外层 Container 上会把总高撑到 46；用 foregroundDecoration 叠在上面。
    return Container(
      height: Dim.tap,
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(color: c.page, borderRadius: Dim.brPill),
      foregroundDecoration: BoxDecoration(
        borderRadius: Dim.brPill,
        border: Border.all(color: c.line2),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          for (var i = 0; i < labels.length; i++)
            Expanded(
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: () => onChanged(i),
                child: AnimatedContainer(
                  duration: const Duration(milliseconds: 180),
                  alignment: Alignment.center,
                  color: i == index ? on : Colors.transparent,
                  child: Text(
                    labels[i],
                    style: TextStyle(
                      fontSize: Dim.t2,
                      fontWeight: FontWeight.w700,
                      height: 1.4,
                      color: i == index ? Colors.white : c.ink2,
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

/// 多选「下拉」外观的字段（原型 `.selfield`）：已选项用小胶囊铺开（带 × 可删），右侧 ˅。
///
/// 最小高 50（34px → 50），r3，line 描边。点整行呼出选择器，点 × 直接移除一项。
class SelectField extends StatelessWidget {
  const SelectField({
    super.key,
    required this.selected,
    required this.placeholder,
    required this.onTap,
    this.onRemove,
  });

  final List<String> selected;
  final String placeholder;
  final VoidCallback onTap;
  final ValueChanged<String>? onRemove;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Container(
        constraints: const BoxConstraints(minHeight: 50),
        padding: const EdgeInsets.symmetric(
          horizontal: Dim.s3,
          vertical: Dim.s1,
        ),
        decoration: BoxDecoration(
          color: c.surface,
          borderRadius: Dim.brCard,
          border: Border.all(color: c.line),
        ),
        child: Row(
          children: [
            Expanded(
              child: selected.isEmpty
                  ? Text(
                      placeholder,
                      style: TextStyle(
                        fontSize: Dim.t3,
                        color: c.ink3,
                        height: 1.5,
                      ),
                    )
                  : Wrap(
                      spacing: Dim.s2,
                      runSpacing: Dim.s1,
                      children: [
                        for (final s in selected)
                          _SelectedChip(
                            label: s,
                            onRemove: onRemove == null
                                ? null
                                : () => onRemove!(s),
                          ),
                      ],
                    ),
            ),
            const SizedBox(width: Dim.s2),
            Icon(Icons.expand_more_rounded, size: 22, color: c.ink3),
          ],
        ),
      ),
    );
  }
}

/// `.selfield .st2`：sea 底胶囊，t2/700，右侧 × 为 ink3。
class _SelectedChip extends StatelessWidget {
  const _SelectedChip({required this.label, this.onRemove});

  final String label;
  final VoidCallback? onRemove;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      padding: const EdgeInsets.fromLTRB(Dim.s2, 2, Dim.s1, 2),
      decoration: BoxDecoration(color: c.sea, borderRadius: Dim.brPill),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            label,
            style: TextStyle(
              fontSize: Dim.t2,
              fontWeight: FontWeight.w700,
              color: c.ink,
              height: 1.4,
            ),
          ),
          if (onRemove != null)
            GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: onRemove,
              child: Padding(
                padding: const EdgeInsets.only(left: 2),
                child: Icon(Icons.close_rounded, size: 14, color: c.ink3),
              ),
            ),
        ],
      ),
    );
  }
}

/// 单选行（原型 `.pay`）：50 高、r3、1.5 描边；选中 brand 描边 + 右侧实心圆点（19，边 6）。
/// 支付方式、举报原因两处共用。
class RadioRow extends StatelessWidget {
  const RadioRow({
    super.key,
    required this.label,
    required this.selected,
    required this.onTap,
    this.icon,
    this.subtitle,
    this.trailing,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;
  final IconData? icon;
  final String? subtitle;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Container(
        constraints: const BoxConstraints(minHeight: 50),
        padding: const EdgeInsets.symmetric(
          horizontal: Dim.s4,
          vertical: Dim.s1,
        ),
        decoration: BoxDecoration(
          color: c.surface,
          borderRadius: Dim.brCard,
          border: Border.all(color: selected ? c.brand : c.line, width: 1.5),
        ),
        child: Row(
          children: [
            if (icon != null) ...[
              Icon(icon, size: 25, color: c.ink2),
              const SizedBox(width: Dim.s3),
            ],
            Expanded(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    label,
                    style: TextStyle(
                      fontSize: Dim.t3,
                      color: c.ink,
                      height: 1.5,
                    ),
                  ),
                  if (subtitle != null)
                    Text(
                      subtitle!,
                      style: TextStyle(
                        fontSize: Dim.t1,
                        color: c.ink3,
                        height: 1.4,
                      ),
                    ),
                ],
              ),
            ),
            ?trailing,
            const SizedBox(width: Dim.s3),
            Container(
              width: 19,
              height: 19,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(
                  color: selected ? c.brand : c.line,
                  width: selected ? 6 : 1.5,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 支付 / 订单状态小标（原型 `.stt`）：t0/800、padding 1.5px/6 → 2/6、r1。
enum StatusTagKind { ok, pending, error, refunded }

class StatusTag extends StatelessWidget {
  const StatusTag({super.key, required this.label, required this.kind});

  final String label;
  final StatusTagKind kind;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    const ok = Color(0xFF2FBF77);
    final (bg, fg) = switch (kind) {
      StatusTagKind.ok => (ok.withValues(alpha: .14), ok),
      StatusTagKind.pending => (c.aqua.withValues(alpha: .14), c.aqua),
      StatusTagKind.error => (c.warn.withValues(alpha: .12), c.warn),
      StatusTagKind.refunded => (c.sea, c.ink3),
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(color: bg, borderRadius: Dim.brTag),
      child: Text(
        label,
        maxLines: 1,
        style: TextStyle(
          fontSize: Dim.t0,
          fontWeight: FontWeight.w800,
          color: fg,
          height: 1.4,
        ),
      ),
    );
  }
}

/// 订单号这类要被复制 / 报给客服的串（原型 `.oid`）：等宽、t1、ink3、sea 底、r1。
class OrderIdChip extends StatelessWidget {
  const OrderIdChip({super.key, required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(color: c.sea, borderRadius: Dim.brTag),
      child: Text(
        text,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(
          fontSize: Dim.t1,
          fontFamily: 'monospace',
          fontFeatures: const [FontFeature.tabularFigures()],
          color: c.ink3,
          height: 1.4,
        ),
      ),
    );
  }
}

/// 值字段（原型 `.fieldr .val`）：34px → 50 高、r3、line 描边，左侧值 t3/600 ink，
/// 右侧 › 提示可点开改。「一个值 + 弹层改」的场景（年龄区间）用它，别把整条滑块摊在页面上。
class ValueField extends StatelessWidget {
  const ValueField({
    super.key,
    required this.value,
    required this.onTap,
    this.leading,
  });

  final String value;
  final VoidCallback onTap;
  final IconData? leading;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Container(
        height: 50,
        padding: const EdgeInsets.symmetric(horizontal: Dim.s3),
        decoration: BoxDecoration(
          color: c.surface,
          borderRadius: Dim.brCard,
          border: Border.all(color: c.line),
        ),
        child: Row(
          children: [
            if (leading != null) ...[
              Icon(leading, size: 20, color: c.ink3),
              const SizedBox(width: Dim.s2),
            ],
            Expanded(
              child: Text(
                value,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: Dim.t3,
                  fontWeight: FontWeight.w600,
                  height: 1.4,
                  color: c.ink,
                ),
              ),
            ),
            Icon(Icons.chevron_right_rounded, size: 20, color: c.ink3),
          ],
        ),
      ),
    );
  }
}
