import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../l10n/app_localizations.dart';
import '../../core/providers.dart';
import '../../domain/models/place.dart';
import '../../ui/widgets/cards.dart';
import 'location_controller.dart';
import 'map_picker_page.dart';

/// M2 地点行。发瓶（B2）与发动态（F2）**共用同一个组件**。
///
/// 拆成两份必然漂移——两边的地点语义、清除方式、降级路径完全一致，
/// 没有任何理由各写一遍。
///
/// 三态：
///   1. 已选地点 → 显示地名 + 一个清除按钮
///   2. 未选但有定位 → 显示「使用当前位置」，点一下即带上
///   3. 未选且无定位 → 显示「添加地点」，点进去仍可手选
///
/// 第 3 态很重要：**拒绝定位不等于不能带地点**。用户仍然可以在地图上
/// 手选一个位置，这是 Z6 里写的第二条降级路径。
class PlaceField extends ConsumerWidget {
  const PlaceField({super.key, required this.value, required this.onChanged});

  /// 当前选中的地点。null = 不带地点。
  final Place? value;

  /// 传 null 表示清除。
  final ValueChanged<Place?> onChanged;

  Future<void> _pick(BuildContext context, WidgetRef ref) async {
    final r = await showMapPicker(context, initial: value);
    if (r == null) return; // 用户返回，保持原样
    onChanged(r.cleared ? null : r.place);
  }

  Future<void> _useCurrent(WidgetRef ref) async {
    final fix = ref.read(locationControllerProvider).fix;
    if (fix == null) return;
    // 先用坐标立刻填上，地名异步补——让用户马上看到反馈，
    // 而不是点完之后盯着一个没反应的行等两秒。
    onChanged(Place(lat: fix.lat, lng: fix.lng));
    final p = await ref.read(geoRepoProvider).regeo(fix.lat, fix.lng);
    onChanged(p);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final hasFix = ref.watch(locationControllerProvider).hasFix;
    final v = value;

    if (v != null) {
      return ListGroup(
        children: [
          ListRowItem(
            title: v.label,
            subtitle: v.subtitle.isEmpty ? null : v.subtitle,
            leading: Icon(Icons.place, size: 20, color: c.brand),
            showChevron: false,
            trailing: InkResponse(
              onTap: () => onChanged(null),
              radius: 20,
              child: Icon(Icons.close_rounded, size: 18, color: c.ink3),
            ),
            onTap: () => _pick(context, ref),
          ),
        ],
      );
    }

    return ListGroup(
      children: [
        ListRowItem(
          title: hasFix ? l.placeUseCurrent : l.placeAdd,
          subtitle: hasFix ? l.placeUseCurrentSub : l.placeAddSub,
          leading: Icon(Icons.place_outlined, size: 20, color: c.ink3),
          // 有定位时主动作是「直接用当前位置」，长按/点箭头才进地图——
          // 但一个行只能有一个 onTap，所以：有定位就直接带上（最常见的选择），
          // 想换地方点进已选态里的那一行。无定位时只能进地图手选。
          onTap: hasFix ? () => _useCurrent(ref) : () => _pick(context, ref),
          trailing: hasFix
              ? InkResponse(
                  onTap: () => _pick(context, ref),
                  radius: 20,
                  child: Icon(Icons.map_outlined, size: 18, color: c.ink3),
                )
              : null,
          showChevron: !hasFix,
        ),
      ],
    );
  }
}
