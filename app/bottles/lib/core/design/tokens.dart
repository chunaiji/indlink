import 'package:flutter/material.dart';

/// 几何刻度 —— 取自 `docs/prototype/v1-screens.html` 的 `--t* / --r* / --s*`。
///
/// 原型里的 px 是 375pt 设备下的等比缩略（比例 0.672），这里直接用还原后的 pt 值。
/// 字号只允许这 8 档，圆角按层级分 6 档，间距走 4pt 栅格。
class Dim {
  const Dim._();

  // 字号：11 角标 / 12 辅助 / 13 次要 / 15 正文 / 17 强调 / 20 小标题 / 24 标题 / 28 数字
  static const double t0 = 11;
  static const double t1 = 12;
  static const double t2 = 13;
  static const double t3 = 15;
  static const double t4 = 17;
  static const double t5 = 20;
  static const double t6 = 24;
  static const double t7 = 28;

  // 圆角：8 标签 / 12 按钮 / 16 卡片 / 20 大卡 / 28 模态 / 999 胶囊
  static const double r1 = 8;
  static const double r2 = 12;
  static const double r3 = 16;
  static const double r4 = 20;
  static const double r5 = 28;
  static const double rPill = 999;

  // 间距：4pt 栅格
  static const double s1 = 4;
  static const double s2 = 8;
  static const double s3 = 12;
  static const double s4 = 16;
  static const double s5 = 20;
  static const double s6 = 24;

  /// 屏幕左右统一 16pt，全 App 一条竖线对齐。
  static const double gutter = 16;

  /// 可点区域下限（列表行 / 按钮 / Tab）。
  static const double tap = 44;

  /// 全圆角胶囊的宽高比下限 1.7:1 —— 单字标签靠这两个值撑开，否则看着是个圆。
  static const double chipMinWidth = 44;
  static const double chipPadH = 12;

  static const BorderRadius brTag = BorderRadius.all(Radius.circular(r1));
  static const BorderRadius brButton = BorderRadius.all(Radius.circular(r2));
  static const BorderRadius brCard = BorderRadius.all(Radius.circular(r3));
  static const BorderRadius brLargeCard = BorderRadius.all(Radius.circular(r4));
  static const BorderRadius brModal = BorderRadius.all(Radius.circular(r5));
  static const BorderRadius brPill = BorderRadius.all(Radius.circular(rPill));
}

/// 断点 —— 全 App 只有这几个数字，页面里不允许再写别的（规范 §3 / §4.2 / §4.3）。
///
/// * 宽度决定布局：≥ [wide] 时 Shell 把内容列限宽 [contentMaxWidth] 居中。
/// * 高度只决定装饰区收缩：< [shortScreen] 时弹性区进入收缩态，**不换布局**。
/// * 360 宽是正常手机（印度 Redmi、三星默认密度），不设更窄的断点。
class Breakpoints {
  const Breakpoints._();

  static const double wide = 600;
  static const double shortScreen = 700;
  static const double contentMaxWidth = 480;

  /// 文字缩放钳制范围：尊重系统无障碍字体，但不允许撑破 360 宽的布局。
  static const double textScaleMin = 0.9;
  static const double textScaleMax = 1.2;

  /// 矮屏（iPhone SE 667）：大标题上方留白减半、插画缩小或隐藏。
  static bool isShort(BuildContext context) =>
      MediaQuery.sizeOf(context).height < shortScreen;

  static bool isWide(BuildContext context) =>
      MediaQuery.sizeOf(context).width >= wide;
}

/// 动效参数 —— 取自原型 §21「交互与转场」。
///
/// 统一原则：响应用户动作的动画要快（≤300ms），自动播放的装饰要慢且可关。
class Motion {
  const Motion._();

  /// 底部弹层：进场有弹性，出场比进场快 100ms（关闭要利落）。
  static const Duration sheetIn = Duration(milliseconds: 320);
  static const Duration sheetOut = Duration(milliseconds: 220);

  /// 被动弹出的模态：scale 0.92→1，不从底部滑入（避免与主动呼出的弹层混淆）。
  static const Duration modal = Duration(milliseconds: 280);

  /// 滑卡飞出 / 页面推入 / Hero。
  static const Duration cardFly = Duration(milliseconds: 280);
  static const Duration pagePush = Duration(milliseconds: 300);

  /// 捞瓶仪式全程约 2.5s —— 把加载态藏进仪式感里。
  static const Duration scoopRitual = Duration(milliseconds: 2600);

