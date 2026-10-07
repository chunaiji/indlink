import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../../domain/models/chat.dart';

/// 礼物图标。
///
/// **必须用它，不要直接 `Text(gift.emoji)`。** 线上 `items.icon` 存的是
/// `https://…/static/gifts/shell.png` 这类图片地址，当文本画出来就是一长串 URL
/// 铺满界面——那正是「礼物页面乱码」的真身。
///
/// 两种形态都要支持：运营既可能配 emoji（省事），也可能配图片（好看）。
class GiftIcon extends StatelessWidget {
  const GiftIcon({super.key, required this.gift, this.size = 28});

  final GiftItem gift;
  final double size;

  @override
  Widget build(BuildContext context) {
    if (!gift.isImageIcon) {
      return Text(gift.emoji, style: TextStyle(fontSize: size));
    }
    return CachedNetworkImage(
      imageUrl: gift.emoji,
      width: size,
      height: size,
      fit: BoxFit.contain,
      // 加载中留住位置,否则整行会先塌一下再弹开。
      placeholder: (_, _) => SizedBox(width: size, height: size),
      // 图挂了退回 emoji,而不是显示破图或那串 URL。
      errorWidget: (_, _, _) => Text('🎁', style: TextStyle(fontSize: size)),
    );
  }
}
