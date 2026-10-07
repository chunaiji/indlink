// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Chinese (`zh`).
class LZh extends L {
  LZh([String locale = 'zh']) : super(locale);

  @override
  String get appName => 'DRIFT';

  @override
  String get appTagline => '一条瓶子，可能漂到任何地方';

  @override
  String get tabOcean => '海洋';

  @override
  String get tabDiscover => '发现';

  @override
  String get tabChats => '消息';

  @override
  String get tabMoments => '动态';

  @override
  String get tabMe => '我的';

  @override
  String get commonConfirm => '确定';

  @override
  String get commonCancel => '取消';

  @override
  String get commonClose => '关闭';

  @override
  String get commonDone => '完成';

  @override
  String get commonRetry => '重试';

  @override
  String get commonReset => '重置';

  @override
  String get commonAll => '全部';

  @override
  String get commonOr => '或';

  @override
  String get commonOnline => '在线';

  @override
  String commonActiveAgo(String time) {
    return '$time活跃';
  }

  @override
  String get commonFemale => '女';

  @override
  String get commonMale => '男';

  @override
  String get commonSecret => '保密';

  @override
  String commonAgeValue(int age) {
    return '$age 岁';
  }

  @override
  String get commonAnonymous => '匿名';

  @override
  String commonDriftedDays(int days) {
    return '漂了 $days 天';
  }

  @override
  String get commonJustNow => '刚刚';

  @override
  String commonMinutesAgo(int n) {
    return '$n 分钟前';
  }

  @override
  String commonHoursAgo(int n) {
    return '$n 小时前';
  }

  @override
  String get commonYesterday => '昨天';

  @override
  String commonDaysAgo(int n) {
    return '$n 天前';
  }

  @override
  String get commonLastWeek => '上周';

  @override
  String get authLoginTitle => '欢迎回来';

  @override
  String get authRegisterTitle => '创建账号';

  @override
  String get authResetTitle => '重设密码';

  @override
  String get authPasswordHint => '密码';

  @override
  String get authRememberPassword => '记住密码';

  @override
  String get authNewPasswordHint => '设置新密码';

  @override
  String get authPasswordTooShort => '密码至少 8 位';

  @override
  String get authSignIn => '登录';

  @override
  String get authSignUp => '注册';

  @override
  String get authForgotPassword => '忘记密码？';

  @override
  String get authNoAccount => '还没有账号？';

  @override
  String get authHasAccount => '已有账号？';

  @override
  String get authUseOtpInstead => '用验证码登录';

  @override
  String get authUsePasswordInstead => '用密码登录';

  @override
  String get authRegisterNext => '下一步';

  @override
  String get authRegisterDone => '注册并开始';

  @override
  String get authPasswordUpdated => '密码已更新';

  @override
  String get authResetDone => '重设并登录';

  @override
  String get authSetPasswordHint => '之后就用这个密码登录';

  @override
  String get authStartWithPhone => '用手机号开始';

  @override
  String get authStartWithEmail => '用邮箱开始';

  @override
  String get authTabPhone => '手机号';

  @override
  String get authTabEmail => '邮箱';

  @override
  String get authTabPassword => '密码登录';

  @override
  String get authTabCode => '验证码登录';

  @override
  String get authPhoneHint => '手机号';

  @override
  String get authEmailHint => 'you@example.com';

  @override
  String get authSendCode => '发送验证码';

  @override
  String get authContinueWithGoogle => '用 Google 继续';

  @override
  String get authContinueWithApple => '用 Apple 继续';

  @override
  String get authAgreementPrefix => '继续即表示同意';

  @override
  String get authAgreeTitle => '请先阅读并同意协议';

  @override
  String get authAgreeBody => '继续前需要你同意以下条款：';

  @override
  String get authAgreeAndContinue => '同意并继续';

  @override
  String get authTerms => '《用户协议》';

  @override
  String get authAnd => '与';

  @override
  String get authPrivacy => '《隐私政策》';

  @override
  String get privacyGateTitle => '用户协议与隐私政策';

  @override
  String get privacyGateBody =>
      '登录前，请阅读并同意以下条款。我们仅在你授权后收集必要信息（如登录凭证；定位用于推荐附近的人，可拒绝），你可随时在设置中管理或撤回。';

  @override
  String get privacyGateAgree => '同意并继续';

  @override
  String get privacyGateExit => '不同意';

  @override
  String get authVerifyPhone => '验证手机号';

  @override
  String get authVerifyEmail => '验证邮箱';

  @override
  String get authEnterCode => '输入验证码';

  @override
  String get authSentTo => '已发送到';

  @override
  String get authChangeNumber => '改号码';

  @override
  String get authChangeEmail => '改邮箱';

  @override
  String authResendIn(int seconds) {
    return '$seconds 秒后可重新发送';
  }

  @override
  String get authResend => '重新发送验证码';

