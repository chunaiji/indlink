import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../core/design/tokens.dart';
import '../../core/media/image_pipeline.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/chat.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/flows/gift_flows.dart';
import '../../ui/flows/purchase_flows.dart';
import '../../ui/flows/safety_flows.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/gift_icon.dart';
import '../../ui/widgets/chat_bubbles.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/image_viewer.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';
import '../discover/discover_controller.dart';
import 'chat_controller.dart';

/// D2 聊天窗。
///
/// App 侧比小程序多三件事：前后台切换断连、指数退避重连、重连后拉增量补齐断线消息
/// （见 `core/network/chat_socket.dart`）。
///
/// ⚠️ 已知架构缺口：后端 chat Hub 是**单实例内存态**，多实例部署会丢消息，
/// 水平扩容前需改 Redis Pub/Sub。
/// 类别：聊天页（规范 §6）
/// 固定区：NavBar（头像 + 昵称 + 在线态）、ChatInputBar
/// 弹性区：无
/// 可滚动区：消息列表
/// 键盘：输入栏随 viewInsets 上移，列表 resizeToAvoidBottomInset 收缩后可滚
class ChatRoomPage extends ConsumerStatefulWidget {
  const ChatRoomPage({super.key, required this.conversationId});

  final String conversationId;

  @override
  ConsumerState<ChatRoomPage> createState() => _ChatRoomPageState();
}

class _ChatRoomPageState extends ConsumerState<ChatRoomPage> {
  final _input = TextEditingController();
  final _scroll = ScrollController();

  /// 送礼特效要飞到「对方头像」，用它拿到头像在屏幕上的真实位置。
  final _peerAvatarKey = GlobalKey();

  bool _sending = false;

  /// dispose 里不能再碰 ref（widget 已卸载），进房时先把「关闭会话」这个动作抓在手里。
  late final void Function() _closeActiveConversation;

  Offset? _peerAvatarCenter() {
    final box = _peerAvatarKey.currentContext?.findRenderObject() as RenderBox?;
    if (box == null) return null;
    return box.localToGlobal(box.size.center(Offset.zero));
  }

