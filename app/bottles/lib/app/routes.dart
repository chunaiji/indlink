/// 路由表。
///
/// 17 个模块收敛为 5 个底部 Tab：模块 2/3/4/6 各占一个，其余全部挂在「我的」下层。
/// 与小程序的关键差异：App 不需要「工具形态 / 社交形态」那套审核伪装，
/// **5 个 Tab 固定常驻**，不走 `GET /api/tabs` 远程下发。
class Routes {
  const Routes._();

  // 鉴权
  static const splash = '/splash';
  static const login = '/login';
  static const otp = '/login/otp';

  /// 注册与找回密码是两条独立流程,不能塞进登录页——
  /// 它们各自要走「标识 → 验证码 → 设密码」三步。
  static const register = '/login/register';
  static const forgotPassword = '/login/forgot';

  static const onboarding = '/onboarding';

  // Tab 1 · 海洋
  static const ocean = '/ocean';
  static const bottleWrite = '/ocean/write';
  static const myBottles = '/ocean/mine';
  static String bottleDetail(String id) => '/ocean/bottle/$id';
  static String driftMap(String id) => '/ocean/map/$id';

  // Tab 2 · 发现
  static const discover = '/discover';
  static const discoverFilter = '/discover/filter';
  static String userProfile(String id) => '/discover/user/$id';

  // Tab 3 · 消息
  static const chats = '/chats';
  static String chatRoom(String id) => '/chats/$id';

  // Tab 4 · 动态
  static const moments = '/moments';
  static const momentPost = '/moments/post';
  static String momentDetail(String id) => '/moments/$id';

  // Tab 5 · 我的
  static const me = '/me';
  static const wallet = '/me/wallet';
  static const recharge = '/me/recharge';

  /// H7a / H7b / H8 / H9 —— **四个阶段共用一条路由**。
  ///
  /// 拆成四条会让「付完按返回」退回「支付中」，这是支付页最常见的返回栈事故。
  /// [from] 是被打断的原场景，H8 的主按钮照它回跳：多数充值是被 H5 次数用尽
  /// 或 Z4 余额不足打断后进来的，付完回到「我的」等于让用户自己找回去。
  static String payFlow(String orderNo, {String? from}) =>
      '/me/recharge/pay/$orderNo'
      '${from == null || from.isEmpty ? '' : '?from=${Uri.encodeComponent(from)}'}';

  /// H10 充值记录。
  static const payRecords = '/me/recharge/records';

  /// H11 退款已撤销。
  static const voided = '/me/wallet/voided';

  /// 道具背包。与 [recharge] 是两件事：充值买金币，这里用金币买道具。
  static const items = '/me/items';
  static const rewards = '/me/rewards';
  static const relations = '/me/relations';
  static const notifications = '/me/notifications';
  static const blocklist = '/me/blocklist';
  static const settings = '/me/settings';

  /// 编辑资料（头像 / 昵称 / 出生日期）。完善资料（onboarding）是注册后那一次，不复用。
  static const profileEdit = '/me/profile';

  /// A6 账号与安全（绑定 Google / Apple）。
  /// A2k 提示用户「到账号与安全里绑定」，指的就是这里。
  static const accountSecurity = '/me/account-security';

  /// 日志查看(开发者面板)。设置页「关于」连点 7 次进入,不在任何导航里露出。
  static const logViewer = '/me/log-viewer';

  /// 用户协议 / 隐私政策（[doc] = `terms` / `privacy`）。
  ///
  /// 顶层路由而不是挂在「我的」下面：登录页也要能点开，那时还没有登录态。
  static String legal(String doc) => '/legal/$doc';
  static const legalPrefix = '/legal/';

  static const tabRoots = [ocean, discover, chats, moments, me];
}
