import 'package:dio/dio.dart';

import '../../core/config/app_config.dart';
import '../../core/config/remote_config.dart';
import '../../core/network/api_client.dart';
import '../../core/network/api_exception.dart';
import '../../domain/models/bottle.dart';
import '../../ui/widgets/sea_scene.dart' show bottleSlots;
import '../../domain/models/chat.dart';
import '../../domain/models/engagement.dart';
import '../../domain/models/moment.dart';
import '../../domain/models/place.dart';
import '../../domain/models/user.dart';
import '../../domain/models/wallet.dart';
import '../repositories.dart';

/// 真实后端实现。
///
/// 路径与字段取自 `server/internal/*/handler.go` 的实际路由。
///
/// **响应形状**：App 请求由后端按 JWT 里的 `platform=app` 分派到 `common/appdto`
/// 转换后的结构（id 而非 user_id / bottle_id、tags 与 images 是数组、author 是嵌套对象）。
/// 小程序走原结构，两边互不影响。
///
/// ⚠️ 已核对并跑通的是**主链路**：登录 → 资料 → 抛瓶/捞瓶 → 配额/钱包。
/// 聊天、发现、动态三组尚未逐字段核对；方法内标了 ⚠️ 的是已知不匹配，别当它能用。
List<Map<String, dynamic>> _rows(Object? data, [String? key]) {
  final raw = data is Map && key != null ? data[key] : data;
  if (raw is List) {
    return [
      for (final e in raw)
        if (e is Map) Map<String, dynamic>.from(e),
    ];
  }
  return const [];
}

Map<String, dynamic> _obj(Object? data, [String? key]) {
  final raw = data is Map && key != null ? (data[key] ?? data) : data;
  return raw is Map ? Map<String, dynamic>.from(raw) : <String, dynamic>{};
}

/// 道具 ID 在后端是 `int64` 且**没有** `,string` tag——传字符串会被 JSON 解析
/// 直接拒掉（400），不是静默忽略。与用户/瓶子 ID 的约定相反，这里必须发数字。
int _itemId(String id) => int.tryParse(id) ?? 0;

class RemoteAuthRepository implements AuthRepository {
  RemoteAuthRepository(this._api, {this.onLogConfig});

  final ApiClient _api;

  /// clientConfig 里的日志配置回调。由 providers 层接到 Prefs 与 Log，
  /// 这里不直接依赖它们——data 层不该知道存储和日志门面的存在。
  final void Function(Map<String, Object?>)? onLogConfig;

  /// 登录与 /user/profile 的响应都平铺着 clientConfig，两处各调一次。
  void _applyLogConfig(Map<String, Object?> m) {
    const keys = [
      'app_log_enabled',
      'app_log_level',
      'app_log_body',
      'app_log_max_mb',
      'app_log_retain_days',
    ];
    final cfg = <String, Object?>{};
    for (final k in keys) {
      if (m.containsKey(k)) cfg[k] = m[k];
    }
    if (cfg.isEmpty) return;
    onLogConfig?.call(cfg);
  }

  @override
  Future<void> sendOtp({
    String? dialCode,
    String? phone,
    String? email,
    String purpose = 'login',
  }) => _api.post(
    '/auth/otp/send',
    body: {
      'appid': AppConfig.appId,
      'purpose': purpose,
      ..._identityBody(dialCode, phone, email),
    },
  );

  @override
  Future<AuthResult> register({
    required String code,
    required String password,
    String? dialCode,
    String? phone,
    String? email,
  }) async => _authResult(
    await _api.post<dynamic>(
      '/auth/register',
      body: {
        'appid': AppConfig.appId,
        ..._identityBody(dialCode, phone, email),
        'code': code,
        'password': password,
      },
    ),
  );

  @override
  Future<AuthResult> loginWithPassword({
    required String password,
    String? dialCode,
    String? phone,
    String? email,
  }) async => _authResult(
    await _api.post<dynamic>(
      '/auth/login/password',
      body: {
        'appid': AppConfig.appId,
        ..._identityBody(dialCode, phone, email),
        'password': password,
      },
    ),
  );

