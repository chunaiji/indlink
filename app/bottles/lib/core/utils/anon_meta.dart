/// 匿名作者的元信息行：「匿名 · 女 24」。
///
/// 性别保密 / 年龄为 0 的部分直接不显示——按模板硬拼会渲染成「匿名 · 0」（真机截到过）。
String anonMeta(String name, String gender, int? age) {
  final tail = [
    if (gender.isNotEmpty) gender,
    if (age != null && age > 0) '$age',
  ].join(' ');
  return tail.isEmpty ? name : '$name · $tail';
}
