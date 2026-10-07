import '../../core/utils/json_parse.dart';

/// 流水场景，与后端 `WalletTxn.Scene` 一一对应。
enum TxnScene {
  unlock,
  chat,
  msg,
  gift,
  recharge,
  reward,
  checkin,
  share,
  adReward,

  /// 发现页撤回：把刚左滑掉的人捞回来。
  rewind,

  /// 发现页左滑跳过扣币（付费功能，默认关）。服务端 `wallet.SceneDiscoverSkip`。
  discoverSkip,

  /// 充值被渠道退款后扣回。服务端 `wallet.SceneRefund`（`wallet/service.go:29`）。
  ///
  /// 少了这一支，退款流水会掉进兜底的 [reward]——H11 里「退款 −300」会显示成「奖励」。
  refund;

  static TxnScene parse(Object? v) => switch (v) {
        'unlock' => TxnScene.unlock,
        'chat' => TxnScene.chat,
        'msg' => TxnScene.msg,
        'gift' => TxnScene.gift,
        'recharge' => TxnScene.recharge,
        'refund' => TxnScene.refund,
        'checkin' => TxnScene.checkin,
        'share' => TxnScene.share,
        'ad_reward' => TxnScene.adReward,
        'rewind' => TxnScene.rewind,
        'discover_skip' => TxnScene.discoverSkip,
        _ => TxnScene.reward,
      };

  String get wire => switch (this) {
        TxnScene.adReward => 'ad_reward',
        TxnScene.discoverSkip => 'discover_skip',
        _ => name,
      };
}

class WalletTxn {
  const WalletTxn({
    required this.id,
    required this.scene,
    required this.amount,
    required this.at,
    this.balanceAfter = 0,
    this.note,
  });

  final String id;
  final TxnScene scene;

  /// 正数入账、负数出账。
  final int amount;

  final DateTime at;

  /// `WalletTxn.BalanceAfter` 逐条记录变动后余额，流水天然可对账。
  final int balanceAfter;

  final String? note;

  bool get isIncome => amount > 0;

  factory WalletTxn.fromJson(Map<String, dynamic> j) => WalletTxn(
        id: idOf(j['id']),
        scene: TxnScene.parse(j['scene']),
        amount: intOf(j['amount']),
        at: utcOrEpoch(j['created_at']),
        balanceAfter: intOf(j['balance_after']),
        note: j['note'] as String?,
      );
}

enum PayChannel {
  upi,
  card,

  /// iOS 构建下整屏替换为 IAP 商品列表，第三方渠道全部不展示。
  iap,
}

class RechargePackage {
  const RechargePackage({
    required this.id,
    required this.coins,
    required this.priceLabel,
    this.originalPriceLabel,
    this.bonusPercent = 0,
  });

  final String id;
  final int coins;
  final String priceLabel;
  final String? originalPriceLabel;
  final int bonusPercent;

  /// ⚠️ 这一组**没有后端 DTO**：`GET /pay/packages` 不挂鉴权，拿不到 platform
  /// 分派不了，所以在这里兼容后端的原始形状（`package_id` / `price_fen` / 分开的赠币）。
  ///
  /// `price_fen` 是「分」，沿用小程序的人民币口径——**印度上线前要改币种**。
  factory RechargePackage.fromJson(Map<String, dynamic> j) {
    final base = intOf(j['coins']);
    final bonus = intOf(j['bonus_coins']);
    final fen = intOf(j['price_fen']);
    return RechargePackage(
      id: idOf(j['id'] ?? j['package_id']),
      // 展示的是到账总数，不是基础币数。
      coins: base + bonus,
      priceLabel: (j['price_label'] as String?) ??
          (fen > 0 ? '¥${(fen / 100).toStringAsFixed(2)}' : ''),
      originalPriceLabel: j['original_price_label'] as String?,
      bonusPercent: j['bonus_percent'] != null
          ? intOf(j['bonus_percent'])
          : (base > 0 ? (bonus * 100 ~/ base) : 0),
    );
  }
}

/// 下单结果。`payParams` 原样透传给渠道适配器，服务端给哪个渠道就由谁认。
class PendingOrder {
  const PendingOrder({required this.orderNo, required this.payParams});

  final String orderNo;
  final Map<String, dynamic> payParams;

  /// 服务端的 mock 渠道会在 pay_params 里打这个标记（`pay/driver_mock.go`）。
  bool get isMock => payParams['mock'] == true;
}

/// 服务端订单状态。**它是支付成功与否的唯一判据**——渠道说什么都不算。
enum OrderStatus {
  pending,
  paid,
  failed,
  refunded;

  /// 未知取值一律当 pending：继续轮询好过谎报成功。
  static OrderStatus parse(Object? v) => switch (v) {
        'paid' => OrderStatus.paid,
        'failed' => OrderStatus.failed,
        'refunded' => OrderStatus.refunded,
        _ => OrderStatus.pending,
      };

  /// 终态才停轮询。
  bool get isTerminal => this != OrderStatus.pending;
}

class PayOrderInfo {
  const PayOrderInfo({
    required this.orderNo,
    required this.status,
    required this.coins,
    required this.priceMinor,
    required this.currency,
    required this.platform,
    required this.createdAt,
    this.paidAt,
    this.rawPriceLabel,
  });

  final String orderNo;
  final OrderStatus status;
  final int coins;

  /// 最小货币单位（分 / paise / cent）。
  ///
  /// 服务端的 JSON 名是 `price_fen`，那个名字**不许改**——线上小程序订单页
  /// 直接读它（`model.go:318`），改了立刻显示 `¥NaN`。Go 侧字段名已经改成
  /// `PriceMinor` 了，误导人的那一半已经处理过。
  final int priceMinor;
  final String currency;
  final String platform;
  final DateTime createdAt;
  final DateTime? paidAt;

  /// 只有 mock 渠道会直接给标签（它的档位来自 [RechargePackage.priceLabel]，
  /// 那里本来就只有标签没有数值）。真实订单走 [priceMinor] 自己格式化。
  final String? rawPriceLabel;

  static const _symbols = {'CNY': '¥', 'INR': '₹', 'USD': '\$', 'EUR': '€'};

  String get priceLabel {
    final raw = rawPriceLabel;
    if (raw != null) return raw;
    // currency 为空 = CNY，那是改名之前的历史数据（`model.go:328`）。
    final symbol = _symbols[currency.isEmpty ? 'CNY' : currency] ?? '$currency ';
    return '$symbol${(priceMinor / 100).toStringAsFixed(2)}';
  }

  factory PayOrderInfo.fromJson(Map<String, dynamic> j) => PayOrderInfo(
        orderNo: (j['order_no'] ?? '').toString(),
        status: OrderStatus.parse(j['status']),
        coins: intOf(j['coins']),
        priceMinor: intOf(j['price_fen']),
        currency: (j['currency'] ?? '').toString(),
        platform: (j['platform'] ?? '').toString(),
        createdAt: utcOrEpoch(j['created_at']),
        paidAt: j['paid_at'] == null ? null : utcOrEpoch(j['paid_at']),
        rawPriceLabel: j['price_label'] as String?,
      );
}
