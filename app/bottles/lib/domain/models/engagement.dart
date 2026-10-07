import '../../core/utils/json_parse.dart';
import 'user.dart';

enum NotifyKind { reply, gift, like, comment, system }

class NotificationItem {
  const NotificationItem({
    required this.id,
    required this.kind,
    required this.title,
    required this.at,
    this.body = '',
    this.read = false,
  });

  final String id;
  final NotifyKind kind;

  /// 后端改为「写模板键、读时按 Accept-Language 渲染」后，这里拿到的已是渲染结果。
  final String title;

  final DateTime at;
  final String body;
  final bool read;

  NotificationItem markRead() => NotificationItem(
        id: id,
        kind: kind,
        title: title,
        at: at,
        body: body,
        read: true,
      );

  factory NotificationItem.fromJson(Map<String, dynamic> j) => NotificationItem(
        id: idOf(j['id']),
        kind: switch (j['kind'] ?? j['type']) {
          'gift' => NotifyKind.gift,
          'like' => NotifyKind.like,
          'comment' => NotifyKind.comment,
          'reply' => NotifyKind.reply,
          _ => NotifyKind.system,
        },
        title: (j['title'] ?? '') as String,
        at: utcOrEpoch(j['created_at']),
        body: (j['body'] ?? '') as String,
        read: j['read'] == true,
      );
}

/// 关系列表项。进度条取 [strength]，文案取 [stage]。
class RelationItem {
  const RelationItem({
    required this.user,
    required this.interactionCount,
    required this.strength,
    required this.stage,
  });

  final UserBrief user;
  final int interactionCount;

  /// `Relation.StrengthScore`，0~100，由「互动次数 × 时间衰减」算出。
  final int strength;

  final RelationStage stage;

  factory RelationItem.fromJson(Map<String, dynamic> j) {
    final score = intOf(j['strength_score']);
    return RelationItem(
      user: UserBrief.fromJson(
        (j['user'] ?? const <String, dynamic>{}) as Map<String, dynamic>,
      ),
      interactionCount: intOf(j['interaction_count']),
      strength: score,
      stage: j['stage'] == null
          ? RelationStage.fromScore(score)
          : RelationStage.parse(j['stage']),
    );
  }
}

/// 签到状态。`CheckinLog` 的唯一索引 `(user_id, date)` 保证一人一天一次。
class CheckinStatus {
  const CheckinStatus({
    required this.streak,
    required this.checkedToday,
    required this.todayReward,
    this.cycle = 7,
    this.adDone = 0,
    this.adTotal = 5,
    this.adCoins = 5,
    this.inviteReward = 50,
    this.momentReward = 5,
  });

  final int streak;
  final bool checkedToday;
  final int todayReward;

  /// 一个签到周期的天数，第 [cycle] 天有大奖。
  final int cycle;

  final int adDone;
  final int adTotal;
  final int adCoins;
  final int inviteReward;
  final int momentReward;

  CheckinStatus checkedIn() => CheckinStatus(
        streak: streak + 1,
        checkedToday: true,
        todayReward: todayReward,
        cycle: cycle,
        adDone: adDone,
        adTotal: adTotal,
        adCoins: adCoins,
        inviteReward: inviteReward,
        momentReward: momentReward,
      );

  CheckinStatus watchedAd() => CheckinStatus(
        streak: streak,
        checkedToday: checkedToday,
        todayReward: todayReward,
        cycle: cycle,
        adDone: (adDone + 1).clamp(0, adTotal),
        adTotal: adTotal,
        adCoins: adCoins,
        inviteReward: inviteReward,
        momentReward: momentReward,
      );

  /// ⚠️ 后端 `checkin.StatusResp` 用的是 `signed_today` / `today_coins`，
  /// 与这里原本读的 `checked_today` / `today_reward` 对不上——结果是
  /// **签到奖励恒显示 0、也永远不知道今天签没签**。两种命名都认，mock 不受影响。
  ///
  /// cycle 后端不下发，改由 `ladder` 数组长度推断：阶梯几档就是几天一轮。
  factory CheckinStatus.fromJson(Map<String, dynamic> j) => CheckinStatus(
        streak: intOf(j['streak']),
        checkedToday: j['checked_today'] == true || j['signed_today'] == true,
        todayReward: intOf(j['today_reward'] ?? j['today_coins'] ?? j['coins']),
        cycle: intOf(j['cycle'], (j['ladder'] as List?)?.length ?? 7),
        adDone: intOf(j['ad_done']),
        adTotal: intOf(j['ad_total'], 5),
        adCoins: intOf(j['ad_coins'], 5),
        inviteReward: intOf(j['invite_reward'], 50),
        momentReward: intOf(j['moment_reward'], 5),
      );
}

/// 发现页筛选条件，本地持久化后随请求下发。
class DiscoverFilter {
  const DiscoverFilter({
    this.languages = const [],
    this.interests = const [],
    this.maxDistanceKm = 25,
    this.gender,
    this.minAge = 18,
    this.maxAge = 30,
  });

  final List<String> languages;
  final List<String> interests;

  /// null 表示不限距离。
  final int? maxDistanceKm;

  /// null 表示不限性别。
  final Gender? gender;

  final int minAge;
  final int maxAge;

  DiscoverFilter copyWith({
    List<String>? languages,
    List<String>? interests,
    int? maxDistanceKm,
    bool clearDistance = false,
    Gender? gender,
    bool clearGender = false,
    int? minAge,
    int? maxAge,
  }) {
    return DiscoverFilter(
      languages: languages ?? this.languages,
      interests: interests ?? this.interests,
      maxDistanceKm: clearDistance ? null : (maxDistanceKm ?? this.maxDistanceKm),
      gender: clearGender ? null : (gender ?? this.gender),
      minAge: minAge ?? this.minAge,
      maxAge: maxAge ?? this.maxAge,
    );
  }

  Map<String, dynamic> toJson() => {
        'languages': languages,
        'interests': interests,
        'max_distance_km': maxDistanceKm,
        'gender': gender?.wire,
        'min_age': minAge,
        'max_age': maxAge,
      };

  factory DiscoverFilter.fromJson(Map<String, dynamic> j) => DiscoverFilter(
        languages: stringList(j['languages']),
        interests: stringList(j['interests']),
        maxDistanceKm: (j['max_distance_km'] as num?)?.toInt(),
        gender: j['gender'] == null ? null : Gender.parse(j['gender']),
        minAge: intOf(j['min_age'], 18),
        maxAge: intOf(j['max_age'], 30),
      );
}
