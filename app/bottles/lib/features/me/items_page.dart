import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../domain/models/chat.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/flows/gift_flows.dart';
import '../../ui/flows/purchase_flows.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/coin.dart';
import '../../ui/widgets/gift_icon.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';
import '../../core/utils/time_format.dart';
import '../../ui/widgets/cards.dart';

/// 道具流水（买进 / 送出 / 收到）。买完、送完都要 invalidate，否则记录区不动。
final itemRecordsProvider = FutureProvider<List<ItemRecord>>((ref) {
  return ref.watch(walletRepoProvider).itemRecords();
});

/// 「我的-道具」。
///
/// 这个入口以前直接跳充值页——充值买的是金币，道具是另一层：
/// 金币买道具进背包，送礼时消耗背包里的道具。两件事混在一个入口里，
/// 用户点「道具」看到的是充值档位，会以为送礼要现买现付。
class ItemsPage extends ConsumerStatefulWidget {
  const ItemsPage({super.key});

  @override
  ConsumerState<ItemsPage> createState() => _ItemsPageState();
}

class _ItemsPageState extends ConsumerState<ItemsPage> {
  String? _buying;

  Future<void> _buy(GiftItem gift) async {
    if (_buying != null) return;
    final l = L.of(context);
    final balance = ref.read(walletProvider).value?.coins ?? 0;

    if (balance < gift.coins) {
      await showInsufficientModal(
        context,
        ref,
        itemName: gift.name,
        need: gift.coins,
        have: balance,
      );
      return;
    }

    final ok = await showAppModal<bool>(
      context,
      builder: (ctx) => ModalCard(
        iconWidget: GiftIcon(gift: gift, size: 40),
        title: gift.name,
        body: Text(l.itemsBuyBody(gift.coins)),
        actions: [
          AppButton(
            label: l.itemsBuy,
            kind: BtnKind.pay,
            coins: gift.coins,
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

    setState(() => _buying = gift.id);
    try {
      await ref.read(walletRepoProvider).buyItem(gift.id);
      ref.invalidate(myItemsProvider);
      ref.invalidate(itemRecordsProvider);
      await ref.read(walletProvider.notifier).refresh();
      if (mounted) showToast(context, l.itemsBought(gift.name));
    } on ApiException catch (e) {
      if (!mounted) return;
      if (e.isInsufficientBalance) {
        await showInsufficientModal(
          context,
          ref,
          itemName: gift.name,
          need: gift.coins,
          have: balance,
        );
      } else {
        showToast(context, e.message, error: true);
      }
    } finally {
      if (mounted) setState(() => _buying = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final gifts = ref.watch(giftsProvider);
    final owned = ref.watch(myItemsProvider).value ?? const <String, int>{};
    final balance = ref.watch(walletProvider).value?.coins ?? 0;
    final records = ref.watch(itemRecordsProvider);

    return Scaffold(
      appBar: NavBar(
        title: l.meItems,
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: Dim.s2),
            child: Center(
              child: CoinChip(
                coins: balance,
                onHeader: false,
                onTap: () => context.push(Routes.recharge),
              ),
            ),
          ),
        ],
      ),
      body: AsyncView(
        value: gifts,
        onRetry: () {
          ref.invalidate(giftsProvider);
          ref.invalidate(myItemsProvider);
        },
        errorTitle: l.stateErrorTitle,
        errorBody: l.stateErrorBody,
        retryLabel: l.commonRetry,
        skeleton: const Padding(
          padding: EdgeInsets.all(Dim.gutter),
          child: SkeletonRows(count: 5),
        ),
        data: (list) {
          if (list.isEmpty) {
            return EmptyState(
              icon: Icons.card_giftcard_outlined,
              title: l.itemsEmptyTitle,
              body: l.itemsHint,
            );
          }
          return RefreshIndicator(
            onRefresh: () async {
              ref.invalidate(giftsProvider);
              ref.invalidate(myItemsProvider);
              ref.invalidate(itemRecordsProvider);
            },
            child: ListView(
              padding: const EdgeInsets.fromLTRB(
                Dim.gutter,
                Dim.s3,
                Dim.gutter,
                Dim.s6,
              ),
              children: [
                GridView.builder(
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 3,
                    mainAxisSpacing: Dim.s3,
                    crossAxisSpacing: Dim.s3,
                    childAspectRatio: 0.78,
                  ),
                  itemCount: list.length + 1,
                  itemBuilder: (context, i) {
                    // 第一格放说明：道具和金币的关系不解释清楚，用户不知道买来干嘛。
                    if (i == 0) {
                      return _HintTile(text: l.itemsHint);
                    }
                    final g = list[i - 1];
                    return _ItemTile(
                      gift: g,
                      owned: owned[g.id] ?? 0,
                      busy: _buying == g.id,
                      onTap: () => _buy(g),
                    );
                  },
                ),
                const SizedBox(height: Dim.s5),
                // 流水：买了什么、送给了谁、收到了谁的。送出的即使是背包抵扣也在这里。
                SectionLabel(l.itemsRecords),
                const SizedBox(height: Dim.s2),
                records.when(
                  loading: () => const SkeletonRows(count: 3),
                  error: (_, _) => Text(
                    l.stateLoadFailed,
                    style: TextStyle(fontSize: Dim.t2, color: c.ink3),
                  ),
                  data: (rows) => rows.isEmpty
                      ? Padding(
                          padding: const EdgeInsets.symmetric(vertical: Dim.s4),
                          child: Text(
                            l.itemsRecordsEmpty,
                            style: TextStyle(fontSize: Dim.t2, color: c.ink3),
                          ),
                        )
                      : ListGroup(
                          children: [
                            for (final r in rows) _RecordRow(record: r),
                          ],
                        ),
                ),
              ],
            ),
          );
        },
      ),
      backgroundColor: c.page,
    );
  }
}

/// 道具流水一行：图标 + 「买入 / 送给 xx / 来自 xx」+ 时间；买入显示花了多少币。
class _RecordRow extends StatelessWidget {
  const _RecordRow({required this.record});

  final ItemRecord record;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final r = record;
    final gift = GiftItem(
      id: r.id,
      name: r.itemName,
      emoji: r.itemIcon,
      coins: r.coins,
    );
    final kind = switch (r.kind) {
      'buy' => l.itemsRecordBuy,
      'sent' => l.itemsRecordSent(r.peerName.isEmpty ? '—' : r.peerName),
      _ => l.itemsRecordReceived(r.peerName.isEmpty ? '—' : r.peerName),
    };
    return ListRowItem(
      title: r.itemName.isEmpty ? '—' : r.itemName,
      subtitle: kind,
      leading: GiftIcon(gift: gift, size: 28),
      showChevron: false,
      trailing: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          if (r.kind == 'buy')
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  '-${r.coins}',
                  style: TextStyle(
                    fontSize: Dim.t2,
                    fontWeight: FontWeight.w800,
                    color: c.ink,
                  ),
                ),
                const SizedBox(width: 3),
                const CoinIcon(size: 12),
              ],
            ),
          Text(
            timeAgo(context, r.at),
            style: TextStyle(fontSize: Dim.t0, color: c.ink3),
          ),
        ],
      ),
    );
  }
}