  @override
  Future<AuthResult> resetPassword({
    required String code,
    required String password,
    String? dialCode,
    String? phone,
    String? email,
  }) async => _authResult(
    await _api.post<dynamic>(
      '/auth/password/reset',
      body: {
        'appid': AppConfig.appId,
        ..._identityBody(dialCode, phone, email),
        'code': code,
        'password': password,
      },
    ),
  );

  @override
  Future<AuthResult> verifyOtp({
    required String code,
    String? dialCode,
    String? phone,
    String? email,
  }) async {
    final data = await _api.post<dynamic>(
      '/auth/otp/verify',
      body: {
        'appid': AppConfig.appId,
        ..._identityBody(dialCode, phone, email),
        'code': code,
      },
    );
    return _authResult(data);
  }

  /// 身份字段二选一。后端按 `email` 非空分派渠道，所以走手机号时不能把
  /// 空的 email 也传上去——那会被当成邮箱链路。
  Map<String, dynamic> _identityBody(
    String? dialCode,
    String? phone,
    String? email,
  ) {
    if (email != null && email.isNotEmpty) return {'email': email};
    return {'dial_code': dialCode ?? '', 'phone': phone ?? ''};
  }

  @override
  Future<AuthResult> loginWithProvider(
    String provider, {
    String? idToken,
  }) async {
    // provider: 'google' | 'apple'。服务端按各自 JWKS 验签并核对 aud/iss。
    final data = await _api.post<dynamic>(
      '/auth/$provider',
      body: {'appid': AppConfig.appId, 'id_token': idToken ?? ''},
    );
    return _authResult(data);
  }

  AuthResult _authResult(Object? data) {
    final m = _obj(data);
    _applyLogConfig(m);
    return AuthResult(
      token: '${m['token'] ?? ''}',
      profile: UserProfile.fromJson(_obj(m['user'])),
      isNew: m['is_new'] == true,
    );
  }

  @override
  Future<void> bindOAuth(String provider, String idToken) => _api.post<dynamic>(
    '/auth/$provider/bind',
    body: {'appid': AppConfig.appId, 'id_token': idToken},
  );

  @override
  Future<void> unbindOAuth(String provider) =>
      _api.post<dynamic>('/auth/unbind', body: {'provider': provider});

  /// ⚠️ 响应是 `{user:{...}, ...clientConfig}`——用户资料在 `user` 键里，
  /// 与它平级的是 sysconfig 下发的客户端配置。取错层会得到一个字段全空的 profile。
  @override
  Future<UserProfile> fetchProfile() async {
    final m = _obj(await _api.get<dynamic>('/user/profile'));
    _applyLogConfig(m);
    return UserProfile.fromJson(_obj(m['user']));
  }

  @override
  Future<UserProfile> updateProfile({
    String? nickname,
    String? avatar,
    Gender? gender,
    int? age,
    String? birthday,
    String? bio,
    List<String>? languages,
    List<String>? interests,
    double? lat,
    double? lng,
  }) async {
    // 成对才发：服务端 user/service.go 里 `if in.Lat != nil && in.Lng != nil` 才写库。
    final pair = lat != null && lng != null;
    final data = await _api.post<dynamic>(
      '/user/update',
      body: {
        'nickname': ?nickname,
        'avatar': ?avatar,
        // 后端 User.Gender 是 int8，不是字符串。
        'gender': ?_genderWire(gender),
        'age': ?age,
        'birthday': ?birthday,
        'bio': ?bio,
        // 后端是逗号分隔串，键名也是单数 language。
        'language': ?languages?.join(','),
        'interests': ?interests?.join(','),
        'lat': ?(pair ? lat : null),
        'lng': ?(pair ? lng : null),
      },
    );
    return UserProfile.fromJson(_obj(data));
  }

  /// 后端 `User.Gender`：0 未知 / 1 男 / 2 女。
  static int? _genderWire(Gender? g) => switch (g) {
    Gender.male => 1,
    Gender.female => 2,
    Gender.secret => 0,
    null => null,
  };

