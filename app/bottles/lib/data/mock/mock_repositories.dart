import '../../core/config/remote_config.dart';
import '../../core/network/api_exception.dart';
import '../../domain/models/bottle.dart';
import '../../domain/models/chat.dart';
import '../../domain/models/engagement.dart';
import '../../domain/models/moment.dart';
import '../../domain/models/place.dart';
import '../../domain/models/user.dart';
import '../../domain/models/wallet.dart';
import '../repositories.dart';
import 'mock_backend.dart';

/// 统一的假延迟：略高于 300ms 的加载态阈值，让骨架屏有机会出现一次。
Future<T> _delay<T>(T value, [int ms = 340]) =>
    Future.delayed(Duration(milliseconds: ms), () => value);

class MockAuthRepository implements AuthRepository {
  MockAuthRepository(this._db);

  final MockBackend _db;

  @override
  Future<void> sendOtp({
    String? dialCode,
    String? phone,
    String? email,
    String purpose = 'login',
  }) => _delay(null, 600);

  /// Mock 下任意 6 位码 + 任意 8 位以上密码都通过；真实校验在后端。
  @override
  Future<AuthResult> register({
    required String code,
    required String password,
    String? dialCode,
    String? phone,
    String? email,
  }) =>
      _delay(AuthResult(token: 'mock-jwt', profile: _db.me, isNew: true), 700);

  @override
  Future<AuthResult> loginWithPassword({
    required String password,
    String? dialCode,
    String? phone,
    String? email,
  }) =>
      _delay(AuthResult(token: 'mock-jwt', profile: _db.me, isNew: false), 700);

  @override
  Future<AuthResult> resetPassword({
    required String code,
    required String password,
    String? dialCode,
    String? phone,
    String? email,
  }) =>
      _delay(AuthResult(token: 'mock-jwt', profile: _db.me, isNew: false), 700);

  @override
  Future<AuthResult> verifyOtp({
    required String code,
    String? dialCode,
    String? phone,
    String? email,
  }) async {
    // Mock 下任意 6 位数字都通过；真实链路由后端校验 Redis 里的验证码。
    return _delay(
      AuthResult(token: 'mock-jwt', profile: _db.me, isNew: code == '000000'),
      700,
    );
  }

  @override
  Future<AuthResult> loginWithProvider(String provider, {String? idToken}) =>
      _delay(
        AuthResult(token: 'mock-jwt-$provider', profile: _db.me, isNew: false),
        600,
      );

  @override
  Future<void> bindOAuth(String provider, String idToken) async {
    // Mock 后端固定放行。idToken 传 'taken' 可模拟「已被他人绑定」，
    // 方便单独调 A6b 那一屏而不必真去配两个 Google 账号。
    if (idToken == 'taken') {
      throw ApiException(ErrCode.oauthAlreadyBound, '该账号已被其他用户绑定');
    }
  }

  @override
  Future<void> unbindOAuth(String provider) async {}

  @override
  Future<UserProfile> fetchProfile() => _delay(_db.me, 200);

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
    // Mock 不关心位置：它只影响服务端的距离计算，本地无处可用。
    // 接收但丢弃，保持签名一致即可。
    double? lat,
    double? lng,
  }) async {
    _db.me = UserProfile(
      brief: UserBrief(
        id: _db.me.id,
        nickname: nickname ?? _db.me.nickname,
        avatar: avatar ?? _db.me.brief.avatar,
        gender: gender ?? _db.me.brief.gender,
        age: age ?? _db.me.brief.age,
        city: _db.me.brief.city,
      ),
      bio: bio ?? _db.me.bio,
      birthday: birthday ?? _db.me.birthday,
      languages: languages ?? _db.me.languages,
      interests: interests ?? _db.me.interests,
      charm: _db.me.charm,
      bottleCount: _db.me.bottleCount,
      momentCount: _db.me.momentCount,
    );
    return _delay(_db.me, 400);
  }

  @override
  Future<void> deleteAccount() => _delay(null, 500);

  /// mock 返回空选项 = 「后台没配」，页面回退到内置 Catalog。
  @override
  Future<ProfileOptions> profileOptions() =>
      _delay(const ProfileOptions(), 200);

  /// mock 不造法务文本：返回空串等于「后台没配」，页面自然回退到内置 H5。
  @override
  Future<String> legalDoc(String doc, {required String lang}) =>
      _delay('', 200);

  /// 同理返回内置值 = 「后台没配」：文案全走 ARB、入口全显、客服走 mailto。
  /// 离线跑的就该是 App 不联网时的样子。
  @override
  Future<AppRemoteConfig> appConfig({required String lang}) =>
      _delay(const AppRemoteConfig.builtin(), 200);
}

