import 'dart:async';

import 'package:flutter/foundation.dart' show debugPrint;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/network/api_exception.dart';
import '../../core/platform/oauth.dart';
import '../../core/providers.dart';
import '../../core/utils/birthday.dart';
import '../../data/repositories.dart';
import '../../domain/models/user.dart';

/// 验证码发往哪里。两条链路在服务端共用同一套频控与验证码键空间。
enum OtpChannel { phone, email }

/// 手机号登录页上的两种方式，并排 Tab 切换。
///
/// 只对手机号有意义：邮箱没有验证码登录（会和注册流程撞车）。
enum LoginMethod { password, code }

/// 当前处在认证的哪条流程上。
///
/// 三条流程共用一份状态（标识、验证码倒计时、密码），只有提交动作不同——
/// 拆成三个 Notifier 会让「填了邮箱去注册、发现已注册、切回登录」这种来回丢掉输入。
enum AuthFlow {
  /// 邮箱/手机号 + 密码
  login,

  /// 标识 → 验证码 → 设密码
  register,

  /// 标识 → 验证码 → 新密码。老账号补设密码也走这条
  reset,

  /// 手机号免密：标识 → 验证码 → 直接进
  otpLogin,
}

extension AuthFlowWire on AuthFlow {
  /// 服务端要的 purpose。验证码与用途绑定，注册的码换不成重置的码。
  String get purpose => switch (this) {
    AuthFlow.register => 'register',
    AuthFlow.reset => 'reset',
    _ => 'login',
  };
}

class OtpState {
  const OtpState({
    this.channel = OtpChannel.phone,
    this.method = LoginMethod.password,
    this.flow = AuthFlow.login,
    this.dialCode = '+91',
    this.phone = '',
    this.email = '',
    this.password = '',
    this.secondsLeft = 0,
    this.busy = false,
    this.error,
    this.conflictEmail,
    this.pendingProvider,
  });

  final OtpChannel channel;
  final LoginMethod method;
  final AuthFlow flow;
  final String dialCode;
  final String phone;
  final String email;
  final String password;

  /// 同一标识 60 秒内只发一次，倒计时期间重发按钮不可点。
  final int secondsLeft;

  final bool busy;
  final String? error;

  /// 第三方登录撞上「该邮箱已注册」(2004) 时的服务端提示语。
  ///
  /// 与 [error] 分开：那条走 toast，这条要跳一整屏（A2k）。
  /// 混在一起，「请用密码登录后去绑定」就会变成一个两秒后消失的提示，
  /// 而用户根本来不及理解该做什么。
  final String? conflictEmail;

  /// 第三方登录进行中的渠道（'google' / 'apple'）；非空时登录页盖 A2g 整屏等待层。
  final String? pendingProvider;

  bool get isEmail => channel == OtpChannel.email;

  /// 邮箱只有密码一条路——所以「验证码」那个 Tab 只在手机号下成立。
  bool get isCodeMethod => !isEmail && method == LoginMethod.code;

  /// A3 顶部「已发送到 …」显示的目标，按渠道取。
  String get target => isEmail ? email : '$dialCode $phone';

  /// 邮箱只做形状校验，真正能不能收到由后端那封信决定。
  static final _emailRe = RegExp(r'^[^@\s]+@[^@\s.]+(\.[^@\s.]+)+$');

  /// 标识本身填对了没有（与倒计时、busy 无关）。
  bool get identityValid =>
      isEmail ? _emailRe.hasMatch(email) : phone.length >= 6;

  bool get canSend => identityValid && !busy && secondsLeft == 0;

  /// 与服务端的 `minPasswordLen` 对齐。改这里要同时改 `user/password.go`。
  static const minPasswordLen = 8;

  bool get passwordValid => password.length >= minPasswordLen;

  /// 密码登录能不能提交。
  bool get canLogin => identityValid && password.isNotEmpty && !busy;

  /// 注册 / 重置的最后一步能不能提交（验证码由页面自己校验长度）。
  bool get canSubmitPassword => identityValid && passwordValid && !busy;

