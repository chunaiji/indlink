import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/providers.dart';
import '../../data/repositories.dart';
import '../../domain/models/engagement.dart';
import '../../domain/models/user.dart';

/// 筛选条件本地持久化（`shared_preferences`），随请求下发。
final discoverFilterProvider =
    NotifierProvider<DiscoverFilterNotifier, DiscoverFilter>(
      DiscoverFilterNotifier.new,
    );

class DiscoverFilterNotifier extends Notifier<DiscoverFilter> {
  @override
  DiscoverFilter build() {
    final raw = ref.watch(prefsProvider).discoverFilter;
    if (raw == null) return const DiscoverFilter();
    try {
      return DiscoverFilter.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    } catch (_) {
      return const DiscoverFilter();
    }
  }

  Future<void> update(DiscoverFilter next) async {
    state = next;
    await ref.read(prefsProvider).setDiscoverFilter(jsonEncode(next.toJson()));
  }

  Future<void> reset() => update(const DiscoverFilter());
}

final discoverTabProvider = NotifierProvider<DiscoverTabNotifier, String>(
  DiscoverTabNotifier.new,
);

class DiscoverTabNotifier extends Notifier<String> {
  @override
  String build() => 'recommend';

  void set(String v) => state = v;
}

/// 滑卡队列。
///
/// 一次拉 15~20 人缓存在本地，剩 5 张时静默续拉——**不可一张一请求**：
/// 滑动是连续手势，等接口会直接卡住手感。
final deckProvider = AsyncNotifierProvider<DeckNotifier, List<UserProfile>>(
  DeckNotifier.new,
);

class DeckNotifier extends AsyncNotifier<List<UserProfile>> {
  static const _refillThreshold = 5;

  bool _refilling = false;

  @override
  Future<List<UserProfile>> build() {
    final filter = ref.watch(discoverFilterProvider);
    final tab = ref.watch(discoverTabProvider);
    return ref.watch(discoverRepoProvider).users(filter: filter, tab: tab);
  }

  /// 右滑 = like，左滑 = skip。**两个动作都必须落库**——
  /// 否则下次拉人去重不掉，同一个人会反复推上来。
  Future<void> consumeTop({required bool liked}) async {
    final list = state.value;
    if (list == null || list.isEmpty) return;
    final top = list.first;

    state = AsyncData(list.sublist(1));

    // 落库失败**不能把卡片吞掉**：卡已经飞出去了，这里再抛异常只会让
    // _refill 不执行，牌堆最终空掉而界面没有任何解释。
    // 记一笔日志 + 继续续牌，比整个发现页停摆好。
    final repo = ref.read(discoverRepoProvider);
    try {
      liked ? await repo.like(top.id) : await repo.pass(top.id);
    } catch (e) {
      debugPrint('[discover] 记录${liked ? "喜欢" : "跳过"}失败 id=${top.id}: $e');
    }

    if ((state.value?.length ?? 0) <= _refillThreshold) await _refill();
  }

  /// 点爱心：只记「喜欢」，不切人——左右滑才切（真机反馈：点心不该把人划走）。
  /// 落库失败只记日志：心已经亮了，再收回去比重复喜欢一次更糟。
  Future<void> likeTop() async {
    final top = state.value?.firstOrNull;
    if (top == null) return;
    ref.read(likedIdsProvider.notifier).add(top.id);
    try {
      await ref.read(discoverRepoProvider).like(top.id);
    } catch (e) {
      debugPrint('[discover] 点赞失败 id=${top.id}: $e');
    }
  }

  /// 左滑扣费跳过：二次确认通过后调用。扣费在服务端 `POST /discover/skip` 里完成。
  ///
  /// 扣费失败（余额被并发花掉 / 网络错）→ 把人放回牌堆顶并 rethrow，
  /// 交由 UI 弹充值或 toast——不能让人被划走了钱却没扣、界面也没解释。
  Future<void> paidSkipTop() async {
    final list = state.value;
    if (list == null || list.isEmpty) return;
    final top = list.first;

    state = AsyncData(list.sublist(1));
    try {
      await ref.read(discoverRepoProvider).pass(top.id);
    } catch (_) {
      state = AsyncData([top, ...(state.value ?? const <UserProfile>[])]);
      rethrow;
    }

    if ((state.value?.length ?? 0) <= _refillThreshold) await _refill();
  }

  /// 撤回上一次左滑：扣币把人放回牌堆顶。
  ///
  /// 先落库再改本地——扣费失败（余额不足 / 没人可撤）时牌堆不能动，
  /// 否则用户看见人回来了钱却没扣，下次刷新又消失。
  Future<UserProfile> rewind() async {
    final user = await ref.read(discoverRepoProvider).rewind();
    final rest = (state.value ?? const <UserProfile>[]).where(
      (u) => u.id != user.id,
    );
    state = AsyncData([user, ...rest]);
    return user;
  }

  Future<void> _refill() async {
    if (_refilling) return;
    _refilling = true;
    try {
      final more = await ref
          .read(discoverRepoProvider)
          .users(
            filter: ref.read(discoverFilterProvider),
            tab: ref.read(discoverTabProvider),
          );
      final have = {for (final u in state.value ?? const <UserProfile>[]) u.id};
      final fresh = more.where((u) => !have.contains(u.id));
      state = AsyncData([...?state.value, ...fresh]);
    } finally {
      _refilling = false;
    }
  }
}

/// 本次会话里点过爱心的人。点爱心只记「喜欢」不切人，心要一直亮着。
final likedIdsProvider = NotifierProvider<LikedIdsNotifier, Set<String>>(
  LikedIdsNotifier.new,
);

class LikedIdsNotifier extends Notifier<Set<String>> {
  @override
  Set<String> build() => const {};

  void add(String id) => state = {...state, id};
}

/// 筛选页底部的「N 人符合」。
final matchCountProvider = FutureProvider.family<int, DiscoverFilter>((
  ref,
  filter,
) {
  return ref.watch(discoverRepoProvider).matchCount(filter);
});

/// 答题匹配题库 + 我的答案（进筛选页拉一次）。开关关时 enabled=false，
/// 筛选页据此回落语言/兴趣维度。
final discoverQuizProvider = FutureProvider<DiscoverQuiz>((ref) {
  return ref.watch(discoverRepoProvider).quiz();
});

final userCardProvider = FutureProvider.family<UserProfile, String>((ref, id) {
  return ref.watch(discoverRepoProvider).card(id);
});
