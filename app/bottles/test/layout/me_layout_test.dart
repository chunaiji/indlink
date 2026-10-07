import 'package:bottles/features/me/account_security_page.dart';
import 'package:bottles/features/me/me_page.dart';
import 'package:bottles/features/me/notifications_page.dart';
import 'package:bottles/features/me/pay_records_page.dart';
import 'package:bottles/features/me/recharge_page.dart';
import 'package:bottles/features/me/rewards_page.dart';
import 'package:bottles/features/me/voided_page.dart';
import 'package:bottles/features/me/wallet_page.dart';
import 'package:flutter_test/flutter_test.dart';

import 'matrix.dart';

/// 「我的」系页面在六档机型上都不允许 overflow（规范 §8.2）。
void main() {
  for (final cfg in layoutMatrix) {
    testWidgets('wallet fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const WalletPage()));
    testWidgets('pay records fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const PayRecordsPage()));
    testWidgets('voided fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const VoidedPage()));
    testWidgets('recharge fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const RechargePage()));
    testWidgets('rewards fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const RewardsPage()));
    testWidgets('account security fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const AccountSecurityPage()));
    testWidgets('me fits: ${cfg.name}', (t) => pumpAt(t, cfg, const MePage()));
    testWidgets('notifications fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const NotificationsPage()));
  }
}
