import 'dart:async';
import 'dart:math';

import '../../core/network/api_exception.dart';
import '../../domain/models/bottle.dart';
import '../../domain/models/chat.dart';
import '../../domain/models/engagement.dart';
import '../../domain/models/moment.dart';
import '../../domain/models/user.dart';
import '../../domain/models/wallet.dart';

/// 内存假后端。
///
/// App 端要的三块（OTP 登录 / discover 找人 / FCM 推送）后端尚未实现，
/// 真实服务也不在本地，所以先用它把全链路跑通：扣币、配额、会话、轨迹都是**有状态**的，
/// 不是静态假数据——否则「解锁回信扣 2 币」这类链路根本走不出来。
class MockBackend {
  MockBackend() {
    _seed();
  }

  final _rand = Random(20260915);

  /// 价格与 sysconfig 默认值对齐：解锁 2 / 开聊 5 / 次数包 20 / 撤回 10。
  static const priceUnlock = 2;
  static const priceChat = 5;
  static const priceScoopPack = 20;
  static const priceRewind = 10;
  static const scoopPackSize = 20;

  late UserProfile me;
  Wallet wallet = const Wallet(
    coins: 248,
    totalRecharged: 500,
    totalSpent: 252,
  );
  QuotaInfo quota = const QuotaInfo(
    throwLeft: 3,
    throwTotal: 5,
    scoopLeft: 8,
    scoopTotal: 10,
  );

  final List<UserProfile> people = [];
  final List<Bottle> _oceanPool = [];
  final List<Bottle> myThrown = [];
  final List<Bottle> myScooped = [];
  final List<Bottle> myCollected = [];
  final Map<String, List<BottleReply>> _replies = {};
  final Set<String> _unlockedReplies = {};
  final List<Conversation> conversations = [];
  final Map<String, List<ChatMessage>> messages = {};
  final List<Moment> moments = [];
  final Map<String, List<MomentComment>> momentComments = {};
  final List<NotificationItem> notifications = [];
  final List<WalletTxn> txns = [];
  final List<UserBrief> blocked = [];
  final Set<String> _passed = {};

  /// 撤回要的是「最后划掉的那个」，Set 没顺序，另存一条时间线。
  final List<String> _passOrder = [];
  final Set<String> _collected = {};
  final Set<String> _following = {};

  CheckinStatus checkin = const CheckinStatus(
    streak: 4,
    checkedToday: false,
    todayReward: 20,
  );

  final List<GiftItem> gifts = const [
    GiftItem(id: 'g1', name: '小星星', emoji: '⭐', coins: 10, charm: 10),
    GiftItem(id: 'g2', name: '玫瑰', emoji: '🌹', coins: 20, charm: 20),
    GiftItem(id: 'g3', name: '蛋糕', emoji: '🍰', coins: 66, charm: 66),
    GiftItem(id: 'g4', name: '钻石', emoji: '💎', coins: 188, charm: 188),
  ];

  final List<RechargePackage> packages = const [
    RechargePackage(id: 'p1', coins: 60, priceLabel: '₹49'),
    RechargePackage(
      id: 'p2',
      coins: 300,
      priceLabel: '₹199',
      originalPriceLabel: '₹249',
      bonusPercent: 20,
    ),
    RechargePackage(id: 'p3', coins: 680, priceLabel: '₹449'),
    RechargePackage(id: 'p4', coins: 1980, priceLabel: '₹1299'),
  ];

  final _incoming = StreamController<ChatMessage>.broadcast();
  Stream<ChatMessage> get incoming => _incoming.stream;

  int _seq = 1000;
  String _nextId() => '${++_seq}';

  DateTime _ago({int days = 0, int hours = 0, int minutes = 0}) =>
      DateTime.now().toUtc().subtract(
        Duration(days: days, hours: hours, minutes: minutes),
      );

  // ---------------------------------------------------------------- seed

