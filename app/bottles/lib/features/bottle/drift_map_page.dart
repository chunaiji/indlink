import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:gal/gal.dart';
import 'package:path_provider/path_provider.dart';
import 'package:share_plus/share_plus.dart';

import '../../core/design/tokens.dart';
import '../../domain/models/bottle.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/states.dart';
import 'my_bottles_page.dart' show traceItems;
import 'ocean_controller.dart';

/// B4s 漂流海图（可分享）。
///
/// 这是全 App **唯一值得截图外发**的东西。「被 42 个人看到，跨越 3 座城市」的数据
/// B4 早就有了，但藏在二级页的一行小字里，等于没有。
///
/// 渲染走客户端本地合成（`RepaintBoundary` + `toImage`），**不走服务端出图**——
/// 多一次网络往返，分享意愿掉一半。
/// 类别：详情页（规范 §6）
/// 固定区：NavBar、底部「保存 / 分享」
/// 弹性区：无
/// 可滚动区：海报卡 + 隐私提示
/// 键盘：无
class DriftMapPage extends ConsumerStatefulWidget {
  const DriftMapPage({super.key, required this.bottleId});

  final String bottleId;

  @override
  ConsumerState<DriftMapPage> createState() => _DriftMapPageState();
}

class _DriftMapPageState extends ConsumerState<DriftMapPage> {
  final _posterKey = GlobalKey();
  bool _busy = false;