class _HintTile extends StatelessWidget {
  const _HintTile({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      padding: const EdgeInsets.all(Dim.s3),
      decoration: BoxDecoration(
        color: c.brand.withValues(alpha: .06),
        borderRadius: Dim.brCard,
        border: Border.all(color: c.line),
      ),
      alignment: Alignment.center,
      child: Text(
        text,
        textAlign: TextAlign.center,
        style: TextStyle(fontSize: Dim.t1, height: 1.5, color: c.ink3),
      ),
    );
  }
}

class _ItemTile extends StatelessWidget {
  const _ItemTile({
    required this.gift,
    required this.owned,
    required this.busy,
    required this.onTap,
  });

  final GiftItem gift;
  final int owned;
  final bool busy;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    return GestureDetector(
      onTap: busy ? null : onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: Dim.s3),
        decoration: BoxDecoration(
          color: c.surface,
          borderRadius: Dim.brCard,
          border: Border.all(color: c.line),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Stack(
              clipBehavior: Clip.none,
              children: [
                GiftIcon(gift: gift, size: 40),
                // 持有数角标：库存制的关键信息，没它就不知道能不能送。
                if (owned > 0)
                  Positioned(
                    right: -12,
                    top: -6,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 5,
                        vertical: 1,
                      ),
                      decoration: BoxDecoration(
                        color: c.brand,
                        borderRadius: Dim.brPill,
                      ),
                      child: Text(
                        '×$owned',
                        style: const TextStyle(
                          fontSize: 10,
                          fontWeight: FontWeight.w800,
                          color: Colors.white,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: Dim.s2),
            Text(
              gift.name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: Dim.t2,
                fontWeight: FontWeight.w700,
                color: c.ink,
              ),
            ),
            const SizedBox(height: 2),
            Text(
              owned > 0 ? l.itemsOwned(owned) : l.itemsNotOwned,
              style: TextStyle(fontSize: Dim.t0, color: c.ink3),
            ),
            const SizedBox(height: Dim.s2),
            if (busy)
              const SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            else
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const CoinIcon(size: 13),
                  const SizedBox(width: 3),
                  Text(
                    '${gift.coins}',
                    style: TextStyle(
                      fontSize: Dim.t2,
                      fontWeight: FontWeight.w800,
                      color: c.gold,
                    ),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }
}
