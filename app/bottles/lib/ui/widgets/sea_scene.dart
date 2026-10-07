import 'dart:math';
import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../core/design/tokens.dart';

/// 海面画布的设计尺寸：原型 B1 的 `.scene`（头图与 Tab 栏之间）在 375 宽框里约 490 高。
/// 页面里的装饰坐标全部按这个坐标系给，`DesignCanvas` 按宽度等比缩放（规范 §5）。
const seaDesignSize = Size(375, 490);

/// 九个瓶位，按**水带**的比例给：x 是「可放宽度」的比例，y 是「海平线到水带底」的比例。
/// 整图在不同屏高上海平线位置不同，瓶子跟着水带走，永远漂在水面、不进沙滩、不压 dock。
///
/// 后台可配 5~9 个（`ocean_bottle_count`），前 N 个瓶位按序点亮，所以**顺序就是优先级**：
/// 前五个先把三排撑开（上排三只、中排两只），6~9 依次补下排与中排最右。
/// 三排错落、相邻方框横向至少隔 54pt；各瓶随机相位漂浮（相位在 FloatingBottle 内部）。
const bottleSlots = <Offset>[
  Offset(.15, .06), // 上排 左
  Offset(.50, .00), // 上排 中
  Offset(.84, .08), // 上排 右
  Offset(.33, .50), // 中排 左
  Offset(.66, .56), // 中排 右
  Offset(.05, 1.0), // 下排 左
  Offset(.48, .98), // 下排 中
  Offset(.92, 1.0), // 下排 右
  Offset(.99, .52), // 中排 最右
];
const bottleSize = Size(54, 54);

/// 各瓶位的摆放角度（顺时针度数，0 = 横躺、-90 = 竖直瓶口朝上）。
/// 六个角度刻意拉开：同一张素材摆成一排一样的角度，就是六个复制粘贴。
const bottleTilts = <double>[-28, 18, -62, 40, -8, 66, -45, 30, 55];

/// 捞瓶涟漪中心（原型 `.ripple` left 50% top 46%）。
const rippleCenter = Offset(187, 225);

/// 海面皮肤：一张整图 + 它的海平线 / 沙滩线位置（占图高的比例）。
///
/// 两张图都是竖幅海滩插画。渲染时按高铺满、最多放大 [SeaScene.maxZoom] 把海平线往上抬，
/// 瓶子按实际海平线与沙滩线之间的水带摆放。
class SeaSkin {
  const SeaSkin(
    this.asset, {
    required this.horizon,
    required this.shore,
    required this.aspect,
  });

  final String asset;

  /// 海平线距图顶的比例。
  final double horizon;

  /// 浪花打到沙滩的位置（水带下沿）距图顶的比例。
  final double shore;

  /// 图的高宽比（高 / 宽）。
  final double aspect;
}

/// 海面场景。整屏铺满（头像、金币、切换、筛选都悬浮在它上面）。
///
/// 白天 / 夜晚各一张整图（[daySkin] [nightSkin]），太阳、月亮、云、星星、棕榈都画在图里；
/// 这里只负责把图铺满、找到水带、压一层底部暗色让 dock 按钮可读，再把 [bottles]
/// 放到水带里的六个瓶位上。
///
/// **夜场是产品状态，系统暗色是显示偏好，两根独立的轴**，四种组合都要成立；
/// 皮肤是第三根轴（用户手动切白天 / 夜晚，见 `seaLookProvider`），它只决定画哪张图。
class SeaScene extends StatelessWidget {
  const SeaScene({
    super.key,
    required this.night,
    this.bottles = const [],
    this.dimmed = false,
  });

  /// 画夜晚那张。
  final bool night;

  /// 漂在水面上的瓶子（按序落到 [bottleSlots]），最多六个。
  final List<Widget> bottles;

  /// 捞瓶仪式期间压一层深色，把注意力收到水面上。
  final bool dimmed;

  static const dayAsset = 'assets/images/sea-day.jpg';
  static const nightAsset = 'assets/images/sea-night.jpg';

  /// 1080×1920：海平线约 62.5%，浪花线约 86%（左下角有躺椅，更靠下的位置不放瓶子）。
  static const daySkin = SeaSkin(
    dayAsset,
    horizon: .625,
    shore: .86,
    aspect: 1920 / 1080,
  );
  static const nightSkin = SeaSkin(
    nightAsset,
    horizon: .625,
    shore: .86,
    aspect: 1920 / 1080,
  );

