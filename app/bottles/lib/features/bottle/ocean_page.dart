import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/logging/log_record.dart';
import '../../core/logging/logger.dart';
import '../../core/catalog.dart';
import '../../core/design/tokens.dart';
import '../../core/network/api_exception.dart';
import '../../core/platform/haptics.dart';
import '../../core/providers.dart';
import '../../domain/models/bottle.dart';
import '../../domain/models/user.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/flows/purchase_flows.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/coin.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/sea_scene.dart';
import '../../ui/widgets/states.dart';
import 'ocean_controller.dart';
import 'scoop_flow.dart';

enum _Phase { idle, scooping, opened }

/// B1 海洋首页（含 B1n 夜场、B1a 捞取中、B1b 开瓶、Z1 空态）。
///
/// 类别：场景页（规范 §6）
/// 固定区：头图 GradientHeader、底部 DockButton 两枚
/// 弹性区：SeaScene 画布（375×490 坐标系，按宽度等比；高 < 700 时裁底）
/// 可滚动区：无
/// 键盘：无
/// 高屏多余高度：全部给画布，海水渐变向下延伸
///
/// 配色规则：海里两个按钮**都是海蓝**——全 App 用「粉 = 要花币」，
/// 海面整片不出现粉色，就是在告诉用户「这里免费」。
class OceanPage extends ConsumerStatefulWidget {
  const OceanPage({super.key});

  @override
  ConsumerState<OceanPage> createState() => _OceanPageState();
}

class _OceanPageState extends ConsumerState<OceanPage> {
  _Phase _phase = _Phase.idle;
  Bottle? _pending;
  bool _ready = false;
  bool _casting = false;
  DateTime? _pressedAt;
  double _pullOffset = 0;

  /// 本次捞取由点击触发：仪式放完自动开瓶，不等松手，也不做误触判定。
  bool _byTap = false;
  Timer? _hapticTimer;

  /// 捞取中的持续轻震——「正在把瓶子往上拉」的手感就来自这里。
  ///
  /// 上限 12 次（约 4.2s）：弱网下接口迟迟不回，也不能一直震下去。
  void _beginHaptics() {
    Haptics.medium();
    _hapticTimer?.cancel();
    var ticks = 0;
    _hapticTimer = Timer.periodic(const Duration(milliseconds: 350), (_) {
      if (++ticks > 12) {
        _stopHaptics();
        return;
      }
      Haptics.tick();
    });
  }

  void _stopHaptics() {
    _hapticTimer?.cancel();
    _hapticTimer = null;
  }

  @override
  void dispose() {
    _stopHaptics();
    super.dispose();
  }

  /// 按下瞬间就发请求，动画与网络并行——用户感知不到加载。
  ///
  /// [byTap] 点击触发，与长按共用整套仪式，只在收尾方式上分叉。
  Future<void> _startScoop({bool byTap = false}) async {
    if (_phase != _Phase.idle) return;

    final quota = ref.read(quotaProvider).value;
    if (quota != null && !quota.canScoop) {
      await showQuotaExceededModal(context, ref, usedUp: quota.scoopTotal);
      return;
    }

    setState(() {
      _phase = _Phase.scooping;
      _ready = false;
      _pending = null;
      _byTap = byTap;
      _pressedAt = DateTime.now();
    });
    _beginHaptics();

    final tag = ref.read(oceanTagProvider);
    try {
      final bottle = await ref
          .read(bottleRepoProvider)
          .scoop(tag: tag == 'all' ? null : tag);
      Log.i(LogTag.biz, 'scoop', fields: {'empty': bottle == null});
      if (!mounted) return;
      if (bottle == null) {
        // 空池子不是错误，走空态而不是错误分支。
        _stopHaptics();
        setState(() => _phase = _Phase.idle);
        return;
      }
      setState(() {
        _pending = bottle;
        _ready = true;
      });
      // 咬钩：拉扯停下，换一记重震——这一下是整段仪式的情绪顶点。
      _stopHaptics();
      Haptics.heavy();
      await ref.read(quotaProvider.notifier).refresh();
      if (byTap) await _openAfterRitual();
    } on ApiException catch (e) {
      if (!mounted) return;
      _stopHaptics();
      setState(() => _phase = _Phase.idle);
      if (e.isQuotaExceeded) {
        final total = ref.read(quotaProvider).value?.scoopTotal ?? 0;
        await showQuotaExceededModal(context, ref, usedUp: total);
      } else {
        // 动画期间接口失败 → 瓶子沉回去 + 轻提示，不扣次数。
        showToast(context, L.of(context).oceanScoopFailed);
      }
    }
  }

