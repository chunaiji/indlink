import '../../core/utils/json_parse.dart';

/// 性别。后端存字符串，这里收敛成枚举避免各页面各判各的。
enum Gender {
  female,
  male,
  secret;

  static Gender parse(Object? v) => switch (v) {
    'female' || 'f' || 2 => Gender.female,
    'male' || 'm' || 1 => Gender.male,
    _ => Gender.secret,
  };

  String get wire => name;
}

/// 关系阶段，对应 `model.Relation.Stage`。
enum RelationStage {
  stranger,
  known,
  familiar;

  static RelationStage parse(Object? v) => switch (v) {
    'known' || 'acquainted' => RelationStage.known,
    'familiar' || 'close' => RelationStage.familiar,
    _ => RelationStage.stranger,
  };

  static RelationStage fromScore(int score) => switch (score) {
    >= 60 => RelationStage.familiar,
    >= 25 => RelationStage.known,
    _ => RelationStage.stranger,
  };
}

/// 一个可选兴趣。key 入库，中英文只管显示。
class InterestOption {
  const InterestOption({required this.key, required this.zh, required this.en});

  final String key;
  final String zh;
  final String en;

  factory InterestOption.fromJson(Map<String, dynamic> j) {
    final key = (j['key'] ?? '') as String;
    return InterestOption(
      key: key,
      zh: (j['zh'] as String?)?.trim().isNotEmpty == true
          ? j['zh'] as String
          : key,
      en: (j['en'] as String?)?.trim().isNotEmpty == true
          ? j['en'] as String
          : key,
    );
  }

  String label({required bool zhLocale}) => zhLocale ? zh : en;
}

/// 「完善资料」页的可选项，来自 `GET /api/profile-options`。
///
/// 语言 / 兴趣以前写死在 `Catalog` 里——运营想加一门语言就得发版。
/// 现在走后台配置；**后台没配就返回空列表，客户端回退到内置表**，
/// 不能因为运营没配过就让新用户卡在注册流程里。
class ProfileOptions {
  const ProfileOptions({
    this.languages = const [],
    this.interests = const [],
    this.minInterests = 3,
    this.minAge = 18,
    this.maxAge = 60,
  });

  final List<String> languages;
  final List<InterestOption> interests;
  final int minInterests;

  /// 合规底线由服务端兜（低于 18 会被抬到 18），这里只管展示与校验。
  final int minAge;
  final int maxAge;

  factory ProfileOptions.fromJson(Map<String, dynamic> j) => ProfileOptions(
    languages: stringList(j['languages']),
    interests: [
      for (final e in (j['interests'] as List? ?? const []))
        if (e is Map) InterestOption.fromJson(Map<String, dynamic>.from(e)),
    ],
    minInterests: intOf(j['min_interests'], 3),
    minAge: intOf(j['min_age'], 18),
    maxAge: intOf(j['max_age'], 60),
  );
}

/// 列表 / 卡片 / 气泡里用到的用户摘要。
class UserBrief {
  const UserBrief({
    required this.id,
    required this.nickname,
    this.avatar,
    this.gender = Gender.secret,
    this.age,
    this.city,
    this.online = false,
    this.deleted = false,
    this.lastActiveAt,
  });

  /// 硬约束：ID 一律字符串下发，JS/Dart 侧 int64 会丢精度。
  final String id;
  final String nickname;

  /// 为空时由 UI 兜底成 emoji 头像，不出现破图。
  final String? avatar;

  final Gender gender;
  final int? age;
  final String? city;

  /// 真在线（WS presence）。后端还没有 presence 能力时恒为 false，
  /// 此时用 [lastActiveAt] 降级成「最近活跃」。
  final bool online;

  /// 对方已注销：会话只读，资料不可点进。
  final bool deleted;

  /// `User.LastActiveAt`，在线状态的降级数据源。
  final DateTime? lastActiveAt;

  /// 5 分钟内有动作就当作「在线」——这是没有 presence 时最接近真相的近似。
  bool get presumedOnline {
    if (online) return true;
    final t = lastActiveAt;
    if (t == null) return false;
    return DateTime.now().toUtc().difference(t).inMinutes < 5;
  }

  factory UserBrief.fromJson(Map<String, dynamic> j) => UserBrief(
    id: idOf(j['id']),
    nickname: (j['nickname'] ?? '') as String,
    avatar: j['avatar'] as String?,
    gender: Gender.parse(j['gender']),
    age: (j['age'] as num?)?.toInt(),
    city: j['city'] as String?,
    online: j['online'] == true,
    deleted: j['status'] == 'deleted',
    lastActiveAt: utcOrNull(j['last_active_at']),
  );
}