  /// 抛瓶抛物线 + 落水涟漪。
  static const Duration castArc = Duration(milliseconds: 600);
  static const Duration castSplash = Duration(milliseconds: 400);

  /// 瓶子自动浮动，各瓶随机相位偏移，避免「军训感」。
  static const Duration bob = Duration(milliseconds: 4500);

  /// 骨架屏交叉淡入，不做位移。
  static const Duration skeletonFade = Duration(milliseconds: 160);

  /// 未读红点：出现有弹性，进入该 Tab 后延迟 400ms 再消失。
  static const Duration badgeIn = Duration(milliseconds: 200);
  static const Duration badgeOut = Duration(milliseconds: 120);
  static const Duration badgeLinger = Duration(milliseconds: 400);

  /// <300ms 不显示任何加载态（闪一下更烦）。
  static const Duration loadingDelay = Duration(milliseconds: 300);

  /// >8s 转失败态。
  static const Duration loadingTimeout = Duration(seconds: 8);

  static const Curve emphasized = Curves.easeOutCubic;
  static const Curve exit = Curves.easeInCubic;
  static const Curve spring = Curves.easeOutBack;
}

/// 阴影 —— 原型 CSS 的 blur / offset 按 1.488 放大，透明度不变。
///
/// 组件只引用这里，不各写一份 BoxShadow；否则同一种「悬浮」在三个页面会长出三种阴影。
class Shadows {
  const Shadows._();

  /// 静态卡片（`.mcd` `.lst`）：`--shadow` = 0 1px 2px .05 + 0 8px 24px .07。
  static List<BoxShadow> card(AppColors c) => [
    BoxShadow(
      color: c.ink.withValues(alpha: .05),
      blurRadius: 3,
      offset: const Offset(0, 1),
    ),
    BoxShadow(
      color: c.ink.withValues(alpha: .07),
      blurRadius: 36,
      offset: const Offset(0, 12),
    ),
  ];

  /// 悬浮块（滑卡 `.swc`、钱包 `.wallet`、余额卡 `.pcard`）：0 10px 26px .18。
  static List<BoxShadow> floating(AppColors c) => [
    BoxShadow(
      color: c.ink.withValues(alpha: .18),
      blurRadius: 39,
      offset: const Offset(0, 15),
    ),
  ];

  /// 底部弹层 `.sheet`：0 -8px 28px .2。
  static List<BoxShadow> sheet() => const [
    BoxShadow(color: Color(0x33000000), blurRadius: 42, offset: Offset(0, -12)),
  ];

  /// 居中模态 `.modal`：0 18px 44px .36。
  static List<BoxShadow> modal() => const [
    BoxShadow(color: Color(0x5C000000), blurRadius: 65, offset: Offset(0, 27)),
  ];

  /// 实心按钮同色投影 `.cta`：0 5px 16px。alpha 按语义给：海蓝 .38 / 粉 .40 / 警示 .34。
  static List<BoxShadow> glow(Color color, {double alpha = .38}) => [
    BoxShadow(
      color: color.withValues(alpha: alpha),
      blurRadius: 24,
      offset: const Offset(0, 7),
    ),
  ];

  /// 小按钮 `.jn` / `.hi` / chips 选中态：0 3px 10px。
  static List<BoxShadow> glowSmall(Color color, {double alpha = .34}) => [
    BoxShadow(
      color: color.withValues(alpha: alpha),
      blurRadius: 15,
      offset: const Offset(0, 4),
    ),
  ];
}

/// 语义色板。
///
/// 明暗两套值逐一对应原型的 `:root` 与 `@media (prefers-color-scheme:dark)`。
/// 注意：**夜场与系统暗色是两根独立的轴**，夜场只改 [brandGradient] 与 [seaScene]，
/// 不动 [brand]（粉色永远只代表「要花金币」）。
@immutable
class AppColors extends ThemeExtension<AppColors> {
  const AppColors({
    required this.sea,
    required this.mist,
    required this.coral,
    required this.surface,
    required this.ink,
    required this.ink2,
    required this.ink3,
    required this.page,
    required this.line,
    required this.line2,
    required this.gold,
    required this.info,
    required this.warn,
    required this.brand,
    required this.brand2,
    required this.aqua,
    required this.coin,
    required this.success,
    required this.brandGradient,
    required this.shadow,
  });

  final Color sea;
  final Color mist;
  final Color coral;

  /// 卡片 / 弹层底色。红点白边取它，暗色下自动变深，不能写死白色。
  final Color surface;

