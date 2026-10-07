import 'package:bottles/core/logging/log_config.dart';
import 'package:bottles/core/logging/log_record.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('默认值：开启、info、不记 body、100MB、31 天', () {
    const c = LogConfig.defaults();
    expect(c.enabled, isTrue);
    expect(c.minLevel, LogLevel.info);
    expect(c.logBody, isFalse); // 默认不记 body 是安全前提，不能改
    expect(c.maxMB, 100);
    expect(c.retainDays, 31);
  });

  test('服务端配置覆盖默认值', () {
    final c = LogConfig.merge(
      base: const LogConfig.defaults(),
      server: {
        'app_log_enabled': false,
        'app_log_level': 'warn',
        'app_log_body': true,
        'app_log_max_mb': 20,
        'app_log_retain_days': 7,
      },
    );
    expect(c.enabled, isFalse);
    expect(c.minLevel, LogLevel.warn);
    expect(c.logBody, isTrue);
    expect(c.maxMB, 20);
    expect(c.retainDays, 7);
  });

  test('服务端只给一部分时，其余保持原值', () {
    final c = LogConfig.merge(
      base: const LogConfig.defaults(),
      server: {'app_log_level': 'debug'},
    );
    expect(c.minLevel, LogLevel.debug);
    expect(c.maxMB, 100); // 没给就不动
  });

  test('服务端给了无法识别的级别时回退默认，不崩', () {
    final c = LogConfig.merge(
      base: const LogConfig.defaults(),
      server: {'app_log_level': 'verbose'},
    );
    expect(c.minLevel, LogLevel.info);
  });

  test('sysconfig 下发的是字符串，"0"/"1" 要当成布尔', () {
    // sysconfig 存的全是字符串，JSON 里过来就是 "0" 而不是 false。
    // 把 "0" 当成真值会让「关掉日志」这个开关彻底失效。
    final c = LogConfig.merge(
      base: const LogConfig.defaults(),
      server: {
        'app_log_enabled': '0',
        'app_log_body': '1',
        'app_log_max_mb': '20',
      },
    );
    expect(c.enabled, isFalse);
    expect(c.logBody, isTrue);
    expect(c.maxMB, 20);
  });

  test('本地开发者开关覆盖一切：强制 debug + 记 body', () {
    // 弥补「配置只能按租户」的限制：本机排查不该动线上配置。
    final c = LogConfig.merge(
      base: const LogConfig.defaults(),
      server: {'app_log_enabled': false, 'app_log_level': 'error'},
      devForce: true,
    );
    expect(c.enabled, isTrue);
    expect(c.minLevel, LogLevel.debug);
    expect(c.logBody, isTrue);
  });

  test('server 为 null 时等于不覆盖', () {
    final c = LogConfig.merge(base: const LogConfig.defaults(), server: null);
    expect(c.minLevel, LogLevel.info);
  });
}
