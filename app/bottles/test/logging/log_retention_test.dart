import 'package:bottles/core/logging/log_retention.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final now = DateTime(2026, 9, 19);
  const mb = 1024 * 1024;

  LogFileInfo f(String name, int bytes, int daysAgo) => LogFileInfo(
        name: name,
        bytes: bytes,
        day: now.subtract(Duration(days: daysAgo)),
      );

  test('超过保留天数的整个删掉', () {
    final del = planCleanup(
      [f('2026-09-19.log', mb, 0), f('2026-08-01.log', mb, 49)],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-09-19.log',
    );
    expect(del, ['2026-08-01.log']);
  });

  test('天数都没超但总量超了，从最旧的开始删到降下来为止', () {
    final del = planCleanup(
      [
        f('2026-09-19.log', 40 * mb, 0),
        f('2026-09-18.log', 40 * mb, 1),
        f('2026-09-17.log', 40 * mb, 2),
      ],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-09-19.log',
    );
    // 120MB 超了 100MB，删最旧的一个降到 80MB 即可，不该多删。
    expect(del, ['2026-09-17.log']);
  });

  test('正在写入的文件永远不删，哪怕它自己就超了总量', () {
    // 这是最危险的边界：边写边删会损坏文件。
    // 实际很难遇到（需要当天日志就超过总量上限），但遇到就是数据损坏。
    final del = planCleanup(
      [f('2026-09-19.log', 200 * mb, 0)],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-09-19.log',
    );
    expect(del, isEmpty);
  });

  test('活跃文件超龄也不删', () {
    final del = planCleanup(
      [f('2026-08-01.log', mb, 49)],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-08-01.log',
    );
    expect(del, isEmpty);
  });

  test('什么都没超时不删任何东西', () {
    final del = planCleanup(
      [f('2026-09-19.log', mb, 0), f('2026-09-18.log', mb, 1)],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-09-19.log',
    );
    expect(del, isEmpty);
  });

  test('空列表不炸', () {
    expect(
      planCleanup([],
          maxTotalBytes: 100 * mb,
          retainDays: 31,
          now: now,
          activeFile: 'x'),
      isEmpty,
    );
  });
}