  final Color ink;
  final Color ink2;
  final Color ink3;
  final Color page;
  final Color line;
  final Color line2;
  final Color gold;
  final Color info;
  final Color warn;

  /// 粉色 —— 全 App 只有一个含义：要花金币或真钱。
  final Color brand;
  final Color brand2;

  /// 海蓝 —— 主线免费动作（捞瓶 / 抛瓶 / 登录 / 签到领币）。
  final Color aqua;

  final Color coin;
  final Color success;

  /// 品牌渐变，只用于头图 / Tab 选中块 / 进度条，按钮一律实色。
  final List<Color> brandGradient;

  final List<BoxShadow> shadow;

  static const AppColors light = AppColors(
    sea: Color(0xFFDCEFF2),
    mist: Color(0xFF9DBDC8),
    coral: Color(0xFFFF4D6D),
    surface: Color(0xFFFBFEFF),
    ink: Color(0xFF16323C),
    ink2: Color(0xFF56727D),
    ink3: Color(0xFF8AA3AC),
    page: Color(0xFFF2F8FA),
    line: Color(0x2216323C),
    line2: Color(0x1216323C),
    gold: Color(0xFFC8912F),
    info: Color(0xFF3E7F99),
    warn: Color(0xFFC2562F),
    brand: Color(0xFFFF4D6D),
    brand2: Color(0xFF7B5CFF),
    aqua: Color(0xFF12B5CE),
    coin: Color(0xFFFFC53D),
    success: Color(0xFF2FBF77),
    brandGradient: [Color(0xFFFF4D6D), Color(0xFFFF7EA8), Color(0xFF9B6BE0)],
    shadow: [
      BoxShadow(color: Color(0x0D16323C), blurRadius: 2, offset: Offset(0, 1)),
      BoxShadow(color: Color(0x1216323C), blurRadius: 24, offset: Offset(0, 8)),
    ],
  );

  static const AppColors dark = AppColors(
    sea: Color(0xFF12252D),
    mist: Color(0xFF5A7C88),
    coral: Color(0xFFFF6B85),
    surface: Color(0xFF16292F),
    ink: Color(0xFFE3F0F3),
    ink2: Color(0xFF9BB5BD),
    ink3: Color(0xFF6E8992),
    page: Color(0xFF0D1B21),
    line: Color(0x26E3F0F3),
    line2: Color(0x12E3F0F3),
    gold: Color(0xFFE0AE55),
    info: Color(0xFF6FB3CC),
    warn: Color(0xFFF09070),
    brand: Color(0xFFFF6B85),
    brand2: Color(0xFF9B82FF),
    aqua: Color(0xFF3FC6DC),
    coin: Color(0xFFFFD166),
    success: Color(0xFF3FD48C),
    brandGradient: [Color(0xFFE8365A), Color(0xFFE8619A), Color(0xFF7B4FD6)],
    shadow: [
      BoxShadow(color: Color(0x4D000000), blurRadius: 2, offset: Offset(0, 1)),
      BoxShadow(color: Color(0x59000000), blurRadius: 24, offset: Offset(0, 8)),
    ],
  );

  /// 夜场（21:00–01:00 的产品状态）下的品牌渐变。
  static const List<Color> nightGradient = [
    Color(0xFF2E2A7A),
    Color(0xFF5B3E9E),
    Color(0xFF8C3D80),
  ];

  /// 黄昏的海 —— 首页 scene 背景，自上而下 9 个色标。
  static const List<Color> duskScene = [
    Color(0xFF8E6FE0),
    Color(0xFFB06CC8),
    Color(0xFFE07AA0),
    Color(0xFFF2A47E),
    Color(0xFF6389B6),
    Color(0xFF43729F),
    Color(0xFF33608A),
    Color(0xFFC9B48C),
    Color(0xFFE8DAB9),
  ];

  /// 明亮卡通白天海 —— 首页日间 scene 背景（对齐原型 B1 浅蓝渐变），
  /// 色标位置与 [sceneStops] 一一对应。上部浅蓝天空、约 0.47 处海天交界、下部通透海水。
  static const List<Color> dayScene = [
    Color(0xFFA9E6FF),
    Color(0xFFC2ECFF),
    Color(0xFFC9F0FF),
    Color(0xFFA6E3F5),
    Color(0xFF8FDCF3),
    Color(0xFF39B0E4),
    Color(0xFF2298D6),
    Color(0xFF1E92D4),
    Color(0xFF1877BC),
  ];

