// test/spark_model_test.dart
import 'package:bottles/domain/models/spark.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const payload = {
    'type': 'spark',
    'peer_id': '42',
    'peer': {'id': '42', 'nickname': '小鱼', 'avatar': 'a.png'},
    'title': '有人和你对上眼了',
    'text': '小鱼 与你碰撞出了火花',
    'title_en': 'Someone caught your eye',
    'text_en': 'You and 小鱼 just sparked',
  };

  test('picks the copy matching the app locale', () {
    final zh = SparkEvent.fromJson(payload, english: false);
    expect(zh.title, '有人和你对上眼了');
    expect(zh.body, '小鱼 与你碰撞出了火花');
    final en = SparkEvent.fromJson(payload, english: true);
    expect(en.title, 'Someone caught your eye');
  });

  // 服务端那半没配文案时推空串,客户端必须回落到内置文案,不能显示空白弹窗
  test('falls back to built-in copy when the server sends empty strings', () {
    final e = SparkEvent.fromJson({
      ...payload,
      'title': '',
      'text': '',
      'title_en': '',
      'text_en': '',
    }, english: false);
    expect(e.title.isNotEmpty, isTrue);
    expect(e.body.contains('小鱼'), isTrue);
  });

  // 真人配对没有 chat_id:客户端据此决定是直接跳转还是先调 /spark/accept
  test('hasChat distinguishes bot match from real match', () {
    expect(SparkEvent.fromJson(payload, english: false).hasChat, isFalse);
    expect(
      SparkEvent.fromJson({...payload, 'chat_id': '777'}, english: false).hasChat,
      isTrue,
    );
  });
}