  void _seed() {
    me = UserProfile(
      brief: const UserBrief(
        id: '8842103',
        nickname: '流浪的猫',
        avatar: null,
        gender: Gender.female,
        age: 24,
        city: 'Mumbai',
      ),
      bio: '偶尔写点没人看的东西。',
      languages: const ['English', 'हिन्दी'],
      interests: const ['音乐', '旅行', '美食'],
      charm: 320,
      bottleCount: 28,
      momentCount: 12,
    );

    people.addAll([
      _person(
        'u01',
        'Aanya',
        Gender.female,
        23,
        'Mumbai',
        2.4,
        '喜欢在深夜听歌，偶尔写点没人看的东西。',
        ['हिन्दी', 'English'],
        ['音乐', '旅行', '美食'],
        1280,
        true,
      ),
      _person(
        'u02',
        'Rohan',
        Gender.male,
        26,
        'Pune',
        6.1,
        '每天一杯 chai，剩下的交给命运。',
        ['English'],
        ['板球', '电影'],
        420,
        false,
      ),
      _person(
        'u03',
        'Meera',
        Gender.female,
        22,
        'Pune',
        11.8,
        '在等雨停，也在等人。',
        ['தமிழ்', 'English'],
        ['绘画', '音乐'],
        860,
        true,
      ),
      _person(
        'u04',
        'Vikram',
        Gender.male,
        29,
        'Delhi',
        23.4,
        '喜欢长途火车和陌生站台。',
        ['हिन्दी'],
        ['旅行', '摄影'],
        210,
        false,
      ),
      _person(
        'u05',
        'Ishita',
        Gender.female,
        25,
        'Mumbai',
        3.9,
        '会做很难吃的饭，但很努力。',
        ['English', 'हिन्दी'],
        ['美食', '电影'],
        640,
        true,
      ),
      _person(
        'u06',
        'Kabir',
        Gender.male,
        27,
        'Bengaluru',
        48.2,
        '写代码，也写诗，两样都不太行。',
        ['English', 'తెలుగు'],
        ['音乐', '阅读'],
        300,
        false,
      ),
      _person(
        'u07',
        'Nisha',
        Gender.female,
        21,
        'Delhi',
        18.7,
        '想去看一次海。',
        ['हिन्दी', 'English'],
        ['旅行', '绘画'],
        980,
        true,
      ),
    ]);

    const oceanTexts = [
      '如果有一天你也在海边，记得替我看一眼日落。',
      '有人在听周杰伦吗，随便哪首都行。',
      '今天在海边捡到一枚很好看的贝壳，突然想把它送给某个陌生人。',
      '深夜的城市很吵，但没有一个声音是对我说的。',
      '明天要去面试了，有人祝我好运吗。',
      '我妈today打电话说家门口的树开花了。',
      '最近在学做咖喱，第五次还是失败。',
      '有没有人也睡不着，聊两句就好。',
    ];

    for (var i = 0; i < oceanTexts.length; i++) {
      final p = people[i % people.length];
      _oceanPool.add(
        Bottle(
          id: 'b${100 + i}',
          content: oceanTexts[i],
          author: UserBrief(
            id: p.id,
            nickname: '匿名',
            gender: p.brief.gender,
            age: p.brief.age,
            city: p.brief.city,
          ),
          createdAt: _ago(days: 1 + i % 4, hours: i * 3),
          tags: i.isEven ? const ['night'] : const ['hollow'],
          city: p.brief.city,
          replyCount: i % 4,
          viewCount: 6 + i * 5,
          likeCount: i % 3,
          cityCount: 1 + i % 3,
        ),
      );
    }

    myThrown.addAll([
      Bottle(
        id: 'mb1',
        content: '如果有一天你也在海边，记得替我看一眼日落。',
        author: me.brief,
        createdAt: _ago(days: 3),
        city: 'Mumbai',
        viewCount: 42,
        replyCount: 3,
        likeCount: 8,
        cityCount: 3,
        trace: [
          BottleTraceNode(
            kind: TraceKind.thrown,
            city: 'Mumbai',
            at: _ago(days: 3),
          ),
          BottleTraceNode(
            kind: TraceKind.seen,
            city: 'Pune',
            at: _ago(hours: 1),
            count: 18,
          ),
          BottleTraceNode(
            kind: TraceKind.replied,
            city: 'Delhi',
            at: _ago(minutes: 12),
            count: 3,
          ),
        ],
      ),
      Bottle(
        id: 'mb2',
        content: '有人在听周杰伦吗，随便哪首都行。',
        author: me.brief,
        createdAt: _ago(days: 1),
        city: 'Mumbai',
        viewCount: 7,
        likeCount: 1,
      ),
      Bottle(
        id: 'mb3',
        content: '深夜写的那句话，现在看有点想删掉。',
        author: me.brief,
        createdAt: _ago(days: 7),
        city: 'Mumbai',
        status: BottleStatus.expired,
      ),
    ]);

    _replies['mb1'] = [
      BottleReply(
        id: 'r1',
        author: const UserBrief(
          id: 'u02',
          nickname: '匿名',
          gender: Gender.male,
          age: 26,
        ),
        content: '我昨天刚好在果阿看了日落，那天云特别厚。',
        createdAt: _ago(days: 2),
        unlockCost: priceUnlock,
      ),
      BottleReply(
        id: 'r2',
        author: const UserBrief(
          id: 'u03',
          nickname: '匿名',
          gender: Gender.female,
          age: 22,
        ),
        content: '我也想去，可惜一直没时间。',
        createdAt: _ago(days: 1),
        unlockCost: priceUnlock,
      ),
      BottleReply(
        id: 'r3',
        author: const UserBrief(
          id: 'u01',
          nickname: '匿名',
          gender: Gender.female,
          age: 24,
        ),
        content: '日落这种东西，还是要有人一起看。',
        createdAt: _ago(minutes: 12),
      ),
    ];

    myScooped.addAll(_oceanPool.take(2));
    myCollected.add(_oceanPool[2]);

    _seedChats();
    _seedMoments();

    notifications.addAll([
      NotificationItem(
        id: 'n1',
        kind: NotifyKind.reply,
        title: '你的瓶子收到新回信',
        body: '「我昨天刚好在果阿看了日落…」',
        at: _ago(minutes: 2),
      ),
      NotificationItem(
        id: 'n2',
        kind: NotifyKind.gift,
        title: 'Meera 送了你一份礼物',
        body: '小星星 · 魅力 +10',
        at: _ago(hours: 1),
      ),
      NotificationItem(
        id: 'n3',
        kind: NotifyKind.like,
        title: 'Rohan 赞了你的动态',
        body: '果阿的日落，值得跑一趟。',
        at: _ago(days: 1),
        read: true,
      ),
    ]);

    txns.addAll([
      WalletTxn(
        id: 't1',
        scene: TxnScene.unlock,
        amount: -2,
        at: _ago(hours: 2),
        balanceAfter: 248,
      ),
      WalletTxn(
        id: 't2',
        scene: TxnScene.chat,
        amount: -5,
        at: _ago(hours: 3),
        balanceAfter: 250,
      ),
      WalletTxn(
        id: 't3',
        scene: TxnScene.checkin,
        amount: 10,
        at: _ago(hours: 14),
        balanceAfter: 255,
      ),
      WalletTxn(
        id: 't4',
        scene: TxnScene.recharge,
        amount: 120,
        at: _ago(days: 1, hours: 4),
        balanceAfter: 245,
      ),
      WalletTxn(
        id: 't5',
        scene: TxnScene.gift,
        amount: -10,
        at: _ago(days: 1, hours: 8),
        balanceAfter: 125,
      ),
      WalletTxn(
        id: 't6',
        scene: TxnScene.adReward,
        amount: 5,
        at: _ago(days: 2),
        balanceAfter: 135,
      ),
      WalletTxn(
        id: 't7',
        scene: TxnScene.gift,
        amount: -20,
        at: _ago(days: 3),
        balanceAfter: 130,
        note: '捞瓶次数包 ×20',
      ),
    ]);
  }

