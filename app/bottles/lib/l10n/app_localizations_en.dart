// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class LEn extends L {
  LEn([String locale = 'en']) : super(locale);

  @override
  String get appName => 'DRIFT';

  @override
  String get appTagline => 'One bottle. It could wash up anywhere.';

  @override
  String get tabOcean => 'Ocean';

  @override
  String get tabDiscover => 'Discover';

  @override
  String get tabChats => 'Chats';

  @override
  String get tabMoments => 'Moments';

  @override
  String get tabMe => 'Me';

  @override
  String get commonConfirm => 'Confirm';

  @override
  String get commonCancel => 'Cancel';

  @override
  String get commonClose => 'Close';

  @override
  String get commonDone => 'Done';

  @override
  String get commonRetry => 'Retry';

  @override
  String get commonReset => 'Reset';

  @override
  String get commonAll => 'All';

  @override
  String get commonOr => 'or';

  @override
  String get commonOnline => 'Online';

  @override
  String commonActiveAgo(String time) {
    return 'Active $time';
  }

  @override
  String get commonFemale => 'Female';

  @override
  String get commonMale => 'Male';

  @override
  String get commonSecret => 'Private';

  @override
  String commonAgeValue(int age) {
    return '$age';
  }

  @override
  String get commonAnonymous => 'Anonymous';

  @override
  String commonDriftedDays(int days) {
    return 'Drifted $days days';
  }

  @override
  String get commonJustNow => 'Just now';

  @override
  String commonMinutesAgo(int n) {
    return '$n min ago';
  }

  @override
  String commonHoursAgo(int n) {
    return '$n h ago';
  }

  @override
  String get commonYesterday => 'Yesterday';

  @override
  String commonDaysAgo(int n) {
    return '$n d ago';
  }

  @override
  String get commonLastWeek => 'Last week';

  @override
  String get authLoginTitle => 'Welcome back';

  @override
  String get authRegisterTitle => 'Create account';

  @override
  String get authResetTitle => 'Reset password';

  @override
  String get authPasswordHint => 'Password';

  @override
  String get authRememberPassword => 'Remember password';

  @override
  String get authNewPasswordHint => 'New password';

  @override
  String get authPasswordTooShort => 'At least 8 characters';

  @override
  String get authSignIn => 'Sign in';

  @override
  String get authSignUp => 'Sign up';

  @override
  String get authForgotPassword => 'Forgot password?';

  @override
  String get authNoAccount => 'No account yet?';

  @override
  String get authHasAccount => 'Already have an account?';

  @override
  String get authUseOtpInstead => 'Sign in with a code';

  @override
  String get authUsePasswordInstead => 'Sign in with password';

  @override
  String get authRegisterNext => 'Next';

  @override
  String get authRegisterDone => 'Create and start';

  @override
  String get authPasswordUpdated => 'Password updated';

  @override
  String get authResetDone => 'Reset and sign in';

  @override
  String get authSetPasswordHint => 'You\'ll use this to sign in from now on';

  @override
  String get authStartWithPhone => 'Start with your phone';

  @override
  String get authStartWithEmail => 'Start with your email';

  @override
  String get authTabPhone => 'Phone';

  @override
  String get authTabEmail => 'Email';

  @override
  String get authTabPassword => 'Password';

  @override
  String get authTabCode => 'Code';

  @override
  String get authPhoneHint => 'Phone number';

  @override
  String get authEmailHint => 'you@example.com';

  @override
  String get authSendCode => 'Send code';

  @override
  String get authContinueWithGoogle => 'Continue with Google';

  @override
  String get authContinueWithApple => 'Continue with Apple';

  @override
  String get authAgreementPrefix => 'By continuing you agree to the';

  @override
  String get authAgreeTitle => 'Please accept the terms first';

  @override
  String get authAgreeBody => 'To continue, you need to accept:';

  @override
  String get authAgreeAndContinue => 'Agree and continue';

  @override
  String get authTerms => 'Terms of Service';

  @override
  String get authAnd => 'and';

  @override
  String get authPrivacy => 'Privacy Policy';

  @override
  String get privacyGateTitle => 'Terms & Privacy Policy';

  @override
  String get privacyGateBody =>
      'Before you sign in, please read and agree to the terms. We only collect necessary data with your consent (login credentials; location to suggest people nearby, which you can decline), and you can manage or withdraw it anytime in Settings.';

  @override
  String get privacyGateAgree => 'Agree & continue';

  @override
  String get privacyGateExit => 'Decline';

  @override
  String get authVerifyPhone => 'Verify phone';

  @override
  String get authVerifyEmail => 'Verify email';

  @override
  String get authEnterCode => 'Enter the code';

  @override
  String get authSentTo => 'Sent to';

  @override
  String get authChangeNumber => 'Change';

  @override
  String get authChangeEmail => 'Change';

  @override
  String authResendIn(int seconds) {
    return 'Resend in ${seconds}s';
  }

  @override
  String get authResend => 'Resend code';

  @override
  String get authOtpWarning =>
      'Didn\'t get it? Check your spam filter. One SMS per number every 60s; 5 in a row triggers rate limiting.';

  @override
  String get authOtpWarningEmail =>
      'Didn\'t get it? Check your spam folder first. One email per address every 60s; 5 in a row triggers rate limiting.';

  @override
  String get authVerifyAndLogin => 'Verify and sign in';

  @override
  String get authInvalidPhone => 'Enter a valid phone number';

  @override
  String get authInvalidCode => 'That code isn\'t right';

  @override
  String get profileCompleteTitle => 'Your profile';

  @override
  String profileStepOf(int current, int total) {
    return 'Step $current of $total';
  }

  @override
  String get profileUploadAvatar => 'Add a photo';

  @override
  String get profileUploadAvatarHint =>
      'People with photos get far more replies';

  @override
  String get profileNicknameHint => 'Name';

  @override
  String get profileGenderLockTitle => 'Gender can\'t be changed later';

  @override
  String profileGenderLockBody(String gender) {
    return 'You picked $gender. To keep matching fair it can\'t be changed after this. Continue?';
  }

  @override
  String get profileGenderRethink => 'Let me think';

  @override
  String get profileEditTitle => 'Edit profile';

  @override
  String get profileEditAvatarHint => 'Tap to change your photo';

  @override
  String get profileEditNickname => 'Nickname';

  @override
  String get profileEditNicknameRequired => 'Nickname can\'t be empty';

  @override
  String get profileEditBirthday => 'Date of birth';

  @override
  String get profileEditBirthdayHint => 'Pick your date of birth';

  @override
  String get profileEditBirthdayNote =>
      'Your age is worked out from it; the date itself is only visible to you.';

  @override
  String get profileEditSave => 'Save';

  @override
  String get profileEditSaved => 'Profile updated';

  @override
  String get profileGender => 'Gender';

  @override
  String get profileLanguages => 'Languages you speak';

  @override
  String get profileLanguagePick => 'Pick the languages you speak';

  @override
  String profileInterests(int n) {
    return 'Interests · pick at least $n';
  }

  @override
  String get profileEnableLocation => 'Enable location';

  @override
  String get profileEnableLocationHint =>
      'Used to suggest people nearby. Off anytime.';

  @override
  String get profileStartDrifting => 'Start drifting';

  @override
  String get profileNeedNickname => 'Pick a name first';

  @override
  String profileNeedInterests(int n) {
    return '$n more interests and you\'re ready';
  }

  @override
  String profileAgeRange(int min, int max) {
    return 'Age $min-$max';
  }

  @override
  String profileAgeInvalid(int min, int max) {
    return 'Must be $min-$max';
  }

  @override
  String get oceanTitle => 'Tonight\'s sea';

  @override
  String oceanSubtitle(int count, String time) {
    final intl.NumberFormat countNumberFormat = intl.NumberFormat.compact(
      locale: localeName,
    );
    final String countString = countNumberFormat.format(count);

    return '$countString replies drifting back · night opens $time';
  }

  @override
  String get oceanNightTitle => 'Night has opened';

  @override
  String oceanNightSubtitle(int hours, int minutes) {
    return 'Only late-night bottles adrift · ${hours}h ${minutes}m left';
  }

  @override
  String get oceanTagNight => '🌙 Night';

  @override
  String get oceanTagNightShort => '🌙 Late';

  @override
  String get oceanTagHollow => 'Vent';

  @override
  String get oceanPullToRefresh => 'Pull to raise the tide';

  @override
  String get oceanSeaReport => 'Today\'s sea report';

  @override
  String get oceanSkinDay => 'Switched to the daytime sea';

  @override
  String get oceanSkinNight => 'Switched to the night sea';

  @override
  String get oceanSkinAuto => 'Sea follows the time of day again';

  @override
  String oceanOnlineTonight(int n) {
    return '$n online tonight';
  }

  @override
  String get oceanThrowOne => 'Throw one';

  @override
  String oceanThrowRemain(int n) {
    return '$n left today';
  }

  @override
  String get oceanThrowUnlimited => 'Night bottles unlimited';

  @override
  String get oceanScoopOne => 'Scoop one';

  @override
  String oceanScoopRemain(int n) {
    return '$n left today';
  }

  @override
  String oceanTracePeek(int days, int cities) {
    return 'Drifted $days days · $cities cities';
  }

  @override
  String get oceanSomethingBit => 'Something\'s on the line';

  @override
  String get oceanReleaseToOpen => 'Let go to open it';

  @override
  String get oceanOpeningNow => 'Opening…';

  @override
  String get oceanScoopFailed =>
      'The wind picked up — the bottle sank back. No scoop used.';

  @override
  String get oceanPutBack => 'Put it back';

  @override
  String get oceanPutBackHint => 'They won\'t be disturbed';

  @override
  String get oceanWriteReply => 'Write back';

  @override
  String get oceanWriteReplyHint => 'They\'ll be notified';

  @override
  String oceanDriftedRecent(String city) {
    return 'Just drifted in · $city';
  }

  @override
  String oceanDriftedFromTo(int days, String city) {
    return 'Drifted $days days · $city → here';
  }

  @override
  String get oceanCastDone => 'Your bottle has drifted away';

  @override
  String get bottleWriteTitle => 'Write a bottle';

  @override
  String bottleThrowRemainToday(int n) {
    return '$n throws left today';
  }

  @override
  String get bottleContentHint => 'Write something, send it to sea…';

  @override
  String bottleCounter(int used, int max) {
    return '$used / $max';
  }

  @override
  String get bottleTags => 'Tags';

  @override
  String get bottleRange => 'Reach';

  @override
  String get bottleRangeNationwide => 'Everywhere';

  @override
  String get bottleRangeCity => 'My city';

  @override
  String get bottleCastToSea => 'Cast into the sea';

  @override
  String get bottleNeedContent => 'Write something first';

  @override
  String get bottleScoopedTitle => 'The bottle you found';

  @override
  String get bottleUnlockPill => 'Unlock';

  @override
  String get bottleUnlockOneTitle => 'Read this reply?';

  @override
  String bottleUnlockOneBody(int n) {
    return 'Spend $n coins to unlock this reply. It stays unlocked.';
  }

  @override
  String bottleUnlockOneConfirm(int n) {
    return 'Unlock for $n coins';
  }

  @override
  String get bottleUnlockedOne => 'Unlocked';

  @override
  String bottleRepliesHeader(int count, String time) {
    return '$count replies · last one $time';
  }

  @override
  String get bottleReplyHint => 'Write your reply…';

  @override
  String get bottleReplySent => 'Your reply is on its way';

  @override
  String get bottleCollected => 'Saved. Find it under My bottles.';

  @override
  String get bottleUncollected => 'Removed from saved';

  @override
  String get bottleMineTitle => 'My bottles';

  @override
  String get bottleTabThrown => 'Thrown';

  @override
  String get bottleTabScooped => 'Scooped';

  @override
  String get bottleTabCollected => 'Saved';

  @override
  String get bottleTraceTitle => 'Where this bottle has been';

  @override
  String get bottleTraceHere => 'Here · you scooped it';

  @override
  String bottleTraceSummary(int people, int cities) {
    return 'Seen by $people people across $cities cities';
  }

  @override
  String get bottleExpired => 'Expired';

  @override
  String bottleStatViews(int n) {
    return '$n';
  }

  @override
  String get bottleMapTitle => 'The route it drifted';

  @override
  String get bottleMapPosterTitle => 'A letter\'s journey';

  @override
  String bottleMapDeparted(String date, String city) {
    return 'Left $city on $date';
  }

  @override
  String get bottleMapStatPeople => 'people saw it';

  @override
  String get bottleMapStatCities => 'cities';

  @override
  String get bottleMapStatDays => 'days adrift';

  @override
  String get bottleMapStatKm => 'kilometres';

  @override
  String get bottleMapPrivacy =>
      'The poster shows only your words and cities — never anything about who replied';

  @override
  String get bottleMapSave => 'Save image';

  @override
  String get bottleMapShare => 'Share';

  @override
  String get bottleMapSaved => 'Saved to your photos';

  @override
  String bottleTraceThrown(String city) {
    return '$city · you cast it';
  }

  @override
  String bottleTraceSeen(String city, int n) {
    return '$city · seen by $n';
  }

  @override
  String bottleTraceReplied(String city, int n) {
    return '$city · $n replies';
  }

  @override
  String get discoverTitle => 'Online today';

  @override
  String get discoverTabRecommend => 'For you';

  @override
  String get discoverTabNearby => 'Nearby';

  @override
  String get discoverTabNew => 'New';

  @override
  String get discoverSayHi => 'Say hi';

  @override
  String get discoverViewProfile => 'Profile';

  @override
  String get discoverLike => 'Like';

  @override
  String get discoverPass => 'Pass';

  @override
  String get discoverRewind => 'Rewind';

  @override
  String get discoverRewindTitle => 'Bring them back?';

  @override
  String discoverRewindBody(int n) {
    return 'Spend $n coins to undo your last left swipe — they\'ll return to the top of your deck.';
  }

  @override
  String discoverRewindConfirm(int n) {
    return 'Rewind for $n coins';
  }

  @override
  String discoverRewindDone(String name) {
    return '$name is back';
  }

  @override
  String get discoverPassChargeTitle => 'Pass this person?';

  @override
  String discoverPassChargeBody(int n) {
    return 'A left swipe costs $n coins — they won\'t show up in your deck again.';
  }

  @override
  String discoverPassChargeConfirm(int n) {
    return 'Pass for $n coins';
  }

  @override
  String get discoverFiltersTitle => 'Filters';

  @override
  String get discoverLanguage => 'Language';

  @override
  String get discoverInterest => 'Interests';

  @override
  String get discoverQuizTitle => 'Quiz match';

  @override
  String get discoverQuizHint => 'People who pick the same answers rank higher';

  @override
  String get discoverDistance => 'Distance';

  @override
  String get discoverDistanceUnlimited => 'Any';

  @override
  String discoverDistanceKm(int n) {
    return '$n km';
  }

  @override
  String get discoverAgeRange => 'Age';

  @override
  String discoverAgeRangeValue(int min, int max) {
    return '$min – $max';
  }

  @override
  String discoverApplyFilter(int n) {
    return 'Apply · $n people match';
  }

  @override
  String get discoverNoMoreCards => 'That\'s everyone for today';

  @override
  String get discoverStatBottles => 'bottles';

  @override
  String get discoverStatMoments => 'moments';

  @override
  String get discoverStatRelation => 'closeness';

  @override
  String get discoverCharm => 'Charm';

  @override
  String profileCommonPoint(String value) {
    return 'In common · you both chose \"$value\"';
  }

  @override
  String get profileTheirMoments => 'Their moments';

  @override
  String profileMomentCount(int n) {
    return '$n total';
  }

  @override
  String get chatTitle => 'Chats';

  @override
  String get chatOneMessage => 'Sending a message';

  @override
  String chatPricingHint(int free, int price) {
    return 'First $free messages are free, then $price coins each';
  }

  @override
  String chatPricingHintNoFree(int price) {
    return '$price coins per message';
  }

  @override
  String chatTabUnread(int n) {
    return 'Unread $n';
  }

  @override
  String get chatTabBottleFriends => 'Bottle friends';

  @override
  String get chatAnonymousFriend => 'Anonymous';

  @override
  String get chatStartedFromBottle => 'This started with a bottle';

  @override
  String get chatRead => 'Read';

  @override
  String get chatImageMessage => '[Photo]';

  @override
  String chatGiftSentBy(String gift) {
    return 'sent a $gift';
  }

  @override
  String chatGiftReceived(String gift) {
    return 'They sent you a $gift';
  }

  @override
  String chatCharmPlus(int n) {
    return 'Charm +$n';
  }

  @override
  String get chatInputHint => 'Say something…';

  @override
  String get chatDeletedUser => 'Deleted account';

  @override
  String get chatDeletedUserHint =>
      'This person deleted their account. Read-only.';

  @override
  String get chatReconnecting => 'Reconnecting…';

  @override
  String safetyReportUser(String name) {
    return 'Report $name';
  }

  @override
  String get safetyReasonPorn => 'Sexual or explicit content';

  @override
  String get safetyReasonSpam => 'Spam, ads or scams';

  @override
  String get safetyReasonHarass => 'Harassment or abuse';

  @override
  String get safetyReasonMinor => 'Involves a minor';

  @override
  String get safetyBlockOnly => 'Just block';

  @override
  String get safetyReportAndBlock => 'Report and block';

  @override
  String get safetyReported => 'Report received — we\'ll look into it';

  @override
  String get safetyBlocked => 'Blocked. You won\'t see them again.';

  @override
  String get safetyBlocklist => 'Blocked';

  @override
  String get safetyUnblock => 'Unblock';

  @override
  String get safetyBlocklistEmpty => 'You haven\'t blocked anyone';

  @override
  String get momentTitle => 'Moments';

  @override
  String get momentGift => 'Gift';

  @override
  String get momentTabRecommend => 'For you';

  @override
  String get momentTabFollowing => 'Following';

  @override
  String get momentTabCity => 'Nearby';

  @override
  String get momentDetailTitle => 'Moment';

  @override
  String get momentFollow => 'Follow';

  @override
  String get momentFollowing => 'Following';

  @override
  String momentCommentsHeader(int n) {
    return '$n comments';
  }

  @override
  String get momentCommentHint => 'Add a comment…';

  @override
  String momentGiftComment(String gift) {
    return 'sent a $gift';
  }

  @override
  String get momentPostTitle => 'New moment';

  @override
  String get momentPostHint => 'What\'s on your mind…';

  @override
  String get momentPost => 'Post';

  @override
  String get momentWhoCanSee => 'Who can see';

  @override
  String get momentVisPublic => 'Public';

  @override
  String get momentVisSelf => 'Only me';

  @override
  String get momentPosted => 'Posted';

  @override
  String get mediaChooseTitle => 'Choose photos';

  @override
  String mediaSelectedOf(int n, int max) {
    return '$n / $max selected';
  }

  @override
  String mediaCompressing(int percent) {
    return 'Compressing $percent%';
  }

  @override
  String get mediaCompressingSimple => 'Compressing…';

  @override
  String mediaUploadingPercent(int percent) {
    return 'Uploading $percent%';
  }

  @override
  String mediaOriginalSize(String size) {
    return 'Original $size';
  }

  @override
  String mediaCompressedSize(String size) {
    return 'Compressed $size';
  }

  @override
  String mediaUploadingOf(int done, int total) {
    return 'Uploading $done / $total';
  }

  @override
  String mediaRetryFailed(int n) {
    return '$n failed — retry';
  }

  @override
  String get notifyTitle => 'Notifications';

  @override
  String get notifyMarkAllRead => 'Mark all read';

  @override
  String get notifyEnablePush => 'Turn on push';

  @override
  String get notifyEnablePushHint =>
      'Know the moment someone answers your bottle';

  @override
  String get notifyEnable => 'Turn on';

  @override
  String get notifyNewReply => 'Your bottle got a reply';

  @override
  String notifyGiftReceived(String name) {
    return '$name sent you a gift';
  }

  @override
  String notifyMomentLiked(String name) {
    return '$name liked your moment';
  }

  @override
  String get notifyEmpty => 'Nothing new yet';

  @override
  String meIdLine(String id, String gender, int age) {
    return 'ID $id · $gender $age';
  }

  @override
  String get meCoinBalance => 'Coin balance';

  @override
  String get meRecharge => 'Top up';

  @override
  String get meCheckin => 'Check in';

  @override
  String get meItems => 'Gifts';

  @override
  String get itemsHint =>
      'Buy gifts with coins. They sit in your bag until you send one.';

  @override
  String get itemsRecords => 'History';

  @override
  String get itemsRecordsEmpty => 'Nothing yet';

  @override
  String get itemsRecordBuy => 'Bought';

  @override
  String itemsRecordSent(String name) {
    return 'Sent to $name';
  }

  @override
  String itemsRecordReceived(String name) {
    return 'From $name';
  }

  @override
  String get itemsEmptyTitle => 'Nothing to buy yet';

  @override
  String get itemsBuy => 'Buy one';

  @override
  String itemsBuyBody(int n) {
    return 'Spend $n coins for one, kept in your bag until you send it.';
  }

  @override
  String itemsBought(String name) {
    return 'Added to your bag: $name';
  }

  @override
  String itemsOwned(int n) {
    return 'Have $n';
  }

  @override
  String get itemsNotOwned => 'None yet';

  @override
  String get meWatchVideo => 'Watch';

  @override
  String get meRelations => 'Closeness';

  @override
  String get meWalletTxns => 'Transactions';

  @override
  String get meMyBottles => 'My bottles';

  @override
  String get meStatBottles => 'Bottles';

  @override
  String get meStatFollowing => 'Following';

  @override
  String get meStatFollowers => 'Followers';

  @override
  String meBottlesCount(int n) {
    return '$n';
  }

  @override
  String get meSettings => 'Settings';

  @override
  String get meLocation => 'Location';

  @override
  String get meLocationHint =>
      'When off, your position is never read or uploaded; distances show by city';

  @override
  String get meLanguage => 'Language';

  @override
  String get meLanguageFollowSystem => 'System';

  @override
  String get meTheme => 'Appearance';

  @override
  String get meThemeFollowSystem => 'System';

  @override
  String get meThemeLight => 'Light';

  @override
  String get meThemeDark => 'Dark';

  @override
  String get meExportLogs => 'Export logs';

  @override
  String get meExportLogsHint =>
      'Send them to support to help us track down a problem';

  @override
  String get meExportLogsConfirm =>
      'This exports the last 3 days of activity (up to 5 MB).\n\nIncluded: the endpoints you reached, error messages, screen changes.\nNot included: passwords, verification codes, sign-in credentials.';

  @override
  String get meExportLogsBodyWarn =>
      'Note: detailed logging is currently on, so request contents are included too.';

  @override
  String get meExportLogsGo => 'Export and share';

  @override
  String get meAbout => 'About';

  @override
  String get meContact => 'Contact us';

  @override
  String get meLogout => 'Sign out';

  @override
  String get meLogoutConfirm => 'Sign out of this account?';

  @override
  String get meDeleteAccount => 'Delete account';

  @override
  String get meDeleteAccountWarn =>
      'Your profile and bottles will be erased and the phone number freed for re-registration. This cannot be undone.';

  @override
  String get meDeleteAccountConfirm => 'Delete permanently';

  @override
  String get walletTitle => 'Wallet';

  @override
  String get walletGoRecharge => 'Top up';

  @override
  String get walletCurrentBalance => 'Current balance';

  @override
  String get walletTotalRecharged => 'Topped up';

  @override
  String get walletTotalSpent => 'Spent';

  @override
  String get walletTabIncome => 'In';

  @override
  String get walletTabExpense => 'Out';

  @override
  String get walletSceneUnlock => 'Unlocked replies';

  @override
  String get walletSceneChat => 'Started a chat';

  @override
  String get walletSceneMsg => 'Sent a message';

  @override
  String get walletSceneGift => 'Sent a gift';

  @override
  String get walletSceneRecharge => 'Top-up';

  @override
  String get walletSceneReward => 'Welcome bonus';

  @override
  String get walletSceneCheckin => 'Daily check-in';

  @override
  String get walletSceneShare => 'Share reward';

  @override
  String get walletSceneAdReward => 'Rewarded video';

  @override
  String get payWaitingTitle => 'Payment in progress';

  @override
  String get payWaitingBody =>
      'This transaction is still running. Please do not pay again';

  @override
  String payWaitingElapsed(int sec) {
    return 'Waiting for ${sec}s';
  }

  @override
  String get payVerifyingTitle => 'Verifying your receipt';

  @override
  String get payVerifyingBody => 'Checking this transaction with the App Store';

  @override
  String get payDoneButton => 'I have paid';

  @override
  String get payTroubleButton => 'Something went wrong';

  @override
  String get paySuccessTitle => 'Payment complete';

  @override
  String paySuccessCoins(int coins) {
    return '$coins coins added';
  }

  @override
  String payBalanceNow(int coins) {
    return 'Balance $coins';
  }

  @override
  String get payBackToScene => 'Back to where you were';

  @override
  String get payViewRecords => 'View recharge history';

  @override
  String get payDroppedTitle => 'Charged but not credited';

  @override
  String get payDroppedBody =>
      'Your money is safe. If the coins have not arrived within 24 hours, contact support and we will credit it manually';

  @override
  String get payRecheckButton => 'Check again';

  @override
  String get payContactSupport => 'Contact support';

  @override
  String get payFailedTitle => 'Payment not completed';

  @override
  String get payFailedBody =>
      'This transaction did not go through. You were not charged';

  @override
  String get payBackToRecharge => 'Choose another pack';

  @override
  String get payLeaveConfirm =>
      'The transaction may still be running. Leave anyway?';

  @override
  String get payLeaveStay => 'Keep waiting';

  @override
  String get payLeaveGo => 'Leave';

  @override
  String get payLeftRunning =>
      'The transaction is still running. You can track it in your recharge history';

  @override
  String get payMockTitle => 'Mock payment (testing)';

  @override
  String get payMockSuccess => 'Simulate success';

  @override
  String get payMockFail => 'Simulate failure';

  @override
  String get payMockNoResponse => 'Return nothing (simulate a dropped order)';

  @override
  String get payRecordsTitle => 'Recharge history';

  @override
  String get payRestore => 'Restore purchases';

  @override
  String payRestoreDone(int n) {
    return '$n order(s) updated';
  }

  @override
  String get payRestoreNone => 'Nothing needed updating';

  @override
  String get payStatusPaid => 'Credited';

  @override
  String get payStatusPending => 'Processing';

  @override
  String get payStatusFailed => 'Failed';

  @override
  String get payStatusRefunded => 'Refunded';

  @override
  String get payRecordsEmpty => 'No recharges yet';

  @override
  String get payRecordsEmptyHint => 'Add coins';

  @override
  String payCoinsAmount(int coins) {
    return '$coins coins';
  }

  @override
  String get voidedTitle => 'One of your recharges was refunded';

  @override
  String get voidedBody =>
      'The store refunded this purchase and the matching coins were taken back. Part of them had already been spent, so the balance went negative';

  @override
  String get voidedFrozen =>
      'While the balance is negative, throwing bottles, unlocking and gifting are paused. Recharging still works, and everything resumes once the balance is back above zero';

  @override
  String get voidedRelated => 'Related transactions';

  @override
  String get voidedGoRecharge => 'Add coins';

  @override
  String get voidedNotMe => 'I did not request this';

  @override
  String get voidedBalance => 'Current coin balance';

  @override
  String voidedTotals(int recharged, int spent) {
    return 'Recharged $recharged · Spent $spent';
  }

  @override
  String get walletNegativeBanner =>
      'Your balance is negative. Tap to find out why';

  @override
  String get walletSceneRefund => 'Refund';

  @override
  String get walletSceneRewind => 'Rewind';

  @override
  String get walletSceneDiscoverSkip => 'Pass';

  @override
  String get walletEmpty => 'No transactions yet';

  @override
  String get legalNotReadyTitle => 'Not published yet';

  @override
  String get legalNotReadyBody =>
      'This document hasn\'t been configured yet. You can read the web version in your browser.';

  @override
  String get legalOpenInBrowser => 'Open in browser';

  @override
  String get rechargeTitle => 'Top up coins';

  @override
  String get rechargeChoosePackage => 'Choose a pack';

  @override
  String rechargeBonus(int percent) {
    return '+$percent% bonus';
  }

  @override
  String get rechargePaymentMethod => 'Payment method';

  @override
  String get rechargeUpiSub => 'GPay · PhonePe · Paytm';

  @override
  String get rechargeCardSub => 'Card / Netbanking';

  @override
  String get rechargeIosChannel => 'Pay with App Store';

  @override
  String get rechargeIosHint =>
      'Digital items are billed by the App Store; the price shown there is final';

  @override
  String rechargePayAndGet(String price, int coins) {
    return 'Pay $price · get $coins coins';
  }

  @override
  String get giftPanelTitle => 'Send a gift';

  @override
  String get giftSend => 'Send';

  @override
  String giftQty(int n) {
    return '×$n';
  }

  @override
  String giftFromBag(int n) {
    return '×$n from your bag';
  }

  @override
  String get giftNeedChatFirst => 'Say hi first — gifts are sent in chat';

  @override
  String giftSentToast(String gift) {
    return 'Sent a $gift';
  }

  @override
  String get giftOutOfStockTitle => 'Not enough gifts';

  @override
  String get giftGoBuy => 'Go buy gifts';

  @override
  String quotaUsedUpTitle(int n) {
    return 'All $n scoops used';
  }

  @override
  String get quotaUsedUpBody =>
      'They reset at midnight.\nTwo ways to keep going now.';

  @override
  String quotaPackTitle(int n) {
    return 'Scoop pack ×$n';
  }

  @override
  String get quotaPackHint => 'Instant, never expires';

  @override
  String quotaBuyWithCoins(int n) {
    return 'Buy for $n coins';
  }

  @override
  String get quotaWatchAdForOne => 'Watch a video for +1 free';

  @override
  String quotaThrowUsedUpTitle(int n) {
    return 'All $n throws used';
  }

  @override
  String get rewardTitle => 'Daily rewards';

  @override
  String rewardStreakBadge(int n) {
    return '$n-day streak';
  }

  @override
  String rewardStreakTitle(int n) {
    return '$n days in a row';
  }

  @override
  String get rewardDay7 => 'Big one on day 7';

  @override
  String rewardCheckinGet(int n) {
    return 'Check in for $n coins';
  }

  @override
  String get rewardCheckedIn => 'Checked in today';

  @override
  String get rewardEarnMore => 'Other ways to earn';

  @override
  String get rewardWatchAd => 'Rewarded video';

  @override
  String rewardWatchAdHint(int done, int total, int coins) {
    return '$done / $total today · +$coins each';
  }

  @override
  String get rewardInvite => 'Invite a friend';

  @override
  String rewardInviteHint(int n) {
    return 'You both get $n when they sign up';
  }

  @override
  String get rewardPostMoment => 'Post a moment';

  @override
  String rewardPostMomentHint(int n) {
    return '+$n for your first each day';
  }

  @override
  String get rewardGoWatch => 'Watch';

  @override
  String get rewardInviteAction => 'Invite';

  @override
  String get rewardGoPost => 'Post';

  @override
  String rewardGot(int n) {
    return '+$n coins';
  }

  @override
  String get relationTitle => 'Closeness';

  @override
  String relationTabLikedMe(int n) {
    return 'Liked me $n';
  }

  @override
  String relationTabILiked(int n) {
    return 'I liked $n';
  }

  @override
  String get relationTabViewedMe => 'Viewed me';

  @override
  String relationInteractions(int n) {
    return '$n interactions';
  }

  @override
  String get relationStageStranger => 'Stranger';

  @override
  String get relationStageKnown => 'Acquainted';

  @override
  String get relationStageFamiliar => 'Close';

  @override
  String get relationStrengthHint =>
      'Strength = interactions × time decay. It falls back after 30 quiet days.';

  @override
  String get stateEmptyOceanTitle => 'The sea is quiet';

  @override
  String get stateEmptyOceanBody =>
      'Try another language filter, or throw one yourself —\nsomeone usually finds it within minutes';

  @override
  String get stateEmptyOceanCta => 'Throw a bottle';

  @override
  String get stateEmptyChatsTitle => 'Nobody\'s talking to you yet';

  @override
  String get stateEmptyChatsBody =>
      'Answer a bottle you found,\nor go say hi on Discover';

  @override
  String get stateEmptyChatsCta => 'Go scoop a bottle';

  @override
  String get stateEmptyChatsAlt => 'See who\'s online';

  @override
  String get stateEmptyMomentsTitle => 'Nothing here yet';

  @override
  String get stateEmptyMomentsBody =>
      'Post the first moment and let people find you';

  @override
  String get stateEmptyMomentsCta => 'Post a moment';

  @override
  String get stateEmptyBottlesTitle => 'No bottles yet';

  @override
  String get stateEmptyBottlesBody =>
      'Write your first line and see how far it drifts';

  @override
  String get stateEmptyScoopedTitle => 'No bottles scooped yet';

  @override
  String get stateEmptyScoopedBody =>
      'Scoop one from the sea and meet a stranger\'s mood';

  @override
  String get stateEmptyCollectedTitle => 'Nothing saved yet';

  @override
  String get stateEmptyCollectedBody =>
      'Tap the heart on a bottle to keep it here';

  @override
  String get bottleTraceShort => 'Trail';

  @override
  String stateInsufficientTitle(int n) {
    return '$n coins short';
  }

  @override
  String stateInsufficientBody(String item, int need, int have) {
    return '$item costs $need. You have $have.';
  }

  @override
  String get stateInsufficientCta => 'Top up';

  @override
  String get stateInsufficientAlt => 'Pick a cheaper gift';

  @override
  String get stateOfflineTitle => 'You\'re offline';

  @override
  String get stateOfflineBody =>
      'Check Wi-Fi or mobile data.\nNothing you wrote was lost.';

  @override
  String get stateOfflineCta => 'Reload';

  @override
  String get stateErrorTitle => 'Something went wrong';

  @override
  String get stateErrorBody =>
      'The server didn\'t answer. Try again in a moment.';

  @override
  String get stateLoadFailed => 'Couldn\'t load — pull to retry';

  @override
  String authOauthNav(String provider) {
    return '$provider sign-in';
  }

  @override
  String authOauthPendingTitle(String provider) {
    return 'Verifying your $provider account';
  }

  @override
  String get authOauthPendingBody => 'Confirming your identity with the server';

  @override
  String get authConflictTitle => 'This email already has an account';

  @override
  String get authConflictBody =>
      'For your safety we won\'t merge them automatically';

  @override
  String get authConflictHint =>
      'Sign in to that account with your password first, then link it under Me → Account & security. After that you can sign in with this provider directly.';

  @override
  String get authUsePassword => 'Sign in with password';

  @override
  String authSwitchAccount(String provider) {
    return 'Use another $provider account';
  }

  @override
  String get locationIntroTitle => 'Let your bottles know where you are';

  @override
  String get locationIntroLead => 'With location on you can:';

  @override
  String get locationReasonDistance => 'See how far a bottle drifted';

  @override
  String get locationReasonDistanceSub => '\"From 2.4 km away\"';

  @override
  String get locationReasonNearby => 'Meet people nearby first';

  @override
  String get locationReasonNearbySub =>
      'Distance is only one of the ranking signals';

  @override
  String get locationReasonPlace => 'Tag a place when you cast a bottle';

  @override
  String get locationReasonPlaceSub => 'You can change or drop it every time';

  @override
  String get locationIntroPrivacy =>
      'Others only see your city and a rough distance, never your exact location. You can turn it off in Settings anytime.';

  @override
  String get locationEnable => 'Turn on location';

  @override
  String get locationLater => 'Not now';

  @override
  String get commonGotIt => 'Got it';

  @override
  String get accountSecurityTitle => 'Account & security';

  @override
  String get accountLoginMethods => 'Sign-in methods';

  @override
  String get accountSecuritySection => 'Security';

  @override
  String get accountPhone => 'Phone';

  @override
  String get accountEmail => 'Email';

  @override
  String get accountBound => 'Linked';

  @override
  String get accountNotBound => 'Not linked';

  @override
  String get accountBind => 'Link';

  @override
  String get accountUnbind => 'Unlink';

  @override
  String get accountAppleIosOnly => 'iOS only';

  @override
  String get accountChangePassword => 'Change password';

  @override
  String get accountDelete => 'Delete account';

  @override
  String get accountKeepOneHint =>
      'Keep at least one way to sign in. When only one is left, its unlink action is disabled.';

  @override
  String get accountBoundTitle => 'Linked';

  @override
  String get accountBoundBody => 'You can sign in with it directly next time.';

  @override
  String accountTakenTitle(String provider) {
    return 'This $provider account is already in use';
  }

  @override
  String accountTakenBody(String provider) {
    return 'It is linked to another Drift account. A $provider account can only be linked to one Drift account.';
  }

  @override
  String get accountTakenHint =>
      'If that account is also yours, sign in with it, unlink there, then come back and link here.';

  @override
  String get accountBindFailed => 'Linking failed, please try again later';

  @override
  String get accountDeleteTitle => 'Delete account';

  @override
  String get accountDeleteBody =>
      'Your profile and bottles will be erased. The phone number can register again. This cannot be undone.';

  @override
  String get accountDeleteConfirm => 'Delete';

  @override
  String get bottlePlaceSection => 'Place';

  @override
  String get placeUseCurrent => 'Use current location';

  @override
  String get placeUseCurrentSub => 'You can also pick one on the map';

  @override
  String get placeAdd => 'Add a place';

  @override
  String get placeAddSub => 'This bottle won\'t carry a location';

  @override
  String get mapPickTitle => 'Pick a place';

  @override
  String get mapDragHint => 'Drag the map to move the pin';

  @override
  String get mapLocating => 'Locating…';

  @override
  String get mapUseThis => 'Use this place';

  @override
  String get mapHidePlace => 'Don\'t show a place';

  @override
  String get locationDeniedTitle => 'Location access is off';

  @override
  String get locationDeniedBody =>
      '\"Nearby\" needs your location. Turn it on in Settings, or browse recommendations first.';

  @override
  String get locationOpenSettings => 'Open Settings';

  @override
  String get locationBrowseInstead => 'Browse recommendations';

  @override
  String get sparkGoChat => 'Say hi';

  @override
  String get sparkLater => 'Maybe later';

  @override
  String get sparkExpired => 'This match has expired';
}