  /// 想把海平线抬到画布的这个高度（比例）——天空留给悬浮的顶栏与筛选条。
  static const targetHorizon = .5;

  /// 为了抬海平线，图最多放大到这个倍数（左右各裁掉约 8%）。
  /// 再大就会把左侧棕榈 / 右侧云裁没；抬不够就让海平线低一点，瓶子跟着水带走。
  static const maxZoom = 1.2;

  /// 底部 dock（海况标签 + 两枚按钮 + 边距）占的高度，水带下沿不能压到它。
  static const dockReserve = 132.0;

  SeaSkin get skin => night ? nightSkin : daySkin;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, box) {
        final fit = SkinFit.solve(skin, box.maxWidth, box.maxHeight);
        return Stack(
          fit: StackFit.expand,
          clipBehavior: Clip.hardEdge,
          children: [
            Positioned(
              left: fit.left,
              top: fit.top,
              width: fit.width,
              height: fit.height,
              child: Image.asset(
                skin.asset,
                fit: BoxFit.fill,
                filterQuality: FilterQuality.medium,
              ),
            ),
            // 图的下沿是亮沙滩，压一层暗色让海况标签与两枚 dock 按钮读得清。
            const Positioned.fill(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [
                      Color(0x00000000),
                      Color(0x00000000),
                      Color(0x59000000),
                    ],
                    stops: [0, .66, 1],
                  ),
                ),
              ),
            ),
            for (var i = 0; i < bottles.length && i < bottleSlots.length; i++)
              Positioned(
                left: fit.slotLeft(bottleSlots[i].dx, box.maxWidth),
                top: fit.slotTop(bottleSlots[i].dy),
                child: bottles[i],
              ),
            if (dimmed)
              const Positioned.fill(
                child: ColoredBox(color: Color(0x57081424)),
              ),
          ],
        );
      },
    );
  }
}

/// 整图在画布里的摆法 + 由此算出的水带。
///
/// 1. 先保证铺满（按高 cover）；2. 再最多放大到 [SeaScene.maxZoom]，把海平线往
/// [SeaScene.targetHorizon] 抬；3. 水带 = 实际海平线 → min(浪花线, dock 上沿)。
class SkinFit {
  const SkinFit({
    required this.left,
    required this.top,
    required this.width,
    required this.height,
    required this.horizon,
    required this.waterBottom,
  });

  final double left;
  final double top;
  final double width;
  final double height;

  /// 实际海平线（画布坐标）。
  final double horizon;

  /// 水带下沿（画布坐标）。
  final double waterBottom;

  /// 瓶位可用的水带高度（扣掉瓶子自身）。
  double get bandHeight =>
      math.max(0, waterBottom - horizon - bottleSize.height - 8);

  double slotLeft(double fx, double w) =>
      Dim.s2 + fx * math.max(0, w - bottleSize.width - Dim.s2 * 2);

  double slotTop(double fy) => horizon + 8 + fy * bandHeight;

  static SkinFit solve(SeaSkin skin, double w, double h) {
    final baseH = w * skin.aspect;
    final cover = math.max(1.0, h / baseH);
    final target = h * SeaScene.targetHorizon;
    // 底边贴齐时海平线 = h - (1-horizon)·ih；要它到 target 需要多高的图。
    final needH = (h - target) / (1 - skin.horizon);
    final zoom = (needH / baseH).clamp(
      cover,
      math.max(cover, SeaScene.maxZoom),
    );
    final iw = w * zoom;
    final ih = iw * skin.aspect;
    // 在「海平线对齐」与「底边不露底」之间取可行值（图一定不矮于画布）。
    final top = math.max(h - ih, target - skin.horizon * ih).clamp(h - ih, 0.0);
    final horizon = top + skin.horizon * ih;
    final shore = top + skin.shore * ih;
    return SkinFit(
      left: (w - iw) / 2,
      top: top,
      width: iw,
      height: ih,
      horizon: horizon,
      waterBottom: math.min(shore, h - SeaScene.dockReserve),
    );
  }
}