  /// 点击路径的收尾：仪式与网络谁慢等谁，两边都就绪才开瓶。
  Future<void> _openAfterRitual() async {
    final remain =
        Motion.scoopRitual -
        DateTime.now().difference(_pressedAt ?? DateTime.now());
    if (remain > Duration.zero) await Future.delayed(remain);
    if (!mounted || _phase != _Phase.scooping) return;
    Haptics.light();
    setState(() => _phase = _Phase.opened);
  }

  /// 松手。按住不足 400ms 视为误触：瓶子沉回海里，不扣次数。
  ///
  /// ⚠️ 点击时长按识别器会被 tap 挤掉并触发 onLongPressCancel，
  /// 也走到这里——`_byTap` 这道守卫拦的就是它，否则刚起手的点击会被自己掐断。
  Future<void> _endScoop() async {
    if (_phase != _Phase.scooping || _byTap) return;
    _stopHaptics();
    final held = DateTime.now().difference(_pressedAt ?? DateTime.now());
    final bottle = _pending;

    if (bottle == null || held.inMilliseconds < 400) {
      setState(() => _phase = _Phase.idle);
      if (bottle != null) {
        await ref.read(bottleRepoProvider).putBack(bottle.id);
        await ref.read(quotaProvider.notifier).refresh();
        if (mounted) showToast(context, L.of(context).oceanScoopFailed);
      }
      return;
    }
    Haptics.light();
    setState(() => _phase = _Phase.opened);
  }

  Future<void> _putBack() async {
    final bottle = _pending;
    setState(() {
      _phase = _Phase.idle;
      _pending = null;
    });
    // 放回 = 记 MatchLog(action=skip)，不写 view。
    if (bottle != null) await ref.read(bottleRepoProvider).putBack(bottle.id);
  }

  void _reply() {
    final bottle = _pending;
    setState(() => _phase = _Phase.idle);
    if (bottle != null) context.push(Routes.bottleDetail(bottle.id));
  }

  Future<void> _throwBottle() async {
    final quota = ref.read(quotaProvider).value;
    final night = ref.read(nightModeProvider);
    // 夜场扔深夜瓶不限次，用配额差异把人留在这个时段。
    if (!night && quota != null && !quota.canThrow) {
      await showQuotaExceededModal(context, ref, usedUp: quota.throwTotal);
      return;
    }
    if (mounted) context.push(Routes.bottleWrite);
  }

