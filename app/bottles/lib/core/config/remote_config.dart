/// 运营远程配置（`GET /api/app-config`）。
///
/// 三层来源，后者覆盖前者——与 `LogConfig` 同一套路数：
///   1. [AppRemoteConfig.builtin] —— 全空 / 全开，就是 App 不联网时的样子
///   2. 上次缓存在 Prefs 里的服务端响应 —— 冷启动第一帧就能用
///   3. 本次拉取 —— 回来后覆盖，Riverpod 自然重建
///
/// **空串一律表示「后台没配过」**，调用方回退到 ARB 内置文案。服务端那批
/// `app_ui_*` / `app_contact_*` 键的默认值也全是空串，两边对齐，所以
/// 「运营没动过」与「App 内置」永远是同一个结果。
///
/// 开关默认全 true 同理：那是 App 今天的样子。默认 false 等于发版即砍功能。
class AppRemoteConfig {
  const AppRemoteConfig({
    required this.supportText,
    required this.supportImage,
    required this.showWallet,
    required this.showRecharge,
    required this.showWalletLog,
    required this.showItems,
    required this.showBlocklist,
    required this.showPrivacyGate,
    required this.anonSender,
    required this.oceanTitle,
    required this.quotaTitle,
    required this.quotaBody,
    required this.googleClientId,
    required this.wechatLogin,
    required this.alipayLogin,
    required this.googleLogin,
    required this.appleLogin,
    required this.chatStartPrice,
    required this.chatMsgPrice,
    required this.chatFreeMsgs,
    required this.loginPhone,
    required this.loginEmail,
    required this.rewindPrice,
    required this.skipChargeEnabled,
    required this.skipPrice,
    this.splashImages = const [],
    this.oceanBottleCount = 5,
  });

  const AppRemoteConfig.builtin()
    : supportText = '',
      supportImage = '',
      showWallet = true,
      showRecharge = true,
      showWalletLog = true,
      showItems = true,
      showBlocklist = true,
      showPrivacyGate = false,
      anonSender = '',
      oceanTitle = '',
      quotaTitle = '',
      quotaBody = '',
      googleClientId = '',
      wechatLogin = false,
      alipayLogin = false,
      googleLogin = true,
      appleLogin = true,
      chatStartPrice = 5,
      chatMsgPrice = 0,
      chatFreeMsgs = 0,
      loginPhone = true,
      loginEmail = true,
      rewindPrice = 10,
      skipChargeEnabled = false,
      skipPrice = 1,
      splashImages = const [],
      oceanBottleCount = 5;

  /// 客服弹框文案。空 = App 回退到内置 `mailto:`，所以不配也一定有求助入口。
  final String supportText;

  /// 客服图片（一般是二维码）。不分语种。
  final String supportImage;

  final bool showWallet;
  final bool showRecharge;
  final bool showWalletLog;
  final bool showItems;
  final bool showBlocklist;

  /// 首次登录前隐私授权弹框（合规）。**新功能，默认关**——上线前由后台打开，
  /// 与其它开关「默认开」相反：默认弹一个空内容的合规框反而是 bug。
  final bool showPrivacyGate;

  final String anonSender;
  final String oceanTitle;

  /// 支持占位符 `{n}`（每日次数）——见 [quotaTitleOr]。
  final String quotaTitle;
  final String quotaBody;

  /// Google 登录的 **Web** client ID，Android 拿它当 `serverClientId`。
  ///
  /// 没有它 `google_sign_in` 返回的 `idToken` 为 null，登录请求根本发不出去
  /// ——项目里没有 `google-services.json`，也就没有 `default_web_client_id`
  /// 可读。空串 = 后台没配，调用方据此走「未配置」提示，而不是让用户
  /// 点一下没反应。
  ///
  /// 与服务端验签同源（都来自 `app_google_client_id`），所以不会出现
  /// 「客户端拿 A 去签、服务端拿 B 去验」那种查半天的错配。
  final String googleClientId;

