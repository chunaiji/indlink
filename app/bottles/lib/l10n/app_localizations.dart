import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_en.dart';
import 'app_localizations_zh.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of L
/// returned by `L.of(context)`.
///
/// Applications need to include `L.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'l10n/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: L.localizationsDelegates,
///   supportedLocales: L.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the L.supportedLocales
/// property.
abstract class L {
  L(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static L of(BuildContext context) {
    return Localizations.of<L>(context, L)!;
  }

  static const LocalizationsDelegate<L> delegate = _LDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('en'),
    Locale('zh'),
  ];

  /// No description provided for @appName.
  ///
  /// In zh, this message translates to:
  /// **'DRIFT'**
  String get appName;

  /// No description provided for @appTagline.
  ///
  /// In zh, this message translates to:
  /// **'一条瓶子，可能漂到任何地方'**
  String get appTagline;

  /// No description provided for @tabOcean.
  ///
  /// In zh, this message translates to:
  /// **'海洋'**
  String get tabOcean;

  /// No description provided for @tabDiscover.
  ///
  /// In zh, this message translates to:
  /// **'发现'**
  String get tabDiscover;

  /// No description provided for @tabChats.
  ///
  /// In zh, this message translates to:
  /// **'消息'**
  String get tabChats;

  /// No description provided for @tabMoments.
  ///
  /// In zh, this message translates to:
  /// **'动态'**
  String get tabMoments;

  /// No description provided for @tabMe.
  ///
  /// In zh, this message translates to:
  /// **'我的'**
  String get tabMe;

  /// No description provided for @commonConfirm.
  ///
  /// In zh, this message translates to:
  /// **'确定'**
  String get commonConfirm;

  /// No description provided for @commonCancel.
  ///
  /// In zh, this message translates to:
  /// **'取消'**
  String get commonCancel;

  /// No description provided for @commonClose.
  ///
  /// In zh, this message translates to:
  /// **'关闭'**
  String get commonClose;

  /// No description provided for @commonDone.
  ///
  /// In zh, this message translates to:
  /// **'完成'**
  String get commonDone;

  /// No description provided for @commonRetry.
  ///
  /// In zh, this message translates to:
  /// **'重试'**
  String get commonRetry;

  /// No description provided for @commonReset.
  ///
  /// In zh, this message translates to:
  /// **'重置'**
  String get commonReset;

  /// No description provided for @commonAll.
  ///
  /// In zh, this message translates to:
  /// **'全部'**
  String get commonAll;

  /// No description provided for @commonOr.
  ///
  /// In zh, this message translates to:
  /// **'或'**
  String get commonOr;

  /// No description provided for @commonOnline.
  ///
  /// In zh, this message translates to:
  /// **'在线'**
  String get commonOnline;

  /// No description provided for @commonActiveAgo.
  ///
  /// In zh, this message translates to:
  /// **'{time}活跃'**
  String commonActiveAgo(String time);

  /// No description provided for @commonFemale.
  ///
  /// In zh, this message translates to:
  /// **'女'**
  String get commonFemale;

  /// No description provided for @commonMale.
  ///
  /// In zh, this message translates to:
  /// **'男'**
  String get commonMale;

  /// No description provided for @commonSecret.
  ///
  /// In zh, this message translates to:
  /// **'保密'**
  String get commonSecret;

  /// No description provided for @commonAgeValue.
  ///
  /// In zh, this message translates to:
  /// **'{age} 岁'**
  String commonAgeValue(int age);

  /// No description provided for @commonAnonymous.
  ///
  /// In zh, this message translates to:
  /// **'匿名'**
  String get commonAnonymous;

  /// No description provided for @commonDriftedDays.
  ///
  /// In zh, this message translates to:
  /// **'漂了 {days} 天'**
  String commonDriftedDays(int days);

  /// No description provided for @commonJustNow.
  ///
  /// In zh, this message translates to:
  /// **'刚刚'**
  String get commonJustNow;

  /// No description provided for @commonMinutesAgo.
  ///
  /// In zh, this message translates to:
  /// **'{n} 分钟前'**
  String commonMinutesAgo(int n);

  /// No description provided for @commonHoursAgo.
  ///
  /// In zh, this message translates to:
  /// **'{n} 小时前'**
  String commonHoursAgo(int n);

  /// No description provided for @commonYesterday.
  ///
  /// In zh, this message translates to:
  /// **'昨天'**
  String get commonYesterday;

  /// No description provided for @commonDaysAgo.
  ///
  /// In zh, this message translates to:
  /// **'{n} 天前'**
  String commonDaysAgo(int n);

  /// No description provided for @commonLastWeek.
  ///
  /// In zh, this message translates to:
  /// **'上周'**
  String get commonLastWeek;

  /// No description provided for @authLoginTitle.
  ///
  /// In zh, this message translates to:
  /// **'欢迎回来'**
  String get authLoginTitle;

  /// No description provided for @authRegisterTitle.
  ///
  /// In zh, this message translates to:
  /// **'创建账号'**
  String get authRegisterTitle;

  /// No description provided for @authResetTitle.
  ///
  /// In zh, this message translates to:
  /// **'重设密码'**
  String get authResetTitle;

  /// No description provided for @authPasswordHint.
  ///
  /// In zh, this message translates to:
  /// **'密码'**
  String get authPasswordHint;

  /// No description provided for @authRememberPassword.
  ///
  /// In zh, this message translates to:
  /// **'记住密码'**
  String get authRememberPassword;

  /// No description provided for @authNewPasswordHint.
  ///
  /// In zh, this message translates to:
  /// **'设置新密码'**
  String get authNewPasswordHint;

  /// No description provided for @authPasswordTooShort.
  ///
  /// In zh, this message translates to:
  /// **'密码至少 8 位'**
  String get authPasswordTooShort;

  /// No description provided for @authSignIn.
  ///
  /// In zh, this message translates to:
  /// **'登录'**
  String get authSignIn;

  /// No description provided for @authSignUp.
  ///
  /// In zh, this message translates to:
  /// **'注册'**
  String get authSignUp;

  /// No description provided for @authForgotPassword.
  ///
  /// In zh, this message translates to:
  /// **'忘记密码？'**
  String get authForgotPassword;

  /// No description provided for @authNoAccount.
  ///
  /// In zh, this message translates to:
  /// **'还没有账号？'**
  String get authNoAccount;

  /// No description provided for @authHasAccount.
  ///
  /// In zh, this message translates to:
  /// **'已有账号？'**
  String get authHasAccount;

  /// No description provided for @authUseOtpInstead.
  ///
  /// In zh, this message translates to:
  /// **'用验证码登录'**
  String get authUseOtpInstead;

  /// No description provided for @authUsePasswordInstead.
  ///
  /// In zh, this message translates to:
  /// **'用密码登录'**
  String get authUsePasswordInstead;

  /// No description provided for @authRegisterNext.
  ///
  /// In zh, this message translates to:
  /// **'下一步'**
  String get authRegisterNext;

  /// No description provided for @authRegisterDone.
  ///
  /// In zh, this message translates to:
  /// **'注册并开始'**
  String get authRegisterDone;

  /// No description provided for @authPasswordUpdated.
  ///
  /// In zh, this message translates to:
  /// **'密码已更新'**
  String get authPasswordUpdated;

  /// No description provided for @authResetDone.
  ///
  /// In zh, this message translates to:
  /// **'重设并登录'**
  String get authResetDone;

  /// No description provided for @authSetPasswordHint.
  ///
  /// In zh, this message translates to:
  /// **'之后就用这个密码登录'**
  String get authSetPasswordHint;

  /// No description provided for @authStartWithPhone.
  ///
  /// In zh, this message translates to:
  /// **'用手机号开始'**
  String get authStartWithPhone;

  /// No description provided for @authStartWithEmail.
  ///
  /// In zh, this message translates to:
  /// **'用邮箱开始'**
  String get authStartWithEmail;

  /// No description provided for @authTabPhone.
  ///
  /// In zh, this message translates to:
  /// **'手机号'**
  String get authTabPhone;

  /// No description provided for @authTabEmail.
  ///
  /// In zh, this message translates to:
  /// **'邮箱'**
  String get authTabEmail;

  /// No description provided for @authTabPassword.
  ///
  /// In zh, this message translates to:
  /// **'密码登录'**
  String get authTabPassword;

  /// No description provided for @authTabCode.
  ///
  /// In zh, this message translates to:
  /// **'验证码登录'**
  String get authTabCode;

  /// No description provided for @authPhoneHint.
  ///
  /// In zh, this message translates to:
  /// **'手机号'**
  String get authPhoneHint;

  /// No description provided for @authEmailHint.
  ///
  /// In zh, this message translates to:
  /// **'you@example.com'**
  String get authEmailHint;

  /// No description provided for @authSendCode.
  ///
  /// In zh, this message translates to:
  /// **'发送验证码'**
  String get authSendCode;

  /// No description provided for @authContinueWithGoogle.
  ///
  /// In zh, this message translates to:
  /// **'用 Google 继续'**
  String get authContinueWithGoogle;

  /// No description provided for @authContinueWithApple.
  ///
  /// In zh, this message translates to:
  /// **'用 Apple 继续'**
  String get authContinueWithApple;

  /// No description provided for @authAgreementPrefix.
  ///
  /// In zh, this message translates to:
  /// **'继续即表示同意'**
  String get authAgreementPrefix;

  /// No description provided for @authAgreeTitle.
  ///
  /// In zh, this message translates to:
  /// **'请先阅读并同意协议'**
  String get authAgreeTitle;

  /// No description provided for @authAgreeBody.
  ///
  /// In zh, this message translates to:
  /// **'继续前需要你同意以下条款：'**
  String get authAgreeBody;

  /// No description provided for @authAgreeAndContinue.
  ///
  /// In zh, this message translates to:
  /// **'同意并继续'**
  String get authAgreeAndContinue;

  /// No description provided for @authTerms.
  ///
  /// In zh, this message translates to:
  /// **'《用户协议》'**
  String get authTerms;

  /// No description provided for @authAnd.
  ///
  /// In zh, this message translates to:
  /// **'与'**
  String get authAnd;

  /// No description provided for @authPrivacy.
  ///
  /// In zh, this message translates to:
  /// **'《隐私政策》'**
  String get authPrivacy;

  /// No description provided for @privacyGateTitle.
  ///
  /// In zh, this message translates to:
  /// **'用户协议与隐私政策'**
  String get privacyGateTitle;

  /// No description provided for @privacyGateBody.
  ///
  /// In zh, this message translates to:
  /// **'登录前，请阅读并同意以下条款。我们仅在你授权后收集必要信息（如登录凭证；定位用于推荐附近的人，可拒绝），你可随时在设置中管理或撤回。'**
  String get privacyGateBody;

  /// No description provided for @privacyGateAgree.
  ///
  /// In zh, this message translates to:
  /// **'同意并继续'**
  String get privacyGateAgree;

  /// No description provided for @privacyGateExit.
  ///
  /// In zh, this message translates to:
  /// **'不同意'**
  String get privacyGateExit;

  /// No description provided for @authVerifyPhone.
  ///
  /// In zh, this message translates to:
  /// **'验证手机号'**
  String get authVerifyPhone;

