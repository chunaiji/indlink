import 'package:bottles/core/design/theme.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/data/mock/mock_backend.dart';
import 'package:bottles/data/mock/mock_repositories.dart';
import 'package:bottles/domain/models/wallet.dart';
import 'package:bottles/domain/models/user.dart';
import 'package:bottles/features/me/pay_records_page.dart';
import 'package:bottles/features/me/payment_flow_page.dart';
import 'package:bottles/features/me/voided_page.dart';
import 'package:bottles/features/me/wallet_page.dart';
import 'package:bottles/l10n/app_localizations.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

/// 支付状态机的三条路径:成功 → H8、失败 → 失败态、不返回结果 → 超时 → H9。
///
/// 掉单那条用虚拟时钟推进 31 秒,不要真等——测试跑得慢,人就不跑测试了。
void main() {
  late MockBackend db;
  late MockWalletRepository repo;

  setUp(() {
    db = MockBackend();
    repo = MockWalletRepository(db);
  });

  /// 造一笔已下单的 pending 订单,连同它的 PendingOrder。
  PendingOrder seedOrder() {
    final pkg = db.packages.first;
    final d = db.createOrder(pkg);
    return PendingOrder(
      orderNo: d['order_no'] as String,
      payParams: (d['pay_params'] as Map).cast<String, dynamic>(),
    );
  }

  Widget host(PendingOrder order) => ProviderScope(
        overrides: [walletRepoProvider.overrideWithValue(repo)],
        child: MaterialApp(
          // context.c 取的是 AppColors 这个 ThemeExtension，
          // 不给应用主题，所有用了语义色的组件都会在 build 里炸。
          theme: AppTheme.light(),
          locale: const Locale('zh'),
          localizationsDelegates: const [
            L.delegate,
            GlobalMaterialLocalizations.delegate,
            GlobalWidgetsLocalizations.delegate,
            GlobalCupertinoLocalizations.delegate,
          ],
          supportedLocales: L.supportedLocales,
          // pending 传 null:走 resume 分支,不弹联调面板。
          // 面板是人机交互,这里要测的是状态机对服务端状态的反应。
          home: PaymentFlowPage(orderNo: order.orderNo),
        ),
      );

  testWidgets('a settled order lands on the success screen', (tester) async {
    final order = seedOrder();
    db.settleMock(order.orderNo, true);

    await tester.pumpWidget(host(order));
    await tester.pump(); // postFrame → resume()
    await tester.pump(const Duration(milliseconds: 400)); // mock 仓储的延迟
    await tester.pump();

    expect(find.text('充值成功'), findsOneWidget);
  });

  testWidgets('a failed order lands on the failure screen', (tester) async {
    final order = seedOrder();
    db.settleMock(order.orderNo, false);

    await tester.pumpWidget(host(order));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump();

    expect(find.text('支付未完成'), findsOneWidget);
  });

  testWidgets('an order nobody settles times out into the dropped screen',
      (tester) async {
    final order = seedOrder(); // 故意不结算:这就是掉单现场

    await tester.pumpWidget(host(order));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    // 还在等,不该提前认输。
    expect(find.text('已扣款未到账'), findsNothing);

    // 推过 30 秒的超时线。轮询是 1/2/4/8 秒退避,逐段推进让定时器真的跑起来。
    for (var i = 0; i < 12; i++) {
      await tester.pump(const Duration(seconds: 4));
    }

    expect(find.text('已扣款未到账'), findsOneWidget);
  });

  group('H10 recharge history', () {
    Widget recordsHost() => ProviderScope(
          overrides: [walletRepoProvider.overrideWithValue(repo)],
          child: MaterialApp(
            theme: AppTheme.light(),
            locale: const Locale('zh'),
            localizationsDelegates: const [
              L.delegate,
              GlobalMaterialLocalizations.delegate,
              GlobalWidgetsLocalizations.delegate,
              GlobalCupertinoLocalizations.delegate,
            ],
            supportedLocales: L.supportedLocales,
            home: const PayRecordsPage(),
          ),
        );

    testWidgets('each order status carries its own label', (tester) async {
      final paid = seedOrder();
      db.settleMock(paid.orderNo, true);
      final failed = seedOrder();
      db.settleMock(failed.orderNo, false);
      seedOrder(); // 留一笔 pending

      await tester.pumpWidget(recordsHost());
      await tester.pumpAndSettle();

      expect(find.text('已到账'), findsOneWidget);
      expect(find.text('支付失败'), findsOneWidget);
      expect(find.text('处理中'), findsOneWidget);
    });

    testWidgets('no orders shows the empty state', (tester) async {
      await tester.pumpWidget(recordsHost());
      await tester.pumpAndSettle();

      expect(find.text('还没有充值记录'), findsOneWidget);
    });
  });

  group('H11 voided refund', () {
    /// 默认 800x600 的测试画布比真机矮，钱包页在它下面本来就会溢出 4px
    /// （与本轮改动无关：无横幅的用例同样溢出）。给一个手机尺寸，
    /// 免得布局警告盖掉真正的断言失败。
    Future<void> phoneSurface(WidgetTester tester) async {
      await tester.binding.setSurfaceSize(const Size(420, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));
    }

    Widget pageHost(Widget page) => ProviderScope(
          overrides: [walletRepoProvider.overrideWithValue(repo)],
          child: MaterialApp(
            theme: AppTheme.light(),
            locale: const Locale('zh'),
            localizationsDelegates: const [
              L.delegate,
              GlobalMaterialLocalizations.delegate,
              GlobalWidgetsLocalizations.delegate,
              GlobalCupertinoLocalizations.delegate,
            ],
            supportedLocales: L.supportedLocales,
            home: page,
          ),
        );

    /// 把钱包压成负数，并留下一条退款流水——这正是 H11 要解释的现场。
    void seedVoided() {
      db.wallet = const Wallet(coins: -12, totalRecharged: 300, totalSpent: 312);
      db.txns.insert(
        0,
        WalletTxn(
          id: 'refund-1',
          scene: TxnScene.refund,
          amount: -300,
          at: DateTime.now().toUtc(),
          balanceAfter: -12,
        ),
      );
    }

    testWidgets('the wallet warns about a negative balance', (tester) async {
      await phoneSurface(tester);
      seedVoided();

      await tester.pumpWidget(pageHost(const WalletPage()));
      await tester.pumpAndSettle();

      expect(find.text('余额为负，点此了解原因'), findsOneWidget);
    });

    testWidgets('a healthy balance shows no banner', (tester) async {
      await phoneSurface(tester);
      await tester.pumpWidget(pageHost(const WalletPage()));
      await tester.pumpAndSettle();

      expect(find.text('余额为负，点此了解原因'), findsNothing);
    });

    // 这条同时是 TxnScene.refund 那个 bug 的端到端回归：
    // 修之前 parse 会把 refund 落进 reward，这里会显示成「奖励」。
    testWidgets('a refund reads as a refund, never as a reward',
        (tester) async {
      await phoneSurface(tester);
      seedVoided();

      await tester.pumpWidget(pageHost(const VoidedPage()));
      // H11 没有加载动画，没有任何东西在排帧，pumpAndSettle 会在第一帧就返回，
      // 假仓储那 160ms 的延迟根本没机会触发。显式推进时钟。
      await tester.pump(const Duration(milliseconds: 500));
      await tester.pump();

      expect(find.text('-12'), findsOneWidget);
      expect(find.text('充值退款'), findsWidgets);
      expect(find.text('奖励'), findsNothing);
    });
  });
}
