import 'package:bottles/core/config/remote_config.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('内置默认值', () {
    // 与服务端 TestAppConfigDefaultsAreInert 是同一条契约的两端：
    // 「没配过」必须等于「App 原来的样子」。任何一边把默认值改成非空文案
    // 或 false，所有 App 租户会在发版当天集体改样。
    test('文案全空、开关全开', () {
      const c = AppRemoteConfig.builtin();
      expect(c.anonSender, '');
      expect(c.oceanTitle, '');
      expect(c.quotaTitle, '');
      expect(c.quotaBody, '');
      expect(c.supportText, '');
      expect(c.supportImage, '');
      expect(c.showWallet, isTrue);
      expect(c.showRecharge, isTrue);
      expect(c.showWalletLog, isTrue);
      expect(c.showItems, isTrue);
      expect(c.showBlocklist, isTrue);
      // 聊天扣费：与服务端默认一致——开聊 5、按条 0、免费 0。
      expect(c.chatStartPrice, 5);
      expect(c.chatMsgPrice, 0);
      expect(c.chatFreeMsgs, 0);
      expect(c.loginPhone, isTrue);
      expect(c.loginEmail, isTrue);
      // 发现页定价：与服务端默认一致——撤回 10、左滑不扣币。
      expect(c.rewindPrice, 10);
      expect(c.skipChargeEnabled, isFalse);
    });
  });

  group('登录标识开关', () {
    test('关一个只剩另一个，切换隐藏', () {
      final c = AppRemoteConfig.fromJson({
        'auth': {'phone': true, 'email': '0'},
      });
      expect(c.phoneLoginOn, isTrue);
      expect(c.emailLoginOn, isFalse);
      expect(c.loginChannelsBoth, isFalse);
    });

    test('两个都关等于都开（不能把人锁在门外）', () {
      final c = AppRemoteConfig.fromJson({
        'auth': {'phone': false, 'email': false},
      });
      expect(c.phoneLoginOn, isTrue);
      expect(c.emailLoginOn, isTrue);
      expect(c.loginChannelsBoth, isTrue);
    });
  });

  group('pricing', () {
    test('读 pricing 段，数字字符串也认', () {
      final c = AppRemoteConfig.fromJson({
        'pricing': {'chat_start': 3, 'chat_msg': '1', 'chat_free_msgs': 5},
      });
      expect(c.chatStartPrice, 3);
      expect(c.chatMsgPrice, 1);
      expect(c.chatFreeMsgs, 5);
    });

    test('缺字段或负数退回内置值，并能原样缓存', () {
      final c = AppRemoteConfig.fromJson({
        'pricing': {'chat_msg': -1},
      });
      expect(c.chatStartPrice, 5);
      expect(c.chatMsgPrice, 0);
      final back = AppRemoteConfig.fromJson(c.toJson());
      expect(back.chatStartPrice, 5);
    });
  });

  group('or', () {
    test('空串与纯空白都算没配过', () {
      expect(AppRemoteConfig.or('', '内置'), '内置');
      expect(AppRemoteConfig.or('   ', '内置'), '内置');
      expect(AppRemoteConfig.or('\n', '内置'), '内置');
    });

    test('配了就用配的，首尾空格照原样保留', () {
      expect(AppRemoteConfig.or('今晚的海', '内置'), '今晚的海');
      expect(AppRemoteConfig.or(' 海 ', '内置'), ' 海 ');
    });
  });

  group('quotaTitleOr', () {
    const cfg = AppRemoteConfig.builtin();

    test('没配走内置文案', () {
      expect(cfg.quotaTitleOr('今天的 5 次捞完了', 5), '今天的 5 次捞完了');
    });

    test('配了就替换 {n}', () {
      final c = AppRemoteConfig.fromJson({
        'copy': {'quota_title': '今天的 {n} 次用光啦'},
      });
      expect(c.quotaTitleOr('内置', 8), '今天的 8 次用光啦');
    });

    test('不写 {n} 也合法', () {
      final c = AppRemoteConfig.fromJson({
        'copy': {'quota_title': '次数用完了'},
      });
      expect(c.quotaTitleOr('内置', 8), '次数用完了');
    });
  });

  group('fromJson 容错', () {
    // 后台填错一个字不该让 App 起不来 —— 这批解析全部走回退，不抛。
    test('空对象退回内置值', () {
      final c = AppRemoteConfig.fromJson({});
      expect(c.oceanTitle, '');
      expect(c.showRecharge, isTrue);
    });

    test('字段类型不对也不抛', () {
      final c = AppRemoteConfig.fromJson({
        'support': 'not an object',
        'mine': 42,
        'copy': {'ocean_title': 123},
      });
      expect(c.supportText, '');
      expect(c.showWallet, isTrue);
      expect(c.oceanTitle, '');
    });

    test('开关认 bool 也认 sysconfig 的 "0"/"1" 字符串', () {
      final c = AppRemoteConfig.fromJson({
        'mine': {
          'wallet': false,
          'recharge': '0',
          'items': '1',
          'blocklist': 1,
        },
      });
      expect(c.showWallet, isFalse);
      expect(c.showRecharge, isFalse);
      expect(c.showItems, isTrue);
      expect(c.showBlocklist, isTrue);
    });

    test('只给一半字段，另一半走默认', () {
      final c = AppRemoteConfig.fromJson({
        'mine': {'recharge': false},
      });
      expect(c.showRecharge, isFalse);
      expect(c.showWallet, isTrue);
    });
  });

  test('toJson → fromJson 原样往返（Prefs 缓存靠它）', () {
    const original = AppRemoteConfig(
      supportText: '客服 QQ 12345',
      supportImage: 'https://cdn/qr.png',
      showWallet: true,
      showRecharge: false,
      showPrivacyGate: false,
      showWalletLog: true,
      showItems: false,
      showBlocklist: true,
      anonSender: '海上的朋友',
      googleClientId: 'web.apps.googleusercontent.com',
      wechatLogin: false,
      alipayLogin: false,
      googleLogin: true,
      appleLogin: true,
      oceanTitle: '今晚的海',
      quotaTitle: '今天的 {n} 次捞完了',
      quotaBody: '明天恢复',
      chatStartPrice: 3,
      chatMsgPrice: 1,
      chatFreeMsgs: 5,
      loginPhone: true,
      loginEmail: false,
      rewindPrice: 7,
      skipChargeEnabled: true,
      skipPrice: 2,
    );
    final back = AppRemoteConfig.fromJson(original.toJson());

    expect(back.supportText, original.supportText);
    expect(back.supportImage, original.supportImage);
    expect(back.showRecharge, original.showRecharge);
    expect(back.showItems, original.showItems);
    expect(back.showWallet, original.showWallet);
    expect(back.chatStartPrice, 3);
    expect(back.chatMsgPrice, 1);
    expect(back.chatFreeMsgs, 5);
    expect(back.loginEmail, isFalse);
    expect(back.rewindPrice, 7);
    expect(back.skipChargeEnabled, isTrue);
    expect(back.skipPrice, 2);
    expect(back.anonSender, original.anonSender);
    expect(back.oceanTitle, original.oceanTitle);
    expect(back.quotaTitle, original.quotaTitle);
    expect(back.quotaBody, original.quotaBody);
  });

  group('Google 登录的 serverClientId', () {
    // Android 上 google_sign_in 只有拿到 serverClientId 才会回 idToken；
    // 项目里没有 google-services.json，所以没有 default_web_client_id 可读。
    // 这个值只能靠 /app-config 下发 —— 没有它，登录请求根本发不出去，
    // 服务端日志里连一条记录都不会有（2026-09-22 的线上故障就是这样）。
    test('从 auth.google_client_id 读', () {
      final c = AppRemoteConfig.fromJson({
        'auth': {'google_client_id': 'web.apps.googleusercontent.com'},
      });
      expect(c.googleClientId, 'web.apps.googleusercontent.com');
    });

    test('没下发时为空串，由调用方决定怎么降级', () {
      expect(const AppRemoteConfig.builtin().googleClientId, '');
      expect(AppRemoteConfig.fromJson({}).googleClientId, '');
      expect(AppRemoteConfig.fromJson({'auth': {}}).googleClientId, '');
    });

    test('类型不对也不抛，退回空串', () {
      final c = AppRemoteConfig.fromJson({
        'auth': {'google_client_id': 123},
      });
      expect(c.googleClientId, '');
    });
  });

  group('SSO switches', () {
    test('parses the four sso switches', () {
      final c = AppRemoteConfig.fromJson({
        'auth': {
          'wechat': false,
          'alipay': false,
          'google': true,
          'apple': true,
        },
      });
      expect(c.googleLogin, isTrue);
      expect(c.appleLogin, isTrue);
      expect(c.wechatLogin, isFalse);
    });

    // 老服务端不下发这四个键时按内置值走:国际版内置 Google 与 Apple 都开,
    // 行为与改动前(无条件显示)一致,升级服务端之前不会丢按钮。
    test('keeps the old behaviour when the server omits them', () {
      final c = AppRemoteConfig.fromJson(const {'auth': {}});
      expect(c.googleLogin, isTrue);
      expect(c.appleLogin, isTrue);
    });
  });
}