  @override
  Future<void> deleteAccount() => _api.delete('/user/account');

  @override
  Future<ProfileOptions> profileOptions() async => ProfileOptions.fromJson(
    _obj(await _api.get<dynamic>('/profile-options')),
  );

  @override
  Future<String> legalDoc(String doc, {required String lang}) async {
    final data = _obj(
      await _api.get<dynamic>('/legal/$doc', query: {'lang': lang}),
    );
    return (data['content'] as String?) ?? '';
  }

  @override
  Future<AppRemoteConfig> appConfig({required String lang}) async =>
      AppRemoteConfig.fromJson(
        _obj(await _api.get<dynamic>('/app-config', query: {'lang': lang})),
      );
}

class RemoteBottleRepository implements BottleRepository {
  RemoteBottleRepository(this._api);

  final ApiClient _api;

  @override
  Future<QuotaInfo> quota() async =>
      QuotaInfo.fromJson(_obj(await _api.get<dynamic>('/bottle/quota')));

  @override
  Future<SeaCondition> seaCondition() async {
    final data = _obj(await _api.get<dynamic>('/hook-count'));
    final now = DateTime.now();
    return SeaCondition(
      repliesDrifting: (data['count'] as num?)?.toInt() ?? 0,
      onlineCount: (data['online'] as num?)?.toInt() ?? 0,
      nightStartsAt: DateTime(now.year, now.month, now.day, 21),
    );
  }

  /// ⚠️ 是 `/bottle/scoop-one` 而不是 `/bottle/scoop`——两条路由的语义正好相反：
  /// `scoop-one` 才是「捞一个」（计次数 + 去重），`scoop` 是不计次数的一批预览。
  @override
  Future<Bottle?> scoop({String? tag, String? language}) async {
    final data = await _api.get<dynamic>(
      '/bottle/scoop-one',
      query: {
        // 后端用 c.Query("tags") 取单值再按逗号拆，不能让 Dio 展开成重复 key。
        if (tag != null && tag != 'all') 'tags': tag,
      },
    );
    // 空池子返回 null / 空数组 / 空对象，都不是错误——走空态 Z1，不弹 toast。
    if (data == null) return null;
    if (data is List) {
      return data.isEmpty ? null : Bottle.fromJson(_obj(data.first));
    }
    final m = _obj(data);
    return m.isEmpty ? null : Bottle.fromJson(m);
  }

  /// 海面上漂着的那几个瓶子。走 `/bottle/scoop`——它读 Redis 预生成的 feed，
  /// **不消耗次数**，正是这里要的语义。后端没有 peek/limit 参数，取回来自己截。
  @override
  Future<List<Bottle>> oceanPeek({String? tag, int limit = 5}) async {
    final data = await _api.get<dynamic>(
      '/bottle/scoop',
      query: {if (tag != null && tag != 'all') 'tags': tag},
    );
    final list = [for (final b in _rows(data, 'list')) Bottle.fromJson(b)];
    final n = limit.clamp(1, bottleSlots.length);
    return list.length > n ? list.sublist(0, n) : list;
  }

  @override
  Future<Bottle> cast({
    required String content,
    List<String> images = const [],
    List<String> tags = const [],
    bool cityOnly = false,
    Place? place,
  }) async {
    // 后端字段与这里的参数名对不上，逐个翻译：
    // images(数组) → media_url(单个，库里就一列)；cityOnly → scope=local/national。
    final data = await _api.post<dynamic>(
      '/bottle/create',
      body: {
        'content': content,
        'content_type': images.isEmpty ? 'text' : 'image',
        'media_url': images.isEmpty ? '' : images.first,
        'tags': tags,
        'scope': cityOnly ? 'local' : 'national',
        'is_anonymous': true,
        'night': tags.contains('night'),
        // 地点可选。lat/lng 必须成对——服务端只在两者都有时才写库。
        'lat': ?place?.lat,
        'lng': ?place?.lng,
        'place_name': ?place?.name,
        'city': ?place?.city,
      },
    );
    return Bottle.fromJson(_obj(data));
  }

