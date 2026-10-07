import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../domain/models/chat.dart';
import '../widgets/gift_icon.dart';
import '../../l10n/app_localizations.dart';
import '../widgets/buttons.dart';
import '../widgets/coin.dart';
import '../widgets/overlays.dart';
import 'purchase_flows.dart';

final giftsProvider = FutureProvider<List<GiftItem>>((ref) {
  return ref.watch(walletRepoProvider).gifts();
});

/// 背包:每种礼物的持有数量。聊天送礼是库存制,送出一次消耗一件。
/// 与 [giftsProvider] 同处定义,道具页 / 礼物弹框共用,避免 flows 反依赖具体页面。
final myItemsProvider = FutureProvider<Map<String, int>>((ref) {
  // 背包是每个人自己的：换号后要重拉，否则礼物面板上的「×1」是上一个账号的（真机反馈）。
  ref.watch(authProvider.select((s) => s.profile?.id));
  return ref.watch(walletRepoProvider).myItems();
});

/// H4 礼物面板。
///
/// 送礼三处（聊天 / 动态 / 用户主页）走统一链路：
/// 扣币事务 + `ItemOrder(target)` + `User.Charm` 累加 + 通知。
/// 连送 ×10 要**合并成一条通知**，否则刷屏。
///
/// 返回选中的礼物与数量，扣费由调用方按各自场景的接口完成。
Future<(GiftItem, int)?> showGiftSheet(BuildContext context, WidgetRef ref) {
  // 每次打开都重新数一遍背包:上次送礼后的扣减、别处买的道具都要在这里立刻看到。
  ref.invalidate(myItemsProvider);
  return showAppSheet<(GiftItem, int)>(
    context,
    builder: (ctx) => _GiftSheet(ref: ref),
  );
}

/// 送礼特效：礼物图标飞向对方头像。
///
/// 连送 ×10 **合并成一次特效**（角标显示数量），否则连播 10 遍会卡住聊天。
/// 走根 Overlay 而不是页面内 Stack——这样坐标直接用 global，不必跟着布局层级换算。
void playGiftFlight(
  BuildContext context, {
  required GiftItem gift,
  required int qty,
  required Offset target,
}) {
  final overlay = Overlay.maybeOf(context);
  if (overlay == null) return;
  if (MediaQuery.disableAnimationsOf(context)) return;

  final size = MediaQuery.sizeOf(context);
  final from = Offset(size.width / 2, size.height - 90);

  late final OverlayEntry entry;
  entry = OverlayEntry(
    builder: (_) => _GiftFlight(
      gift: gift,
      qty: qty,
      from: from,
      to: target,
      onDone: entry.remove,
    ),
  );
  overlay.insert(entry);
}

class _GiftFlight extends StatefulWidget {
  const _GiftFlight({
    required this.gift,
    required this.qty,
    required this.from,
    required this.to,
    required this.onDone,
  });

  final GiftItem gift;
  final int qty;
  final Offset from;
  final Offset to;
  final VoidCallback onDone;

  @override
  State<_GiftFlight> createState() => _GiftFlightState();
}

