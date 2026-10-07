import 'dart:io';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// 打开全屏图片预览。本地路径与远程 URL 都支持。
///
/// 聊天、动态等处共用——图片查看是通用能力,不该绑在某个页面里。
void openImageViewer(BuildContext context, String url) {
  Navigator.of(context).push(
    PageRouteBuilder<void>(
      opaque: false,
      barrierColor: Colors.black,
      transitionDuration: const Duration(milliseconds: 180),
      pageBuilder: (_, _, _) => _ImageViewerPage(url: url),
      transitionsBuilder: (_, anim, _, child) =>
          FadeTransition(opacity: anim, child: child),
    ),
  );
}

/// 全屏图片查看器:黑底、双指缩放 / 拖动,点击任意处或左上角关闭。
///
/// 规范 §4.8：全 App 锁竖屏，**只有这里**允许横屏看图——进入放开、退出恢复。
class _ImageViewerPage extends StatefulWidget {
  const _ImageViewerPage({required this.url});

  final String url;

  @override
  State<_ImageViewerPage> createState() => _ImageViewerPageState();
}

class _ImageViewerPageState extends State<_ImageViewerPage> {
  @override
  void initState() {
    super.initState();
    SystemChrome.setPreferredOrientations([
      DeviceOrientation.portraitUp,
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
    ]);
  }

  @override
  void dispose() {
    SystemChrome.setPreferredOrientations([DeviceOrientation.portraitUp]);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final url = widget.url;
    final isLocal = !url.startsWith('http');
    // 大图按屏幕宽 × DPR 解码上限（规范 §4.10），双指放大到 4x 仍够清晰，
    // 但不会把一张 4000px 的原图整张解进内存。
    final cacheWidth =
        (MediaQuery.sizeOf(context).longestSide *
                MediaQuery.devicePixelRatioOf(context))
            .round();
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: GestureDetector(
        onTap: () => Navigator.of(context).maybePop(),
        child: Stack(
          children: [
            Positioned.fill(
              child: InteractiveViewer(
                minScale: 1,
                maxScale: 4,
                child: Center(
                  child: isLocal
                      ? Image.file(File(url), cacheWidth: cacheWidth)
                      : CachedNetworkImage(
                          imageUrl: url,
                          fit: BoxFit.contain,
                          memCacheWidth: cacheWidth,
                          placeholder: (_, _) => const SizedBox(
                            width: 40,
                            height: 40,
                            child: Center(
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: Colors.white,
                              ),
                            ),
                          ),
                          errorWidget: (_, _, _) => const Icon(
                            Icons.broken_image_outlined,
                            color: Colors.white54,
                            size: 48,
                          ),
                        ),
                ),
              ),
            ),
            SafeArea(
              child: Align(
                alignment: Alignment.topLeft,
                child: IconButton(
                  icon: const Icon(Icons.close_rounded, color: Colors.white),
                  onPressed: () => Navigator.of(context).maybePop(),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
