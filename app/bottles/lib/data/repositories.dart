import '../core/config/remote_config.dart';
import '../domain/models/bottle.dart';
import '../domain/models/place.dart';
import '../domain/models/chat.dart';
import '../domain/models/engagement.dart';
import '../domain/models/moment.dart';
import '../domain/models/user.dart';
import '../domain/models/wallet.dart';

/// 登录结果，与现有 `/api/auth/login` 同构：`{token, user, is_new}`。
class AuthResult {
  const AuthResult({
    required this.token,
    required this.profile,
    required this.isNew,
  });

  final String token;
  final UserProfile profile;

  /// 新注册用户要先走「完善资料」两步引导。
  final bool isNew;
}

abstract class AuthRepository {
  /// `POST /api/auth/otp/send`
  ///
  /// 手机号与邮箱**二选一**：给了 [email] 走邮件，否则用 [dialCode] + [phone] 发短信。
  ///
  /// [purpose] 决定这个码能用在哪：`register` / `reset` / `login`。
  /// 服务端把用途和码绑在一起——注册场景发的码**无法**拿去重置密码。
  Future<void> sendOtp({
    String? dialCode,
    String? phone,
    String? email,
    String purpose = 'login',
  });

  /// `POST /api/auth/register` —— 验证码 + 密码，一次建号。
  Future<AuthResult> register({
    required String code,
    required String password,
    String? dialCode,
    String? phone,
    String? email,
  });

  /// `POST /api/auth/login/password` —— 日常登录。
  Future<AuthResult> loginWithPassword({
    required String password,
    String? dialCode,
    String? phone,
    String? email,
  });

  /// `POST /api/auth/password/reset` —— 忘记密码；也是验证码时代的老账号补设密码的唯一入口。
  Future<AuthResult> resetPassword({
    required String code,
    required String password,
    String? dialCode,
    String? phone,
    String? email,
  });

  /// `POST /api/auth/otp/verify`
  ///
  /// 身份参数与 [sendOtp] 一一对应——发哪个渠道就用哪个渠道验。
  Future<AuthResult> verifyOtp({
    required String code,
    String? dialCode,
    String? phone,
    String? email,
  });

  /// `POST /api/auth/google` / `POST /api/auth/apple`
  ///
  /// [idToken] 是 Google / Apple 官方 SDK 返回的 ID Token，服务端按 JWKS 验签。
  /// 尚未接入 SDK 时传 null——那样后端会因缺 id_token 拒绝，这是预期行为。
  Future<AuthResult> loginWithProvider(String provider, {String? idToken});

  /// `POST /api/auth/google/bind` / `POST /api/auth/apple/bind`
  ///
  /// 与登录**同一套验签**，区别只在最后一步：登录按 sub 查用户，
  /// 绑定把 sub 写到当前登录用户上。
  ///
  /// 该第三方账号已被别人绑定时抛 [ApiException]，code 为
  /// [ErrCode.oauthAlreadyBound]。
  Future<void> bindOAuth(String provider, String idToken);

  /// `POST /api/auth/unbind`
  ///
  /// 解绑后若没有任何可登录方式，服务端拒绝并返回
  /// [ErrCode.oauthLastMethod]——那会把用户锁在门外。
  Future<void> unbindOAuth(String provider);

  /// `GET /api/user/profile`
  Future<UserProfile> fetchProfile();

  /// `POST /api/user/update`
  ///
  /// [lat] 与 [lng] **必须成对传**——服务端只在两者都有时才写库
  /// （半个坐标算不出距离，不如不写）。
  /// [birthday] 为 `YYYY-MM-DD`；给了它服务端会据此重算 age。
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
  });

  /// `DELETE /api/user/account` —— Apple 5.1.1(v) 强制要求 App 内可删号。
  Future<void> deleteAccount();

  /// `GET /api/profile-options` —— 「完善资料」页的语言 / 兴趣 / 年龄范围（后台可配）。
  ///
  /// 空列表 = 后台没配，调用方回退到内置 `Catalog`，绝不能因此卡住注册流程。
  Future<ProfileOptions> profileOptions();

  /// `GET /api/legal/:doc` —— 用户协议 / 隐私政策正文（[doc] = `terms` / `privacy`）。
  ///
  /// 挂在 AuthRepository 上是因为**登录页就要能点开**，那时还没有 token。
  /// 返回空串 = 后台没配，调用方回退到内置 H5 地址，不能开天窗。
  Future<String> legalDoc(String doc, {required String lang});

  /// `GET /api/app-config` —— 运营可配的文案、「我的」入口显隐、客服。
  ///
  /// 和 [legalDoc] 同理挂在这里：冷启动时还没登录就要用（服务端免鉴权，
  /// 没 token 就按默认租户下发）。文案为空 = 后台没配，调用方回退内置 ARB。
  Future<AppRemoteConfig> appConfig({required String lang});
}

