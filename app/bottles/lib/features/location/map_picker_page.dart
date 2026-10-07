import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';

import '../../core/design/tokens.dart';
import '../../l10n/app_localizations.dart';
import '../../core/platform/location.dart';
import '../../core/providers.dart';
import '../../domain/models/place.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/headers.dart';
import 'location_controller.dart';

/// M1 地图选点。
///
/// 返回值：选中的 [Place]；点「不显示地点」返回 `const _NoPlace()` 的语义——
/// 实际用 `Navigator.pop(context, _PickResult.none)` 区分「选了」「清空」「取消」三种。
Future<PickResult?> showMapPicker(BuildContext context, {Place? initial}) {
  return Navigator.of(context).push<PickResult>(
    MaterialPageRoute(builder: (_) => MapPickerPage(initial: initial)),
  );
}

/// 选点结果。
///
/// 三种情况必须分开，不能都用 null 表达：
///   - [place] 非空：选了一个地点
///   - [cleared] 为 true：显式选择「不显示地点」——这是一等操作，
///     和「取消」完全不同，前者要清掉已有地点，后者要保持原样
///   - 整个返回 null（没有 PickResult）：用户返回，什么都别改
class PickResult {
  const PickResult.picked(this.place) : cleared = false;
  const PickResult.cleared() : place = null, cleared = true;

  final Place? place;
  final bool cleared;
}

/// 类别：场景页（规范 §6）
/// 固定区：NavBar、底部地址行 + 两个按钮
/// 弹性区：地图（吃掉全部剩余高度）
/// 可滚动区：无
/// 键盘：无
class MapPickerPage extends ConsumerStatefulWidget {
  const MapPickerPage({super.key, this.initial});

  final Place? initial;

  @override
  ConsumerState<MapPickerPage> createState() => _MapPickerPageState();
}

class _MapPickerPageState extends ConsumerState<MapPickerPage> {
  /// 没有任何位置信息时的落点。孟买——首发市场，比 (0,0) 那片海合理得多。
  static const _fallback = LatLng(19.0760, 72.8777);

  GoogleMapController? _map;
  late LatLng _center;
  Place? _resolved;
  bool _resolving = false;

  /// 拖动地图时的防抖：手指还在动就不要发逆地理请求。
  /// 每次移动都请求，拖一次屏会打出几十个调用——Geocoding 是按量计费的。
  Timer? _debounce;

  @override
  void initState() {
    super.initState();
    final init = widget.initial;
    final fix = ref.read(locationControllerProvider).fix;
    _center = init != null
        ? LatLng(init.lat, init.lng)
        : (fix != null ? LatLng(fix.lat, fix.lng) : _fallback);
    _resolved = init;
    if (init == null) _resolve(_center);
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _map?.dispose();
    super.dispose();
  }

  Future<void> _resolve(LatLng at) async {
    setState(() => _resolving = true);
    final p = await ref.read(geoRepoProvider).regeo(at.latitude, at.longitude);
    if (!mounted) return;
    setState(() {
      _resolved = p;
      _resolving = false;
    });
  }

  void _onCameraMove(CameraPosition pos) {
    _center = pos.target;
    // 地名在拖动期间保持上一次的值并置灰，而不是清空——
    // 闪烁的空白比稍旧的地名更难看，也让人以为出错了。
    if (!_resolving) setState(() => _resolving = true);
  }

  void _onCameraIdle() {
    _debounce?.cancel();
    _debounce = Timer(
      const Duration(milliseconds: 400),
      () => _resolve(_center),
    );
  }

  Future<void> _backToMe() async {
    final fix = await ref.read(locationClientProvider).current();
    if (fix == null || !mounted) return;
    final target = LatLng(fix.lat, fix.lng);
    await _map?.animateCamera(CameraUpdate.newLatLng(target));
    // animateCamera 结束会触发 onCameraIdle，逆地理在那里发，这里不重复请求。
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);

