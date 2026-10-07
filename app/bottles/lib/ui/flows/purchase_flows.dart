import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/config/remote_config.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../widgets/buttons.dart';
import '../widgets/coin.dart';
import '../widgets/overlays.dart';

/// H5 次数用尽。
///
/// 后端返回 `3003 CodeQuotaExceeded` 时弹这里，**不要弹成通用错误提示**——这是转化入口。
/// 弹窗必须可关闭（✕ + 点遮罩）；余额不足时「购买」直接跳充值并带回跳，
/// 不要只弹一句「余额不足」。
Future<void> showQuotaExceededModal(
  BuildContext context,
  WidgetRef ref, {
  required int usedUp,
  int packSize = 20,
  int packPrice = 20,
}) async {
  final l = L.of(context);
  // 这是转化弹窗，文案值得让运营调。后台没配就用内置的那两句。
  final cfg = ref.read(appConfigProvider);

  await showAppModal<void>(
    context,
    builder: (ctx) => _QuotaModal(
      title: cfg.quotaTitleOr(l.quotaUsedUpTitle(usedUp), usedUp),
      body: AppRemoteConfig.or(cfg.quotaBody, l.quotaUsedUpBody),
      packSize: packSize,
      packPrice: packPrice,
      ref: ref,
    ),
  );
}

class _QuotaModal extends StatefulWidget {
  const _QuotaModal({
    required this.title,
    required this.body,
    required this.packSize,
    required this.packPrice,
    required this.ref,
  });

  final String title;
  final String body;
  final int packSize;
  final int packPrice;
  final WidgetRef ref;

  @override
  State<_QuotaModal> createState() => _QuotaModalState();
}

class _QuotaModalState extends State<_QuotaModal> {
  bool _busy = false;

  Future<void> _buy() async {
    final l = L.of(context);
    final balance = widget.ref.read(walletProvider).value?.coins ?? 0;
    if (balance < widget.packPrice) {
      Navigator.of(context).pop();
      await showInsufficientModal(
        context,
        widget.ref,
        itemName: l.quotaPackTitle(widget.packSize),
        need: widget.packPrice,
        have: balance,
      );
      return;
    }

    setState(() => _busy = true);
    try {
      final quota = await widget.ref.read(walletRepoProvider).buyScoopPack();
      widget.ref.read(quotaProvider.notifier).setLocal(quota);
      await widget.ref.read(walletProvider.notifier).refresh();
      if (mounted) Navigator.of(context).pop();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _watchAd() async {
    setState(() => _busy = true);
    try {
      // App 端的激励视频要换 AdMob，这里只走入账链路。
      final coins = await widget.ref.read(walletRepoProvider).adReward();
      await widget.ref.read(walletProvider.notifier).refresh();
      if (!mounted) return;
      Navigator.of(context).pop();
      showToast(context, L.of(context).rewardGot(coins));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return ModalCard(
      icon: Icons.phishing_rounded,
      title: widget.title,
      body: Text(widget.body),
      actions: [
        _OfferRow(
          icon: Icons.phishing_rounded,
          title: l.quotaPackTitle(widget.packSize),
          subtitle: l.quotaPackHint,
          price: widget.packPrice,
        ),
        const SizedBox(height: Dim.s1),
        AppButton(
          label: l.quotaBuyWithCoins(widget.packPrice),
          kind: BtnKind.pay,
          loading: _busy,
          onTap: _busy ? null : _buy,
        ),
        AppButton(
          label: l.quotaWatchAdForOne,
          kind: BtnKind.ghost,
          icon: Icons.slow_motion_video_rounded,
          onTap: _busy ? null : _watchAd,
        ),
      ],
    );
  }
}

/// Z4 余额不足。
///
/// 带**推荐档位**直达充值并回跳原场景续做，少一跳转化差很多；
/// 同时必须给「换个便宜的」退路——不是每个人都会当场充值，别把会话堵死。
Future<void> showInsufficientModal(
  BuildContext context,
  WidgetRef ref, {
  required String itemName,
  required int need,
  required int have,
  VoidCallback? onPickCheaper,
}) {
  final l = L.of(context);
  return showAppModal<void>(
    context,
    builder: (ctx) => ModalCard(
      iconWidget: const CoinIcon(size: 40),
      title: l.stateInsufficientTitle(need - have),
      body: Text(l.stateInsufficientBody(itemName, need, have)),
      actions: [
        AppButton(
          label: l.stateInsufficientCta,
          kind: BtnKind.pay,
          onTap: () {
            Navigator.of(ctx).pop();
            ctx.push(Routes.recharge);
          },
        ),
        if (onPickCheaper != null)
          AppButton(
            label: l.stateInsufficientAlt,
            kind: BtnKind.ghost,
            onTap: () {
              Navigator.of(ctx).pop();
              onPickCheaper();
            },
          ),
      ],
    ),
  );
}

/// 礼物库存不足(聊天送礼是库存制,消耗背包里预购的礼物)。
///
/// 与金币不足(跳充值页买金币)不是一回事——这里跳**道具页**,用金币买礼物入库。
/// 弹确认再跳,不直接跳走,免得打断当前会话。
Future<void> showItemInsufficientModal(
  BuildContext context, {
  required String giftName,
}) {
  final l = L.of(context);
  return showAppModal<void>(
    context,
    builder: (ctx) => ModalCard(
      icon: Icons.card_giftcard_rounded,
      title: l.giftOutOfStockTitle,
      body: Text(l.itemsHint),
      actions: [
        AppButton(
          label: l.giftGoBuy,
          kind: BtnKind.pay,
          onTap: () {
            Navigator.of(ctx).pop();
            ctx.push(Routes.items);
          },
        ),
        AppButton(
          label: l.commonCancel,
          kind: BtnKind.ghost,
          onTap: () => Navigator.of(ctx).pop(),
        ),
      ],
    ),
  );
}

class _OfferRow extends StatelessWidget {
  const _OfferRow({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.price,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final int price;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    // 原型 .offer：padding s3、1.5 brand 描边、r3、粉 .06 底；价格 t5/800 gold。
    return Container(
      padding: const EdgeInsets.all(Dim.s3),
      decoration: BoxDecoration(
        color: c.brand.withValues(alpha: .06),
        borderRadius: Dim.brCard,
        border: Border.all(color: c.brand, width: 1.5),
      ),
      child: Row(
        children: [
          Icon(icon, size: 26, color: c.brand),
          const SizedBox(width: Dim.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    fontSize: Dim.t3,
                    fontWeight: FontWeight.w700,
                    color: c.ink,
                  ),
                ),
                Text(
                  subtitle,
                  style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                ),
              ],
            ),
          ),
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              const CoinIcon(size: 17),
              const SizedBox(width: 3),
              Text(
                '$price',
                style: TextStyle(
                  fontSize: Dim.t5,
                  fontWeight: FontWeight.w800,
                  color: c.gold,
                  height: 1.25,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
