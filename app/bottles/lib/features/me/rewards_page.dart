import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:share_plus/share_plus.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';
import 'me_controller.dart';

/// H6 签到与奖励。
///
/// ⚠️ 现有 `ad` 模块是**微信流量主**，16 个广告位配置在 App 端作废，需换 AdMob。
/// 邀请奖励是薅羊毛重灾区（设备指纹 + 同 IP 限制 + 新用户留存门槛），属反作弊模块范畴。
/// 类别：列表页（规范 §6）
/// 固定区：NavBar（已连签 N 天）
/// 弹性区：无
/// 可滚动区：签到卡 + 「还能这样赚」分组
/// 键盘：无
class RewardsPage extends ConsumerStatefulWidget {
  const RewardsPage({super.key});

  @override
  ConsumerState<RewardsPage> createState() => _RewardsPageState();
}

class _RewardsPageState extends ConsumerState<RewardsPage> {
  bool _busy = false;

  Future<void> _checkin() async {
    setState(() => _busy = true);
    try {
      final coins = await ref.read(checkinProvider.notifier).checkin();
      if (mounted && coins > 0) {
        showToast(context, L.of(context).rewardGot(coins));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// 邀请好友。
  ///
  /// 邀请码带在链接里，注册时回传给后端发双边奖励。
  /// ⚠️ 这是薅羊毛重灾区，服务端必须配合设备指纹 + 同 IP 限制 + 新用户留存门槛。
  Future<void> _invite() async {
    final me = ref.read(authProvider).profile;
    final l = L.of(context);
    await SharePlus.instance.share(
      ShareParams(
        text: '${l.appTagline}\nhttps://ambertu.com/drift?ref=${me?.id ?? ''}',
        subject: l.appName,
      ),
    );
  }

  Future<void> _watchAd() async {
    setState(() => _busy = true);
    try {
      final coins = await ref.read(checkinProvider.notifier).adReward();
      if (mounted && coins > 0) {
        showToast(context, L.of(context).rewardGot(coins));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final status = ref.watch(checkinProvider);

    return Scaffold(
      appBar: NavBar(
        title: l.rewardTitle,
        subtitle: status.value == null
            ? null
            : l.rewardStreakBadge(status.value!.streak),
      ),
      body: AsyncView(
        value: status,
        onRetry: () => ref.invalidate(checkinProvider),
        errorTitle: l.stateErrorTitle,
        errorBody: l.stateErrorBody,
        retryLabel: l.commonRetry,
        skeleton: const Padding(
          padding: EdgeInsets.all(Dim.gutter),
          child: SkeletonRows(count: 4),
        ),
        data: (s) => ListView(
          padding: context.pagePadding(),
          children: [
            BrandCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.baseline,
                    textBaseline: TextBaseline.alphabetic,
                    children: [
                      Expanded(
                        child: Text(
                          l.rewardStreakTitle(s.streak),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: Dim.t3,
                            fontWeight: FontWeight.w800,
                            height: 1.4,
                            color: context.c.ink,
                          ),
                        ),
                      ),
                      const SizedBox(width: Dim.s2),
                      Text(
                        l.rewardDay7,
                        style: TextStyle(
                          fontSize: Dim.t1,
                          color: context.c.ink3,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: Dim.s3),
                  CheckInStrip(
                    cycle: s.cycle,
                    streak: s.streak,
                    checkedToday: s.checkedToday,
                    todayReward: s.todayReward,
                  ),
                  const SizedBox(height: Dim.s3),
                  AppButton(
                    label: s.checkedToday
                        ? l.rewardCheckedIn
                        : l.rewardCheckinGet(s.todayReward),
                    loading: _busy,
                    onTap: s.checkedToday || _busy ? null : _checkin,
                  ),
                ],
              ),
            ),
            const SizedBox(height: Dim.s5),
            SectionLabel(l.rewardEarnMore),
            const SizedBox(height: Dim.s3),
            ListGroup(
              children: [
                ListRowItem(
                  title: l.rewardWatchAd,
                  subtitle: l.rewardWatchAdHint(s.adDone, s.adTotal, s.adCoins),
                  leading: Icon(
                    Icons.smart_display_outlined,
                    size: 22,
                    color: context.c.ink2,
                  ),
                  trailingText: l.rewardGoWatch,
                  onTap: s.adDone >= s.adTotal || _busy ? null : _watchAd,
                ),
                ListRowItem(
                  title: l.rewardInvite,
                  subtitle: l.rewardInviteHint(s.inviteReward),
                  leading: Icon(
                    Icons.link_rounded,
                    size: 22,
                    color: context.c.ink2,
                  ),
                  trailingText: l.rewardInviteAction,
                  onTap: _invite,
                ),
                ListRowItem(
                  title: l.rewardPostMoment,
                  subtitle: l.rewardPostMomentHint(s.momentReward),
                  leading: Icon(
                    Icons.auto_awesome_outlined,
                    size: 22,
                    color: context.c.ink2,
                  ),
                  trailingText: l.rewardGoPost,
                  onTap: () => context.push(Routes.momentPost),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
