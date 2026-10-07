import '../../core/utils/json_parse.dart';
import 'user.dart';

/// 「我的瓶子」三个分页。
enum BottleMineTab { thrown, scooped, collected }

enum BottleStatus {
  active,
  expired,
  deleted;

  static BottleStatus parse(Object? v) => switch (v) {
        'expired' => BottleStatus.expired,
        'deleted' => BottleStatus.deleted,
        _ => BottleStatus.active,
      };
}

/// 漂流轨迹节点的三种事件，由 `MatchLog`（action=view/reply/like/skip）聚合而来，
/// 不是独立表。
enum TraceKind { thrown, seen, replied }

class BottleTraceNode {
  const BottleTraceNode({
    required this.kind,
    required this.city,
    required this.at,
    this.count = 0,
  });

  final TraceKind kind;
  final String city;
  final DateTime at;
  final int count;

  factory BottleTraceNode.fromJson(Map<String, dynamic> j) => BottleTraceNode(
        kind: switch (j['kind']) {
          'thrown' => TraceKind.thrown,
          'replied' => TraceKind.replied,
          _ => TraceKind.seen,
        },
        city: (j['city'] ?? '') as String,
        at: utcOrEpoch(j['at']),
        count: intOf(j['count']),
      );
}

class Bottle {
  const Bottle({
    required this.id,
    required this.content,
    required this.author,
    required this.createdAt,
    this.images = const [],
    this.tags = const [],
    this.city,
    this.expireAt,
    this.viewCount = 0,
    this.replyCount = 0,
    this.likeCount = 0,
    this.status = BottleStatus.active,
    this.cityCount = 1,
    this.liked = false,
    this.collected = false,
    this.trace = const [],
  });

  final String id;
  final String content;

  /// 捞到的瓶子只给匿名摘要（「匿名 · 女 24」），不给真实昵称。
  final UserBrief author;

  final DateTime createdAt;
  final List<String> images;
  final List<String> tags;
  final String? city;
  final DateTime? expireAt;
  final int viewCount;
  final int replyCount;
  final int likeCount;
  final BottleStatus status;

  /// 轨迹摘要里的「跨越 N 座城市」。
  final int cityCount;

  /// 被喜欢过（对应 `MatchLog(action=like)`），用于统计行的 ♡ 数。
  final bool liked;

  /// 已收藏 —— 与 [liked] 是两件事：收藏进「我的瓶子 · 已收藏」分页。
  final bool collected;

  /// 首页点瓶子浮出的轨迹预览，直接读 feed 里已有的摘要，不额外请求。
  final List<BottleTraceNode> trace;

  Bottle copyWith({bool? collected, bool? liked}) => Bottle(
        id: id,
        content: content,
        author: author,
        createdAt: createdAt,
        images: images,
        tags: tags,
        city: city,
        expireAt: expireAt,
        viewCount: viewCount,
        replyCount: replyCount,
        likeCount: likeCount,
        status: status,
        cityCount: cityCount,
        liked: liked ?? this.liked,
        collected: collected ?? this.collected,
        trace: trace,
      );

  /// 夜场瓶只在夜场时段可捞，后端按 night 标签过滤。
  bool get isNight => tags.contains('night');

  /// 「漂了 3 天」是时长口径，和回信的「12 分钟前」时刻口径不能混。
  int get driftedDays =>
      DateTime.now().toUtc().difference(createdAt).inDays.clamp(0, 9999);

  factory Bottle.fromJson(Map<String, dynamic> j) => Bottle(
        id: idOf(j['id']),
        content: (j['content'] ?? '') as String,
        author: UserBrief.fromJson(
          (j['author'] ?? j['user'] ?? const <String, dynamic>{})
              as Map<String, dynamic>,
        ),
        createdAt: utcOrEpoch(j['created_at']),
        images: stringList(j['images']),
        tags: stringList(j['tags']),
        city: j['city'] as String?,
        expireAt: utcOrNull(j['expire_at']),
        viewCount: intOf(j['view_count']),
        replyCount: intOf(j['reply_count']),
        likeCount: intOf(j['like_count']),
        status: BottleStatus.parse(j['status']),
        cityCount: intOf(j['city_count'], 1),
        liked: j['liked'] == true,
        collected: j['collected'] == true,
        trace: [
          for (final t in (j['trace'] as List? ?? const []))
            BottleTraceNode.fromJson(t as Map<String, dynamic>),
        ],
      );
}