  @override
  void initState() {
    super.initState();
    _closeActiveConversation = ref
        .read(activeConversationProvider.notifier)
        .close;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      // messagesProvider 是单例(Riverpod 3 手写 family 拿不到参数),进房先 invalidate,
      // 否则首帧会闪一下上一个会话的缓存消息,等异步拉取完才切过来。
      ref.invalidate(messagesProvider);
      ref.read(activeConversationProvider.notifier).open(widget.conversationId);
    });
  }

  @override
  void dispose() {
    // 离开聊天室清掉"当前会话":否则残留的会话号会让 MessagesNotifier 一直挂着旧订阅,
    // 分发器也会误以为该会话还开着而持续把它标记已读。
    _closeActiveConversation();
    _input.dispose();
    _scroll.dispose();
    super.dispose();
  }

  /// 跳到最新一条。
  ///
  /// [animate] 为 false 时**直接跳**：首次进入要立刻停在底部，
  /// 让用户看着列表从头滚到尾是一次没有意义的等待。
  void _scrollToEnd({bool animate = true}) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scroll.hasClients) return;
      final max = _scroll.position.maxScrollExtent;
      if (animate) {
        _scroll.animateTo(
          max,
          duration: const Duration(milliseconds: 220),
          curve: Curves.easeOut,
        );
      } else {
        _scroll.jumpTo(max);
      }
    });
  }

  /// 首帧对齐到底部只做一次,之后由发消息/收消息各自触发。
  bool _landedAtBottom = false;

  Future<void> _send() async {
    final text = _input.text.trim();
    if (text.isEmpty || _sending) return;
    setState(() => _sending = true);
    _input.clear();
    // 发送失败不弹 toast：消息会以「发送失败」留在列表里，点一下就能重发。
    final err = await ref.read(messagesProvider.notifier).send(text);
    _scrollToEnd();
    if (mounted) setState(() => _sending = false);
    await _afterSendFailure(err);
  }

  Future<void> _retry(ChatMessage failed) async {
    final err = await ref.read(messagesProvider.notifier).retry(failed);
    await _afterSendFailure(err);
  }

  /// 按条收费开着、余额不够发这一条：弹 Z4「还差 N 金币」而不是只留一个红叹号。
  /// 消息仍以失败态留在列表里，充完值点一下就能重发。
  Future<void> _afterSendFailure(ApiException? err) async {
    if (err == null || !mounted || !err.isInsufficientBalance) return;
    final l = L.of(context);
    final need = ref.read(appConfigProvider).chatMsgPrice;
    // 先刷一次余额：被拒说明服务端那边的数比本地新。
    try {
      await ref.read(walletProvider.notifier).refresh();
    } catch (_) {}
    if (!mounted) return;
    await showInsufficientModal(
      context,
      ref,
      itemName: l.chatOneMessage,
      need: need,
      have: ref.read(walletProvider).value?.coins ?? 0,
    );
  }

  Future<void> _sendImage() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery);
    if (picked == null) return;
    final compressed = await ImagePipeline.compress(picked.path);
    await ref.read(messagesProvider.notifier).sendImage(compressed.path);
    _scrollToEnd();
  }

  Future<void> _sendGift() async {
    final picked = await showGiftSheet(context, ref);
    if (picked == null) return;
    try {
      await ref.read(messagesProvider.notifier).sendGift(picked.$1, picked.$2);
      // 送礼成功,对方魅力值已在后端累加。userCardProvider 无 autoDispose 会缓存,
      // 不 invalidate 的话再进对方资料页看到的还是旧魅力值(表现为"送礼后没涨")。
      final peerId = ref
          .read(conversationsProvider)
          .value
          ?.where((e) => e.id == widget.conversationId)
          .firstOrNull
          ?.peer
          .id;
      if (peerId != null) {
        ref.invalidate(userCardProvider(peerId));
      }
      // 差额扣了币,余额胶囊要跟着变;背包里的礼物也少了,礼物面板的「×N」要跟着减。
      ref.invalidate(myItemsProvider);
      await ref.read(walletProvider.notifier).refresh();
      _scrollToEnd();
      if (!mounted) return;
      final target = _peerAvatarCenter();
      if (target != null) {
        playGiftFlight(
          context,
          gift: picked.$1,
          qty: picked.$2,
          target: target,
        );
      }
      showToast(context, L.of(context).giftSentToast(picked.$1.name));
    } on ApiException catch (e) {
      if (!mounted) return;
      // 背包不够、余额也不够:引导去买 / 去充值,不弹通用 toast。
      if (e.isItemInsufficient) {
        await showItemInsufficientModal(context, giftName: picked.$1.name);
      } else if (e.isInsufficientBalance) {
        await showInsufficientModal(
          context,
          ref,
          itemName: picked.$1.name,
          need: picked.$1.coins * picked.$2,
          have: ref.read(walletProvider).value?.coins ?? 0,
        );
      } else {
        showToast(context, e.message, error: true);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final messages = ref.watch(messagesProvider);
    final conversation = ref
        .watch(conversationsProvider)
        .value
        ?.where((e) => e.id == widget.conversationId)
        .firstOrNull;
    final peer = conversation?.peer;
    final readOnly = conversation?.readOnly ?? false;

    return Scaffold(
      appBar: NavBar(
        titleWidget: Row(
          children: [
            if (peer != null) ...[
              AvatarRing.of(peer, key: _peerAvatarKey, size: 30),
              const SizedBox(width: Dim.s2),
            ],
            Flexible(
              child: Text(
                peer?.deleted == true
                    ? l.chatDeletedUser
                    : (peer?.nickname ?? ''),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: Dim.t4,
                  fontWeight: FontWeight.w700,
                  color: c.ink,
                ),
              ),
            ),
            // 有 presence 就显示「在线」，没有就退到「N 分钟前活跃」——
            // 不要因为拿不到实时状态就什么都不显示。
            if (peer != null && peer.presumedOnline) ...[
              const SizedBox(width: Dim.s2),
              Text(
                l.commonOnline,
                style: TextStyle(
                  fontSize: Dim.t1,
                  fontWeight: FontWeight.w700,
                  color: c.success,
                ),
              ),
            ] else if (peer?.lastActiveAt != null && peer?.deleted != true) ...[
              const SizedBox(width: Dim.s2),
              Text(
                l.commonActiveAgo(timeAgo(context, peer!.lastActiveAt!)),
                style: TextStyle(fontSize: Dim.t1, color: c.ink3),
              ),
            ],
          ],
        ),
        actions: [
          if (peer != null)
            SafetyMenuButton(
              userId: peer.id,
              userName: peer.nickname,
              onBlocked: () => Navigator.of(context).maybePop(),
            ),
        ],
      ),
      body: Column(
        children: [
          if (ref.watch(chatConnectedProvider).value == false)
            _ReconnectingBar(label: l.chatReconnecting),
          Expanded(
            child: AsyncView(
              value: messages,
              onRetry: () => ref.invalidate(messagesProvider),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const Padding(
                padding: EdgeInsets.all(Dim.gutter),
                child: SkeletonRows(count: 5),
              ),
              data: (list) {
                // 历史加载完成后落到最新一条。聊天页停在最老的消息上
                // 等于每次进来都要手动滑到底，而人要看的永远是最后那句。
                if (!_landedAtBottom && list.isNotEmpty) {
                  _landedAtBottom = true;
                  _scrollToEnd(animate: false);
                }
                // 按条收费开着时，列表顶部先放一条规则提示（前 L 条免费 / 每条 N 币），
                // 让人在发之前就知道价，而不是扣了才在钱包里发现。
                final pricing = ref.watch(appConfigProvider);
                final hint = pricing.chatMsgPrice > 0 && !readOnly
                    ? (pricing.chatFreeMsgs > 0
                          ? l.chatPricingHint(
                              pricing.chatFreeMsgs,
                              pricing.chatMsgPrice,
                            )
                          : l.chatPricingHintNoFree(pricing.chatMsgPrice))
                    : null;
                final offset = hint == null ? 0 : 1;
                return ListView.builder(
                  controller: _scroll,
                  padding: const EdgeInsets.fromLTRB(
                    Dim.gutter,
                    Dim.s3,
                    Dim.gutter,
                    Dim.s3,
                  ),
                  itemCount: list.length + offset,
                  itemBuilder: (context, i) {
                    if (hint != null && i == 0) {
                      return Padding(
                        key: const ValueKey('chat-pricing-hint'),
                        padding: const EdgeInsets.only(bottom: Dim.s3),
                        child: SystemPill(
                          text: hint,
                          icon: Icons.monetization_on_outlined,
                        ),
                      );
                    }
                    final m = list[i - offset];
                    return _MessageRow(message: m, onRetry: () => _retry(m));
                  },
                );
              },
            ),
          ),
          if (readOnly)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(Dim.s4),
              color: c.sea,
              child: Text(
                l.chatDeletedUserHint,
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: Dim.t2, color: c.ink2),
              ),
            )
          else
            // 原型 .inp：输入框 44 胶囊、发送圆钮 44 粉（聊天里的发送带情绪价值，是粉）。
            ChatInputBar(
              controller: _input,
              hint: l.chatInputHint,
              sending: _sending,
              onSend: _send,
              onPickImage: _sendImage,
              onGift: _sendGift,
            ),
        ],
      ),
    );
  }
}