  @override
  String get authOtpWarning => '没收到？检查是否被拦截。同一号码 60 秒内只发一次，连续 5 次触发风控';

  @override
  String get authOtpWarningEmail => '没收到？先翻一下垃圾邮件。同一邮箱 60 秒内只发一次，连续 5 次触发风控';

  @override
  String get authVerifyAndLogin => '验证并登录';

  @override
  String get authInvalidPhone => '请输入正确的手机号';

  @override
  String get authInvalidCode => '验证码不正确';

  @override
  String get profileCompleteTitle => '完善资料';

  @override
  String profileStepOf(int current, int total) {
    return '第 $current 步，共 $total 步';
  }

  @override
  String get profileUploadAvatar => '上传头像';

  @override
  String get profileUploadAvatarHint => '有头像的人，收到的回信多得多';

  @override
  String get profileNicknameHint => '昵称';

  @override
  String get profileGenderLockTitle => '性别确认后不能再改';

  @override
  String profileGenderLockBody(String gender) {
    return '你选择的是「$gender」。为了保证匹配公平，提交后性别不可修改，确定吗？';
  }

  @override
  String get profileGenderRethink => '再想想';

  @override
  String get profileEditTitle => '编辑资料';

  @override
  String get profileEditAvatarHint => '点击更换头像';

  @override
  String get profileEditNickname => '昵称';

  @override
  String get profileEditNicknameRequired => '昵称不能为空';

  @override
  String get profileEditBirthday => '出生日期';

  @override
  String get profileEditBirthdayHint => '选择出生日期';

  @override
  String get profileEditBirthdayNote => '年龄按出生日期计算；日期本身只有你自己能看到。';

  @override
  String get profileEditSave => '保存';

  @override
  String get profileEditSaved => '资料已更新';

  @override
  String get profileGender => '性别';

  @override
  String get profileLanguages => '你说什么语言';

  @override
  String get profileLanguagePick => '选择你会的语言';

  @override
  String profileInterests(int n) {
    return '兴趣 · 至少选 $n 个';
  }

  @override
  String get profileEnableLocation => '开启定位';

  @override
  String get profileEnableLocationHint => '用来推荐附近的人，随时可关';

  @override
  String get profileStartDrifting => '开始漂流';

  @override
  String get profileNeedNickname => '先给自己起个名字';

  @override
  String profileNeedInterests(int n) {
    return '再选 $n 个兴趣就可以出发了';
  }

  @override
  String profileAgeRange(int min, int max) {
    return '年龄 $min-$max';
  }

  @override
  String profileAgeInvalid(int min, int max) {
    return '需 $min-$max 岁';
  }

  @override
  String get oceanTitle => '今晚的海';

  @override
  String oceanSubtitle(int count, String time) {
    final intl.NumberFormat countNumberFormat = intl.NumberFormat.compact(
      locale: localeName,
    );
    final String countString = countNumberFormat.format(count);

    return '$countString 条回应正在漂回来 · 夜场 $time 开';
  }

  @override
  String get oceanNightTitle => '夜场开始了';

  @override
  String oceanNightSubtitle(int hours, int minutes) {
    return '只有深夜瓶在漂 · 还剩 $hours 小时 $minutes 分';
  }

  @override
  String get oceanTagNight => '🌙 夜场';

  @override
  String get oceanTagNightShort => '🌙 深夜';

  @override
  String get oceanTagHollow => '树洞';

  @override
  String get oceanPullToRefresh => '下拉涨潮，换一批';

  @override
  String get oceanSeaReport => '今日海况';

  @override
  String get oceanSkinDay => '已切换为白天海面';

  @override
  String get oceanSkinNight => '已切换为夜晚海面';

  @override
  String get oceanSkinAuto => '海面恢复跟随时间';

  @override
  String oceanOnlineTonight(int n) {
    return '今晚 $n 人在线';
  }

  @override
  String get oceanThrowOne => '扔一个';

  @override
  String oceanThrowRemain(int n) {
    return '今日还剩 $n 次';
  }

  @override
  String get oceanThrowUnlimited => '深夜瓶不限次';

  @override
  String get oceanScoopOne => '捞一个';

  @override
  String oceanScoopRemain(int n) {
    return '今日还剩 $n 次';
  }

  @override
  String oceanTracePeek(int days, int cities) {
    return '漂了 $days 天 · $cities 座城市';
  }

  @override
  String get oceanSomethingBit => '有东西上钩了';

  @override
  String get oceanReleaseToOpen => '松手打开它';

  @override
  String get oceanOpeningNow => '正在打开…';

  @override
  String get oceanScoopFailed => '海面起风了，瓶子沉回去了 · 没有消耗次数';

  @override
  String get oceanPutBack => '放回海里';

  @override
  String get oceanPutBackHint => '对方不会收到打扰';

  @override
  String get oceanWriteReply => '写回信';

  @override
  String get oceanWriteReplyHint => '对方会收到通知';