  /// 本地合成海报位图（`RepaintBoundary` → PNG 字节）。
  Future<Uint8List?> _renderPoster() async {
    final boundary =
        _posterKey.currentContext?.findRenderObject() as RenderRepaintBoundary?;
    if (boundary == null) return null;
    final image = await boundary.toImage(pixelRatio: 3);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      return data?.buffer.asUint8List();
    } finally {
      image.dispose();
    }
  }

  Future<void> _save() async {
    setState(() => _busy = true);
    try {
      final bytes = await _renderPoster();
      if (bytes == null) {
        if (mounted) {
          showToast(context, L.of(context).stateLoadFailed, error: true);
        }
        return;
      }
      if (!await Gal.hasAccess()) await Gal.requestAccess();
      await Gal.putImageBytes(bytes, name: 'drift-${widget.bottleId}');
      if (mounted) showToast(context, L.of(context).bottleMapSaved);
    } on GalException catch (e) {
      if (mounted) showToast(context, e.type.message, error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _share() async {
    setState(() => _busy = true);
    try {
      final bytes = await _renderPoster();
      if (bytes == null) {
        if (mounted) {
          showToast(context, L.of(context).stateLoadFailed, error: true);
        }
        return;
      }
      // 系统分享面板要的是文件，先落到临时目录。
      final dir = await getTemporaryDirectory();
      final file = File('${dir.path}/drift-${widget.bottleId}.png');
      await file.writeAsBytes(bytes);

      await SharePlus.instance.share(
        ShareParams(
          files: [XFile(file.path, mimeType: 'image/png')],
          // 水印在图里，文案里再带一次域名，回流入口不止一个。
          text: 'DRIFT · ambertu.com',
        ),
      );
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final detail = ref.watch(bottleDetailProvider(widget.bottleId));

    return Scaffold(
      appBar: NavBar(title: l.bottleMapTitle),
      body: AsyncView(
        value: detail,
        onRetry: () => ref.invalidate(bottleDetailProvider(widget.bottleId)),
        errorTitle: l.stateErrorTitle,
        errorBody: l.stateErrorBody,
        retryLabel: l.commonRetry,
        skeleton: const Padding(
          padding: EdgeInsets.all(Dim.gutter),
          child: Skeleton(height: 320, radius: Dim.r4),
        ),
        data: (bottle) => Column(
          children: [
            Expanded(
              child: ListView(
                padding: const EdgeInsets.all(Dim.gutter),
                children: [
                  RepaintBoundary(
                    key: _posterKey,
                    child: _Poster(bottle: bottle),
                  ),
                  const SizedBox(height: Dim.s3),
                  // 回信是匿名的，分享出去等于泄露。
                  NoticeBanner(
                    icon: Icons.lock_outline_rounded,
                    text: l.bottleMapPrivacy,
                  ),
                ],
              ),
            ),
            BottomActionBar(
              child: Row(
                children: [
                  // 原型 .foot 两枚并排：保存（描边）约 4 份、分享（实心）约 5 份。
                  Expanded(
                    flex: 4,
                    child: AppButton(
                      label: l.bottleMapSave,
                      kind: BtnKind.oauth,
                      loading: _busy,
                      onTap: _busy ? null : _save,
                    ),
                  ),
                  const SizedBox(width: Dim.s2),
                  Expanded(
                    flex: 5,
                    child: AppButton(
                      label: l.bottleMapShare,
                      icon: Icons.ios_share_rounded,
                      onTap: _busy ? null : _share,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Poster extends StatelessWidget {
  const _Poster({required this.bottle});

  final Bottle bottle;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final origin = bottle.trace.isNotEmpty ? bottle.trace.first : null;

    // 原型 .poster：padding s4、r4、深海渐变 + 星点、0 12 32 .3 投影。
    return Container(
      clipBehavior: Clip.antiAlias,
      padding: const EdgeInsets.all(Dim.s4),
      decoration: BoxDecoration(
        borderRadius: Dim.brLargeCard,
        gradient: const LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            Color(0xFF0D1436),
            Color(0xFF1A2456),
            Color(0xFF1E3E68),
            Color(0xFF26586F),
          ],
          stops: [0, .34, .66, 1],
        ),
        boxShadow: [
          BoxShadow(
            color: const Color(0xFF0A1630).withValues(alpha: .3),
            blurRadius: 32,
            offset: const Offset(0, 12),
          ),
        ],
      ),
      child: Stack(
        children: [
          const Positioned.fill(child: _PosterStars()),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                l.bottleMapPosterTitle,
                style: const TextStyle(
                  fontSize: Dim.t5,
                  fontWeight: FontWeight.w800,
                  color: Colors.white,
                  letterSpacing: -0.2,
                ),
              ),
              const SizedBox(height: Dim.s1),
              Text(
                l.bottleMapDeparted(
                  _date(bottle.createdAt),
                  origin?.city ?? bottle.city ?? '',
                ),
                style: TextStyle(
                  fontSize: Dim.t1,
                  color: Colors.white.withValues(alpha: .6),
                ),
              ),
              const SizedBox(height: Dim.s4),
              TimelineList(items: traceItems(context, bottle), onDark: true),
              const SizedBox(height: Dim.s3),
              SizedBox(
                height: 26,
                child: CustomPaint(
                  size: const Size(double.infinity, 26),
                  painter: _WavePainter(),
                ),
              ),
              const SizedBox(height: Dim.s3),
              Row(
                children: [
                  _PosterStat(
                    value: '${bottle.viewCount}',
                    label: l.bottleMapStatPeople,
                  ),
                  _PosterStat(
                    value: '${bottle.cityCount}',
                    label: l.bottleMapStatCities,
                  ),
                  _PosterStat(
                    value: '${bottle.driftedDays}',
                    label: l.bottleMapStatDays,
                  ),
                ],
              ),
              const SizedBox(height: Dim.s4),
              Container(
                padding: const EdgeInsets.only(left: Dim.s3),
                decoration: BoxDecoration(
                  border: Border(
                    left: BorderSide(
                      color: Colors.white.withValues(alpha: .35),
                      width: 2,
                    ),
                  ),
                ),
                // .poster .quote：t3 lh1.7 白 .92，左线 2 白 .35。
                child: Text(
                  bottle.content,
                  style: TextStyle(
                    fontSize: Dim.t3,
                    height: 1.7,
                    color: Colors.white.withValues(alpha: .92),
                  ),
                ),
              ),
              const SizedBox(height: Dim.s4),
              // 没有水印的分享图等于白送：这是自传播的回流入口。
              Row(
                children: [
                  // .poster .wm2：t1/800 字距 .16em 白 .5；右侧域名同色常规。
                  Text(
                    'DRIFT',
                    style: TextStyle(
                      fontSize: Dim.t1,
                      fontWeight: FontWeight.w800,
                      color: Colors.white.withValues(alpha: .5),
                      letterSpacing: 1.9,
                    ),
                  ),
                  const Spacer(),
                  Text(
                    'ambertu.com',
                    style: TextStyle(
                      fontSize: Dim.t1,
                      color: Colors.white.withValues(alpha: .5),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    );
  }

  static String _date(DateTime utc) {
    final d = utc.toLocal();
    return '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';
  }
}

class _PosterStat extends StatelessWidget {
  const _PosterStat({required this.value, required this.label});

  final String value;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            value,
            style: const TextStyle(
              fontSize: Dim.t6,
              fontWeight: FontWeight.w800,
              color: Colors.white,
              height: 1.1,
            ),
          ),
          Text(
            label,
            style: TextStyle(
              fontSize: Dim.t0,
              color: Colors.white.withValues(alpha: .55),
            ),
          ),
        ],
      ),
    );
  }
}

class _PosterStars extends StatelessWidget {
  const _PosterStars();

  @override
  Widget build(BuildContext context) => CustomPaint(painter: _StarsPainter());
}

class _StarsPainter extends CustomPainter {
  static const _pts = [
    (0.22, 0.10, 1.5, .9),
    (0.68, 0.07, 1.3, .75),
    (0.45, 0.17, 1.1, .5),
    (0.88, 0.14, 1.4, .8),
    (0.12, 0.22, 1.0, .45),
  ];

  @override
  void paint(Canvas canvas, Size size) {
    for (final (fx, fy, r, a) in _pts) {
      canvas.drawCircle(
        Offset(size.width * fx, size.height * fy),
        r,
        Paint()..color = Colors.white.withValues(alpha: a),
      );
    }
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

class _WavePainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    void wave(
      double baseY,
      double amp,
      double step,
      double alpha,
      double width,
    ) {
      final path = Path()..moveTo(0, baseY);
      for (var x = 0.0; x < size.width; x += step) {
        path.relativeQuadraticBezierTo(step / 2, -amp, step, 0);
        path.relativeQuadraticBezierTo(step / 2, amp, step, 0);
      }
      canvas.drawPath(
        path,
        Paint()
          ..style = PaintingStyle.stroke
          ..strokeWidth = width
          ..strokeCap = StrokeCap.round
          ..color = Colors.white.withValues(alpha: alpha),
      );
    }

    wave(10, 7, 13, 1, 1.6);
    wave(20, 6, 12, .5, 1.3);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