  UserProfile _person(
    String id,
    String name,
    Gender gender,
    int age,
    String city,
    double km,
    String bio,
    List<String> langs,
    List<String> interests,
    int charm,
    bool online,
  ) {
    final lastActive = _ago(minutes: online ? 1 : 90);
    return UserProfile(
      brief: UserBrief(
        id: id,
        nickname: name,
        gender: gender,
        age: age,
        city: city,
        online: online,
        lastActiveAt: lastActive,
      ),
      bio: bio,
      languages: langs,
      interests: interests,
      charm: charm,
      distanceKm: km,
      bottleCount: 12 + charm % 30,
      momentCount: 8 + charm % 60,
      relationStage: RelationStage.fromScore(charm ~/ 20),
      lastActiveAt: lastActive,
    );
  }

  void _seedChats() {
    final aanya = people[0].brief;
    final rohan = people[1].brief;
    final meera = people[2].brief;
    final vikram = people[3].brief;

    conversations.addAll([
      Conversation(
        id: 'c1',
        peer: aanya,
        lastAt: _ago(minutes: 2),
        lastText: '那你今晚也会看日落吗？',
        unread: 2,
        fromBottle: true,
      ),
      Conversation(
        id: 'c2',
        peer: rohan,
        lastAt: _ago(hours: 1),
        lastText: '',
        lastType: MessageType.image,
      ),
      Conversation(
        id: 'c3',
        peer: const UserBrief(
          id: 'u09',
          nickname: '匿名瓶友',
          gender: Gender.secret,
        ),
        lastAt: _ago(days: 1),
        lastText: '小星星',
        lastType: MessageType.gift,
        fromBottle: true,
      ),
      Conversation(
        id: 'c4',
        peer: meera,
        lastAt: _ago(days: 3),
        lastText: '好呀，那就这样说定了',
      ),
      Conversation(
        id: 'c5',
        peer: vikram,
        lastAt: _ago(days: 7),
        lastText: '好的，晚点聊',
      ),
      Conversation(
        id: 'c6',
        peer: const UserBrief(
          id: 'u10',
          nickname: '已注销用户',
          gender: Gender.secret,
          deleted: true,
        ),
        lastAt: _ago(days: 8),
        lastText: '对方已注销，会话只读',
      ),
    ]);

    messages['c1'] = [
      ChatMessage(
        id: 'm1',
        fromMe: false,
        type: MessageType.system,
        at: _ago(hours: 5),
        text: 'from_bottle',
      ),
      ChatMessage(
        id: 'm2',
        fromMe: false,
        type: MessageType.text,
        at: _ago(hours: 4),
        text: '我昨天刚好在果阿看了日落 🌅',
      ),
      ChatMessage(
        id: 'm3',
        fromMe: true,
        type: MessageType.text,
        at: _ago(hours: 4),
        text: '真的吗！好羡慕',
        read: true,
      ),
      ChatMessage(
        id: 'm4',
        fromMe: false,
        type: MessageType.text,
        at: _ago(hours: 3),
        text: '下次一起去？',
      ),
      ChatMessage(
        id: 'm5',
        fromMe: true,
        type: MessageType.text,
        at: _ago(hours: 2),
        text: '那说定了，我请你吃海边的烤鱼',
        read: true,
      ),
      ChatMessage(
        id: 'm6',
        fromMe: false,
        type: MessageType.gift,
        at: _ago(minutes: 30),
        gift: gifts[0],
      ),
      ChatMessage(
        id: 'm7',
        fromMe: false,
        type: MessageType.text,
        at: _ago(minutes: 2),
        text: '那你今晚也会看日落吗？',
      ),
    ];

    messages['c2'] = [
      ChatMessage(
        id: 'm10',
        fromMe: false,
        type: MessageType.text,
        at: _ago(hours: 2),
        text: '今天的 chai 格外好喝',
      ),
      ChatMessage(
        id: 'm11',
        fromMe: false,
        type: MessageType.image,
        at: _ago(hours: 1),
        imageUrl: '',
      ),
    ];

    messages['c3'] = [
      ChatMessage(
        id: 'm20',
        fromMe: false,
        type: MessageType.system,
        at: _ago(days: 2),
        text: 'from_bottle',
      ),
      ChatMessage(
        id: 'm21',
        fromMe: false,
        type: MessageType.text,
        at: _ago(days: 2),
        text: '你捞到的那个瓶子，是我扔的',
      ),
      ChatMessage(
        id: 'm22',
        fromMe: false,
        type: MessageType.gift,
        at: _ago(days: 1),
        gift: gifts[0],
      ),
    ];

    messages['c4'] = [
      ChatMessage(
        id: 'm30',
        fromMe: true,
        type: MessageType.text,
        at: _ago(days: 3),
        text: '周末有空吗',
        read: true,
      ),
      ChatMessage(
        id: 'm31',
        fromMe: false,
        type: MessageType.text,
        at: _ago(days: 3),
        text: '好呀，那就这样说定了',
      ),
    ];

    messages['c5'] = [
      ChatMessage(
        id: 'm40',
        fromMe: false,
        type: MessageType.text,
        at: _ago(days: 7),
        text: '好的，晚点聊',
      ),
    ];

    messages['c6'] = [
      ChatMessage(
        id: 'm50',
        fromMe: false,
        type: MessageType.text,
        at: _ago(days: 8),
        text: '很高兴认识你',
      ),
    ];
  }

