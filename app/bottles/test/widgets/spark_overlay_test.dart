// test/widgets/spark_overlay_test.dart
import 'dart:async';

import 'package:bottles/domain/models/spark.dart';
import 'package:bottles/features/spark/spark_overlay.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

SparkEvent _event({String? chatId}) => SparkEvent(
  chatId: chatId,
  peerId: '42',
  nickname: '小鱼',
  avatar: '',
  title: '有人和你对上眼了',
  body: '小鱼 与你碰撞出了火花',
);

void main() {
  testWidgets('shows the peer nickname and both buttons', (t) async {
    await pumpAt(t, layoutMatrix[2], SparkDialog(event: _event()));
    expect(find.textContaining('小鱼'), findsWidgets);
    expect(find.text('有人和你对上眼了'), findsOneWidget);
  });

  // 两条火花同时到(重连补推 / 两个事件挨着)时只弹一个,不叠弹窗也不排队——
  // 叠起来用户要连点两次才能回到原来的页面。
  testWidgets('a second spark while one is open is dropped', (t) async {
    await pumpAt(t, layoutMatrix[2], const _Host());
    final ctx = t.element(find.byType(_Host));
    // 不能 await:showSparkDialog 的 future 要等弹窗被关掉才 resolve,
    // await 它就是在等一个永远不来的点击。
    unawaited(showSparkDialog(ctx, _event()));
    await t.pump();
    unawaited(showSparkDialog(ctx, _event()));
    await t.pump();
    expect(find.byType(SparkDialog), findsOneWidget);

    // 关掉它:_sparkOpen 是全局的,挂着会把后续测试一起拖下水。
    Navigator.of(ctx).pop();
    await t.pumpAndSettle();
  });
}

class _Host extends StatelessWidget {
  const _Host();
  @override
  Widget build(BuildContext context) => const Scaffold(body: SizedBox());
}
