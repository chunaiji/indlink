import 'package:bottles/domain/models/wallet.dart';
import 'package:bottles/features/me/payment_controller.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('poll backoff', () {
    // 1 → 2 → 4 → 8 秒封顶。开头密是因为多数支付在几秒内就有结果,
    // 封顶是因为再稀就不如让用户自己点「已完成支付」。
    test('doubles up to eight seconds and then stays there', () {
      expect(pollDelay(0), const Duration(seconds: 1));
      expect(pollDelay(1), const Duration(seconds: 2));
      expect(pollDelay(2), const Duration(seconds: 4));
      expect(pollDelay(3), const Duration(seconds: 8));
      expect(pollDelay(4), const Duration(seconds: 8));
      expect(pollDelay(99), const Duration(seconds: 8));
    });
  });

  group('phase mapping', () {
    test('pending shows the platform-specific waiting screen', () {
      expect(
        phaseFor(OrderStatus.pending, ios: false),
        PaymentPhase.awaitingChannel,
      );
      expect(phaseFor(OrderStatus.pending, ios: true), PaymentPhase.verifying);
    });

    test('terminal statuses map to their own screens', () {
      expect(phaseFor(OrderStatus.paid, ios: false), PaymentPhase.succeeded);
      expect(phaseFor(OrderStatus.failed, ios: false), PaymentPhase.failed);
      // 退款不是支付失败,它有自己的一屏(H11),且要解释负余额。
      expect(phaseFor(OrderStatus.refunded, ios: false), PaymentPhase.refunded);
    });
  });

  group('timeout', () {
    // 原型明示「不能一直挂着」。30 秒之后继续转圈只会让用户重复支付。
    test('gives up after thirty seconds', () {
      expect(hasTimedOut(const Duration(seconds: 29)), isFalse);
      expect(hasTimedOut(const Duration(seconds: 30)), isTrue);
      expect(hasTimedOut(const Duration(seconds: 31)), isTrue);
    });
  });
}
