import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../core/catalog.dart';
import '../../core/design/tokens.dart';
import '../../core/utils/birthday.dart';
import '../../core/media/image_pipeline.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/overlays.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/headers.dart';
import 'auth_controller.dart';

/// A4 完善资料。
///
/// 拆成两步而不是一长屏：第 1 步是身份（头像 / 名字 / 年龄 / 性别），
/// 第 2 步是匹配维度（语言 / 兴趣 / 定位）。后者直接决定发现页的排序质量，
/// 单独一屏能把完成率做上去。
///
/// **定位拒绝要能降级，不可阻断注册流程。**
///
/// 语言 / 兴趣 / 年龄范围 / 最少兴趣数全部来自 `GET /api/profile-options`
/// （后台「App 完善资料」分组）。拉不到就退回内置 [Catalog]——
/// 配置读失败不该让新用户卡在注册最后一步。
/// 类别：表单页（规范 §6）
/// 固定区：NavBar（含步骤）、进度条、底部主按钮
/// 弹性区：无
/// 可滚动区：每一步的表单本体
/// 键盘：昵称 / 年龄输入时表单区随 resizeToAvoidBottomInset 收缩后可滚
class OnboardingPage extends ConsumerWidget {
  const OnboardingPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = L.of(context);
    final draft = ref.watch(onboardingProvider);
    final notifier = ref.read(onboardingProvider.notifier);
    final opts = ref.watch(profileOptionsProvider).value;
    final minInterests = opts?.minInterests ?? OnboardingDraft.minInterests;
    final minAge = opts?.minAge ?? OnboardingDraft.minAge;
    final maxAge = opts?.maxAge ?? OnboardingDraft.maxAge;

    return Scaffold(
      appBar: NavBar(
        title: l.profileCompleteTitle,
        subtitle: l.profileStepOf(draft.step + 1, OnboardingDraft.totalSteps),
        onBack: draft.step == 0 ? () {} : notifier.back,
        leading: draft.step == 0 ? const SizedBox(width: Dim.gutter) : null,
      ),
      body: Column(
        children: [
          ThinProgressBar(
            value: (draft.step + 1) / OnboardingDraft.totalSteps,
            height: 3,
            gradient: true,
          ),
          Expanded(
            child: AnimatedSwitcher(
              duration: const Duration(milliseconds: 220),
              child: draft.step == 0
                  ? const _IdentityStep(key: ValueKey('identity'))
                  : const _MatchingStep(key: ValueKey('matching')),
            ),
          ),
          BottomActionBar(
            child: draft.step == 0
                ? AppButton(
                    label: l.commonConfirm,
                    onTap: draft.canGoNextWith(minAge, maxAge)
                        ? () => _confirmGender(context, ref)
                        : null,
                  )
                : AppButton(
                    label: l.profileStartDrifting,
                    loading: draft.busy,
                    onTap: draft.canFinishWith(minInterests)
                        ? notifier.submit
                        : null,
                  ),
          ),
        ],
      ),
    );
  }
}

class _IdentityStep extends ConsumerStatefulWidget {
  const _IdentityStep({super.key});

  @override
  ConsumerState<_IdentityStep> createState() => _IdentityStepState();
}

class _IdentityStepState extends ConsumerState<_IdentityStep> {
  late final TextEditingController _nickname = TextEditingController(
    text: ref.read(onboardingProvider).nickname,
  );

  @override
  void dispose() {
    _nickname.dispose();
    super.dispose();
  }

  /// 出生日期选择器。门槛来自后台 `app_profile_min_age` / `app_profile_max_age`：
  /// 最晚只能选到 minAge 年前的今天，最早到 maxAge 年前，选出来的日期天然在范围内。
  Future<void> _pickBirthday(int minAge, int maxAge) async {
    final now = DateTime.now();
    final last = DateTime(now.year - minAge, now.month, now.day);
    final first = DateTime(now.year - maxAge, now.month, now.day);
    var current = parseBirthday(ref.read(onboardingProvider).birthday) ?? last;
    if (current.isAfter(last)) current = last;
    if (current.isBefore(first)) current = first;
    final picked = await showDatePicker(
      context: context,
      initialDate: current,
      firstDate: first,
      lastDate: last,
    );
    if (picked != null) {
      ref.read(onboardingProvider.notifier).setBirthday(formatBirthday(picked));
    }
  }

