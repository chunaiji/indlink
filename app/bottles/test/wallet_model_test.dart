import 'package:bottles/domain/models/wallet.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  // 服务端有 SceneRefund = "refund"(wallet/service.go:29),客户端此前没有这一支,
  // 退款流水会掉进兜底的 reward——H11 里会把「退款 −300」显示成「奖励」。
  test('refund txns are not mistaken for rewards', () {
    expect(TxnScene.parse('refund'), TxnScene.refund);
    expect(TxnScene.refund.wire, 'refund');
  });

  test('order status parses the four server states', () {
    expect(OrderStatus.parse('pending'), OrderStatus.pending);
    expect(OrderStatus.parse('paid'), OrderStatus.paid);
    expect(OrderStatus.parse('failed'), OrderStatus.failed);
    expect(OrderStatus.parse('refunded'), OrderStatus.refunded);
    // 未知状态当作 pending:继续轮询好过谎报成功。
    expect(OrderStatus.parse('weird'), OrderStatus.pending);
    expect(OrderStatus.parse(null), OrderStatus.pending);
  });

  test('only paid/failed/refunded end the polling loop', () {
    expect(OrderStatus.pending.isTerminal, isFalse);
    expect(OrderStatus.paid.isTerminal, isTrue);
    expect(OrderStatus.failed.isTerminal, isTrue);
    expect(OrderStatus.refunded.isTerminal, isTrue);
  });

  test('a pending order knows whether the mock channel produced it', () {
    expect(
      const PendingOrder(orderNo: 'A', payParams: {'mock': true}).isMock,
      isTrue,
    );
    expect(const PendingOrder(orderNo: 'A', payParams: {}).isMock, isFalse);
  });
}