class BottleReply {
  const BottleReply({
    required this.id,
    required this.author,
    required this.content,
    required this.createdAt,
    this.locked = true,
    this.unlockCost = 0,
  });

  final String id;
  final UserBrief author;
  final String content;
  final DateTime createdAt;

  /// `ReplyUnlock` 按 viewer 维度记录——任何人付费即可看，不只瓶主。
  final bool locked;

  /// 解锁这一条要花多少金币（`price_unlock`，后台可调）。
  /// 界面上的价格必须来自这里，写死的数字和服务端真扣的对不上就是欺诈。
  final int unlockCost;

  /// ⚠️ 后端 `ReplyView` 是**平铺**的，没有嵌套 author：
  /// `{reply_id, user_id, nickname, avatar, content, locked, unlock_cost}`。
  ///
  /// 之前按 `{id, author:{}, unlocked}` 解析，三个字段全落空：
  /// id 为空 → 解锁请求发不出去；author 为空 → 回信全变「无名氏 + 破头像」；
  /// `unlocked` 键根本不存在 → `locked` 恒为 true，连瓶主看自己的回信都是锁着的。
  factory BottleReply.fromJson(Map<String, dynamic> j) {
    final nested = j['author'];
    return BottleReply(
      id: idOf(j['reply_id'] ?? j['id']),
      author: nested is Map
          ? UserBrief.fromJson(Map<String, dynamic>.from(nested))
          : UserBrief(
              id: idOf(j['user_id']),
              nickname: (j['nickname'] ?? '') as String,
              avatar: j['avatar'] as String?,
            ),
      content: (j['content'] ?? '') as String,
      createdAt: utcOrEpoch(j['created_at']),
      // 缺字段时按「锁着」处理：误判成解锁等于把付费内容白送。
      locked: j.containsKey('locked') ? j['locked'] == true : j['unlocked'] != true,
      unlockCost: intOf(j['unlock_cost']),
    );
  }

  BottleReply unlockedCopy() => BottleReply(
        id: id,
        author: author,
        content: content,
        createdAt: createdAt,
        locked: false,
        unlockCost: unlockCost,
      );
}

/// 每日免费次数。前端拿它做预判，真正的限流在后端 `throwLimit` 中间件。
class QuotaInfo {
  const QuotaInfo({
    required this.throwLeft,
    required this.throwTotal,
    required this.scoopLeft,
    required this.scoopTotal,
  });

  final int throwLeft;
  final int throwTotal;
  final int scoopLeft;
  final int scoopTotal;

  bool get canThrow => throwLeft > 0;
  bool get canScoop => scoopLeft > 0;

  QuotaInfo consumeThrow() => QuotaInfo(
        throwLeft: (throwLeft - 1).clamp(0, throwTotal),
        throwTotal: throwTotal,
        scoopLeft: scoopLeft,
        scoopTotal: scoopTotal,
      );

  QuotaInfo consumeScoop() => QuotaInfo(
        throwLeft: throwLeft,
        throwTotal: throwTotal,
        scoopLeft: (scoopLeft - 1).clamp(0, scoopTotal),
        scoopTotal: scoopTotal,
      );

  QuotaInfo addScoop(int n) => QuotaInfo(
        throwLeft: throwLeft,
        throwTotal: throwTotal,
        scoopLeft: scoopLeft + n,
        scoopTotal: scoopTotal + n,
      );

  factory QuotaInfo.fromJson(Map<String, dynamic> j) => QuotaInfo(
        throwLeft: intOf(j['throw_left']),
        throwTotal: intOf(j['throw_total']),
        scoopLeft: intOf(j['scoop_left']),
        scoopTotal: intOf(j['scoop_total']),
      );
}

/// 首页头部的海况摘要。
class SeaCondition {
  const SeaCondition({
    required this.repliesDrifting,
    required this.onlineCount,
    required this.nightStartsAt,
  });

  final int repliesDrifting;
  final int onlineCount;

  /// 夜场开场时刻（本地时区），默认 21:00。
  final DateTime nightStartsAt;
}