  /// No description provided for @authVerifyEmail.
  ///
  /// In zh, this message translates to:
  /// **'验证邮箱'**
  String get authVerifyEmail;

  /// No description provided for @authEnterCode.
  ///
  /// In zh, this message translates to:
  /// **'输入验证码'**
  String get authEnterCode;

  /// No description provided for @authSentTo.
  ///
  /// In zh, this message translates to:
  /// **'已发送到'**
  String get authSentTo;

  /// No description provided for @authChangeNumber.
  ///
  /// In zh, this message translates to:
  /// **'改号码'**
  String get authChangeNumber;

  /// No description provided for @authChangeEmail.
  ///
  /// In zh, this message translates to:
  /// **'改邮箱'**
  String get authChangeEmail;

  /// No description provided for @authResendIn.
  ///
  /// In zh, this message translates to:
  /// **'{seconds} 秒后可重新发送'**
  String authResendIn(int seconds);

  /// No description provided for @authResend.
  ///
  /// In zh, this message translates to:
  /// **'重新发送验证码'**
  String get authResend;

  /// No description provided for @authOtpWarning.
  ///
  /// In zh, this message translates to:
  /// **'没收到？检查是否被拦截。同一号码 60 秒内只发一次，连续 5 次触发风控'**
  String get authOtpWarning;

  /// No description provided for @authOtpWarningEmail.
  ///
  /// In zh, this message translates to:
  /// **'没收到？先翻一下垃圾邮件。同一邮箱 60 秒内只发一次，连续 5 次触发风控'**
  String get authOtpWarningEmail;

  /// No description provided for @authVerifyAndLogin.
  ///
  /// In zh, this message translates to:
  /// **'验证并登录'**
  String get authVerifyAndLogin;

  /// No description provided for @authInvalidPhone.
  ///
  /// In zh, this message translates to:
  /// **'请输入正确的手机号'**
  String get authInvalidPhone;

  /// No description provided for @authInvalidCode.
  ///
  /// In zh, this message translates to:
  /// **'验证码不正确'**
  String get authInvalidCode;

  /// No description provided for @profileCompleteTitle.
  ///
  /// In zh, this message translates to:
  /// **'完善资料'**
  String get profileCompleteTitle;

  /// No description provided for @profileStepOf.
  ///
  /// In zh, this message translates to:
  /// **'第 {current} 步，共 {total} 步'**
  String profileStepOf(int current, int total);

  /// No description provided for @profileUploadAvatar.
  ///
  /// In zh, this message translates to:
  /// **'上传头像'**
  String get profileUploadAvatar;

  /// No description provided for @profileUploadAvatarHint.
  ///
  /// In zh, this message translates to:
  /// **'有头像的人，收到的回信多得多'**
  String get profileUploadAvatarHint;

  /// No description provided for @profileNicknameHint.
  ///
  /// In zh, this message translates to:
  /// **'昵称'**
  String get profileNicknameHint;

  /// No description provided for @profileGenderLockTitle.
  ///
  /// In zh, this message translates to:
  /// **'性别确认后不能再改'**
  String get profileGenderLockTitle;

  /// No description provided for @profileGenderLockBody.
  ///
  /// In zh, this message translates to:
  /// **'你选择的是「{gender}」。为了保证匹配公平，提交后性别不可修改，确定吗？'**
  String profileGenderLockBody(String gender);

  /// No description provided for @profileGenderRethink.
  ///
  /// In zh, this message translates to:
  /// **'再想想'**
  String get profileGenderRethink;

  /// No description provided for @profileEditTitle.
  ///
  /// In zh, this message translates to:
  /// **'编辑资料'**
  String get profileEditTitle;

  /// No description provided for @profileEditAvatarHint.
  ///
  /// In zh, this message translates to:
  /// **'点击更换头像'**
  String get profileEditAvatarHint;

  /// No description provided for @profileEditNickname.
  ///
  /// In zh, this message translates to:
  /// **'昵称'**
  String get profileEditNickname;

  /// No description provided for @profileEditNicknameRequired.
  ///
  /// In zh, this message translates to:
  /// **'昵称不能为空'**
  String get profileEditNicknameRequired;

  /// No description provided for @profileEditBirthday.
  ///
  /// In zh, this message translates to:
  /// **'出生日期'**
  String get profileEditBirthday;

  /// No description provided for @profileEditBirthdayHint.
  ///
  /// In zh, this message translates to:
  /// **'选择出生日期'**
  String get profileEditBirthdayHint;

  /// No description provided for @profileEditBirthdayNote.
  ///
  /// In zh, this message translates to:
  /// **'年龄按出生日期计算；日期本身只有你自己能看到。'**
  String get profileEditBirthdayNote;

  /// No description provided for @profileEditSave.
  ///
  /// In zh, this message translates to:
  /// **'保存'**
  String get profileEditSave;

  /// No description provided for @profileEditSaved.
  ///
  /// In zh, this message translates to:
  /// **'资料已更新'**
  String get profileEditSaved;

  /// No description provided for @profileGender.
  ///
  /// In zh, this message translates to:
  /// **'性别'**
  String get profileGender;

  /// No description provided for @profileLanguages.
  ///
  /// In zh, this message translates to:
  /// **'你说什么语言'**
  String get profileLanguages;

  /// No description provided for @profileLanguagePick.
  ///
  /// In zh, this message translates to:
  /// **'选择你会的语言'**
  String get profileLanguagePick;

  /// No description provided for @profileInterests.
  ///
  /// In zh, this message translates to:
  /// **'兴趣 · 至少选 {n} 个'**
  String profileInterests(int n);

  /// No description provided for @profileEnableLocation.
  ///
  /// In zh, this message translates to:
  /// **'开启定位'**
  String get profileEnableLocation;

  /// No description provided for @profileEnableLocationHint.
  ///
  /// In zh, this message translates to:
  /// **'用来推荐附近的人，随时可关'**
  String get profileEnableLocationHint;

  /// No description provided for @profileStartDrifting.
  ///
  /// In zh, this message translates to:
  /// **'开始漂流'**
  String get profileStartDrifting;

  /// No description provided for @profileNeedNickname.
  ///
  /// In zh, this message translates to:
  /// **'先给自己起个名字'**
  String get profileNeedNickname;

  /// No description provided for @profileNeedInterests.
  ///
  /// In zh, this message translates to:
  /// **'再选 {n} 个兴趣就可以出发了'**
  String profileNeedInterests(int n);

  /// No description provided for @profileAgeRange.
  ///
  /// In zh, this message translates to:
  /// **'年龄 {min}-{max}'**
  String profileAgeRange(int min, int max);

  /// No description provided for @profileAgeInvalid.
  ///
  /// In zh, this message translates to:
  /// **'需 {min}-{max} 岁'**
  String profileAgeInvalid(int min, int max);

  /// No description provided for @oceanTitle.
  ///
  /// In zh, this message translates to:
  /// **'今晚的海'**
  String get oceanTitle;

  /// No description provided for @oceanSubtitle.
  ///
  /// In zh, this message translates to:
  /// **'{count} 条回应正在漂回来 · 夜场 {time} 开'**
  String oceanSubtitle(int count, String time);

  /// No description provided for @oceanNightTitle.
  ///
  /// In zh, this message translates to:
  /// **'夜场开始了'**
  String get oceanNightTitle;

  /// No description provided for @oceanNightSubtitle.
  ///
  /// In zh, this message translates to:
  /// **'只有深夜瓶在漂 · 还剩 {hours} 小时 {minutes} 分'**
  String oceanNightSubtitle(int hours, int minutes);

  /// No description provided for @oceanTagNight.
  ///
  /// In zh, this message translates to:
  /// **'🌙 夜场'**
  String get oceanTagNight;

  /// No description provided for @oceanTagNightShort.
  ///
  /// In zh, this message translates to:
  /// **'🌙 深夜'**
  String get oceanTagNightShort;

  /// No description provided for @oceanTagHollow.
  ///
  /// In zh, this message translates to:
  /// **'树洞'**
  String get oceanTagHollow;

  /// No description provided for @oceanPullToRefresh.
  ///
  /// In zh, this message translates to:
  /// **'下拉涨潮，换一批'**
  String get oceanPullToRefresh;

  /// No description provided for @oceanSeaReport.
  ///
  /// In zh, this message translates to:
  /// **'今日海况'**
  String get oceanSeaReport;

  /// No description provided for @oceanSkinDay.
  ///
  /// In zh, this message translates to:
  /// **'已切换为白天海面'**
  String get oceanSkinDay;

  /// No description provided for @oceanSkinNight.
  ///
  /// In zh, this message translates to:
  /// **'已切换为夜晚海面'**
  String get oceanSkinNight;

  /// No description provided for @oceanSkinAuto.
  ///
  /// In zh, this message translates to:
  /// **'海面恢复跟随时间'**
  String get oceanSkinAuto;

  /// No description provided for @oceanOnlineTonight.
  ///
  /// In zh, this message translates to:
  /// **'今晚 {n} 人在线'**
  String oceanOnlineTonight(int n);

  /// No description provided for @oceanThrowOne.
  ///
  /// In zh, this message translates to:
  /// **'扔一个'**
  String get oceanThrowOne;

  /// No description provided for @oceanThrowRemain.
  ///
  /// In zh, this message translates to:
  /// **'今日还剩 {n} 次'**
  String oceanThrowRemain(int n);

  /// No description provided for @oceanThrowUnlimited.
  ///
  /// In zh, this message translates to:
  /// **'深夜瓶不限次'**
  String get oceanThrowUnlimited;

  /// No description provided for @oceanScoopOne.
  ///
  /// In zh, this message translates to:
  /// **'捞一个'**
  String get oceanScoopOne;

  /// No description provided for @oceanScoopRemain.
  ///
  /// In zh, this message translates to:
  /// **'今日还剩 {n} 次'**
  String oceanScoopRemain(int n);

  /// No description provided for @oceanTracePeek.
  ///
  /// In zh, this message translates to:
  /// **'漂了 {days} 天 · {cities} 座城市'**
  String oceanTracePeek(int days, int cities);

  /// No description provided for @oceanSomethingBit.
  ///
  /// In zh, this message translates to:
  /// **'有东西上钩了'**
  String get oceanSomethingBit;

  /// No description provided for @oceanReleaseToOpen.
  ///
  /// In zh, this message translates to:
  /// **'松手打开它'**
  String get oceanReleaseToOpen;

  /// No description provided for @oceanOpeningNow.
  ///
  /// In zh, this message translates to:
  /// **'正在打开…'**
  String get oceanOpeningNow;

  /// No description provided for @oceanScoopFailed.
  ///
  /// In zh, this message translates to:
  /// **'海面起风了，瓶子沉回去了 · 没有消耗次数'**
  String get oceanScoopFailed;

  /// No description provided for @oceanPutBack.
  ///
  /// In zh, this message translates to:
  /// **'放回海里'**
  String get oceanPutBack;

  /// No description provided for @oceanPutBackHint.
  ///
  /// In zh, this message translates to:
  /// **'对方不会收到打扰'**
  String get oceanPutBackHint;