  static const List<double> sceneStops = [
    0,
    0.13,
    0.25,
    0.32,
    0.40,
    0.56,
    0.70,
    0.82,
    1.0,
  ];

  /// 入夜的海 —— 夜场 scene 背景，色标位置与 [duskScene] 一一对应。
  static const List<Color> nightScene = [
    Color(0xFF0A0E2E),
    Color(0xFF161C48),
    Color(0xFF2A2160),
    Color(0xFF3B2A63),
    Color(0xFF16244A),
    Color(0xFF101C3A),
    Color(0xFF0B1730),
    Color(0xFF2E2A3C),
    Color(0xFF423C4E),
  ];

  @override
  AppColors copyWith({
    Color? sea,
    Color? mist,
    Color? coral,
    Color? surface,
    Color? ink,
    Color? ink2,
    Color? ink3,
    Color? page,
    Color? line,
    Color? line2,
    Color? gold,
    Color? info,
    Color? warn,
    Color? brand,
    Color? brand2,
    Color? aqua,
    Color? coin,
    Color? success,
    List<Color>? brandGradient,
    List<BoxShadow>? shadow,
  }) {
    return AppColors(
      sea: sea ?? this.sea,
      mist: mist ?? this.mist,
      coral: coral ?? this.coral,
      surface: surface ?? this.surface,
      ink: ink ?? this.ink,
      ink2: ink2 ?? this.ink2,
      ink3: ink3 ?? this.ink3,
      page: page ?? this.page,
      line: line ?? this.line,
      line2: line2 ?? this.line2,
      gold: gold ?? this.gold,
      info: info ?? this.info,
      warn: warn ?? this.warn,
      brand: brand ?? this.brand,
      brand2: brand2 ?? this.brand2,
      aqua: aqua ?? this.aqua,
      coin: coin ?? this.coin,
      success: success ?? this.success,
      brandGradient: brandGradient ?? this.brandGradient,
      shadow: shadow ?? this.shadow,
    );
  }

  @override
  AppColors lerp(covariant AppColors? other, double t) {
    if (other == null) return this;
    return AppColors(
      sea: Color.lerp(sea, other.sea, t)!,
      mist: Color.lerp(mist, other.mist, t)!,
      coral: Color.lerp(coral, other.coral, t)!,
      surface: Color.lerp(surface, other.surface, t)!,
      ink: Color.lerp(ink, other.ink, t)!,
      ink2: Color.lerp(ink2, other.ink2, t)!,
      ink3: Color.lerp(ink3, other.ink3, t)!,
      page: Color.lerp(page, other.page, t)!,
      line: Color.lerp(line, other.line, t)!,
      line2: Color.lerp(line2, other.line2, t)!,
      gold: Color.lerp(gold, other.gold, t)!,
      info: Color.lerp(info, other.info, t)!,
      warn: Color.lerp(warn, other.warn, t)!,
      brand: Color.lerp(brand, other.brand, t)!,
      brand2: Color.lerp(brand2, other.brand2, t)!,
      aqua: Color.lerp(aqua, other.aqua, t)!,
      coin: Color.lerp(coin, other.coin, t)!,
      success: Color.lerp(success, other.success, t)!,
      brandGradient: [
        for (var i = 0; i < brandGradient.length; i++)
          Color.lerp(brandGradient[i], other.brandGradient[i], t)!,
      ],
      shadow: BoxShadow.lerpList(shadow, other.shadow, t)!,
    );
  }
}

extension AppColorsX on BuildContext {
  /// 语义色板。明暗自动切换，页面里不要直接写死色值。
  AppColors get c => Theme.of(this).extension<AppColors>()!;
}

/// 可滚动页面的内容边距：左右 gutter，底部再加系统手势区 / 三键导航。
///
/// 用 `MediaQuery.padding`（不是 viewPadding）：Scaffold 在 Tab 栏之下会把它清零，
/// 所以 Tab 根页用它不会多出一截；根 Navigator 上的全屏二级页最后一项才不会贴着手势条。
/// 有底部按钮的页面不用它——安全区由 BottomActionBar 自己吃。
extension PageInsets on BuildContext {
  EdgeInsets pagePadding({
    double top = Dim.gutter,
    double bottom = Dim.gutter,
  }) {
    return EdgeInsets.fromLTRB(
      Dim.gutter,
      top,
      Dim.gutter,
      bottom + MediaQuery.paddingOf(this).bottom,
    );
  }
}
