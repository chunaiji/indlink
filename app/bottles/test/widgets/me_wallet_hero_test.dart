import 'package:bottles/core/design/tokens.dart';
import 'package:bottles/features/me/me_page.dart';
import 'package:bottles/ui/widgets/cards.dart';
import 'package:bottles/ui/widgets/headers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// H1：钱包卡上探 39 压在头图预留的底边距上。真机上它曾盖住关注 / 粉丝 / 魅力那行——
/// 之前按 bottom: 0 定位，上探量变成了「卡高 - 39」。
void main() {
  testWidgets('wallet hero overlaps the header by 39 and nothing else', (
    t,
  ) async {
    await pumpAt(t, layoutMatrix[2], const MePage());
    final header = t.getRect(find.byType(GradientHeader));
    final stats = t.getRect(find.byType(HeaderStats));
    final hero = t.getRect(find.byKey(const ValueKey('wallet-hero')));
    final quad = t.getRect(find.byType(QuadGrid));

    expect(header.bottom - hero.top, closeTo(39, .5));
    expect(stats.bottom, lessThanOrEqualTo(hero.top));
    expect(quad.top, greaterThanOrEqualTo(hero.bottom + Dim.s4));
  });
}