  /// No description provided for @oceanWriteReply.
  ///
  /// In zh, this message translates to:
  /// **'写回信'**
  String get oceanWriteReply;

  /// No description provided for @oceanWriteReplyHint.
  ///
  /// In zh, this message translates to:
  /// **'对方会收到通知'**
  String get oceanWriteReplyHint;

  /// No description provided for @oceanDriftedRecent.
  ///
  /// In zh, this message translates to:
  /// **'刚漂到你这里 · {city}'**
  String oceanDriftedRecent(String city);

  /// No description provided for @oceanDriftedFromTo.
  ///
  /// In zh, this message translates to:
  /// **'漂了 {days} 天 · {city} → 你这里'**
  String oceanDriftedFromTo(int days, String city);

  /// No description provided for @oceanCastDone.
  ///
  /// In zh, this message translates to:
  /// **'瓶子已经漂走了'**
  String get oceanCastDone;

  /// No description provided for @bottleWriteTitle.
  ///
  /// In zh, this message translates to:
  /// **'写一个瓶子'**
  String get bottleWriteTitle;

  /// No description provided for @bottleThrowRemainToday.
  ///
  /// In zh, this message translates to:
  /// **'今天还能扔 {n} 个'**
  String bottleThrowRemainToday(int n);

  /// No description provided for @bottleContentHint.
  ///
  /// In zh, this message translates to:
  /// **'写点什么，扔进海里…'**
  String get bottleContentHint;

  /// No description provided for @bottleCounter.
  ///
  /// In zh, this message translates to:
  /// **'{used} / {max}'**
  String bottleCounter(int used, int max);

  /// No description provided for @bottleTags.
  ///
  /// In zh, this message translates to:
  /// **'标签'**
  String get bottleTags;

  /// No description provided for @bottleRange.
  ///
  /// In zh, this message translates to:
  /// **'投放范围'**
  String get bottleRange;

  /// No description provided for @bottleRangeNationwide.
  ///
  /// In zh, this message translates to:
  /// **'全国'**
  String get bottleRangeNationwide;

  /// No description provided for @bottleRangeCity.
  ///
  /// In zh, this message translates to:
  /// **'同城'**
  String get bottleRangeCity;

  /// No description provided for @bottleCastToSea.
  ///
  /// In zh, this message translates to:
  /// **'抛向大海'**
  String get bottleCastToSea;

  /// No description provided for @bottleNeedContent.
  ///
  /// In zh, this message translates to:
  /// **'先写点什么再扔'**
  String get bottleNeedContent;

  /// No description provided for @bottleScoopedTitle.
  ///
  /// In zh, this message translates to:
  /// **'捞到的瓶子'**
  String get bottleScoopedTitle;

  /// No description provided for @bottleUnlockPill.
  ///
  /// In zh, this message translates to:
  /// **'解锁'**
  String get bottleUnlockPill;

  /// No description provided for @bottleUnlockOneTitle.
  ///
  /// In zh, this message translates to:
  /// **'看看这条回信？'**
  String get bottleUnlockOneTitle;

  /// No description provided for @bottleUnlockOneBody.
  ///
  /// In zh, this message translates to:
  /// **'花 {n} 金币解锁这条回信，解锁后永久可见。'**
  String bottleUnlockOneBody(int n);

  /// No description provided for @bottleUnlockOneConfirm.
  ///
  /// In zh, this message translates to:
  /// **'花 {n} 金币解锁'**
  String bottleUnlockOneConfirm(int n);

  /// No description provided for @bottleUnlockedOne.
  ///
  /// In zh, this message translates to:
  /// **'已解锁'**
  String get bottleUnlockedOne;

  /// No description provided for @bottleRepliesHeader.
  ///
  /// In zh, this message translates to:
  /// **'{count} 条回信 · 最近一条 {time}'**
  String bottleRepliesHeader(int count, String time);

  /// No description provided for @bottleReplyHint.
  ///
  /// In zh, this message translates to:
  /// **'写下你的回信…'**
  String get bottleReplyHint;

  /// No description provided for @bottleReplySent.
  ///
  /// In zh, this message translates to:
  /// **'回信已经送出去了'**
  String get bottleReplySent;

  /// No description provided for @bottleCollected.
  ///
  /// In zh, this message translates to:
  /// **'已收藏，去「我的瓶子」能找到它'**
  String get bottleCollected;

  /// No description provided for @bottleUncollected.
  ///
  /// In zh, this message translates to:
  /// **'已取消收藏'**
  String get bottleUncollected;

  /// No description provided for @bottleMineTitle.
  ///
  /// In zh, this message translates to:
  /// **'我的瓶子'**
  String get bottleMineTitle;

  /// No description provided for @bottleTabThrown.
  ///
  /// In zh, this message translates to:
  /// **'我扔的'**
  String get bottleTabThrown;

  /// No description provided for @bottleTabScooped.
  ///
  /// In zh, this message translates to:
  /// **'我捞的'**
  String get bottleTabScooped;

  /// No description provided for @bottleTabCollected.
  ///
  /// In zh, this message translates to:
  /// **'已收藏'**
  String get bottleTabCollected;

  /// No description provided for @bottleTraceTitle.
  ///
  /// In zh, this message translates to:
  /// **'这个瓶子漂过哪些地方'**
  String get bottleTraceTitle;

  /// No description provided for @bottleTraceHere.
  ///
  /// In zh, this message translates to:
  /// **'你这里 · 被你捞起'**
  String get bottleTraceHere;

  /// No description provided for @bottleTraceSummary.
  ///
  /// In zh, this message translates to:
  /// **'一共被 {people} 个人看到，跨越 {cities} 座城市'**
  String bottleTraceSummary(int people, int cities);

  /// No description provided for @bottleExpired.
  ///
  /// In zh, this message translates to:
  /// **'已过期'**
  String get bottleExpired;

  /// No description provided for @bottleStatViews.
  ///
  /// In zh, this message translates to:
  /// **'{n}'**
  String bottleStatViews(int n);

  /// No description provided for @bottleMapTitle.
  ///
  /// In zh, this message translates to:
  /// **'这个瓶子漂过的路'**
  String get bottleMapTitle;

  /// No description provided for @bottleMapPosterTitle.
  ///
  /// In zh, this message translates to:
  /// **'一封信的旅程'**
  String get bottleMapPosterTitle;

  /// No description provided for @bottleMapDeparted.
  ///
  /// In zh, this message translates to:
  /// **'{date} 从 {city} 出发'**
  String bottleMapDeparted(String date, String city);

  /// No description provided for @bottleMapStatPeople.
  ///
  /// In zh, this message translates to:
  /// **'个人看到'**
  String get bottleMapStatPeople;

  /// No description provided for @bottleMapStatCities.
  ///
  /// In zh, this message translates to:
  /// **'座城市'**
  String get bottleMapStatCities;

  /// No description provided for @bottleMapStatDays.
  ///
  /// In zh, this message translates to:
  /// **'天漂流'**
  String get bottleMapStatDays;

  /// No description provided for @bottleMapStatKm.
  ///
  /// In zh, this message translates to:
  /// **'公里'**
  String get bottleMapStatKm;

  /// No description provided for @bottleMapPrivacy.
  ///
  /// In zh, this message translates to:
  /// **'海报只包含你写的内容和城市，不会出现任何回信者的信息'**
  String get bottleMapPrivacy;

  /// No description provided for @bottleMapSave.
  ///
  /// In zh, this message translates to:
  /// **'保存图片'**
  String get bottleMapSave;

  /// No description provided for @bottleMapShare.
  ///
  /// In zh, this message translates to:
  /// **'分享给朋友'**
  String get bottleMapShare;

  /// No description provided for @bottleMapSaved.
  ///
  /// In zh, this message translates to:
  /// **'海报已经存到相册'**
  String get bottleMapSaved;

  /// No description provided for @bottleTraceThrown.
  ///
  /// In zh, this message translates to:
  /// **'{city} · 你扔出'**
  String bottleTraceThrown(String city);

  /// No description provided for @bottleTraceSeen.
  ///
  /// In zh, this message translates to:
  /// **'{city} · 被 {n} 人看到'**
  String bottleTraceSeen(String city, int n);

  /// No description provided for @bottleTraceReplied.
  ///
  /// In zh, this message translates to:
  /// **'{city} · 收到 {n} 条回信'**
  String bottleTraceReplied(String city, int n);

  /// No description provided for @discoverTitle.
  ///
  /// In zh, this message translates to:
  /// **'今天在线的人'**
  String get discoverTitle;

  /// No description provided for @discoverTabRecommend.
  ///
  /// In zh, this message translates to:
  /// **'推荐'**
  String get discoverTabRecommend;

  /// No description provided for @discoverTabNearby.
  ///
  /// In zh, this message translates to:
  /// **'附近'**
  String get discoverTabNearby;

  /// No description provided for @discoverTabNew.
  ///
  /// In zh, this message translates to:
  /// **'新人'**
  String get discoverTabNew;

  /// No description provided for @discoverSayHi.
  ///
  /// In zh, this message translates to:
  /// **'打招呼'**
  String get discoverSayHi;

  /// No description provided for @discoverViewProfile.
  ///
  /// In zh, this message translates to:
  /// **'主页'**
  String get discoverViewProfile;

  /// No description provided for @discoverLike.
  ///
  /// In zh, this message translates to:
  /// **'喜欢'**
  String get discoverLike;

  /// No description provided for @discoverPass.
  ///
  /// In zh, this message translates to:
  /// **'跳过'**
  String get discoverPass;

  /// No description provided for @discoverRewind.
  ///
  /// In zh, this message translates to:
  /// **'撤回'**
  String get discoverRewind;

  /// No description provided for @discoverRewindTitle.
  ///
  /// In zh, this message translates to:
  /// **'把刚划走的人请回来？'**
  String get discoverRewindTitle;

  /// No description provided for @discoverRewindBody.
  ///
  /// In zh, this message translates to:
  /// **'花 {n} 金币撤回上一次左滑，TA 会重新出现在卡片顶部。'**
  String discoverRewindBody(int n);

  /// No description provided for @discoverRewindConfirm.
  ///
  /// In zh, this message translates to:
  /// **'花 {n} 金币撤回'**
  String discoverRewindConfirm(int n);

  /// No description provided for @discoverRewindDone.
  ///
  /// In zh, this message translates to:
  /// **'{name} 回来了'**
  String discoverRewindDone(String name);

  /// No description provided for @discoverPassChargeTitle.
  ///
  /// In zh, this message translates to:
  /// **'跳过 TA？'**
  String get discoverPassChargeTitle;

  /// No description provided for @discoverPassChargeBody.
  ///
  /// In zh, this message translates to:
  /// **'左滑跳过将花 {n} 金币，TA 不会再出现在你的卡片里。'**
  String discoverPassChargeBody(int n);

  /// No description provided for @discoverPassChargeConfirm.
  ///
  /// In zh, this message translates to:
  /// **'花 {n} 金币跳过'**
  String discoverPassChargeConfirm(int n);

  /// No description provided for @discoverFiltersTitle.
  ///
  /// In zh, this message translates to:
  /// **'筛选'**
  String get discoverFiltersTitle;

