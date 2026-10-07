import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/pay/pay_channel.dart';
import '../../core/providers.dart';
import '../../domain/models/wallet.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/payment_status.dart';
import '../../ui/widgets/overlays.dart';
import 'payment_controller.dart';

/// H7a / H7b / H8 / H9 —— 一笔支付的四个阶段，一页装下。
///
/// 为什么是一页：它们是同一笔交易的四个阶段。做成四条路由之后，H8 按返回会回到
/// 「支付中」——用户会以为还要再付一次，而那正是支付链路最贵的一种误解。
/// 类别：场景页（规范 §6）
/// 固定区：NavBar、底部按钮组（BottomActionBar）
/// 弹性区：居中的状态视图（PaymentStatusView，原型 .pst）
/// 可滚动区：无
/// 键盘：无
class PaymentFlowPage extends ConsumerStatefulWidget {
  const PaymentFlowPage({
    super.key,
    required this.orderNo,
    this.pending,
    this.from,
  });

  final String orderNo;

  /// 下单结果。页面据它选适配器。
  ///
  /// 为空 = 从充值记录深链进来，这笔单早就下过了，直接接管轮询即可。
  final PendingOrder? pending;

  /// 被打断的原场景。H8 主按钮照它回跳。
  final String? from;

  @override
  ConsumerState<PaymentFlowPage> createState() => _PaymentFlowPageState();
}

class _PaymentFlowPageState extends ConsumerState<PaymentFlowPage>
    with WidgetsBindingObserver {
  /// 在 initState 就存住：dispose 里已经不能再 `ref.read`（Riverpod 会抛
  /// StateError），而停表恰恰必须发生在那时。
  late final PaymentController _ctrl = ref.read(
    paymentControllerProvider.notifier,
  );

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) => _start());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    // 停表交给页面而不是 provider 生命周期：定时器活过页面会一直打接口。
    _ctrl.stop();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState s) {
    // 从渠道 App 切回来的那一刻，状态最可能已经变了。
    if (s == AppLifecycleState.resumed) {
      _ctrl.onResumed();
    }
  }

  Future<void> _start() async {
    final ctrl = _ctrl;
    final pending = widget.pending;
    if (pending == null) {
      await ctrl.resume(widget.orderNo);
      return;
    }
    final repo = ref.read(walletRepoProvider);
    await ctrl.start(
      MockChannelAdapter(settle: repo.settleMock, ask: () => _askMockChoice()),
      pending,
    );
  }

  /// 联调面板。真实渠道接入后，这里换成对应的 Adapter，本页其余代码不动。
  Future<MockChoice?> _askMockChoice() {
    if (!mounted) return Future.value(null);
    final l = L.of(context);
    return showAppSheet<MockChoice>(
      context,
      builder: (ctx) => Padding(
        padding: const EdgeInsets.fromLTRB(
          Dim.gutter,
          Dim.s4,
          Dim.gutter,
          Dim.s5,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              l.payMockTitle,
              style: const TextStyle(
                fontSize: Dim.t4,
                fontWeight: FontWeight.w800,
              ),
            ),
            const SizedBox(height: Dim.s4),
            AppButton(
              label: l.payMockSuccess,
              onTap: () => Navigator.of(ctx).pop(MockChoice.success),
            ),
            const SizedBox(height: Dim.s2),
            AppButton(
              label: l.payMockFail,
              kind: BtnKind.ghost,
              onTap: () => Navigator.of(ctx).pop(MockChoice.fail),
            ),
            const SizedBox(height: Dim.s2),
            AppButton(
              label: l.payMockNoResponse,
              kind: BtnKind.ghost,
              onTap: () => Navigator.of(ctx).pop(MockChoice.noResponse),
            ),
          ],
        ),
      ),
    );
  }

  /// 「支付中」期间离开要二次确认。
  ///
  /// 确认之后**不停轮询**——它挂在 controller 上，页面销毁只是停表；
  /// 用户在充值记录里仍然查得到这笔单。
  Future<bool> _confirmLeave() async {
    final l = L.of(context);
    final leave = await showAppModal<bool>(
      context,
      builder: (ctx) => ModalCard(
        title: l.payLeaveConfirm,
        body: const SizedBox.shrink(),
        actions: [
          AppButton(
            label: l.payLeaveStay,
            onTap: () => Navigator.of(ctx).pop(false),
          ),
          const SizedBox(height: Dim.s2),
          AppButton(
            label: l.payLeaveGo,
            kind: BtnKind.ghost,
            onTap: () => Navigator.of(ctx).pop(true),
          ),
        ],
      ),
    );
    if (leave == true && mounted) {
      showToast(context, l.payLeftRunning);
    }
    return leave ?? false;
  }

  void _exitToScene() {
    final from = widget.from;
    if (from != null && from.isNotEmpty) {
      context.go(from);
    } else {
      context.go(Routes.wallet);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final state = ref.watch(paymentControllerProvider);
    final waiting =
        state.phase == PaymentPhase.launching ||
        state.phase == PaymentPhase.awaitingChannel ||
        state.phase == PaymentPhase.verifying;

    // 退款有自己的一屏，而且要解释负余额，不在本页兜。
    ref.listen<PaymentState>(paymentControllerProvider, (_, next) {
      if (next.phase == PaymentPhase.refunded && mounted) {
        context.pushReplacement(Routes.voided);
      }
    });

    return PopScope(
      canPop: !waiting,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        final leave = await _confirmLeave();
        if (!leave || !context.mounted) return;
        context.pop();
      },
      child: Scaffold(
        appBar: NavBar(
          title: l.rechargeTitle,
          onBack: waiting
              ? () async {
                  final leave = await _confirmLeave();
                  if (!leave || !context.mounted) return;
                  context.pop();
                }
              : null,
        ),
        body: SafeArea(
          bottom: false,
          child: switch (state.phase) {
            PaymentPhase.launching || PaymentPhase.awaitingChannel => _Waiting(
              title: l.payWaitingTitle,
              body: l.payWaitingBody,
              elapsed: state.elapsed,
              onRecheck: () => _ctrl.pollNow(),
              onTrouble: () => _ctrl.giveUp(),
            ),
            // H7b 不给取消：票据已在苹果侧生成，取消只会变成掉单。
            PaymentPhase.verifying => _Waiting(
              title: l.payVerifyingTitle,
              body: l.payVerifyingBody,
              elapsed: state.elapsed,
              onRecheck: () => _ctrl.pollNow(),
            ),
            PaymentPhase.succeeded => _Succeeded(
              coins: state.order?.coins ?? 0,
              balance: ref.watch(walletProvider).value?.coins ?? 0,
              onBack: _exitToScene,
              onRecords: () => context.push(Routes.payRecords),
            ),
            PaymentPhase.dropped => _Dropped(
              onRecheck: () => _ctrl.pollNow(),
              onRecords: () => context.push(Routes.payRecords),
              onSupport: () => context.push(Routes.settings),
            ),
            PaymentPhase.failed ||
            PaymentPhase.refunded => _Failed(onBack: () => context.pop()),
          },
        ),
      ),
    );
  }
}

