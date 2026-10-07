import 'package:bottles/ui/widgets/design_canvas.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
      'scale follows width: 393 wide → 1.048, child at (100,100) lands at (104.8,104.8)',
      (t) async {
    t.view.devicePixelRatio = 1;
    t.view.physicalSize = const Size(393, 851);
    addTearDown(t.view.reset);
    await t.pumpWidget(const MaterialApp(
      home: DesignCanvas(children: [
        CanvasPositioned(
          left: 100,
          top: 100,
          width: 10,
          height: 10,
          child: SizedBox(key: Key('p')),
        ),
      ]),
    ));
    final r = t.getRect(find.byKey(const Key('p')));
    expect(r.left, closeTo(104.8, .1));
    expect(r.top, closeTo(104.8, .1));
    expect(r.width, closeTo(10.48, .1));
  });

  testWidgets('short screen clips the bottom instead of squashing', (t) async {
    t.view.devicePixelRatio = 1;
    t.view.physicalSize = const Size(375, 600);
    addTearDown(t.view.reset);
    await t.pumpWidget(const MaterialApp(
      home: DesignCanvas(children: [
        CanvasPositioned(
          left: 0,
          top: 700,
          width: 10,
          height: 10,
          child: SizedBox(key: Key('low')),
        ),
      ]),
    ));
    // 坐标不变（scale 1.0），只是被裁掉。
    expect(t.getRect(find.byKey(const Key('low'))).top, closeTo(700, .1));
    expect(t.takeException(), isNull);
  });

  testWidgets('extendBottom fills the gap below the canvas on tall screens',
      (t) async {
    t.view.devicePixelRatio = 1;
    t.view.physicalSize = const Size(375, 900);
    addTearDown(t.view.reset);
    await t.pumpWidget(const MaterialApp(
      home: DesignCanvas(
        designSize: Size(375, 500),
        extendBottom: ColoredBox(key: Key('ext'), color: Colors.blue),
        children: [],
      ),
    ));
    final r = t.getRect(find.byKey(const Key('ext')));
    expect(r.top, closeTo(500, .1));
    expect(r.bottom, closeTo(900, .1));
  });
}
