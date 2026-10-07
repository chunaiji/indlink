import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/providers.dart';
import '../../domain/models/bottle.dart';

/// 当前海面筛选（全部 / 夜场 / 语言）。
final oceanTagProvider = NotifierProvider<OceanTagNotifier, String>(
  OceanTagNotifier.new,
);

class OceanTagNotifier extends Notifier<String> {
  @override
  String build() => 'all';

  void set(String tag) => state = tag;
}

/// 抛瓶信号。
///
/// 写瓶子页在**另一个路由**里提交，动画却要在首页播——用一个自增计数器把两者解耦：
/// 首页 listen 到它变化就起动画，不需要 B2 知道首页的存在。
final castSignalProvider = NotifierProvider<CastSignalNotifier, int>(
  CastSignalNotifier.new,
);

class CastSignalNotifier extends Notifier<int> {
  @override
  int build() => 0;

  void fire() => state = state + 1;
}

class OceanData {
  const OceanData({required this.condition, required this.peek});

  final SeaCondition condition;

  /// 海面上漂着的瓶子，点它们不消耗次数。
  final List<Bottle> peek;
}

final oceanProvider = AsyncNotifierProvider<OceanNotifier, OceanData>(
  OceanNotifier.new,
);

class OceanNotifier extends AsyncNotifier<OceanData> {
  @override
  Future<OceanData> build() async {
    final tag = ref.watch(oceanTagProvider);
    final repo = ref.watch(bottleRepoProvider);
    final count = ref.watch(appConfigProvider).oceanBottleCount;
    // 启动即并发拉，避免首屏二次布局抖动。
    final results = await Future.wait([
      repo.seaCondition(),
      repo.oceanPeek(tag: tag == 'all' ? null : tag, limit: count),
    ]);
    return OceanData(
      condition: results[0] as SeaCondition,
      peek: results[1] as List<Bottle>,
    );
  }

  /// 涨潮刷新。刷新中不清空旧数据，否则用户会以为内容丢了。
  Future<void> refreshTide() async {
    final tag = ref.read(oceanTagProvider);
    final repo = ref.read(bottleRepoProvider);
    final count = ref.read(appConfigProvider).oceanBottleCount;
    final results = await Future.wait([
      repo.seaCondition(),
      repo.oceanPeek(tag: tag == 'all' ? null : tag, limit: count),
    ]);
    state = AsyncData(
      OceanData(
        condition: results[0] as SeaCondition,
        peek: results[1] as List<Bottle>,
      ),
    );
    await ref.read(quotaProvider.notifier).refresh();
  }
}

/// 「我的瓶子」三个分页。
final myBottlesProvider = FutureProvider.family<List<Bottle>, BottleMineTab>((
  ref,
  tab,
) {
  return ref.watch(bottleRepoProvider).mine(tab);
});

/// 瓶子详情与它的回信。解锁后 invalidate 这个 provider 即可整屏刷新。
final bottleDetailProvider = FutureProvider.family<Bottle, String>((ref, id) {
  return ref.watch(bottleRepoProvider).detail(id);
});

final bottleRepliesProvider = FutureProvider.family<List<BottleReply>, String>((
  ref,
  id,
) {
  return ref.watch(bottleRepoProvider).replies(id);
});

final bottleTraceProvider =
    FutureProvider.family<List<BottleTraceNode>, String>((ref, id) {
      return ref.watch(bottleRepoProvider).trace(id);
    });
