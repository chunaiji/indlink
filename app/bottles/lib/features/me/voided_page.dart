import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/wallet.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/headers.dart';
import 'me_controller.dart';
import 'wallet_page.dart' show txnSceneLabel;

/// H11 退款已撤销。
///
/// 用户直接向渠道申请退款时，App 端**收不到任何同步信号**——服务端主动查是感知
/// 退款的唯一途径（`pay/iap.go:295` 对 Apple 侧写过同一句话）。服务端的 `revokeIAP`
/// 明确「允许扣成负数」：币可能已经花掉，装作没发生只会让账永远对不平。
///
/// 那么界面就必须有个说法。否则用户打开钱包看到负数余额，
/// 只会认为是系统 bug 并直接投诉。
///
/// ⚠️ 注意「消费暂停」**不需要任何客户端代码**：`wallet.Debit` 在 `bal < coins`
/// 时返回 `ErrInsufficient`（`wallet/service.go:115`），余额为负时任何消费都过不去。
/// 这一屏只负责把它说清楚，不要在客户端再造一套拦截。
/// 类别：详情页（规范 §6）
/// 固定区：NavBar
/// 弹性区：无
/// 可滚动区：余额卡 + 说明 + 相关流水 + 两个按钮
/// 键盘：无
class VoidedPage extends ConsumerWidget {
  const VoidedPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final wallet = ref.watch(walletProvider).value;
    final txns = ref.watch(walletTxnsProvider).value ?? const <WalletTxn>[];

    // 只挑与这次退款相关的两类：退款本身，以及被退掉的那笔充值。
    final related = txns
        .where(
          (t) => t.scene == TxnScene.refund || t.scene == TxnScene.recharge,
        )
        .take(4)
        .toList();

    return Scaffold(
      appBar: NavBar(title: l.voidedTitle),
      body: ListView(
        padding: context.pagePadding(),
        children: [
          Container(
            padding: const EdgeInsets.all(Dim.s5),
            decoration: BoxDecoration(
              color: c.surface,
              borderRadius: BorderRadius.circular(Dim.r3),
            ),
            child: Column(
              children: [
                Text(
                  '${wallet?.coins ?? 0}',
                  style: TextStyle(
                    // 原型 .bal 数字 26px → 39 tabular。
                    fontSize: 39,
                    height: 1.1,
                    fontFeatures: const [FontFeature.tabularFigures()],
                    fontWeight: FontWeight.w800,
                    // 负余额用警示色：这是这一屏要解释的那个数字。
                    color: (wallet?.coins ?? 0) < 0 ? c.coral : c.ink,
                  ),
                ),
                const SizedBox(height: Dim.s1),
                Text(
                  l.voidedBalance,
                  style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                ),
                const SizedBox(height: Dim.s3),
                Text(
                  l.voidedTotals(
                    wallet?.totalRecharged ?? 0,
                    wallet?.totalSpent ?? 0,
                  ),
                  style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                ),
              ],
            ),
          ),
          const SizedBox(height: Dim.s4),
          Text(
            l.voidedTitle,
            style: const TextStyle(
              fontSize: Dim.t4,
              fontWeight: FontWeight.w800,
            ),
          ),
          const SizedBox(height: Dim.s2),
          Text(
            l.voidedBody,
            style: TextStyle(fontSize: Dim.t2, color: c.ink2, height: 1.6),
          ),
          const SizedBox(height: Dim.s3),
          Container(
            padding: const EdgeInsets.all(Dim.s3),
            decoration: BoxDecoration(
              color: c.warn.withValues(alpha: .10),
              borderRadius: BorderRadius.circular(Dim.r2),
            ),
            child: Text(
              l.voidedFrozen,
              style: TextStyle(fontSize: Dim.t1, color: c.ink2, height: 1.5),
            ),
          ),
          if (related.isNotEmpty) ...[
            const SizedBox(height: Dim.s5),
            Text(
              l.voidedRelated,
              style: const TextStyle(
                fontSize: Dim.t2,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: Dim.s2),
            for (final t in related)
              Padding(
                padding: const EdgeInsets.only(bottom: Dim.s2),
                child: Row(
                  children: [
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            txnSceneLabel(context, t.scene),
                            style: const TextStyle(fontSize: Dim.t2),
                          ),
                          const SizedBox(height: Dim.s1),
                          Text(
                            compactAgo(context, t.at),
                            style: TextStyle(fontSize: Dim.t0, color: c.ink3),
                          ),
                        ],
                      ),
                    ),
                    Text(
                      t.isIncome ? '+${t.amount}' : '${t.amount}',
                      style: TextStyle(
                        fontSize: Dim.t3,
                        fontWeight: FontWeight.w700,
                        color: t.isIncome ? c.success : c.coral,
                      ),
                    ),
                  ],
                ),
              ),
          ],
          const SizedBox(height: Dim.s6),
          AppButton(
            label: l.voidedGoRecharge,
            onTap: () => context.push(Routes.recharge),
          ),
          const SizedBox(height: Dim.s2),
          AppButton(
            label: l.voidedNotMe,
            kind: BtnKind.oauth,
            onTap: () => context.push(Routes.settings),
          ),
        ],
      ),
    );
  }
}
