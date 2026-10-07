import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/catalog.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../domain/models/engagement.dart';
import '../../domain/models/user.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import 'discover_controller.dart';

/// C2 筛选。
///
/// 条件本地持久化后随请求下发。地理筛选需要给 `User` 的 lat/lng 建索引，
/// 或引入 geohash 前缀查询——直接 `WHERE` 算距离全表扫会很慢。
/// 类别：表单页（规范 §6）
/// 固定区：NavBar（✕ / 重置）、底部「应用筛选 · N 人符合」
/// 弹性区：无
/// 可滚动区：答题卡 + 距离 + 性别 + 年龄
/// 键盘：无
class FilterPage extends ConsumerStatefulWidget {
  const FilterPage({super.key});

  @override
  ConsumerState<FilterPage> createState() => _FilterPageState();
}

class _FilterPageState extends ConsumerState<FilterPage> {
  late DiscoverFilter _draft = ref.read(discoverFilterProvider);

  /// 答题匹配本地草稿（qkey → 选中项下标）。null = 尚未改动，
  /// 应用时不覆盖服务端已存的答案；首次改动时从服务端答案 seed。
  Map<String, String>? _quizDraft;

  static const _maxDistance = 100;

  void _patch(DiscoverFilter next) => setState(() => _draft = next);