abstract class BottleRepository {
  /// `GET /api/bottle/quota`
  Future<QuotaInfo> quota();

  /// `GET /api/hook-count` + 在线数，用于首页头部。
  Future<SeaCondition> seaCondition();

  /// `GET /api/bottle/scoop`。空数组不是错误，走空态 Z1。
  Future<Bottle?> scoop({String? tag, String? language});

  /// 海面上漂着的那几个瓶子。
  ///
  /// 点它们只浮出轨迹预览气泡、**不消耗次数**，读的是捞瓶 feed 里已有的 trace 摘要。
  /// 对应 `GET /api/bottle/scoop?peek=1`——后端需要给 feed 接口加一个只读参数，
  /// 数据本身是现成的。首页每天被打开十几次，不能只有两个可点区。
  Future<List<Bottle>> oceanPeek({String? tag, int limit = 5});

  /// `POST /api/bottle/create`
  /// [place] 为 null 表示不带地点——用户可以显式选择「不显示地点」。
  Future<Bottle> cast({
    required String content,
    List<String> images,
    List<String> tags,
    bool cityOnly = false,
    Place? place,
  });

  /// `GET /api/bottle/:id`
  Future<Bottle> detail(String id);

  /// `GET /api/bottle/:id/replies`
  Future<List<BottleReply>> replies(String bottleId);

  /// `POST /api/bottle/:id/reply`
  Future<BottleReply> reply(String bottleId, String content);

  /// `POST /api/bottle/reply/:rid/unlock` —— 解锁**这一条**回信，扣 `price_unlock`，scene=unlock。
  ///
  /// 按条解锁，不整瓶打包：用户想看哪条付哪条（真机反馈「一次性解锁不合理」）。
  /// 瓶主 / 回信本人服务端直接放行不扣费；已解锁过的再调也不重复扣。
  Future<void> unlockReply(String replyId);

  /// 放回海里：只记 `MatchLog(action=skip)`，不写 view，不扣次数。
  Future<void> putBack(String bottleId);

  /// 收藏 / 取消收藏。进「我的瓶子 · 已收藏」分页，与点赞是两件事。
  /// `POST /api/collection/add` · `POST /api/collection/remove`
  Future<void> setCollected(String bottleId, bool collected);

  /// `GET /api/bottle/mine`
  Future<List<Bottle>> mine(BottleMineTab tab);

  /// `GET /api/bottle/:id/trace` —— 由 MatchLog 聚合，受 `bottle_trace_enabled` 控制。
  Future<List<BottleTraceNode>> trace(String bottleId);
}

/// 发现页定价配置（进页面拉一次）。二次确认弹层显示的数字必须与服务端真扣的一致。
class DiscoverPricing {
  const DiscoverPricing({
    required this.rewindPrice,
    required this.skipChargeEnabled,
    required this.skipPrice,
  });

  /// 撤回单价。
  final int rewindPrice;

  /// 左滑跳过是否扣币（付费功能，后台默认关）。
  final bool skipChargeEnabled;

  /// 左滑跳过单价。
  final int skipPrice;

  static const zero = DiscoverPricing(
    rewindPrice: 0,
    skipChargeEnabled: false,
    skipPrice: 0,
  );
}

/// 一道答题匹配题：key + 问题 + 选项，筛选页渲染用。
class QuizQuestion {
  const QuizQuestion({
    required this.key,
    required this.question,
    required this.options,
  });

  final String key;
  final String question;
  final List<String> options;

  factory QuizQuestion.fromJson(Map<String, dynamic> j) => QuizQuestion(
    key: (j['key'] ?? '').toString(),
    question: (j['question'] ?? '').toString(),
    options: [
      for (final o in (j['options'] as List? ?? const [])) o.toString(),
    ],
  );
}

/// 答题匹配题库 + 我的答案（进筛选页拉一次）。
///
/// 开关关时 `enabled=false`，筛选页回落语言/兴趣维度。答案是
/// qkey → 选中项下标（字符串），后端存紧凑串 "qkey:idx,..."。
class DiscoverQuiz {
  const DiscoverQuiz({
    required this.enabled,
    required this.questions,
    required this.answers,
  });

  final bool enabled;
  final List<QuizQuestion> questions;
  final Map<String, String> answers;