class MockBottleRepository implements BottleRepository {
  MockBottleRepository(this._db);

  final MockBackend _db;

  @override
  Future<QuotaInfo> quota() => _delay(_db.quota, 160);

  @override
  Future<SeaCondition> seaCondition() {
    final now = DateTime.now();
    return _delay(
      SeaCondition(
        repliesDrifting: 1284,
        onlineCount: 312,
        nightStartsAt: DateTime(now.year, now.month, now.day, 21),
      ),
      160,
    );
  }

  @override
  Future<Bottle?> scoop({String? tag, String? language}) =>
      _delay(_db.scoop(tag: tag), 500);

  @override
  Future<List<Bottle>> oceanPeek({String? tag, int limit = 5}) =>
      _delay(_db.peek(tag), 280);

  @override
  Future<Bottle> cast({
    required String content,
    List<String> images = const [],
    List<String> tags = const [],
    bool cityOnly = false,
    // Mock 不存地点：它只影响服务端的距离计算,本地无处可用。
    Place? place,
  }) => _delay(_db.cast(content: content, images: images, tags: tags), 500);

  @override
  Future<Bottle> detail(String id) => _delay(_db.bottleById(id), 240);

  @override
  Future<List<BottleReply>> replies(String bottleId) =>
      _delay(_db.repliesOf(bottleId), 240);

  @override
  Future<BottleReply> reply(String bottleId, String content) =>
      _delay(_db.addReply(bottleId, content), 400);

  @override
  Future<void> unlockReply(String replyId) =>
      _delay(_db.unlockReply(replyId), 420);

  @override
  Future<void> putBack(String bottleId) => _delay(null, 120);

  @override
  Future<void> setCollected(String bottleId, bool collected) async {
    _db.setCollected(bottleId, collected);
    return _delay(null, 180);
  }

  @override
  Future<List<Bottle>> mine(BottleMineTab tab) => _delay(switch (tab) {
    BottleMineTab.thrown => _db.myThrown,
    BottleMineTab.scooped => _db.myScooped,
    BottleMineTab.collected => _db.myCollected,
  }, 300);

  @override
  Future<List<BottleTraceNode>> trace(String bottleId) =>
      _delay(_db.bottleById(bottleId).trace, 300);
}

class MockDiscoverRepository implements DiscoverRepository {
  MockDiscoverRepository(this._db);

  final MockBackend _db;

  @override
  Future<List<UserProfile>> users({
    required DiscoverFilter filter,
    String? tab,
  }) => _delay(_db.discover(filter), 520);

  @override
  Future<void> like(String userId) => _delay(null, 120);

  @override
  Future<void> pass(String userId) async {
    _db.pass(userId);
    return _delay(null, 80);
  }

  @override
  Future<UserProfile> card(String userId) => _delay(
    _db.people.firstWhere(
      (p) => p.id == userId,
      orElse: () => _db.people.first,
    ),
    240,
  );

  @override
  Future<int> matchCount(DiscoverFilter filter) =>
      _delay(_db.discover(filter).length, 150);

  @override
  Future<DiscoverPricing> discoverConfig() => _delay(
    const DiscoverPricing(
      rewindPrice: MockBackend.priceRewind,
      skipChargeEnabled: false,
      skipPrice: 0,
    ),
    120,
  );

