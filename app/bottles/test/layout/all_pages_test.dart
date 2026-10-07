// 一条命令跑全量机型矩阵（规范 §8.2）：
//   flutter test test/layout/all_pages_test.dart
// 各分节文件仍可单独跑，这里只是把它们串在一个入口里。
import 'auth_layout_test.dart' as auth;
import 'bottle_layout_test.dart' as bottle;
import 'chat_layout_test.dart' as chat;
import 'discover_layout_test.dart' as discover;
import 'me_layout_test.dart' as me;
import 'moment_layout_test.dart' as moment;

void main() {
  auth.main();
  bottle.main();
  chat.main();
  discover.main();
  me.main();
  moment.main();
}