  OtpState copyWith({
    OtpChannel? channel,
    LoginMethod? method,
    AuthFlow? flow,
    String? dialCode,
    String? phone,
    String? email,
    String? password,
    int? secondsLeft,
    bool? busy,
    String? error,
    bool clearError = false,
    String? conflictEmail,
    bool clearConflict = false,
    String? pendingProvider,
    bool clearPending = false,
  }) {
    return OtpState(
      channel: channel ?? this.channel,
      method: method ?? this.method,
      flow: flow ?? this.flow,
      dialCode: dialCode ?? this.dialCode,
      phone: phone ?? this.phone,
      email: email ?? this.email,
      password: password ?? this.password,
      secondsLeft: secondsLeft ?? this.secondsLeft,
      busy: busy ?? this.busy,
      error: clearError ? null : (error ?? this.error),
      conflictEmail: clearConflict
          ? null
          : (conflictEmail ?? this.conflictEmail),
      pendingProvider: clearPending
          ? null
          : (pendingProvider ?? this.pendingProvider),
    );
  }
}

final otpProvider = NotifierProvider<OtpNotifier, OtpState>(OtpNotifier.new);

class OtpNotifier extends Notifier<OtpState> {
  Timer? _timer;

  @override
  OtpState build() {
    ref.onDispose(() => _timer?.cancel());
    return const OtpState();
  }

  void setDialCode(String code) => state = state.copyWith(dialCode: code);

  void setPhone(String phone) =>
      state = state.copyWith(phone: phone.trim(), clearError: true);

  /// 邮箱统一小写：服务端也做同样归一，两边不一致会查不到同一个账号。
  void setEmail(String email) => state = state.copyWith(
    email: email.trim().toLowerCase(),
    clearError: true,
  );

  /// 切渠道只清错误，输入内容留着——来回切一次就被清空很恼人。
  void setChannel(OtpChannel channel) =>
      state = state.copyWith(channel: channel, clearError: true);

  void setPassword(String v) =>
      state = state.copyWith(password: v, clearError: true);

  /// 切登录方式。同时把 flow 对齐——验证码方式走 otpLogin（purpose=login）。
  void setMethod(LoginMethod m) => state = state.copyWith(
    method: m,
    flow: m == LoginMethod.code ? AuthFlow.otpLogin : AuthFlow.login,
    clearError: true,
  );

  /// 切流程时**清掉密码**：注册页填了一半跳去登录，密码框不该还留着上一条流程的输入。
  void setFlow(AuthFlow flow) =>
      state = state.copyWith(flow: flow, password: '', clearError: true);

  /// 返回 true 表示已发出，可以进验证码页。
  ///
  /// purpose 跟着当前流程走——服务端据此拒绝「注册已存在的号」「重置不存在的号」。
  Future<bool> sendCode() async {
    if (!state.canSend) return false;
    state = state.copyWith(busy: true, clearError: true);
    try {
      final repo = ref.read(authRepoProvider);
      final purpose = state.flow.purpose;
      if (state.isEmail) {
        await repo.sendOtp(email: state.email, purpose: purpose);
      } else {
        await repo.sendOtp(
          dialCode: state.dialCode,
          phone: state.phone,
          purpose: purpose,
        );
      }
      state = state.copyWith(busy: false, secondsLeft: 60);
      _tick();
      return true;
    } on ApiException catch (e) {
      state = state.copyWith(busy: false, error: e.message);
      return false;
    }
  }

  /// 密码登录。
  Future<bool> loginWithPassword() async {
    if (!state.canLogin) return false;
    return _run(
      (repo) => state.isEmail
          ? repo.loginWithPassword(email: state.email, password: state.password)
          : repo.loginWithPassword(
              dialCode: state.dialCode,
              phone: state.phone,
              password: state.password,
            ),
    );
  }

  /// 注册：验证码 + 密码一次提交。
  Future<bool> register(String code) async {
    if (!state.canSubmitPassword) return false;
    return _run(
      (repo) => state.isEmail
          ? repo.register(
              email: state.email,
              code: code,
              password: state.password,
            )
          : repo.register(
              dialCode: state.dialCode,
              phone: state.phone,
              code: code,
              password: state.password,
            ),
    );
  }

  /// 重置密码。成功后服务端直接签发令牌，不用再登一次。
  Future<bool> resetPassword(String code) async {
    if (!state.canSubmitPassword) return false;
    return _run(
      (repo) => state.isEmail
          ? repo.resetPassword(
              email: state.email,
              code: code,
              password: state.password,
            )
          : repo.resetPassword(
              dialCode: state.dialCode,
              phone: state.phone,
              code: code,
              password: state.password,
            ),
    );
  }

  /// 三条提交路径的公共外壳：busy 开关、异常转错误文案、成功后写登录态。
  Future<bool> _run(Future<AuthResult> Function(AuthRepository) action) async {
    state = state.copyWith(busy: true, clearError: true);
    try {
      final result = await action(ref.read(authRepoProvider));
      await ref.read(authProvider.notifier).completeLogin(result);
      state = state.copyWith(busy: false);
      return true;
    } on ApiException catch (e) {
      state = state.copyWith(busy: false, error: e.message);
      return false;
    }
  }