  /// No description provided for @discoverLanguage.
  ///
  /// In zh, this message translates to:
  /// **'语言'**
  String get discoverLanguage;

  /// No description provided for @discoverInterest.
  ///
  /// In zh, this message translates to:
  /// **'兴趣'**
  String get discoverInterest;

  /// No description provided for @discoverQuizTitle.
  ///
  /// In zh, this message translates to:
  /// **'答题匹配'**
  String get discoverQuizTitle;

  /// No description provided for @discoverQuizHint.
  ///
  /// In zh, this message translates to:
  /// **'选相同答案的人，优先推给你'**
  String get discoverQuizHint;

  /// No description provided for @discoverDistance.
  ///
  /// In zh, this message translates to:
  /// **'距离'**
  String get discoverDistance;

  /// No description provided for @discoverDistanceUnlimited.
  ///
  /// In zh, this message translates to:
  /// **'不限'**
  String get discoverDistanceUnlimited;

  /// No description provided for @discoverDistanceKm.
  ///
  /// In zh, this message translates to:
  /// **'{n} km'**
  String discoverDistanceKm(int n);

  /// No description provided for @discoverAgeRange.
  ///
  /// In zh, this message translates to:
  /// **'年龄'**
  String get discoverAgeRange;

  /// No description provided for @discoverAgeRangeValue.
  ///
  /// In zh, this message translates to:
  /// **'{min} – {max} 岁'**
  String discoverAgeRangeValue(int min, int max);

  /// No description provided for @discoverApplyFilter.
  ///
  /// In zh, this message translates to:
  /// **'应用筛选 · {n} 人符合'**
  String discoverApplyFilter(int n);

  /// No description provided for @discoverNoMoreCards.
  ///
  /// In zh, this message translates to:
  /// **'今天的人看完了，明天再来'**
  String get discoverNoMoreCards;

  /// No description provided for @discoverStatBottles.
  ///
  /// In zh, this message translates to:
  /// **'瓶子'**
  String get discoverStatBottles;

  /// No description provided for @discoverStatMoments.
  ///
  /// In zh, this message translates to:
  /// **'动态'**
  String get discoverStatMoments;

  /// No description provided for @discoverStatRelation.
  ///
  /// In zh, this message translates to:
  /// **'关系'**
  String get discoverStatRelation;

  /// No description provided for @discoverCharm.
  ///
  /// In zh, this message translates to:
  /// **'魅力'**
  String get discoverCharm;

  /// No description provided for @profileCommonPoint.
  ///
  /// In zh, this message translates to:
  /// **'共同点 · 你们都选了「{value}」'**
  String profileCommonPoint(String value);

  /// No description provided for @profileTheirMoments.
  ///
  /// In zh, this message translates to:
  /// **'TA 的动态'**
  String get profileTheirMoments;

  /// No description provided for @profileMomentCount.
  ///
  /// In zh, this message translates to:
  /// **'共 {n} 条'**
  String profileMomentCount(int n);

  /// No description provided for @chatTitle.
  ///
  /// In zh, this message translates to:
  /// **'消息'**
  String get chatTitle;

  /// No description provided for @chatOneMessage.
  ///
  /// In zh, this message translates to:
  /// **'发一条消息'**
  String get chatOneMessage;

  /// No description provided for @chatPricingHint.
  ///
  /// In zh, this message translates to:
  /// **'前 {free} 条免费，之后每条 {price} 金币'**
  String chatPricingHint(int free, int price);

  /// No description provided for @chatPricingHintNoFree.
  ///
  /// In zh, this message translates to:
  /// **'每条消息 {price} 金币'**
  String chatPricingHintNoFree(int price);

  /// No description provided for @chatTabUnread.
  ///
  /// In zh, this message translates to:
  /// **'未读 {n}'**
  String chatTabUnread(int n);

  /// No description provided for @chatTabBottleFriends.
  ///
  /// In zh, this message translates to:
  /// **'瓶友'**
  String get chatTabBottleFriends;

  /// No description provided for @chatAnonymousFriend.
  ///
  /// In zh, this message translates to:
  /// **'匿名瓶友'**
  String get chatAnonymousFriend;

  /// No description provided for @chatStartedFromBottle.
  ///
  /// In zh, this message translates to:
  /// **'你们从一个瓶子开始聊天'**
  String get chatStartedFromBottle;

  /// No description provided for @chatRead.
  ///
  /// In zh, this message translates to:
  /// **'已读'**
  String get chatRead;

  /// No description provided for @chatImageMessage.
  ///
  /// In zh, this message translates to:
  /// **'［图片］'**
  String get chatImageMessage;

  /// No description provided for @chatGiftSentBy.
  ///
  /// In zh, this message translates to:
  /// **'送出了「{gift}」'**
  String chatGiftSentBy(String gift);

  /// No description provided for @chatGiftReceived.
  ///
  /// In zh, this message translates to:
  /// **'对方送来「{gift}」'**
  String chatGiftReceived(String gift);

  /// No description provided for @chatCharmPlus.
  ///
  /// In zh, this message translates to:
  /// **'魅力 +{n}'**
  String chatCharmPlus(int n);

  /// No description provided for @chatInputHint.
  ///
  /// In zh, this message translates to:
  /// **'说点什么…'**
  String get chatInputHint;

  /// No description provided for @chatDeletedUser.
  ///
  /// In zh, this message translates to:
  /// **'已注销用户'**
  String get chatDeletedUser;

  /// No description provided for @chatDeletedUserHint.
  ///
  /// In zh, this message translates to:
  /// **'对方已注销，会话只读'**
  String get chatDeletedUserHint;

  /// No description provided for @chatReconnecting.
  ///
  /// In zh, this message translates to:
  /// **'正在重新连接…'**
  String get chatReconnecting;

  /// No description provided for @safetyReportUser.
  ///
  /// In zh, this message translates to:
  /// **'举报 {name}'**
  String safetyReportUser(String name);

  /// No description provided for @safetyReasonPorn.
  ///
  /// In zh, this message translates to:
  /// **'色情或低俗内容'**
  String get safetyReasonPorn;

  /// No description provided for @safetyReasonSpam.
  ///
  /// In zh, this message translates to:
  /// **'引流 / 广告 / 诈骗'**
  String get safetyReasonSpam;

  /// No description provided for @safetyReasonHarass.
  ///
  /// In zh, this message translates to:
  /// **'骚扰或人身攻击'**
  String get safetyReasonHarass;

  /// No description provided for @safetyReasonMinor.
  ///
  /// In zh, this message translates to:
  /// **'未成年人相关'**
  String get safetyReasonMinor;

  /// No description provided for @safetyBlockOnly.
  ///
  /// In zh, this message translates to:
  /// **'仅拉黑'**
  String get safetyBlockOnly;

  /// No description provided for @safetyReportAndBlock.
  ///
  /// In zh, this message translates to:
  /// **'举报并拉黑'**
  String get safetyReportAndBlock;

  /// No description provided for @safetyReported.
  ///
  /// In zh, this message translates to:
  /// **'已收到举报，我们会尽快处理'**
  String get safetyReported;

  /// No description provided for @safetyBlocked.
  ///
  /// In zh, this message translates to:
  /// **'已拉黑，对方不会再出现'**
  String get safetyBlocked;

  /// No description provided for @safetyBlocklist.
  ///
  /// In zh, this message translates to:
  /// **'黑名单'**
  String get safetyBlocklist;

  /// No description provided for @safetyUnblock.
  ///
  /// In zh, this message translates to:
  /// **'解除'**
  String get safetyUnblock;

  /// No description provided for @safetyBlocklistEmpty.
  ///
  /// In zh, this message translates to:
  /// **'还没有拉黑过任何人'**
  String get safetyBlocklistEmpty;

  /// No description provided for @momentTitle.
  ///
  /// In zh, this message translates to:
  /// **'动态'**
  String get momentTitle;

  /// No description provided for @momentGift.
  ///
  /// In zh, this message translates to:
  /// **'送礼'**
  String get momentGift;

  /// No description provided for @momentTabRecommend.
  ///
  /// In zh, this message translates to:
  /// **'推荐'**
  String get momentTabRecommend;

  /// No description provided for @momentTabFollowing.
  ///
  /// In zh, this message translates to:
  /// **'关注'**
  String get momentTabFollowing;

  /// No description provided for @momentTabCity.
  ///
  /// In zh, this message translates to:
  /// **'同城'**
  String get momentTabCity;

  /// No description provided for @momentDetailTitle.
  ///
  /// In zh, this message translates to:
  /// **'动态'**
  String get momentDetailTitle;

  /// No description provided for @momentFollow.
  ///
  /// In zh, this message translates to:
  /// **'关注'**
  String get momentFollow;

  /// No description provided for @momentFollowing.
  ///
  /// In zh, this message translates to:
  /// **'已关注'**
  String get momentFollowing;

  /// No description provided for @momentCommentsHeader.
  ///
  /// In zh, this message translates to:
  /// **'{n} 条评论'**
  String momentCommentsHeader(int n);

  /// No description provided for @momentCommentHint.
  ///
  /// In zh, this message translates to:
  /// **'写评论…'**
  String get momentCommentHint;

  /// No description provided for @momentGiftComment.
  ///
  /// In zh, this message translates to:
  /// **'送出「{gift}」'**
  String momentGiftComment(String gift);

  /// No description provided for @momentPostTitle.
  ///
  /// In zh, this message translates to:
  /// **'发一条动态'**
  String get momentPostTitle;

  /// No description provided for @momentPostHint.
  ///
  /// In zh, this message translates to:
  /// **'此刻在想什么…'**
  String get momentPostHint;

  /// No description provided for @momentPost.
  ///
  /// In zh, this message translates to:
  /// **'发布'**
  String get momentPost;

  /// No description provided for @momentWhoCanSee.
  ///
  /// In zh, this message translates to:
  /// **'谁可以看'**
  String get momentWhoCanSee;

  /// No description provided for @momentVisPublic.
  ///
  /// In zh, this message translates to:
  /// **'公开'**
  String get momentVisPublic;

  /// No description provided for @momentVisSelf.
  ///
  /// In zh, this message translates to:
  /// **'仅自己'**
  String get momentVisSelf;

  /// No description provided for @momentPosted.
  ///
  /// In zh, this message translates to:
  /// **'动态已发布'**
  String get momentPosted;

  /// No description provided for @mediaChooseTitle.
  ///
  /// In zh, this message translates to:
  /// **'选择图片'**
  String get mediaChooseTitle;

  /// No description provided for @mediaSelectedOf.
  ///
  /// In zh, this message translates to:
  /// **'已选 {n} / {max}'**
  String mediaSelectedOf(int n, int max);

  /// No description provided for @mediaCompressing.
  ///
  /// In zh, this message translates to:
  /// **'压缩中 {percent}%'**
  String mediaCompressing(int percent);

  /// No description provided for @mediaCompressingSimple.
  ///
  /// In zh, this message translates to:
  /// **'压缩中…'**
  String get mediaCompressingSimple;

  /// No description provided for @mediaUploadingPercent.
  ///
  /// In zh, this message translates to:
  /// **'上传中 {percent}%'**
  String mediaUploadingPercent(int percent);

