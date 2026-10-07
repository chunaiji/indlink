import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/logging/log_record.dart';
import '../../core/logging/logger.dart';
import '../../core/design/tokens.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../core/utils/anon_meta.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/bottle.dart';
import '../../domain/models/user.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/flows/purchase_flows.dart';
import '../../ui/flows/safety_flows.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chat_bubbles.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/coin.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';
import 'my_bottles_page.dart' show traceItems;
import 'ocean_controller.dart';

/// B3 瓶子详情。
///
/// 时间在这屏有两种口径且不能混：瓶子说「漂了 3 天」（时长），
/// 回信说「12 分钟前」（时刻）。
///
/// 解锁按 viewer 维度记录（`ReplyUnlock`），**任何人付费即可看，不只瓶主**。
/// 按条解锁：每条锁着的回信右侧挂一枚「解锁 · 价」小标，想看哪条付哪条——
/// 整瓶打包「解锁 N 条」已去掉（真机反馈一次性解锁不合理）。
/// 类别：详情页（规范 §6）
/// 固定区：NavBar、底部回信输入栏（ChatInputBar）
/// 弹性区：无
/// 可滚动区：瓶子正文卡 + 回信列表
/// 键盘：输入栏随 viewInsets 上移，列表 resizeToAvoidBottomInset 收缩后可滚
class BottleDetailPage extends ConsumerStatefulWidget {
  const BottleDetailPage({super.key, required this.bottleId});

  final String bottleId;

  @override
  ConsumerState<BottleDetailPage> createState() => _BottleDetailPageState();
}

class _BottleDetailPageState extends ConsumerState<BottleDetailPage> {
  final _input = TextEditingController();
  bool _sending = false;

