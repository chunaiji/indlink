import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/catalog.dart';
import '../../core/design/tokens.dart';
import '../../core/network/api_exception.dart';
import '../../core/platform/location.dart';
import '../../core/providers.dart';
import '../../domain/models/user.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/flows/purchase_flows.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/coin.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';
import '../chat/chat_controller.dart';
import '../location/location_controller.dart';
import 'discover_controller.dart';
import 'swipe_deck.dart';

/// 开聊：扣 `price_chat`（后台「价格」分组配，App 从 /app-config 的 pricing 拿来预判）。
///
/// C1 和 C3 共用这一条链路，余额不足统一走 Z4 模态而不是通用 toast。
Future<void> startChatWith(
  BuildContext context,
  WidgetRef ref,
  UserProfile user,
) async {
  final price = ref.read(appConfigProvider).chatStartPrice;
  final l = L.of(context);
  final balance = ref.read(walletProvider).value?.coins ?? 0;

  if (balance < price) {
    await showInsufficientModal(
      context,
      ref,
      itemName: l.discoverSayHi,
      need: price,
      have: balance,
    );
    return;
  }

  try {
    final conv = await ref.read(chatRepoProvider).startWith(user.id);
    await ref.read(walletProvider.notifier).refresh();
    ref.invalidate(conversationsProvider);
    if (context.mounted) context.push(Routes.chatRoom(conv.id));
  } on ApiException catch (e) {
    if (!context.mounted) return;
    if (e.isInsufficientBalance) {
      await showInsufficientModal(
        context,
        ref,
        itemName: l.discoverSayHi,
        need: price,
        have: balance,
      );
    } else {
      showToast(context, e.message, error: true);
    }
  }
}

/// C1 滑卡发现。
///
/// 语义提醒：现有后端 `match` 模块是「瓶子推荐打分 + MatchLog 行为记录」，
/// 这里要的「按语言 / 兴趣 / 地理找人」是**新的候选集来源**（用户池而非瓶子池），
/// 依赖 `User` 的 language / interests / 经纬度三个新字段。
/// 类别：场景页（规范 §6）
/// 固定区：头图（标题 + 金币 + 筛选）、压在头图下沿的 chips 行
/// 弹性区：卡片堆（原型 .deck margin s6 s3 s3；宽 ≥ 600 限宽 480）
/// 可滚动区：无
/// 键盘：无
class DiscoverPage extends ConsumerStatefulWidget {
  const DiscoverPage({super.key});

  @override
  ConsumerState<DiscoverPage> createState() => _DiscoverPageState();
}

class _DiscoverPageState extends ConsumerState<DiscoverPage> {
  final _deckKey = GlobalKey<SwipeDeckState>();
  bool _rewinding = false;

  /// 点爱心的动效：计数变一次卡上飘一颗心；只在被点的那张卡上播。
  int _burstTick = 0;
  String? _burstUserId;

  /// 点爱心 = 喜欢，不切人；只有左右滑才切。
  Future<void> _like(UserProfile user) async {
    setState(() {
      _burstTick++;
      _burstUserId = user.id;
    });
    await ref.read(deckProvider.notifier).likeTop();
    // 服务端首次喜欢会给对方 +1 魅力：资料页缓存失效，再点进去看到的是新值。
    ref.invalidate(userCardProvider(user.id));
  }