  static const empty = DiscoverQuiz(enabled: false, questions: [], answers: {});
}

abstract class DiscoverRepository {
  /// `GET /api/discover/users`（待新增）。一次拉 15~20 人，剩 5 张时静默续拉。
  Future<List<UserProfile>> users({
    required DiscoverFilter filter,
    String? tab,
  });

  /// `POST /api/relation/like`（右滑）
  Future<void> like(String userId);

  /// 左滑同样落库 `MatchLog(action=skip)`，否则下次拉人去重不掉。
  Future<void> pass(String userId);

  /// `GET /api/user/card/:id`
  Future<UserProfile> card(String userId);

  /// 符合当前筛选的人数，用于筛选页的「应用筛选 · N 人符合」。
  Future<int> matchCount(DiscoverFilter filter);

  /// `GET /api/discover/rewind` —— 发现页定价配置：撤回单价 + 左滑跳过扣币开关/单价。
  ///
  /// 单独拉一次而不是写死：价格改了要立刻生效，二次确认弹层上显示的数字
  /// 必须和服务端真扣的一致，否则就是欺诈式扣费。
  Future<DiscoverPricing> discoverConfig();

  /// `POST /api/discover/rewind` —— 扣币撤回上一次左滑，返回被捞回来的人。
  ///
  /// 只撤左滑：右滑已经写进关系链、可能触发了匹配与通知，回滚不了。
  /// 余额不足抛 [ErrCode.insufficientBalance]，调用方要弹充值而不是通用 toast。
  Future<UserProfile> rewind();

  /// `GET /api/discover/quiz` —— 答题匹配题库 + 我的答案。开关关时 `enabled=false`。
  Future<DiscoverQuiz> quiz();

  /// `POST /api/discover/quiz` —— 存我的答案（紧凑串 "qkey:idx,..."，空串合法=清空）。
  Future<void> saveQuiz(String answers);
}

abstract class ChatRepository {
  /// `GET /api/chat/list`
  Future<List<Conversation>> conversations();

  /// `GET /api/chat/:id/messages`
  Future<List<ChatMessage>> messages(String conversationId);

  /// `POST /api/chat/:id/send`
  Future<ChatMessage> send(String conversationId, String text);

  Future<ChatMessage> sendImage(String conversationId, String localPath);

  /// `POST /api/chat/:id/gift` —— 扣币事务 + ItemOrder + Charm 累加 + 通知。
  Future<ChatMessage> sendGift(String conversationId, GiftItem gift, int qty);

  /// `POST /api/chat/start` —— 扣 `price_chat`，scene=chat。
  Future<Conversation> startWith(String userId);

  /// 某个会话的实时消息流(已按会话过滤)。聊天室用它渲染当前会话。
  /// 断线重连后由 [messages] 拉增量补齐。
  Stream<ChatMessage> incoming(String conversationId);

  /// 全局实时消息流(所有会话,不过滤)。常驻分发器用它更新会话列表预览 / 未读 /
  /// Tab 红点,不依赖聊天室是否打开。
  Stream<ChatMessage> allMessages();

  /// `POST /api/report`
  Future<void> report(String userId, ReportReason reason);

  /// `POST /api/block`
  Future<void> block(String userId);

  /// `GET /api/block/list`
  Future<List<UserBrief>> blocklist();

  /// `POST /api/block/remove`
  Future<void> unblock(String userId);
}

abstract class MomentRepository {
  /// `GET /api/moment/feed` —— 受 `square_enabled` 控制。
  Future<List<Moment>> feed(String tab);

  /// `GET /api/moment/user/:id` —— 某用户的公开动态（用户主页 C3 的「TA 的动态」预览）。
  Future<List<Moment>> userMoments(String userId);

  /// `GET /api/moment/:id`
  Future<Moment> detail(String id);

  /// `GET /api/moment/:id/comments`
  Future<List<MomentComment>> comments(String momentId);

  /// `POST /api/moment/:id/like`
  ///
  /// ⚠️ 后端是 **toggle**：忽略 [liked] 入参，自己翻转并返回翻转后的状态。
  /// 传 [liked] 只是为了 mock 能表现一致；**以返回值为准**校正本地乐观更新。
  Future<bool> like(String momentId, bool liked);

  /// `POST /api/moment/:id/comment`
  Future<MomentComment> comment(String momentId, String text);

  /// `POST /api/moment/:id/gift`
  Future<MomentComment> giftComment(String momentId, GiftItem gift);

