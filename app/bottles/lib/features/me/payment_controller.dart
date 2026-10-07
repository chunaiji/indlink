import 'dart:async';
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/logging/log_record.dart';
import '../../core/logging/logger.dart';
import '../../core/pay/pay_channel.dart';
import '../../core/providers.dart';
import '../../domain/models/wallet.dart';
import 'me_controller.dart';

/// 一笔支付走过的阶段。
///
/// ⚠️ [awaitingChannel] 与 [verifying] 是**同一档的两个平台变体，不是先后两步**：
/// Android 跳出去付（H7a），iOS 把票据交服务端核实（H7b）。两者共用同一个轮询器。
enum PaymentPhase {
  launching,
  awaitingChannel, // H7a
  verifying, // H7b
  succeeded, // H8
  dropped, // H9
  failed,
  refunded, // → H11
}

/// 轮询退避：1 → 2 → 4 → 8 秒封顶。
///
/// 开头密是因为多数支付在几秒内就有结果；封顶是因为再稀下去，
/// 不如让用户自己点「已完成支付」。
Duration pollDelay(int attempt) {
  const steps = [1, 2, 4, 8];
  return Duration(seconds: attempt < steps.length ? steps[attempt] : 8);
}

/// 累计多久还没到终态就判掉单。
///
/// 原型明示「服务端校验失败时不能一直挂着」。30 秒之后继续转圈，
/// 换来的不是耐心而是第二次支付。
const kPaymentTimeout = Duration(seconds: 30);

bool hasTimedOut(Duration elapsed) => elapsed >= kPaymentTimeout;

/// 服务端订单状态 → 屏幕阶段。
PaymentPhase phaseFor(OrderStatus status, {required bool ios}) =>
    switch (status) {
      OrderStatus.paid => PaymentPhase.succeeded,
      OrderStatus.failed => PaymentPhase.failed,
      OrderStatus.refunded => PaymentPhase.refunded,
      OrderStatus.pending =>
        ios ? PaymentPhase.verifying : PaymentPhase.awaitingChannel,
    };

class PaymentState {
  const PaymentState({
    required this.phase,
    this.order,
    this.elapsed = Duration.zero,
    this.attempts = 0,
  });

  final PaymentPhase phase;
  final PayOrderInfo? order;
  final Duration elapsed;
  final int attempts;

  PaymentState copyWith({
    PaymentPhase? phase,
    PayOrderInfo? order,
    Duration? elapsed,
    int? attempts,
  }) => PaymentState(
    phase: phase ?? this.phase,
    order: order ?? this.order,
    elapsed: elapsed ?? this.elapsed,
    attempts: attempts ?? this.attempts,
  );
}

/// 支付状态机。
///
/// 只有一条真理：**服务端订单状态**。渠道回调、用户点「我已支付」、从后台切回来——
/// 这些统统只是「该查一次了」的信号，不改变状态本身。
///
/// 同一时刻只会有一笔支付在进行，所以这里是普通 Notifier 而不是 family：
/// 与 `chat_controller.dart` 的 `activeConversationProvider` 是同一个路子。
/// 页面负责在 dispose 时调 [stop]——停表这件事不能指望 provider 的生命周期，
/// 那会随 Riverpod 版本而变。
class PaymentController extends Notifier<PaymentState> {
  Timer? _timer;

  /// 已等待时长。
  ///
  /// **按已调度的轮询间隔累计，不用 Stopwatch。** 墙上时钟在 widget test 里不受
  /// `tester.pump` 控制，用它会让「30 秒超时」这条分支根本测不到——而那正是
  /// 掉单（H9）唯一的入口。
  Duration _elapsed = Duration.zero;

  String _orderNo = '';
  bool _stopped = false;

  bool get _ios => !kIsWeb && Platform.isIOS;

  @override
  PaymentState build() => const PaymentState(phase: PaymentPhase.launching);

