import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/logging/log_record.dart';
import '../../core/logging/logger.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../domain/models/chat.dart';
import '../../domain/models/user.dart';

/// 会话列表。
///
/// WS 推来新消息时**本地更新**列表，不重拉——否则每条消息一次全量请求。
final conversationsProvider =
    AsyncNotifierProvider<ConversationsNotifier, List<Conversation>>(
      ConversationsNotifier.new,
    );

class ConversationsNotifier extends AsyncNotifier<List<Conversation>> {
  @override
  Future<List<Conversation>> build() async {
    final list = await ref.watch(chatRepoProvider).conversations();
    _syncBadge(list);
    return list;
  }

  /// Tab 未读红点。build 期间不能直接改别的 provider，推到下一帧。
  void _syncBadge(List<Conversation> list) {
    final total = list.fold<int>(0, (sum, c) => sum + c.unread);
    Future.microtask(() => ref.read(unreadProvider.notifier).set(total));
  }

  Future<void> refresh() async {
    final list = await ref.read(chatRepoProvider).conversations();
    state = AsyncData(list);
    _syncBadge(list);
  }

  /// 本地落一条消息到对应会话，避免整表重拉。
  void applyIncoming(String conversationId, ChatMessage msg) {
    final list = state.value;
    if (list == null) return;
    final i = list.indexWhere((c) => c.id == conversationId);
    if (i < 0) return;

    final next = [...list];
    next[i] = next[i].copyWith(
      lastText: msg.type == MessageType.gift
          ? (msg.gift?.name ?? '')
          : msg.text,
      lastType: msg.type,
      lastAt: msg.at,
      unread: msg.fromMe ? next[i].unread : next[i].unread + 1,
    );
    next.sort((a, b) => b.lastAt.compareTo(a.lastAt));
    state = AsyncData(next);
    _syncBadge(next);
  }

  void markRead(String conversationId) {
    final list = state.value;
    if (list == null) return;
    final i = list.indexWhere((c) => c.id == conversationId);
    if (i < 0 || list[i].unread == 0) return;
    final next = [...list];
    next[i] = next[i].copyWith(unread: 0);
    state = AsyncData(next);
    _syncBadge(next);
  }
}

/// 会话列表筛选：全部 / 未读 / 瓶友。
final chatFilterProvider = NotifierProvider<ChatFilterNotifier, String>(
  ChatFilterNotifier.new,
);

class ChatFilterNotifier extends Notifier<String> {
  @override
  String build() => 'all';

  void set(String v) => state = v;
}

/// 当前打开的会话。
///
/// 同一时刻只会有一个聊天窗，所以用单例 notifier 而不是 family——
/// Riverpod 3 手写 family notifier 拿不到参数（需要 codegen）。
final activeConversationProvider =
    NotifierProvider<ActiveConversationNotifier, String?>(
      ActiveConversationNotifier.new,
    );

class ActiveConversationNotifier extends Notifier<String?> {
  @override
  String? build() => null;

  void open(String id) => state = id;
  void close() => state = null;
}

final messagesProvider =
    AsyncNotifierProvider<MessagesNotifier, List<ChatMessage>>(
      MessagesNotifier.new,
    );

class MessagesNotifier extends AsyncNotifier<List<ChatMessage>> {
  StreamSubscription<ChatMessage>? _sub;

  String? get _id => ref.read(activeConversationProvider);

  StreamSubscription<void>? _reconnectSub;

  @override
  Future<List<ChatMessage>> build() async {
    final id = ref.watch(activeConversationProvider);
    if (id == null) return const [];

    final repo = ref.watch(chatRepoProvider);

    _sub?.cancel();
    _sub = repo.incoming(id).listen((msg) {
      if (msg.fromMe) return;
      // 只负责渲染当前会话的消息流;会话列表预览 / 未读 / Tab 红点由
      // chatDispatcherProvider 统一处理(不再只有开着聊天室时才更新)。
      state = AsyncData([...?state.value, msg]);
    });

    // 断线期间对方发的消息不会经由 WS 补发，重连后必须主动拉一次增量。
    _reconnectSub?.cancel();
    _reconnectSub = ref.watch(chatSocketProvider)?.reconnected.listen((_) {
      resync();
    });

    ref.onDispose(() {
      _sub?.cancel();
      _reconnectSub?.cancel();
    });

    final list = await repo.messages(id);
    Future.microtask(
      () => ref.read(conversationsProvider.notifier).markRead(id),
    );
    return list;
  }

