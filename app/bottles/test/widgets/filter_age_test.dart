import 'package:bottles/features/discover/filter_page.dart';
import 'package:bottles/ui/widgets/chips.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// C2：年龄是一个值字段（原型 .fieldr「18 – 30 岁」），点开才出滑块。
void main() {
  testWidgets('age shows as a field and opens a range sheet', (t) async {
    await pumpAt(t, layoutMatrix[2], const FilterPage());
    expect(find.byType(RangeSlider), findsNothing);

    // 年龄在列表末尾，393×851 下在首屏之外；ListView 是懒构建的，得先滚到。
    await t.scrollUntilVisible(
      find.byType(ValueField),
      200,
      scrollable: find.byType(Scrollable).first,
    );
    await t.pumpAndSettle();
    expect(find.text('18 – 30'), findsOneWidget);

    await t.tap(find.byType(ValueField));
    await t.pumpAndSettle();
    expect(find.byType(RangeSlider), findsOneWidget);
  });
}
