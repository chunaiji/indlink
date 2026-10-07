import '../../core/utils/json_parse.dart';
import 'user.dart';

enum MessageType { text, image, gift, system }

/// 礼物。送礼三处（聊天 / 动态 / 用户主页）共用一份定义。
/// 道具流水一行：买进背包 / 送出 / 收到。送出一件记一行，用背包抵扣的也记——
/// 用户要看的是「我的礼物去哪了」，不只是钱。
class ItemRecord {
  const ItemRecord({
    required this.id,
    required this.kind,
    required this.itemName,
    required this.itemIcon,
    required this.coins,
    required this.peerName,
    required this.at,
  });

  final String id;

  /// 'buy' | 'sent' | 'received'
  final String kind;
  final String itemName;
  final String itemIcon;
  final int coins;
  final String peerName;
  final DateTime at;

  factory ItemRecord.fromJson(Map<String, dynamic> j) => ItemRecord(
    id: idOf(j['id']),
    kind: (j['kind'] ?? '') as String,
    itemName: (j['item_name'] ?? '') as String,
    itemIcon: (j['item_icon'] ?? '') as String,
    coins: intOf(j['coins']),
    peerName: (j['peer_name'] ?? '') as String,
    at: utcOrEpoch(j['created_at']),
  );
}

class GiftItem {
  const GiftItem({
    required this.id,
    required this.name,
    required this.emoji,
    required this.coins,
    this.charm = 0,
  });

  final String id;
  final String name;

  /// 图标。**可能是 emoji 字符，也可能是一个图片 URL** ——
  /// 线上 `items.icon` 存的是 `https://…/static/gifts/shell.png` 这类地址。
  /// 别直接当文本渲染：那样界面上会出现一长串 URL（看起来就是「乱码」）。
  /// 用 [isImageIcon] 分流。
  final String emoji;

  final int coins;

  /// 送出后累加到对方 `User.Charm`。
  final int charm;

  /// 图标是图片地址还是 emoji。
  bool get isImageIcon =>
      emoji.startsWith('http://') ||
      emoji.startsWith('https://') ||
      emoji.startsWith('/');

  factory GiftItem.fromJson(Map<String, dynamic> j) => GiftItem(
    id: idOf(j['id'] ?? j['item_id']),
    name: (j['name'] ?? '') as String,
    emoji: (j['icon'] ?? '🎁') as String,
    coins: intOf(j['coins'] ?? j['price'] ?? j['price_coin']),
    // charm 加回退:后端 appdto 里 charm==price_coin,若某分支没下发 charm 字段,
    // 退到 coins/price_coin 仍是正确值,避免聊天礼物气泡显示「魅力 +0」。
    charm: intOf(j['charm'] ?? j['coins'] ?? j['price_coin']),
  );
}

class ChatMessage {
  const ChatMessage({
    required this.id,
    required this.fromMe,
    required this.type,
    required this.at,
    this.conversationId,
    this.text = '',
    this.imageUrl,
    this.gift,
    this.giftQty = 1,
    this.read = false,
    this.sending = false,
    this.failed = false,
  });

  final String id;

  /// 所属会话号。来自 WS 帧顶层的 `chat_id`(见 chat_socket)——常驻分发器据此把
  /// 消息落到正确会话,不再"张冠李戴"贴到当前打开的会话。历史消息(按会话查)可为空。
  final String? conversationId;

  final bool fromMe;
  final MessageType type;
  final DateTime at;
  final String text;
  final String? imageUrl;
  final GiftItem? gift;

  /// 连送 ×10 合并成一条，否则刷屏。
  final int giftQty;

  final bool read;

  /// 乐观发送：先上屏再等 ACK，失败可重发。
  final bool sending;
  final bool failed;

  ChatMessage copyWith({
    bool? read,
    bool? sending,
    bool? failed,
    String? id,
    String? conversationId,
  }) {
    return ChatMessage(
      id: id ?? this.id,
      conversationId: conversationId ?? this.conversationId,
      fromMe: fromMe,
      type: type,
      at: at,
      text: text,
      imageUrl: imageUrl,
      gift: gift,
      giftQty: giftQty,
      read: read ?? this.read,
      sending: sending ?? this.sending,
      failed: failed ?? this.failed,
    );
  }

  factory ChatMessage.fromJson(
    Map<String, dynamic> j, {
    required String myId,
    String? conversationId,
  }) {
    final giftJson = j['gift'] as Map<String, dynamic>?;
    return ChatMessage(
      id: idOf(j['id']),
      // 会话号在 WS 帧顶层(chat_id),由调用方传入;消息对象自身也可能带,兜底取一次。
      conversationId:
          conversationId ?? (j['chat_id'] == null ? null : idOf(j['chat_id'])),
      fromMe: idOf(j['from_id']) == myId,
      type: switch (j['type']) {
        'image' => MessageType.image,
        'gift' => MessageType.gift,
        'system' => MessageType.system,
        _ => MessageType.text,
      },
      at: utcOrEpoch(j['created_at']),
      text: (j['content'] ?? '') as String,
      imageUrl: j['image'] as String?,
      gift: giftJson == null ? null : GiftItem.fromJson(giftJson),
      giftQty: intOf(j['qty'], 1),
      read: j['read_status'] == 1 || j['read'] == true,
    );
  }
}

class Conversation {
  const Conversation({
    required this.id,
    required this.peer,
    required this.lastAt,
    this.lastText = '',
    this.lastType = MessageType.text,
    this.unread = 0,
    this.fromBottle = false,
  });

  final String id;
  final UserBrief peer;
  final DateTime lastAt;
  final String lastText;
  final MessageType lastType;
  final int unread;

  /// 「你们从一个瓶子开始聊天」——瓶友会话的来源标记。
  final bool fromBottle;

  /// 对方注销后会话只读。
  bool get readOnly => peer.deleted;

  Conversation copyWith({
    String? lastText,
    MessageType? lastType,
    DateTime? lastAt,
    int? unread,
  }) {
    return Conversation(
      id: id,
      peer: peer,
      lastAt: lastAt ?? this.lastAt,
      lastText: lastText ?? this.lastText,
      lastType: lastType ?? this.lastType,
      unread: unread ?? this.unread,
      fromBottle: fromBottle,
    );
  }

  factory Conversation.fromJson(Map<String, dynamic> j) => Conversation(
    id: idOf(j['id']),
    peer: UserBrief.fromJson(
      (j['peer'] ?? const <String, dynamic>{}) as Map<String, dynamic>,
    ),
    lastAt: utcOrEpoch(j['last_at']),
    lastText: (j['last_message'] ?? '') as String,
    lastType: switch (j['last_type']) {
      'image' => MessageType.image,
      'gift' => MessageType.gift,
      _ => MessageType.text,
    },
    unread: intOf(j['unread']),
    fromBottle: j['from_bottle'] == true,
  );
}

/// 举报理由，与后台 `Report.Reason` 对齐。
enum ReportReason {
  porn,
  spam,
  harassment,
  minor;

  String get wire => name;
}
