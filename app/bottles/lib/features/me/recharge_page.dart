import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/logging/log_record.dart';
import '../../core/logging/logger.dart';
import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../domain/models/wallet.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/coin.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';
import 'me_controller.dart';

/// H3 充值 · 平台分叉。
///
/// ⚠️ **iOS 数字商品必须走 IAP**（App Store 3.1.1），第三方渠道会被拒审；
/// 引导到站外网页充值同样违规（3.1.3 / 4.2）。所以 iOS 构建下整屏只展示 IAP 商品列表，
/// UPI / Card 全部不出现。这条没有绕过路径。
///
/// 连锁后果：同一份金币在两端**价格与到账链路分叉**，`PayOrder.Platform` 需扩展，
/// 运营配置的充值档位要按平台拆分。
/// 类别：表单页（规范 §6）
/// 固定区：头图、底部「支付 ¥ · 得 N」
/// 弹性区：无
/// 可滚动区：档位网格 + 支付方式
/// 键盘：无
class RechargePage extends ConsumerStatefulWidget {
  const RechargePage({super.key, this.from});

  /// 被打断的原场景，一路透传到 H8 的主按钮。
  ///
  /// 多数充值是被 H5「次数用尽」或 Z4「余额不足」打断后进来的，
  /// 付完把人丢回「我的」，等于让他自己找回刚才在干什么。
  final String? from;

  @override
  ConsumerState<RechargePage> createState() => _RechargePageState();
}

class _RechargePageState extends ConsumerState<RechargePage> {
  String? _selectedId;
  PayChannel _channel = PayChannel.upi;
  bool _busy = false;

  /// iOS 上强制 IAP。
  ///
  /// ⚠️ 未完成的一半：iOS 的档位与价格应当来自 **StoreKit 商品列表**
  /// （`in_app_purchase` + App Store Connect 里配好的商品 ID），而不是后端
  /// `GET /api/pay/packages` —— 苹果要求展示商店本地化后的价格。
  /// 现在两端共用后端档位，接 StoreKit 时这里要换数据源。
  bool get _iosIap => !kIsWeb && Platform.isIOS;

  Future<void> _pay(RechargePackage pkg) async {
    setState(() => _busy = true);
    final channel = _iosIap ? PayChannel.iap : _channel;
    // 支付这条链路要能事后对账：发起与结果各记一条，中间断在哪一目了然。
    Log.i(
      LogTag.biz,
      'pay_start',
      fields: {'pkg': pkg.id, 'channel': channel.name},
    );
    try {
      // ⚠️ 下单 ≠ 已支付。这里只拿到 order_no，钱到没到由支付状态机轮询服务端说了算。
      // 旧实现在这一步直接刷钱包并 toast「到账」，那是在替服务端做结论。
      final order = await ref.read(walletRepoProvider).recharge(pkg, channel);
      Log.i(
        LogTag.biz,
        'pay_order_created',
        fields: {'pkg': pkg.id, 'order': order.orderNo},
      );
      if (!mounted) return;
      context.push(
        Routes.payFlow(order.orderNo, from: widget.from),
        extra: order,
      );
    } on ApiException catch (e) {
      Log.w(
        LogTag.biz,
        'pay_result',
        fields: {'pkg': pkg.id, 'ok': false, 'code': e.code},
      );
      if (!mounted) return;
      showToast(context, e.message, error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final packages = ref.watch(packagesProvider);
    final wallet = ref.watch(walletProvider).value;

    return Scaffold(
      body: Column(
        children: [
          GradientHeader(
            slim: true,
            topRow: Row(
              children: [
                IconButton(
                  icon: const Icon(
                    Icons.arrow_back_ios_new_rounded,
                    size: 18,
                    color: Colors.white,
                  ),
                  onPressed: () => context.pop(),
                ),
                Expanded(
                  child: Text(
                    l.rechargeTitle,
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
                CoinChip(coins: wallet?.coins ?? 0),
              ],
            ),
          ),
          Expanded(
            child: AsyncView(
              value: packages,
              onRetry: () => ref.invalidate(packagesProvider),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const Padding(
                padding: EdgeInsets.all(Dim.gutter),
                child: SkeletonRows(count: 4),
              ),
              data: (list) {
                _selectedId ??= list.isEmpty ? null : list.first.id;
                final selected = list
                    .where((p) => p.id == _selectedId)
                    .firstOrNull;

                return Column(
                  children: [
                    Expanded(
                      child: ListView(
                        padding: const EdgeInsets.all(Dim.gutter),
                        children: [
                          SectionLabel(l.rechargeChoosePackage),
                          const SizedBox(height: Dim.s3),
                          GridView.count(
                            crossAxisCount: 2,
                            shrinkWrap: true,
                            physics: const NeverScrollableScrollPhysics(),
                            mainAxisSpacing: Dim.s2,
                            crossAxisSpacing: Dim.s2,
                            // 原型 .pkg ≈ 172×77（2.2）；按 2.0 留给 360 宽 + 大字体一点余量。
                            childAspectRatio: 2.0,
                            children: [
                              for (final pkg in list)
                                PackageTile(
                                  coins: pkg.coins,
                                  price: pkg.priceLabel,
                                  original: pkg.originalPriceLabel,
                                  badge: pkg.bonusPercent > 0
                                      ? l.rechargeBonus(pkg.bonusPercent)
                                      : null,
                                  selected: pkg.id == _selectedId,
                                  onTap: () =>
                                      setState(() => _selectedId = pkg.id),
                                ),
                            ],
                          ),
                          const SizedBox(height: Dim.s5),
                          SectionLabel(l.rechargePaymentMethod),
                          const SizedBox(height: Dim.s3),
                          // iOS：只有 App Store 一条路，不给选择（给了也是违规）。
                          if (_iosIap) ...[
                            RadioRow(
                              icon: Icons.apple,
                              label: l.rechargeIosChannel,
                              selected: true,
                              onTap: () {},
                            ),
                            const SizedBox(height: Dim.s3),
                            NoticeBanner(
                              icon: Icons.credit_card_rounded,
                              text: l.rechargeIosHint,
                            ),
                          ] else ...[
                            // 原型 .pay 单选行：50 高，选中 brand 描边 + 实心圆点。
                            RadioRow(
                              icon: Icons.phone_android_rounded,
                              label: 'UPI',
                              subtitle: l.rechargeUpiSub,
                              selected: _channel == PayChannel.upi,
                              onTap: () =>
                                  setState(() => _channel = PayChannel.upi),
                            ),
                            const SizedBox(height: Dim.s2),
                            RadioRow(
                              icon: Icons.credit_card_rounded,
                              label: l.rechargeCardSub,
                              selected: _channel == PayChannel.card,
                              onTap: () =>
                                  setState(() => _channel = PayChannel.card),
                            ),
                          ],
                        ],
                      ),
                    ),
                    BottomActionBar(
                      child: AppButton(
                        label: selected == null
                            ? l.rechargeChoosePackage
                            : l.rechargePayAndGet(
                                selected.priceLabel,
                                selected.coins,
                              ),
                        kind: BtnKind.pay,
                        loading: _busy,
                        onTap: selected == null || _busy
                            ? null
                            : () => _pay(selected),
                      ),
                    ),
                  ],
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