  /// 正在解锁的那条回信 id；同一时间只允许一条在途。
  String? _unlockingId;

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  /// 解锁一条回信。价格是这一条服务端下发的 `unlock_cost`（`price_unlock`，后台可调），
  /// 不要在客户端写死——界面写 2、后端扣 6，用户看到的就是乱扣费。
  Future<void> _unlockOne(BottleReply reply) async {
    if (_unlockingId != null) return;
    final l = L.of(context);
    final price = reply.unlockCost;
    // 这一页不 watch 钱包，首次进来 walletProvider 可能还没加载；
    // 直接 `.value ?? 0` 会把有钱的人也拦成「余额不足」。等它一次。
    int balance;
    try {
      balance = (await ref.read(walletProvider.future)).coins;
    } catch (_) {
      balance = ref.read(walletProvider).value?.coins ?? 0;
    }
    if (!mounted) return;

    if (balance < price) {
      await showInsufficientModal(
        context,
        ref,
        itemName: l.bottleUnlockOneTitle,
        need: price,
        have: balance,
      );
      return;
    }

    // 扣费动作二次确认：列表行一点就扣钱会被当成坑钱。
    final ok = await showAppModal<bool>(
      context,
      builder: (ctx) => ModalCard(
        icon: Icons.lock_open_rounded,
        title: l.bottleUnlockOneTitle,
        body: Text(l.bottleUnlockOneBody(price)),
        actions: [
          AppButton(
            label: l.bottleUnlockOneConfirm(price),
            kind: BtnKind.pay,
            onTap: () => Navigator.of(ctx).pop(true),
          ),
          AppButton(
            label: l.commonCancel,
            kind: BtnKind.ghost,
            onTap: () => Navigator.of(ctx).pop(false),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;

    setState(() => _unlockingId = reply.id);
    try {
      await ref.read(bottleRepoProvider).unlockReply(reply.id);
      Log.i(LogTag.biz, 'unlock', fields: {'price': price});
      ref.invalidate(bottleRepliesProvider(widget.bottleId));
      await ref.read(walletProvider.notifier).refresh();
      if (mounted) showToast(context, l.bottleUnlockedOne);
    } on ApiException catch (e) {
      if (!mounted) return;
      if (e.isInsufficientBalance) {
        await showInsufficientModal(
          context,
          ref,
          itemName: l.bottleUnlockOneTitle,
          need: price,
          have: balance,
        );
      } else {
        showToast(context, e.message, error: true);
      }
    } finally {
      if (mounted) setState(() => _unlockingId = null);
    }
  }

  Future<void> _toggleCollect(Bottle bottle) async {
    final next = !bottle.collected;
    final l = L.of(context);
    try {
      await ref.read(bottleRepoProvider).setCollected(bottle.id, next);
      ref.invalidate(bottleDetailProvider(widget.bottleId));
      ref.invalidate(myBottlesProvider(BottleMineTab.collected));
      if (mounted) {
        showToast(context, next ? l.bottleCollected : l.bottleUncollected);
      }
    } on ApiException catch (e) {
      if (mounted) showToast(context, e.message, error: true);
    }
  }

  Future<void> _sendReply() async {
    final text = _input.text.trim();
    if (text.isEmpty) return;
    setState(() => _sending = true);
    try {
      await ref.read(bottleRepoProvider).reply(widget.bottleId, text);
      _input.clear();
      ref.invalidate(bottleRepliesProvider(widget.bottleId));
      if (mounted) showToast(context, L.of(context).bottleReplySent);
    } on ApiException catch (e) {
      if (mounted) showToast(context, e.message, error: true);
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final detail = ref.watch(bottleDetailProvider(widget.bottleId));
    final replies = ref.watch(bottleRepliesProvider(widget.bottleId));
    final authorId = detail.value?.author.id ?? '';

    return Scaffold(
      appBar: NavBar(
        title: l.bottleScoopedTitle,
        actions: [
          IconButton(
            icon: Icon(
              detail.value?.collected == true
                  ? Icons.favorite_rounded
                  : Icons.favorite_border_rounded,
              color: detail.value?.collected == true
                  ? context.c.brand
                  : context.c.ink,
            ),
            onPressed: detail.value == null
                ? null
                : () => _toggleCollect(detail.value!),
          ),
          // 合规：举报入口常驻，不可藏在更深的层级。
          SafetyMenuButton(
            userId: authorId,
            userName: anonSenderName(ref, l.commonAnonymous),
            onBlocked: () => Navigator.of(context).maybePop(),
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: AsyncView(
              value: detail,
              onRetry: () =>
                  ref.invalidate(bottleDetailProvider(widget.bottleId)),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const Padding(
                padding: EdgeInsets.all(Dim.gutter),
                child: SkeletonRows(count: 4),
              ),
              data: (bottle) => ListView(
                padding: const EdgeInsets.all(Dim.gutter),
                children: [
                  _BottleCard(bottle: bottle),
                  // 漂流路径：服务端只在开关开启、且我是瓶主或捞过它时才下发节点。
                  if (bottle.trace.isNotEmpty) ...[
                    const SizedBox(height: Dim.s3),
                    _DriftPathCard(bottle: bottle),
                  ],
                  const SizedBox(height: Dim.s4),
                  replies.when(
                    loading: () => const SkeletonRows(count: 3),
                    error: (_, _) => Text(
                      l.stateLoadFailed,
                      style: TextStyle(fontSize: Dim.t2, color: context.c.ink3),
                    ),
                    data: (list) => _Replies(
                      replies: list,
                      unlockingId: _unlockingId,
                      onUnlock: _unlockOne,
                    ),
                  ),
                ],
              ),
            ),
          ),
          // 回信是免费动作：发送钮海蓝（原型 .inp .snd 在聊天里才是粉）。
          ChatInputBar(
            controller: _input,
            hint: l.bottleReplyHint,
            sending: _sending,
            sendTone: ChatSendTone.aqua,
            onSend: _sendReply,
          ),
        ],
      ),
    );
  }
}

class _BottleCard extends ConsumerWidget {
  const _BottleCard({required this.bottle});

  final Bottle bottle;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final genderLabel = switch (bottle.author.gender) {
      Gender.female => l.commonFemale,
      Gender.male => l.commonMale,
      Gender.secret => '',
    };

    return BrandCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 原型 .btlc：作者行 avatar 44 + 匿名元信息 t2 ink2，右侧「漂了 N 天」粉标签。
          // 元信息必须可收缩，否则窄屏 + 大字体直接把标签顶出卡片。
          Row(
            children: [
              AvatarRing.of(bottle.author, size: 44),
              const SizedBox(width: Dim.s3),
              Expanded(
                child: Text(
                  anonMeta(
                    anonSenderName(ref, l.commonAnonymous),
                    genderLabel,
                    bottle.author.age,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: Dim.t2,
                    color: c.ink2,
                    height: 1.4,
                  ),
                ),
              ),
              const SizedBox(width: Dim.s2),
              // 原型 .tg 默认样式（sea 底 ink2），不是粉色：漂流天数是信息，不是付费点。
              TagChip(label: driftedFor(context, bottle.createdAt)),
            ],
          ),
          const SizedBox(height: Dim.s3),
          Text(
            bottle.content,
            style: TextStyle(fontSize: Dim.t4, height: 1.75, color: c.ink),
          ),
        ],
      ),
    );
  }
}

/// 「这个瓶子漂过哪些地方」。
///
/// 捞到的人看到它从哪漂来，终点是「你这里」——「漂了 3 天 · Mumbai → 你这里」在 B1b
/// 只是一句话，这里把整条路摊开。与 B4 共用同一份 [TimelineList] 与节点文案。
/// 瓶主看的是自己的瓶子：没有「你这里」这一站，右上角多一个进漂流海图的入口
/// （海报只该由瓶主生成，B4s 的隐私提示写的就是这个）。
class _DriftPathCard extends ConsumerWidget {
  const _DriftPathCard({required this.bottle});