  /// 年龄区间弹层：滑块在弹层里改，页面上只留一个值字段（原型 C2 .fieldr）。
  Future<void> _pickAge() async {
    var range = RangeValues(_draft.minAge.toDouble(), _draft.maxAge.toDouble());
    final picked = await showAppSheet<RangeValues>(
      context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheet) {
          final l = L.of(ctx);
          final c = ctx.c;
          return Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l.discoverAgeRange,
                style: TextStyle(
                  fontSize: Dim.t4,
                  fontWeight: FontWeight.w800,
                  height: 1.3,
                  color: c.ink,
                ),
              ),
              const SizedBox(height: Dim.s3),
              Center(
                child: Text(
                  l.discoverAgeRangeValue(
                    range.start.round(),
                    range.end.round(),
                  ),
                  style: TextStyle(
                    fontSize: Dim.t5,
                    fontWeight: FontWeight.w800,
                    height: 1.25,
                    color: c.ink,
                  ),
                ),
              ),
              RangeSlider(
                values: range,
                min: 18,
                max: 60,
                divisions: 42,
                onChanged: (v) => setSheet(() => range = v),
              ),
              const SizedBox(height: Dim.s3),
              AppButton(
                label: l.commonConfirm,
                onTap: () => Navigator.of(ctx).pop(range),
              ),
            ],
          );
        },
      ),
    );
    if (picked == null) return;
    _patch(
      _draft.copyWith(minAge: picked.start.round(), maxAge: picked.end.round()),
    );
  }

  /// 单选：点已选项再点一次取消。首次改动时从服务端答案 seed 草稿。
  void _selectAnswer(
    Map<String, String> serverAnswers,
    String key,
    String idx,
  ) {
    setState(() {
      final draft = _quizDraft ??= {...serverAnswers};
      draft[key] == idx ? draft.remove(key) : draft[key] = idx;
    });
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final count = ref.watch(matchCountProvider(_draft));

    // 答题匹配（后台开关开时替代语言/兴趣维度）。开关关 → enabled=false，回落原筛选。
    final quiz = ref.watch(discoverQuizProvider).value;
    final quizEnabled = quiz?.enabled ?? false;
    final serverAnswers = quiz?.answers ?? const <String, String>{};
    final answers = _quizDraft ?? serverAnswers;

    return Scaffold(
      appBar: NavBar(
        title: l.discoverFiltersTitle,
        closeIcon: true,
        actions: [
          TextButton(
            onPressed: () => _patch(const DiscoverFilter()),
            child: Text(
              l.commonReset,
              style: TextStyle(
                fontSize: Dim.t2,
                fontWeight: FontWeight.w700,
                color: c.brand,
              ),
            ),
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: ListView(
              padding: const EdgeInsets.all(Dim.gutter),
              children: [
                // 答题匹配开启：用「选相同答案」替代语言/兴趣（加分排序，非硬筛选）。
                if (quizEnabled) ...[
                  // 原型 .sec 一行：「答题匹配 · 只推选了相同答案的人」。
                  SectionLabel(
                    '${l.discoverQuizTitle} · ${l.discoverQuizHint}',
                  ),
                  const SizedBox(height: Dim.s3),
                  // 原型 .qz：每题一张 sea 底 r3 卡，padding s3，题干 t3/700，选项胶囊。
                  for (final q in quiz!.questions) ...[
                    Container(
                      padding: const EdgeInsets.all(Dim.s3),
                      decoration: BoxDecoration(
                        color: c.sea,
                        borderRadius: Dim.brCard,
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            q.question,
                            style: TextStyle(
                              fontSize: Dim.t3,
                              fontWeight: FontWeight.w700,
                              height: 1.4,
                              color: c.ink,
                            ),
                          ),
                          const SizedBox(height: Dim.s2),
                          Wrap(
                            spacing: Dim.s2,
                            runSpacing: Dim.s2,
                            children: [
                              for (var i = 0; i < q.options.length; i++)
                                PillChip(
                                  label: q.options[i],
                                  selected: answers[q.key] == '$i',
                                  onTap: () =>
                                      _selectAnswer(serverAnswers, q.key, '$i'),
                                ),
                            ],
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: Dim.s3),
                  ],
                  const SizedBox(height: Dim.s2),
                ] else ...[
                  SectionLabel(l.discoverLanguage),
                  const SizedBox(height: Dim.s3),
                  Wrap(
                    spacing: Dim.s2,
                    runSpacing: Dim.s2,
                    children: [
                      for (final lang in Catalog.languages)
                        PillChip(
                          label: lang,
                          tone: ChipTone.aqua,
                          selected: _draft.languages.contains(lang),
                          onTap: () {
                            final next = [..._draft.languages];
                            next.contains(lang)
                                ? next.remove(lang)
                                : next.add(lang);
                            _patch(_draft.copyWith(languages: next));
                          },
                        ),
                    ],
                  ),
                  const SizedBox(height: Dim.s5),
                  SectionLabel(l.discoverInterest),
                  const SizedBox(height: Dim.s3),
                  Wrap(
                    spacing: Dim.s2,
                    runSpacing: Dim.s2,
                    children: [
                      for (final key in Catalog.interests.keys)
                        PillChip(
                          label: Catalog.interest(context, key),
                          selected: _draft.interests.contains(key),
                          onTap: () {
                            final next = [..._draft.interests];
                            next.contains(key)
                                ? next.remove(key)
                                : next.add(key);
                            _patch(_draft.copyWith(interests: next));
                          },
                        ),
                    ],
                  ),
                  const SizedBox(height: Dim.s5),
                ],
                SectionLabel(l.discoverDistance),
                Slider(
                  value: (_draft.maxDistanceKm ?? _maxDistance).toDouble(),
                  min: 1,
                  max: _maxDistance.toDouble(),
                  divisions: _maxDistance - 1,
                  onChanged: (v) => _patch(
                    v >= _maxDistance
                        ? _draft.copyWith(clearDistance: true)
                        : _draft.copyWith(maxDistanceKm: v.round()),
                  ),
                ),
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      l.discoverDistanceKm(1),
                      style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                    ),
                    Text(
                      _draft.maxDistanceKm == null
                          ? l.discoverDistanceUnlimited
                          : l.discoverDistanceKm(_draft.maxDistanceKm!),
                      style: TextStyle(
                        fontSize: Dim.t2,
                        fontWeight: FontWeight.w700,
                        color: c.ink,
                      ),
                    ),
                    Text(
                      l.discoverDistanceUnlimited,
                      style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                    ),
                  ],
                ),
                const SizedBox(height: Dim.s5),
                SectionLabel(l.profileGender),
                const SizedBox(height: Dim.s2),
                SegmentedRow(
                  labels: [l.commonAll, l.commonFemale, l.commonMale],
                  index: switch (_draft.gender) {
                    null => 0,
                    Gender.female => 1,
                    Gender.male => 2,
                    Gender.secret => 0,
                  },
                  onChanged: (i) => _patch(
                    i == 0
                        ? _draft.copyWith(clearGender: true)
                        : _draft.copyWith(
                            gender: i == 1 ? Gender.female : Gender.male,
                          ),
                  ),
                ),
                const SizedBox(height: Dim.s5),
                SectionLabel(l.discoverAgeRange),
                const SizedBox(height: Dim.s2),
                // 原型 .fieldr：年龄是一个值字段，点开才出区间滑块（弹层）。
                ValueField(
                  value: l.discoverAgeRangeValue(_draft.minAge, _draft.maxAge),
                  onTap: _pickAge,
                ),
              ],
            ),
          ),
          BottomActionBar(
            child: AppButton(
              label: l.discoverApplyFilter(count.value ?? 0),
              onTap: () async {
                // 答题匹配开启且改过答案：存答案 → 刷新题库缓存与牌堆(重新打分)。
                if (quizEnabled && _quizDraft != null) {
                  final answersStr = _quizDraft!.entries
                      .map((e) => '${e.key}:${e.value}')
                      .join(',');
                  await ref.read(discoverRepoProvider).saveQuiz(answersStr);
                  ref.invalidate(discoverQuizProvider);
                  ref.invalidate(deckProvider);
                }
                await ref.read(discoverFilterProvider.notifier).update(_draft);
                if (context.mounted) context.pop();
              },
            ),
          ),
        ],
      ),
    );
  }
}