  void _tick() {
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (state.secondsLeft <= 1) {
        t.cancel();
        state = state.copyWith(secondsLeft: 0);
      } else {
        state = state.copyWith(secondsLeft: state.secondsLeft - 1);
      }
    });
  }

  /// 返回 true 表示登录成功，由调用方按 `is_new` 决定去引导页还是主 Tab。
  Future<bool> verify(String code) async {
    state = state.copyWith(busy: true, clearError: true);
    try {
      final repo = ref.read(authRepoProvider);
      final result = state.isEmail
          ? await repo.verifyOtp(email: state.email, code: code)
          : await repo.verifyOtp(
              dialCode: state.dialCode,
              phone: state.phone,
              code: code,
            );
      await ref.read(authProvider.notifier).completeLogin(result);
      state = state.copyWith(busy: false);
      return true;
    } on ApiException catch (e) {
      state = state.copyWith(busy: false, error: e.message);
      return false;
    }
  }

  /// 清掉冲突态。页面跳走 A2k 之后调，免得返回登录页时又弹一次。
  void clearConflict() => state = state.copyWith(clearConflict: true);

  /// 第三方登录。先向 SDK 要 idToken，再交服务端验签。
  ///
  /// 三种失败要分开：
  ///   - 用户取消 → **静默返回**，不报错。改主意不是错误。
  ///   - 邮箱已注册(2004) → 置 [OtpState.conflictEmail]，由登录页跳 A2k。
  ///     这是产品选择「不自动合并」的落点，不能当成普通错误弹个 toast 了事。
  ///   - 其余 → 常规错误提示。
  Future<bool> loginWithProvider(String provider) async {
    state = state.copyWith(
      busy: true,
      clearError: true,
      pendingProvider: provider,
    );
    try {
      final client = ref.read(oauthClientProvider);
      final idToken = provider == 'apple'
          ? await client.appleIdToken()
          : await client.googleIdToken();
      // 用户在 A2g 点了「取消」：SDK 的结果回来也当没发生。
      if (state.pendingProvider != provider) return false;
      final result = await ref
          .read(authRepoProvider)
          .loginWithProvider(provider, idToken: idToken);
      if (state.pendingProvider != provider) return false;
      await ref.read(authProvider.notifier).completeLogin(result);
      state = state.copyWith(busy: false, clearPending: true);
      return true;
    } on OAuthCancelled {
      state = state.copyWith(busy: false, clearPending: true);
      return false;
    } on ApiException catch (e) {
      if (e.isOAuthEmailTaken) {
        state = state.copyWith(
          busy: false,
          clearPending: true,
          conflictEmail: e.message,
        );
        return false;
      }
      state = state.copyWith(busy: false, clearPending: true, error: e.message);
      return false;
    } catch (e) {
      // SDK 侧的意外(没装 Play 服务、SHA-1 没登记、缺 Client ID 等)。
      //
      // 给用户看笼统提示,但**必须打日志**——最常见的失败是
      // PlatformException(sign_in_failed, ...ApiException: 10...),
      // 那是 DEVELOPER_ERROR:包名 + 签名 SHA-1 的组合没在 Google Cloud Console
      // 注册,或根本没建 Android OAuth Client ID。
      // 吞掉原始异常会让这个问题完全无法诊断——排查时只能看到
      // 「账号选择器闪一下就没了」,而日志里什么都没有。
      debugPrint('[oauth] $provider 登录失败: $e');
      state = state.copyWith(
        busy: false,
        clearPending: true,
        error: '登录失败，请稍后再试',
      );
      return false;
    }
  }

  /// A2g 整屏等待层上的「取消」：清掉进行中的渠道，之后回来的结果一律忽略。
  void cancelProvider() =>
      state = state.copyWith(busy: false, clearPending: true);
}

/// 「完善资料」页的可选项（语言 / 兴趣 / 年龄范围），后台可配。
///
/// 失败不该把引导页打成错误态——注册刚走完就看见「加载失败」很劝退，
/// 所以调用方一律读 `.value`，拿不到就用内置表兜底。
final profileOptionsProvider = FutureProvider<ProfileOptions>((ref) {
  return ref.watch(authRepoProvider).profileOptions();
});

