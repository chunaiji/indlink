import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/chat_bubbles.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('bubble never exceeds 76% of the row', (t) async {
    t.view.devicePixelRatio = 1;
    t.view.physicalSize = const Size(360, 780);
    addTearDown(t.view.reset);
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      // 和聊天室一样放在 Row 里（宽度由内容决定，上限 76%）。
      home: Scaffold(
        body: Row(
          mainAxisAlignment: MainAxisAlignment.end,
          children: [
            Flexible(child: MessageBubble(mine: true, text: 'x' * 400)),
          ],
        ),
      ),
    ));
    expect(t.getSize(find.byType(MessageBubble)).width,
        lessThanOrEqualTo(360 * .76 + 1));
  });

  testWidgets('input bar field and send button are 44 tall', (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(
        body: ChatInputBar(
          controller: TextEditingController(),
          onSend: () {},
          onPickImage: () {},
        ),
      ),
    ));
    expect(t.getSize(find.byKey(const ValueKey('chat-send'))).height, 44);
  });

  testWidgets('image message is 178 wide with a 4:3 body', (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: const Scaffold(
        body: Align(
          alignment: Alignment.centerRight,
          child: ImageMessage(url: '', readLabel: '已读'),
        ),
      ),
    ));
    final s = t.getSize(find.byType(ImageMessage));
    expect(s.width, 178);
    expect(s.height, closeTo(178 * 3 / 4, 1));
  });
}