  void _seedMoments() {
    moments.addAll([
      Moment(
        id: 'mo1',
        author: people[0].brief,
        text: '果阿的日落，值得跑一趟。',
        createdAt: _ago(hours: 2),
        images: const ['', '', ''],
        city: 'Mumbai',
        likeCount: 42,
        commentCount: 8,
      ),
      Moment(
        id: 'mo2',
        author: people[1].brief,
        text: '今天的 chai 格外好喝。',
        createdAt: _ago(hours: 5),
        likeCount: 6,
        commentCount: 1,
      ),
      Moment(
        id: 'mo3',
        author: people[2].brief,
        text: '有人扔了个瓶子问我在哪，我说在等雨停。',
        createdAt: _ago(days: 1),
        images: const ['', ''],
        city: 'Pune',
        likeCount: 128,
        commentCount: 24,
        liked: true,
      ),
      Moment(
        id: 'mo4',
        author: people[4].brief,
        text: '第五次做咖喱，这次没糊。',
        createdAt: _ago(days: 2),
        images: const [''],
        city: 'Mumbai',
        likeCount: 33,
        commentCount: 4,
      ),
    ]);

    momentComments['mo1'] = [
      MomentComment(
        id: 'mc1',
        author: people[1].brief,
        createdAt: _ago(hours: 1),
        text: '下次带上我',
      ),
      MomentComment(
        id: 'mc2',
        author: people[2].brief,
        createdAt: _ago(minutes: 40),
        gift: gifts[0],
      ),
    ];
  }