  /// No description provided for @mediaOriginalSize.
  ///
  /// In zh, this message translates to:
  /// **'原图 {size}'**
  String mediaOriginalSize(String size);

  /// No description provided for @mediaCompressedSize.
  ///
  /// In zh, this message translates to:
  /// **'压缩后 {size}'**
  String mediaCompressedSize(String size);

  /// No description provided for @mediaUploadingOf.
  ///
  /// In zh, this message translates to:
  /// **'上传中 {done} / {total}'**
  String mediaUploadingOf(int done, int total);

  /// No description provided for @mediaRetryFailed.
  ///
  /// In zh, this message translates to:
  /// **'{n} 张失败，重试'**
  String mediaRetryFailed(int n);

  /// No description provided for @notifyTitle.
  ///
  /// In zh, this message translates to:
  /// **'通知'**
  String get notifyTitle;

  /// No description provided for @notifyMarkAllRead.
  ///
  /// In zh, this message translates to:
  /// **'全部已读'**
  String get notifyMarkAllRead;

  /// No description provided for @notifyEnablePush.
  ///
  /// In zh, this message translates to:
  /// **'开启推送'**
  String get notifyEnablePush;

  /// No description provided for @notifyEnablePushHint.
  ///
  /// In zh, this message translates to:
  /// **'有人回你的瓶子时第一时间知道'**
  String get notifyEnablePushHint;

  /// No description provided for @notifyEnable.
  ///
  /// In zh, this message translates to:
  /// **'开启'**
  String get notifyEnable;

  /// No description provided for @notifyNewReply.
  ///
  /// In zh, this message translates to:
  /// **'你的瓶子收到新回信'**
  String get notifyNewReply;

  /// No description provided for @notifyGiftReceived.
  ///
  /// In zh, this message translates to:
  /// **'{name} 送了你一份礼物'**
  String notifyGiftReceived(String name);

  /// No description provided for @notifyMomentLiked.
  ///
  /// In zh, this message translates to:
  /// **'{name} 赞了你的动态'**
  String notifyMomentLiked(String name);

  /// No description provided for @notifyEmpty.
  ///
  /// In zh, this message translates to:
  /// **'还没有新通知'**
  String get notifyEmpty;

  /// No description provided for @meIdLine.
  ///
  /// In zh, this message translates to:
  /// **'ID {id} · {gender} {age}'**
  String meIdLine(String id, String gender, int age);

  /// No description provided for @meCoinBalance.
  ///
  /// In zh, this message translates to:
  /// **'金币余额'**
  String get meCoinBalance;

  /// No description provided for @meRecharge.
  ///
  /// In zh, this message translates to:
  /// **'充值'**
  String get meRecharge;

  /// No description provided for @meCheckin.
  ///
  /// In zh, this message translates to:
  /// **'签到'**
  String get meCheckin;

  /// No description provided for @meItems.
  ///
  /// In zh, this message translates to:
  /// **'道具'**
  String get meItems;

  /// No description provided for @itemsHint.
  ///
  /// In zh, this message translates to:
  /// **'道具用金币买，买来放进背包，送礼时消耗'**
  String get itemsHint;

  /// No description provided for @itemsRecords.
  ///
  /// In zh, this message translates to:
  /// **'道具记录'**
  String get itemsRecords;

  /// No description provided for @itemsRecordsEmpty.
  ///
  /// In zh, this message translates to:
  /// **'还没有记录'**
  String get itemsRecordsEmpty;

  /// No description provided for @itemsRecordBuy.
  ///
  /// In zh, this message translates to:
  /// **'买入背包'**
  String get itemsRecordBuy;

  /// No description provided for @itemsRecordSent.
  ///
  /// In zh, this message translates to:
  /// **'送给 {name}'**
  String itemsRecordSent(String name);

  /// No description provided for @itemsRecordReceived.
  ///
  /// In zh, this message translates to:
  /// **'来自 {name}'**
  String itemsRecordReceived(String name);

  /// No description provided for @itemsEmptyTitle.
  ///
  /// In zh, this message translates to:
  /// **'还没有可买的道具'**
  String get itemsEmptyTitle;

  /// No description provided for @itemsBuy.
  ///
  /// In zh, this message translates to:
  /// **'买 1 个'**
  String get itemsBuy;

  /// No description provided for @itemsBuyBody.
  ///
  /// In zh, this message translates to:
  /// **'花 {n} 金币买 1 个放进背包，送礼时从背包里取。'**
  String itemsBuyBody(int n);

  /// No description provided for @itemsBought.
  ///
  /// In zh, this message translates to:
  /// **'已放进背包：{name}'**
  String itemsBought(String name);

  /// No description provided for @itemsOwned.
  ///
  /// In zh, this message translates to:
  /// **'持有 {n}'**
  String itemsOwned(int n);

  /// No description provided for @itemsNotOwned.
  ///
  /// In zh, this message translates to:
  /// **'还没有'**
  String get itemsNotOwned;

  /// No description provided for @meWatchVideo.
  ///
  /// In zh, this message translates to:
  /// **'看视频'**
  String get meWatchVideo;

  /// No description provided for @meRelations.
  ///
  /// In zh, this message translates to:
  /// **'关系'**
  String get meRelations;

  /// No description provided for @meWalletTxns.
  ///
  /// In zh, this message translates to:
  /// **'钱包流水'**
  String get meWalletTxns;

  /// No description provided for @meMyBottles.
  ///
  /// In zh, this message translates to:
  /// **'我的瓶子'**
  String get meMyBottles;

  /// No description provided for @meStatBottles.
  ///
  /// In zh, this message translates to:
  /// **'瓶子'**
  String get meStatBottles;

  /// No description provided for @meStatFollowing.
  ///
  /// In zh, this message translates to:
  /// **'关注'**
  String get meStatFollowing;

  /// No description provided for @meStatFollowers.
  ///
  /// In zh, this message translates to:
  /// **'粉丝'**
  String get meStatFollowers;

  /// No description provided for @meBottlesCount.
  ///
  /// In zh, this message translates to:
  /// **'{n} 个'**
  String meBottlesCount(int n);

  /// No description provided for @meSettings.
  ///
  /// In zh, this message translates to:
  /// **'设置'**
  String get meSettings;

  /// No description provided for @meLocation.
  ///
  /// In zh, this message translates to:
  /// **'定位'**
  String get meLocation;

  /// No description provided for @meLocationHint.
  ///
  /// In zh, this message translates to:
  /// **'关闭后不再获取和上报位置，距离按城市显示'**
  String get meLocationHint;

  /// No description provided for @meLanguage.
  ///
  /// In zh, this message translates to:
  /// **'语言'**
  String get meLanguage;

  /// No description provided for @meLanguageFollowSystem.
  ///
  /// In zh, this message translates to:
  /// **'跟随系统'**
  String get meLanguageFollowSystem;

  /// No description provided for @meTheme.
  ///
  /// In zh, this message translates to:
  /// **'外观'**
  String get meTheme;

  /// No description provided for @meThemeFollowSystem.
  ///
  /// In zh, this message translates to:
  /// **'跟随系统'**
  String get meThemeFollowSystem;

  /// No description provided for @meThemeLight.
  ///
  /// In zh, this message translates to:
  /// **'浅色'**
  String get meThemeLight;

  /// No description provided for @meThemeDark.
  ///
  /// In zh, this message translates to:
  /// **'深色'**
  String get meThemeDark;

  /// No description provided for @meExportLogs.
  ///
  /// In zh, this message translates to:
  /// **'导出日志'**
  String get meExportLogs;

  /// No description provided for @meExportLogsHint.
  ///
  /// In zh, this message translates to:
  /// **'遇到问题时发给客服，帮助定位'**
  String get meExportLogsHint;

  /// No description provided for @meExportLogsConfirm.
  ///
  /// In zh, this message translates to:
  /// **'将导出最近 3 天的运行记录（最多 5 MB）。\n\n包含：访问过的接口、错误信息、页面跳转。\n不包含：密码、验证码、登录凭证。'**
  String get meExportLogsConfirm;

  /// No description provided for @meExportLogsBodyWarn.
  ///
  /// In zh, this message translates to:
  /// **'提示：当前开启了详细日志，导出内容还包含请求内容。'**
  String get meExportLogsBodyWarn;

  /// No description provided for @meExportLogsGo.
  ///
  /// In zh, this message translates to:
  /// **'导出并分享'**
  String get meExportLogsGo;

  /// No description provided for @meAbout.
  ///
  /// In zh, this message translates to:
  /// **'关于'**
  String get meAbout;

  /// No description provided for @meContact.
  ///
  /// In zh, this message translates to:
  /// **'联系我们'**
  String get meContact;

  /// No description provided for @meLogout.
  ///
  /// In zh, this message translates to:
  /// **'退出登录'**
  String get meLogout;

  /// No description provided for @meLogoutConfirm.
  ///
  /// In zh, this message translates to:
  /// **'确定要退出登录吗？'**
  String get meLogoutConfirm;

  /// No description provided for @meDeleteAccount.
  ///
  /// In zh, this message translates to:
  /// **'注销账号'**
  String get meDeleteAccount;

  /// No description provided for @meDeleteAccountWarn.
  ///
  /// In zh, this message translates to:
  /// **'注销后资料与瓶子会被清除，手机号可以重新注册。此操作不可撤销。'**
  String get meDeleteAccountWarn;

  /// No description provided for @meDeleteAccountConfirm.
  ///
  /// In zh, this message translates to:
  /// **'确认注销'**
  String get meDeleteAccountConfirm;

  /// No description provided for @walletTitle.
  ///
  /// In zh, this message translates to:
  /// **'钱包'**
  String get walletTitle;

  /// No description provided for @walletGoRecharge.
  ///
  /// In zh, this message translates to:
  /// **'去充值'**
  String get walletGoRecharge;

  /// No description provided for @walletCurrentBalance.
  ///
  /// In zh, this message translates to:
  /// **'当前金币余额'**
  String get walletCurrentBalance;

  /// No description provided for @walletTotalRecharged.
  ///
  /// In zh, this message translates to:
  /// **'累计充值'**
  String get walletTotalRecharged;

  /// No description provided for @walletTotalSpent.
  ///
  /// In zh, this message translates to:
  /// **'累计消费'**
  String get walletTotalSpent;

  /// No description provided for @walletTabIncome.
  ///
  /// In zh, this message translates to:
  /// **'收入'**
  String get walletTabIncome;

  /// No description provided for @walletTabExpense.
  ///
  /// In zh, this message translates to:
  /// **'支出'**
  String get walletTabExpense;

  /// No description provided for @walletSceneUnlock.
  ///
  /// In zh, this message translates to:
  /// **'解锁回信'**
  String get walletSceneUnlock;

  /// No description provided for @walletSceneChat.
  ///
  /// In zh, this message translates to:
  /// **'开启聊天'**
  String get walletSceneChat;

  /// No description provided for @walletSceneMsg.
  ///
  /// In zh, this message translates to:
  /// **'发送消息'**
  String get walletSceneMsg;

  /// No description provided for @walletSceneGift.
  ///
  /// In zh, this message translates to:
  /// **'送出礼物'**
  String get walletSceneGift;

