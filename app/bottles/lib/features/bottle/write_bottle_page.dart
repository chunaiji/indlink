import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import '../../core/logging/log_record.dart';
import '../../core/logging/logger.dart';
import '../../core/catalog.dart';
import '../../core/design/tokens.dart';
import '../../domain/models/place.dart';
import '../location/place_field.dart';
import '../../core/media/image_pipeline.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/flows/purchase_flows.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import 'ocean_controller.dart';

/// B2 写瓶子。
///
/// 后端已挂 `throwLimit` 中间件，前端拿 quota 做预判即可。
/// ⚠️ 内容安全：微信 `msgSecCheck` 在 App 端完全失效，文本审核需换第三方，
/// 目前仅保留后端的本地敏感词库与防引流。
/// 类别：表单页（规范 §6）
/// 固定区：NavBar（含今日剩余）、底部「抛向大海」
/// 弹性区：正文卡（原型 .ta flex:1，内容不足一屏时吃掉剩余高度，最小 180）
/// 可滚动区：正文 + 图片 + 标签 + 范围 + 地点
/// 键盘：弹性区归零后整体可滚，底部按钮随 resizeToAvoidBottomInset 上移
class WriteBottlePage extends ConsumerStatefulWidget {
  const WriteBottlePage({super.key});

  @override
  ConsumerState<WriteBottlePage> createState() => _WriteBottlePageState();
}

class _WriteBottlePageState extends ConsumerState<WriteBottlePage> {
  static const _maxLength = 500;
  static const _maxImages = 3;

  final _controller = TextEditingController();
  final _tags = <String>{};
  final _images = <String>[];

  /// 「精确位置 / 仅城市」记住上次的选择，下次写瓶子不用再选。
  late bool _cityOnly = ref.read(prefsProvider).castCityOnly;

  Future<void> _setCityOnly(bool v) async {
    setState(() => _cityOnly = v);
    await ref.read(prefsProvider).setCastCityOnly(v);
  }