  /// 撤回：扣币把刚左滑掉的人捞回牌堆顶。
  ///
  /// 扣费动作必须二次确认——滑卡是连续手势，手抖点到一次就扣钱会被当成坑钱。
  /// 价格从服务端拿，弹层上写多少就扣多少。
  Future<void> _rewind() async {
    if (_rewinding) return;
    final l = L.of(context);
    final price = ref.read(appConfigProvider).rewindPrice;
    final balance = ref.read(walletProvider).value?.coins ?? 0;

    if (balance < price) {
      await showInsufficientModal(
        context,
        ref,
        itemName: l.discoverRewind,
        need: price,
        have: balance,
      );
      return;
    }

    final ok = await showAppModal<bool>(
      context,
      builder: (ctx) => ModalCard(
        iconWidget: const Text('↩️', style: TextStyle(fontSize: 40)),
        title: l.discoverRewindTitle,
        body: Text(l.discoverRewindBody(price)),
        actions: [
          AppButton(
            label: l.discoverRewindConfirm(price),
            kind: BtnKind.pay,
            onTap: () => Navigator.of(ctx).pop(true),
          ),
          AppButton(
            label: l.commonCancel,
            kind: BtnKind.ghost,
            onTap: () => Navigator.of(ctx).pop(false),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;

    setState(() => _rewinding = true);
    try {
      final user = await ref.read(deckProvider.notifier).rewind();
      await ref.read(walletProvider.notifier).refresh();
      if (mounted) showToast(context, l.discoverRewindDone(user.nickname));
    } on ApiException catch (e) {
      if (!mounted) return;
      if (e.isInsufficientBalance) {
        await showInsufficientModal(
          context,
          ref,
          itemName: l.discoverRewind,
          need: price,
          have: balance,
        );
      } else {
        showToast(context, e.message, error: true);
      }
    } finally {
      if (mounted) setState(() => _rewinding = false);
    }
  }

  /// 左滑扣费跳过的二次确认（付费功能，后台默认关）。
  ///
  /// 滑卡是连续手势，手抖划一下就扣钱会被当成坑钱——所以每次左滑都要确认。
  /// 余额不足直接拦下弹充值，返回 false 不飞卡、不扣费。
  Future<bool> _confirmPaidSkip(int price) async {
    final l = L.of(context);
    final balance = ref.read(walletProvider).value?.coins ?? 0;
    if (balance < price) {
      await showInsufficientModal(
        context,
        ref,
        itemName: l.discoverPass,
        need: price,
        have: balance,
      );
      return false;
    }
    final ok = await showAppModal<bool>(
      context,
      builder: (ctx) => ModalCard(
        iconWidget: const Text('👋', style: TextStyle(fontSize: 40)),
        title: l.discoverPassChargeTitle,
        body: Text(l.discoverPassChargeBody(price)),
        actions: [
          AppButton(
            label: l.discoverPassChargeConfirm(price),
            kind: BtnKind.pay,
            onTap: () => Navigator.of(ctx).pop(true),
          ),
          AppButton(
            label: l.commonCancel,
            kind: BtnKind.ghost,
            onTap: () => Navigator.of(ctx).pop(false),
          ),
        ],
      ),
    );
    return ok == true;
  }

  /// 二次确认通过后真正扣费跳过：移除卡片 + 服务端扣费 + 刷新余额。
  ///
  /// 扣费失败（并发下余额被花掉 / 网络错）时 paidSkipTop 已把人放回牌堆顶，
  /// 这里按余额不足弹充值、其余弹 toast。
  Future<void> _paidSkipConsume() async {
    final l = L.of(context);
    final price = ref.read(appConfigProvider).skipPrice;
    final balance = ref.read(walletProvider).value?.coins ?? 0;
    try {
      await ref.read(deckProvider.notifier).paidSkipTop();
      await ref.read(walletProvider.notifier).refresh();
    } on ApiException catch (e) {
      if (!mounted) return;
      if (e.isInsufficientBalance) {
        await showInsufficientModal(
          context,
          ref,
          itemName: l.discoverPass,
          need: price,
          have: balance,
        );
      } else {
        showToast(context, e.message, error: true);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final deck = ref.watch(deckProvider);
    final tab = ref.watch(discoverTabProvider);
    final wallet = ref.watch(walletProvider).value;
    final me = ref.watch(authProvider).profile;
    // 定价来自启动配置（/app-config，缓存在本地），进页面就有数，第一次点撤回不会写 0 币。
    final pricing = ref.watch(appConfigProvider);
    final rewindPrice = pricing.rewindPrice;
    // 左滑扣币开关（付费功能，后台默认关）+ 有效价格才生效。
    final skipCharge = pricing.skipChargeEnabled && pricing.skipPrice > 0;
    final skipPrice = pricing.skipPrice;
    final likedIds = ref.watch(likedIdsProvider);

    return Scaffold(
      body: Column(
        children: [
          GradientHeader(
            slim: true,
            topRow: Row(
              children: [
                AvatarRing(
                  avatar: me?.brief.avatar,
                  seed: me?.id ?? '',
                  size: 34,
                  online: true,
                  onTap: () => context.go(Routes.me),
                ),
                const SizedBox(width: Dim.s3),
                // 原型 .hdr .ttl：t5/800。可收缩：窄屏 + 大字体下金币和筛选不能被顶出去。
                Expanded(
                  child: Text(
                    l.discoverTitle,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: Dim.t5,
                      fontWeight: FontWeight.w800,
                      color: Colors.white,
                      letterSpacing: -0.2,
                      height: 1.25,
                    ),
                  ),
                ),
                const SizedBox(width: Dim.s2),
                CoinChip(
                  coins: wallet?.coins ?? 0,
                  onTap: () => context.push(Routes.recharge),
                ),
                HeaderIconButton(
                  icon: Icons.tune_rounded,
                  onTap: () => context.push(Routes.discoverFilter),
                ),
              ],
            ),
          ),
          // 原型 .chs.raise 其实没有负边距：胶囊行在头图下方留 s2，不压头图。
          ChipBar(
            padding: const EdgeInsets.fromLTRB(
              Dim.gutter,
              Dim.s2,
              Dim.gutter,
              Dim.s1,
            ),
            children: [
              for (final (key, label) in [
                ('recommend', l.discoverTabRecommend),
                ('nearby', l.discoverTabNearby),
                ('new', l.discoverTabNew),
              ])
                PillChip(
                  label: label,
                  tone: key == 'nearby' ? ChipTone.pay : ChipTone.plain,
                  selected: tab == key,
                  onTap: () => ref.read(discoverTabProvider.notifier).set(key),
                ),
              for (final lang in Catalog.languages.take(2))
                PillChip(
                  label: lang,
                  tone: ChipTone.aqua,
                  selected: tab == lang,
                  onTap: () => ref.read(discoverTabProvider.notifier).set(lang),
                ),
            ],
          ),
          Expanded(
            child: AsyncView(
              value: deck,
              onRetry: () => ref.invalidate(deckProvider),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const _DeckSkeleton(),
              data: (users) {
                // Z6：「附近」被永久拒绝定位时，系统框唤不起来，只能送去设置；
                // 同时给「先逛逛推荐」这条退路，别把人堵死在一个入口。
                final perm = ref.watch(locationControllerProvider).permission;
                if (tab == 'nearby' &&
                    perm == LocationPermissionState.deniedForever) {
                  return EmptyState(
                    icon: Icons.location_off_outlined,
                    title: l.locationDeniedTitle,
                    body: l.locationDeniedBody,
                    ctaLabel: l.locationOpenSettings,
                    onCta: () => ref
                        .read(locationControllerProvider.notifier)
                        .openSettings(),
                    altLabel: l.locationBrowseInstead,
                    onAlt: () =>
                        ref.read(discoverTabProvider.notifier).set('recommend'),
                  );
                }
                if (users.isEmpty) {
                  return EmptyState(
                    icon: Icons.explore_outlined,
                    title: l.discoverNoMoreCards,
                    body: l.stateEmptyOceanBody,
                    ctaLabel: l.stateEmptyChatsCta,
                    onCta: () => context.go(Routes.ocean),
                  );
                }
                return Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      maxWidth: Breakpoints.contentMaxWidth,
                    ),
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(
                        Dim.s3,
                        Dim.s3,
                        Dim.s3,
                        Dim.s4,
                      ),
                      child: SwipeDeck(
                        key: _deckKey,
                        itemCount: users.length,
                        // 开了左滑扣币才拦截：卡片停住等二次确认，确认通过再飞出扣费。
                        confirmPass: skipCharge
                            ? () => _confirmPaidSkip(skipPrice)
                            : null,
                        onSwipe: (liked) {
                          if (!liked && skipCharge) {
                            _paidSkipConsume();
                          } else {
                            ref
                                .read(deckProvider.notifier)
                                .consumeTop(liked: liked);
                          }
                        },
                        builder: (context, i, progress) => _UserCard(
                          user: users[i],
                          progress: progress,
                          interactive: i == 0,
                          rewindPrice: rewindPrice,
                          rewinding: _rewinding,
                          hiPrice: pricing.chatStartPrice,
                          liked: likedIds.contains(users[i].id),
                          burstTick: _burstUserId == users[i].id
                              ? _burstTick
                              : 0,
                          onLike: () => _like(users[i]),
                          onRewind: _rewind,
                          onSayHi: () => startChatWith(context, ref, users[i]),
                          onOpenProfile: () =>
                              context.push(Routes.userProfile(users[i].id)),
                        ),
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _UserCard extends StatelessWidget {
  const _UserCard({
    required this.user,
    required this.progress,
    required this.interactive,
    required this.rewindPrice,
    required this.rewinding,
    required this.hiPrice,
    required this.liked,
    required this.burstTick,
    required this.onLike,
    required this.onRewind,
    required this.onSayHi,
    required this.onOpenProfile,
  });

  final UserProfile user;
  final double progress;
  final bool interactive;

  /// null = 价格还没拉到，此时不给点，避免弹层上写 0 币。
  final int? rewindPrice;
  final bool rewinding;

  /// 打招呼（开聊）单价，来自后台 price_chat。
  final int hiPrice;

  /// 这张卡是否已点过爱心（心保持点亮）。
  final bool liked;

  /// 爱心动效触发计数：变一次播一次；0 = 不播。
  final int burstTick;
  final VoidCallback onLike;
  final VoidCallback onRewind;
  final VoidCallback onSayHi;
  final VoidCallback onOpenProfile;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: c.surface,
        borderRadius: Dim.brModal,
        border: Border.all(color: c.line),
        boxShadow: Shadows.floating(c),
      ),
      child: Column(
        children: [
          Expanded(
            child: Stack(
              children: [
                // C1 → C3 用 Hero 共享这块，卡片「撑开」成主页。
                Hero(
                  tag: 'user-photo-${user.id}',
                  child: UserPhoto(user: user.brief),
                ),
                // 点爱心时卡中央飘一颗心（0.9s），不拦手势。
                Positioned.fill(
                  child: IgnorePointer(child: _HeartBurst(tick: burstTick)),
                ),
                if (user.brief.presumedOnline)
                  Positioned(
                    left: Dim.s3,
                    top: Dim.s3,
                    child: _OnlinePill(label: l.commonOnline),
                  ),
                Positioned(
                  left: Dim.s3,
                  top: 68,
                  child: SwipeStamp(
                    progress: progress,
                    likeLabel: l.discoverLike,
                    passLabel: l.discoverPass,
                  ),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(Dim.s4),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // 原型 .info .nm：t6/800；右侧「主页 ›」t2/700 ink3。
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        '${user.nickname}, ${user.brief.age ?? 0}',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: Dim.t6,
                          fontWeight: FontWeight.w800,
                          color: c.ink,
                          letterSpacing: -0.24,
                          height: 1.25,
                        ),
                      ),
                    ),
                    const SizedBox(width: Dim.s2),
                    GestureDetector(
                      onTap: interactive ? onOpenProfile : null,
                      child: Row(
                        children: [
                          Text(
                            l.discoverViewProfile,
                            style: TextStyle(
                              fontSize: Dim.t2,
                              fontWeight: FontWeight.w700,
                              color: c.ink3,
                            ),
                          ),
                          Icon(
                            Icons.chevron_right_rounded,
                            size: 16,
                            color: c.ink3,
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: Dim.s2),
                Wrap(
                  spacing: Dim.s1,
                  runSpacing: Dim.s1,
                  children: [
                    if (user.distanceKm != null)
                      TagChip(
                        label: '${user.distanceKm!.toStringAsFixed(1)} km',
                        tone: TagTone.pay,
                        icon: Icons.place_outlined,
                      ),
                    for (final lang in user.languages.take(1))
                      TagChip(label: lang),
                    for (final key in user.interests.take(2))
                      TagChip(
                        label: Catalog.interest(context, key),
                        tone: TagTone.purple,
                      ),
                  ],
                ),
                if (user.bio != null) ...[
                  const SizedBox(height: Dim.s2),
                  Text(
                    user.bio!,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: Dim.t2,
                      height: 1.55,
                      color: c.ink2,
                    ),
                  ),
                ],
                const SizedBox(height: Dim.s4),
                // 动作行：撤回（后台开了价才出现）、打招呼 pill（粉 = 要花币）、♥ 点亮圆。
                // 没有 ✕：跳过只靠左滑（用户决定），按钮少一枚，打招呼更宽。
                Row(
                  children: [
                    // 花金币的是「把划走的人找回来」：价格角标挂在撤回键上。
                    if (rewindPrice != null) ...[
                      _PricedAction(
                        key: const ValueKey('rewind-action'),
                        icon: Icons.replay_rounded,
                        price: rewindPrice! > 0 ? rewindPrice : null,
                        onTap: interactive && !rewinding ? onRewind : null,
                      ),
                      const SizedBox(width: Dim.s2),
                    ],
                    Expanded(
                      child: HiButton(
                        label: l.discoverSayHi,
                        coins: hiPrice,
                        onTap: interactive ? onSayHi : null,
                      ),
                    ),
                    const SizedBox(width: Dim.s2),
                    ActionCircleButton(
                      key: const ValueKey('like-action'),
                      icon: liked
                          ? Icons.favorite_rounded
                          : Icons.favorite_border_rounded,
                      liked: true,
                      onTap: interactive ? onLike : null,
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// 50 圆动作键 + 右上金色价格角标（原型 .ab 的付费变体）。
class _PricedAction extends StatelessWidget {
  const _PricedAction({
    super.key,
    required this.icon,
    required this.price,
    this.onTap,
  });

  final IconData icon;

  /// null = 免费，只是普通圆键。
  final int? price;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final base = ActionCircleButton(icon: icon, onTap: onTap);
    if (price == null) return base;
    final c = context.c;
    return Stack(
      clipBehavior: Clip.none,
      children: [
        base,
        Positioned(
          right: -6,
          top: -6,
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
            decoration: BoxDecoration(
              color: c.gold,
              borderRadius: Dim.brPill,
              border: Border.all(color: c.surface, width: 1.5),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const CoinIcon(size: 10, onDark: true),
                const SizedBox(width: 2),
                Text(
                  '$price',
                  style: const TextStyle(
                    fontSize: Dim.t0,
                    fontWeight: FontWeight.w800,
                    color: Colors.white,
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

class _OnlinePill extends StatelessWidget {
  const _OnlinePill({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: Dim.s2, vertical: 3),
      decoration: BoxDecoration(
        color: Colors.black.withValues(alpha: .35),
        borderRadius: Dim.brPill,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 6,
            height: 6,
            decoration: const BoxDecoration(
              shape: BoxShape.circle,
              color: Color(0xFF35C77E),
            ),
          ),
          const SizedBox(width: 4),
          Text(
            label,
            style: const TextStyle(
              fontSize: Dim.t0,
              fontWeight: FontWeight.w700,
              color: Colors.white,
            ),
          ),
        ],
      ),
    );
  }
}

class _DeckSkeleton extends StatelessWidget {
  const _DeckSkeleton();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(Dim.s3, Dim.s3, Dim.s3, Dim.s4),
      child: Column(
        children: [
          const Expanded(
            child: Skeleton(height: double.infinity, radius: Dim.r5),
          ),
          const SizedBox(height: Dim.s3),
          Row(
            children: [
              const Skeleton.circle(size: 44),
              const SizedBox(width: Dim.s2),
              const Expanded(child: Skeleton(height: 44, radius: Dim.rPill)),
              const SizedBox(width: Dim.s2),
              const Skeleton.circle(size: 44),
            ],
          ),
        ],
      ),
    );
  }
}

/// 点爱心的动效：一颗心从中心放大到约 1/3 卡宽再淡出。[tick] 变一次播一次。
class _HeartBurst extends StatefulWidget {
  const _HeartBurst({required this.tick});

  final int tick;

  @override
  State<_HeartBurst> createState() => _HeartBurstState();
}

class _HeartBurstState extends State<_HeartBurst>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 900),
  );
  late final Animation<double> _scale = TweenSequence<double>([
    TweenSequenceItem(tween: Tween(begin: .3, end: 1.15), weight: 35),
    TweenSequenceItem(tween: Tween(begin: 1.15, end: 1.0), weight: 15),
    TweenSequenceItem(tween: ConstantTween(1.0), weight: 50),
  ]).animate(CurvedAnimation(parent: _ctrl, curve: Curves.easeOut));
  late final Animation<double> _fade = TweenSequence<double>([
    TweenSequenceItem(tween: Tween(begin: 0.0, end: 1.0), weight: 20),
    TweenSequenceItem(tween: ConstantTween(1.0), weight: 40),
    TweenSequenceItem(tween: Tween(begin: 1.0, end: 0.0), weight: 40),
  ]).animate(_ctrl);

  @override
  void didUpdateWidget(covariant _HeartBurst old) {
    super.didUpdateWidget(old);
    if (widget.tick != old.tick && widget.tick > 0) _ctrl.forward(from: 0);
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return AnimatedBuilder(
      animation: _ctrl,
      builder: (context, _) {
        if (_ctrl.isDismissed) return const SizedBox.shrink();
        return Center(
          child: Opacity(
            opacity: _fade.value,
            child: Transform.scale(
              scale: _scale.value,
              child: Icon(
                Icons.favorite_rounded,
                size: 110,
                color: c.brand,
                shadows: [
                  Shadow(color: c.brand.withValues(alpha: .4), blurRadius: 24),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
