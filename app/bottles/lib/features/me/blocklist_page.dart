import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/states.dart';
import '../chat/chat_controller.dart';

/// 黑名单管理。
///
/// 拉黑「动作」是上架硬性要求（App Store 1.2），这一页是它的管理面：
/// 已拉黑的人可以在这里解除。
class BlocklistPage extends ConsumerWidget {
  const BlocklistPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.c;
    final l = L.of(context);
    final list = ref.watch(blocklistProvider);

    return Scaffold(
      appBar: NavBar(title: l.safetyBlocklist),
      body: AsyncView(
        value: list,
        onRetry: () => ref.invalidate(blocklistProvider),
        errorTitle: l.stateErrorTitle,
        errorBody: l.stateErrorBody,
        retryLabel: l.commonRetry,
        skeleton: const Padding(
          padding: EdgeInsets.all(Dim.gutter),
          child: SkeletonRows(count: 3),
        ),
        data: (users) {
          if (users.isEmpty) {
            return EmptyState(
              icon: Icons.block_rounded,
              title: l.safetyBlocklistEmpty,
              body: l.stateEmptyMomentsBody,
            );
          }
          return ListView.builder(
            padding: context.pagePadding(),
            itemCount: users.length,
            itemBuilder: (context, i) {
              final u = users[i];
              return Container(
                constraints: const BoxConstraints(minHeight: 65),
                padding: const EdgeInsets.symmetric(vertical: Dim.s2),
                child: Row(
                  children: [
                    AvatarRing.of(u, size: 44, online: false),
                    const SizedBox(width: Dim.s3),
                    Expanded(
                      child: Text(
                        u.nickname,
                        style: TextStyle(
                          fontSize: Dim.t3,
                          fontWeight: FontWeight.w700,
                          color: c.ink,
                        ),
                      ),
                    ),
                    MiniButton(
                      label: l.safetyUnblock,
                      kind: BtnKind.ghost,
                      onTap: () async {
                        await ref.read(chatRepoProvider).unblock(u.id);
                        ref.invalidate(blocklistProvider);
                      },
                    ),
                  ],
                ),
              );
            },
          );
        },
      ),
    );
  }
}