  @override
  String oceanDriftedRecent(String city) {
    return '刚漂到你这里 · $city';
  }

  @override
  String oceanDriftedFromTo(int days, String city) {
    return '漂了 $days 天 · $city → 你这里';
  }

  @override
  String get oceanCastDone => '瓶子已经漂走了';

  @override
  String get bottleWriteTitle => '写一个瓶子';

  @override
  String bottleThrowRemainToday(int n) {
    return '今天还能扔 $n 个';
  }

  @override
  String get bottleContentHint => '写点什么，扔进海里…';

  @override
  String bottleCounter(int used, int max) {
    return '$used / $max';
  }

  @override
  String get bottleTags => '标签';

  @override
  String get bottleRange => '投放范围';

  @override
  String get bottleRangeNationwide => '全国';

  @override
  String get bottleRangeCity => '同城';

  @override
  String get bottleCastToSea => '抛向大海';

  @override
  String get bottleNeedContent => '先写点什么再扔';

  @override
  String get bottleScoopedTitle => '捞到的瓶子';

  @override
  String get bottleUnlockPill => '解锁';

  @override
  String get bottleUnlockOneTitle => '看看这条回信？';

  @override
  String bottleUnlockOneBody(int n) {
    return '花 $n 金币解锁这条回信，解锁后永久可见。';
  }

  @override
  String bottleUnlockOneConfirm(int n) {
    return '花 $n 金币解锁';
  }

  @override
  String get bottleUnlockedOne => '已解锁';

  @override
  String bottleRepliesHeader(int count, String time) {
    return '$count 条回信 · 最近一条 $time';
  }

  @override
  String get bottleReplyHint => '写下你的回信…';

  @override
  String get bottleReplySent => '回信已经送出去了';

  @override
  String get bottleCollected => '已收藏，去「我的瓶子」能找到它';

  @override
  String get bottleUncollected => '已取消收藏';

  @override
  String get bottleMineTitle => '我的瓶子';

  @override
  String get bottleTabThrown => '我扔的';

  @override
  String get bottleTabScooped => '我捞的';

  @override
  String get bottleTabCollected => '已收藏';

  @override
  String get bottleTraceTitle => '这个瓶子漂过哪些地方';

  @override
  String get bottleTraceHere => '你这里 · 被你捞起';

  @override
  String bottleTraceSummary(int people, int cities) {
    return '一共被 $people 个人看到，跨越 $cities 座城市';
  }

  @override
  String get bottleExpired => '已过期';

  @override
  String bottleStatViews(int n) {
    return '$n';
  }

  @override
  String get bottleMapTitle => '这个瓶子漂过的路';

  @override
  String get bottleMapPosterTitle => '一封信的旅程';

  @override
  String bottleMapDeparted(String date, String city) {
    return '$date 从 $city 出发';
  }

  @override
  String get bottleMapStatPeople => '个人看到';

  @override
  String get bottleMapStatCities => '座城市';

  @override
  String get bottleMapStatDays => '天漂流';

  @override
  String get bottleMapStatKm => '公里';

  @override
  String get bottleMapPrivacy => '海报只包含你写的内容和城市，不会出现任何回信者的信息';

  @override
  String get bottleMapSave => '保存图片';

  @override
  String get bottleMapShare => '分享给朋友';

  @override
  String get bottleMapSaved => '海报已经存到相册';

  @override
  String bottleTraceThrown(String city) {
    return '$city · 你扔出';
  }

  @override
  String bottleTraceSeen(String city, int n) {
    return '$city · 被 $n 人看到';
  }

  @override
  String bottleTraceReplied(String city, int n) {
    return '$city · 收到 $n 条回信';
  }

  @override
  String get discoverTitle => '今天在线的人';

  @override
  String get discoverTabRecommend => '推荐';

  @override
  String get discoverTabNearby => '附近';

  @override
  String get discoverTabNew => '新人';

  @override
  String get discoverSayHi => '打招呼';

  @override
  String get discoverViewProfile => '主页';

  @override
  String get discoverLike => '喜欢';

  @override
  String get discoverPass => '跳过';

  @override
  String get discoverRewind => '撤回';

  @override
  String get discoverRewindTitle => '把刚划走的人请回来？';

  @override
  String discoverRewindBody(int n) {
    return '花 $n 金币撤回上一次左滑，TA 会重新出现在卡片顶部。';
  }

  @override
  String discoverRewindConfirm(int n) {
    return '花 $n 金币撤回';
  }

  @override
  String discoverRewindDone(String name) {
    return '$name 回来了';
  }

  @override
  String get discoverPassChargeTitle => '跳过 TA？';

  @override
  String discoverPassChargeBody(int n) {
    return '左滑跳过将花 $n 金币，TA 不会再出现在你的卡片里。';
  }

  @override
  String discoverPassChargeConfirm(int n) {
    return '花 $n 金币跳过';
  }

  @override
  String get discoverFiltersTitle => '筛选';

