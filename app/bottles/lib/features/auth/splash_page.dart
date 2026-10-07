import 'dart:io';

import 'package:flutter_cache_manager/flutter_cache_manager.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../ui/widgets/sea_scene.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/design_canvas.dart';

/// A1 启动鉴权。
///
/// 类别：场景页（规范 §6）
/// 固定区：底部进度条（距底 max(安全区, s6)）
/// 弹性区：整屏品牌渐变画布，三个白色光斑按原型坐标放
/// 可滚动区：无
/// 键盘：无
///
/// secure storage 有 JWT → 静默校验 → 进 Ocean；无或 401 → 进登录。
/// 实际跳转由 `router` 的 redirect 守卫完成，这里只负责发起校验并稳住首屏。
class SplashPage extends ConsumerStatefulWidget {
  const SplashPage({super.key});

  @override
  ConsumerState<SplashPage> createState() => _SplashPageState();
}

class _SplashPageState extends ConsumerState<SplashPage> {
  /// 后台配的启动页图在本地缓存里的文件；没缓存（首次安装 / 还没下完）为 null，用内置启动页。
  File? _remote;

  @override
  void initState() {
    super.initState();
    // 启动即并发拉资料与开关，避免首屏二次布局抖动。
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(authProvider.notifier).bootstrap();
    });
    _loadRemoteSplash();
  }

  /// 只认**已经在缓存里**的图：启动页不等网络。多张按日轮播（同一天看到同一张）。
  Future<void> _loadRemoteSplash() async {
    final urls = ref.read(appConfigProvider).splashImages;
    if (urls.isEmpty) return;
    final url = urls[DateTime.now().day % urls.length];
    try {
      final info = await DefaultCacheManager().getFileFromCache(url);
      if (!mounted || info == null) return;
      setState(() => _remote = info.file);
    } catch (_) {
      // 缓存读失败就当没配。
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final bottomInset = MediaQuery.viewPaddingOf(context).bottom;

    final remote = _remote;
    if (remote != null) {
      // 运营配置的启动页：整图铺满，底部只留进度条。
      return Scaffold(
        body: Stack(
          fit: StackFit.expand,
          children: [
            Image.file(remote, fit: BoxFit.cover),
            _loader(bottomInset),
          ],
        ),
      );
    }

    return Scaffold(
      body: Stack(
        fit: StackFit.expand,
        children: [
          // 原型 .splash：全屏 --grad + 三个白色光斑（.18 / .13 / .11）。
          // 光斑按 375×762 坐标系放，随屏宽等比，不会在宽屏上跑到边上去。
          DesignCanvas(
            background: DecoratedBox(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: const Alignment(-0.9, -1),
                  end: const Alignment(0.9, 1),
                  colors: c.brandGradient,
                  stops: const [0, .52, 1],
                ),
              ),
            ),
            children: const [
              CanvasPositioned(
                left: 31,
                top: 132,
                width: 72,
                height: 72,
                child: _Glow(alpha: .18),
              ),
              CanvasPositioned(
                left: 257,
                top: 71,
                width: 101,
                height: 101,
                child: _Glow(alpha: .13),
              ),
              CanvasPositioned(
                left: 155,
                top: 570,
                width: 140,
                height: 140,
                child: _Glow(alpha: .11),
              ),
            ],
          ),
          // 品牌：瓶子 44px → 65、DRIFT t6/800 字距 .2em、标语 t2 白 .82。
          Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const BottleSprite(width: 96, tilt: -30),
                const SizedBox(height: Dim.s3),
                Text(
                  l.appName,
                  style: const TextStyle(
                    fontSize: Dim.t6,
                    fontWeight: FontWeight.w800,
                    color: Colors.white,
                    letterSpacing: 4.8,
                    height: 1.25,
                  ),
                ),
                const SizedBox(height: Dim.s2),
                Text(
                  l.appTagline,
                  style: TextStyle(
                    fontSize: Dim.t2,
                    height: 1.5,
                    color: Colors.white.withValues(alpha: .82),
                  ),
                ),
              ],
            ),
          ),
          _loader(bottomInset),
        ],
      ),
    );
  }

  /// 原型 .ldr：74px × 2px → 110 × 3，白 .3 轨 + 白条，距底 s6（加安全区）。
  Widget _loader(double bottomInset) => Positioned(
    left: 0,
    right: 0,
    bottom: bottomInset > Dim.s6 ? bottomInset : Dim.s6,
    child: Center(
      child: SizedBox(
        width: 110,
        height: 3,
        child: ClipRRect(
          borderRadius: Dim.brPill,
          child: LinearProgressIndicator(
            backgroundColor: Colors.white.withValues(alpha: .3),
            valueColor: const AlwaysStoppedAnimation(Colors.white),
          ),
        ),
      ),
    ),
  );
}

class _Glow extends StatelessWidget {
  const _Glow({required this.alpha});

  final double alpha;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: Colors.white.withValues(alpha: alpha),
      ),
    );
  }
}
