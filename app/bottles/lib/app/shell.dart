import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../core/design/tokens.dart';
import '../core/providers.dart';
import '../l10n/app_localizations.dart';
import '../features/chat/chat_controller.dart';
import '../features/location/location_controller.dart';
import '../features/location/location_intro_page.dart';
import '../ui/widgets/avatar.dart';

/// 5 Tab 外壳。各 Tab 保留自己的 Navigator 栈与滚动位置。
///
/// 有状态是为了挂一次性的进入钩子：定位。
class AppShell extends ConsumerStatefulWidget {
  const AppShell({super.key, required this.navigationShell});

  final StatefulNavigationShell navigationShell;

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  @override
  void initState() {
    super.initState();
    // 进主界面之后再处理定位，而**不是**在注册流程里——
    // 注册时多一个权限框会明显拉低完成率，而 iOS 的定位框只给一次机会。
    WidgetsBinding.instance.addPostFrameCallback((_) => _initLocation());
  }

  Future<void> _initLocation() async {
    final ctrl = ref.read(locationControllerProvider.notifier);
    // 已授权就静默取位置并上报；没授权时这一步什么都不做，不弹任何框。
    await ctrl.refreshIfGranted();
    if (!mounted) return;
    final prefs = ref.read(prefsProvider);
    if (prefs.locationAsked || !prefs.locationEnabled) return;
    // 只在「还能再问」时展示 A7：已授权不必问，已被永久拒绝时系统框也唤不起来，
    // 再给一个「开启定位」按钮等于骗用户点一个不会有反应的东西。
    if (!ref.read(locationControllerProvider).canAsk) return;
    await Navigator.of(
      context,
      rootNavigator: true,
    ).push(MaterialPageRoute<void>(builder: (_) => const LocationIntroPage()));
  }

  @override
  Widget build(BuildContext context) {
    // 激活常驻消息分发器:登录进主界面后一直订阅全局消息流,
    // 保证收到任意会话的新消息时,会话列表 / 未读 / Tab 红点都实时刷新。
    ref.watch(chatDispatcherProvider);
    final navigationShell = widget.navigationShell;
    // 宽 ≥ 600（折叠屏展开 / 平板）：内容列限宽居中，Tab 栏保持全宽（规范 §4.2）。
    // 每次 build 重新判，折叠 / 展开切换时窗口尺寸会变，不能缓存。
    final body = Breakpoints.isWide(context)
        ? Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: Breakpoints.contentMaxWidth,
              ),
              child: navigationShell,
            ),
          )
        : navigationShell;
    return Scaffold(
      body: body,
      bottomNavigationBar: AppTabBar(
        index: navigationShell.currentIndex,
        onTap: (i) {
          // 再点当前 Tab 回到该栈的根页。
          navigationShell.goBranch(
            i,
            initialLocation: i == navigationShell.currentIndex,
          );
          if (i == 2) ref.read(unreadProvider.notifier).clearDelayed();
        },
      ),
    );
  }
}

/// 底部 5 Tab（原型 `.tb.hf`）：图标 27、标签 t1/700、上下 10pt，选中只变 brand 色。
class AppTabBar extends ConsumerWidget {
  const AppTabBar({super.key, required this.index, required this.onTap});

  final int index;
  final ValueChanged<int> onTap;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final unread = ref.watch(unreadProvider);

    // 图标语汇跟着产品走：海洋是波、发现是罗盘、动态是四角星。
    // 不用 emoji —— emoji 自带颜色，压在彩色选中块上会很脏，且各机型字形不一。
    final items = <(IconData, String, int)>[
      (Icons.waves_rounded, l.tabOcean, 0),
      (Icons.explore_outlined, l.tabDiscover, 0),
      (Icons.chat_bubble_outline_rounded, l.tabChats, unread),
      (Icons.auto_awesome_outlined, l.tabMoments, 0),
      (Icons.person_outline_rounded, l.tabMe, 0),
    ];

    return Container(
      decoration: BoxDecoration(
        color: c.surface,
        border: Border(top: BorderSide(color: c.line2)),
      ),
      child: SafeArea(
        top: false,
        // 高度由内容撑：10 + 27 + 4 + 17 + 10 ≈ 68，不写死。
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (var i = 0; i < items.length; i++)
              Expanded(
                child: _TabItem(
                  icon: items[i].$1,
                  label: items[i].$2,
                  badge: items[i].$3,
                  selected: i == index,
                  onTap: () => onTap(i),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _TabItem extends StatelessWidget {
  const _TabItem({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
    this.badge = 0,
  });

  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;
  final int badge;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return InkResponse(
      onTap: onTap,
      radius: 40,
      child: Padding(
        // 原型 .tb.hf div：padding 7px → 10pt，图标 18px → 27，标签 t1/700。
        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 10),
        // 选中态只体现在图标 + 文字变 brand 色，不再用渐变药丸背景。
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Stack(
              clipBehavior: Clip.none,
              children: [
                Icon(icon, size: 27, color: selected ? c.brand : c.ink3),
                if (badge > 0)
                  Positioned(
                    right: -10,
                    top: -5,
                    child: UnreadBadge(count: badge),
                  ),
              ],
            ),
            const SizedBox(height: Dim.s1),
            Text(
              label,
              maxLines: 1,
              style: TextStyle(
                fontSize: Dim.t1,
                height: 1.4,
                fontWeight: FontWeight.w700,
                color: selected ? c.brand : c.ink3,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
