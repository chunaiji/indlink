import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/providers.dart';
import '../../domain/models/chat.dart';
import '../../domain/models/moment.dart';

final momentTabProvider = NotifierProvider<MomentTabNotifier, String>(
  MomentTabNotifier.new,
);

class MomentTabNotifier extends Notifier<String> {
  @override
  String build() => 'recommend';

  void set(String v) => state = v;
}

/// 某用户的公开动态（用户主页 C3「TA 的动态」预览列表）。
final userMomentsProvider = FutureProvider.family<List<Moment>, String>((
  ref,
  userId,
) {
  return ref.watch(momentRepoProvider).userMoments(userId);
});

/// 动态广场。受 `square_enabled` 控制，App 端需在后台打开。
final momentFeedProvider =
    AsyncNotifierProvider<MomentFeedNotifier, List<Moment>>(
      MomentFeedNotifier.new,
    );

class MomentFeedNotifier extends AsyncNotifier<List<Moment>> {
  @override
  Future<List<Moment>> build() {
    final tab = ref.watch(momentTabProvider);
    return ref.watch(momentRepoProvider).feed(tab);
  }

  Future<void> refresh() async {
    state = AsyncData(
      await ref.read(momentRepoProvider).feed(ref.read(momentTabProvider)),
    );
  }

  /// 点赞先上屏再落库：失败回滚，避免每次点赞都等一个来回。
  Future<void> toggleLike(String momentId) async {
    final list = state.value;
    if (list == null) return;
    final i = list.indexWhere((m) => m.id == momentId);
    if (i < 0) return;

    final liked = !list[i].liked;
    final next = [...list];
    next[i] = next[i].copyWith(
      liked: liked,
      likeCount: next[i].likeCount + (liked ? 1 : -1),
    );
    state = AsyncData(next);

    try {
      final actual = await ref.read(momentRepoProvider).like(momentId, liked);
      // 后端是 toggle，以它翻转后的状态为准——快速连点时本地乐观值可能与它相反。
      if (actual != liked) {
        final base = list[i];
        final fixed = [...state.value ?? next];
        fixed[i] = base.copyWith(
          liked: actual,
          likeCount: base.likeCount + (actual ? 1 : 0) - (base.liked ? 1 : 0),
        );
        state = AsyncData(fixed);
      }
    } catch (_) {
      final rollback = [...state.value ?? next];
      rollback[i] = list[i];
      state = AsyncData(rollback);
    }
  }

  /// 详情页点赞后把状态同步回列表,避免返回列表还看到旧的❤️。
  void syncLike(String momentId, bool liked, int likeCount) {
    final list = state.value;
    if (list == null) return;
    final i = list.indexWhere((m) => m.id == momentId);
    if (i < 0) return;
    final next = [...list];
    next[i] = next[i].copyWith(liked: liked, likeCount: likeCount);
    state = AsyncData(next);
  }

  void prepend(Moment moment) {
    state = AsyncData([moment, ...?state.value]);
  }
}

final momentDetailProvider = FutureProvider.family<Moment, String>((ref, id) {
  return ref.watch(momentRepoProvider).detail(id);
});

final momentCommentsProvider =
    FutureProvider.family<List<MomentComment>, String>((ref, id) {
      return ref.watch(momentRepoProvider).comments(id);
    });

/// 发表评论 / 送礼评论。两者都会让详情页的评论列表失效重拉。
Future<void> postComment(
  WidgetRef ref,
  String momentId, {
  String? text,
  GiftItem? gift,
}) async {
  final repo = ref.read(momentRepoProvider);
  if (gift != null) {
    await repo.giftComment(momentId, gift);
    await ref.read(walletProvider.notifier).refresh();
  } else if (text != null && text.trim().isNotEmpty) {
    await repo.comment(momentId, text.trim());
  }
  ref.invalidate(momentCommentsProvider(momentId));
  ref.invalidate(momentDetailProvider(momentId));
}
