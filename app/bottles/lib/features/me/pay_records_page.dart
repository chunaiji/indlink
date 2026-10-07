import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/wallet.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';

/// 充值订单列表。每次进页面重拉，不缓存——用户来这里就是因为对状态有疑问。
final payOrdersProvider = FutureProvider.autoDispose<List<PayOrderInfo>>(
  (ref) => ref.watch(walletRepoProvider).orders(),
);

/// H10 充值记录。
///
/// 支付客诉集中在「我付了钱没到账」，而这一屏加上 H9 是用户能自己查清楚的唯一途径。
/// 少了它，省下的前端工时会以十倍的客服成本还回来。
/// 类别：列表页（规范 §6）
/// 固定区：NavBar（恢复购买）
/// 弹性区：无
/// 可滚动区：订单列表
/// 键盘：无
class PayRecordsPage extends ConsumerStatefulWidget {
  const PayRecordsPage({super.key});

  @override
  ConsumerState<PayRecordsPage> createState() => _PayRecordsPageState();
}

class _PayRecordsPageState extends ConsumerState<PayRecordsPage> {
  bool _restoring = false;

  /// 「恢复购买」。
  ///
  /// 金币是消耗型商品，苹果不强制提供这个入口，但它是掉单时用户能自救的唯一按钮。
  /// 当前语义 = 把所有 pending 单重查一遍；接入 StoreKit / Play 之后，
  /// 在这里补上「拉渠道未完成交易」那一步，**按钮与文案都不用改**。
  Future<void> _restore() async {
    if (_restoring) return;
    setState(() => _restoring = true);
    final l = L.of(context);
    try {
      final repo = ref.read(walletRepoProvider);
      final list = await repo.orders();
      var updated = 0;
      for (final o in list.where((o) => o.status == OrderStatus.pending)) {
        final fresh = await repo.orderStatus(o.orderNo);
        if (fresh.status != OrderStatus.pending) updated++;
      }
      if (updated > 0) await ref.read(walletProvider.notifier).refresh();
      ref.invalidate(payOrdersProvider);
      if (!mounted) return;
      showToast(
        context,
        updated > 0 ? l.payRestoreDone(updated) : l.payRestoreNone,
      );
    } finally {
      if (mounted) setState(() => _restoring = false);
    }
  }

  /// 点一笔处理中的订单：主动查单。状态变了就刷新钱包与列表。
  Future<void> _recheck(PayOrderInfo order) async {
    final repo = ref.read(walletRepoProvider);
    final fresh = await repo.orderStatus(order.orderNo);
    if (fresh.status == OrderStatus.paid) {
      await ref.read(walletProvider.notifier).refresh();
    }
    ref.invalidate(payOrdersProvider);
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final orders = ref.watch(payOrdersProvider);

    return Scaffold(
      appBar: NavBar(
        title: l.payRecordsTitle,
        actions: [
          TextButton(
            onPressed: _restoring ? null : _restore,
            child: Text(l.payRestore),
          ),
        ],
      ),
      body: AsyncView(
        value: orders,
        onRetry: () => ref.invalidate(payOrdersProvider),
        errorTitle: l.stateErrorTitle,
        errorBody: l.stateErrorBody,
        retryLabel: l.commonRetry,
        skeleton: const Padding(
          padding: EdgeInsets.all(Dim.gutter),
          child: SkeletonRows(count: 4),
        ),
        data: (list) {
          if (list.isEmpty) {
            return EmptyState(
              icon: Icons.receipt_long_rounded,
              title: l.payRecordsEmpty,
              body: l.payRecordsEmptyHint,
              ctaLabel: l.payRecordsEmptyHint,
              onCta: () => context.push(Routes.recharge),
            );
          }
          return ListView.separated(
            padding: context.pagePadding(),
            itemCount: list.length,
            separatorBuilder: (_, _) => const SizedBox(height: Dim.s2),
            itemBuilder: (_, i) =>
                _OrderRow(order: list[i], onRecheck: () => _recheck(list[i])),
          );
        },
      ),
    );
  }
}

class _OrderRow extends StatelessWidget {
  const _OrderRow({required this.order, required this.onRecheck});

  final PayOrderInfo order;
  final Future<void> Function() onRecheck;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    // 原型 .stt 状态标：ok 绿 / ing 海蓝 / err 警示 / rfd 灰；.oid 订单号等宽。
    final (label, kind) = switch (order.status) {
      OrderStatus.paid => (l.payStatusPaid, StatusTagKind.ok),
      OrderStatus.pending => (l.payStatusPending, StatusTagKind.pending),
      OrderStatus.failed => (l.payStatusFailed, StatusTagKind.error),
      OrderStatus.refunded => (l.payStatusRefunded, StatusTagKind.refunded),
    };

    // 待支付的单点一下会去服务端重查一次状态（掉单自救入口）。
    final pending = order.status == OrderStatus.pending;

    return Material(
      color: c.surface,
      borderRadius: Dim.brCard,
      child: InkWell(
        onTap: pending ? () => onRecheck() : null,
        borderRadius: Dim.brCard,
        child: Container(
          constraints: const BoxConstraints(minHeight: 65),
          padding: const EdgeInsets.symmetric(
            horizontal: Dim.s4,
            vertical: Dim.s3,
          ),
          decoration: BoxDecoration(
            borderRadius: Dim.brCard,
            border: Border.all(color: c.line),
          ),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Flexible(
                          child: Text(
                            l.payCoinsAmount(order.coins),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: Dim.t3,
                              fontWeight: FontWeight.w700,
                              height: 1.4,
                              color: c.ink,
                            ),
                          ),
                        ),
                        const SizedBox(width: Dim.s2),
                        StatusTag(label: label, kind: kind),
                      ],
                    ),
                    const SizedBox(height: Dim.s1),
                    Wrap(
                      spacing: Dim.s2,
                      runSpacing: Dim.s1,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: [
                        Text(
                          '${order.platform} · ${compactAgo(context, order.createdAt)}',
                          style: TextStyle(
                            fontSize: Dim.t1,
                            color: c.ink3,
                            height: 1.4,
                          ),
                        ),
                        OrderIdChip(text: order.orderNo),
                      ],
                    ),
                  ],
                ),
              ),
              const SizedBox(width: Dim.s3),
              Text(
                order.priceLabel,
                style: TextStyle(
                  fontSize: Dim.t3,
                  fontWeight: FontWeight.w800,
                  color: c.ink,
                  height: 1.4,
                  fontFeatures: const [FontFeature.tabularFigures()],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