  @override
  Future<Bottle> detail(String id) async =>
      Bottle.fromJson(_obj(await _api.get<dynamic>('/bottle/$id')));

  @override
  Future<List<BottleReply>> replies(String bottleId) async {
    final data = await _api.get<dynamic>('/bottle/$bottleId/replies');
    return [for (final r in _rows(data, 'list')) BottleReply.fromJson(r)];
  }

  @override
  Future<BottleReply> reply(String bottleId, String content) async {
    final data = await _api.post<dynamic>(
      '/bottle/$bottleId/reply',
      body: {'content': content},
    );
    return BottleReply.fromJson(_obj(data));
  }

  /// `POST /bottle/reply/:rid/unlock` —— 单条解锁。服务端返回 `{content}`，
  /// 这里不用它：调用方重新拉一次回信列表，locked / 全文一起刷新，少一条状态合并逻辑。
  /// 整瓶解锁的 `/bottle/:id/unlock` 仍在服务端，App 不再用。
  @override
  Future<void> unlockReply(String replyId) =>
      _api.post('/bottle/reply/$replyId/unlock');

  @override
  Future<void> putBack(String bottleId) => _api.post('/bottle/$bottleId/skip');

  @override
  Future<void> setCollected(String bottleId, bool collected) => _api.post(
    collected ? '/collection/add' : '/collection/remove',
    // 后端 colReq 要的是 target_id + target_type(都 required)，
    // 发 bottle_id 会被判「参数错误」。收藏对象类型固定为 bottle。
    body: {'target_id': bottleId, 'target_type': 'bottle'},
  );

  /// 三个 tab 各自的数据源：
  /// - 我扔的 `/bottle/mine`（按 user_id）
  /// - 我捞的 `/bottle/scooped`（从 MatchLog viewer_id 反查，与 feed 已看过去重同源）
  /// - 已收藏 `/collection/list`
  @override
  Future<List<Bottle>> mine(BottleMineTab tab) async {
    final path = switch (tab) {
      BottleMineTab.collected => '/collection/list',
      BottleMineTab.scooped => '/bottle/scooped',
      BottleMineTab.thrown => '/bottle/mine',
    };
    final data = await _api.get<dynamic>(path, query: {'page': 1, 'size': 20});
    return [for (final b in _rows(data, 'list')) Bottle.fromJson(b)];
  }

  @override
  Future<List<BottleTraceNode>> trace(String bottleId) async {
    final data = await _api.get<dynamic>('/bottle/$bottleId/trace');
    return [for (final t in _rows(data, 'list')) BottleTraceNode.fromJson(t)];
  }
}

class RemoteDiscoverRepository implements DiscoverRepository {
  RemoteDiscoverRepository(this._api);

  final ApiClient _api;

  /// 后端用 `c.Query()` 取单值，所以多选项必须逗号拼接，
  /// 不能让 Dio 展开成 `languages=a&languages=b`（那样只有第一个会被读到）。
  static Map<String, dynamic> _filterQuery(DiscoverFilter f, String? tab) => {
    if (f.languages.isNotEmpty) 'languages': f.languages.join(','),
    if (f.interests.isNotEmpty) 'interests': f.interests.join(','),
    if (f.maxDistanceKm != null) 'max_distance_km': f.maxDistanceKm,
    if (f.gender != null) 'gender': f.gender!.wire,
    'min_age': f.minAge,
    'max_age': f.maxAge,
    'tab': ?tab,
  };

  @override
  Future<List<UserProfile>> users({
    required DiscoverFilter filter,
    String? tab,
  }) async {
    final data = await _api.get<dynamic>(
      '/discover/users',
      query: {..._filterQuery(filter, tab), 'limit': 20},
    );
    return [for (final u in _rows(data, 'list')) UserProfile.fromJson(u)];
  }

  @override
  Future<void> like(String userId) =>
      _api.post('/relation/like', body: {'target_id': userId});

  @override
  Future<void> pass(String userId) =>
      _api.post('/discover/skip', body: {'target_id': userId});

