import 'package:bottles/core/providers.dart';
import 'package:bottles/data/mock/mock_repositories.dart';
import 'package:bottles/domain/models/user.dart';
import 'package:bottles/ui/flows/gift_flows.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class _CountingWallet extends MockWalletRepository {
  _CountingWallet(super.db);

  int walletCalls = 0;
  int itemCalls = 0;

  @override
  Future<Wallet> wallet() {
    walletCalls++;
    return super.wallet();
  }

  @override
  Future<Map<String, int>> myItems() {
    itemCalls++;
    return super.myItems();
  }
}

/// 钱包 / 背包是每个账号自己的：登录态换人后必须重拉，
/// 否则刚注册的人看到余额 0（登录前那次），礼物面板上是上一个账号的库存。
void main() {
  test('wallet and bag refetch when the signed-in user changes', () async {
    late _CountingWallet repo;
    final container = ProviderContainer(
      overrides: [
        walletRepoProvider.overrideWith((ref) {
          repo = _CountingWallet(ref.watch(mockBackendProvider));
          return repo;
        }),
      ],
    );
    addTearDown(container.dispose);

    await container.read(walletProvider.future);
    await container.read(myItemsProvider.future);
    expect(repo.walletCalls, 1);
    expect(repo.itemCalls, 1);

    container
        .read(authProvider.notifier)
        .finishOnboarding(
          const UserProfile(
            brief: UserBrief(id: 'new', nickname: 'New'),
          ),
        );
    await container.read(walletProvider.future);
    await container.read(myItemsProvider.future);
    expect(repo.walletCalls, 2);
    expect(repo.itemCalls, 2);
  });
}