  /// No description provided for @walletSceneRecharge.
  ///
  /// In zh, this message translates to:
  /// **'充值'**
  String get walletSceneRecharge;

  /// No description provided for @walletSceneReward.
  ///
  /// In zh, this message translates to:
  /// **'新人奖励'**
  String get walletSceneReward;

  /// No description provided for @walletSceneCheckin.
  ///
  /// In zh, this message translates to:
  /// **'每日签到'**
  String get walletSceneCheckin;

  /// No description provided for @walletSceneShare.
  ///
  /// In zh, this message translates to:
  /// **'分享奖励'**
  String get walletSceneShare;

  /// No description provided for @walletSceneAdReward.
  ///
  /// In zh, this message translates to:
  /// **'看激励视频'**
  String get walletSceneAdReward;

  /// No description provided for @payWaitingTitle.
  ///
  /// In zh, this message translates to:
  /// **'支付处理中'**
  String get payWaitingTitle;

  /// No description provided for @payWaitingBody.
  ///
  /// In zh, this message translates to:
  /// **'交易还在进行，请不要重复支付'**
  String get payWaitingBody;

  /// No description provided for @payWaitingElapsed.
  ///
  /// In zh, this message translates to:
  /// **'已等待 {sec} 秒'**
  String payWaitingElapsed(int sec);

  /// No description provided for @payVerifyingTitle.
  ///
  /// In zh, this message translates to:
  /// **'正在核实票据'**
  String get payVerifyingTitle;

  /// No description provided for @payVerifyingBody.
  ///
  /// In zh, this message translates to:
  /// **'正在向 App Store 核实这笔交易'**
  String get payVerifyingBody;

  /// No description provided for @payDoneButton.
  ///
  /// In zh, this message translates to:
  /// **'我已完成支付'**
  String get payDoneButton;

  /// No description provided for @payTroubleButton.
  ///
  /// In zh, this message translates to:
  /// **'遇到问题'**
  String get payTroubleButton;

  /// No description provided for @paySuccessTitle.
  ///
  /// In zh, this message translates to:
  /// **'充值成功'**
  String get paySuccessTitle;

  /// No description provided for @paySuccessCoins.
  ///
  /// In zh, this message translates to:
  /// **'{coins} 金币已到账'**
  String paySuccessCoins(int coins);

  /// No description provided for @payBalanceNow.
  ///
  /// In zh, this message translates to:
  /// **'当前余额 {coins}'**
  String payBalanceNow(int coins);

  /// No description provided for @payBackToScene.
  ///
  /// In zh, this message translates to:
  /// **'回到刚才的页面'**
  String get payBackToScene;

  /// No description provided for @payViewRecords.
  ///
  /// In zh, this message translates to:
  /// **'查看充值记录'**
  String get payViewRecords;

  /// No description provided for @payDroppedTitle.
  ///
  /// In zh, this message translates to:
  /// **'已扣款未到账'**
  String get payDroppedTitle;

  /// No description provided for @payDroppedBody.
  ///
  /// In zh, this message translates to:
  /// **'钱不会丢。若 24 小时内仍未到账，请联系客服，我们会人工补单'**
  String get payDroppedBody;

  /// No description provided for @payRecheckButton.
  ///
  /// In zh, this message translates to:
  /// **'主动查单'**
  String get payRecheckButton;

  /// No description provided for @payContactSupport.
  ///
  /// In zh, this message translates to:
  /// **'联系客服'**
  String get payContactSupport;

  /// No description provided for @payFailedTitle.
  ///
  /// In zh, this message translates to:
  /// **'支付未完成'**
  String get payFailedTitle;

  /// No description provided for @payFailedBody.
  ///
  /// In zh, this message translates to:
  /// **'这笔交易没有成功，你没有被扣款'**
  String get payFailedBody;

  /// No description provided for @payBackToRecharge.
  ///
  /// In zh, this message translates to:
  /// **'重新选择档位'**
  String get payBackToRecharge;

  /// No description provided for @payLeaveConfirm.
  ///
  /// In zh, this message translates to:
  /// **'交易可能仍在进行，确定离开？'**
  String get payLeaveConfirm;

  /// No description provided for @payLeaveStay.
  ///
  /// In zh, this message translates to:
  /// **'继续等待'**
  String get payLeaveStay;

  /// No description provided for @payLeaveGo.
  ///
  /// In zh, this message translates to:
  /// **'离开'**
  String get payLeaveGo;

  /// No description provided for @payLeftRunning.
  ///
  /// In zh, this message translates to:
  /// **'交易仍在进行，可在充值记录里查看'**
  String get payLeftRunning;

  /// No description provided for @payMockTitle.
  ///
  /// In zh, this message translates to:
  /// **'模拟支付（联调）'**
  String get payMockTitle;

  /// No description provided for @payMockSuccess.
  ///
  /// In zh, this message translates to:
  /// **'模拟支付成功'**
  String get payMockSuccess;

  /// No description provided for @payMockFail.
  ///
  /// In zh, this message translates to:
  /// **'模拟支付失败'**
  String get payMockFail;

  /// No description provided for @payMockNoResponse.
  ///
  /// In zh, this message translates to:
  /// **'不返回结果（模拟掉单）'**
  String get payMockNoResponse;

  /// No description provided for @payRecordsTitle.
  ///
  /// In zh, this message translates to:
  /// **'充值记录'**
  String get payRecordsTitle;

  /// No description provided for @payRestore.
  ///
  /// In zh, this message translates to:
  /// **'恢复购买'**
  String get payRestore;

  /// No description provided for @payRestoreDone.
  ///
  /// In zh, this message translates to:
  /// **'{n} 笔订单已更新'**
  String payRestoreDone(int n);

  /// No description provided for @payRestoreNone.
  ///
  /// In zh, this message translates to:
  /// **'没有需要更新的订单'**
  String get payRestoreNone;

  /// No description provided for @payStatusPaid.
  ///
  /// In zh, this message translates to:
  /// **'已到账'**
  String get payStatusPaid;

  /// No description provided for @payStatusPending.
  ///
  /// In zh, this message translates to:
  /// **'处理中'**
  String get payStatusPending;

  /// No description provided for @payStatusFailed.
  ///
  /// In zh, this message translates to:
  /// **'支付失败'**
  String get payStatusFailed;

  /// No description provided for @payStatusRefunded.
  ///
  /// In zh, this message translates to:
  /// **'已退款'**
  String get payStatusRefunded;

  /// No description provided for @payRecordsEmpty.
  ///
  /// In zh, this message translates to:
  /// **'还没有充值记录'**
  String get payRecordsEmpty;

  /// No description provided for @payRecordsEmptyHint.
  ///
  /// In zh, this message translates to:
  /// **'去充值'**
  String get payRecordsEmptyHint;

  /// No description provided for @payCoinsAmount.
  ///
  /// In zh, this message translates to:
  /// **'{coins} 金币'**
  String payCoinsAmount(int coins);

  /// No description provided for @voidedTitle.
  ///
  /// In zh, this message translates to:
  /// **'有一笔充值被退款了'**
  String get voidedTitle;

  /// No description provided for @voidedBody.
  ///
  /// In zh, this message translates to:
  /// **'渠道已退还这笔充值，对应的金币已从账户扣回。由于其中一部分已经消费，余额出现负数'**
  String get voidedBody;

  /// No description provided for @voidedFrozen.
  ///
  /// In zh, this message translates to:
  /// **'余额为负时，扔瓶 / 解锁 / 送礼暂停，充值仍然可用。补足后自动恢复'**
  String get voidedFrozen;

  /// No description provided for @voidedRelated.
  ///
  /// In zh, this message translates to:
  /// **'相关流水'**
  String get voidedRelated;

  /// No description provided for @voidedGoRecharge.
  ///
  /// In zh, this message translates to:
  /// **'去充值'**
  String get voidedGoRecharge;

  /// No description provided for @voidedNotMe.
  ///
  /// In zh, this message translates to:
  /// **'这不是我操作的'**
  String get voidedNotMe;

  /// No description provided for @voidedBalance.
  ///
  /// In zh, this message translates to:
  /// **'当前金币余额'**
  String get voidedBalance;

  /// No description provided for @voidedTotals.
  ///
  /// In zh, this message translates to:
  /// **'累计充值 {recharged} · 累计消费 {spent}'**
  String voidedTotals(int recharged, int spent);

  /// No description provided for @walletNegativeBanner.
  ///
  /// In zh, this message translates to:
  /// **'余额为负，点此了解原因'**
  String get walletNegativeBanner;

  /// No description provided for @walletSceneRefund.
  ///
  /// In zh, this message translates to:
  /// **'充值退款'**
  String get walletSceneRefund;

  /// No description provided for @walletSceneRewind.
  ///
  /// In zh, this message translates to:
  /// **'撤回'**
  String get walletSceneRewind;

  /// No description provided for @walletSceneDiscoverSkip.
  ///
  /// In zh, this message translates to:
  /// **'左滑跳过'**
  String get walletSceneDiscoverSkip;

  /// No description provided for @walletEmpty.
  ///
  /// In zh, this message translates to:
  /// **'还没有任何流水'**
  String get walletEmpty;

  /// No description provided for @legalNotReadyTitle.
  ///
  /// In zh, this message translates to:
  /// **'正文还在准备中'**
  String get legalNotReadyTitle;

  /// No description provided for @legalNotReadyBody.
  ///
  /// In zh, this message translates to:
  /// **'这份文本还没在后台配置，可以先在浏览器里看网页版。'**
  String get legalNotReadyBody;

  /// No description provided for @legalOpenInBrowser.
  ///
  /// In zh, this message translates to:
  /// **'在浏览器中打开'**
  String get legalOpenInBrowser;

  /// No description provided for @rechargeTitle.
  ///
  /// In zh, this message translates to:
  /// **'充值金币'**
  String get rechargeTitle;

  /// No description provided for @rechargeChoosePackage.
  ///
  /// In zh, this message translates to:
  /// **'选择档位'**
  String get rechargeChoosePackage;

  /// No description provided for @rechargeBonus.
  ///
  /// In zh, this message translates to:
  /// **'多送 {percent}%'**
  String rechargeBonus(int percent);

  /// No description provided for @rechargePaymentMethod.
  ///
  /// In zh, this message translates to:
  /// **'支付方式'**
  String get rechargePaymentMethod;

  /// No description provided for @rechargeUpiSub.
  ///
  /// In zh, this message translates to:
  /// **'GPay · PhonePe · Paytm'**
  String get rechargeUpiSub;

  /// No description provided for @rechargeCardSub.
  ///
  /// In zh, this message translates to:
  /// **'Card / Netbanking'**
  String get rechargeCardSub;

  /// No description provided for @rechargeIosChannel.
  ///
  /// In zh, this message translates to:
  /// **'App Store 付款'**
  String get rechargeIosChannel;

  /// No description provided for @rechargeIosHint.
  ///
  /// In zh, this message translates to:
  /// **'数字商品由 App Store 收款，实际价格以商店展示为准'**
  String get rechargeIosHint;

