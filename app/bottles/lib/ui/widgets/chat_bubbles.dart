import 'dart:io';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';

/// 底部输入栏（原型 `.inp`）：padding s2 s4 s3、顶线 line2；输入框 30px → 44 高的
/// page 底胶囊、t3；发送圆钮 30px → 44 brand。左侧可挂图片 / 礼物两个 28pt 线性图标。
///
/// 底部 = max(安全区, s3) 再加键盘高度——它自己处理，调用方不要再包 SafeArea。
/// 聊天室、瓶子详情的回信、动态评论三处共用。
class ChatInputBar extends StatelessWidget {
  const ChatInputBar({
    super.key,
    required this.controller,
    required this.onSend,
    this.hint,
    this.sending = false,
    this.onPickImage,
    this.onGift,
    this.focusNode,
    this.sendTone = ChatSendTone.brand,
  });

  final TextEditingController controller;
  final VoidCallback onSend;
  final String? hint;
  final bool sending;
  final VoidCallback? onPickImage;
  final VoidCallback? onGift;
  final FocusNode? focusNode;

  /// 聊天里发送是粉色（原型 .inp .snd），回信 / 评论这类免费动作用海蓝。
  final ChatSendTone sendTone;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final inset = MediaQuery.viewPaddingOf(context).bottom;
    final keyboard = MediaQuery.viewInsetsOf(context).bottom;
    final send = sendTone == ChatSendTone.brand ? c.brand : c.aqua;
    return Container(
      decoration: BoxDecoration(
        color: c.surface,
        border: Border(top: BorderSide(color: c.line2)),
      ),
      padding: EdgeInsets.fromLTRB(
        Dim.gutter,
        Dim.s2,
        Dim.gutter,
        // s3 + 安全区 + 键盘：相加不取大，否则手势导航机型上输入框骑在手势条上。
        Dim.s3 + inset + keyboard,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          if (onPickImage != null)
            _BarIcon(icon: Icons.image_outlined, onTap: onPickImage!),
          if (onGift != null)
            _BarIcon(icon: Icons.card_giftcard_rounded, onTap: onGift!),
          Expanded(
            child: Container(
              constraints: const BoxConstraints(minHeight: Dim.tap),
              padding: const EdgeInsets.symmetric(horizontal: Dim.s3),
              decoration: BoxDecoration(
                color: c.page,
                borderRadius: Dim.brPill,
              ),
              alignment: Alignment.centerLeft,
              child: TextField(
                controller: controller,
                focusNode: focusNode,
                minLines: 1,
                maxLines: 4,
                textInputAction: TextInputAction.send,
                onSubmitted: (_) => sending ? null : onSend(),
                style: TextStyle(fontSize: Dim.t3, color: c.ink, height: 1.4),
                decoration: InputDecoration(
                  isDense: true,
                  filled: false,
                  border: InputBorder.none,
                  enabledBorder: InputBorder.none,
                  focusedBorder: InputBorder.none,
                  hintText: hint,
                  hintStyle: TextStyle(fontSize: Dim.t3, color: c.ink3),
                  contentPadding: const EdgeInsets.symmetric(vertical: 11),
                ),
              ),
            ),
          ),
          const SizedBox(width: Dim.s2),
          GestureDetector(
            onTap: sending ? null : onSend,
            child: Container(
              key: const ValueKey('chat-send'),
              width: Dim.tap,
              height: Dim.tap,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: sending ? c.mist : send,
              ),
              child: sending
                  ? const Padding(
                      padding: EdgeInsets.all(13),
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : const Icon(
                      Icons.send_rounded,
                      size: 20,
                      color: Colors.white,
                    ),
            ),
          ),
        ],
      ),
    );
  }
}

enum ChatSendTone { brand, aqua }

class _BarIcon extends StatelessWidget {
  const _BarIcon({required this.icon, required this.onTap});

  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return InkResponse(
      onTap: onTap,
      radius: 24,
      child: SizedBox(
        width: Dim.tap,
        height: Dim.tap,
        child: Icon(icon, size: 28, color: c.ink2),
      ),
    );
  }
}

/// 文字气泡（原型 `.bub`）：最宽 76%、padding s2 s3、t3 lh1.55；
/// 对方 sea 底、右下角尖（r3 r3 r3 r1）；我方 brand 白字、左下角尖（r3 r3 r1 r3）。
/// [readLabel] 非空时在右下角用 t0 .8 显示「已读」。
class MessageBubble extends StatelessWidget {
  const MessageBubble({
    super.key,
    required this.mine,
    required this.text,
    this.readLabel,
  });

