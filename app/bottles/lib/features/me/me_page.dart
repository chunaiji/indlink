import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../domain/models/user.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/coin.dart';
import '../../ui/widgets/headers.dart';

/// H1 我的。
///
/// 各入口显隐在小程序侧由 `/api/mine-functions` 远程下发（14 个开关）；
/// App 端 5 个 Tab 固定常驻，但这些二级入口仍建议保留远程开关，用于灰度与应急关闭。
///
/// 合规：设置页内必须有**账号删除**入口（App Store 5.1.1(v)）。
/// 类别：列表页（规范 §6）
/// 固定区：无（头图随列表滚）
/// 弹性区：无
/// 可滚动区：头图 + 上探的钱包卡 + 四宫格 + 分组列表
/// 键盘：无
class MePage extends ConsumerWidget {
  const MePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final me = ref.watch(authProvider).profile;
    final wallet = ref.watch(walletProvider).value;
    // 运营可在管理后台隐藏这几个入口。默认全开 —— 不配就是现在这个样子。
    final cfg = ref.watch(appConfigProvider);

    final walletHero = cfg.showWallet
        ? _WalletHero(
            coins: wallet?.coins ?? 0,
            showRecharge: cfg.showRecharge,
            showDetail: cfg.showWalletLog,
            onRecharge: () => context.push(Routes.recharge),
            onDetail: () => context.push(Routes.wallet),
            onRecords: () => context.push(Routes.payRecords),
          )
        : null;

    return Scaffold(
      body: ListView(
        padding: EdgeInsets.zero,
        children: [
          Stack(
            clipBehavior: Clip.none,
            children: [
              Column(
                children: [
                  // 原型 .wallet margin-top -26px → 钱包卡上探 39，头图底部多留出这段。
                  GradientHeader(
                    bottomPadding: cfg.showWallet ? Dim.s5 + 39 : Dim.s5,
                    topRow: Row(
                      children: [
                        AvatarRing(
                          avatar: me?.brief.avatar,
                          seed: me?.id ?? '',
                          gender: me?.brief.gender ?? Gender.secret,
                          size: 48,
                          online: true,
                        ),
                        const SizedBox(width: Dim.s3),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Text(
                                me?.nickname ?? '',
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  fontSize: Dim.t5,
                                  fontWeight: FontWeight.w800,
                                  color: Colors.white,
                                  letterSpacing: -0.2,
                                ),
                              ),
                              const SizedBox(height: 2),
                              Text(
                                l.meIdLine(
                                  me?.id ?? '',
                                  switch (me?.brief.gender) {
                                    Gender.female => l.commonFemale,
                                    Gender.male => l.commonMale,
                                    _ => l.commonSecret,
                                  },
                                  me?.brief.age ?? 0,
                                ),
                                style: TextStyle(
                                  fontSize: Dim.t1,
                                  color: Colors.white.withValues(alpha: .85),
                                ),
                              ),
                            ],
                          ),
                        ),
                        // 编辑资料是高频动作，不该埋进设置里 —— 与设置并列。
                        HeaderIconButton(
                          icon: Icons.edit_outlined,
                          onTap: () => context.push(Routes.profileEdit),
                        ),
                        const SizedBox(width: Dim.s1),
                        HeaderIconButton(
                          icon: Icons.settings_outlined,
                          onTap: () => context.push(Routes.settings),
                        ),
                      ],
                    ),
                    bottom: HeaderStats(
                      items: [
                        ('${me?.followingCount ?? 0}', l.meStatFollowing),
                        ('${me?.followerCount ?? 0}', l.meStatFollowers),
                        ('${me?.charm ?? 0}', l.discoverCharm),
                      ],
                    ),
                  ),
                  // 钱包卡的占位：真卡用 Positioned 压上去，占位只负责把下面的内容推到
                  // 正确位置（卡高减去上探的 39），不再靠猜卡高。
                  if (walletHero != null)
                    Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: Dim.gutter,
                      ),
                      child: Visibility(
                        visible: false,
                        maintainSize: true,
                        maintainAnimation: true,
                        maintainState: true,
                        child: walletHero,
                      ),
                    ),
                ],
              ),
              // 真卡：底边比占位高 39，正好压在头图预留的那段上。
              // 之前按 bottom: 0 定位，上探量 = 卡高 - 39，卡一高就把关注 / 粉丝 / 魅力那行盖掉了。
              if (walletHero != null)
                Positioned(
                  left: Dim.gutter,
                  right: Dim.gutter,
                  bottom: 39,
                  child: KeyedSubtree(
                    key: const ValueKey('wallet-hero'),
                    child: walletHero,
                  ),
                ),
            ],
          ),
          // 占位比真卡低 39，这段就是卡与四宫格之间的间距；没有钱包卡时补回 s6。
          if (walletHero == null) const SizedBox(height: Dim.s6),
          // 四宫格快捷入口：图标浮起卡片，与下方列表拉开层级。
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: Dim.gutter),
            child: QuadGrid(
              roomy: true,
              items: [
                QuadItem(
                  icon: Icons.event_available_outlined,
                  label: l.meCheckin,
                  onTap: () => context.push(Routes.rewards),
                ),
                // 道具 ≠ 充值：充值买金币，这里用金币买道具进背包。
                if (cfg.showItems)
                  QuadItem(
                    icon: Icons.card_giftcard_outlined,
                    label: l.meItems,
                    onTap: () => context.push(Routes.items),
                  ),
                QuadItem(
                  icon: Icons.smart_display_outlined,
                  label: l.meWatchVideo,
                  onTap: () => context.push(Routes.rewards),
                ),
                QuadItem(
                  icon: Icons.favorite_border_rounded,
                  label: l.meRelations,
                  onTap: () => context.push(Routes.relations),
                ),
              ],
            ),
          ),
          const SizedBox(height: Dim.s6),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: Dim.gutter),
            child: ListGroup(
              children: [
                // 钱包流水已收进钱包 Hero 卡的「金币明细」，列表不再重复。
                ListRowItem(
                  title: l.meMyBottles,
                  roomy: true,
                  trailingText: l.meBottlesCount(me?.bottleCount ?? 0),
                  onTap: () => context.push(Routes.myBottles),
                ),
                ListRowItem(
                  title: l.notifyTitle,
                  roomy: true,
                  onTap: () => context.push(Routes.notifications),
                ),
                if (cfg.showBlocklist)
                  ListRowItem(
                    title: l.safetyBlocklist,
                    roomy: true,
                    onTap: () => context.push(Routes.blocklist),
                  ),
                ListRowItem(
                  title: l.meSettings,
                  roomy: true,
                  onTap: () => context.push(Routes.settings),
                ),
              ],
            ),
          ),
          const SizedBox(height: Dim.s6),
          Center(
            child: Text(
              'DRIFT',
              style: TextStyle(
                fontSize: Dim.t1,
                fontWeight: FontWeight.w800,
                color: c.ink3.withValues(alpha: .5),
                letterSpacing: 4,
              ),
            ),
          ),
          const SizedBox(height: Dim.s6),
        ],
      ),
    );
  }
}

