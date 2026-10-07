/// 后端错误码 —— 与 `server/internal/common/errs` 对齐。
class ErrCode {
  const ErrCode._();

  static const int badRequest = 1001;
  static const int serverError = 1002;
  static const int rateLimited = 1003;
  static const int forbidden = 1004;
  static const int notFound = 1005;
  static const int unauthorized = 2001;
  static const int loginFailed = 2002;
  static const int needVerify = 2003;

  /// 第三方登录时该邮箱已注册。不是错误提示，是要跳 A2k 冲突屏——
  /// 产品明确选择「不自动按邮箱合并账号」，让用户自己去绑定。
  static const int oauthEmailTaken = 2004;

  /// 绑定时该第三方账号已被其他用户绑定。弹 A6b，**不要说是哪个账号**。
  static const int oauthAlreadyBound = 2005;

  /// 解绑后将没有任何可登录方式。拦住，否则用户把自己锁在门外。
  static const int oauthLastMethod = 2006;

  static const int contentBlocked = 3002;

  /// 金币不足。拦截后弹「余额不足」模态（Z4），不要弹通用 toast。
  /// 注意是 5xxx 段（钱包支付），不是 3xxx（漂流瓶）。
  static const int insufficientBalance = 5001;

  /// 礼物 / 道具库存不足。聊天送礼是库存制,拦截后弹「去道具页购买」引导,
  /// 与金币不足(5001,跳充值页买金币)是两回事——这里跳道具页用金币买礼物入库。
  static const int itemInsufficient = 5005;

  /// 每日次数用完。拦截后弹次数包模态（H5），这是转化入口，不是错误提示。
  static const int quotaExceeded = 3003;
}

/// 业务异常。网络层已把 `{code, msg, data}` 解包，页面只需要认 [code]。
class ApiException implements Exception {
  ApiException(this.code, this.message, {this.data});

  /// 无法归类的本地/网络异常。
  ApiException.network(this.message)
      : code = -1,
        data = null;

  final int code;
  final String message;
  final Object? data;

  bool get isNetwork => code == -1;

  /// 只有「未登录/登录失效」才该踢回登录页。
  /// forbidden(1004) 覆盖封禁、注销、开关关闭等场景，不能一律当成掉登录态。
  bool get isAuthExpired => code == ErrCode.unauthorized;
  bool get isQuotaExceeded => code == ErrCode.quotaExceeded;
  bool get isOAuthEmailTaken => code == ErrCode.oauthEmailTaken;
  bool get isInsufficientBalance => code == ErrCode.insufficientBalance;
  bool get isItemInsufficient => code == ErrCode.itemInsufficient;

  @override
  String toString() => 'ApiException($code): $message';
}