  @override
  Future<UserProfile> rewind() async {
    final u = _db.rewind();
    return _delay(u, 200);
  }

  // 答题匹配默认关，mock 直接回落语言/兴趣（enabled=false）。
  @override
  Future<DiscoverQuiz> quiz() => _delay(DiscoverQuiz.empty, 150);

  @override
  Future<void> saveQuiz(String answers) => _delay(null, 120);
}

class MockChatRepository implements ChatRepository {
  MockChatRepository(this._db);

  final MockBackend _db;

  @override
  Future<List<Conversation>> conversations() => _delay(_db.conversations, 320);

  @override
  Future<List<ChatMessage>> messages(String conversationId) {
    _db.markRead(conversationId);
    return _delay(_db.messages[conversationId] ?? const [], 260);
  }

  @override
  Future<ChatMessage> send(String conversationId, String text) =>
      _delay(_db.sendText(conversationId, text), 220);

  @override
  Future<ChatMessage> sendImage(String conversationId, String localPath) =>
      _delay(
        _db.push(
          conversationId,
          ChatMessage(
            id: 'local-${DateTime.now().microsecondsSinceEpoch}',
            fromMe: true,
            type: MessageType.image,
            at: DateTime.now().toUtc(),
            imageUrl: localPath,
          ),
        ),
        600,
      );

  @override
  Future<ChatMessage> sendGift(String conversationId, GiftItem gift, int qty) =>
      _delay(_db.sendGift(conversationId, gift, qty), 320);

  @override
  Future<Conversation> startWith(String userId) =>
      _delay(_db.startChat(userId), 420);

  @override
  Stream<ChatMessage> incoming(String conversationId) => _db.incoming.where(
    (m) => m.conversationId == null || m.conversationId == conversationId,
  );

  @override
  Stream<ChatMessage> allMessages() => _db.incoming;

  @override
  Future<void> report(String userId, ReportReason reason) => _delay(null, 320);

  @override
  Future<void> block(String userId) async {
    _db.blockUser(userId);
    return _delay(null, 260);
  }

  @override
  Future<List<UserBrief>> blocklist() => _delay(_db.blocked, 240);

  @override
  Future<void> unblock(String userId) async {
    _db.unblock(userId);
    return _delay(null, 200);
  }
}

class MockMomentRepository implements MomentRepository {
  MockMomentRepository(this._db);

  final MockBackend _db;

  @override
  Future<List<Moment>> userMoments(String userId) =>
      _delay(_db.moments.take(3).toList(), 300);

  @override
  Future<List<Moment>> feed(String tab) => _delay(
    tab == 'following' ? _db.moments.take(2).toList() : _db.moments,
    360,
  );

  @override
  Future<Moment> detail(String id) => _delay(
    _db.moments.firstWhere((m) => m.id == id, orElse: () => _db.moments.first),
    220,
  );

  @override
  Future<List<MomentComment>> comments(String momentId) =>
      _delay(_db.momentComments[momentId] ?? const [], 260);

  @override
  Future<bool> like(String momentId, bool liked) async {
    _db.toggleLike(momentId, liked);
    return _delay(liked, 120);
  }

  @override
  Future<MomentComment> comment(String momentId, String text) =>
      _delay(_db.addComment(momentId, text), 300);

  @override
  Future<MomentComment> giftComment(String momentId, GiftItem gift) =>
      _delay(_db.addComment(momentId, '', gift: gift), 320);

  /// Mock 下原样返回本地路径：没有服务端，回显靠 Image.file 也能看。
  @override
  Future<String> uploadImage(String localPath) => _delay(localPath, 300);

  @override
  Future<Moment> post({
    required String text,
    List<String> images = const [],
    Place? place,
    String visible = 'public',
  }) => _delay(_db.postMoment(text, images), 500);

