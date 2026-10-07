import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import '../../core/design/tokens.dart';
import '../../domain/models/place.dart';
import '../location/place_field.dart';
import '../../core/media/image_pipeline.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/photo_grid.dart';
import '../../ui/widgets/overlays.dart';
import 'moment_controller.dart';

/// 发动态 + G1 媒体上传状态。
///
/// 选图交给系统选择器（`image_picker`），所以原型里独立的「选择图片」屏在这里
/// 收敛成本页内的九宫格 + 压缩/上传进度——独立一屏会和系统选择器重复。
///
/// ⚠️ 后端 `upload` 目前落**本地磁盘**、`/static` 裸服务，无压缩、无缩略图、无 CDN。
/// 客户端压缩（flutter_image_compress）与对象存储 + 海外 CDN 是上线前的必做项：
/// 对印度用户，图片走中国服务器的本地磁盘是体验硬伤。
/// 类别：表单页（规范 §6）
/// 固定区：NavBar（右侧「发布」文字动作，原型 .nv .sp2；没有底部按钮）
/// 弹性区：正文卡（原型 .ta flex:1，内容不足一屏时吃掉剩余高度，最小 160）
/// 可滚动区：正文 + 九宫格 + 上传进度 + 地点 + 谁可以看
/// 键盘：弹性区归零后整体可滚
class PostMomentPage extends ConsumerStatefulWidget {
  const PostMomentPage({super.key});

  @override
  ConsumerState<PostMomentPage> createState() => _PostMomentPageState();
}

class _UploadItem {
  _UploadItem(this.sourcePath);

  /// 相册里选中的原图。
  final String sourcePath;

  /// 压缩产物。压缩完成前为 null。
  CompressedImage? compressed;

  bool compressing = true;
  double progress = 0;
  bool failed = false;

  /// 上传成功后的**服务端 URL**。发动态时提交的是它，不是本地路径。
  String? remoteURL;

  bool get done => !compressing && !failed && remoteURL != null;
}

class _PostMomentPageState extends ConsumerState<PostMomentPage> {
  static const _maxImages = 9;

  final _controller = TextEditingController();
  final _items = <_UploadItem>[];
  final _timers = <Timer>[];
  bool _busy = false;
  Place? _place;

  /// `public` / `self`——后端只认这两个（原型 F2 的「仅好友」没有对应字段，不露出）。
  String _visible = 'public';

  @override
  void dispose() {
    for (final t in _timers) {
      t.cancel();
    }
    _controller.dispose();
    super.dispose();
  }

  Future<void> _pick() async {
    final picked = await ImagePicker().pickMultiImage(
      limit: _maxImages - _items.length,
    );
    if (picked.isEmpty) return;

    final fresh = <_UploadItem>[];
    setState(() {
      for (final x in picked.take(_maxImages - _items.length)) {
        final item = _UploadItem(x.path);
        _items.add(item);
        fresh.add(item);
      }
    });

    // 压缩 → 真实上传。逐张串行:一次并发九张会把弱网直接打满,
    // 而用户本来就要等,并发省下的时间还不如一张传失败重来的代价大。
    for (final item in fresh) {
      final result = await ImagePipeline.compress(item.sourcePath);
      if (!mounted) return;
      setState(() {
        item.compressed = result;
        item.compressing = false;
      });
      await _upload(item);
    }
  }

  /// 真实上传。成功后 item 才算 done，发动态提交的是返回的 URL。
  /// Z8：点失败的那一格重传这一张（整批重试在进度卡上）。
  Future<void> _retryOne(_UploadItem item) async {
    if (!item.failed) return;
    setState(() => item.failed = false);
    await _upload(item);
  }

  Future<void> _upload(_UploadItem item) async {
    final path = item.compressed?.path ?? item.sourcePath;
    // 进度条这里只有"传输中/完成"两态：后端 upload 是一次性 multipart，
    // 拿不到细粒度进度。与其画一根假的匀速条，不如让它停在中间等真结果。
    setState(() => item.progress = 0.35);
    try {
      final url = await ref.read(momentRepoProvider).uploadImage(path);
      if (!mounted) return;
      setState(() {
        item.remoteURL = url;
        item.progress = 1;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        item.failed = true;
        item.progress = 0;
      });
    }
  }