  /// No description provided for @rechargePayAndGet.
  ///
  /// In zh, this message translates to:
  /// **'支付 {price} · 得 {coins} 金币'**
  String rechargePayAndGet(String price, int coins);

  /// No description provided for @giftPanelTitle.
  ///
  /// In zh, this message translates to:
  /// **'送出礼物'**
  String get giftPanelTitle;

  /// No description provided for @giftSend.
  ///
  /// In zh, this message translates to:
  /// **'送出'**
  String get giftSend;

  /// No description provided for @giftQty.
  ///
  /// In zh, this message translates to:
  /// **'×{n}'**
  String giftQty(int n);

  /// No description provided for @giftFromBag.
  ///
  /// In zh, this message translates to:
  /// **'背包抵扣 ×{n}'**
  String giftFromBag(int n);

  /// No description provided for @giftNeedChatFirst.
  ///
  /// In zh, this message translates to:
  /// **'先和 TA 打个招呼，开聊后就能送礼'**
  String get giftNeedChatFirst;

  /// No description provided for @giftSentToast.
  ///
  /// In zh, this message translates to:
  /// **'已送出「{gift}」'**
  String giftSentToast(String gift);

  /// No description provided for @giftOutOfStockTitle.
  ///
  /// In zh, this message translates to:
  /// **'礼物数量不足'**
  String get giftOutOfStockTitle;

  /// No description provided for @giftGoBuy.
  ///
  /// In zh, this message translates to:
  /// **'去道具页购买'**
  String get giftGoBuy;

  /// No description provided for @quotaUsedUpTitle.
  ///
  /// In zh, this message translates to:
  /// **'今天的 {n} 次捞完了'**
  String quotaUsedUpTitle(int n);

  /// No description provided for @quotaUsedUpBody.
  ///
  /// In zh, this message translates to:
  /// **'明天 00:00 自动恢复。\n想现在继续，有两个办法。'**
  String get quotaUsedUpBody;

  /// No description provided for @quotaPackTitle.
  ///
  /// In zh, this message translates to:
  /// **'捞瓶次数包 ×{n}'**
  String quotaPackTitle(int n);

  /// No description provided for @quotaPackHint.
  ///
  /// In zh, this message translates to:
  /// **'立即到账，不过期'**
  String get quotaPackHint;

  /// No description provided for @quotaBuyWithCoins.
  ///
  /// In zh, this message translates to:
  /// **'用 {n} 金币购买'**
  String quotaBuyWithCoins(int n);

  /// No description provided for @quotaWatchAdForOne.
  ///
  /// In zh, this message translates to:
  /// **'看段视频，免费 +1 次'**
  String get quotaWatchAdForOne;

  /// No description provided for @quotaThrowUsedUpTitle.
  ///
  /// In zh, this message translates to:
  /// **'今天的 {n} 次都扔完了'**
  String quotaThrowUsedUpTitle(int n);

  /// No description provided for @rewardTitle.
  ///
  /// In zh, this message translates to:
  /// **'每日奖励'**
  String get rewardTitle;

  /// No description provided for @rewardStreakBadge.
  ///
  /// In zh, this message translates to:
  /// **'已连签 {n} 天'**
  String rewardStreakBadge(int n);

  /// No description provided for @rewardStreakTitle.
  ///
  /// In zh, this message translates to:
  /// **'连续签到 {n} 天'**
  String rewardStreakTitle(int n);

  /// No description provided for @rewardDay7.
  ///
  /// In zh, this message translates to:
  /// **'第 7 天有大奖'**
  String get rewardDay7;

  /// No description provided for @rewardCheckinGet.
  ///
  /// In zh, this message translates to:
  /// **'签到领 {n} 金币'**
  String rewardCheckinGet(int n);

  /// No description provided for @rewardCheckedIn.
  ///
  /// In zh, this message translates to:
  /// **'今天已签到'**
  String get rewardCheckedIn;

  /// No description provided for @rewardEarnMore.
  ///
  /// In zh, this message translates to:
  /// **'还能这样赚'**
  String get rewardEarnMore;

  /// No description provided for @rewardWatchAd.
  ///
  /// In zh, this message translates to:
  /// **'看激励视频'**
  String get rewardWatchAd;

  /// No description provided for @rewardWatchAdHint.
  ///
  /// In zh, this message translates to:
  /// **'今天 {done} / {total} 次 · 每次 +{coins}'**
  String rewardWatchAdHint(int done, int total, int coins);

  /// No description provided for @rewardInvite.
  ///
  /// In zh, this message translates to:
  /// **'邀请好友'**
  String get rewardInvite;

  /// No description provided for @rewardInviteHint.
  ///
  /// In zh, this message translates to:
  /// **'好友注册后，双方各得 {n}'**
  String rewardInviteHint(int n);

  /// No description provided for @rewardPostMoment.
  ///
  /// In zh, this message translates to:
  /// **'发一条动态'**
  String get rewardPostMoment;

  /// No description provided for @rewardPostMomentHint.
  ///
  /// In zh, this message translates to:
  /// **'每天第一条 +{n}'**
  String rewardPostMomentHint(int n);

  /// No description provided for @rewardGoWatch.
  ///
  /// In zh, this message translates to:
  /// **'去看'**
  String get rewardGoWatch;

  /// No description provided for @rewardInviteAction.
  ///
  /// In zh, this message translates to:
  /// **'邀请'**
  String get rewardInviteAction;

  /// No description provided for @rewardGoPost.
  ///
  /// In zh, this message translates to:
  /// **'去发'**
  String get rewardGoPost;

  /// No description provided for @rewardGot.
  ///
  /// In zh, this message translates to:
  /// **'到账 +{n} 金币'**
  String rewardGot(int n);

  /// No description provided for @relationTitle.
  ///
  /// In zh, this message translates to:
  /// **'我的关系'**
  String get relationTitle;

  /// No description provided for @relationTabLikedMe.
  ///
  /// In zh, this message translates to:
  /// **'喜欢我的 {n}'**
  String relationTabLikedMe(int n);

  /// No description provided for @relationTabILiked.
  ///
  /// In zh, this message translates to:
  /// **'我喜欢的 {n}'**
  String relationTabILiked(int n);

  /// No description provided for @relationTabViewedMe.
  ///
  /// In zh, this message translates to:
  /// **'看过我'**
  String get relationTabViewedMe;

  /// No description provided for @relationInteractions.
  ///
  /// In zh, this message translates to:
  /// **'互动 {n} 次'**
  String relationInteractions(int n);

  /// No description provided for @relationStageStranger.
  ///
  /// In zh, this message translates to:
  /// **'陌生'**
  String get relationStageStranger;

  /// No description provided for @relationStageKnown.
  ///
  /// In zh, this message translates to:
  /// **'认识'**
  String get relationStageKnown;

  /// No description provided for @relationStageFamiliar.
  ///
  /// In zh, this message translates to:
  /// **'熟悉'**
  String get relationStageFamiliar;

  /// No description provided for @relationStrengthHint.
  ///
  /// In zh, this message translates to:
  /// **'强度分由「互动次数 × 时间衰减」算出，30 天不互动会回落'**
  String get relationStrengthHint;

  /// No description provided for @stateEmptyOceanTitle.
  ///
  /// In zh, this message translates to:
  /// **'海面很安静'**
  String get stateEmptyOceanTitle;

  /// No description provided for @stateEmptyOceanBody.
  ///
  /// In zh, this message translates to:
  /// **'换个语言筛选，或者自己先扔一个\n通常十几分钟就会有人捞到'**
  String get stateEmptyOceanBody;

  /// No description provided for @stateEmptyOceanCta.
  ///
  /// In zh, this message translates to:
  /// **'扔一个瓶子'**
  String get stateEmptyOceanCta;

  /// No description provided for @stateEmptyChatsTitle.
  ///
  /// In zh, this message translates to:
  /// **'还没有人和你说话'**
  String get stateEmptyChatsTitle;

  /// No description provided for @stateEmptyChatsBody.
  ///
  /// In zh, this message translates to:
  /// **'回一个你捞到的瓶子，\n或者去发现页打个招呼'**
  String get stateEmptyChatsBody;

  /// No description provided for @stateEmptyChatsCta.
  ///
  /// In zh, this message translates to:
  /// **'去捞一个瓶子'**
  String get stateEmptyChatsCta;

  /// No description provided for @stateEmptyChatsAlt.
  ///
  /// In zh, this message translates to:
  /// **'看看谁在线'**
  String get stateEmptyChatsAlt;

  /// No description provided for @stateEmptyMomentsTitle.
  ///
  /// In zh, this message translates to:
  /// **'这里还很空'**
  String get stateEmptyMomentsTitle;

  /// No description provided for @stateEmptyMomentsBody.
  ///
  /// In zh, this message translates to:
  /// **'发第一条动态，让别人找到你'**
  String get stateEmptyMomentsBody;

  /// No description provided for @stateEmptyMomentsCta.
  ///
  /// In zh, this message translates to:
  /// **'发一条动态'**
  String get stateEmptyMomentsCta;

  /// No description provided for @stateEmptyBottlesTitle.
  ///
  /// In zh, this message translates to:
  /// **'还没扔过瓶子'**
  String get stateEmptyBottlesTitle;

  /// No description provided for @stateEmptyBottlesBody.
  ///
  /// In zh, this message translates to:
  /// **'写下第一句话，看看它能漂多远'**
  String get stateEmptyBottlesBody;

  /// No description provided for @stateEmptyScoopedTitle.
  ///
  /// In zh, this message translates to:
  /// **'还没捞过瓶子'**
  String get stateEmptyScoopedTitle;

  /// No description provided for @stateEmptyScoopedBody.
  ///
  /// In zh, this message translates to:
  /// **'去海面捞一个，遇见陌生的心情'**
  String get stateEmptyScoopedBody;

  /// No description provided for @stateEmptyCollectedTitle.
  ///
  /// In zh, this message translates to:
  /// **'还没有收藏'**
  String get stateEmptyCollectedTitle;

  /// No description provided for @stateEmptyCollectedBody.
  ///
  /// In zh, this message translates to:
  /// **'在瓶子详情点 ♡ 收藏，方便以后回来看'**
  String get stateEmptyCollectedBody;

  /// No description provided for @bottleTraceShort.
  ///
  /// In zh, this message translates to:
  /// **'轨迹'**
  String get bottleTraceShort;

  /// No description provided for @stateInsufficientTitle.
  ///
  /// In zh, this message translates to:
  /// **'还差 {n} 金币'**
  String stateInsufficientTitle(int n);

  /// No description provided for @stateInsufficientBody.
  ///
  /// In zh, this message translates to:
  /// **'「{item}」需要 {need}，你现在有 {have}。'**
  String stateInsufficientBody(String item, int need, int have);

  /// No description provided for @stateInsufficientCta.
  ///
  /// In zh, this message translates to:
  /// **'去充值'**
  String get stateInsufficientCta;

  /// No description provided for @stateInsufficientAlt.
  ///
  /// In zh, this message translates to:
  /// **'换一个便宜的礼物'**
  String get stateInsufficientAlt;

  /// No description provided for @stateOfflineTitle.
  ///
  /// In zh, this message translates to:
  /// **'网络好像断了'**
  String get stateOfflineTitle;

