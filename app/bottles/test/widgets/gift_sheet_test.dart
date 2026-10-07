import 'package:bottles/core/providers.dart';
import 'package:bottles/data/mock/mock_repositories.dart';
import 'package:bottles/domain/models/chat.dart';
import 'package:bottles/domain/models/user.dart';
import 'package:bottles/ui/flows/gift_flows.dart';
import 'package:bottles/ui/widgets/buttons.dart';
import 'package:bottles/ui/widgets/overlays.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// 背包里有一个 g1，钱包 0 币。
class _BagWallet extends MockWalletRepository {
  _BagWallet(super.db);

  @override
  Future<Map<String, int>> myItems() async => {'g1': 1};

  @override
  Future<Wallet> wallet() async =>
      const Wallet(coins: 0, totalRecharged: 0, totalSpent: 0);
}

class _Host extends ConsumerStatefulWidget {
  const _Host();

  @override
  ConsumerState<_Host> createState() => _HostState();
}

class _HostState extends ConsumerState<_Host> {
  (GiftItem, int)? result;

  @override
  Widget build(BuildContext context) => Scaffold(
    body: Center(
      child: TextButton(
        onPressed: () async => result = await showGiftSheet(context, ref),
        child: const Text('open'),
      ),
    ),
  );
}

/// H4：手里有 ×1 就不该再为这一个标价、算余额——背包先抵扣，差额才扣币。
/// 数量下拉框至少 84 宽，不是个圆。
void main() {
  testWidgets('a gift already in the bag sends for free', (t) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const _Host(),
      skipDefaults: {walletRepoProvider},
      overrides: [
        walletRepoProvider.overrideWith(
          (ref) => _BagWallet(ref.watch(mockBackendProvider)),
        ),
      ],
    );
    await t.tap(find.text('open'));
    await t.pumpAndSettle();

    expect(
      find.descendant(of: find.byType(AppButton), matching: find.text('10')),
      findsNothing,
      reason: 'the send button must not price a gift that comes from the bag',
    );
    expect(find.text('×1 from your bag'), findsOneWidget);
    expect(
      t.getSize(find.byKey(const ValueKey('gift-qty-picker'))).width,
      greaterThanOrEqualTo(84),
    );

    await t.tap(find.text('Send'));
    await t.pumpAndSettle();
    expect(
      find.byType(ModalCard),
      findsNothing,
      reason: 'no insufficient modal',
    );
    final host = t.state<_HostState>(find.byType(_Host));
    expect(host.result?.$1.id, 'g1');
    expect(host.result?.$2, 1);
  });
}