  @override
  String get discoverLanguage => '语言';

  @override
  String get discoverInterest => '兴趣';

  @override
  String get discoverQuizTitle => '答题匹配';

  @override
  String get discoverQuizHint => '选相同答案的人，优先推给你';

  @override
  String get discoverDistance => '距离';

  @override
  String get discoverDistanceUnlimited => '不限';

  @override
  String discoverDistanceKm(int n) {
    return '$n km';
  }

  @override
  String get discoverAgeRange => '年龄';

  @override
  String discoverAgeRangeValue(int min, int max) {
    return '$min – $max 岁';
  }

  @override
  String discoverApplyFilter(int n) {
    return '应用筛选 · $n 人符合';
  }

  @override
  String get discoverNoMoreCards => '今天的人看完了，明天再来';

  @override
  String get discoverStatBottles => '瓶子';

  @override
  String get discoverStatMoments => '动态';

  @override
  String get discoverStatRelation => '关系';

  @override
  String get discoverCharm => '魅力';

  @override
  String profileCommonPoint(String value) {
    return '共同点 · 你们都选了「$value」';
  }

  @override
  String get profileTheirMoments => 'TA 的动态';

  @override
  String profileMomentCount(int n) {
    return '共 $n 条';
  }

  @override
  String get chatTitle => '消息';

  @override
  String get chatOneMessage => '发一条消息';

  @override
  String chatPricingHint(int free, int price) {
    return '前 $free 条免费，之后每条 $price 金币';
  }

  @override
  String chatPricingHintNoFree(int price) {
    return '每条消息 $price 金币';
  }

  @override
  String chatTabUnread(int n) {
    return '未读 $n';
  }

  @override
  String get chatTabBottleFriends => '瓶友';

  @override
  String get chatAnonymousFriend => '匿名瓶友';

  @override
  String get chatStartedFromBottle => '你们从一个瓶子开始聊天';

  @override
  String get chatRead => '已读';

  @override
  String get chatImageMessage => '［图片］';

  @override
  String chatGiftSentBy(String gift) {
    return '送出了「$gift」';
  }

  @override
  String chatGiftReceived(String gift) {
    return '对方送来「$gift」';
  }

  @override
  String chatCharmPlus(int n) {
    return '魅力 +$n';
  }

  @override
  String get chatInputHint => '说点什么…';

  @override
  String get chatDeletedUser => '已注销用户';

  @override
  String get chatDeletedUserHint => '对方已注销，会话只读';

  @override
  String get chatReconnecting => '正在重新连接…';

  @override
  String safetyReportUser(String name) {
    return '举报 $name';
  }

  @override
  String get safetyReasonPorn => '色情或低俗内容';

  @override
  String get safetyReasonSpam => '引流 / 广告 / 诈骗';

  @override
  String get safetyReasonHarass => '骚扰或人身攻击';

  @override
  String get safetyReasonMinor => '未成年人相关';

  @override
  String get safetyBlockOnly => '仅拉黑';

  @override
  String get safetyReportAndBlock => '举报并拉黑';

  @override
  String get safetyReported => '已收到举报，我们会尽快处理';

  @override
  String get safetyBlocked => '已拉黑，对方不会再出现';

  @override
  String get safetyBlocklist => '黑名单';

  @override
  String get safetyUnblock => '解除';

  @override
  String get safetyBlocklistEmpty => '还没有拉黑过任何人';

  @override
  String get momentTitle => '动态';

  @override
  String get momentGift => '送礼';

  @override
  String get momentTabRecommend => '推荐';

  @override
  String get momentTabFollowing => '关注';

  @override
  String get momentTabCity => '同城';

  @override
  String get momentDetailTitle => '动态';

  @override
  String get momentFollow => '关注';

  @override
  String get momentFollowing => '已关注';

  @override
  String momentCommentsHeader(int n) {
    return '$n 条评论';
  }

  @override
  String get momentCommentHint => '写评论…';

  @override
  String momentGiftComment(String gift) {
    return '送出「$gift」';
  }

  @override
  String get momentPostTitle => '发一条动态';

  @override
  String get momentPostHint => '此刻在想什么…';

  @override
  String get momentPost => '发布';

  @override
  String get momentWhoCanSee => '谁可以看';

  @override
  String get momentVisPublic => '公开';

  @override
  String get momentVisSelf => '仅自己';

  @override
  String get momentPosted => '动态已发布';

  @override
  String get mediaChooseTitle => '选择图片';

  @override
  String mediaSelectedOf(int n, int max) {
    return '已选 $n / $max';
  }

  @override
  String mediaCompressing(int percent) {
    return '压缩中 $percent%';
  }

  @override
  String get mediaCompressingSimple => '压缩中…';

  @override
  String mediaUploadingPercent(int percent) {
    return '上传中 $percent%';
  }

  @override
  String mediaOriginalSize(String size) {
    return '原图 $size';
  }