class _GiftFlightState extends State<_GiftFlight>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 700),
  );

  @override
  void initState() {
    super.initState();
    _ctrl.forward().then((_) => widget.onDone());
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // 大额礼物叠一层全屏微光，小额不加——否则每次送礼都像中奖。
    final grand = widget.gift.coins * widget.qty >= 100;

    return IgnorePointer(
      child: AnimatedBuilder(
        animation: _ctrl,
        builder: (context, _) {
          final t = Curves.easeInOutCubic.transform(_ctrl.value);
          // 控制点偏左上，飞行轨迹带一点弧而不是直线冲过去
          final control = Offset(
            widget.from.dx - 60,
            widget.to.dy + (widget.from.dy - widget.to.dy) * .25,
          );
          final u = 1 - t;
          final pos = Offset(
            u * u * widget.from.dx +
                2 * u * t * control.dx +
                t * t * widget.to.dx,
            u * u * widget.from.dy +
                2 * u * t * control.dy +
                t * t * widget.to.dy,
          );
          final scale = 1.0 + 0.5 * (1 - (t - .5).abs() * 2) - t * .55;

          return Stack(
            children: [
              if (grand)
                Positioned.fill(
                  child: ColoredBox(
                    color: const Color(0xFFFFC53D)
                        .withValues(alpha: (1 - t) * .12),
                  ),
                ),
              Positioned(
                left: pos.dx - 22,
                top: pos.dy - 22,
                child: Opacity(
                  opacity: t > .88 ? (1 - t) / .12 : 1,
                  child: Transform.scale(
                    scale: scale.clamp(.4, 1.6),
                    child: Stack(
                      clipBehavior: Clip.none,
                      children: [
                        GiftIcon(gift: widget.gift, size: 40),
                        if (widget.qty > 1)
                          Positioned(
                            right: -10,
                            top: -4,
                            child: Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 5,
                                vertical: 1,
                              ),
                              decoration: BoxDecoration(
                                color: context.c.brand,
                                borderRadius: Dim.brPill,
                              ),
                              child: Text(
                                '×${widget.qty}',
                                style: const TextStyle(
                                  fontSize: 11,
                                  fontWeight: FontWeight.w800,
                                  color: Colors.white,
                                ),
                              ),
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _GiftSheet extends ConsumerStatefulWidget {
  const _GiftSheet({required this.ref});

  final WidgetRef ref;

  @override
  ConsumerState<_GiftSheet> createState() => _GiftSheetState();
}

class _GiftSheetState extends ConsumerState<_GiftSheet> {
  GiftItem? _selected;
  int _qty = 1;

  static const _quantities = [1, 10, 99];

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final gifts = ref.watch(giftsProvider);
    final owned = ref.watch(myItemsProvider).value ?? const <String, int>{};
    final balance = ref.watch(walletProvider).value?.coins ?? 0;
    // 默认选第一件。必须在算抵扣之前定下来：之前是在网格 builder 里顺手赋值的，
    // 同一帧里上面的价格已经按「没选中」算完了——真机上手里有 ×1 也照样标全价。
    final loaded = gifts.value;
    if (_selected == null && loaded != null && loaded.isNotEmpty) {
      _selected = loaded.first;
    }
    // 背包里有几件就抵几件，差额才算钱。
    final have = _selected == null ? 0 : (owned[_selected!.id] ?? 0);
    final fromBag = have < _qty ? have : _qty;
    final pay = (_selected?.coins ?? 0) * (_qty - fromBag);

    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Text(
              l.giftPanelTitle,
              style: TextStyle(
                fontSize: Dim.t3,
                fontWeight: FontWeight.w800,
                color: c.ink,
              ),
            ),
            const Spacer(),
            CoinChip(coins: balance, onHeader: false),
          ],
        ),
        const SizedBox(height: Dim.s3),
        gifts.when(
          loading: () => const SizedBox(
            height: 88,
            child: Center(child: CircularProgressIndicator()),
          ),
          error: (_, _) => SizedBox(
            height: 88,
            child: Center(
              child: Text(
                l.stateLoadFailed,
                style: TextStyle(fontSize: Dim.t2, color: c.ink3),
              ),
            ),
          ),
          data: (items) {
            return GridView.count(
              crossAxisCount: 4,
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              mainAxisSpacing: Dim.s2,
              crossAxisSpacing: Dim.s2,
              // 原型 .gitem ≈ 82×67：接近 1.2 的横向格，不是竖长条。
              childAspectRatio: 1.2,
              children: [
                for (final g in items)
                  _GiftTile(
                    gift: g,
                    owned: owned[g.id] ?? 0,
                    selected: _selected?.id == g.id,
                    onTap: () => setState(() => _selected = g),
                  ),
              ],
            );
          },
        ),
        const SizedBox(height: Dim.s3),
        Row(
          children: [
            _QtyPicker(
              qty: _qty,
              options: _quantities,
              onChanged: (v) => setState(() => _qty = v),
            ),
            const SizedBox(width: Dim.s2),
            Expanded(
              child: AppButton(
                label: l.giftSend,
                kind: BtnKind.pay,
                // 背包先抵扣，只对差额标价；全部能从背包出就不显示金币
                // （服务端 SendGift 同一条规则，之前手里有 ×1 也照样标 10 币）。
                coins: pay > 0 ? pay : null,
                onTap: _selected == null
                    ? null
                    : () async {
                        final gift = _selected!;
                        if (balance < pay) {
                          Navigator.of(context).pop();
                          await showInsufficientModal(
                            context,
                            widget.ref,
                            itemName: gift.name,
                            need: pay,
                            have: balance,
                          );
                          return;
                        }
                        Navigator.of(context).pop((gift, _qty));
                      },
              ),
            ),
          ],
        ),
        if (fromBag > 0) ...[
          const SizedBox(height: Dim.s2),
          Text(
            l.giftFromBag(fromBag),
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: Dim.t1, color: c.ink3, height: 1.4),
          ),
        ],
      ],
    );
  }
}

class _GiftTile extends StatelessWidget {
  const _GiftTile({
    required this.gift,
    required this.owned,
    required this.selected,
    required this.onTap,
  });

  final GiftItem gift;
  final int owned;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      onTap: onTap,
      child: Container(
        decoration: BoxDecoration(
          color: selected ? c.brand.withValues(alpha: .07) : Colors.transparent,
          borderRadius: Dim.brCard,
          border: Border.all(
            color: selected ? c.brand : Colors.transparent,
            width: 1.5,
          ),
        ),
        // 图标 + 价格整体按格高缩放：4 列格只有 ~67 高，Ahem / 大字体下差 1px 就溢出。
        child: Center(
          child: FittedBox(
            fit: BoxFit.scaleDown,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Stack(
                  clipBehavior: Clip.none,
                  children: [
                    GiftIcon(gift: gift, size: 30),
                    // 持有数角标:库存制下"手里有几个能送"是选礼物时的关键信息。
                    if (owned > 0)
                      Positioned(
                        right: -10,
                        top: -6,
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 5,
                            vertical: 1,
                          ),
                          // 原型 .gitem em：ink 底、surface 字、t0/800。
                          decoration: BoxDecoration(
                            color: c.ink,
                            borderRadius: Dim.brPill,
                          ),
                          child: Text(
                            '×$owned',
                            style: TextStyle(
                              fontSize: Dim.t0,
                              fontWeight: FontWeight.w800,
                              color: c.surface,
                              height: 1.4,
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
                const SizedBox(height: 2),
                Text(
                  '${gift.coins}',
                  style: TextStyle(
                    fontSize: Dim.t1,
                    fontWeight: FontWeight.w700,
                    color: c.gold,
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

/// 数量选择。×1 / ×10 / ×99 连送是客单价杠杆。
class _QtyPicker extends StatelessWidget {
  const _QtyPicker({
    required this.qty,
    required this.options,
    required this.onChanged,
  });

  final int qty;
  final List<int> options;
  final ValueChanged<int> onChanged;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return PopupMenuButton<int>(
      initialValue: qty,
      onSelected: onChanged,
      position: PopupMenuPosition.over,
      itemBuilder: (_) => [
        for (final v in options)
          PopupMenuItem(value: v, child: Text(L.of(context).giftQty(v))),
      ],
      child: Container(
        key: const ValueKey('gift-qty-picker'),
        height: 50,
        // 最窄 84：「×1 ▾」按内容只有 60 宽，50 高就是个圆，看着像个按钮不像下拉框。
        constraints: const BoxConstraints(minWidth: 84),
        padding: const EdgeInsets.symmetric(horizontal: Dim.s3),
        decoration: BoxDecoration(
          color: c.surface,
          borderRadius: Dim.brPill,
          border: Border.all(color: c.line),
        ),
        child: Center(
          widthFactor: 1,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                L.of(context).giftQty(qty),
                style: TextStyle(
                  fontSize: Dim.t3,
                  fontWeight: FontWeight.w700,
                  color: c.ink,
                ),
              ),
              Icon(Icons.arrow_drop_down_rounded, size: 18, color: c.ink3),
            ],
          ),
        ),
      ),
    );
  }
}