  void _showSeaReport(SeaCondition condition) {
    final l = L.of(context);
    showAppSheet<void>(
      context,
      builder: (_) => Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(l.oceanSeaReport, style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: Dim.s4),
          _ReportRow(
            label: l.oceanOnlineTonight(condition.onlineCount),
            emoji: '🌊',
          ),
          _ReportRow(
            label: l.oceanSubtitle(
              condition.repliesDrifting,
              _hhmm(condition.nightStartsAt),
            ),
            emoji: '💌',
          ),
          const SizedBox(height: Dim.s4),
        ],
      ),
    );
  }

  Future<void> _setSkin(OceanSkin skin) async {
    await ref.read(oceanSkinProvider.notifier).set(skin);
    if (!mounted) return;
    final l = L.of(context);
    showToast(context, switch (skin) {
      OceanSkin.day => l.oceanSkinDay,
      OceanSkin.night => l.oceanSkinNight,
      OceanSkin.auto => l.oceanSkinAuto,
    });
  }

  static String _hhmm(DateTime t) =>
      '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';

  @override
  Widget build(BuildContext context) {
    // B2 在另一个路由里提交，回到首页时把抛瓶动画补演出来。
    ref.listen<int>(castSignalProvider, (prev, next) {
      if (prev != null && next > prev) setState(() => _casting = true);
    });

    final l = L.of(context);
    // night = 夜场（产品状态：夜瓶、不限次扔瓶、标题）；look = 画哪张海（用户可手动切）。
    final night = ref.watch(nightModeProvider);
    final look = ref.watch(seaLookProvider);
    final visualNight = look == SeaLook.night;
    final ocean = ref.watch(oceanProvider);
    final quota = ref.watch(quotaProvider).value;
    final wallet = ref.watch(walletProvider).value;
    final me = ref.watch(authProvider).profile;
    final peek = ocean.value?.peek ?? const <Bottle>[];

    return Scaffold(
      body: Stack(
        children: [
          // 整屏海面：头像、金币、昼夜切换、筛选都悬浮在它上面，不再有不透明的头图。
          Positioned.fill(
            child: GestureDetector(
              onVerticalDragUpdate: (d) {
                if (d.delta.dy > 0) {
                  setState(
                    () => _pullOffset = (_pullOffset + d.delta.dy).clamp(0, 90),
                  );
                }
              },
              onVerticalDragEnd: (_) async {
                final shouldRefresh = _pullOffset > 60;
                setState(() => _pullOffset = 0);
                if (shouldRefresh) {
                  await ref.read(oceanProvider.notifier).refreshTide();
                }
              },
              child: Transform.translate(
                offset: Offset(0, _pullOffset * .25),
                child: SeaScene(
                  night: visualNight,
                  dimmed: _phase != _Phase.idle,
                  bottles: [
                    for (var i = 0; i < peek.length; i++)
                      _bottle(i, peek[i], visualNight),
                  ],
                ),
              ),
            ),
          ),

          // 悬浮顶栏：头像 / 金币 / 昼夜切换 / 我的瓶子，下面一行是筛选胶囊。
          Positioned(
            left: 0,
            right: 0,
            top: 0,
            child: SafeArea(
              bottom: false,
              child: Padding(
                padding: const EdgeInsets.fromLTRB(
                  Dim.gutter,
                  Dim.s2,
                  Dim.gutter,
                  0,
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Row(
                      children: [
                        AvatarRing(
                          avatar: me?.brief.avatar,
                          seed: me?.id ?? '',
                          gender: me?.brief.gender ?? Gender.secret,
                          size: 34,
                          online: true,
                          onTap: () => context.go(Routes.me),
                        ),
                        const SizedBox(width: Dim.s2),
                        CoinChip(
                          coins: wallet?.coins ?? 0,
                          onTap: () => context.push(Routes.recharge),
                        ),
                        const Spacer(),
                        // 手动切白天 / 夜晚海面；长按恢复跟随时间。只换画面，不动夜场规则。
                        GestureDetector(
                          onLongPress: () => _setSkin(OceanSkin.auto),
                          child: HeaderIconButton(
                            key: const ValueKey('ocean-skin-toggle'),
                            icon: visualNight
                                ? Icons.wb_sunny_rounded
                                : Icons.nightlight_round,
                            onTap: () => _setSkin(
                              visualNight ? OceanSkin.day : OceanSkin.night,
                            ),
                          ),
                        ),
                        const SizedBox(width: Dim.s2),
                        HeaderIconButton(
                          icon: Icons.menu_rounded,
                          onTap: () => context.push(Routes.myBottles),
                        ),
                      ],
                    ),
                    const SizedBox(height: Dim.s3),
                    _TagChips(
                      current: ref.watch(oceanTagProvider),
                      night: night,
                      onSelect: ref.read(oceanTagProvider.notifier).set,
                    ),
                  ],
                ),
              ),
            ),
          ),

          // 空态：捞瓶 feed 为空时，把死路转成投放量。
          if (ocean.hasValue && peek.isEmpty && _phase == _Phase.idle)
            Positioned.fill(
              bottom: 120,
              child: EmptyState(
                onDark: true,
                icon: Icons.waves_rounded,
                title: l.stateEmptyOceanTitle,
                body: l.stateEmptyOceanBody,
                ctaLabel: l.stateEmptyOceanCta,
                onCta: _throwBottle,
              ),
            ),

          // 固定区：海况标签 + 两枚 dock 按钮贴着画布底部（原型 .dock bottom s4）。
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (_phase == _Phase.idle && ocean.value != null)
                  Padding(
                    padding: const EdgeInsets.only(bottom: Dim.s2),
                    child: _SeaReportTag(
                      label: night
                          ? l.oceanOnlineTonight(
                              ocean.value!.condition.onlineCount,
                            )
                          : l.oceanSeaReport,
                      onTap: () => _showSeaReport(ocean.value!.condition),
                    ),
                  ),
                Padding(
                  padding: const EdgeInsets.fromLTRB(
                    Dim.gutter,
                    0,
                    Dim.gutter,
                    Dim.s4,
                  ),
                  child: Row(
                    children: [
                      DockButton(
                        label: l.oceanThrowOne,
                        hint: night
                            ? l.oceanThrowUnlimited
                            : l.oceanThrowRemain(quota?.throwLeft ?? 0),
                        glass: true,
                        onTap: _throwBottle,
                      ),
                      const SizedBox(width: Dim.s2),
                      DockButton(
                        label: l.oceanScoopOne,
                        hint: l.oceanScoopRemain(quota?.scoopLeft ?? 0),
                        onTap: () => _startScoop(byTap: true),
                        onLongPressStart: _startScoop,
                        onLongPressEnd: _endScoop,
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),

          if (_casting)
            CastBottleOverlay(onDone: () => setState(() => _casting = false)),

          if (_phase == _Phase.scooping)
            ScoopRitualLayer(ready: _ready, byTap: _byTap),
          if (_phase == _Phase.opened && _pending != null)
            LetterLayer(
              bottle: _pending!,
              onPutBack: _putBack,
              onReply: _reply,
            ),
        ],
      ),
    );
  }

  /// 海面上的一只瓶子。位置由 SeaScene 按水带分配；这里只管样子与点击。
  /// 点瓶子 = 捞一个：与「捞一个」按钮同一套仪式与配额逻辑（点击触发、仪式放完自动开瓶）。
  /// 捞上来的是 feed 推荐的那只，不一定是被点的这只——海面上的瓶子只是「有东西漂着」的示意。
  Widget _bottle(int i, Bottle b, bool night) {
    return FloatingBottle(
      replyCount: b.replyCount,
      phase: (i * 0.23) % 1,
      night: night,
      tilt: bottleTilts[i % bottleTilts.length],
      // 整图素材左右朝向交替；夜晚由 SeaBottleImage 换成抠掉水面的 -night 变体。
      art: i.isEven ? BottleArt.seaLeft : BottleArt.seaRight,
      onTap: () => _startScoop(byTap: true),
    );
  }
}

/// 悬浮在海面上的筛选胶囊：全部 + 瓶子标签。夜场标签只在夜场时段出现——
/// 深夜瓶本来就只在那时能捞，白天给一个点了没结果的筛选等于骗人。
class _TagChips extends StatelessWidget {
  const _TagChips({
    required this.current,
    required this.night,
    required this.onSelect,
  });

  final String current;
  final bool night;
  final ValueChanged<String> onSelect;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final keys = [
      'all',
      for (final k in Catalog.bottleTags.keys)
        if (k != 'night' || night) k,
    ];
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      clipBehavior: Clip.none,
      child: Row(
        children: [
          for (final k in keys) ...[
            _GlassChip(
              key: ValueKey('ocean-tag-$k'),
              label: k == 'all' ? l.commonAll : Catalog.bottleTag(context, k),
              selected: current == k,
              onTap: () => onSelect(k),
            ),
            const SizedBox(width: Dim.s2),
          ],
        ],
      ),
    );
  }
}