  /// 拉起渠道后开始轮询。[adapter] 由页面按平台注入。
  Future<void> start(PayChannelAdapter adapter, PendingOrder pending) async {
    _reset(pending.orderNo);
    state = PaymentState(
      phase: _ios ? PaymentPhase.verifying : PaymentPhase.awaitingChannel,
    );
    Log.i(LogTag.biz, 'pay_launch', fields: {'order': pending.orderNo});

    final outcome = await adapter.launch(pending);
    if (outcome == ChannelOutcome.failed) {
      // 渠道明确失败也不直接下结论：服务端可能已经入账（回调先到）。查一次再说。
      Log.w(
        LogTag.biz,
        'pay_channel_failed',
        fields: {'order': pending.orderNo},
      );
    }
    await pollNow();
    _schedule();
  }

  /// 接管一笔已经存在的订单（从充值记录深链进来时）。
  Future<void> resume(String orderNo) async {
    _reset(orderNo);
    state = PaymentState(
      phase: _ios ? PaymentPhase.verifying : PaymentPhase.awaitingChannel,
    );
    await pollNow();
    _schedule();
  }

  /// 立刻查一次。用户点「已完成支付」、从后台切回来、H9 点「主动查单」都走这里。
  Future<void> pollNow() async {
    if (_stopped || _orderNo.isEmpty) return;
    try {
      final info = await ref.read(walletRepoProvider).orderStatus(_orderNo);
      if (_stopped) return;
      state = state.copyWith(
        phase: phaseFor(info.status, ios: _ios),
        order: info,
        elapsed: _elapsed,
        attempts: state.attempts + 1,
      );
      if (info.status.isTerminal) {
        _halt();
        if (info.status == OrderStatus.paid) {
          // 到账了才刷钱包，顺带让流水页下次进入是新的。
          await ref.read(walletProvider.notifier).refresh();
          ref.invalidate(walletTxnsProvider);
        }
        Log.i(
          LogTag.biz,
          'pay_settled',
          fields: {'order': _orderNo, 'status': info.status.name},
        );
        return;
      }
    } catch (_) {
      // 查单失败不改变阶段：网络抖一下不该让用户以为钱丢了，继续退避重试。
      if (_stopped) return;
      state = state.copyWith(elapsed: _elapsed, attempts: state.attempts + 1);
      Log.w(LogTag.biz, 'pay_poll_failed', fields: {'order': _orderNo});
    }
    _checkTimeout();
  }

  /// 从后台切回来：立刻查一次并把退避重置。
  ///
  /// 用户从渠道 App 切回来的那一刻，是状态最可能已经变了的时刻。
  /// 等退避周期到期是白等，而那几秒正是他盯着转圈的几秒。
  void onResumed() {
    if (_stopped || _orderNo.isEmpty) return;
    state = state.copyWith(attempts: 0);
    _timer?.cancel();
    unawaited(pollNow().then((_) => _schedule()));
  }

  /// 用户从「支付中」点「遇到问题」：直接认掉单，别让他一直等。
  void giveUp() {
    if (_stopped) return;
    _halt();
    state = state.copyWith(phase: PaymentPhase.dropped);
    Log.w(LogTag.biz, 'pay_given_up', fields: {'order': _orderNo});
  }

  /// 页面销毁时调。停掉计时器，避免定时器活过页面。
  void stop() => _halt();

  void _reset(String orderNo) {
    _halt();
    _orderNo = orderNo;
    _stopped = false;
    _elapsed = Duration.zero;
  }

  void _schedule() {
    if (_stopped) return;
    _timer?.cancel();
    final delay = pollDelay(state.attempts);
    _timer = Timer(delay, () {
      _elapsed += delay;
      unawaited(pollNow().then((_) => _schedule()));
    });
  }

  void _checkTimeout() {
    if (_stopped || state.phase == PaymentPhase.succeeded) return;
    if (hasTimedOut(_elapsed)) {
      _halt();
      state = state.copyWith(phase: PaymentPhase.dropped);
      Log.w(LogTag.biz, 'pay_dropped', fields: {'order': _orderNo});
    }
  }

  void _halt() {
    _stopped = true;
    _timer?.cancel();
    _timer = null;
  }
}

final paymentControllerProvider =
    NotifierProvider<PaymentController, PaymentState>(PaymentController.new);