  // ------------------------------------------------------------- 资金/配额

  void _debit(int coins, TxnScene scene, {String? note}) {
    if (wallet.coins < coins) {
      throw ApiException(
        ErrCode.insufficientBalance,
        'insufficient balance',
        data: {'need': coins, 'have': wallet.coins},
      );
    }
    wallet = Wallet(
      coins: wallet.coins - coins,
      totalRecharged: wallet.totalRecharged,
      totalSpent: wallet.totalSpent + coins,
    );
    txns.insert(
      0,
      WalletTxn(
        id: _nextId(),
        scene: scene,
        amount: -coins,
        at: DateTime.now().toUtc(),
        balanceAfter: wallet.coins,
        note: note,
      ),
    );
  }

  void credit(int coins, TxnScene scene, {String? note}) {
    wallet = Wallet(
      coins: wallet.coins + coins,
      totalRecharged:
          wallet.totalRecharged + (scene == TxnScene.recharge ? coins : 0),
      totalSpent: wallet.totalSpent,
    );
    txns.insert(
      0,
      WalletTxn(
        id: _nextId(),
        scene: scene,
        amount: coins,
        at: DateTime.now().toUtc(),
        balanceAfter: wallet.coins,
        note: note,
      ),
    );
  }

  // ---------------------------------------------------------------- 瓶子

  Bottle? scoop({String? tag}) {
    if (!quota.canScoop) {
      throw ApiException(ErrCode.quotaExceeded, 'quota exceeded');
    }
    final pool = _oceanPool
        .where((b) => tag == null || tag == 'all' || b.tags.contains(tag))
        .toList();
    if (pool.isEmpty) return null;
    quota = quota.consumeScoop();
    final b = pool[_rand.nextInt(pool.length)];
    if (!myScooped.any((e) => e.id == b.id)) myScooped.insert(0, b);
    return b;
  }

  /// 海面预览：只读，不消耗次数。
  List<Bottle> peek(String? tag) {
    final pool = _oceanPool
        .where((b) => tag == null || tag == 'all' || b.tags.contains(tag))
        .toList();
    return pool.take(5).toList();
  }

  Bottle cast({
    required String content,
    required List<String> images,
    required List<String> tags,
  }) {
    if (!quota.canThrow && !tags.contains('night')) {
      throw ApiException(ErrCode.quotaExceeded, 'quota exceeded');
    }
    if (!tags.contains('night')) quota = quota.consumeThrow();
    final b = Bottle(
      id: _nextId(),
      content: content,
      author: me.brief,
      createdAt: DateTime.now().toUtc(),
      images: images,
      tags: tags,
      city: me.brief.city,
      trace: [
        BottleTraceNode(
          kind: TraceKind.thrown,
          city: me.brief.city ?? '',
          at: DateTime.now().toUtc(),
        ),
      ],
    );
    myThrown.insert(0, b);
    _oceanPool.insert(0, b);
    return b;
  }

