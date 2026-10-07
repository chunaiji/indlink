import 'package:bottles/app/nav_logger.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('数字段替换成占位，不把业务 ID 写进日志', () {
    expect(routePattern('/ocean/bottle/1938274650283'), '/ocean/bottle/:id');
    expect(routePattern('/discover/user/77'), '/discover/user/:id');
    expect(routePattern('/chats/1938274650283'), '/chats/:id');
  });

  test('没有 ID 的路径原样返回', () {
    expect(routePattern('/ocean'), '/ocean');
    expect(routePattern('/me/settings'), '/me/settings');
  });

  test('多个 ID 段都替换', () {
    expect(routePattern('/a/123/b/456'), '/a/:id/b/:id');
  });

  test('含数字的单词不误伤', () {
    // 'v1' 'sha256' 这类是路径的一部分，不是 ID。
    expect(routePattern('/api/v1/ping'), '/api/v1/ping');
  });

  test('空串与根路径', () {
    expect(routePattern(''), '');
    expect(routePattern('/'), '/');
  });
}
