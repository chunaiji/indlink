import 'package:bottles/core/design/tokens.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('shadows follow the prototype blur scaled by 1.488', () {
    // .swc: 0 10px 26px .18 → 0 15 39
    expect(Shadows.floating(AppColors.light).single.blurRadius, closeTo(39, 1));
    expect(Shadows.floating(AppColors.light).single.offset.dy, closeTo(15, 1));
    // .sheet: 0 -8px 28px .2 → 0 -12 42
    expect(Shadows.sheet().single.offset.dy, closeTo(-12, 1));
    // .cta glow: 0 5px 16px → 0 7 24
    final g = Shadows.glow(Colors.blue).single;
    expect(g.blurRadius, closeTo(24, 1));
    expect(g.offset.dy, closeTo(7, 1));
  });
}