  /// 重连后的增量补齐：服务端结果为准，但**保留本地还没发出去的消息**
  /// （正在发送 / 发送失败），否则用户刚写的内容会凭空消失。
  Future<void> resync() async {
    final id = _id;
    if (id == null) return;
    final server = await ref.read(chatRepoProvider).messages(id);
    final local = (state.value ?? const <ChatMessage>[]).where(
      (m) => m.sending || m.failed,
    );
    state = AsyncData([...server, ...local]);
  }

  /// 乐观发送：先上屏再等 ACK。
  ///
  /// 失败**不抛异常**——消息以 `failed` 留在列表里，用户点一下就能重发，
  /// 比弹一个 toast 然后把内容丢掉要好。但失败原因要交还给页面：
  /// 余额不足（按条收费开着）要弹「还差 N 金币」充值弹窗，不能只留一个红叹号。
  Future<ApiException?> send(String text) async {
    final id = _id;
    if (id == null) return null;

    final draft = ChatMessage(
      id: 'local-${DateTime.now().microsecondsSinceEpoch}',
      fromMe: true,
      type: MessageType.text,
      at: DateTime.now().toUtc(),
      text: text,
      sending: true,
    );
    state = AsyncData([...?state.value, draft]);

    try {
      final msg = await ref.read(chatRepoProvider).send(id, text);
      _replace(draft.id, msg);
      ref.read(conversationsProvider.notifier).applyIncoming(id, msg);
      return null;
    } on ApiException catch (e) {
      _replace(draft.id, draft.copyWith(sending: false, failed: true));
      return e;
    } catch (_) {
      _replace(draft.id, draft.copyWith(sending: false, failed: true));
      return null;
    }
  }

  /// 重发一条失败的消息。失败原因同 [send] 交还给页面。
  Future<ApiException?> retry(ChatMessage failed) async {
    final list = [...?state.value]..removeWhere((m) => m.id == failed.id);
    state = AsyncData(list);
    return send(failed.text);
  }

  void _replace(String id, ChatMessage next) {
    final list = [...?state.value];
    final i = list.indexWhere((m) => m.id == id);
    if (i < 0) return;
    list[i] = next;
    state = AsyncData(list);
  }

  Future<void> sendImage(String path) async {
    final id = _id;
    if (id == null) return;
    final msg = await ref.read(chatRepoProvider).sendImage(id, path);
    state = AsyncData([...?state.value, msg]);
    ref.read(conversationsProvider.notifier).applyIncoming(id, msg);
  }

  Future<void> sendGift(GiftItem gift, int qty) async {
    final id = _id;
    if (id == null) return;
    final msg = await ref.read(chatRepoProvider).sendGift(id, gift, qty);
    Log.i(LogTag.biz, 'gift', fields: {'gift': gift.id, 'qty': qty});
    state = AsyncData([...?state.value, msg]);
    ref.read(conversationsProvider.notifier).applyIncoming(id, msg);
    await ref.read(walletProvider.notifier).refresh();
  }
}

final blocklistProvider = FutureProvider<List<UserBrief>>((ref) {
  return ref.watch(chatRepoProvider).blocklist();
});

/// 长连接是否在线。断开时聊天页顶部显示「正在重新连接…」。
/// Mock 模式没有真实长连接，恒为已连接。
final chatConnectedProvider = StreamProvider<bool>((ref) {
  final socket = ref.watch(chatSocketProvider);
  if (socket == null) return Stream.value(true);
  return socket.connected;
});

/// 常驻消息分发器:与聊天室解耦,登录进主界面后一直订阅**全局**消息流。
///
/// 收到任意会话的新消息都在这里更新会话列表预览 + 未读 + Tab 红点——不管用户当前
/// 在哪个 Tab、有没有打开聊天室。聊天室([MessagesNotifier])只负责渲染当前会话的
/// 消息流,不再兼职更新列表(那样只有开着聊天室时列表才会刷新,正是"收消息不刷新"的根)。
///
/// 由 [AppShell] 在登录后 `watch` 激活(见 shell.dart);mock 与真机都走 `allMessages`。
final chatDispatcherProvider = Provider<void>((ref) {
  // 先确保会话列表已加载:未进过消息 Tab 时 conversationsProvider 尚未 build,
  // 收到的消息会因列表为空而落不进去(红点不涨)。read 一次触发加载并常驻。
  ref.read(conversationsProvider);
  final repo = ref.watch(chatRepoProvider);
  final sub = repo.allMessages().listen((msg) {
    if (msg.fromMe) return;
    final cid = msg.conversationId;
    if (cid == null || cid.isEmpty) return;
    final convs = ref.read(conversationsProvider.notifier);
    convs.applyIncoming(cid, msg);
    // 正开着的会话:收到即已读,红点不涨。其它会话累加未读。
    if (ref.read(activeConversationProvider) == cid) {
      convs.markRead(cid);
    }
  });
  ref.onDispose(sub.cancel);
});