  @override
  Future<UserProfile> card(String userId) async =>
      UserProfile.fromJson(_obj(await _api.get<dynamic>('/user/card/$userId')));

  @override
  Future<int> matchCount(DiscoverFilter filter) async {
    final data = _obj(
      await _api.get<dynamic>(
        '/discover/count',
        query: _filterQuery(filter, null),
      ),
    );
    return (data['count'] as num?)?.toInt() ?? 0;
  }

  @override
  Future<DiscoverPricing> discoverConfig() async {
    final data = _obj(await _api.get<dynamic>('/discover/rewind'));
    return DiscoverPricing(
      rewindPrice: (data['price'] as num?)?.toInt() ?? 0,
      skipChargeEnabled: data['skip_charge_enabled'] == true,
      skipPrice: (data['skip_price'] as num?)?.toInt() ?? 0,
    );
  }

  @override
  Future<UserProfile> rewind() async {
    final data = _obj(await _api.post<dynamic>('/discover/rewind'));
    return UserProfile.fromJson(_obj(data['user']));
  }

  @override
  Future<DiscoverQuiz> quiz() async {
    final data = _obj(await _api.get<dynamic>('/discover/quiz'));
    return DiscoverQuiz(
      enabled: data['enabled'] == true,
      questions: [
        for (final q in _rows(data, 'questions')) QuizQuestion.fromJson(q),
      ],
      answers: _parseQuizAnswers((data['answers'] ?? '').toString()),
    );
  }

  @override
  Future<void> saveQuiz(String answers) =>
      _api.post('/discover/quiz', body: {'answers': answers});
}

/// 解析后端紧凑串 "qkey:idx,qkey:idx" → map[qkey]=idx。与服务端 parseAnswers 对称。
Map<String, String> _parseQuizAnswers(String s) {
  final out = <String, String>{};
  for (final p in s.split(',')) {
    final kv = p.trim().split(':');
    if (kv.length == 2 && kv[0].isNotEmpty && kv[1].isNotEmpty) {
      out[kv[0]] = kv[1];
    }
  }
  return out;
}

class RemoteChatRepository implements ChatRepository {
  RemoteChatRepository(this._api, {required this.myId, required this.socket});

  final ApiClient _api;
  final String myId;
  final Stream<ChatMessage> socket;

  @override
  Future<List<Conversation>> conversations() async {
    final data = await _api.get<dynamic>('/chat/list');
    return [for (final c in _rows(data, 'list')) Conversation.fromJson(c)];
  }

  @override
  Future<List<ChatMessage>> messages(String conversationId) async {
    final data = await _api.get<dynamic>('/chat/$conversationId/messages');
    return [
      for (final m in _rows(data, 'list')) ChatMessage.fromJson(m, myId: myId),
    ];
  }

  @override
  Future<ChatMessage> send(String conversationId, String text) async {
    final data = await _api.post<dynamic>(
      '/chat/$conversationId/send',
      body: {'type': 'text', 'content': text},
    );
    return ChatMessage.fromJson(_obj(data), myId: myId);
  }

  @override
  Future<ChatMessage> sendImage(String conversationId, String localPath) async {
    final form = FormData.fromMap({
      'file': await MultipartFile.fromFile(localPath),
    });
    final up = _obj(await _api.upload<dynamic>('/upload', form));
    // 后端 sendReq 只有 content 与 type 两个字段，图片 URL 就放 content——
    // 传 'image' 会被静默丢弃，发出去的是一条空消息。
    final data = await _api.post<dynamic>(
      '/chat/$conversationId/send',
      body: {'type': 'image', 'content': up['url']},
    );
    return ChatMessage.fromJson(_obj(data), myId: myId);
  }

  @override
  Future<ChatMessage> sendGift(
    String conversationId,
    GiftItem gift,
    int qty,
  ) async {
    final data = await _api.post<dynamic>(
      '/chat/$conversationId/gift',
      body: {'item_id': _itemId(gift.id), 'qty': qty},
    );
    return ChatMessage.fromJson(_obj(data), myId: myId);
  }

