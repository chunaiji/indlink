import 'package:bottles/core/utils/anon_meta.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('anonMeta drops unknown gender and zero age', () {
    expect(anonMeta('匿名', '女', 24), '匿名 · 女 24');
    expect(anonMeta('匿名', '女', 0), '匿名 · 女');
    expect(anonMeta('匿名', '', 24), '匿名 · 24');
    expect(anonMeta('匿名', '', 0), '匿名');
    expect(anonMeta('匿名', '', null), '匿名');
  });
}