  /// 四个第三方登录渠道是否可用。来源是后台「服务商 → 登录」页：启用且填齐。
  /// Apple 还要再过一道平台门（见 showAppleSignIn）——配置能关掉它，但开不出来。
  /// 内置默认 Google / Apple 开、微信支付宝关，跟升级服务端之前的行为一致。
  final bool wechatLogin;
  final bool alipayLogin;
  final bool googleLogin;
  final bool appleLogin;

  /// 聊天扣费规则（后台「价格」分组）：开聊扣 [chatStartPrice]，之后每条扣
  /// [chatMsgPrice]，每个会话前 [chatFreeMsgs] 条免费。App 只用来**显示**
  /// （打招呼按钮上的价、聊天页顶部的提示、余额预判）；真正扣多少以服务端为准。
  /// 内置值 5 / 0 / 0 与服务端默认一致：没配过 = 开聊 5 币、按条不收费。
  final int chatStartPrice;
  final int chatMsgPrice;
  final int chatFreeMsgs;

  /// 登录 / 注册开放哪些标识（后台「App 登录」分组），默认都开。
  /// 两个都关时按都开处理——配置错了也不能把所有人锁在门外，见 [loginChannelsBoth]。
  final bool loginPhone;
  final bool loginEmail;

  /// 手机号是否可用（含「两个都关 = 都开」的兜底）。
  bool get phoneLoginOn => loginPhone || !loginEmail;

  /// 邮箱是否可用（含兜底）。
  bool get emailLoginOn => loginEmail || !loginPhone;

  /// 两种都可用时才显示渠道切换。
  bool get loginChannelsBoth => phoneLoginOn && emailLoginOn;

  /// 发现页定价：撤回单价、左滑是否扣币及单价。随启动配置下来（免登录），
  /// 第一次点撤回时就有数——以前进发现页才拉，弹层上写过 0 币。内置值与服务端默认一致。
  final int rewindPrice;
  final bool skipChargeEnabled;
  final int skipPrice;

  /// 后台配的启动页图片（最多 5 张）。拉到后静默下载进本地缓存，
  /// **下一次**冷启动才显示——启动页不能等网络，没缓存就用内置启动页。
  final List<String> splashImages;

  /// 海面同时漂几个瓶子（后台 5~9，客户端再夹一次防御）。
  final int oceanBottleCount;

  /// 服务端整份响应解析。任何字段缺失或类型不对都退回内置值，
  /// **不抛异常**——后台填错一个字不该让 App 起不来。
  factory AppRemoteConfig.fromJson(Map<String, Object?> json) {
    final support = _obj(json['support']);
    final mine = _obj(json['mine']);
    final copy = _obj(json['copy']);
    final auth = _obj(json['auth']);
    final pricing = _obj(json['pricing']);
    final splash = _obj(json['splash']);
    final ocean = _obj(json['ocean']);
    const b = AppRemoteConfig.builtin();
    return AppRemoteConfig(
      supportText: _str(support['text']),
      supportImage: _str(support['image']),
      showWallet: _bool(mine['wallet']) ?? b.showWallet,
      showRecharge: _bool(mine['recharge']) ?? b.showRecharge,
      showWalletLog: _bool(mine['wallet_log']) ?? b.showWalletLog,
      showItems: _bool(mine['items']) ?? b.showItems,
      showBlocklist: _bool(mine['blocklist']) ?? b.showBlocklist,
      showPrivacyGate: _bool(json['privacy_gate']) ?? b.showPrivacyGate,
      anonSender: _str(copy['anon_sender']),
      oceanTitle: _str(copy['ocean_title']),
      quotaTitle: _str(copy['quota_title']),
      quotaBody: _str(copy['quota_body']),
      googleClientId: _str(auth['google_client_id']),
      wechatLogin: _bool(auth['wechat']) ?? b.wechatLogin,
      alipayLogin: _bool(auth['alipay']) ?? b.alipayLogin,
      googleLogin: _bool(auth['google']) ?? b.googleLogin,
      appleLogin: _bool(auth['apple']) ?? b.appleLogin,
      chatStartPrice: _int(pricing['chat_start']) ?? b.chatStartPrice,
      chatMsgPrice: _int(pricing['chat_msg']) ?? b.chatMsgPrice,
      chatFreeMsgs: _int(pricing['chat_free_msgs']) ?? b.chatFreeMsgs,
      loginPhone: _bool(auth['phone']) ?? b.loginPhone,
      loginEmail: _bool(auth['email']) ?? b.loginEmail,
      rewindPrice: _int(pricing['rewind']) ?? b.rewindPrice,
      skipChargeEnabled:
          _bool(pricing['skip_charge_enabled']) ?? b.skipChargeEnabled,
      skipPrice: _int(pricing['skip_price']) ?? b.skipPrice,
      splashImages: _strList(splash['images']),
      oceanBottleCount: (_int(ocean['bottle_count']) ?? b.oceanBottleCount)
          .clamp(5, 9),
    );
  }

