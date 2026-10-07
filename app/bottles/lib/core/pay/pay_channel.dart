import '../../domain/models/wallet.dart';

/// 渠道拉起的结果。
///
/// ⚠️ 它**不代表支付成功**——成功与否一律以服务端订单状态为准，渠道说什么都不算。
/// 渠道说成功而服务端没入账，给用户发币就是白送；渠道说失败而服务端已入账，
/// 报失败会引来一通「我明明付了」的客诉。两边不一致时，服务端说了算。
enum ChannelOutcome { launched, cancelled, failed }

/// 渠道适配器。
///
/// 本轮只有 [MockChannelAdapter]。接入 Google Play Billing 与 StoreKit 时
/// 各实现一份，**支付状态机与六个屏幕的代码都不用改**——它们只认
/// [ChannelOutcome] 与服务端订单状态。
abstract class PayChannelAdapter {
  /// 拉起渠道。返回后进入轮询，由服务端订单状态定生死。
  Future<ChannelOutcome> launch(PendingOrder order);
}

/// 联调时让人自己选结果。
enum MockChoice { success, fail, noResponse }

/// 假渠道适配器。
///
/// 三个选项对应三条必须测到的路径：成功（H8）、失败、以及**什么都不回**——
/// 最后一个正是掉单（H9）的现场，也是真实渠道里最难复现的一种。
class MockChannelAdapter implements PayChannelAdapter {
  const MockChannelAdapter({required this.settle, required this.ask});

  /// 调 `POST /pay/mock/settle`。
  final Future<void> Function(String orderNo, bool success) settle;

  /// 弹出三选一。返回 null 表示用户把面板关掉了。
  final Future<MockChoice?> Function() ask;

  @override
  Future<ChannelOutcome> launch(PendingOrder order) async {
    final choice = await ask();
    if (choice == null) return ChannelOutcome.cancelled;
    switch (choice) {
      case MockChoice.success:
        await settle(order.orderNo, true);
      case MockChoice.fail:
        await settle(order.orderNo, false);
      case MockChoice.noResponse:
        break; // 故意什么都不做
    }
    return ChannelOutcome.launched;
  }
}