  @override
  Future<Conversation> startWith(String userId) async {
    final data = await _api.post<dynamic>(
      '/chat/start',
      body: {'target_id': userId},
    );
    return Conversation.fromJson(_obj(data));
  }

  @override
  Stream<ChatMessage> incoming(String conversationId) => socket.where(
    (m) => m.conversationId == null || m.conversationId == conversationId,
  );

  @override
  Stream<ChatMessage> allMessages() => socket;

  @override
  Future<void> report(String userId, ReportReason reason) => _api.post(
    '/report',
    body: {'target_id': userId, 'type': 'user', 'reason': reason.wire},
  );

  @override
  Future<void> block(String userId) =>
      _api.post('/block', body: {'target_id': userId});

  @override
  Future<List<UserBrief>> blocklist() async {
    final data = await _api.get<dynamic>('/block/list');
    return [for (final u in _rows(data, 'list')) UserBrief.fromJson(u)];
  }

  @override
  Future<void> unblock(String userId) =>
      _api.post('/block/remove', body: {'target_id': userId});
}

class RemoteMomentRepository implements MomentRepository {
  RemoteMomentRepository(this._api);

  final ApiClient _api;

  /// `tab`：`recommend`（全部）/ `following`（只看我关注的）/ `city`（同城）。
  /// 以前这个参数根本没发出去，三个 Tab 拿到的是同一份列表。
  @override
  Future<List<Moment>> userMoments(String userId) async {
    final data = await _api.get<dynamic>(
      '/moment/user/$userId',
      query: {'page': 1, 'size': 6},
    );
    return [for (final m in _rows(data, 'list')) Moment.fromJson(m)];
  }

  @override
  Future<List<Moment>> feed(String tab) async {
    final data = await _api.get<dynamic>(
      '/moment/feed',
      query: {'tab': tab, 'page': 1, 'size': 20},
    );
    return [for (final m in _rows(data, 'list')) Moment.fromJson(m)];
  }

  @override
  Future<Moment> detail(String id) async =>
      Moment.fromJson(_obj(await _api.get<dynamic>('/moment/$id')));

  @override
  Future<List<MomentComment>> comments(String momentId) async {
    final data = await _api.get<dynamic>('/moment/$momentId/comments');
    return [for (final c in _rows(data, 'list')) MomentComment.fromJson(c)];
  }

  @override
  Future<bool> like(String momentId, bool liked) async {
    // 后端 toggle，不读请求体，返回 {liked}。
    final data = _obj(await _api.post<dynamic>('/moment/$momentId/like'));
    return data['liked'] == true;
  }

  @override
  Future<MomentComment> comment(String momentId, String text) async {
    final data = await _api.post<dynamic>(
      '/moment/$momentId/comment',
      body: {'content': text},
    );
    return MomentComment.fromJson(_obj(data));
  }

  @override
  Future<MomentComment> giftComment(String momentId, GiftItem gift) async {
    final data = await _api.post<dynamic>(
      '/moment/$momentId/gift',
      body: {'item_id': _itemId(gift.id)},
    );
    return MomentComment.fromJson(_obj(data));
  }

  @override
  Future<String> uploadImage(String localPath) async {
    final form = FormData.fromMap({
      'file': await MultipartFile.fromFile(localPath),
    });
    final up = _obj(await _api.upload<dynamic>('/upload', form));
    final url = '${up['url'] ?? ''}';
    if (url.isEmpty) {
      throw ApiException(ErrCode.serverError, '图片上传失败');
    }
    return url;
  }

  @override
  Future<Moment> post({
    required String text,
    List<String> images = const [],
    Place? place,
    String visible = 'public',
  }) async {
    final data = await _api.post<dynamic>(
      '/moment',
      body: {
        'content': text,
        'visible': visible,
        'images': images,
        'lat': ?place?.lat,
        'lng': ?place?.lng,
        'place_name': ?place?.name,
        'city': ?place?.city,
      },
    );
    return Moment.fromJson(_obj(data));
  }

