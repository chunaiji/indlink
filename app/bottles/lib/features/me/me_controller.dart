import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/providers.dart';
import '../../domain/models/engagement.dart';
import '../../domain/models/wallet.dart';

final walletTxnsProvider = FutureProvider<List<WalletTxn>>((ref) {
  // 余额变动后 invalidate 这个 provider 即可让流水页跟着刷新。
  ref.watch(walletProvider);
  return ref.watch(walletRepoProvider).txns();
});

final packagesProvider = FutureProvider<List<RechargePackage>>((ref) {
  return ref.watch(walletRepoProvider).packages();
});

final relationsProvider = FutureProvider.family<List<RelationItem>, String>((
  ref,
  tab,
) {
  return ref.watch(walletRepoProvider).relations(tab);
});

/// 签到与奖励。
///
/// `CheckinLog` 的唯一索引 `(user_id, date)` 保证一人一天一次，资金可对账。
final checkinProvider = AsyncNotifierProvider<CheckinNotifier, CheckinStatus>(
  CheckinNotifier.new,
);

class CheckinNotifier extends AsyncNotifier<CheckinStatus> {
  @override
  Future<CheckinStatus> build() =>
      ref.watch(walletRepoProvider).checkinStatus();

  /// 返回到账金币数，0 表示今天已签过。
  Future<int> checkin() async {
    final coins = await ref.read(walletRepoProvider).checkin();
    state = AsyncData(await ref.read(walletRepoProvider).checkinStatus());
    await ref.read(walletProvider.notifier).refresh();
    return coins;
  }

  /// 激励视频。App 端广告源要换 AdMob，这里只负责入账与状态刷新。
  Future<int> adReward() async {
    final coins = await ref.read(walletRepoProvider).adReward();
    state = AsyncData(await ref.read(walletRepoProvider).checkinStatus());
    await ref.read(walletProvider.notifier).refresh();
    return coins;
  }
}

final notificationsProvider =
    AsyncNotifierProvider<NotificationsNotifier, List<NotificationItem>>(
      NotificationsNotifier.new,
    );

class NotificationsNotifier extends AsyncNotifier<List<NotificationItem>> {
  @override
  Future<List<NotificationItem>> build() =>
      ref.watch(notifyRepoProvider).list();

  Future<void> markAllRead() async {
    await ref.read(notifyRepoProvider).markAllRead();
    state = AsyncData([
      for (final n in state.value ?? const <NotificationItem>[]) n.markRead(),
    ]);
  }
}
