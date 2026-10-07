import 'package:bottles/core/pay/pay_channel.dart';
import 'package:bottles/domain/models/wallet.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const order = PendingOrder(orderNo: 'NO1', payParams: {'mock': true});

  test('choosing success settles the order as paid', () async {
    String? settled;
    bool? asPaid;
    final adapter = MockChannelAdapter(
      settle: (no, ok) async {
        settled = no;
        asPaid = ok;
      },
      ask: () async => MockChoice.success,
    );

    expect(await adapter.launch(order), ChannelOutcome.launched);
    expect(settled, 'NO1');
    expect(asPaid, isTrue);
  });

  test('choosing failure settles the order as failed', () async {
    bool? asPaid;
    final adapter = MockChannelAdapter(
      settle: (_, ok) async => asPaid = ok,
      ask: () async => MockChoice.fail,
    );

    expect(await adapter.launch(order), ChannelOutcome.launched);
    expect(asPaid, isFalse);
  });

  // 「不返回结果」就是掉单现场:什么都不结算,让轮询自己超时到 H9。
  test('choosing no response settles nothing, so the poller can time out',
      () async {
    var called = false;
    final adapter = MockChannelAdapter(
      settle: (_, _) async => called = true,
      ask: () async => MockChoice.noResponse,
    );

    expect(await adapter.launch(order), ChannelOutcome.launched);
    expect(called, isFalse);
  });

  test('dismissing the sheet counts as cancelled', () async {
    final adapter = MockChannelAdapter(
      settle: (_, _) async {},
      ask: () async => null,
    );

    expect(await adapter.launch(order), ChannelOutcome.cancelled);
  });
}
