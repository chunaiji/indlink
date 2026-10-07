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
import '../../ui/widgets/states.dart';
import 'me_controller.dart';

/// 流水场景 → 展示文案。
String txnSceneLabel(BuildContext context, TxnScene scene) {
  final l = L.of(context);
  return switch (scene) {
    TxnScene.unlock => l.walletSceneUnlock,
    TxnScene.chat => l.walletSceneChat,
    TxnScene.msg => l.walletSceneMsg,
    TxnScene.gift => l.walletSceneGift,
    TxnScene.recharge => l.walletSceneRecharge,
    TxnScene.reward => l.walletSceneReward,
    TxnScene.checkin => l.walletSceneCheckin,
    TxnScene.share => l.walletSceneShare,
    TxnScene.adReward => l.walletSceneAdReward,
    TxnScene.rewind => l.walletSceneRewind,
    TxnScene.discoverSkip => l.walletSceneDiscoverSkip,
    TxnScene.refund => l.walletSceneRefund,
  };
}

/// H2 钱包流水。
///
/// `WalletTxn.BalanceAfter` 逐条记录变动后余额，流水天然可对账，不需要额外快照表。
/// 类别：列表页（规范 §6）
/// 固定区：NavBar（去充值）、余额卡、筛选 chips
/// 弹性区：无
/// 可滚动区：流水列表
/// 键盘：无
class WalletPage extends ConsumerStatefulWidget {
  const WalletPage({super.key});

  @override
  ConsumerState<WalletPage> createState() => _WalletPageState();
}

class _WalletPageState extends ConsumerState<WalletPage> {
  String _filter = 'all';

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final wallet = ref.watch(walletProvider).value;
    final txns = ref.watch(walletTxnsProvider);

    return Scaffold(
      appBar: NavBar(
        title: l.walletTitle,
        actions: [
          TextButton(
            onPressed: () => context.push(Routes.recharge),
            child: Text(
              l.walletGoRecharge,
              style: TextStyle(
                fontSize: Dim.t2,
                fontWeight: FontWeight.w700,
                color: c.brand,
              ),
            ),
          ),
        ],
      ),
      body: Column(
        children: [
          // 负余额 = 有一笔充值被渠道退款且部分已消费。不解释的话，
          // 用户只会当成系统 bug 然后直接投诉。
          if ((wallet?.coins ?? 0) < 0)
            Material(
              color: c.coral.withValues(alpha: .12),
              child: InkWell(
                onTap: () => context.push(Routes.voided),
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: Dim.gutter,
                    vertical: Dim.s3,
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.error_outline_rounded,
                        size: 18,
                        color: c.coral,
                      ),
                      const SizedBox(width: Dim.s2),
                      Expanded(
                        child: Text(
                          l.walletNegativeBanner,
                          style: TextStyle(fontSize: Dim.t1, color: c.coral),
                        ),
                      ),
                      Icon(
                        Icons.chevron_right_rounded,
                        size: 18,
                        color: c.coral,
                      ),
                    ],
                  ),
                ),
              ),
            ),
          Padding(
            padding: const EdgeInsets.all(Dim.gutter),
            // 原型 .bal 通栏；Column 默认居中，不写宽度它就按内容缩成中间一小块。
            child: Container(
              key: const ValueKey('wallet-balance-card'),
              width: double.infinity,
              padding: const EdgeInsets.all(Dim.s4),
              decoration: BoxDecoration(
                borderRadius: Dim.brLargeCard,
                border: Border.all(color: c.line2),
                gradient: LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    c.brand.withValues(alpha: .10),
                    c.brand2.withValues(alpha: .12),
                  ],
                ),
              ),
              child: Column(
                children: [
                  // 原型 .bal：数字 26px → 39 tabular，标签 t1 ink3，总计行 gap s6。
                  Text(
                    '${wallet?.coins ?? 0}',
                    style: TextStyle(
                      fontSize: 39,
                      fontWeight: FontWeight.w800,
                      height: 1.1,
                      color: c.ink,
                      fontFeatures: const [FontFeature.tabularFigures()],
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    l.walletCurrentBalance,
                    style: TextStyle(
                      fontSize: Dim.t1,
                      color: c.ink3,
                      height: 1.4,
                    ),
                  ),
                  const SizedBox(height: Dim.s3),
                  Wrap(
                    alignment: WrapAlignment.center,
                    spacing: Dim.s6,
                    runSpacing: Dim.s1,
                    children: [
                      _MiniStat(
                        label: l.walletTotalRecharged,
                        value: wallet?.totalRecharged ?? 0,
                      ),
                      _MiniStat(
                        label: l.walletTotalSpent,
                        value: wallet?.totalSpent ?? 0,
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
          ChipBar(
            children: [
              for (final (key, label) in [
                ('all', l.commonAll),
                ('in', l.walletTabIncome),
                ('out', l.walletTabExpense),
              ])
                PillChip(
                  label: label,
                  selected: _filter == key,
                  onTap: () => setState(() => _filter = key),
                ),
            ],
          ),
          const SizedBox(height: Dim.s2),
          Expanded(
            child: AsyncView(
              value: txns,
              onRetry: () => ref.invalidate(walletTxnsProvider),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const Padding(
                padding: EdgeInsets.symmetric(horizontal: Dim.gutter),
                child: SkeletonRows(count: 6),
              ),
              data: (all) {
                final list = switch (_filter) {
                  'in' => all.where((t) => t.isIncome).toList(),
                  'out' => all.where((t) => !t.isIncome).toList(),
                  _ => all,
                };
                if (list.isEmpty) {
                  return EmptyState(
                    icon: Icons.receipt_long_outlined,
                    title: l.walletEmpty,
                    body: l.stateEmptyBottlesBody,
                  );
                }
                return ListView.builder(
                  padding: context.pagePadding(top: 0, bottom: Dim.s5),
                  itemCount: list.length,
                  itemBuilder: (context, i) => _TxnRow(txn: list[i]),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _MiniStat extends StatelessWidget {
  const _MiniStat({required this.label, required this.value});

  final String label;
  final int value;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          label,
          style: TextStyle(fontSize: Dim.t1, color: c.ink3),
        ),
        const SizedBox(width: 4),
        Text(
          '$value',
          style: TextStyle(
            fontSize: Dim.t2,
            fontWeight: FontWeight.w800,
            color: c.ink,
          ),
        ),
      ],
    );
  }
}

class _TxnRow extends StatelessWidget {
  const _TxnRow({required this.txn});

  final WalletTxn txn;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    // 原型 .ordr a：最小高 44、padding s2 0、底线 line2。
    return Container(
      constraints: const BoxConstraints(minHeight: Dim.tap),
      padding: const EdgeInsets.symmetric(vertical: Dim.s2),
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: c.line2)),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Text(
                  txn.note ?? txnSceneLabel(context, txn.scene),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: Dim.t3,
                    fontWeight: FontWeight.w700,
                    height: 1.4,
                    color: c.ink,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  '${txn.scene.wire} · ${stampShort(txn.at)}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: Dim.t1,
                    color: c.ink3,
                    height: 1.4,
                    fontFeatures: const [FontFeature.tabularFigures()],
                  ),
                ),
              ],
            ),
          ),
          Text(
            txn.isIncome ? '+${txn.amount}' : '${txn.amount}',
            style: TextStyle(
              fontSize: Dim.t4,
              fontWeight: FontWeight.w800,
              color: txn.isIncome ? c.success : c.brand,
            ),
          ),
        ],
      ),
    );
  }
}
