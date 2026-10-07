import 'dart:io';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';
import '../../domain/models/user.dart';

/// 头像环。
///
/// 金色 conic 环 + 内圈 sea 底，没有头像时兜底 emoji——**不出现破图**，
/// 这在匿名社交里是常态而非异常（大量用户就是不传头像）。
class AvatarRing extends StatelessWidget {
  const AvatarRing({
    super.key,
    this.avatar,
    this.seed = '',
    this.gender = Gender.secret,
    this.size = 44,
    this.online = false,
    this.onTap,
  });

  /// 绿点走 [UserBrief.presumedOnline]：后端还没有 presence 能力时，
  /// 用 `LastActiveAt` 做「最近活跃」降级，而不是一律显示离线。
  AvatarRing.of(
    UserBrief user, {
    super.key,
    this.size = 44,
    bool? online,
    this.onTap,
  }) : avatar = user.avatar,
       seed = user.id,
       gender = user.gender,
       online = online ?? user.presumedOnline;

  final String? avatar;
  final String seed;
  final Gender gender;
  final double size;
  final bool online;
  final VoidCallback? onTap;

  static const _pool = ['🙂', '👩', '🧑', '👧', '🧔', '🐱', '🦊', '🐼'];

  String get _fallbackEmoji {
    if (seed.isEmpty) {
      return switch (gender) {
        Gender.female => '👩',
        Gender.male => '🧑',
        Gender.secret => '🙂',
      };
    }
    final h = seed.codeUnits.fold<int>(0, (a, b) => (a + b) % _pool.length);
    return _pool[h];
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    // 原型 .ring：内圈 inset 2.5px → 4；在线点 10px → 15（sm 8px → 12），边 2px → 3（sm 2）。
    final inset = size * 0.09;
    final large = size >= 40;
    final dot = large ? 15.0 : 12.0;
    final dotBorder = large ? 3.0 : 2.0;

    Widget inner;
    final url = avatar;
    if (url != null && url.isNotEmpty) {
      final fallback = Center(
        child: Text(_fallbackEmoji, style: TextStyle(fontSize: size * 0.4)),
      );
      // 刚选好还没上传的头像是本地路径，不能交给 CachedNetworkImage。
      inner = ClipOval(
        child: url.startsWith('http')
            ? CachedNetworkImage(
                imageUrl: url,
                fit: BoxFit.cover,
                width: size,
                height: size,
                placeholder: (_, _) => ColoredBox(color: c.sea),
                errorWidget: (_, _, _) => fallback,
              )
            : Image.file(
                File(url),
                fit: BoxFit.cover,
                width: size,
                height: size,
                errorBuilder: (_, _, _) => fallback,
              ),
      );
    } else {
      inner = Center(
        child: Text(_fallbackEmoji, style: TextStyle(fontSize: size * 0.4)),
      );
    }

    return GestureDetector(
      onTap: onTap,
      child: SizedBox(
        width: size,
        height: size,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            // 金环
            Container(
              width: size,
              height: size,
              decoration: const BoxDecoration(
                shape: BoxShape.circle,
                gradient: SweepGradient(
                  startAngle: 3.49, // 200°
                  endAngle: 9.77,
                  colors: [
                    Color(0xFFFFDD93),
                    Color(0xFFFF9A62),
                    Color(0xFFFFDD93),
                    Color(0xFFE3A94F),
                  ],
                ),
              ),
            ),
            Positioned.fill(
              child: Padding(
                padding: EdgeInsets.all(inset),
                child: Container(
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: c.sea,
                    border: Border.all(color: c.surface, width: 1.5),
                  ),
                  clipBehavior: Clip.antiAlias,
                  child: inner,
                ),
              ),
            ),
            if (online)
              Positioned(
                right: 0,
                bottom: 0,
                child: Container(
                  key: const ValueKey('avatar-online-dot'),
                  width: dot,
                  height: dot,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: const Color(0xFF35C77E),
                    border: Border.all(color: c.surface, width: dotBorder),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// 整块铺满的用户大图（发现页卡片 / 用户主页顶部）。
///
/// 这两处以前**只画渐变 + 一个 emoji**，用户传了头像也看不见——
/// `Candidate.avatar` 一直有下发，纯粹是没被画出来。
/// 没有头像时仍回退到渐变 + emoji，不出现破图。
class UserPhoto extends StatelessWidget {
  const UserPhoto({super.key, required this.user, this.emojiSize = 92});

  final UserBrief user;
  final double emojiSize;

  static const _gradient = LinearGradient(
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
    colors: [Color(0xFFFFD9E2), Color(0xFFEAD8FF), Color(0xFFCDE9F4)],
    stops: [0, .54, 1],
  );

  @override
  Widget build(BuildContext context) {
    final url = user.avatar;
    final placeholder = Container(
      width: double.infinity,
      height: double.infinity,
      decoration: const BoxDecoration(gradient: _gradient),
      alignment: Alignment.center,
      child: Text(
        user.gender == Gender.male ? '🧑' : '👩',
        style: TextStyle(fontSize: emojiSize),
      ),
    );
    if (url == null || url.isEmpty) return placeholder;

    // 引导页刚选好、还没上传的头像是本地路径，交给 CachedNetworkImage 会直接报错。
    if (!url.startsWith('http')) {
      return Image.file(
        File(url),
        width: double.infinity,
        height: double.infinity,
        fit: BoxFit.cover,
        errorBuilder: (_, _, _) => placeholder,
      );
    }
    return CachedNetworkImage(
      imageUrl: url,
      width: double.infinity,
      height: double.infinity,
      fit: BoxFit.cover,
      placeholder: (_, _) => placeholder,
      errorWidget: (_, _, _) => placeholder,
    );
  }
}

/// 未读红点。白边取 `surface`，暗色下自动变深——不能写死白色。
class UnreadBadge extends StatelessWidget {
  const UnreadBadge({super.key, required this.count, this.dotOnly = false});

  final int count;
  final bool dotOnly;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    if (count <= 0) return const SizedBox.shrink();
    if (dotOnly) {
      return Container(
        width: 10,
        height: 10,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: c.brand,
          border: Border.all(color: c.surface, width: 1.5),
        ),
      );
    }
    return Container(
      constraints: const BoxConstraints(minWidth: 18),
      height: 18,
      padding: const EdgeInsets.symmetric(horizontal: 5),
      decoration: BoxDecoration(
        color: c.brand,
        borderRadius: Dim.brPill,
        border: Border.all(color: c.surface, width: 1.5),
      ),
      alignment: Alignment.center,
      child: Text(
        count > 99 ? '99+' : '$count',
        style: const TextStyle(
          fontSize: 10,
          height: 1,
          fontWeight: FontWeight.w800,
          color: Colors.white,
        ),
      ),
    );
  }
}
