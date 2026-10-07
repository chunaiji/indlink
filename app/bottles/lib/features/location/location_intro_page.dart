import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/headers.dart';
import 'location_controller.dart';

/// A7 定位权限说明。
///
/// 类别：场景页（规范 §6）
/// 固定区：底部两个按钮（开启 primary / 暂不 oauth 样式）
/// 弹性区：说明块上下各一份留白（1:1 居中），内容超出一屏时留白归零再滚动
/// 可滚动区：说明块
/// 键盘：无
///
/// 进主界面之后再问，而不是在注册流程里：注册时多一个权限框会明显拉低完成率，
/// 而 iOS 的定位框只给一次机会。
class LocationIntroPage extends ConsumerStatefulWidget {
  const LocationIntroPage({super.key});

  @override
  ConsumerState<LocationIntroPage> createState() => _LocationIntroPageState();
}

class _LocationIntroPageState extends ConsumerState<LocationIntroPage> {
  Future<void> _close() async {
    await ref.read(prefsProvider).setLocationAsked();
    if (mounted) Navigator.of(context).pop();
  }

  Future<void> _enable() async {
    await ref.read(locationControllerProvider.notifier).requestAndLocate();
    await _close();
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final busy = ref.watch(locationControllerProvider).busy;

    return Scaffold(
      backgroundColor: c.page,
      body: SafeArea(
        bottom: false,
        child: Column(
          children: [
            Expanded(
              child: CustomScrollView(
                slivers: [
                  SliverFillRemaining(
                    hasScrollBody: false,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: Dim.gutter,
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          const Spacer(),
                          // 原型：56×56 粉紫渐变块作图标底，标题 t5，引导语 t2 ink2。
                          Center(
                            child: Container(
                              width: 56,
                              height: 56,
                              decoration: const BoxDecoration(
                                borderRadius: Dim.brCard,
                                gradient: LinearGradient(
                                  begin: Alignment.topLeft,
                                  end: Alignment.bottomRight,
                                  colors: [
                                    Color(0xFFFFD9E2),
                                    Color(0xFFEAD8FF),
                                    Color(0xFFCDE9F4),
                                  ],
                                  stops: [0, .54, 1],
                                ),
                              ),
                              child: Icon(
                                Icons.place_outlined,
                                size: 26,
                                color: c.ink2,
                              ),
                            ),
                          ),
                          const SizedBox(height: Dim.s4),
                          Text(
                            l.locationIntroTitle,
                            textAlign: TextAlign.center,
                            style: TextStyle(
                              fontSize: Dim.t5,
                              fontWeight: FontWeight.w800,
                              height: 1.25,
                              letterSpacing: -0.2,
                              color: c.ink,
                            ),
                          ),
                          const SizedBox(height: Dim.s2),
                          Text(
                            l.locationIntroLead,
                            textAlign: TextAlign.center,
                            style: TextStyle(
                              fontSize: Dim.t2,
                              height: 1.6,
                              color: c.ink2,
                            ),
                          ),
                          const SizedBox(height: Dim.s4),
                          // 三条理由走 .lst 分组卡：标题 t3/700，副文 t2 ink3。
                          ListGroup(
                            children: [
                              ListRowItem(
                                title: l.locationReasonDistance,
                                subtitle: l.locationReasonDistanceSub,
                                showChevron: false,
                                roomy: true,
                              ),
                              ListRowItem(
                                title: l.locationReasonNearby,
                                subtitle: l.locationReasonNearbySub,
                                showChevron: false,
                                roomy: true,
                              ),
                              ListRowItem(
                                title: l.locationReasonPlace,
                                subtitle: l.locationReasonPlaceSub,
                                showChevron: false,
                                roomy: true,
                              ),
                            ],
                          ),
                          const SizedBox(height: Dim.s4),
                          NoticeBanner(
                            icon: Icons.lock_outline_rounded,
                            text: l.locationIntroPrivacy,
                          ),
                          const Spacer(),
                          const SizedBox(height: Dim.s4),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
            BottomActionBar(
              children: [
                AppButton(
                  label: l.locationEnable,
                  loading: busy,
                  onTap: _enable,
                ),
                AppButton(
                  label: l.locationLater,
                  kind: BtnKind.oauth,
                  onTap: busy ? null : _close,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