  Bottle bottleById(String id) {
    final b = [
      ..._oceanPool,
      ...myThrown,
      ...myScooped,
    ].firstWhere((e) => e.id == id, orElse: () => _oceanPool.first);
    return b.copyWith(collected: _collected.contains(b.id));
  }

  void setCollected(String bottleId, bool collected) {
    if (collected) {
      _collected.add(bottleId);
      if (!myCollected.any((b) => b.id == bottleId)) {
        myCollected.insert(0, bottleById(bottleId));
      }
    } else {
      _collected.remove(bottleId);
      myCollected.removeWhere((b) => b.id == bottleId);
    }
  }

  /// 背包：itemId → 持有数量。
  final Map<String, int> _bag = {};

  Map<String, int> myItems() => Map.of(_bag);

  void buyItem(String itemId) {
    final item = gifts.firstWhere(
      (g) => g.id == itemId,
      orElse: () => gifts.first,
    );
    _debit(item.coins, TxnScene.gift, note: '购买 ${item.name}');
    _bag[itemId] = (_bag[itemId] ?? 0) + 1;
  }

  void setFollow(String userId, bool follow) {
    follow ? _following.add(userId) : _following.remove(userId);
    for (var i = 0; i < moments.length; i++) {
      if (moments[i].author.id == userId) {
        moments[i] = moments[i].copyWith(following: follow);
      }
    }
  }

  bool isFollowing(String userId) => _following.contains(userId);

  List<BottleReply> repliesOf(String bottleId) {
    final list = _replies[bottleId] ?? const <BottleReply>[];
    return [
      for (final r in list)
        _unlockedReplies.contains(r.id) ? r.unlockedCopy() : r,
    ];
  }

  /// 按条解锁，扣一条的价；重复解锁同一条不再扣（与服务端 ReplyUnlock 幂等一致）。
  void unlockReply(String replyId) {
    if (_unlockedReplies.add(replyId)) {
      _debit(priceUnlock, TxnScene.unlock);
    }
  }

  BottleReply addReply(String bottleId, String content) {
    final r = BottleReply(
      id: _nextId(),
      author: me.brief,
      content: content,
      createdAt: DateTime.now().toUtc(),
      locked: false,
    );
    _replies.putIfAbsent(bottleId, () => []).add(r);
    return r;
  }

  // ---------------------------------------------------------------- 聊天

  Conversation startChat(String userId) {
    final existing = conversations.where((c) => c.peer.id == userId).toList();
    if (existing.isNotEmpty) return existing.first;
    _debit(priceChat, TxnScene.chat);
    final peer = people.firstWhere((p) => p.id == userId).brief;
    final c = Conversation(
      id: _nextId(),
      peer: peer,
      lastAt: DateTime.now().toUtc(),
      lastText: '',
    );
    conversations.insert(0, c);
    messages[c.id] = [
      ChatMessage(
        id: _nextId(),
        fromMe: false,
        type: MessageType.system,
        at: DateTime.now().toUtc(),
        text: 'from_bottle',
      ),
    ];
    return c;
  }

  ChatMessage push(String convId, ChatMessage msg) {
    // 带上会话号:与真机 WS 帧对齐,常驻分发器据此把消息落到正确会话。
    msg = msg.copyWith(conversationId: convId);
    messages.putIfAbsent(convId, () => []).add(msg);
    final i = conversations.indexWhere((c) => c.id == convId);
    if (i >= 0) {
      conversations[i] = conversations[i].copyWith(
        lastText: msg.type == MessageType.gift
            ? (msg.gift?.name ?? '')
            : msg.text,
        lastType: msg.type,
        lastAt: msg.at,
        unread: msg.fromMe
            ? conversations[i].unread
            : conversations[i].unread + 1,
      );
    }
    return msg;
  }

  ChatMessage sendText(String convId, String text) {
    final msg = push(
      convId,
      ChatMessage(
        id: _nextId(),
        fromMe: true,
        type: MessageType.text,
        at: DateTime.now().toUtc(),
        text: text,
      ),
    );
    _scheduleAutoReply(convId);
    return msg;
  }