  @override
  String mediaCompressedSize(String size) {
    return '压缩后 $size';
  }

  @override
  String mediaUploadingOf(int done, int total) {
    return '上传中 $done / $total';
  }

  @override
  String mediaRetryFailed(int n) {
    return '$n 张失败，重试';
  }

  @override
  String get notifyTitle => '通知';

  @override
  String get notifyMarkAllRead => '全部已读';

  @override
  String get notifyEnablePush => '开启推送';

  @override
  String get notifyEnablePushHint => '有人回你的瓶子时第一时间知道';

  @override
  String get notifyEnable => '开启';

  @override
  String get notifyNewReply => '你的瓶子收到新回信';

  @override
  String notifyGiftReceived(String name) {
    return '$name 送了你一份礼物';
  }

  @override
  String notifyMomentLiked(String name) {
    return '$name 赞了你的动态';
  }

  @override
  String get notifyEmpty => '还没有新通知';

  @override
  String meIdLine(String id, String gender, int age) {
    return 'ID $id · $gender $age';
  }

  @override
  String get meCoinBalance => '金币余额';

  @override
  String get meRecharge => '充值';

  @override
  String get meCheckin => '签到';

  @override
  String get meItems => '道具';

  @override
  String get itemsHint => '道具用金币买，买来放进背包，送礼时消耗';

  @override
  String get itemsRecords => '道具记录';

  @override
  String get itemsRecordsEmpty => '还没有记录';

  @override
  String get itemsRecordBuy => '买入背包';

  @override
  String itemsRecordSent(String name) {
    return '送给 $name';
  }

  @override
  String itemsRecordReceived(String name) {
    return '来自 $name';
  }

  @override
  String get itemsEmptyTitle => '还没有可买的道具';

  @override
  String get itemsBuy => '买 1 个';

  @override
  String itemsBuyBody(int n) {
    return '花 $n 金币买 1 个放进背包，送礼时从背包里取。';
  }

  @override
  String itemsBought(String name) {
    return '已放进背包：$name';
  }

  @override
  String itemsOwned(int n) {
    return '持有 $n';
  }

  @override
  String get itemsNotOwned => '还没有';

  @override
  String get meWatchVideo => '看视频';

  @override
  String get meRelations => '关系';

  @override
  String get meWalletTxns => '钱包流水';

  @override
  String get meMyBottles => '我的瓶子';

  @override
  String get meStatBottles => '瓶子';

  @override
  String get meStatFollowing => '关注';

  @override
  String get meStatFollowers => '粉丝';

  @override
  String meBottlesCount(int n) {
    return '$n 个';
  }

  @override
  String get meSettings => '设置';

  @override
  String get meLocation => '定位';

  @override
  String get meLocationHint => '关闭后不再获取和上报位置，距离按城市显示';

  @override
  String get meLanguage => '语言';

  @override
  String get meLanguageFollowSystem => '跟随系统';

  @override
  String get meTheme => '外观';

  @override
  String get meThemeFollowSystem => '跟随系统';

  @override
  String get meThemeLight => '浅色';

  @override
  String get meThemeDark => '深色';

  @override
  String get meExportLogs => '导出日志';

  @override
  String get meExportLogsHint => '遇到问题时发给客服，帮助定位';

  @override
  String get meExportLogsConfirm =>
      '将导出最近 3 天的运行记录（最多 5 MB）。\n\n包含：访问过的接口、错误信息、页面跳转。\n不包含：密码、验证码、登录凭证。';

  @override
  String get meExportLogsBodyWarn => '提示：当前开启了详细日志，导出内容还包含请求内容。';

  @override
  String get meExportLogsGo => '导出并分享';

  @override
  String get meAbout => '关于';

  @override
  String get meContact => '联系我们';

  @override
  String get meLogout => '退出登录';

  @override
  String get meLogoutConfirm => '确定要退出登录吗？';

  @override
  String get meDeleteAccount => '注销账号';

  @override
  String get meDeleteAccountWarn => '注销后资料与瓶子会被清除，手机号可以重新注册。此操作不可撤销。';

  @override
  String get meDeleteAccountConfirm => '确认注销';

  @override
  String get walletTitle => '钱包';

  @override
  String get walletGoRecharge => '去充值';

  @override
  String get walletCurrentBalance => '当前金币余额';

  @override
  String get walletTotalRecharged => '累计充值';

  @override
  String get walletTotalSpent => '累计消费';

  @override
  String get walletTabIncome => '收入';

  @override
  String get walletTabExpense => '支出';

  @override
  String get walletSceneUnlock => '解锁回信';

  @override
  String get walletSceneChat => '开启聊天';

  @override
  String get walletSceneMsg => '发送消息';

  @override
  String get walletSceneGift => '送出礼物';

  @override
  String get walletSceneRecharge => '充值';

  @override
  String get walletSceneReward => '新人奖励';

  @override
  String get walletSceneCheckin => '每日签到';

