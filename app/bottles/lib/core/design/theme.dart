import 'package:flutter/cupertino.dart' show CupertinoPageTransitionsBuilder;
import 'package:flutter/material.dart';

import 'tokens.dart';

/// 主题构建 —— 明暗两套共用一份排版刻度，只换 [AppColors]。
class AppTheme {
  const AppTheme._();

  static ThemeData light() => _build(Brightness.light, AppColors.light);

  static ThemeData dark() => _build(Brightness.dark, AppColors.dark);

  static ThemeData _build(Brightness brightness, AppColors c) {
    final scheme = ColorScheme.fromSeed(
      seedColor: c.brand,
      brightness: brightness,
      surface: c.surface,
      onSurface: c.ink,
      primary: c.brand,
      secondary: c.aqua,
      error: c.warn,
    );

    // Material 3 默认把按钮撑到 48、列表行撑到 56，比原型的 44pt 体系胖一圈——
    // 这就是真机上「像放大版」的来源之一（规范 §4.7）。密度收紧到 44，
    // 可点区域下限仍由 Dim.tap 守着。
    const buttonMin = Size(0, Dim.tap);
    const buttonShape = RoundedRectangleBorder(borderRadius: Dim.brPill);

    return ThemeData(
      useMaterial3: true,
      brightness: brightness,
      colorScheme: scheme,
      scaffoldBackgroundColor: c.page,
      canvasColor: c.page,
      splashFactory: InkSparkle.splashFactory,
      extensions: <ThemeExtension<dynamic>>[c],
      visualDensity: VisualDensity.compact,
      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
      textTheme: _textTheme(c),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          minimumSize: buttonMin,
          shape: buttonShape,
          textStyle: const TextStyle(
            fontSize: Dim.t3,
            fontWeight: FontWeight.w700,
            height: 1.2,
          ),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          minimumSize: buttonMin,
          shape: buttonShape,
          textStyle: const TextStyle(
            fontSize: Dim.t3,
            fontWeight: FontWeight.w700,
            height: 1.2,
          ),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: buttonMin,
          shape: buttonShape,
          side: BorderSide(color: c.line),
          textStyle: const TextStyle(
            fontSize: Dim.t3,
            fontWeight: FontWeight.w700,
            height: 1.2,
          ),
        ),
      ),
      iconButtonTheme: IconButtonThemeData(
        style: IconButton.styleFrom(
          minimumSize: const Size(Dim.tap, Dim.tap),
          padding: EdgeInsets.zero,
        ),
      ),
      listTileTheme: const ListTileThemeData(
        minVerticalPadding: Dim.s2,
        minTileHeight: Dim.tap,
      ),
      dividerTheme: DividerThemeData(color: c.line2, thickness: 1, space: 1),
      appBarTheme: AppBarTheme(
        backgroundColor: c.page,
        surfaceTintColor: Colors.transparent,
        foregroundColor: c.ink,
        elevation: 0,
        scrolledUnderElevation: 0,
        centerTitle: true,
        titleTextStyle: TextStyle(
          color: c.ink,
          fontSize: Dim.t4,
          fontWeight: FontWeight.w700,
          letterSpacing: -0.1,
        ),
      ),
      bottomSheetTheme: BottomSheetThemeData(
        backgroundColor: c.surface,
        surfaceTintColor: Colors.transparent,
        modalBackgroundColor: c.surface,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(top: Radius.circular(Dim.r5)),
        ),
        showDragHandle: false,
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: c.surface,
        surfaceTintColor: Colors.transparent,
        shape: const RoundedRectangleBorder(borderRadius: Dim.brModal),
      ),
      snackBarTheme: SnackBarThemeData(
        behavior: SnackBarBehavior.floating,
        backgroundColor: c.ink,
        contentTextStyle: TextStyle(color: c.surface, fontSize: Dim.t2),
        shape: const RoundedRectangleBorder(borderRadius: Dim.brButton),
      ),
      progressIndicatorTheme: ProgressIndicatorThemeData(
        color: c.aqua,
        linearTrackColor: c.sea,
        circularTrackColor: Colors.transparent,
      ),
      // 原型 .sld：轨 3、拇指 14px → 21 白底。颜色按产品规则走海蓝（粉 = 花钱），
      // 不照搬原型里的粉。
      sliderTheme: SliderThemeData(
        activeTrackColor: c.aqua,
        inactiveTrackColor: c.line,
        thumbColor: c.surface,
        overlayColor: c.aqua.withValues(alpha: 0.12),
        trackHeight: 3,
        thumbShape: const RoundSliderThumbShape(
          enabledThumbRadius: 10.5,
          elevation: 2,
        ),
      ),
      // 输入框统一走填充式，不用 Material 默认的下划线。
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: c.surface,
        hintStyle: TextStyle(color: c.ink3, fontSize: Dim.t3),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: Dim.s3,
          vertical: Dim.s3,
        ),
        border: OutlineInputBorder(
          borderRadius: Dim.brButton,
          borderSide: BorderSide(color: c.line),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: Dim.brButton,
          borderSide: BorderSide(color: c.line),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: Dim.brButton,
          borderSide: BorderSide(color: c.aqua, width: 1.5),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: Dim.brButton,
          borderSide: BorderSide(color: c.warn),
        ),
      ),
      pageTransitionsTheme: const PageTransitionsTheme(
        builders: {
          // iOS 的边缘返回手势是肌肉记忆，用平台原生转场，不自己写统一转场。
          TargetPlatform.iOS: CupertinoPageTransitionsBuilder(),
          TargetPlatform.macOS: CupertinoPageTransitionsBuilder(),
          TargetPlatform.android: FadeForwardsPageTransitionsBuilder(),
          TargetPlatform.windows: FadeForwardsPageTransitionsBuilder(),
          TargetPlatform.linux: FadeForwardsPageTransitionsBuilder(),
        },
      ),
    );
  }

  /// 每一档都显式给行高（规范 §4.6）：不给的话行高由设备字体决定，
  /// 小米 MiSans / 华为 HarmonyOS Sans / Noto CJK 的 ascent+descent 比 PingFang 大，
  /// 同样 15pt 一行能高出 2～4pt，十行下来卡片就比原型高一截。
  /// `leadingDistribution.even` 让上下留白均分，中英混排基线才对得齐。
  static TextTheme _textTheme(AppColors c) {
    TextStyle s(
      double size,
      FontWeight w, {
      Color? color,
      required double h,
      double? ls,
    }) {
      return TextStyle(
        fontSize: size,
        fontWeight: w,
        color: color ?? c.ink,
        height: h,
        letterSpacing: ls,
        leadingDistribution: TextLeadingDistribution.even,
      );
    }

    return TextTheme(
      // 数字（余额 / 统计）
      displayLarge: s(Dim.t7, FontWeight.w800, h: 1.2, ls: -0.5),
      // 页面主标题
      headlineLarge: s(Dim.t6, FontWeight.w800, h: 1.25, ls: -0.24),
      // 小标题
      titleLarge: s(Dim.t5, FontWeight.w800, h: 1.25, ls: -0.2),
      // 强调 / 导航标题
      titleMedium: s(Dim.t4, FontWeight.w700, h: 1.25, ls: -0.1),
      // 列表行主文案
      titleSmall: s(Dim.t3, FontWeight.w700, h: 1.4),
      // 正文
      bodyLarge: s(Dim.t3, FontWeight.w400, h: 1.6),
      // 次要
      bodyMedium: s(Dim.t2, FontWeight.w400, color: c.ink2, h: 1.55),
      // 辅助
      bodySmall: s(Dim.t1, FontWeight.w400, color: c.ink3, h: 1.5),
      // 角标
      labelSmall: s(Dim.t0, FontWeight.w600, color: c.ink3, h: 1.4),
    );
  }
}