  final Bottle bottle;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final meId = ref.watch(authProvider).profile?.id;
    final mine = meId != null && meId == bottle.author.id;
    final items = [
      ...traceItems(context, bottle),
      if (!mine) (l.bottleTraceHere, l.commonJustNow),
    ];

    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: SectionLabel(l.bottleTraceTitle)),
              if (mine)
                MiniButton(
                  label: l.bottleMapTitle,
                  kind: BtnKind.ghost,
                  onTap: () => context.push(Routes.driftMap(bottle.id)),
                ),
            ],
          ),
          const SizedBox(height: Dim.s3),
          TimelineList(items: items),
          if (bottle.viewCount > 0) ...[
            const SizedBox(height: Dim.s2),
            Text(
              l.bottleTraceSummary(bottle.viewCount, bottle.cityCount),
              style: TextStyle(fontSize: Dim.t1, color: c.ink3),
            ),
          ],
        ],
      ),
    );
  }
}

class _Replies extends StatelessWidget {
  const _Replies({
    required this.replies,
    required this.unlockingId,
    required this.onUnlock,
  });

  final List<BottleReply> replies;
  final String? unlockingId;
  final ValueChanged<BottleReply> onUnlock;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    if (replies.isEmpty) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: Dim.s5),
        child: Text(
          l.stateEmptyBottlesBody,
          style: TextStyle(fontSize: Dim.t2, color: c.ink3),
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SectionLabel(
          l.bottleRepliesHeader(
            replies.length,
            timeAgo(context, replies.last.createdAt),
          ),
        ),
        const SizedBox(height: Dim.s2),
        for (final r in replies)
          _ReplyRow(
            reply: r,
            unlocking: unlockingId == r.id,
            onUnlock: r.locked ? () => onUnlock(r) : null,
          ),
      ],
    );
  }
}

class _ReplyRow extends ConsumerWidget {
  const _ReplyRow({
    required this.reply,
    required this.unlocking,
    required this.onUnlock,
  });

  final BottleReply reply;
  final bool unlocking;

  /// 锁着时整行可点，点了走解锁确认；已解锁为 null。
  final VoidCallback? onUnlock;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final genderLabel = switch (reply.author.gender) {
      Gender.female => l.commonFemale,
      Gender.male => l.commonMale,
      Gender.secret => '',
    };

    final content = Text(
      reply.content,
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
      style: TextStyle(fontSize: Dim.t2, color: c.ink2),
    );

    // 原型 .rws a：行最小高 44px → 65，头像 44，padding s2 0，底线 line2。
    final row = Container(
      constraints: const BoxConstraints(minHeight: 65),
      padding: const EdgeInsets.symmetric(vertical: Dim.s2),
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: c.line2)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          AvatarRing.of(reply.author, size: 44),
          const SizedBox(width: Dim.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  anonMeta(
                    anonSenderName(ref, l.commonAnonymous),
                    genderLabel,
                    reply.author.age,
                  ),
                  style: TextStyle(
                    fontSize: Dim.t2,
                    fontWeight: FontWeight.w700,
                    color: c.ink,
                  ),
                ),
                const SizedBox(height: 2),
                // 未解锁的回信做模糊处理：看得见有内容，读不到内容。
                // 原型 .blur 3.5px → 5。
                reply.locked
                    ? ImageFiltered(
                        imageFilter: ui.ImageFilter.blur(sigmaX: 5, sigmaY: 5),
                        child: content,
                      )
                    : content,
              ],
            ),
          ),
          const SizedBox(width: Dim.s2),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                timeAgo(context, reply.createdAt),
                style: TextStyle(fontSize: Dim.t1, color: c.ink3),
              ),
              // 锁着的回信：时间下面挂一枚「解锁 · 价」粉色小标，整行可点。
              if (reply.locked) ...[
                const SizedBox(height: Dim.s1),
                _UnlockPill(price: reply.unlockCost, busy: unlocking),
              ],
            ],
          ),
        ],
      ),
    );
    if (onUnlock == null) return row;
    return InkWell(onTap: unlocking ? null : onUnlock, child: row);
  }
}

/// 「🔒 解锁 · 2」小标（.tg 的粉色变体 + 金币图标）。
class _UnlockPill extends StatelessWidget {
  const _UnlockPill({required this.price, required this.busy});

  final int price;
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      key: const ValueKey('reply-unlock-pill'),
      padding: const EdgeInsets.symmetric(horizontal: Dim.s2, vertical: Dim.s1),
      decoration: BoxDecoration(
        color: c.brand.withValues(alpha: .12),
        borderRadius: Dim.brPill,
      ),
      child: busy
          ? SizedBox(
              width: 12,
              height: 12,
              child: CircularProgressIndicator(
                strokeWidth: 1.5,
                color: c.brand,
              ),
            )
          : Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.lock_outline_rounded, size: 11, color: c.brand),
                const SizedBox(width: 3),
                Text(
                  L.of(context).bottleUnlockPill,
                  style: TextStyle(
                    fontSize: Dim.t0,
                    fontWeight: FontWeight.w800,
                    color: c.brand,
                    height: 1.3,
                  ),
                ),
                const SizedBox(width: 3),
                const CoinIcon(size: 11),
                const SizedBox(width: 2),
                Text(
                  '$price',
                  style: TextStyle(
                    fontSize: Dim.t0,
                    fontWeight: FontWeight.w800,
                    color: c.brand,
                    height: 1.3,
                  ),
                ),
              ],
            ),
    );
  }
}

/// 底部回信输入条（`.inp`）。