  ChatMessage sendGift(String convId, GiftItem gift, int qty) {
    // 背包先抵扣，差额再扣币（服务端 chat.SendGift 同一条规则）。
    final have = _bag[gift.id] ?? 0;
    final fromBag = have < qty ? have : qty;
    if (fromBag > 0) {
      if (have - fromBag > 0) {
        _bag[gift.id] = have - fromBag;
      } else {
        _bag.remove(gift.id);
      }
    }
    final pay = qty - fromBag;
    if (pay > 0) {
      _debit(gift.coins * pay, TxnScene.gift, note: '送出「${gift.name}」');
    }
    return push(
      convId,
      ChatMessage(
        id: _nextId(),
        fromMe: true,
        type: MessageType.gift,
        at: DateTime.now().toUtc(),
        gift: gift,
        giftQty: qty,
      ),
    );
  }

  /// 模拟 WS 推来的对端消息，让聊天页的实时链路在 Mock 下也能跑通。
  void _scheduleAutoReply(String convId) {
    const canned = [
      '嗯嗯，我也这么觉得',
      '哈哈，你怎么知道',
      '那边现在几点呀',
      '改天真的可以一起去看看',
      '我刚刚在想同一件事',
    ];
    Timer(Duration(milliseconds: 1400 + _rand.nextInt(1200)), () {
      if (_incoming.isClosed) return;
      final msg = push(
        convId,
        ChatMessage(
          id: _nextId(),
          fromMe: false,
          type: MessageType.text,
          at: DateTime.now().toUtc(),
          text: canned[_rand.nextInt(canned.length)],
        ),
      );
      _incoming.add(msg);
    });
  }

  void markRead(String convId) {
    final i = conversations.indexWhere((c) => c.id == convId);
    if (i >= 0) conversations[i] = conversations[i].copyWith(unread: 0);
  }

  int get totalUnread => conversations.fold(0, (sum, c) => sum + c.unread);

  // ---------------------------------------------------------------- 发现

  List<UserProfile> discover(DiscoverFilter f) {
    return people.where((p) {
      if (_passed.contains(p.id)) return false;
      if (blocked.any((b) => b.id == p.id)) return false;
      final age = p.brief.age ?? 0;
      if (age < f.minAge || age > f.maxAge) return false;
      if (f.gender != null && p.brief.gender != f.gender) return false;
      if (f.maxDistanceKm != null && (p.distanceKm ?? 0) > f.maxDistanceKm!) {
        return false;
      }
      if (f.languages.isNotEmpty && !p.languages.any(f.languages.contains)) {
        return false;
      }
      if (f.interests.isNotEmpty && !p.interests.any(f.interests.contains)) {
        return false;
      }
      return true;
    }).toList();
  }

  /// 左滑同样落库，否则下次拉人去重不掉、同一个人反复推上来。
  void pass(String userId) {
    _passed.add(userId);
    _passOrder.add(userId);
  }

  /// 撤回上一次左滑：扣币把人放回候选池。没人可撤时抛参数错误，与后端一致。
  UserProfile rewind() {
    if (_passOrder.isEmpty) {
      throw ApiException(ErrCode.badRequest, 'nothing to rewind');
    }
    _debit(priceRewind, TxnScene.rewind);
    final id = _passOrder.removeLast();
    _passed.remove(id);
    return people.firstWhere((p) => p.id == id, orElse: () => people.first);
  }

  void blockUser(String userId) {
    final p = people.where((e) => e.id == userId).toList();
    final brief = p.isNotEmpty
        ? p.first.brief
        : conversations
              .firstWhere(
                (c) => c.peer.id == userId,
                orElse: () => conversations.first,
              )
              .peer;
    if (!blocked.any((b) => b.id == brief.id)) blocked.add(brief);
    conversations.removeWhere((c) => c.peer.id == userId);
  }

  void unblock(String userId) => blocked.removeWhere((b) => b.id == userId);

  // ---------------------------------------------------------------- 动态

  Moment postMoment(String text, List<String> images) {
    final m = Moment(
      id: _nextId(),
      author: me.brief,
      text: text,
      createdAt: DateTime.now().toUtc(),
      images: images,
      city: me.brief.city,
    );
    moments.insert(0, m);
    return m;
  }