/// 海面上漂浮的瓶子。
///
/// 各瓶**随机相位偏移**，否则会出现整齐划一的「军训感」；
/// 系统开了「减弱动态效果」就停在静止帧。
class FloatingBottle extends StatefulWidget {
  const FloatingBottle({
    super.key,
    this.replyCount = 0,
    this.phase = 0,
    this.night = false,
    this.tilt = -20,
    this.art = BottleArt.sprite,
    this.onTap,
  });

  final int replyCount;

  /// 摆放角度（度，顺时针；0 = 横躺）。海面上每个瓶位一个角度，见 [bottleTilts]。
  /// 只对 [BottleArt.sprite] 生效：海面整图素材自带姿态，不再旋转。
  final double tilt;

  /// 用哪张素材画。海面瓶位用 [BottleArt.seaLeft] / [BottleArt.seaRight]，
  /// 捞瓶上浮与扔瓶抛物线保持 [BottleArt.sprite]（透明底，能转角度、能飞）。
  final BottleArt art;

  /// 0~1 的相位偏移。
  final double phase;

  final bool night;
  final VoidCallback? onTap;

  @override
  State<FloatingBottle> createState() => _FloatingBottleState();
}

class _FloatingBottleState extends State<FloatingBottle>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl = AnimationController(
    vsync: this,
    duration: Motion.bob,
    value: widget.phase,
  );

  @override
  void initState() {
    super.initState();
    _ctrl.repeat(reverse: true);
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final reduceMotion = MediaQuery.disableAnimationsOf(context);
    final c = context.c;

    final bottle = Stack(
      clipBehavior: Clip.none,
      children: [
        // 真实素材瓶：透明底的按瓶位各自角度摆、夜场给一圈淡光；
        // 海面整图素材自带水面与姿态，走羽化贴合。
        if (widget.art == BottleArt.sprite)
          BottleSprite(
            width: bottleSize.width,
            tilt: widget.tilt,
            glow: widget.night,
          )
        else
          SeaBottleImage(art: widget.art, night: widget.night),
        if (widget.replyCount > 0)
          Positioned(
            right: -9,
            top: -8,
            child: Container(
              constraints: const BoxConstraints(minWidth: 16),
              height: 16,
              padding: const EdgeInsets.symmetric(horizontal: 4),
              decoration: BoxDecoration(
                color: c.brand,
                borderRadius: Dim.brPill,
                boxShadow: const [
                  BoxShadow(
                    color: Color(0x40000000),
                    blurRadius: 4,
                    offset: Offset(0, 1),
                  ),
                ],
              ),
              alignment: Alignment.center,
              child: Text(
                '${widget.replyCount}',
                style: const TextStyle(
                  fontSize: 10,
                  height: 1,
                  fontWeight: FontWeight.w800,
                  color: Colors.white,
                ),
              ),
            ),
          ),
      ],
    );

    return GestureDetector(
      onTap: widget.onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        // 撑开可点区域，瓶子本身太小点不准。
        padding: const EdgeInsets.all(6),
        child: reduceMotion
            ? bottle
            : AnimatedBuilder(
                animation: _ctrl,
                builder: (context, child) {
                  final t = Curves.easeInOut.transform(_ctrl.value);
                  return Transform.translate(
                    offset: Offset(0, -9 * t),
                    child: Transform.rotate(
                      angle: (-2 + 4 * t) * pi / 180,
                      child: child,
                    ),
                  );
                },
                child: bottle,
              ),
      ),
    );
  }
}

/// 捞瓶时的水面涟漪：三环错相扩散。
class RippleRings extends StatefulWidget {
  const RippleRings({super.key, this.size = 220});

  final double size;

  @override
  State<RippleRings> createState() => _RippleRingsState();
}

class _RippleRingsState extends State<RippleRings>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 2600),
  )..repeat();

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: widget.size,
      height: widget.size,
      child: AnimatedBuilder(
        animation: _ctrl,
        builder: (context, _) =>
            CustomPaint(painter: _RipplePainter(_ctrl.value)),
      ),
    );
  }
}

class _RipplePainter extends CustomPainter {
  _RipplePainter(this.t);

  final double t;

  @override
  void paint(Canvas canvas, Size size) {
    final center = size.center(Offset.zero);
    for (var i = 0; i < 3; i++) {
      final p = (t + i / 3) % 1.0;
      final eased = Curves.easeOut.transform(p);
      canvas.drawCircle(
        center,
        size.width / 2 * eased,
        Paint()
          ..style = PaintingStyle.stroke
          ..strokeWidth = 2
          ..color = Colors.white.withValues(alpha: (1 - eased) * .75),
      );
    }
  }