  @override
  String get walletSceneShare => '分享奖励';

  @override
  String get walletSceneAdReward => '看激励视频';

  @override
  String get payWaitingTitle => '支付处理中';

  @override
  String get payWaitingBody => '交易还在进行，请不要重复支付';

  @override
  String payWaitingElapsed(int sec) {
    return '已等待 $sec 秒';
  }

  @override
  String get payVerifyingTitle => '正在核实票据';

  @override
  String get payVerifyingBody => '正在向 App Store 核实这笔交易';

  @override
  String get payDoneButton => '我已完成支付';

  @override
  String get payTroubleButton => '遇到问题';

  @override
  String get paySuccessTitle => '充值成功';

  @override
  String paySuccessCoins(int coins) {
    return '$coins 金币已到账';
  }

  @override
  String payBalanceNow(int coins) {
    return '当前余额 $coins';
  }

  @override
  String get payBackToScene => '回到刚才的页面';

  @override
  String get payViewRecords => '查看充值记录';

  @override
  String get payDroppedTitle => '已扣款未到账';

  @override
  String get payDroppedBody => '钱不会丢。若 24 小时内仍未到账，请联系客服，我们会人工补单';

  @override
  String get payRecheckButton => '主动查单';

  @override
  String get payContactSupport => '联系客服';

  @override
  String get payFailedTitle => '支付未完成';

  @override
  String get payFailedBody => '这笔交易没有成功，你没有被扣款';

  @override
  String get payBackToRecharge => '重新选择档位';

  @override
  String get payLeaveConfirm => '交易可能仍在进行，确定离开？';

  @override
  String get payLeaveStay => '继续等待';

  @override
  String get payLeaveGo => '离开';

  @override
  String get payLeftRunning => '交易仍在进行，可在充值记录里查看';

  @override
  String get payMockTitle => '模拟支付（联调）';

  @override
  String get payMockSuccess => '模拟支付成功';

  @override
  String get payMockFail => '模拟支付失败';

  @override
  String get payMockNoResponse => '不返回结果（模拟掉单）';

  @override
  String get payRecordsTitle => '充值记录';

  @override
  String get payRestore => '恢复购买';

  @override
  String payRestoreDone(int n) {
    return '$n 笔订单已更新';
  }

  @override
  String get payRestoreNone => '没有需要更新的订单';

  @override
  String get payStatusPaid => '已到账';

  @override
  String get payStatusPending => '处理中';

  @override
  String get payStatusFailed => '支付失败';

  @override
  String get payStatusRefunded => '已退款';

  @override
  String get payRecordsEmpty => '还没有充值记录';

  @override
  String get payRecordsEmptyHint => '去充值';

  @override
  String payCoinsAmount(int coins) {
    return '$coins 金币';
  }

  @override
  String get voidedTitle => '有一笔充值被退款了';

  @override
  String get voidedBody => '渠道已退还这笔充值，对应的金币已从账户扣回。由于其中一部分已经消费，余额出现负数';

  @override
  String get voidedFrozen => '余额为负时，扔瓶 / 解锁 / 送礼暂停，充值仍然可用。补足后自动恢复';

  @override
  String get voidedRelated => '相关流水';

  @override
  String get voidedGoRecharge => '去充值';

  @override
  String get voidedNotMe => '这不是我操作的';

  @override
  String get voidedBalance => '当前金币余额';

  @override
  String voidedTotals(int recharged, int spent) {
    return '累计充值 $recharged · 累计消费 $spent';
  }

  @override
  String get walletNegativeBanner => '余额为负，点此了解原因';

  @override
  String get walletSceneRefund => '充值退款';

  @override
  String get walletSceneRewind => '撤回';

  @override
  String get walletSceneDiscoverSkip => '左滑跳过';

  @override
  String get walletEmpty => '还没有任何流水';

  @override
  String get legalNotReadyTitle => '正文还在准备中';

  @override
  String get legalNotReadyBody => '这份文本还没在后台配置，可以先在浏览器里看网页版。';

  @override
  String get legalOpenInBrowser => '在浏览器中打开';

  @override
  String get rechargeTitle => '充值金币';

  @override
  String get rechargeChoosePackage => '选择档位';

  @override
  String rechargeBonus(int percent) {
    return '多送 $percent%';
  }

  @override
  String get rechargePaymentMethod => '支付方式';

  @override
  String get rechargeUpiSub => 'GPay · PhonePe · Paytm';

  @override
  String get rechargeCardSub => 'Card / Netbanking';

  @override
  String get rechargeIosChannel => 'App Store 付款';

  @override
  String get rechargeIosHint => '数字商品由 App Store 收款，实际价格以商店展示为准';

  @override
  String rechargePayAndGet(String price, int coins) {
    return '支付 $price · 得 $coins 金币';
  }

  @override
  String get giftPanelTitle => '送出礼物';

  @override
  String get giftSend => '送出';

  @override
  String giftQty(int n) {
    return '×$n';
  }