/// 断线提示条。不挡内容，也不弹窗——重连是后台的事，用户只需要知道它在进行。
class _ReconnectingBar extends StatelessWidget {
  const _ReconnectingBar({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      width: double.infinity,
      color: c.coin.withValues(alpha: .18),
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          SizedBox(
            width: 11,
            height: 11,
            child: CircularProgressIndicator(strokeWidth: 1.6, color: c.gold),
          ),
          const SizedBox(width: Dim.s2),
          Text(
            label,
            style: TextStyle(fontSize: Dim.t1, color: c.ink2),
          ),
        ],
      ),
    );
  }
}

class _MessageRow extends StatelessWidget {
  const _MessageRow({required this.message, this.onRetry});

  final ChatMessage message;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    switch (message.type) {
      case MessageType.system:
        // 居中 pill，非某一方发出。文案随消息流下发（建立联系 / 时间 / 安全提示等），
        // 无内容时退回默认「从一个瓶子开始聊天」。安全提示带警示图标。
        final sysText = message.text.isNotEmpty
            ? message.text
            : l.chatStartedFromBottle;
        final warn = message.text.contains('诈骗') || message.text.contains('转账');
        return Padding(
          padding: const EdgeInsets.symmetric(vertical: Dim.s3),
          child: SystemPill(
            text: sysText,
            icon: warn ? Icons.warning_amber_rounded : null,
          ),
        );

      case MessageType.gift:
        final gift = message.gift;
        return Padding(
          padding: const EdgeInsets.symmetric(vertical: Dim.s2),
          child: GiftPill(
            icon: gift == null
                ? Icon(Icons.card_giftcard_rounded, size: 16, color: c.gold)
                : GiftIcon(gift: gift, size: 16),
            text: message.fromMe
                ? l.chatGiftSentBy(gift?.name ?? '')
                : l.chatGiftReceived(gift?.name ?? ''),
            charm: l.chatCharmPlus((gift?.charm ?? 0) * message.giftQty),
          ),
        );

      case MessageType.image:
      case MessageType.text:
        final me = message.fromMe;
        final read = me && message.read ? l.chatRead : null;
        final body = message.type == MessageType.image
            ? ImageMessage(
                url: message.imageUrl ?? '',
                readLabel: read,
                onTap: (message.imageUrl == null || message.imageUrl!.isEmpty)
                    ? null
                    : () => openImageViewer(context, message.imageUrl!),
              )
            : MessageBubble(mine: me, text: message.text, readLabel: read);
        return Padding(
          padding: const EdgeInsets.symmetric(vertical: Dim.s1),
          child: Row(
            mainAxisAlignment: me
                ? MainAxisAlignment.end
                : MainAxisAlignment.start,
            children: [
              // 失败的消息不消失，点感叹号即可重发。
              if (message.failed)
                GestureDetector(
                  onTap: onRetry,
                  child: Padding(
                    padding: const EdgeInsets.only(right: Dim.s2),
                    child: Icon(
                      Icons.error_outline_rounded,
                      size: 18,
                      color: c.warn,
                    ),
                  ),
                ),
              Flexible(
                child: Opacity(
                  // 发送中先上屏、降透明度；收到 ACK 后恢复。
                  opacity: message.sending ? .55 : 1,
                  child: body,
                ),
              ),
            ],
          ),
        );
    }
  }
}
