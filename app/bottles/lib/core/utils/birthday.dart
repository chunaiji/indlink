/// 出生日期的三件小事：解析、格式化（`YYYY-MM-DD`）、算周岁。
/// 完善资料、编辑资料、年龄门槛判断都用这一份，和服务端 `user.ageOf` 同一算法。
int ageFromBirthday(DateTime birth, DateTime now) {
  var age = now.year - birth.year;
  if (now.month < birth.month ||
      (now.month == birth.month && now.day < birth.day)) {
    age--;
  }
  return age < 0 ? 0 : age;
}

DateTime? parseBirthday(String? s) =>
    s == null || s.isEmpty ? null : DateTime.tryParse(s);

String formatBirthday(DateTime d) =>
    '${d.year.toString().padLeft(4, '0')}-'
    '${d.month.toString().padLeft(2, '0')}-'
    '${d.day.toString().padLeft(2, '0')}';