  /// `POST /api/upload` —— 上传一张本地图片，返回**服务端可访问的 URL**。
  ///
  /// ⚠️ 必须先走这一步再发动态。直接把本地路径塞进 images，库里存下的会是
  /// `/data/user/0/<包名>/cache/...` 这种设备私有路径，换台手机永远打不开。
  Future<String> uploadImage(String localPath);

  /// `POST /api/moment`
  ///
  /// [images] 要的是 [uploadImage] 返回的 URL，不是本地路径。
  /// [place] 为 null 表示不带地点。
  /// [visible] 只有 `public` / `self` 两个值（后端 `moment.service` 把别的都当 public）。
  Future<Moment> post({
    required String text,
    List<String> images,
    Place? place,
    String visible,
  });

  /// 关注 / 取关：`POST /api/relation/like` · `POST /api/relation/unlike`。
  ///
  /// 后端没有独立的 follow 表——`model.Relation` 的 `Type(like/viewed/friend)`
  /// 就是这层语义。
  Future<void> setFollow(String userId, bool follow);
}

/// 逆地理：经纬度 → 地址。
///
/// **服务端代理**（`GET /geo/regeo`）而不是客户端直接调 Google——
/// Geocoding 的 Key 只在服务端（后台「App 地理」分组），不下发。
/// 地图渲染那两把 Key 是另一回事，它们必须打进包里，防护靠包名/Bundle ID 限制。
abstract class GeoRepository {
  /// 拿不到地址时返回 [lat]/[lng] 原样、name 与 city 为空的 Place，
  /// **不抛异常**——没配 Key、供应商超时都属于正常降级，
  /// 用户还是能把这个点选下去，只是没有好看的地名。
  Future<Place> regeo(double lat, double lng);
}

abstract class WalletRepository {
  /// `GET /api/wallet/balance`
  Future<Wallet> wallet();

  /// `GET /api/wallet/txns`
  Future<List<WalletTxn>> txns();

  /// `GET /api/pay/packages`
  Future<List<RechargePackage>> packages();

  /// `POST /api/pay/order` —— **只下单，不代表已支付**。
  ///
  /// 拿到 order_no 之后由支付状态机接管：拉起渠道 → 轮询 [orderStatus]。
  /// 旧实现在这里直接 `return wallet()`，等于下完单就当作已到账。
  Future<PendingOrder> recharge(RechargePackage pkg, PayChannel channel);

  /// `GET /api/pay/order/:orderNo` —— 支付成功与否的唯一判据。
  Future<PayOrderInfo> orderStatus(String orderNo);

  /// `GET /api/pay/orders` —— H10 充值记录。
  Future<List<PayOrderInfo>> orders();

  /// `POST /api/pay/mock/settle` —— 仅联调渠道可用，开关关闭时服务端当作接口不存在。
  Future<void> settleMock(String orderNo, bool success);

  /// `GET /api/item/list`
  Future<List<GiftItem>> gifts();

  /// `GET /api/item/orders` —— 道具流水（买进 / 送出 / 收到）。
  Future<List<ItemRecord>> itemRecords();

  /// `GET /api/item/my` —— 每种道具的持有数量（itemId → 数量）。
  ///
  /// 送礼是**库存制**：先买进背包（`ItemOrder` 且 `target_id=0`），送出时消耗一行。
  /// 没有这个接口就看不到「我有几个」，礼物面板只能盲送。
  Future<Map<String, int>> myItems();

  /// `POST /api/item/buy` —— 买进背包（不指定赠送对象），scene=gift。
  Future<void> buyItem(String itemId);

  /// `POST /api/item/buy` —— 次数包，scene=gift。
  Future<QuotaInfo> buyScoopPack();

  /// `GET /api/checkin/status`
  Future<CheckinStatus> checkinStatus();

  /// `POST /api/checkin`
  Future<int> checkin();

  /// `POST /api/ad/reward` —— App 端需换 AdMob，这里只管入账。
  Future<int> adReward();

  /// 关系列表：`GET /api/relation/list` / `i-viewed`
  Future<List<RelationItem>> relations(String tab);
}

abstract class NotifyRepository {
  /// `GET /api/notify/list`
  Future<List<NotificationItem>> list();

  /// `GET /api/notify/unread`
  Future<int> unreadCount();

  /// `POST /api/notify/read`
  Future<void> markAllRead();

  /// `POST /api/push/device`（待新增）—— FCM / APNs 设备 token 注册。
  Future<void> registerDevice(String token);
}

abstract class SparkRepository {
  /// `POST /api/spark/accept` —— 真人火花点「去聊聊」时换一个免费会话。
  /// 服务端会校验这对确实被匹配过；过期返回错误，调用方提示即可。
  Future<String> accept(String peerId);
}
