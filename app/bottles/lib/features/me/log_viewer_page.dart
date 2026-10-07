import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../core/logging/log_config.dart';
import '../../core/logging/log_export.dart';
import '../../core/logging/log_file_sink.dart';
import '../../core/logging/log_record.dart';
import '../../core/logging/logger.dart';
import '../../core/providers.dart';
import '../../ui/widgets/headers.dart';

/// 日志查看页（开发者面板）。
///
/// 藏在设置页「关于」连点 7 次之后，只给开发与客服用，因此界面不走 l10n，
/// 标签一律英文——译一套没人看的文案不值当。
class LogViewerPage extends ConsumerStatefulWidget {
  const LogViewerPage({super.key});

  @override
  ConsumerState<LogViewerPage> createState() => _LogViewerPageState();
}

class _LogViewerPageState extends ConsumerState<LogViewerPage> {
  /// 一次只读这么多行。
  ///
  /// 5 MB 的 JSONL 约两万行，全量读进来再渲染会直接卡死。
  /// 从尾部取最近的一段，翻到底再加载更多。
  static const _pageLines = 500;

  List<String> _lines = [];
  int _loaded = _pageLines;
  bool _hasMore = false;
  bool _loading = true;

  LogLevel? _level;
  LogTag? _tag;
  String _keyword = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    final all = <String>[];
    try {
      // 从最新的文件往回读，凑够要显示的行数就停——
      // 不是每次都把整个目录读进内存。
      for (final f in await FileLogSink.listFiles()) {
        all.insertAll(0, await f.readAsLines());
        if (all.length > _loaded) break;
      }
    } catch (_) {
      // 读不到就显示空列表，不弹错——这个页面本身不该制造故障。
    }
    if (!mounted) return;
    setState(() {
      _hasMore = all.length > _loaded;
      _lines = all.length > _loaded ? all.sublist(all.length - _loaded) : all;
      _loading = false;
    });
  }

  /// 过滤在渲染时做，不驻留一份过滤结果——关键词是边打边变的，
  /// 留副本只会让两边不同步。
  bool _match(Map<String, Object?> m) {
    if (_level != null) {
      final name = m['lv'];
      final lv = LogLevel.values.where((l) => l.name == name).firstOrNull;
      if (lv == null || lv.index < _level!.index) return false;
    }
    if (_tag != null && m['tag'] != _tag!.name) return false;
    if (_keyword.isNotEmpty) {
      if (!formatLogLine(m).toLowerCase().contains(_keyword.toLowerCase())) {
        return false;
      }
    }
    return true;
  }

  Color _levelColor(BuildContext context, Object? lv) {
    final c = context.c;
    switch (lv) {
      case 'error':
        return c.warn;
      case 'warn':
        return c.gold;
      default:
        return c.ink3;
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final prefs = ref.watch(prefsProvider);

    final rows = <Widget>[];
    for (final raw in _lines.reversed) {
      Map<String, Object?> m;
      try {
        m = jsonDecode(raw) as Map<String, Object?>;
      } catch (_) {
        m = {'raw': raw};
      }
      if (!_match(m)) continue;
      rows.add(
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 3),
          child: SelectableText(
            formatLogLine(m),
            style: TextStyle(
              fontFamily: 'monospace',
              fontSize: 11,
              height: 1.4,
              color: _levelColor(context, m['lv']),
            ),
          ),
        ),
      );
    }

    return Scaffold(
      appBar: NavBar(title: 'Logs'),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(Dim.gutter),
            child: Column(
              children: [
                Row(
                  children: [
                    Expanded(
                      child: DropdownButton<LogLevel?>(
                        isExpanded: true,
                        value: _level,
                        hint: const Text('all levels'),
                        items: [
                          const DropdownMenuItem(child: Text('all levels')),
                          for (final l in LogLevel.values)
                            DropdownMenuItem(
                              value: l,
                              child: Text('>= ${l.name}'),
                            ),
                        ],
                        onChanged: (v) => setState(() => _level = v),
                      ),
                    ),
                    const SizedBox(width: Dim.s3),
                    Expanded(
                      child: DropdownButton<LogTag?>(
                        isExpanded: true,
                        value: _tag,
                        hint: const Text('all tags'),
                        items: [
                          const DropdownMenuItem(child: Text('all tags')),
                          for (final t in LogTag.values)
                            DropdownMenuItem(value: t, child: Text(t.name)),
                        ],
                        onChanged: (v) => setState(() => _tag = v),
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.refresh_rounded),
                      onPressed: _load,
                    ),
                  ],
                ),
                TextField(
                  decoration: const InputDecoration(
                    hintText: 'keyword',
                    isDense: true,
                  ),
                  onChanged: (v) => setState(() => _keyword = v),
                ),
              ],
            ),
          ),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : ListView(
                    padding: const EdgeInsets.symmetric(horizontal: Dim.gutter),
                    children: [
                      if (rows.isEmpty)
                        Padding(
                          padding: const EdgeInsets.all(Dim.s6),
                          child: Text(
                            'nothing matches',
                            style: TextStyle(color: c.ink3),
                          ),
                        ),
                      ...rows,
                      if (_hasMore)
                        TextButton(
                          onPressed: () {
                            _loaded += _pageLines;
                            _load();
                          },
                          child: const Text('load more'),
                        ),
                      const SizedBox(height: Dim.s6),
                    ],
                  ),
          ),
          SafeArea(
            top: false,
            child: SwitchListTile(
              value: prefs.logDevForce,
              title: const Text('Force verbose (this device only)'),
              subtitle: const Text(
                'debug level + request bodies, overrides server config',
              ),
              onChanged: (v) async {
                await prefs.setLogDevForce(v);
                Log.updateConfig(
                  LogConfig.merge(
                    base: const LogConfig.defaults(),
                    server: prefs.logConfig,
                    devForce: v,
                  ),
                );
                if (mounted) setState(() {});
              },
            ),
          ),
        ],
      ),
    );
  }
}