  final bool mine;
  final String text;
  final String? readLabel;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    // 左右对齐交给外层的 Row（mainAxisAlignment），这里只管宽度上限和外观。
    return ConstrainedBox(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * .76,
      ),
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: Dim.s3,
          vertical: Dim.s2,
        ),
        decoration: BoxDecoration(
          color: mine ? c.brand : c.sea,
          borderRadius: BorderRadius.only(
            topLeft: const Radius.circular(Dim.r3),
            topRight: const Radius.circular(Dim.r3),
            bottomLeft: Radius.circular(mine ? Dim.r3 : Dim.r1),
            bottomRight: Radius.circular(mine ? Dim.r1 : Dim.r3),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.end,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              text,
              style: TextStyle(
                fontSize: Dim.t3,
                height: 1.55,
                color: mine ? Colors.white : c.ink,
              ),
            ),
            if (readLabel != null)
              Padding(
                padding: const EdgeInsets.only(top: 2),
                child: Text(
                  readLabel!,
                  style: TextStyle(
                    fontSize: Dim.t0,
                    height: 1.3,
                    color: (mine ? Colors.white : c.ink3).withValues(alpha: .8),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// 居中系统提示（原型 `.sys`）：最宽 82%、t0 ink3、line2 底、padding 2.5px/s3 → 3/12。
class SystemPill extends StatelessWidget {
  const SystemPill({super.key, required this.text, this.icon});

  final String text;
  final IconData? icon;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Center(
      child: Container(
        constraints: BoxConstraints(
          maxWidth: MediaQuery.sizeOf(context).width * .82,
        ),
        padding: const EdgeInsets.symmetric(horizontal: Dim.s3, vertical: 3),
        decoration: BoxDecoration(color: c.line2, borderRadius: Dim.brPill),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (icon != null) ...[
              Icon(icon, size: 13, color: c.gold),
              const SizedBox(width: Dim.s1),
            ],
            Flexible(
              child: Text(
                text,
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: Dim.t0, color: c.ink3, height: 1.5),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 居中礼物条（原型 `.gift`）：coin .2 底、padding s2 s3、t2 ink2，右侧魅力值 t2/700 gold。
class GiftPill extends StatelessWidget {
  const GiftPill({
    super.key,
    required this.icon,
    required this.text,
    required this.charm,
  });

  final Widget icon;
  final String text;
  final String charm;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Center(
      child: Container(
        constraints: BoxConstraints(
          maxWidth: MediaQuery.sizeOf(context).width * .82,
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: Dim.s3,
          vertical: Dim.s2,
        ),
        decoration: BoxDecoration(
          color: c.coin.withValues(alpha: .2),
          borderRadius: Dim.brPill,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            icon,
            const SizedBox(width: Dim.s2),
            Flexible(
              child: Text(
                text,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: Dim.t2, color: c.ink2, height: 1.4),
              ),
            ),
            const SizedBox(width: Dim.s2),
            Text(
              charm,
              style: TextStyle(
                fontSize: Dim.t2,
                fontWeight: FontWeight.w700,
                color: c.gold,
                height: 1.4,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 图片消息（原型 `.imgmsg`）：120px → 178 宽、4:3、r 14px → 21，不套彩色气泡；
/// 已读角标半透明覆盖在右下。本地路径（刚选好还没上传）也能显示。
class ImageMessage extends StatelessWidget {
  const ImageMessage({
    super.key,
    required this.url,
    this.readLabel,
    this.onTap,
  });

  final String url;
  final String? readLabel;
  final VoidCallback? onTap;

  static const double width = 178;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final isLocal = url.isNotEmpty && !url.startsWith('http');
    Widget placeholder() => ColoredBox(color: c.mist.withValues(alpha: .4));
    final dpr = MediaQuery.devicePixelRatioOf(context);

    Widget img;
    if (url.isEmpty) {
      img = placeholder();
    } else if (isLocal) {
      img = Image.file(
        File(url),
        fit: BoxFit.cover,
        cacheWidth: (width * dpr).round(),
        errorBuilder: (_, _, _) => placeholder(),
      );
    } else {
      img = CachedNetworkImage(
        imageUrl: url,
        fit: BoxFit.cover,
        // 规范 §4.10：缩略图按显示宽 × DPR 解码，不解码原图。
        memCacheWidth: (width * dpr).round(),
        placeholder: (_, _) => placeholder(),
        errorWidget: (_, _, _) => placeholder(),
      );
    }

    return GestureDetector(
      onTap: onTap,
      child: SizedBox(
        width: width,
        child: AspectRatio(
          aspectRatio: 4 / 3,
          child: Container(
            clipBehavior: Clip.antiAlias,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(21),
              boxShadow: [
                BoxShadow(
                  color: const Color(0xFF081424).withValues(alpha: .2),
                  blurRadius: 18,
                  offset: const Offset(0, 6),
                ),
              ],
            ),
            child: Stack(
              fit: StackFit.expand,
              children: [
                img,
                if (readLabel != null)
                  Positioned(
                    right: 6,
                    bottom: 5,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 7,
                        vertical: 1,
                      ),
                      decoration: BoxDecoration(
                        color: const Color(0xFF081424).withValues(alpha: .42),
                        borderRadius: Dim.brPill,
                      ),
                      child: Text(
                        readLabel!,
                        style: const TextStyle(
                          fontSize: Dim.t0,
                          color: Colors.white,
                          height: 1.4,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
