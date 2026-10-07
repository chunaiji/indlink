/// 一次「碰撞出火花」事件（WS `type: "spark"`）。
///
/// 服务端中英文各推一份：WS 连接上没有 Accept-Language，它不知道这个客户端
/// 是什么语种，所以两份都给，由这里按 App 当前 locale 挑。
class SparkEvent {
  const SparkEvent({
    required this.chatId,
    required this.peerId,
    required this.nickname,
    required this.avatar,
    required this.title,
    required this.body,
  });

  /// 机器人配对才有：会话已建好、开场白也发了，点击直接跳。
  /// 真人配对为 null，要先调 `/spark/accept` 换一个 chat_id。
  final String? chatId;
  final String peerId;
  final String nickname;
  final String avatar;
  final String title;
  final String body;

  bool get hasChat => (chatId ?? '').isNotEmpty;

  static const _fallbackTitleZh = '有人和你对上眼了';
  static const _fallbackBodyZh = '{nickname} 与你碰撞出了火花';
  static const _fallbackTitleEn = 'Someone caught your eye';
  static const _fallbackBodyEn = 'You and {nickname} just sparked';

  factory SparkEvent.fromJson(Map<String, Object?> j, {required bool english}) {
    final peer = (j['peer'] as Map?)?.cast<String, Object?>() ?? const {};
    final nickname = '${peer['nickname'] ?? ''}';
    String pick(String key, String enKey, String fallback) {
      final v = '${j[english ? enKey : key] ?? ''}';
      final tpl = v.trim().isEmpty ? fallback : v;
      return tpl.replaceAll('{nickname}', nickname);
    }

    final chat = '${j['chat_id'] ?? ''}';
    return SparkEvent(
      chatId: chat.isEmpty ? null : chat,
      peerId: '${j['peer_id'] ?? ''}',
      nickname: nickname,
      avatar: '${peer['avatar'] ?? ''}',
      title: pick('title', 'title_en', english ? _fallbackTitleEn : _fallbackTitleZh),
      body: pick('text', 'text_en', english ? _fallbackBodyEn : _fallbackBodyZh),
    );
  }
}