  @override
  String giftFromBag(int n) {
    return '背包抵扣 ×$n';
  }

  @override
  String get giftNeedChatFirst => '先和 TA 打个招呼，开聊后就能送礼';

  @override
  String giftSentToast(String gift) {
    return '已送出「$gift」';
  }

  @override
  String get giftOutOfStockTitle => '礼物数量不足';

  @override
  String get giftGoBuy => '去道具页购买';

  @override
  String quotaUsedUpTitle(int n) {
    return '今天的 $n 次捞完了';
  }

  @override
  String get quotaUsedUpBody => '明天 00:00 自动恢复。\n想现在继续，有两个办法。';

  @override
  String quotaPackTitle(int n) {
    return '捞瓶次数包 ×$n';
  }

  @override
  String get quotaPackHint => '立即到账，不过期';

  @override
  String quotaBuyWithCoins(int n) {
    return '用 $n 金币购买';
  }

  @override
  String get quotaWatchAdForOne => '看段视频，免费 +1 次';

  @override
  String quotaThrowUsedUpTitle(int n) {
    return '今天的 $n 次都扔完了';
  }

  @override
  String get rewardTitle => '每日奖励';

  @override
  String rewardStreakBadge(int n) {
    return '已连签 $n 天';
  }

  @override
  String rewardStreakTitle(int n) {
    return '连续签到 $n 天';
  }

  @override
  String get rewardDay7 => '第 7 天有大奖';

  @override
  String rewardCheckinGet(int n) {
    return '签到领 $n 金币';
  }

  @override
  String get rewardCheckedIn => '今天已签到';

  @override
  String get rewardEarnMore => '还能这样赚';

  @override
  String get rewardWatchAd => '看激励视频';

  @override
  String rewardWatchAdHint(int done, int total, int coins) {
    return '今天 $done / $total 次 · 每次 +$coins';
  }

  @override
  String get rewardInvite => '邀请好友';

  @override
  String rewardInviteHint(int n) {
    return '好友注册后，双方各得 $n';
  }

  @override
  String get rewardPostMoment => '发一条动态';

  @override
  String rewardPostMomentHint(int n) {
    return '每天第一条 +$n';
  }

  @override
  String get rewardGoWatch => '去看';

  @override
  String get rewardInviteAction => '邀请';

  @override
  String get rewardGoPost => '去发';

  @override
  String rewardGot(int n) {
    return '到账 +$n 金币';
  }

  @override
  String get relationTitle => '我的关系';

  @override
  String relationTabLikedMe(int n) {
    return '喜欢我的 $n';
  }

  @override
  String relationTabILiked(int n) {
    return '我喜欢的 $n';
  }

  @override
  String get relationTabViewedMe => '看过我';

  @override
  String relationInteractions(int n) {
    return '互动 $n 次';
  }

  @override
  String get relationStageStranger => '陌生';

  @override
  String get relationStageKnown => '认识';

  @override
  String get relationStageFamiliar => '熟悉';

  @override
  String get relationStrengthHint => '强度分由「互动次数 × 时间衰减」算出，30 天不互动会回落';

  @override
  String get stateEmptyOceanTitle => '海面很安静';

  @override
  String get stateEmptyOceanBody => '换个语言筛选，或者自己先扔一个\n通常十几分钟就会有人捞到';

  @override
  String get stateEmptyOceanCta => '扔一个瓶子';

  @override
  String get stateEmptyChatsTitle => '还没有人和你说话';

  @override
  String get stateEmptyChatsBody => '回一个你捞到的瓶子，\n或者去发现页打个招呼';

  @override
  String get stateEmptyChatsCta => '去捞一个瓶子';

  @override
  String get stateEmptyChatsAlt => '看看谁在线';

  @override
  String get stateEmptyMomentsTitle => '这里还很空';

  @override
  String get stateEmptyMomentsBody => '发第一条动态，让别人找到你';

  @override
  String get stateEmptyMomentsCta => '发一条动态';

  @override
  String get stateEmptyBottlesTitle => '还没扔过瓶子';

  @override
  String get stateEmptyBottlesBody => '写下第一句话，看看它能漂多远';

  @override
  String get stateEmptyScoopedTitle => '还没捞过瓶子';

  @override
  String get stateEmptyScoopedBody => '去海面捞一个，遇见陌生的心情';

  @override
  String get stateEmptyCollectedTitle => '还没有收藏';

  @override
  String get stateEmptyCollectedBody => '在瓶子详情点 ♡ 收藏，方便以后回来看';

  @override
  String get bottleTraceShort => '轨迹';

  @override
  String stateInsufficientTitle(int n) {
    return '还差 $n 金币';
  }

  @override
  String stateInsufficientBody(String item, int need, int have) {
    return '「$item」需要 $need，你现在有 $have。';
  }

  @override
  String get stateInsufficientCta => '去充值';

  @override
  String get stateInsufficientAlt => '换一个便宜的礼物';

