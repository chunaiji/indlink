import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';

/// 品牌渐变头图（`.hdr`）。
///
/// 渐变只用于品牌面（头图 / Tab 选中块 / 进度条），按钮一律实色。
/// 夜场时整块转深蓝紫——粉色不参与昼夜叙事。
class GradientHeader extends StatelessWidget {
  const GradientHeader({
    super.key,
    required this.topRow,
    this.title,
    this.subtitle,
    this.night = false,
    this.slim = false,
    this.bottomPadding,
    this.bottom,
  });

  final Widget topRow;
  final String? title;
  final String? subtitle;
  final bool night;
  final bool slim;
  final double? bottomPadding;

  /// 头图内、标题/副标题下方的附加内容（如「我的」页的关注/粉丝/魅力 stats 行）。
  final Widget? bottom;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final colors = night ? AppColors.nightGradient : c.brandGradient;

    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: const Alignment(-0.9, -1),
          end: const Alignment(0.9, 1),
          colors: colors,
          stops: const [0, .52, 1],
        ),
      ),
      child: Stack(
        children: [
          // 装饰泡泡：让大面积渐变不至于太平，但不能抢内容。
          Positioned(left: -20, top: 10, child: _bubble(56, .20)),
          Positioned(right: 30, top: -24, child: _bubble(86, .15)),
          Positioned(right: -30, bottom: -30, child: _bubble(70, .10)),
          SafeArea(
            bottom: false,
            child: Padding(
              padding: EdgeInsets.fromLTRB(
                Dim.gutter,
                Dim.s2,
                Dim.gutter,
                bottomPadding ?? (slim ? Dim.s3 : Dim.s5),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  // 顶行最小 44，但允许更高：「我的」页昵称 + ID 两行在大字体下会超过 44。
                  // slim 版（动态 / 会话列表）只放标题 + 30 高的金币胶囊，按 34 收口：
                  // 原型 .hdr.slim 整块 ≈ 79pt，44 的顶行会让它比原型高出 12pt。
                  ConstrainedBox(
                    constraints: BoxConstraints(minHeight: slim ? 34 : Dim.tap),
                    child: topRow,
                  ),
                  if (title != null) ...[
                    const SizedBox(height: Dim.s4),
                    Text(
                      title!,
                      style: const TextStyle(
                        fontSize: Dim.t6,
                        fontWeight: FontWeight.w800,
                        color: Colors.white,
                        height: 1.25,
                        letterSpacing: -0.24,
                      ),
                    ),
                  ],
                  if (subtitle != null) ...[
                    const SizedBox(height: Dim.s1),
                    Text(
                      subtitle!,
                      style: TextStyle(
                        fontSize: Dim.t2,
                        height: 1.5,
                        color: Colors.white.withValues(alpha: .85),
                      ),
                    ),
                  ],
                  if (bottom != null) ...[
                    const SizedBox(height: Dim.s3),
                    bottom!,
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _bubble(double size, double alpha) => Container(
    width: size,
    height: size,
    decoration: BoxDecoration(
      shape: BoxShape.circle,
      color: Colors.white.withValues(alpha: alpha),
    ),
  );
}

/// 头图上的图标按钮（`.hdr .ic`）。
class HeaderIconButton extends StatelessWidget {
  const HeaderIconButton({
    super.key,
    required this.icon,
    this.onTap,
    this.badge,
  });

  final IconData icon;
  final VoidCallback? onTap;
  final Widget? badge;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: Dim.tap,
      height: Dim.tap,
      child: Stack(
        alignment: Alignment.center,
        children: [
          // 原型 .hdr .ic：23px → 34pt 实心白圆（.94）、红字；不是半透明玻璃圆。
          InkResponse(
            onTap: onTap,
            radius: 22,
            child: Container(
              width: 34,
              height: 34,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: Colors.white.withValues(alpha: .94),
              ),
              child: Icon(icon, size: 20, color: const Color(0xFFC43158)),
            ),
          ),
          if (badge != null) Positioned(right: 4, top: 6, child: badge!),
        ],
      ),
    );
  }
}

/// 二级页导航条（`.nv`）。
///
/// 举报 / 拉黑入口必须常驻这里，不可藏在更深的层级（App Store 1.2）。
class NavBar extends StatelessWidget implements PreferredSizeWidget {
  const NavBar({
    super.key,
    this.title,
    this.titleWidget,
    this.leading,
    this.actions = const [],
    this.onBack,
    this.closeIcon = false,
    this.subtitle,
    this.background,
  });

  final String? title;
  final Widget? titleWidget;
  final Widget? leading;
  final List<Widget> actions;
  final VoidCallback? onBack;

  /// 模态式页面（写瓶子 / 筛选）用 ✕ 而不是 ←。
  final bool closeIcon;

  /// 右侧灰色补充说明（「今天还能扔 3 个」）。
  final String? subtitle;

  final Color? background;

  /// 原型 .nv：30px → 44pt。
  @override
  Size get preferredSize => const Size.fromHeight(Dim.tap);

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Material(
      color: background ?? c.page,
      child: SafeArea(
        bottom: false,
        child: Container(
          height: preferredSize.height,
          decoration: BoxDecoration(
            border: Border(bottom: BorderSide(color: c.line2)),
          ),
          padding: const EdgeInsets.only(right: Dim.s2),
          child: LayoutBuilder(
            builder: (context, box) => Row(
              children: [
                leading ??
                    IconButton(
                      icon: Icon(
                        closeIcon
                            ? Icons.close_rounded
                            : Icons.arrow_back_ios_new_rounded,
                        size: closeIcon ? 22 : 18,
                        color: c.ink,
                      ),
                      onPressed:
                          onBack ?? () => Navigator.of(context).maybePop(),
                    ),
                Expanded(
                  child:
                      titleWidget ??
                      Text(
                        title ?? '',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: Dim.t4,
                          fontWeight: FontWeight.w700,
                          color: c.ink,
                          letterSpacing: -0.1,
                        ),
                      ),
                ),
                // 右侧两样都按内容定宽、贴右沿；标题是唯一让步的（Expanded 可缩到 0）。
                // 之前把它们也放进 Flexible：Row 先按 flex 均分空间，内容不满自己那份时
                // 空隙留在右边——真机上「发布」「全部已读」都没贴到右侧。
                if (subtitle != null)
                  ConstrainedBox(
                    constraints: BoxConstraints(maxWidth: box.maxWidth * .35),
                    child: Padding(
                      padding: const EdgeInsets.only(right: Dim.s2),
                      // 原型 .nv .sp2：t2 常规。可收缩：窄屏 + 大字体下标题优先。
                      child: Text(
                        subtitle!,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: Dim.t2,
                          color: c.ink3,
                          height: 1.4,
                        ),
                      ),
                    ),
                  ),
                // actions 超过 45% 宽时等比缩小而不是溢出——
                // 360 宽 × 1.2 字体下「恢复购买」这类文字按钮会碰到这条线。
                if (actions.isNotEmpty)
                  ConstrainedBox(
                    constraints: BoxConstraints(maxWidth: box.maxWidth * .45),
                    child: FittedBox(
                      fit: BoxFit.scaleDown,
                      alignment: Alignment.centerRight,
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: actions,
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// 底部固定动作区（`.foot`）：顶线 line2，padding s3 s4，底部 = s4 + 安全区。
///
/// 是相加不是取大：手势条区域本身不能放内容，按钮还得离它 s4——
/// 取大的话在手势导航的机器上按钮会直接骑在手势条上（真机反馈「底部没留白」）。
///
/// 它自己读 `viewPadding`，调用方**不要**再包 `SafeArea(bottom: true)`，
/// 否则三键导航机型会叠加两份底部留白（规范 §4.4）。多个按钮纵向排，间距 s2。
class BottomActionBar extends StatelessWidget {
  /// 二选一：单个 [child]，或纵向排布的 [children]。两个都不给就是一个空条。
  const BottomActionBar({
    super.key,
    this.child,
    this.children = const [],
    this.flat = false,
  });

  final Widget? child;
  final List<Widget> children;

  /// 不画顶部分割线、不铺底色：原型里资料页（C3）的两枚按钮直接落在 .bd 末尾，
  /// 没有 .foot 那条线。其余表单页保持默认。
  final bool flat;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final inset = MediaQuery.viewPaddingOf(context).bottom;
    final items = child != null ? [child!] : children;
    return Container(
      decoration: flat
          ? null
          : BoxDecoration(
              color: c.page,
              border: Border(top: BorderSide(color: c.line2)),
            ),
      padding: EdgeInsets.fromLTRB(
        Dim.gutter,
        Dim.s3,
        Dim.gutter,
        Dim.s4 + inset,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          for (var i = 0; i < items.length; i++) ...[
            if (i > 0) const SizedBox(height: Dim.s2),
            items[i],
          ],
        ],
      ),
    );
  }
}

/// 头图内的三格统计（原型 `.hdr .stats`）：白 .16 底、r3、数字 t4/800、标签 t0。
class HeaderStats extends StatelessWidget {
  const HeaderStats({super.key, required this.items});

  final List<(String value, String label)> items;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(top: Dim.s3),
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: .16),
        borderRadius: Dim.brCard,
      ),
      child: Row(
        children: [
          for (var i = 0; i < items.length; i++)
            Expanded(
              child: Container(
                padding: const EdgeInsets.symmetric(vertical: Dim.s2),
                decoration: i == 0
                    ? null
                    : BoxDecoration(
                        border: Border(
                          left: BorderSide(
                            color: Colors.white.withValues(alpha: .22),
                          ),
                        ),
                      ),
                child: Column(
                  children: [
                    Text(
                      items[i].$1,
                      style: const TextStyle(
                        fontSize: Dim.t4,
                        fontWeight: FontWeight.w800,
                        color: Colors.white,
                        height: 1.25,
                        fontFeatures: [FontFeature.tabularFigures()],
                      ),
                    ),
                    Text(
                      items[i].$2,
                      style: TextStyle(
                        fontSize: Dim.t0,
                        color: Colors.white.withValues(alpha: .85),
                        height: 1.4,
                      ),
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