/// 头图内的 stats：关注 / 粉丝 / 魅力。关注/粉丝由服务端 Relation 聚合下发，魅力取 charm。
class _WalletHero extends StatelessWidget {
  const _WalletHero({
    required this.coins,
    required this.showRecharge,
    required this.showDetail,
    required this.onRecharge,
    required this.onDetail,
    required this.onRecords,
  });

  final int coins;

  final bool showRecharge;
  final bool showDetail;
  final VoidCallback onRecharge;
  final VoidCallback onDetail;
  final VoidCallback onRecords;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: c.surface,
        borderRadius: Dim.brLargeCard,
        border: Border.all(color: c.line2),
        boxShadow: Shadows.floating(c),
      ),
      child: Stack(
        children: [
          // 右上金色光斑（.wallet::before）
          Positioned(
            right: -45,
            top: -45,
            child: Container(
              width: 164,
              height: 164,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                gradient: RadialGradient(
                  colors: [
                    c.coin.withValues(alpha: .22),
                    c.coin.withValues(alpha: 0),
                  ],
                  stops: const [0, .68],
                ),
              ),
            ),
          ),
          // 顶部金线（.wallet::after）
          Positioned(
            left: 0,
            right: 0,
            top: 0,
            child: Container(
              height: 4,
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  colors: [
                    c.coin.withValues(alpha: 0),
                    c.coin.withValues(alpha: .9),
                    c.coin.withValues(alpha: 0),
                  ],
                ),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(Dim.s4),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    const CoinIcon(size: 56),
                    const SizedBox(width: Dim.s3),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            l.meCoinBalance,
                            style: TextStyle(
                              fontSize: Dim.t1,
                              fontWeight: FontWeight.w700,
                              color: c.ink3,
                              letterSpacing: .7,
                              height: 1.4,
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(
                            '$coins',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 39,
                              fontWeight: FontWeight.w800,
                              color: c.ink,
                              height: 1,
                              letterSpacing: -0.4,
                              fontFeatures: const [
                                FontFeature.tabularFigures(),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                    if (showRecharge) ...[
                      const SizedBox(width: Dim.s2),
                      MiniButton(
                        label: l.meRecharge,
                        kind: BtnKind.pay,
                        height: 48,
                        onTap: onRecharge,
                      ),
                    ],
                  ],
                ),
                if (showDetail) ...[
                  const SizedBox(height: Dim.s3),
                  Divider(height: 1, color: c.line2),
                  const SizedBox(height: Dim.s3),
                  // .wallet .wlinks：t2/600 ink2，链接之间 s5。
                  Wrap(
                    spacing: Dim.s5,
                    runSpacing: Dim.s1,
                    children: [
                      _WalletLink(label: l.meWalletTxns, onTap: onDetail),
                      _WalletLink(label: l.payRecordsTitle, onTap: onRecords),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _WalletLink extends StatelessWidget {
  const _WalletLink({required this.label, required this.onTap});

  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            label,
            style: TextStyle(
              fontSize: Dim.t2,
              fontWeight: FontWeight.w600,
              color: c.ink2,
              height: 1.4,
            ),
          ),
          Icon(Icons.chevron_right_rounded, size: 16, color: c.ink3),
        ],
      ),
    );
  }
}