  /// 整组图片的压缩收益，显示在上传卡上。
  (int, int) get _sizes {
    var original = 0;
    var compressed = 0;
    for (final i in _items) {
      original += i.compressed?.originalBytes ?? 0;
      compressed += i.compressed?.compressedBytes ?? 0;
    }
    return (original, compressed);
  }

  Future<void> _post() async {
    final text = _controller.text.trim();
    if (text.isEmpty) {
      showToast(context, L.of(context).bottleNeedContent, error: true);
      return;
    }
    setState(() => _busy = true);
    try {
      final moment = await ref
          .read(momentRepoProvider)
          .post(
            text: text,
            images: [for (final i in _items.where((e) => e.done)) i.remoteURL!],
            place: _place,
            visible: _visible,
          );
      ref.read(momentFeedProvider.notifier).prepend(moment);
      if (!mounted) return;
      context.pop();
      showToast(context, L.of(context).momentPosted);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      showToast(context, e.message, error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final uploading = _items.where((e) => !e.done && !e.failed).length;
    final failed = _items.where((e) => e.failed).length;

    final canPost = !_busy && uploading == 0;

    return Scaffold(
      appBar: NavBar(
        title: l.momentPostTitle,
        closeIcon: true,
        actions: [
          // 原型 .nv .sp2「发布」：brand 色 800 字重的文字动作，不是底部大按钮。
          TextButton(
            onPressed: canPost ? _post : null,
            child: _busy
                ? SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: c.brand,
                    ),
                  )
                : Text(
                    l.momentPost,
                    style: TextStyle(
                      fontSize: Dim.t3,
                      fontWeight: FontWeight.w800,
                      color: canPost ? c.brand : c.ink3.withValues(alpha: .5),
                    ),
                  ),
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: CustomScrollView(
              keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
              slivers: [
                SliverFillRemaining(
                  hasScrollBody: false,
                  child: Padding(
                    // 没有底部按钮，手势条的安全区由这里的下边距吃掉。
                    padding: EdgeInsets.fromLTRB(
                      Dim.gutter,
                      Dim.gutter,
                      Dim.gutter,
                      Dim.gutter + MediaQuery.viewPaddingOf(context).bottom,
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        // 原型 .ta：描边卡 + 右下计数，是本页的弹性区（同 B2）。
                        Expanded(
                          child: Container(
                            constraints: const BoxConstraints(minHeight: 160),
                            padding: const EdgeInsets.all(Dim.s4),
                            decoration: BoxDecoration(
                              color: c.surface,
                              borderRadius: Dim.brLargeCard,
                              border: Border.all(color: c.line),
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.end,
                              children: [
                                TextField(
                                  controller: _controller,
                                  minLines: 4,
                                  maxLines: null,
                                  maxLength: 500,
                                  onChanged: (_) => setState(() {}),
                                  style: TextStyle(
                                    fontSize: Dim.t3,
                                    height: 1.7,
                                    color: c.ink,
                                  ),
                                  decoration: InputDecoration(
                                    hintText: l.momentPostHint,
                                    filled: false,
                                    border: InputBorder.none,
                                    enabledBorder: InputBorder.none,
                                    focusedBorder: InputBorder.none,
                                    contentPadding: EdgeInsets.zero,
                                    counterText: '',
                                  ),
                                ),
                                const Spacer(),
                                Text(
                                  '${_controller.text.characters.length} / 500',
                                  style: TextStyle(
                                    fontSize: Dim.t1,
                                    color: c.ink3,
                                    height: 1.4,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                        const SizedBox(height: Dim.s4),
                        // 原型 .g9：3 列 gap s2 r2，末尾虚线「+」。
                        PhotoGrid(
                          count: _items.length,
                          max: _maxImages,
                          onAdd: _pick,
                          itemBuilder: (context, i) => _Tile(
                            item: _items[i],
                            onRetry: () => _retryOne(_items[i]),
                          ),
                        ),
                        if (_items.isNotEmpty) ...[
                          const SizedBox(height: Dim.s3),
                          AppCard(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.stretch,
                              children: [
                                if (_sizes.$1 > 0) ...[
                                  Row(
                                    children: [
                                      Text(
                                        l.mediaOriginalSize(
                                          CompressedImage.formatSize(_sizes.$1),
                                        ),
                                        style: TextStyle(
                                          fontSize: Dim.t2,
                                          color: c.ink2,
                                        ),
                                      ),
                                      const Spacer(),
                                      Text(
                                        l.mediaCompressedSize(
                                          CompressedImage.formatSize(_sizes.$2),
                                        ),
                                        style: TextStyle(
                                          fontSize: Dim.t2,
                                          fontWeight: FontWeight.w700,
                                          color: c.ink,
                                        ),
                                      ),
                                    ],
                                  ),
                                  const SizedBox(height: Dim.s2),
                                ],
                                Row(
                                  children: [
                                    Text(
                                      uploading > 0
                                          ? l.mediaUploadingOf(
                                              _items.length - uploading,
                                              _items.length,
                                            )
                                          : l.commonDone,
                                      style: TextStyle(
                                        fontSize: Dim.t1,
                                        color: c.ink3,
                                      ),
                                    ),
                                    const Spacer(),
                                    if (failed > 0)
                                      GestureDetector(
                                        // 重试要真的重传。先清掉 failed 再逐张走一遍上传,
                                        // 串行同选图时一致:失败往往就是网差,并发重试只会更糟。
                                        onTap: () async {
                                          final retry = _items
                                              .where((e) => e.failed)
                                              .toList();
                                          setState(() {
                                            for (final i in retry) {
                                              i.failed = false;
                                            }
                                          });
                                          for (final i in retry) {
                                            await _upload(i);
                                          }
                                        },
                                        child: Text(
                                          l.mediaRetryFailed(failed),
                                          style: TextStyle(
                                            fontSize: Dim.t1,
                                            fontWeight: FontWeight.w700,
                                            color: c.brand,
                                          ),
                                        ),
                                      ),
                                  ],
                                ),
                                const SizedBox(height: Dim.s2),
                                ThinProgressBar(
                                  value: _items.isEmpty
                                      ? 0
                                      : _items
                                                .map((e) => e.progress)
                                                .reduce((a, b) => a + b) /
                                            _items.length,
                                ),
                              ],
                            ),
                          ),
                        ],
                        const SizedBox(height: Dim.s4),
                        SectionLabel(l.bottlePlaceSection),
                        const SizedBox(height: Dim.s2),
                        PlaceField(
                          value: _place,
                          onChanged: (p) => setState(() => _place = p),
                        ),
                        const SizedBox(height: Dim.s4),
                        SectionLabel(l.momentWhoCanSee),
                        const SizedBox(height: Dim.s2),
                        Wrap(
                          spacing: Dim.s2,
                          runSpacing: Dim.s2,
                          children: [
                            for (final (value, label) in [
                              ('public', l.momentVisPublic),
                              ('self', l.momentVisSelf),
                            ])
                              PillChip(
                                label: label,
                                selected: _visible == value,
                                onTap: () => setState(() => _visible = value),
                              ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Tile extends StatelessWidget {
  const _Tile({required this.item, required this.onRetry});

  final _UploadItem item;

  /// 失败态点格子重传（原型 Z8：遮罩上的「重试」要真的能点）。
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    return Stack(
      fit: StackFit.expand,
      children: [
        Image.file(
          File(item.sourcePath),
          fit: BoxFit.cover,
          cacheWidth:
              (MediaQuery.sizeOf(context).width /
                      3 *
                      MediaQuery.devicePixelRatioOf(context))
                  .round(),
        ),
        if (!item.done)
          PhotoTileOverlay(
            onTap: item.failed ? onRetry : null,
            label: item.failed
                ? l.commonRetry
                : item.compressing
                ? l.mediaCompressingSimple
                : l.mediaUploadingPercent((item.progress * 100).round()),
          ),
        if (item.done)
          Positioned(
            right: 4,
            bottom: 4,
            child: Icon(Icons.check_circle_rounded, size: 16, color: c.success),
          ),
      ],
    );
  }
}