  @override
  Future<void> setFollow(String userId, bool follow) async {
    _db.setFollow(userId, follow);
    return _delay(null, 200);
  }
}

class MockWalletRepository implements WalletRepository {
  MockWalletRepository(this._db);

  final MockBackend _db;

  @override
  Future<Map<String, int>> myItems() => _delay(_db.myItems(), 200);

  @override
  Future<List<ItemRecord>> itemRecords() => _delay(const <ItemRecord>[], 150);

  @override
  Future<void> buyItem(String itemId) async {
    _db.buyItem(itemId);
    return _delay(null, 260);
  }

  @override
  Future<Wallet> wallet() => _delay(_db.wallet, 160);

  @override
  Future<List<WalletTxn>> txns() => _delay(_db.txns, 320);

  @override
  Future<List<RechargePackage>> packages() => _delay(_db.packages, 240);

  @override
  Future<PendingOrder> recharge(RechargePackage pkg, PayChannel channel) {
    final d = _db.createOrder(pkg);
    return _delay(
      PendingOrder(
        orderNo: d['order_no'] as String,
        payParams: (d['pay_params'] as Map).cast<String, dynamic>(),
      ),
      600,
    );
  }

  @override
  Future<PayOrderInfo> orderStatus(String orderNo) =>
      _delay(PayOrderInfo.fromJson(_db.orderStatus(orderNo)), 200);

  @override
  Future<List<PayOrderInfo>> orders() =>
      _delay([for (final o in _db.orderList()) PayOrderInfo.fromJson(o)], 300);

  @override
  Future<void> settleMock(String orderNo, bool success) async {
    _db.settleMock(orderNo, success);
    return _delay(null, 300);
  }

  @override
  Future<List<GiftItem>> gifts() => _delay(_db.gifts, 200);

  @override
  Future<QuotaInfo> buyScoopPack() => _delay(_db.buyScoopPack(), 420);

  @override
  Future<CheckinStatus> checkinStatus() => _delay(_db.checkin, 220);

  @override
  Future<int> checkin() => _delay(_db.doCheckin(), 420);

  @override
  Future<int> adReward() => _delay(_db.doAdReward(), 1200);

  @override
  Future<List<RelationItem>> relations(String tab) =>
      _delay(_db.relations(tab), 300);
}

class MockNotifyRepository implements NotifyRepository {
  MockNotifyRepository(this._db);

  final MockBackend _db;

  @override
  Future<List<NotificationItem>> list() => _delay(_db.notifications, 300);

  @override
  Future<int> unreadCount() =>
      _delay(_db.notifications.where((n) => !n.read).length, 120);

  @override
  Future<void> markAllRead() async {
    for (var i = 0; i < _db.notifications.length; i++) {
      _db.notifications[i] = _db.notifications[i].markRead();
    }
    return _delay(null, 200);
  }

  @override
  Future<void> registerDevice(String token) => _delay(null, 200);
}

/// Mock 逆地理。返回一组固定的孟买地名，让地图选点在没有网络/没有 Key 时
/// 也能把交互走通——**地名是假的，位置是真的**（用传入的经纬度）。
class MockGeoRepository implements GeoRepository {
  static const _names = [
    ('Bandra West', 'Mumbai'),
    ('Colaba', 'Mumbai'),
    ('Andheri East', 'Mumbai'),
    ('Koregaon Park', 'Pune'),
  ];

  @override
  Future<Place> regeo(double lat, double lng) {
    // 按坐标取一个稳定的下标：同一个点每次拿到同一个名字，
    // 否则地图上手指没动地名却在跳，看着像 bug。
    final i =
        ((lat * 1000).abs().floor() + (lng * 1000).abs().floor()) %
        _names.length;
    final (name, city) = _names[i];
    return _delay(Place(lat: lat, lng: lng, name: name, city: city), 300);
  }
}

class MockSparkRepository implements SparkRepository {
  @override
  Future<String> accept(String peerId) async => 'mock-chat-$peerId';
}
