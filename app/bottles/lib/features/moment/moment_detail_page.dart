import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../core/utils/time_format.dart';
import '../../domain/models/moment.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/flows/gift_flows.dart';
import '../../ui/flows/safety_flows.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/gift_icon.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chat_bubbles.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/image_viewer.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';
import 'moment_controller.dart';

/// F3 动态详情。
///
/// 评论已挂后端 `commentLimit` 中间件；礼物评论走 `MomentComment.Type=gift`，
/// content 存 JSON `{name,icon,coins}`。
/// 类别：详情页（规范 §6）
/// 固定区：NavBar（❤️ + 举报）、底部评论输入栏（ChatInputBar）
/// 弹性区：无
/// 可滚动区：作者行 + 正文 + 图片 + 评论
/// 键盘：输入栏随 viewInsets 上移，列表 resizeToAvoidBottomInset 收缩后可滚
class MomentDetailPage extends ConsumerStatefulWidget {
  const MomentDetailPage({super.key, required this.momentId});

  final String momentId;

  @override
  ConsumerState<MomentDetailPage> createState() => _MomentDetailPageState();
}

class _MomentDetailPageState extends ConsumerState<MomentDetailPage> {
  final _input = TextEditingController();
  bool _busy = false;

  // 详情页点赞独立处理:momentDetailProvider 是 FutureProvider(改不了 state),
  // feed 的 toggleLike 又只认列表里的动态(详情页这条未必在列表)。所以这里自己
  // 乐观更新 + 调后端,再把结果同步回 feed。
  bool? _liked;
  int? _likeCount;
  bool _liking = false;

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  Future<void> _toggleLike(Moment moment) async {
    if (_liking) return;
    final cur = _liked ?? moment.liked;
    final curCount = _likeCount ?? moment.likeCount;
    final next = !cur;
    setState(() {
      _liking = true;
      _liked = next;
      _likeCount = curCount + (next ? 1 : -1);
    });
    try {
      final actual = await ref.read(momentRepoProvider).like(moment.id, next);
      if (mounted && actual != next) {
        setState(() {
          _liked = actual;
          _likeCount = curCount + (actual ? 1 : 0) - (cur ? 1 : 0);
        });
      }
      ref
          .read(momentFeedProvider.notifier)
          .syncLike(moment.id, _liked!, _likeCount!);
    } catch (_) {
      if (mounted) {
        setState(() {
          _liked = cur;
          _likeCount = curCount;
        });
      }
    } finally {
      if (mounted) setState(() => _liking = false);
    }
  }