/// 「完善资料」两步引导的本地草稿。提交成功前不落服务端。
class OnboardingDraft {
  const OnboardingDraft({
    this.step = 0,
    this.avatarPath,
    this.nickname = '',
    this.birthday,
    this.genderIndex = 0,
    this.languages = const [],
    this.interests = const [],
    this.locationEnabled = false,
    this.busy = false,
  });

  final int step;
  final String? avatarPath;
  final String nickname;

  /// 出生日期 `YYYY-MM-DD`；年龄由它推出，服务端同样按它算，不再让人手填年龄。
  final String? birthday;
  final int genderIndex;

  /// 没选日期时为 0，过不了 [canGoNextWith] 的下限。
  int get age {
    final d = parseBirthday(birthday);
    return d == null ? 0 : ageFromBirthday(d, DateTime.now());
  }

  final List<String> languages;
  final List<String> interests;
  final bool locationEnabled;
  final bool busy;

  static const totalSteps = 2;

  /// 兜底值：接口没回来或后台没配时用它。
  /// 真实门槛来自 `app_profile_min_interests` / `app_profile_min_age`。
  static const minInterests = 3;
  static const minAge = 18;
  static const maxAge = 60;

  /// 门槛由服务端配置决定，所以这里收参数而不是读常量。
  bool canGoNextWith(int min, int max) =>
      nickname.trim().isNotEmpty && age >= min && age <= max;

  bool canFinishWith(int min) => interests.length >= min;

  OnboardingDraft copyWith({
    int? step,
    String? avatarPath,
    String? nickname,
    String? birthday,
    int? genderIndex,
    List<String>? languages,
    List<String>? interests,
    bool? locationEnabled,
    bool? busy,
  }) {
    return OnboardingDraft(
      step: step ?? this.step,
      avatarPath: avatarPath ?? this.avatarPath,
      nickname: nickname ?? this.nickname,
      birthday: birthday ?? this.birthday,
      genderIndex: genderIndex ?? this.genderIndex,
      languages: languages ?? this.languages,
      interests: interests ?? this.interests,
      locationEnabled: locationEnabled ?? this.locationEnabled,
      busy: busy ?? this.busy,
    );
  }
}

final onboardingProvider =
    NotifierProvider<OnboardingNotifier, OnboardingDraft>(
      OnboardingNotifier.new,
    );

class OnboardingNotifier extends Notifier<OnboardingDraft> {
  @override
  OnboardingDraft build() {
    final me = ref.read(authProvider).profile;
    return OnboardingDraft(
      nickname: me?.nickname ?? '',
      birthday: me?.birthday,
      languages: me?.languages ?? const [],
      interests: me?.interests ?? const [],
    );
  }

  void setAvatar(String path) => state = state.copyWith(avatarPath: path);
  void setNickname(String v) => state = state.copyWith(nickname: v);
  void setBirthday(String v) => state = state.copyWith(birthday: v);
  void setGender(int i) => state = state.copyWith(genderIndex: i);

  void toggleLanguage(String v) {
    final next = [...state.languages];
    next.contains(v) ? next.remove(v) : next.add(v);
    state = state.copyWith(languages: next);
  }

  void toggleInterest(String v) {
    final next = [...state.interests];
    next.contains(v) ? next.remove(v) : next.add(v);
    state = state.copyWith(interests: next);
  }

  /// 定位被拒绝要能降级，**不可阻断注册流程**。
  void setLocation(bool v) => state = state.copyWith(locationEnabled: v);

  void next() => state = state.copyWith(step: state.step + 1);
  void back() => state = state.copyWith(step: (state.step - 1).clamp(0, 1));

  Future<bool> submit() async {
    state = state.copyWith(busy: true);
    try {
      // 头像是本地文件，先传成 URL 再提交；直接把 /data/user/0/... 当头像存库，换台手机就是破图。
      final avatarUrl = state.avatarPath == null
          ? null
          : await ref.read(momentRepoProvider).uploadImage(state.avatarPath!);
      final profile = await ref
          .read(authRepoProvider)
          .updateProfile(
            nickname: state.nickname.trim(),
            avatar: avatarUrl,
            gender: switch (state.genderIndex) {
              0 => Gender.female,
              1 => Gender.male,
              _ => Gender.secret,
            },
            age: state.age,
            birthday: state.birthday,
            languages: state.languages,
            interests: state.interests,
          );
      ref.read(authProvider.notifier).finishOnboarding(profile);
      state = state.copyWith(busy: false);
      return true;
    } on ApiException {
      state = state.copyWith(busy: false);
      return false;
    }
  }
}
