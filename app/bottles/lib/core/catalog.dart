import 'package:flutter/widgets.dart';

/// 业务枚举的展示名。
///
/// 语言用本地自称（endonym），不翻译——「हिन्दी」在中文界面里也该是「हिन्दी」。
/// 兴趣与标签存 key、显示随界面语言切换；正式接后端后这份表改为远程下发。
class Catalog {
  const Catalog._();

  static const languages = [
    'English',
    'हिन्दी',
    'తెలుగు',
    'தமிழ்',
    'বাংলা',
    '中文',
  ];

  static const interests = <String, (String, String)>{
    'music': ('音乐', 'Music'),
    'movie': ('电影', 'Movies'),
    'travel': ('旅行', 'Travel'),
    'cricket': ('板球', 'Cricket'),
    'food': ('美食', 'Food'),
    'art': ('绘画', 'Art'),
    'photo': ('摄影', 'Photography'),
    'reading': ('阅读', 'Reading'),
    'fitness': ('健身', 'Fitness'),
    'gaming': ('游戏', 'Gaming'),
  };

  /// 瓶子标签。`night` 是后端已实现的夜场标签，只在夜场时段可捞。
  static const bottleTags = <String, (String, String)>{
    'night': ('🌙 深夜', '🌙 Night'),
    'hollow': ('树洞', 'Vent'),
    'music': ('音乐', 'Music'),
    'travel': ('旅行', 'Travel'),
  };

  static bool _isZh(BuildContext context) =>
      Localizations.localeOf(context).languageCode == 'zh';

  static String interest(BuildContext context, String key) {
    final pair = interests[key];
    if (pair == null) return key;
    return _isZh(context) ? pair.$1 : pair.$2;
  }

  static String bottleTag(BuildContext context, String key) {
    final pair = bottleTags[key];
    if (pair == null) return key;
    return _isZh(context) ? pair.$1 : pair.$2;
  }
}