/// 海面上的玻璃胶囊：选中白底墨字，未选半透明白字——两种状态在亮天空与深夜空上都读得清。
class _GlassChip extends StatelessWidget {
  const _GlassChip({
    super.key,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 140),
        padding: const EdgeInsets.symmetric(
          horizontal: Dim.s3,
          vertical: Dim.s1 + 2,
        ),
        decoration: BoxDecoration(
          color: selected
              ? Colors.white.withValues(alpha: .92)
              : Colors.black.withValues(alpha: .22),
          borderRadius: Dim.brPill,
          border: Border.all(
            color: Colors.white.withValues(alpha: selected ? 0 : .35),
          ),
        ),
        child: Text(
          label,
          style: TextStyle(
            fontSize: Dim.t1,
            fontWeight: FontWeight.w700,
            height: 1.3,
            color: selected ? const Color(0xFF16323C) : Colors.white,
          ),
        ),
      ),
    );
  }
}

class _SeaReportTag extends StatelessWidget {
  const _SeaReportTag({required this.label, required this.onTap});

  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: Dim.s3,
          vertical: Dim.s1 + 2,
        ),
        decoration: BoxDecoration(
          color: Colors.black.withValues(alpha: .28),
          borderRadius: Dim.brPill,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              label,
              style: const TextStyle(
                fontSize: Dim.t1,
                fontWeight: FontWeight.w700,
                color: Colors.white,
              ),
            ),
            const Icon(
              Icons.chevron_right_rounded,
              size: 14,
              color: Colors.white,
            ),
          ],
        ),
      ),
    );
  }
}

class _ReportRow extends StatelessWidget {
  const _ReportRow({required this.label, required this.emoji});

  final String label;
  final String emoji;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: Dim.s2),
      child: Row(
        children: [
          Text(emoji, style: const TextStyle(fontSize: 18)),
          const SizedBox(width: Dim.s3),
          Expanded(
            child: Text(
              label,
              style: TextStyle(fontSize: Dim.t3, color: context.c.ink),
            ),
          ),
        ],
      ),
    );
  }
}