  /// 关注 / 取关。后端没有独立 follow 表，复用 `Relation.Type=like`。
  ///
  /// 取关走 `/relation/unlike`——以前只有 like 没有反向操作，
  /// 点「已关注」等于再 like 一次，按钮看着能点、状态永远不变。
  @override
  Future<void> setFollow(String userId, bool follow) => _api.post(
    follow ? '/relation/like' : '/relation/unlike',
    body: {'target_id': userId},
  );
}

class RemoteWalletRepository implements WalletRepository {
  RemoteWalletRepository(this._api);

  final ApiClient _api;

  @override
  Future<Wallet> wallet() async =>
      Wallet.fromJson(_obj(await _api.get<dynamic>('/wallet/balance')));

  @override
  Future<List<WalletTxn>> txns() async {
    final data = await _api.get<dynamic>('/wallet/txns');
    return [for (final t in _rows(data, 'list')) WalletTxn.fromJson(t)];
  }

  @override
  Future<List<RechargePackage>> packages() async {
    final data = await _api.get<dynamic>('/pay/packages');
    return [for (final p in _rows(data, 'list')) RechargePackage.fromJson(p)];
  }

  /// ⚠️ **支付链路未完成**。这里只下了单，没有真正发起支付：
  /// `/pay/order` 返回 `{order_no, pay_params}`，要把 pay_params 交给渠道 SDK
  /// 唤起收银台，回调到账后余额才会变。iOS 侧还得先拿 StoreKit 票据再来 verify。
  /// 现在等于「下单后刷一下余额」，余额不会真的增加。见原型 §11⁺ 六屏。
  @override
  Future<PendingOrder> recharge(RechargePackage pkg, PayChannel channel) async {
    // ⚠️ iOS 的 IAP 不在这条路上：`/pay/iap/verify` 要的是 StoreKit 的
    // signed_transaction（`pay/handler.go:81`，binding required），不是 package_id。
    // 旧代码给它传 package_id，那条路径从来没通过（必 400）。
    // StoreKit 接入时另走 IAP 适配器，不要再往这里塞。
    final data = _obj(
      await _api.post<dynamic>(
        '/pay/order',
        body: {
          // package_id 后端是 int64 且没有 `,string`，传字符串会 400。
          'package_id': int.tryParse(pkg.id) ?? 0,
        },
      ),
    );
    return PendingOrder(
      orderNo: (data['order_no'] ?? '').toString(),
      payParams:
          (data['pay_params'] as Map?)?.cast<String, dynamic>() ?? const {},
    );
  }

  @override
  Future<PayOrderInfo> orderStatus(String orderNo) async =>
      PayOrderInfo.fromJson(
        _obj(await _api.get<dynamic>('/pay/order/$orderNo')),
      );

  @override
  Future<List<PayOrderInfo>> orders() async {
    final data = await _api.get<dynamic>('/pay/orders');
    return [for (final o in _rows(data, 'list')) PayOrderInfo.fromJson(o)];
  }

  @override
  Future<void> settleMock(String orderNo, bool success) => _api.post<dynamic>(
    '/pay/mock/settle',
    body: {'order_no': orderNo, 'result': success ? 'success' : 'fail'},
  );

  /// 后端 `/item/list` 不接受 type 参数，返回全部道具，这里自己筛礼物。
  @override
  Future<List<GiftItem>> gifts() async {
    final data = await _api.get<dynamic>('/item/list');
    return [
      for (final g in _rows(data, 'list'))
        if (g['type'] == null || g['type'] == 'gift') GiftItem.fromJson(g),
    ];
  }

  @override
  Future<List<ItemRecord>> itemRecords() async {
    final data = await _api.get<dynamic>('/item/orders');
    return [for (final r in _rows(data, 'list')) ItemRecord.fromJson(r)];
  }

  /// 后端返回的是一个裸对象 `{"<itemId>": 数量}`，不是 list。
  @override
  Future<Map<String, int>> myItems() async {
    final data = _obj(await _api.get<dynamic>('/item/my'));
    return {
      for (final e in data.entries) e.key: (e.value as num?)?.toInt() ?? 0,
    };
  }