  /// 原样缓存服务端响应，下次冷启动直接喂回 [AppRemoteConfig.fromJson]。
  /// 存原始形状而不是拍平的字段：加字段时旧缓存仍能解析，不用管版本。
  Map<String, Object?> toJson() => {
    'support': {'text': supportText, 'image': supportImage},
    'mine': {
      'wallet': showWallet,
      'recharge': showRecharge,
      'wallet_log': showWalletLog,
      'items': showItems,
      'blocklist': showBlocklist,
    },
    'copy': {
      'anon_sender': anonSender,
      'ocean_title': oceanTitle,
      'quota_title': quotaTitle,
      'quota_body': quotaBody,
    },
    // 必须进缓存：冷启动第一帧走的是 Prefs 这份，漏了它就等于
    // 「第一次点 Google 登录必失败，重启一次才好」。
    'auth': {
      'google_client_id': googleClientId,
      'wechat': wechatLogin,
      'alipay': alipayLogin,
      'google': googleLogin,
      'apple': appleLogin,
      'phone': loginPhone,
      'email': loginEmail,
    },
    'privacy_gate': showPrivacyGate,
    'pricing': {
      'chat_start': chatStartPrice,
      'chat_msg': chatMsgPrice,
      'chat_free_msgs': chatFreeMsgs,
      'rewind': rewindPrice,
      'skip_charge_enabled': skipChargeEnabled,
      'skip_price': skipPrice,
    },
    'splash': {'images': splashImages},
    'ocean': {'bottle_count': oceanBottleCount},
  };

  /// 空 = 没配过，用 [builtin]（调用方传 ARB 文案进来）。
  /// 只 trim 判断、不 trim 返回值——运营特意加的首尾空格是他的事。
  static String or(String remote, String builtin) =>
      remote.trim().isEmpty ? builtin : remote;

  /// 次数用完弹框标题。后台文案里的 `{n}` 换成每日次数；
  /// 不写 `{n}` 也合法，那就是一句不带数字的话。
  String quotaTitleOr(String builtin, int quota) =>
      or(quotaTitle, builtin).replaceAll('{n}', '$quota');

  static Map<String, Object?> _obj(Object? v) =>
      v is Map ? v.cast<String, Object?>() : const {};

  static String _str(Object? v) => v is String ? v : '';

  /// 字符串数组：非字符串、空串一律丢掉，最多取 5 个。
  static List<String> _strList(Object? v) {
    if (v is! List) return const [];
    final out = <String>[
      for (final e in v)
        if (e is String && e.trim().isNotEmpty) e.trim(),
    ];
    return out.length > 5 ? out.sublist(0, 5) : out;
  }

  /// 价格类字段：认 num，也认 sysconfig 原样下来的数字字符串；负数当没配。
  static int? _int(Object? v) {
    final n = v is num ? v.toInt() : int.tryParse(v?.toString() ?? '');
    return n == null || n < 0 ? null : n;
  }

  /// 服务端这批键在 sysconfig 里存的是字符串，但 /app-config 已经转成了
  /// bool。两种都认，省得以后改下发形状时这里悄悄失效。
  static bool? _bool(Object? v) {
    if (v == null) return null;
    if (v is bool) return v;
    if (v is num) return v != 0;
    final s = v.toString();
    if (s.isEmpty) return null;
    return s != '0' && s.toLowerCase() != 'false';
  }
}