    return Scaffold(
      // 原型 .nv「✕ 选择地点」：它是一个模态式的选择器，用关闭不用返回。
      appBar: NavBar(title: l.mapPickTitle, closeIcon: true),
      body: Column(
        children: [
          Expanded(
            child: Stack(
              alignment: Alignment.center,
              children: [
                GoogleMap(
                  initialCameraPosition: CameraPosition(
                    target: _center,
                    zoom: 15,
                  ),
                  onMapCreated: (m) => _map = m,
                  onCameraMove: _onCameraMove,
                  onCameraIdle: _onCameraIdle,
                  // 只显示用户自己。**绝不标注其他用户**——即使做模糊化，
                  // 多点采样仍可反推真实住址，这是同类产品真实出过事的攻击面。
                  markers: const {},
                  myLocationEnabled: true,
                  // 关掉自带按钮，用我们自己的（位置和视觉都要跟设计一致）。
                  myLocationButtonEnabled: false,
                  zoomControlsEnabled: false,
                  mapToolbarEnabled: false,
                ),

                // 大头针固定在屏幕中心，拖动的是地图。
                // 比「拖大头针」精度高得多——手指不会挡住目标点。
                IgnorePointer(
                  child: Padding(
                    // 针尖在图标底部，整体上移半个图标高度才能对准中心点。
                    padding: const EdgeInsets.only(bottom: 36),
                    child: Icon(Icons.place, size: 36, color: c.brand),
                  ),
                ),

                Positioned(
                  left: Dim.s3,
                  top: Dim.s3,
                  child: _Hint(text: l.mapDragHint),
                ),

                // 原型 .iconb：24px → 36 白圆，贴地图右下 s3。
                Positioned(
                  right: Dim.s3,
                  bottom: Dim.s3,
                  child: IconCircleButton(
                    icon: Icons.my_location_rounded,
                    onTap: _backToMe,
                  ),
                ),
              ],
            ),
          ),

          // 原型 .bd（padding-top s3）里的 .lst 卡：大头针图标 + 地名 + 城市，
          // 不是一条贴边的素色横幅。
          Padding(
            padding: const EdgeInsets.fromLTRB(
              Dim.gutter,
              Dim.s3,
              Dim.gutter,
              Dim.s4,
            ),
            child: ListGroup(
              children: [
                AnimatedOpacity(
                  duration: const Duration(milliseconds: 150),
                  opacity: _resolving ? .45 : 1,
                  child: ListRowItem(
                    leading: Icon(
                      Icons.place_outlined,
                      size: 22,
                      color: c.ink3,
                    ),
                    title: _resolved?.label ?? l.mapLocating,
                    subtitle: _resolved?.subtitle.isNotEmpty == true
                        ? _resolved!.subtitle
                        : null,
                    showChevron: false,
                    roomy: true,
                  ),
                ),
              ],
            ),
          ),

          // 原型 .foot：两枚整宽按钮，底部安全区由 BottomActionBar 处理。
          BottomActionBar(
            children: [
              AppButton(
                label: l.mapUseThis,
                // 逆地理还没回来也允许确认：坐标才是关键，地名只是显示。
                onTap: () => Navigator.of(context).pop(
                  PickResult.picked(
                    _resolved ??
                        Place(lat: _center.latitude, lng: _center.longitude),
                  ),
                ),
              ),
              AppButton(
                label: l.mapHidePlace,
                kind: BtnKind.oauth,
                onTap: () =>
                    Navigator.of(context).pop(const PickResult.cleared()),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _Hint extends StatelessWidget {
  const _Hint({required this.text});
  final String text;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: c.surface.withValues(alpha: .88),
        borderRadius: BorderRadius.circular(Dim.r1),
      ),
      child: Text(
        text,
        style: TextStyle(fontSize: Dim.t0, color: c.ink3),
      ),
    );
  }
}