  /// No description provided for @stateOfflineBody.
  ///
  /// In zh, this message translates to:
  /// **'检查一下 Wi-Fi 或数据网络，\n刚才的内容还在，不会丢'**
  String get stateOfflineBody;

  /// No description provided for @stateOfflineCta.
  ///
  /// In zh, this message translates to:
  /// **'重新加载'**
  String get stateOfflineCta;

  /// No description provided for @stateErrorTitle.
  ///
  /// In zh, this message translates to:
  /// **'出了点问题'**
  String get stateErrorTitle;

  /// No description provided for @stateErrorBody.
  ///
  /// In zh, this message translates to:
  /// **'服务器没有回应，稍后再试一次'**
  String get stateErrorBody;

  /// No description provided for @stateLoadFailed.
  ///
  /// In zh, this message translates to:
  /// **'加载失败，下拉重试'**
  String get stateLoadFailed;

  /// No description provided for @authOauthNav.
  ///
  /// In zh, this message translates to:
  /// **'{provider} 登录'**
  String authOauthNav(String provider);

  /// No description provided for @authOauthPendingTitle.
  ///
  /// In zh, this message translates to:
  /// **'正在校验 {provider} 账号'**
  String authOauthPendingTitle(String provider);

  /// No description provided for @authOauthPendingBody.
  ///
  /// In zh, this message translates to:
  /// **'正在与服务器确认身份'**
  String get authOauthPendingBody;

  /// No description provided for @authConflictTitle.
  ///
  /// In zh, this message translates to:
  /// **'这个邮箱已经有账号了'**
  String get authConflictTitle;

  /// No description provided for @authConflictBody.
  ///
  /// In zh, this message translates to:
  /// **'为了账号安全，不会自动合并'**
  String get authConflictBody;

  /// No description provided for @authConflictHint.
  ///
  /// In zh, this message translates to:
  /// **'请先用密码登录这个账号，再到「我的 → 账号与安全」里绑定。绑定之后，下次就能直接用第三方账号登录了。'**
  String get authConflictHint;

  /// No description provided for @authUsePassword.
  ///
  /// In zh, this message translates to:
  /// **'用密码登录'**
  String get authUsePassword;

  /// No description provided for @authSwitchAccount.
  ///
  /// In zh, this message translates to:
  /// **'换一个 {provider} 账号'**
  String authSwitchAccount(String provider);

  /// No description provided for @locationIntroTitle.
  ///
  /// In zh, this message translates to:
  /// **'让瓶子知道你在哪儿'**
  String get locationIntroTitle;

  /// No description provided for @locationIntroLead.
  ///
  /// In zh, this message translates to:
  /// **'开启后可以：'**
  String get locationIntroLead;

  /// No description provided for @locationReasonDistance.
  ///
  /// In zh, this message translates to:
  /// **'看到瓶子漂了多远'**
  String get locationReasonDistance;

  /// No description provided for @locationReasonDistanceSub.
  ///
  /// In zh, this message translates to:
  /// **'「来自 2.4 km 外」'**
  String get locationReasonDistanceSub;

  /// No description provided for @locationReasonNearby.
  ///
  /// In zh, this message translates to:
  /// **'优先推荐附近的人'**
  String get locationReasonNearby;

  /// No description provided for @locationReasonNearbySub.
  ///
  /// In zh, this message translates to:
  /// **'距离只是排序维度之一'**
  String get locationReasonNearbySub;

  /// No description provided for @locationReasonPlace.
  ///
  /// In zh, this message translates to:
  /// **'发瓶子时自动带上地点'**
  String get locationReasonPlace;

  /// No description provided for @locationReasonPlaceSub.
  ///
  /// In zh, this message translates to:
  /// **'每次都可以改或不带'**
  String get locationReasonPlaceSub;

  /// No description provided for @locationIntroPrivacy.
  ///
  /// In zh, this message translates to:
  /// **'别人只能看到城市和大致距离，看不到你的具体位置。随时可以在设置里关掉。'**
  String get locationIntroPrivacy;

  /// No description provided for @locationEnable.
  ///
  /// In zh, this message translates to:
  /// **'开启定位'**
  String get locationEnable;

  /// No description provided for @locationLater.
  ///
  /// In zh, this message translates to:
  /// **'暂不开启'**
  String get locationLater;

  /// No description provided for @commonGotIt.
  ///
  /// In zh, this message translates to:
  /// **'知道了'**
  String get commonGotIt;

  /// No description provided for @accountSecurityTitle.
  ///
  /// In zh, this message translates to:
  /// **'账号与安全'**
  String get accountSecurityTitle;

  /// No description provided for @accountLoginMethods.
  ///
  /// In zh, this message translates to:
  /// **'登录方式'**
  String get accountLoginMethods;

  /// No description provided for @accountSecuritySection.
  ///
  /// In zh, this message translates to:
  /// **'安全'**
  String get accountSecuritySection;

  /// No description provided for @accountPhone.
  ///
  /// In zh, this message translates to:
  /// **'手机号'**
  String get accountPhone;

  /// No description provided for @accountEmail.
  ///
  /// In zh, this message translates to:
  /// **'邮箱'**
  String get accountEmail;

  /// No description provided for @accountBound.
  ///
  /// In zh, this message translates to:
  /// **'已绑定'**
  String get accountBound;

  /// No description provided for @accountNotBound.
  ///
  /// In zh, this message translates to:
  /// **'未绑定'**
  String get accountNotBound;

  /// No description provided for @accountBind.
  ///
  /// In zh, this message translates to:
  /// **'绑定'**
  String get accountBind;

  /// No description provided for @accountUnbind.
  ///
  /// In zh, this message translates to:
  /// **'解绑'**
  String get accountUnbind;

  /// No description provided for @accountAppleIosOnly.
  ///
  /// In zh, this message translates to:
  /// **'仅 iOS 可绑定'**
  String get accountAppleIosOnly;

  /// No description provided for @accountChangePassword.
  ///
  /// In zh, this message translates to:
  /// **'修改密码'**
  String get accountChangePassword;

  /// No description provided for @accountDelete.
  ///
  /// In zh, this message translates to:
  /// **'删除账号'**
  String get accountDelete;

  /// No description provided for @accountKeepOneHint.
  ///
  /// In zh, this message translates to:
  /// **'至少保留一种可登录方式。只剩一种时，那一项的解绑入口置灰。'**
  String get accountKeepOneHint;

  /// No description provided for @accountBoundTitle.
  ///
  /// In zh, this message translates to:
  /// **'已绑定'**
  String get accountBoundTitle;

  /// No description provided for @accountBoundBody.
  ///
  /// In zh, this message translates to:
  /// **'下次可以直接用它登录了。'**
  String get accountBoundBody;

  /// No description provided for @accountTakenTitle.
  ///
  /// In zh, this message translates to:
  /// **'这个 {provider} 账号已被占用'**
  String accountTakenTitle(String provider);

  /// No description provided for @accountTakenBody.
  ///
  /// In zh, this message translates to:
  /// **'它已经绑在另一个 Drift 账号上。一个 {provider} 账号只能绑一个 Drift 账号。'**
  String accountTakenBody(String provider);

  /// No description provided for @accountTakenHint.
  ///
  /// In zh, this message translates to:
  /// **'如果那个账号也是你的，先用它登录后解绑，再回到这里绑定。'**
  String get accountTakenHint;

  /// No description provided for @accountBindFailed.
  ///
  /// In zh, this message translates to:
  /// **'绑定失败，请稍后再试'**
  String get accountBindFailed;

  /// No description provided for @accountDeleteTitle.
  ///
  /// In zh, this message translates to:
  /// **'删除账号'**
  String get accountDeleteTitle;

  /// No description provided for @accountDeleteBody.
  ///
  /// In zh, this message translates to:
  /// **'删除后资料与瓶子会被清除，手机号可以重新注册。此操作不可撤销。'**
  String get accountDeleteBody;

  /// No description provided for @accountDeleteConfirm.
  ///
  /// In zh, this message translates to:
  /// **'确认删除'**
  String get accountDeleteConfirm;

  /// No description provided for @bottlePlaceSection.
  ///
  /// In zh, this message translates to:
  /// **'地点'**
  String get bottlePlaceSection;

  /// No description provided for @placeUseCurrent.
  ///
  /// In zh, this message translates to:
  /// **'使用当前位置'**
  String get placeUseCurrent;

  /// No description provided for @placeUseCurrentSub.
  ///
  /// In zh, this message translates to:
  /// **'也可以在地图上换一个'**
  String get placeUseCurrentSub;

  /// No description provided for @placeAdd.
  ///
  /// In zh, this message translates to:
  /// **'添加地点'**
  String get placeAdd;

  /// No description provided for @placeAddSub.
  ///
  /// In zh, this message translates to:
  /// **'这条不会带位置'**
  String get placeAddSub;

  /// No description provided for @mapPickTitle.
  ///
  /// In zh, this message translates to:
  /// **'选择地点'**
  String get mapPickTitle;

  /// No description provided for @mapDragHint.
  ///
  /// In zh, this message translates to:
  /// **'拖动地图来移动大头针'**
  String get mapDragHint;

  /// No description provided for @mapLocating.
  ///
  /// In zh, this message translates to:
  /// **'定位中…'**
  String get mapLocating;

  /// No description provided for @mapUseThis.
  ///
  /// In zh, this message translates to:
  /// **'用这个地点'**
  String get mapUseThis;

  /// No description provided for @mapHidePlace.
  ///
  /// In zh, this message translates to:
  /// **'不显示地点'**
  String get mapHidePlace;

  /// No description provided for @locationDeniedTitle.
  ///
  /// In zh, this message translates to:
  /// **'定位被拒绝了'**
  String get locationDeniedTitle;

  /// No description provided for @locationDeniedBody.
  ///
  /// In zh, this message translates to:
  /// **'「附近」需要位置权限。可以到系统设置里重新打开，也可以先看推荐。'**
  String get locationDeniedBody;

  /// No description provided for @locationOpenSettings.
  ///
  /// In zh, this message translates to:
  /// **'去设置开启'**
  String get locationOpenSettings;

  /// No description provided for @locationBrowseInstead.
  ///
  /// In zh, this message translates to:
  /// **'先逛逛推荐'**
  String get locationBrowseInstead;

  /// No description provided for @sparkGoChat.
  ///
  /// In zh, this message translates to:
  /// **'去聊聊'**
  String get sparkGoChat;

  /// No description provided for @sparkLater.
  ///
  /// In zh, this message translates to:
  /// **'以后再说'**
  String get sparkLater;

  /// No description provided for @sparkExpired.
  ///
  /// In zh, this message translates to:
  /// **'这次匹配已经过期了'**
  String get sparkExpired;
}

class _LDelegate extends LocalizationsDelegate<L> {
  const _LDelegate();

  @override
  Future<L> load(Locale locale) {
    return SynchronousFuture<L>(lookupL(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['en', 'zh'].contains(locale.languageCode);

  @override
  bool shouldReload(_LDelegate old) => false;
}

L lookupL(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'en':
      return LEn();
    case 'zh':
      return LZh();
  }

  throw FlutterError(
    'L.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}