/// H7a / H7b：等待渠道 / 票据校验中。
class _Waiting extends StatelessWidget {
  const _Waiting({
    required this.title,
    required this.body,
    required this.elapsed,
    required this.onRecheck,
    this.onTrouble,
  });

  final String title;
  final String body;
  final Duration elapsed;
  final VoidCallback onRecheck;
  final VoidCallback? onTrouble;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return Column(
      children: [
        Expanded(
          child: PaymentStatusView(
            kind: PaymentStatusKind.pending,
            title: title,
            body: body,
            meta: l.payWaitingElapsed(elapsed.inSeconds),
          ),
        ),
        BottomActionBar(
          children: [
            AppButton(
              label: l.payDoneButton,
              kind: BtnKind.oauth,
              onTap: onRecheck,
            ),
            if (onTrouble != null)
              AppButton(
                label: l.payTroubleButton,
                kind: BtnKind.ghost,
                onTap: onTrouble,
              ),
          ],
        ),
      ],
    );
  }
}

/// H8：支付成功。原型：✓ 环 + 「+300」金色金额 + 「到账了」 + 「余额 248 → 548」。
class _Succeeded extends StatelessWidget {
  const _Succeeded({
    required this.coins,
    required this.balance,
    required this.onBack,
    required this.onRecords,
  });

  final int coins;
  final int balance;
  final VoidCallback onBack;
  final VoidCallback onRecords;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return Column(
      children: [
        Expanded(
          child: PaymentStatusView(
            kind: PaymentStatusKind.ok,
            amount: '+$coins',
            title: l.paySuccessTitle,
            body: l.paySuccessCoins(coins),
            meta: l.payBalanceNow(balance),
          ),
        ),
        BottomActionBar(
          children: [
            // 回跳原场景：多数充值是被「次数用尽」或「余额不足」打断后进来的。
            AppButton(
              label: l.payBackToScene,
              kind: BtnKind.pay,
              onTap: onBack,
            ),
            AppButton(
              label: l.payViewRecords,
              kind: BtnKind.oauth,
              onTap: onRecords,
            ),
          ],
        ),
      ],
    );
  }
}

/// H9：已扣款未到账（掉单）。
class _Dropped extends StatelessWidget {
  const _Dropped({
    required this.onRecheck,
    required this.onRecords,
    required this.onSupport,
  });

  final VoidCallback onRecheck;
  final VoidCallback onRecords;
  final VoidCallback onSupport;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return Column(
      children: [
        Expanded(
          child: PaymentStatusView(
            kind: PaymentStatusKind.error,
            icon: Icons.hourglass_bottom_rounded,
            title: l.payDroppedTitle,
            body: l.payDroppedBody,
          ),
        ),
        BottomActionBar(
          children: [
            AppButton(label: l.payRecheckButton, onTap: onRecheck),
            AppButton(
              label: l.payViewRecords,
              kind: BtnKind.oauth,
              onTap: onRecords,
            ),
            AppButton(
              label: l.payContactSupport,
              kind: BtnKind.ghost,
              onTap: onSupport,
            ),
          ],
        ),
      ],
    );
  }
}

/// 支付未完成（用户取消 / 渠道失败）。
class _Failed extends StatelessWidget {
  const _Failed({required this.onBack});

  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return Column(
      children: [
        Expanded(
          child: PaymentStatusView(
            kind: PaymentStatusKind.error,
            title: l.payFailedTitle,
            body: l.payFailedBody,
          ),
        ),
        BottomActionBar(
          child: AppButton(label: l.payBackToRecharge, onTap: onBack),
        ),
      ],
    );
  }
}
