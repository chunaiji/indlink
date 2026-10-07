/// 发布内容时附带的地点。
///
/// 三个字段的分工：
///   - [lat] / [lng] 给服务端算距离用，**永远不会被下发给其他用户**
///     （`model.Bottle` 的 Lat/Lng 标了 `json:"-"`，对外只给 distance_km）
///   - [name] 是短地名（"Bandra West"），界面上显示的就是它
///   - [city] 供服务端做同城判断
///
/// 整个对象为 null 表示「不带地点」——那是一等选项，不是「忘了选」。
class Place {
  const Place({
    required this.lat,
    required this.lng,
    this.name = '',
    this.city = '',
  });

  final double lat;
  final double lng;
  final String name;
  final String city;

  /// 界面上显示哪一个。短地名优先，退回城市，都没有就给坐标——
  /// 坐标很难看，但**不能开天窗**：用户刚选完地点却看到一行空白，
  /// 会以为没选上而反复点。
  String get label {
    if (name.isNotEmpty) return name;
    if (city.isNotEmpty) return city;
    return '${lat.toStringAsFixed(4)}, ${lng.toStringAsFixed(4)}';
  }

  /// 副标题：有短地名时把城市放第二行，否则不显示第二行。
  String get subtitle => (name.isNotEmpty && city.isNotEmpty) ? city : '';

  Place copyWith({double? lat, double? lng, String? name, String? city}) =>
      Place(
        lat: lat ?? this.lat,
        lng: lng ?? this.lng,
        name: name ?? this.name,
        city: city ?? this.city,
      );
}
