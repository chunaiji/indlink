import 'package:bottles/features/chat/chat_list_page.dart';
import 'package:bottles/features/chat/chat_room_page.dart';
import 'package:bottles/features/me/relations_page.dart';
import 'package:flutter_test/flutter_test.dart';

import 'matrix.dart';

/// 私聊 / 关系系页面在六档机型上都不允许 overflow（规范 §8.2）。
/// 聊天室另加键盘变体：360 宽 + 1.2 字体 + 键盘 300 时输入栏仍可见（Review Focus 1）。
void main() {
  for (final cfg in layoutMatrix) {
    testWidgets('chat list fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const ChatListPage()));
    testWidgets('chat room fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const ChatRoomPage(conversationId: 'c1')));
    testWidgets(
      'chat room fits with keyboard: ${cfg.name}',
      (t) => pumpAt(
        t,
        cfg.withInsets(viewInsets: keyboard300, suffix: ' +kb'),
        const ChatRoomPage(conversationId: 'c1'),
      ),
    );
    testWidgets('relations fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const RelationsPage()));
  }
}