  @override
  Future<void> buyItem(String itemId) => _api.post<dynamic>(
    '/item/buy',
    // item_id 后端是 int64 且没有 `,string`，必须发数字。
    body: {'item_id': _itemId(itemId)},
  );

  /// 买捞瓶次数包。
  ///
  /// 后端按 `item.type == "quota_scoop"` 发放次数（`item.Buy` 里走 `quota.AddPack`），
  /// 所以得先在道具表里找到这件商品——**不硬编码 ID**：seed 里它是 7，
  /// 但那只是初始数据，运营在后台能改。
  @override
  Future<QuotaInfo> buyScoopPack() async {
    final items = _rows(await _api.get<dynamic>('/item/list'), 'list');
    final pack = items.firstWhere(
      (e) => e['type'] == 'quota_scoop',
      orElse: () => const <String, dynamic>{},
    );
    if (pack.isEmpty) {
      throw ApiException(ErrCode.badRequest, '暂时买不到次数包');
    }
    await _api.post<dynamic>(
      '/item/buy',
      body: {'item_id': _itemId('${pack['id'] ?? pack['item_id']}')},
    );
    return QuotaInfo.fromJson(_obj(await _api.get<dynamic>('/bottle/quota')));
  }

  @override
  Future<CheckinStatus> checkinStatus() async =>
      CheckinStatus.fromJson(_obj(await _api.get<dynamic>('/checkin/status')));

  @override
  Future<int> checkin() async {
    final data = _obj(await _api.post<dynamic>('/checkin'));
    return (data['coins'] as num?)?.toInt() ?? 0;
  }

  @override
  Future<int> adReward() async {
    final data = _obj(await _api.post<dynamic>('/ad/reward'));
    return (data['coins'] as num?)?.toInt() ?? 0;
  }

  @override
  Future<List<RelationItem>> relations(String tab) async {
    final path = tab == 'viewed' ? '/relation/i-viewed' : '/relation/list';
    final data = await _api.get<dynamic>(path, query: {'type': tab});
    return [for (final r in _rows(data, 'list')) RelationItem.fromJson(r)];
  }
}

class RemoteNotifyRepository implements NotifyRepository {
  RemoteNotifyRepository(this._api);

  final ApiClient _api;

  @override
  Future<List<NotificationItem>> list() async {
    final data = await _api.get<dynamic>('/notify/list');
    return [for (final n in _rows(data, 'list')) NotificationItem.fromJson(n)];
  }

  @override
  Future<int> unreadCount() async {
    final data = _obj(await _api.get<dynamic>('/notify/unread'));
    return (data['count'] as num?)?.toInt() ?? 0;
  }

  @override
  Future<void> markAllRead() => _api.post('/notify/read');

  @override
  Future<void> registerDevice(String token) =>
      _api.post('/push/device', body: {'token': token, 'platform': 'app'});
}

/// 逆地理。失败一律降级成「只有坐标的 Place」，不抛异常——
/// 没配 Key、供应商超时都属于正常情况，用户还是能把这个点选下去。
class RemoteGeoRepository implements GeoRepository {
  RemoteGeoRepository(this._api);
  final ApiClient _api;

  @override
  Future<Place> regeo(double lat, double lng) async {
    try {
      final data = await _api.get<dynamic>('/geo/regeo?lat=$lat&lng=$lng');
      final m = _obj(data);
      return Place(
        lat: lat,
        lng: lng,
        name: (m['place'] as String?) ?? '',
        city: (m['city'] as String?) ?? '',
      );
    } catch (_) {
      return Place(lat: lat, lng: lng);
    }
  }
}

class RemoteSparkRepository implements SparkRepository {
  RemoteSparkRepository(this._api);

  final ApiClient _api;

  @override
  Future<String> accept(String peerId) async {
    final data = _obj(
      await _api.post<dynamic>('/spark/accept', body: {'peer_id': peerId}),
    );
    return '${data['chat_id'] ?? ''}';
  }
}
