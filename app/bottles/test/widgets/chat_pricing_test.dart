import 'package:bottles/core/config/remote_config.dart';
import 'package:bottles/core/network/api_exception.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/data/mock/mock_repositories.dart';
import 'package:bottles/domain/models/chat.dart';
import 'package:bottles/domain/models/user.dart';
import 'package:bottles/features/chat/chat_room_page.dart';
import 'package:bottles/features/discover/discover_page.dart';
import 'package:bottles/ui/widgets/buttons.dart';
import 'package:bottles/ui/widgets/overlays.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// 后台配的聊天扣费规则（开聊 M / 每条 N / 前 L 条免费）要原样显示在 App 上。
class _PricedConfig extends AppConfigNotifier {
  @override
  AppRemoteConfig build() => AppRemoteConfig.fromJson({
    'pricing': {'chat_start': 7, 'chat_msg': 2, 'chat_free_msgs': 3},
  });
}

/// 服务端按条扣费时余额不够：send 返回钱包业务码。
class _BrokeChat extends MockChatRepository {
  _BrokeChat(super.db);

  @override
  Future<ChatMessage> send(String conversationId, String text) async {
    throw ApiException(ErrCode.insufficientBalance, 'insufficient');
  }
}

/// 钱包只剩 1 币。
class _PoorWallet extends MockWalletRepository {
  _PoorWallet(super.db);

  @override
  Future<Wallet> wallet() async =>
      const Wallet(coins: 1, totalRecharged: 0, totalSpent: 0);
}

void main() {
  testWidgets('say-hi button shows the configured start price', (t) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const DiscoverPage(),
      skipDefaults: {appConfigProvider},
      overrides: [appConfigProvider.overrideWith(_PricedConfig.new)],
    );
    expect(
      find.descendant(
        of: find.byType(HiButton).first,
        matching: find.text('7'),
      ),
      findsOneWidget,
    );
  });

  testWidgets('chat room explains the per-message rule when it is on', (
    t,
  ) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const ChatRoomPage(conversationId: 'c1'),
      skipDefaults: {appConfigProvider},
      overrides: [appConfigProvider.overrideWith(_PricedConfig.new)],
    );
    expect(
      find.text('First 3 messages are free, then 2 coins each'),
      findsOneWidget,
    );
  });

  testWidgets('chat room stays quiet when messages are free', (t) async {
    await pumpAt(t, layoutMatrix[2], const ChatRoomPage(conversationId: 'c1'));
    expect(find.byKey(const ValueKey('chat-pricing-hint')), findsNothing);
  });

  testWidgets('a message rejected for balance opens the top-up modal', (
    t,
  ) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const ChatRoomPage(conversationId: 'c1'),
      skipDefaults: {appConfigProvider, chatRepoProvider, walletRepoProvider},
      overrides: [
        appConfigProvider.overrideWith(_PricedConfig.new),
        chatRepoProvider.overrideWith(
          (ref) => _BrokeChat(ref.watch(mockBackendProvider)),
        ),
        walletRepoProvider.overrideWith(
          (ref) => _PoorWallet(ref.watch(mockBackendProvider)),
        ),
      ],
    );
    await t.enterText(find.byType(TextField), 'hello');
    await t.tap(find.byKey(const ValueKey('chat-send')));
    await t.pumpAndSettle();

    // 每条 2 币、手里 1 币 → 「还差 1 金币」；失败的那条仍留在列表里等重发。
    expect(find.byType(ModalCard), findsOneWidget);
    expect(find.text('1 coins short'), findsOneWidget);
  });
}
