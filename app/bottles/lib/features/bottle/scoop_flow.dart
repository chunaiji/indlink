import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../core/utils/anon_meta.dart';
import '../../domain/models/bottle.dart';
import '../../domain/models/user.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/sea_scene.dart';

/// B1a 捞取中。
///
/// 「捞」是写在产品名字里的动词，不该只是点一下就跳页。这 2.5 秒是**唯一不可复制的东西**：
/// 拆盲盒的期待感是留存钩子，也让「今日还剩 8 次」从惩罚变成稀缺。
///
/// 按下瞬间就发请求，动画与网络并行——2.5 秒足够覆盖弱网，用户感知不到加载。
class ScoopRitualLayer extends StatefulWidget {
  const ScoopRitualLayer({super.key, required this.ready, this.byTap = false});

  /// 数据已到位，可以提示「松手打开它」。
  final bool ready;

  /// 点击触发：仪式放完自动开瓶，此时提示「松手」是错的——用户根本没在按。
  final bool byTap;

  @override
  State<ScoopRitualLayer> createState() => _ScoopRitualLayerState();
}

class _ScoopRitualLayerState extends State<ScoopRitualLayer>
    with SingleTickerProviderStateMixin {
  late final AnimationController _rise = AnimationController(
    vsync: this,
    duration: Motion.scoopRitual,
  )..forward();

  @override
  void dispose() {
    _rise.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);

    return Positioned.fill(
      child: ColoredBox(
        color: const Color(0x57081424),
        child: Stack(
          alignment: Alignment.center,
          children: [
            const RippleRings(size: 240),
            // 瓶子从深处浮上来
            AnimatedBuilder(
              animation: _rise,
              builder: (context, child) {
                final t = Curves.easeInOut.transform(_rise.value);
                return Transform.translate(
                  offset: Offset(0, 120 - 120 * t),
                  child: Opacity(opacity: (t * 1.6).clamp(0, 1), child: child),
                );
              },
              // 被捞上来的瓶子接近竖直，像是从水里提起来的。
              child: const FloatingBottle(tilt: -75),
            ),
            Positioned(
              bottom: 120,
              child: AnimatedOpacity(
                opacity: widget.ready ? 1 : 0,
                duration: const Duration(milliseconds: 220),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      l.oceanSomethingBit,
                      style: const TextStyle(
                        fontSize: Dim.t4,
                        fontWeight: FontWeight.w800,
                        color: Colors.white,
                      ),
                    ),
                    const SizedBox(height: Dim.s1),
                    Text(
                      widget.byTap ? l.oceanOpeningNow : l.oceanReleaseToOpen,
                      style: TextStyle(
                        fontSize: Dim.t2,
                        color: Colors.white.withValues(alpha: .85),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// B1b 开瓶。
///
/// 信纸从瓶口展开，暖米色纸张压在冷色海面上——**全 App 唯一一处纸质材质**，
/// 用来标记「这是一封信，不是一条消息」。
///
/// 必须给「放回海里」：不是每个瓶子都想回，强制回信会让人不敢捞。
class LetterLayer extends StatefulWidget {
  const LetterLayer({
    super.key,
    required this.bottle,
    required this.onPutBack,
    required this.onReply,
  });

  final Bottle bottle;
  final VoidCallback onPutBack;
  final VoidCallback onReply;

  @override
  State<LetterLayer> createState() => _LetterLayerState();
}

class _LetterLayerState extends State<LetterLayer>
    with SingleTickerProviderStateMixin {
  late final AnimationController _open = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 340),
  )..forward();

  @override
  void dispose() {
    _open.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final b = widget.bottle;

    return Positioned.fill(
      child: ColoredBox(
        color: const Color(0x80081424),
        child: SafeArea(
          // 两个动作**长在信纸上**，不再用 DockButton 悬在屏幕底部。
          //
          // 原来那对按钮与海面的「扔一个 / 捞一个」是同一个组件、同一个位置，
          // 叠在一起既看不出层级、也容易点错。而「放回」与「回信」本来就是
          // 对**这封信**的处理，长在纸上才说得通——信收起来，选择也跟着消失。
          child: Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.symmetric(vertical: Dim.s5),
              child: AnimatedBuilder(
                animation: _open,
                builder: (context, child) {
                  final t = Motion.emphasized.transform(_open.value);
                  return Transform.scale(
                    scaleY: 0.3 + 0.7 * t,
                    alignment: Alignment.topCenter,
                    child: Opacity(opacity: t, child: child),
                  );
                },
                child: _Letter(
                  bottle: b,
                  onPutBack: widget.onPutBack,
                  onReply: widget.onReply,
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 信纸底部的两个动作。
///
/// 做成纸上的一行而不是浮起来的大按钮：这是读完一封信之后的选择，
/// 不是页面级的主操作，不该抢「捞下一个」的位置。
class _LetterActions extends StatelessWidget {
  const _LetterActions({required this.onPutBack, required this.onReply});

  final VoidCallback onPutBack;
  final VoidCallback onReply;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return Row(
      children: [
        Expanded(
          child: _PaperButton(
            label: l.oceanPutBack,
            hint: l.oceanPutBackHint,
            onTap: onPutBack,
          ),
        ),
        const SizedBox(width: Dim.s3),
        Expanded(
          child: _PaperButton(
            label: l.oceanWriteReply,
            hint: l.oceanWriteReplyHint,
            onTap: onReply,
            primary: true,
          ),
        ),
      ],
    );
  }
}

class _PaperButton extends StatelessWidget {
  const _PaperButton({
    required this.label,
    required this.hint,
    required this.onTap,
    this.primary = false,
  });

  final String label;
  final String hint;
  final VoidCallback onTap;
  final bool primary;

  @override
  Widget build(BuildContext context) {
    // 配色跟着信纸走(暖米色/墨褐)，而不是海面的青蓝——
    // 这一层在视觉上属于「信」，不属于「海」。
    const ink = Color(0xFF3A3226);
    const soft = Color(0xFF8A7B63);
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Container(
        height: 52,
        decoration: BoxDecoration(
          color: primary ? ink : Colors.transparent,
          borderRadius: Dim.brButton,
          border: primary
              ? null
              : Border.all(color: soft.withValues(alpha: .45)),
        ),
        alignment: Alignment.center,
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text(
              label,
              style: TextStyle(
                fontSize: Dim.t3,
                fontWeight: FontWeight.w800,
                height: 1.15,
                color: primary ? const Color(0xFFF7F0E1) : ink,
              ),
            ),
            Text(
              hint,
              style: TextStyle(
                fontSize: Dim.t0,
                height: 1.3,
                color: primary
                    ? const Color(0xFFF7F0E1).withValues(alpha: .7)
                    : soft,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Letter extends ConsumerWidget {
  const _Letter({
    required this.bottle,
    required this.onPutBack,
    required this.onReply,
  });

  final VoidCallback onPutBack;
  final VoidCallback onReply;

  final Bottle bottle;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = L.of(context);
    final b = bottle;
    final genderLabel = switch (b.author.gender) {
      Gender.female => l.commonFemale,
      Gender.male => l.commonMale,
      Gender.secret => '',
    };

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: Dim.s5),
      padding: const EdgeInsets.all(Dim.s5),
      decoration: BoxDecoration(
        color: const Color(0xFFF7F0E1),
        borderRadius: Dim.brLargeCard,
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: .35),
            blurRadius: 30,
            offset: const Offset(0, 12),
          ),
        ],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            b.content,
            style: const TextStyle(
              fontSize: Dim.t4,
              height: 1.75,
              color: Color(0xFF3A3226),
            ),
          ),
          const SizedBox(height: Dim.s5),
          Row(
            children: [
              Text(
                anonMeta(
                  anonSenderName(ref, l.commonAnonymous),
                  genderLabel,
                  b.author.age,
                ),
                style: const TextStyle(
                  fontSize: Dim.t1,
                  color: Color(0xFF8A7B63),
                ),
              ),
              const Spacer(),
              // 「漂了 3 天 · Mumbai → 你这里」是这屏的情绪核心：
              // 异步是产品本质，这句话是它唯一一次被说出来的地方。
              Flexible(
                child: Text(
                  b.driftedDays > 0
                      ? l.oceanDriftedFromTo(b.driftedDays, b.city ?? '')
                      : l.oceanDriftedRecent(b.city ?? ''),
                  textAlign: TextAlign.right,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: Dim.t1,
                    color: Color(0xFF8A7B63),
                  ),
                ),
              ),
            ],
          ),
          // 信纸内的分隔线 + 两个动作：读完了再做选择，顺序与阅读一致。
          const SizedBox(height: Dim.s4),
          const Divider(height: 1, color: Color(0x338A7B63)),
          const SizedBox(height: Dim.s3),
          _LetterActions(onPutBack: onPutBack, onReply: onReply),
        ],
      ),
    );
  }
}

/// B1 抛瓶。
///
/// 瓶子沿抛物线飞出去（`easeOutQuad 600ms`），落水处起涟漪（`400ms`），随后缩小淡出。
///
/// 这是仪式感动画，**全程不可阻塞**：瓶子早已经由接口落库，动画只是把结果演出来，
/// 失败态由调用方在动画之外处理，不能等动画放完。
class CastBottleOverlay extends StatefulWidget {
  const CastBottleOverlay({super.key, required this.onDone});

  final VoidCallback onDone;

  @override
  State<CastBottleOverlay> createState() => _CastBottleOverlayState();
}

class _CastBottleOverlayState extends State<CastBottleOverlay>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl = AnimationController(
    vsync: this,
    duration: Motion.castArc + Motion.castSplash,
  );

  /// 抛掷段占整段的比例。
  late final double _arcRatio =
      Motion.castArc.inMilliseconds /
      (Motion.castArc.inMilliseconds + Motion.castSplash.inMilliseconds);

  @override
  void initState() {
    super.initState();
    _ctrl.forward().then((_) {
      if (mounted) widget.onDone();
    });
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (MediaQuery.disableAnimationsOf(context)) return const SizedBox.shrink();

    return Positioned.fill(
      child: IgnorePointer(
        child: LayoutBuilder(
          builder: (context, box) {
            // 起点贴着「扔一个」按钮，终点落在海面中部偏右。
            final start = Offset(box.maxWidth * .28, box.maxHeight * .86);
            final end = Offset(box.maxWidth * .58, box.maxHeight * .58);
            // 控制点抬到两端上方，抛出一条真正的弧
            final control = Offset(
              (start.dx + end.dx) / 2,
              box.maxHeight * .34,
            );

            return AnimatedBuilder(
              animation: _ctrl,
              builder: (context, _) {
                final v = _ctrl.value;
                final flying = v < _arcRatio;
                final t = flying
                    ? Curves.easeOutQuad.transform(v / _arcRatio)
                    : 1.0;
                final pos = _quadratic(start, control, end, t);

                return Stack(
                  children: [
                    if (!flying)
                      Positioned(
                        left: end.dx - 60,
                        top: end.dy - 60,
                        child: _Splash(
                          progress: (v - _arcRatio) / (1 - _arcRatio),
                        ),
                      ),
                    Positioned(
                      left: pos.dx - 12,
                      top: pos.dy - 18,
                      child: Opacity(
                        opacity: flying ? 1 : 0,
                        child: Transform.rotate(
                          angle: t * 2.6,
                          child: const FloatingBottle(),
                        ),
                      ),
                    ),
                  ],
                );
              },
            );
          },
        ),
      ),
    );
  }

  static Offset _quadratic(Offset p0, Offset p1, Offset p2, double t) {
    final u = 1 - t;
    return Offset(
      u * u * p0.dx + 2 * u * t * p1.dx + t * t * p2.dx,
      u * u * p0.dy + 2 * u * t * p1.dy + t * t * p2.dy,
    );
  }
}

class _Splash extends StatelessWidget {
  const _Splash({required this.progress});

  final double progress;

  @override
  Widget build(BuildContext context) {
    final t = Curves.easeOut.transform(progress.clamp(0, 1));
    return SizedBox(
      width: 120,
      height: 120,
      child: CustomPaint(painter: _SplashPainter(t)),
    );
  }
}

class _SplashPainter extends CustomPainter {
  _SplashPainter(this.t);

  final double t;

  @override
  void paint(Canvas canvas, Size size) {
    final center = size.center(Offset.zero);
    for (var i = 0; i < 2; i++) {
      final p = (t - i * .2).clamp(0.0, 1.0);
      if (p == 0) continue;
      canvas.drawCircle(
        center,
        size.width / 2 * p,
        Paint()
          ..style = PaintingStyle.stroke
          ..strokeWidth = 2.5 - i
          ..color = Colors.white.withValues(alpha: (1 - p) * .8),
      );
    }
  }

  @override
  bool shouldRepaint(covariant _SplashPainter old) => old.t != t;
}