  Place? _place;
  bool _busy = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _pickImage() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery);
    if (picked == null) return;
    // 上传前先在客户端压一道：后端不压缩也不出缩略图。
    final compressed = await ImagePipeline.compress(picked.path);
    if (mounted) setState(() => _images.add(compressed.path));
  }

  Future<void> _cast() async {
    final l = L.of(context);
    final text = _controller.text.trim();
    if (text.isEmpty) {
      showToast(context, l.bottleNeedContent, error: true);
      return;
    }

    setState(() => _busy = true);
    try {
      await ref
          .read(bottleRepoProvider)
          .cast(
            content: text,
            images: _images,
            tags: _tags.toList(),
            cityOnly: _cityOnly,
            place: _place,
          );
      Log.i(LogTag.biz, 'throw', fields: {'place': _place != null});
      await ref.read(quotaProvider.notifier).refresh();
      ref.invalidate(oceanProvider);
      ref.invalidate(myBottlesProvider);
      // 瓶子已经落库，剩下的只是把它演出来——动画不参与成败判定。
      ref.read(castSignalProvider.notifier).fire();
      if (!mounted) return;
      context.pop();
      showToast(context, l.oceanCastDone);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      if (e.isQuotaExceeded) {
        final total = ref.read(quotaProvider).value?.throwTotal ?? 0;
        await showQuotaExceededModal(context, ref, usedUp: total);
      } else {
        showToast(context, e.message, error: true);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final quota = ref.watch(quotaProvider).value;

    return Scaffold(
      appBar: NavBar(
        title: l.bottleWriteTitle,
        closeIcon: true,
        subtitle: l.bottleThrowRemainToday(quota?.throwLeft ?? 0),
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
                    padding: const EdgeInsets.all(Dim.gutter),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        // 原型 .ta：padding s4、line 描边、r4、t3 lh1.75，计数在右下角；
                        // 它是本页的弹性区——内容不足一屏时吃掉剩余高度，超出后按内容高度滚。
                        Expanded(
                          child: Container(
                            constraints: const BoxConstraints(minHeight: 180),
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
                                  maxLines: null,
                                  minLines: 5,
                                  maxLength: _maxLength,
                                  onChanged: (_) => setState(() {}),
                                  style: TextStyle(
                                    fontSize: Dim.t3,
                                    height: 1.75,
                                    color: c.ink,
                                  ),
                                  decoration: InputDecoration(
                                    hintText: l.bottleContentHint,
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
                                  l.bottleCounter(
                                    _controller.text.characters.length,
                                    _maxLength,
                                  ),
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
                        const SizedBox(height: Dim.s3),
                        Row(
                          children: [
                            for (final path in _images) ...[
                              _Thumb(
                                path: path,
                                onRemove: () =>
                                    setState(() => _images.remove(path)),
                              ),
                              const SizedBox(width: Dim.s2),
                            ],
                            if (_images.length < _maxImages)
                              _AddThumb(onTap: _pickImage),
                          ],
                        ),
                        const SizedBox(height: Dim.s5),
                        SectionLabel(l.bottleTags),
                        const SizedBox(height: Dim.s3),
                        Wrap(
                          spacing: Dim.s2,
                          runSpacing: Dim.s2,
                          children: [
                            for (final key in Catalog.bottleTags.keys)
                              PillChip(
                                label: Catalog.bottleTag(context, key),
                                tone: key == 'night'
                                    ? ChipTone.pay
                                    : ChipTone.plain,
                                selected: _tags.contains(key),
                                onTap: () => setState(
                                  () => _tags.contains(key)
                                      ? _tags.remove(key)
                                      : _tags.add(key),
                                ),
                              ),
                          ],
                        ),
                        const SizedBox(height: Dim.s5),
                        SectionLabel(l.bottleRange),
                        const SizedBox(height: Dim.s3),
                        // Wrap 而不是 Row：窄屏 + 大字体下两枚胶囊放不下一行时要能换行。
                        Wrap(
                          spacing: Dim.s2,
                          runSpacing: Dim.s2,
                          children: [
                            PillChip(
                              label: l.bottleRangeNationwide,
                              selected: !_cityOnly,
                              onTap: () => _setCityOnly(false),
                            ),
                            PillChip(
                              label: l.bottleRangeCity,
                              selected: _cityOnly,
                              onTap: () => _setCityOnly(true),
                            ),
                          ],
                        ),
                        const SizedBox(height: Dim.s4),
                        SectionLabel(l.bottlePlaceSection),
                        const SizedBox(height: Dim.s3),
                        PlaceField(
                          value: _place,
                          onChanged: (p) => setState(() => _place = p),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
          BottomActionBar(
            child: AppButton(
              label: l.bottleCastToSea,
              loading: _busy,
              onTap: _busy ? null : _cast,
            ),
          ),
        ],
      ),
    );
  }
}

class _Thumb extends StatelessWidget {
  const _Thumb({required this.path, required this.onRemove});

  final String path;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Stack(
      clipBehavior: Clip.none,
      children: [
        Container(
          width: 65,
          height: 65,
          clipBehavior: Clip.antiAlias,
          decoration: BoxDecoration(color: c.sea, borderRadius: Dim.brButton),
          child: Image.file(File(path), fit: BoxFit.cover),
        ),
        Positioned(
          right: -6,
          top: -6,
          child: GestureDetector(
            onTap: onRemove,
            child: Container(
              width: 22,
              height: 22,
              decoration: BoxDecoration(
                color: c.ink.withValues(alpha: .7),
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.close_rounded,
                size: 14,
                color: Colors.white,
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _AddThumb extends StatelessWidget {
  const _AddThumb({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 65,
        height: 65,
        decoration: BoxDecoration(
          borderRadius: Dim.brButton,
          border: Border.all(color: c.line),
        ),
        child: Icon(Icons.add_rounded, color: c.ink3, size: 24),
      ),
    );
  }
}
