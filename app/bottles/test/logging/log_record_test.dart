import 'dart:convert';

import 'package:bottles/core/logging/log_record.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final t = DateTime.utc(2026, 9, 19, 9, 23, 41, 882);

  test('序列化成一行 JSON，且确实只有一行', () {
    final r = LogRecord(
      time: t,
      level: LogLevel.error,
      tag: LogTag.net,
      message: 'POST /bottle/create',
      fields: {'http': 200, 'code': 3003, 'ms': 412},
    );
    final line = r.toJsonLine('a3f27b19');

    // 一行一条是整个存储格式的前提：清理时按行截断、查看页按行解析。
    // 消息里带换行也不能破坏这一点。
    expect(line.contains('\n'), isFalse);

    final m = jsonDecode(line) as Map<String, dynamic>;
    expect(m['lv'], 'error');
    expect(m['tag'], 'net');
    expect(m['msg'], 'POST /bottle/create');
    expect(m['sid'], 'a3f27b19');
    expect(m['http'], 200);
    expect(m['code'], 3003);
    expect(m['ms'], 412);
  });

  test('消息含换行时不会拆成两行', () {
    final r = LogRecord(
      time: t,
      level: LogLevel.warn,
      tag: LogTag.sys,
      message: '第一行\n第二行',
    );
    expect(r.toJsonLine('s').contains('\n'), isFalse);
  });

  test('fields 为空时不写空对象，异常与堆栈按需写入', () {
    final plain = LogRecord(
      time: t,
      level: LogLevel.info,
      tag: LogTag.biz,
      message: 'ok',
    ).toJsonLine('s');
    final m = jsonDecode(plain) as Map<String, dynamic>;
    expect(m.containsKey('err'), isFalse);
    expect(m.containsKey('st'), isFalse);

    final withErr = LogRecord(
      time: t,
      level: LogLevel.error,
      tag: LogTag.sys,
      message: 'boom',
      error: StateError('bad'),
      stack: StackTrace.fromString('#0 a\n#1 b'),
    ).toJsonLine('s');
    final m2 = jsonDecode(withErr) as Map<String, dynamic>;
    expect(m2['err'].toString(), contains('bad'));
    expect(m2['st'].toString(), contains('#0 a'));
  });

  test('级别顺序即严重度，用于按最低级别过滤', () {
    expect(LogLevel.debug.index < LogLevel.info.index, isTrue);
    expect(LogLevel.info.index < LogLevel.warn.index, isTrue);
    expect(LogLevel.warn.index < LogLevel.error.index, isTrue);
  });
}