  @override
  bool shouldRepaint(covariant _RipplePainter old) => old.t != t;
}

/// 海面上的两个主按钮（`.pbn`）。
///
/// **两个都是海蓝**——全 App 用「粉 = 要花币」，海面不出现粉色就是在说「这里免费」。
class DockButton extends StatelessWidget {
  const DockButton({
    super.key,
    required this.label,
    required this.hint,
    this.onTap,
    this.onLongPressStart,
    this.onLongPressEnd,
    this.glass = false,
  });

  final String label;
  final String hint;
  final VoidCallback? onTap;
  final VoidCallback? onLongPressStart;
  final VoidCallback? onLongPressEnd;

  /// 次要动作用毛玻璃，仍然不是粉色。
  final bool glass;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return Expanded(
      child: GestureDetector(
        onTap: onTap,
        onLongPressStart: onLongPressStart == null
            ? null
            : (_) => onLongPressStart!(),
        onLongPressEnd: onLongPressEnd == null
            ? null
            : (_) => onLongPressEnd!(),
        onLongPressCancel: onLongPressEnd,
        child: Container(
          height: 62,
          decoration: BoxDecoration(
            color: glass ? Colors.white.withValues(alpha: .22) : c.aqua,
            borderRadius: Dim.brLargeCard,
            border: glass
                ? Border.all(color: Colors.white.withValues(alpha: .5))
                : null,
            boxShadow: glass
                ? null
                : [
                    BoxShadow(
                      color: c.aqua.withValues(alpha: .5),
                      blurRadius: 26,
                      offset: const Offset(0, 9),
                    ),
                  ],
          ),
          alignment: Alignment.center,
          padding: const EdgeInsets.symmetric(horizontal: Dim.s2),
          // 原型 .pbn：42px → 62 高，主字 t4/800，副字 t1。两行都只许一行：
          // 大字体 + 窄屏下换行会把 62 的格子撑爆（360 宽 × 1.2 实测）。
          child: Column(
            mainAxisSize: MainAxisSize.min,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: Dim.t4,
                  fontWeight: FontWeight.w800,
                  color: Colors.white,
                  height: 1.2,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                hint,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: Dim.t1,
                  fontWeight: FontWeight.w700,
                  color: Colors.white.withValues(alpha: .94),
                  height: 1.2,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 漂流瓶的三张素材。
enum BottleArt {
  /// `bottle.png`：透明底、横躺、瓶口朝右。能旋转、能沿抛物线飞——捞瓶、扔瓶、品牌标用它。
  sprite,

  /// `bottle-l.png`：海面整图，瓶口朝左上，自带水面。
  seaLeft,

  /// `bottle-r.png`：海面整图，瓶口朝右上，自带水面。
  seaRight,
}

/// 海面瓶位专用的整图素材（`bottle-l.png` / `bottle-r.png`）。
///
/// 这两张图是不透明的 3:2 小图，水面已经画在里面了。直接贴上去是一块方形贴纸，
/// 所以四边各做一段线性羽化（横竖各一层 dstIn），让它自己的水和海面的水接上；
/// 夜场再压一层偏蓝的 modulate，免得白天的亮水在深色海面上发光。
///
/// 外框仍是 [bottleSize] 见方，和 [BottleSprite] 一样占位，瓶位 / 气泡坐标不用改；
/// 图本身比框宽一点（[width]），从框里溢出来，让瓶子在海面上有体量感。
class SeaBottleImage extends StatelessWidget {
  const SeaBottleImage({
    super.key,
    required this.art,
    this.width = 64,
    this.night = false,
  });

  final BottleArt art;
  final double width;
  final bool night;

  static const leftAsset = 'assets/images/bottle-l.png';
  static const rightAsset = 'assets/images/bottle-r.png';

  /// 夜晚变体：同一张图把白天的亮水面抠掉（按色相 / 饱和度去掉青蓝色），
  /// 只剩瓶身、软木塞与白色浪纹——压在深色海面上就是月光下的瓶子。
  static const leftNightAsset = 'assets/images/bottle-l-night.png';
  static const rightNightAsset = 'assets/images/bottle-r-night.png';

  static String assetOf(BottleArt art, {bool night = false}) {
    final right = art == BottleArt.seaRight;
    if (night) return right ? rightNightAsset : leftNightAsset;
    return right ? rightAsset : leftAsset;
  }

  /// 边缘羽化占每边的比例。
  static const _feather = .2;

  @override
  Widget build(BuildContext context) {
    final height = width * 2 / 3;
    Widget img = Image.asset(
      assetOf(art, night: night),
      width: width,
      height: height,
      fit: BoxFit.cover,
      filterQuality: FilterQuality.medium,
    );
    img = _fade(img, Alignment.centerLeft, Alignment.centerRight);
    img = _fade(img, Alignment.topCenter, Alignment.bottomCenter);
    if (night) {
      // 夜晚变体已经没有水面,只需一团椭圆月光垫在底下让瓶子从深色海面上浮出来。
      // 光晕用椭圆径向渐变垫在底下,不用 BoxShadow——阴影会沿羽化矩形描出一个药丸形。
      img = Stack(
        alignment: Alignment.center,
        children: [
          Positioned.fill(
            child: DecoratedBox(
              decoration: const BoxDecoration(
                gradient: RadialGradient(
                  colors: [Color(0x66FFFFFF), Color(0x00FFFFFF)],
                  stops: [.2, 1],
                  radius: .75,
                ),
              ),
            ),
          ),
          img,
        ],
      );
    }
    return SizedBox(
      width: bottleSize.width,
      height: bottleSize.height,
      child: OverflowBox(
        minWidth: width,
        maxWidth: width,
        minHeight: height,
        maxHeight: height,
        child: img,
      ),
    );
  }

  static Widget _fade(Widget child, Alignment begin, Alignment end) {
    return ShaderMask(
      blendMode: BlendMode.dstIn,
      shaderCallback: (rect) => LinearGradient(
        begin: begin,
        end: end,
        colors: const [
          Color(0x00FFFFFF),
          Color(0xFFFFFFFF),
          Color(0xFFFFFFFF),
          Color(0x00FFFFFF),
        ],
        stops: const [0, _feather, 1 - _feather, 1],
      ).createShader(rect),
      child: child,
    );
  }
}

/// 漂流瓶素材（`assets/images/bottle.png`，原图横躺、瓶口朝右）。
///
/// 捞瓶上浮、扔瓶抛物线、登录 / 启动页的品牌标统一用它；海面瓶位改用
/// [SeaBottleImage] 的整图素材（见 [BottleArt]）。
/// [tilt] 是顺时针角度：0 = 横躺，-90 = 竖直瓶口朝上。外框是 [width] 见方，
/// 旋转后仍落在框内（素材长宽比约 2.4:1，对角线 < 边长 × 1.1，留了余量）。
class BottleSprite extends StatelessWidget {
  const BottleSprite({
    super.key,
    this.width = 54,
    this.tilt = 0,
    this.glow = false,
  });

  final double width;
  final double tilt;

  /// 深色背景上加一圈淡白光晕（夜场海面）。
  final bool glow;

  static const asset = 'assets/images/bottle.png';

  @override
  Widget build(BuildContext context) {
    Widget img = Image.asset(
      asset,
      width: width,
      fit: BoxFit.contain,
      filterQuality: FilterQuality.medium,
    );
    if (glow) {
      // 夜晚海面上的月光光晕:椭圆径向柔光垫在瓶子底下 + 瓶身略提亮,
      // 不用 BoxShadow(会沿方框描出一个圆角矩形)。
      img = ColorFiltered(
        colorFilter: const ColorFilter.matrix(<double>[
          1.06,
          0,
          0,
          0,
          6,
          0,
          1.06,
          0,
          0,
          6,
          0,
          0,
          1.06,
          0,
          10,
          0,
          0,
          0,
          1,
          0,
        ]),
        child: img,
      );
      img = Stack(
        alignment: Alignment.center,
        children: [
          Positioned.fill(
            child: DecoratedBox(
              decoration: const BoxDecoration(
                gradient: RadialGradient(
                  colors: [Color(0x73FFFFFF), Color(0x00FFFFFF)],
                  stops: [.1, 1],
                  radius: .72,
                ),
              ),
            ),
          ),
          img,
        ],
      );
    }
    return SizedBox(
      width: width,
      height: width,
      child: Center(
        child: Transform.rotate(angle: tilt * pi / 180, child: img),
      ),
    );
  }
}