  @override
  String get stateOfflineTitle => '网络好像断了';

  @override
  String get stateOfflineBody => '检查一下 Wi-Fi 或数据网络，\n刚才的内容还在，不会丢';

  @override
  String get stateOfflineCta => '重新加载';

  @override
  String get stateErrorTitle => '出了点问题';

  @override
  String get stateErrorBody => '服务器没有回应，稍后再试一次';

  @override
  String get stateLoadFailed => '加载失败，下拉重试';

  @override
  String authOauthNav(String provider) {
    return '$provider 登录';
  }

  @override
  String authOauthPendingTitle(String provider) {
    return '正在校验 $provider 账号';
  }

  @override
  String get authOauthPendingBody => '正在与服务器确认身份';

  @override
  String get authConflictTitle => '这个邮箱已经有账号了';

  @override
  String get authConflictBody => '为了账号安全，不会自动合并';

  @override
  String get authConflictHint =>
      '请先用密码登录这个账号，再到「我的 → 账号与安全」里绑定。绑定之后，下次就能直接用第三方账号登录了。';

  @override
  String get authUsePassword => '用密码登录';

  @override
  String authSwitchAccount(String provider) {
    return '换一个 $provider 账号';
  }

  @override
  String get locationIntroTitle => '让瓶子知道你在哪儿';

  @override
  String get locationIntroLead => '开启后可以：';

  @override
  String get locationReasonDistance => '看到瓶子漂了多远';

  @override
  String get locationReasonDistanceSub => '「来自 2.4 km 外」';

  @override
  String get locationReasonNearby => '优先推荐附近的人';

  @override
  String get locationReasonNearbySub => '距离只是排序维度之一';

  @override
  String get locationReasonPlace => '发瓶子时自动带上地点';

  @override
  String get locationReasonPlaceSub => '每次都可以改或不带';

  @override
  String get locationIntroPrivacy => '别人只能看到城市和大致距离，看不到你的具体位置。随时可以在设置里关掉。';

  @override
  String get locationEnable => '开启定位';

  @override
  String get locationLater => '暂不开启';

  @override
  String get commonGotIt => '知道了';

  @override
  String get accountSecurityTitle => '账号与安全';

  @override
  String get accountLoginMethods => '登录方式';

  @override
  String get accountSecuritySection => '安全';

  @override
  String get accountPhone => '手机号';

  @override
  String get accountEmail => '邮箱';

  @override
  String get accountBound => '已绑定';

  @override
  String get accountNotBound => '未绑定';

  @override
  String get accountBind => '绑定';

  @override
  String get accountUnbind => '解绑';

  @override
  String get accountAppleIosOnly => '仅 iOS 可绑定';

  @override
  String get accountChangePassword => '修改密码';

  @override
  String get accountDelete => '删除账号';

  @override
  String get accountKeepOneHint => '至少保留一种可登录方式。只剩一种时，那一项的解绑入口置灰。';

  @override
  String get accountBoundTitle => '已绑定';

  @override
  String get accountBoundBody => '下次可以直接用它登录了。';

  @override
  String accountTakenTitle(String provider) {
    return '这个 $provider 账号已被占用';
  }

  @override
  String accountTakenBody(String provider) {
    return '它已经绑在另一个 Drift 账号上。一个 $provider 账号只能绑一个 Drift 账号。';
  }

  @override
  String get accountTakenHint => '如果那个账号也是你的，先用它登录后解绑，再回到这里绑定。';

  @override
  String get accountBindFailed => '绑定失败，请稍后再试';

  @override
  String get accountDeleteTitle => '删除账号';

  @override
  String get accountDeleteBody => '删除后资料与瓶子会被清除，手机号可以重新注册。此操作不可撤销。';

  @override
  String get accountDeleteConfirm => '确认删除';

  @override
  String get bottlePlaceSection => '地点';

  @override
  String get placeUseCurrent => '使用当前位置';

  @override
  String get placeUseCurrentSub => '也可以在地图上换一个';

  @override
  String get placeAdd => '添加地点';

  @override
  String get placeAddSub => '这条不会带位置';

  @override
  String get mapPickTitle => '选择地点';

  @override
  String get mapDragHint => '拖动地图来移动大头针';

  @override
  String get mapLocating => '定位中…';

  @override
  String get mapUseThis => '用这个地点';

  @override
  String get mapHidePlace => '不显示地点';

  @override
  String get locationDeniedTitle => '定位被拒绝了';

  @override
  String get locationDeniedBody => '「附近」需要位置权限。可以到系统设置里重新打开，也可以先看推荐。';

  @override
  String get locationOpenSettings => '去设置开启';

  @override
  String get locationBrowseInstead => '先逛逛推荐';

  @override
  String get sparkGoChat => '去聊聊';

  @override
  String get sparkLater => '以后再说';

  @override
  String get sparkExpired => '这次匹配已经过期了';
}