/// 用户主页 / 我的页用到的完整资料。
class UserProfile {
  const UserProfile({
    required this.brief,
    this.bio,
    this.birthday,
    this.languages = const [],
    this.interests = const [],
    this.charm = 0,
    this.followingCount = 0,
    this.followerCount = 0,
    this.distanceKm,
    this.bottleCount = 0,
    this.momentCount = 0,
    this.relationStage = RelationStage.stranger,
    this.lastActiveAt,
    this.googleBound = false,
    this.appleBound = false,
    this.maskedPhone,
    this.maskedEmail,
    this.commonPoint,
  });

  final UserBrief brief;
  final String? bio;

  /// 出生日期 `YYYY-MM-DD`，只有看自己的资料时服务端才下发（别人只给 age）。
  final String? birthday;

  /// V1 新增字段：`User.Language` / `User.Interests`（AutoMigrate 自动加列）。
  final List<String> languages;
  final List<String> interests;

  final int charm;

  /// 关注数(我关注的人) / 粉丝数(关注我的人)。服务端从 Relation(type=like) 聚合下发。
  final int followingCount;
  final int followerCount;

  /// 由 `User.Lat/Lng` 与我方位置算出；未开定位时为 null，UI 不显示距离标签。
  final double? distanceKm;

  final int bottleCount;
  final int momentCount;
  final RelationStage relationStage;

  /// 在线状态的降级方案：没有 presence 能力时按「最近活跃」显示。
  final DateTime? lastActiveAt;

  /// 第三方身份是否已绑定，供「账号与安全」显示状态。
  ///
  /// 服务端**只下发布尔值，不下发 sub 本身**——那是第三方的用户标识，
  /// 前端没有任何用途，下发只会扩大攻击面。
  ///
  /// 只在 `/user/profile`（自己）上有意义；看别人的资料时恒为 false。
  final bool googleBound;
  final bool appleBound;

  /// 已绑定的手机号 / 邮箱（脱敏，服务端下发；null=未绑定）。仅自己的 profile 有值。
  final String? maskedPhone;
  final String? maskedEmail;

  /// C3「共同点」：与 TA 选了相同答案的答题项文案（服务端算；null=无共同点）。
  final String? commonPoint;

  String get id => brief.id;
  String get nickname => brief.nickname;

  factory UserProfile.fromJson(Map<String, dynamic> j) => UserProfile(
    brief: UserBrief.fromJson(j),
    bio: j['bio'] as String?,
    birthday: (j['birthday'] as String?)?.isEmpty == true
        ? null
        : j['birthday'] as String?,
    languages: stringList(j['languages'] ?? j['language']),
    interests: stringList(j['interests']),
    charm: intOf(j['charm']),
    followingCount: intOf(j['following_count']),
    followerCount: intOf(j['follower_count']),
    distanceKm: (j['distance_km'] as num?)?.toDouble(),
    bottleCount: intOf(j['bottle_count']),
    momentCount: intOf(j['moment_count']),
    relationStage: RelationStage.parse(j['relation_stage']),
    googleBound: j['google_bound'] == true,
    appleBound: j['apple_bound'] == true,
    maskedPhone: j['masked_phone'] as String?,
    maskedEmail: j['masked_email'] as String?,
    commonPoint: j['common_point'] as String?,
    lastActiveAt: utcOrNull(j['last_active_at']),
  );

  UserProfile copyWith({
    String? bio,
    List<String>? languages,
    List<String>? interests,
  }) {
    return UserProfile(
      brief: brief,
      bio: bio ?? this.bio,
      birthday: birthday,
      languages: languages ?? this.languages,
      interests: interests ?? this.interests,
      charm: charm,
      followingCount: followingCount,
      followerCount: followerCount,
      distanceKm: distanceKm,
      bottleCount: bottleCount,
      momentCount: momentCount,
      relationStage: relationStage,
      lastActiveAt: lastActiveAt,
      googleBound: googleBound,
      appleBound: appleBound,
      maskedPhone: maskedPhone,
      maskedEmail: maskedEmail,
      commonPoint: commonPoint,
    );
  }
}

/// 「我」的账户状态：余额与每日次数在多个页面共用。
class Wallet {
  const Wallet({
    required this.coins,
    this.totalRecharged = 0,
    this.totalSpent = 0,
  });

  final int coins;
  final int totalRecharged;
  final int totalSpent;

  factory Wallet.fromJson(Map<String, dynamic> j) => Wallet(
    coins: intOf(j['balance']),
    totalRecharged: intOf(j['total_recharged']),
    totalSpent: intOf(j['total_spent']),
  );
}