  Future<void> _comment() async {
    final text = _input.text.trim();
    if (text.isEmpty || _busy) return;
    setState(() => _busy = true);
    try {
      await postComment(ref, widget.momentId, text: text);
      _input.clear();
    } on ApiException catch (e) {
      if (mounted) showToast(context, e.message, error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _gift() async {
    final picked = await showGiftSheet(context, ref);
    if (picked == null) return;
    try {
      await postComment(ref, widget.momentId, gift: picked.$1);
      // 送出一件就少一件:不刷新的话礼物面板还显示旧库存,像是"送了没扣"。
      ref.invalidate(myItemsProvider);
      await ref.read(walletProvider.notifier).refresh();
      if (mounted) {
        showToast(context, L.of(context).giftSentToast(picked.$1.name));
      }
    } on ApiException catch (e) {
      if (mounted) showToast(context, e.message, error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final detail = ref.watch(momentDetailProvider(widget.momentId));
    final comments = ref.watch(momentCommentsProvider(widget.momentId));

    return Scaffold(
      appBar: NavBar(
        title: l.momentDetailTitle,
        actions: [
          if (detail.value != null) ...[
            // ❤️ 放到 header:点赞是对整条动态的操作,比塞在评论输入栏更合理。
            _LikeAction(
              liked: _liked ?? detail.value!.liked,
              count: _likeCount ?? detail.value!.likeCount,
              onTap: () => _toggleLike(detail.value!),
            ),
            SafetyMenuButton(
              userId: detail.value!.author.id,
              userName: detail.value!.author.nickname,
              onBlocked: () => Navigator.of(context).maybePop(),
            ),
          ],
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: AsyncView(
              value: detail,
              onRetry: () =>
                  ref.invalidate(momentDetailProvider(widget.momentId)),
              errorTitle: l.stateErrorTitle,
              errorBody: l.stateErrorBody,
              retryLabel: l.commonRetry,
              skeleton: const Padding(
                padding: EdgeInsets.all(Dim.gutter),
                child: SkeletonRows(count: 4),
              ),
              data: (moment) => ListView(
                padding: const EdgeInsets.all(Dim.gutter),
                children: [
                  _AuthorRow(moment: moment),
                  const SizedBox(height: Dim.s3),
                  Text(
                    moment.text,
                    style: TextStyle(
                      fontSize: Dim.t3,
                      height: 1.6,
                      color: context.c.ink,
                    ),
                  ),
                  if (moment.images.isNotEmpty) ...[
                    const SizedBox(height: Dim.s3),
                    for (final img in moment.images)
                      Padding(
                        padding: const EdgeInsets.only(bottom: Dim.s2),
                        // 点图看全图(不裁切);缩略仍用统一比例,避免长图撑爆详情页。
                        child: GestureDetector(
                          onTap: img.isEmpty
                              ? null
                              : () => openImageViewer(context, img),
                          child: ClipRRect(
                            borderRadius: Dim.brCard,
                            child: AspectRatio(
                              aspectRatio: 16 / 10,
                              child: img.isEmpty
                                  ? ColoredBox(color: context.c.sea)
                                  : CachedNetworkImage(
                                      imageUrl: img,
                                      fit: BoxFit.cover,
                                      placeholder: (_, _) =>
                                          ColoredBox(color: context.c.sea),
                                      errorWidget: (_, _, _) =>
                                          ColoredBox(color: context.c.sea),
                                    ),
                            ),
                          ),
                        ),
                      ),
                  ],
                  const SizedBox(height: Dim.s3),
                  SectionLabel(l.momentCommentsHeader(moment.commentCount)),
                  const SizedBox(height: Dim.s2),
                  comments.when(
                    loading: () => const SkeletonRows(count: 3),
                    error: (_, _) => Text(
                      l.stateLoadFailed,
                      style: TextStyle(fontSize: Dim.t2, color: context.c.ink3),
                    ),
                    data: (list) => Column(
                      children: [for (final c in list) _CommentRow(comment: c)],
                    ),
                  ),
                ],
              ),
            ),
          ),
          // 评论是免费动作：发送钮海蓝；礼物入口挂在栏左侧。
          ChatInputBar(
            controller: _input,
            hint: l.momentCommentHint,
            sending: _busy,
            sendTone: ChatSendTone.aqua,
            onSend: _comment,
            onGift: _busy ? null : _gift,
          ),
        ],
      ),
    );
  }
}

class _AuthorRow extends ConsumerWidget {
  const _AuthorRow({required this.moment});

  final Moment moment;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);

    return Row(
      children: [
        GestureDetector(
          onTap: () => context.push(Routes.userProfile(moment.author.id)),
          child: AvatarRing.of(moment.author, size: 40),
        ),
        const SizedBox(width: Dim.s2),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                moment.author.nickname,
                style: TextStyle(
                  fontSize: Dim.t3,
                  fontWeight: FontWeight.w700,
                  color: c.ink,
                ),
              ),
              const SizedBox(height: 1),
              Text(
                [
                  timeAgo(context, moment.createdAt),
                  if (moment.city != null) moment.city!,
                ].join(' · '),
                style: TextStyle(fontSize: Dim.t1, color: c.ink3),
              ),
            ],
          ),
        ),
        MiniButton(
          label: moment.following ? l.momentFollowing : l.momentFollow,
          kind: moment.following ? BtnKind.ghost : BtnKind.primary,
          onTap: () async {
            final next = !moment.following;
            try {
              await ref
                  .read(momentRepoProvider)
                  .setFollow(moment.author.id, next);
              ref.invalidate(momentDetailProvider(moment.id));
              // 关注变了，「关注」Tab 的内容也跟着变，整份 feed 一起失效。
              ref.invalidate(momentFeedProvider);
            } on ApiException catch (e) {
              // 失败必须说话：静默失败正是「点了没反应」的观感来源。
              if (context.mounted) showToast(context, e.message, error: true);
            }
          },
        ),
      ],
    );
  }
}

class _CommentRow extends StatelessWidget {
  const _CommentRow({required this.comment});

  final MomentComment comment;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    // 原型 .rws a：最小高 65、头像 44、padding s2 0、底线 line2。
    return Container(
      constraints: const BoxConstraints(minHeight: 65),
      padding: const EdgeInsets.symmetric(vertical: Dim.s2),
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: c.line2)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          AvatarRing.of(comment.author, size: 44),
          const SizedBox(width: Dim.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  comment.author.nickname,
                  style: TextStyle(
                    fontSize: Dim.t2,
                    fontWeight: FontWeight.w700,
                    color: c.ink,
                  ),
                ),
                const SizedBox(height: 2),
                comment.isGift
                    ? Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          GiftIcon(gift: comment.gift!, size: 16),
                          const SizedBox(width: 4),
                          Flexible(
                            child: Text(
                              l.momentGiftComment(comment.gift!.name),
                              style: TextStyle(
                                fontSize: Dim.t2,
                                fontWeight: FontWeight.w700,
                                color: c.gold,
                              ),
                            ),
                          ),
                        ],
                      )
                    : Text(
                        comment.text,
                        style: TextStyle(
                          fontSize: Dim.t2,
                          height: 1.55,
                          color: c.ink2,
                        ),
                      ),
              ],
            ),
          ),
          const SizedBox(width: Dim.s2),
          Text(
            timeAgo(context, comment.createdAt),
            style: TextStyle(fontSize: Dim.t1, color: c.ink3),
          ),
        ],
      ),
    );
  }
}

/// 详情页 header 的点赞按钮:❤️ + 数量。
class _LikeAction extends StatelessWidget {
  const _LikeAction({
    required this.liked,
    required this.count,
    required this.onTap,
  });

  final bool liked;
  final int count;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: Dim.s2),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              liked ? Icons.favorite_rounded : Icons.favorite_border_rounded,
              size: 20,
              color: liked ? c.brand : c.ink2,
            ),
            if (count > 0) ...[
              const SizedBox(width: 3),
              Text(
                '$count',
                style: TextStyle(fontSize: Dim.t1, color: c.ink2),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