  Future<void> _pickAvatar() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery);
    if (picked == null) return;
    final compressed = await ImagePipeline.compress(picked.path);
    ref.read(onboardingProvider.notifier).setAvatar(compressed.path);
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final draft = ref.watch(onboardingProvider);
    final notifier = ref.read(onboardingProvider.notifier);
    final opts = ref.watch(profileOptionsProvider).value;
    final minAge = opts?.minAge ?? OnboardingDraft.minAge;
    final maxAge = opts?.maxAge ?? OnboardingDraft.maxAge;
    final ageOk = draft.age >= minAge && draft.age <= maxAge;

    return ListView(
      padding: const EdgeInsets.fromLTRB(
        Dim.gutter,
        Dim.s5,
        Dim.gutter,
        Dim.s5,
      ),
      children: [
        // 头像居中放大 + 相机角标：它是资料页的第一眼，值得被强调。
        Center(
          child: GestureDetector(
            onTap: _pickAvatar,
            behavior: HitTestBehavior.opaque,
            child: Column(
              children: [
                Stack(
                  clipBehavior: Clip.none,
                  children: [
                    // 原型 .avup：74px → 110 圆、虚线 line 描边、sea 底、中间 +。
                    draft.avatarPath == null
                        ? SizedBox(
                            width: 110,
                            height: 110,
                            child: CustomPaint(
                              painter: _DashedCirclePainter(color: c.line),
                              child: DecoratedBox(
                                decoration: BoxDecoration(
                                  shape: BoxShape.circle,
                                  color: c.sea,
                                ),
                                child: Icon(
                                  Icons.add_rounded,
                                  color: c.ink3,
                                  size: 40,
                                ),
                              ),
                            ),
                          )
                        : AvatarRing(avatar: draft.avatarPath, size: 110),
                    // 相机角标：24px → 36，brand 底，surface 边 3。
                    Positioned(
                      right: -1,
                      bottom: -1,
                      child: Container(
                        width: 36,
                        height: 36,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: c.brand,
                          border: Border.all(color: c.surface, width: 3),
                        ),
                        child: const Icon(
                          Icons.camera_alt_rounded,
                          size: 18,
                          color: Colors.white,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: Dim.s2),
                Text(
                  l.profileUploadAvatarHint,
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: Dim.s5),
        TextField(
          controller: _nickname,
          onChanged: notifier.setNickname,
          maxLength: 16,
          decoration: InputDecoration(
            hintText: l.profileNicknameHint,
            counterText: '',
          ),
        ),
        const SizedBox(height: Dim.s3),
        // 出生日期代替手填年龄（真机反馈）：年龄由它推出，门槛不再写死 18。
        ValueField(
          value: draft.birthday ?? l.profileEditBirthdayHint,
          leading: Icons.cake_outlined,
          onTap: () => _pickBirthday(minAge, maxAge),
        ),
        if (draft.birthday != null && !ageOk) ...[
          const SizedBox(height: Dim.s1),
          // 越界当场标红，而不是让「确认」莫名点不动。
          Text(
            l.profileAgeInvalid(minAge, maxAge),
            style: TextStyle(fontSize: Dim.t0, color: c.warn),
          ),
        ],
        const SizedBox(height: Dim.s4),
        SectionLabel(l.profileGender),
        const SizedBox(height: Dim.s2),
        SegmentedRow(
          labels: [l.commonFemale, l.commonMale, l.commonSecret],
          index: draft.genderIndex,
          onChanged: notifier.setGender,
        ),
      ],
    );
  }
}

class _MatchingStep extends ConsumerWidget {
  const _MatchingStep({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = L.of(context);
    final draft = ref.watch(onboardingProvider);
    final notifier = ref.read(onboardingProvider.notifier);
    final opts = ref.watch(profileOptionsProvider).value;
    final zhLocale = Localizations.localeOf(context).languageCode == 'zh';

    // 后台配了就用后台的，没配（或没拉到）退回内置表。
    final languages = opts != null && opts.languages.isNotEmpty
        ? opts.languages
        : Catalog.languages;
    final interests = opts != null && opts.interests.isNotEmpty
        ? [for (final i in opts.interests) (i.key, i.label(zhLocale: zhLocale))]
        : [
            for (final key in Catalog.interests.keys)
              (key, Catalog.interest(context, key)),
          ];
    final minInterests = opts?.minInterests ?? OnboardingDraft.minInterests;

    return ListView(
      padding: const EdgeInsets.fromLTRB(
        Dim.gutter,
        Dim.s5,
        Dim.gutter,
        Dim.s5,
      ),
      children: [
        SectionLabel(l.profileLanguages),
        const SizedBox(height: Dim.s2),
        // 语言多选「下拉」（原型 .selfield）：点开底部选择器勾选，胶囊上的 × 直接移除。
        SelectField(
          selected: draft.languages,
          placeholder: l.profileLanguagePick,
          onTap: () => _showLanguageSheet(context, ref, languages),
          onRemove: notifier.toggleLanguage,
        ),
        const SizedBox(height: Dim.s5),
        SectionLabel(l.profileInterests(minInterests)),
        const SizedBox(height: Dim.s3),
        Wrap(
          spacing: Dim.s2,
          runSpacing: Dim.s2,
          children: [
            for (final (key, label) in interests)
              PillChip(
                label: label,
                selected: draft.interests.contains(key),
                onTap: () => notifier.toggleInterest(key),
              ),
          ],
        ),
        if (!draft.canFinishWith(minInterests)) ...[
          const SizedBox(height: Dim.s3),
          Text(
            l.profileNeedInterests(minInterests - draft.interests.length),
            style: TextStyle(fontSize: Dim.t1, color: context.c.ink3),
          ),
        ],
        const SizedBox(height: Dim.s5),
        ListGroup(
          children: [
            ListRowItem(
              title: l.profileEnableLocation,
              subtitle: l.profileEnableLocationHint,
              showChevron: false,
              trailing: Switch(
                value: draft.locationEnabled,
                onChanged: notifier.setLocation,
              ),
            ),
          ],
        ),
      ],
    );
  }
}

/// 语言选择底部弹层：勾选即时写回 draft（多选）。
void _showLanguageSheet(
  BuildContext context,
  WidgetRef ref,
  List<String> options,
) {
  final c = context.c;
  final l = L.of(context);
  // 统一走 showAppSheet：SheetShell 自带圆角、抓手、左右 gutter 与底部安全区，
  // 这里只排内容，不再自己包 SafeArea / padding。
  showAppSheet<void>(
    context,
    builder: (_) => Consumer(
      builder: (context, ref, _) {
        final selected = ref.watch(onboardingProvider).languages;
        final notifier = ref.read(onboardingProvider.notifier);
        return Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.only(top: Dim.s2, bottom: Dim.s2),
              child: Text(
                l.profileLanguagePick,
                style: TextStyle(
                  fontSize: Dim.t4,
                  fontWeight: FontWeight.w800,
                  color: c.ink,
                  height: 1.25,
                ),
              ),
            ),
            for (final lang in options)
              CheckboxListTile(
                value: selected.contains(lang),
                onChanged: (_) => notifier.toggleLanguage(lang),
                title: Text(
                  lang,
                  style: TextStyle(fontSize: Dim.t3, color: c.ink, height: 1.4),
                ),
                controlAffinity: ListTileControlAffinity.trailing,
                activeColor: c.aqua,
                contentPadding: EdgeInsets.zero,
              ),
            const SizedBox(height: Dim.s3),
            AppButton(
              label: l.commonConfirm,
              onTap: () => Navigator.of(context).pop(),
            ),
          ],
        );
      },
    ),
  );
}

/// 头像占位的虚线圆环（Flutter 没有 dashed border）。
class _DashedCirclePainter extends CustomPainter {
  const _DashedCirclePainter({required this.color});

  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5;
    final r = size.shortestSide / 2 - 0.75;
    final center = size.center(Offset.zero);
    const dash = 5.0;
    const gap = 4.0;
    final circumference = 2 * math.pi * r;
    final count = (circumference / (dash + gap)).floor();
    final sweep = (circumference / count - gap) / r;
    final step = circumference / count / r;
    for (var i = 0; i < count; i++) {
      canvas.drawArc(
        Rect.fromCircle(center: center, radius: r),
        i * step,
        sweep,
        false,
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(covariant _DashedCirclePainter old) => old.color != color;
}

/// 性别选定后服务端不再放行修改（匹配 / 筛选都靠它），离开这一步前说清楚。
Future<void> _confirmGender(BuildContext context, WidgetRef ref) async {
  final l = L.of(context);
  final draft = ref.read(onboardingProvider);
  final label = switch (draft.genderIndex) {
    0 => l.commonFemale,
    1 => l.commonMale,
    _ => l.commonSecret,
  };
  final ok = await showAppModal<bool>(
    context,
    builder: (ctx) => ModalCard(
      icon: Icons.wc_rounded,
      title: l.profileGenderLockTitle,
      body: Text(l.profileGenderLockBody(label)),
      actions: [
        AppButton(
          label: l.commonConfirm,
          onTap: () => Navigator.of(ctx).pop(true),
        ),
        AppButton(
          label: l.profileGenderRethink,
          kind: BtnKind.ghost,
          onTap: () => Navigator.of(ctx).pop(false),
        ),
      ],
    ),
  );
  if (ok == true) ref.read(onboardingProvider.notifier).next();
}