  void toggleLike(String momentId, bool liked) {
    final i = moments.indexWhere((m) => m.id == momentId);
    if (i < 0) return;
    moments[i] = moments[i].copyWith(
      liked: liked,
      likeCount: moments[i].likeCount + (liked ? 1 : -1),
    );
  }

  MomentComment addComment(String momentId, String text, {GiftItem? gift}) {
    if (gift != null) {
      _debit(gift.coins, TxnScene.gift, note: '送出「${gift.name}」');
    }
    final c = MomentComment(
      id: _nextId(),
      author: me.brief,
      createdAt: DateTime.now().toUtc(),
      text: text,
      gift: gift,
    );
    momentComments.putIfAbsent(momentId, () => []).add(c);
    final i = moments.indexWhere((m) => m.id == momentId);
    if (i >= 0) {
      moments[i] = moments[i].copyWith(
        commentCount: moments[i].commentCount + 1,
      );
    }
    return c;
  }

  // ---------------------------------------------------------------- 变现

  QuotaInfo buyScoopPack() {
    _debit(priceScoopPack, TxnScene.gift, note: '捞瓶次数包 ×$scoopPackSize');
    quota = quota.addScoop(scoopPackSize);
    return quota;
  }

  /// 联调用的订单表。key 是 order_no。
  final Map<String, Map<String, dynamic>> orders = {};
  int _orderSeq = 0;

  /// 下单：只落一笔 pending，**不加币**。加币发生在结算时，与真实链路一致。
  ///
  /// 旧的 `recharge()` 在这里直接加币，那正是「下完单就当作已到账」这个 bug 的
  /// mock 侧镜像——两边一起改掉，否则 widget test 会测出一个不存在的世界。
  Map<String, dynamic> createOrder(RechargePackage pkg) {
    final no = 'MOCK${(++_orderSeq).toString().padLeft(6, '0')}';
    orders[no] = {
      'order_no': no,
      'status': 'pending',
      'coins': pkg.coins,
      // RechargePackage 只有标签没有数值（它的 fromJson 就是这么长的），
      // 所以这里给 price_label，PayOrderInfo 会优先用它。
      'price_label': pkg.priceLabel,
      'price_fen': 0,
      'currency': '',
      'platform': 'app',
      'created_at': DateTime.now().toUtc().toIso8601String(),
      'paid_at': null,
    };
    return {
      'order_no': no,
      'pay_params': {'mock': true, 'order_no': no},
    };
  }

  Map<String, dynamic> orderStatus(String orderNo) =>
      orders[orderNo] ?? {'order_no': orderNo, 'status': 'pending', 'coins': 0};

  List<Map<String, dynamic>> orderList() =>
      orders.values.toList().reversed.toList();

  /// 结算。成功时才加币，且对同一单幂等——真实链路的 `HandleCallback` 也是这么做的。
  void settleMock(String orderNo, bool success) {
    final o = orders[orderNo];
    if (o == null || o['status'] != 'pending') return;
    if (!success) {
      o['status'] = 'failed';
      return;
    }
    o['status'] = 'paid';
    o['paid_at'] = DateTime.now().toUtc().toIso8601String();
    credit(o['coins'] as int, TxnScene.recharge, note: '充值 · $orderNo');
  }

  int doCheckin() {
    if (checkin.checkedToday) return 0;
    checkin = checkin.checkedIn();
    credit(checkin.todayReward, TxnScene.checkin);
    return checkin.todayReward;
  }

  int doAdReward() {
    if (checkin.adDone >= checkin.adTotal) return 0;
    checkin = checkin.watchedAd();
    credit(checkin.adCoins, TxnScene.adReward);
    return checkin.adCoins;
  }

  List<RelationItem> relations(String tab) {
    final base = people.take(5).toList();
    return [
      for (var i = 0; i < base.length; i++)
        RelationItem(
          user: base[i].brief,
          interactionCount: switch (tab) {
            'i-liked' => 4 + i * 3,
            'viewed' => 1 + i,
            _ => 24 - i * 5,
          },
          strength: switch (tab) {
            'i-liked' => 40 - i * 6,
            'viewed' => 12 + i * 3,
            _ => 72 - i * 14,
          }.clamp(1, 100),
          stage: RelationStage.fromScore(switch (tab) {
            'i-liked' => 40 - i * 6,
            'viewed' => 12 + i * 3,
            _ => 72 - i * 14,
          }),
        ),
    ];
  }

  void dispose() => _incoming.close();
}
