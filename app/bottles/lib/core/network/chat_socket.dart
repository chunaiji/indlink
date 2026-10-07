import 'dart:async';
import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';

import '../../domain/models/chat.dart';
import '../config/app_config.dart';

/// 聊天长连接。直连现有 `wss://…/ws?token=<JWT>`，后端零改动。
///
/// App 侧比小程序多三件事：前后台切换时断开/重连、指数退避、
/// 重连后由调用方拉增量消息补齐断线期间的空档。
class ChatSocket {
  ChatSocket({required this.myId});

  final String myId;

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _sub;
  Timer? _retryTimer;
  int _attempt = 0;
  bool _manuallyClosed = false;
  String? _token;

  final _messages = StreamController<ChatMessage>.broadcast();
  final _connected = StreamController<bool>.broadcast();

  Stream<ChatMessage> get messages => _messages.stream;

  /// 连接状态。断线期间聊天页顶部显示「正在重新连接…」。
  Stream<bool> get connected => _connected.stream;

  /// 重连成功的时刻，调用方据此拉增量补齐断线期间的消息。
  final _reconnected = StreamController<void>.broadcast();
  Stream<void> get reconnected => _reconnected.stream;

  /// 火花帧的原始载荷。
  ///
  /// 这里**不**解成 SparkEvent:语种要到弹窗那一刻才算。
  /// ChatSocket 是登录时建的，把当时的 locale 存进来，
  /// 用户在设置里改了语言之后弹出来的还是旧语种。
  final _sparks = StreamController<Map<String, dynamic>>.broadcast();
  Stream<Map<String, dynamic>> get sparkFrames => _sparks.stream;


  void connect(String token) {
    _token = token;
    _manuallyClosed = false;
    _open();
  }

  void _open() {
    if (_token == null) return;
    _retryTimer?.cancel();
    try {
      final uri = Uri.parse('${AppConfig.wsBase}?token=$_token');
      final ch = WebSocketChannel.connect(uri);
      _channel = ch;
      _sub = ch.stream.listen(
        _onFrame,
        onDone: _scheduleRetry,
        onError: (_) => _scheduleRetry(),
        cancelOnError: true,
      );
      if (_attempt > 0) _reconnected.add(null);
      _attempt = 0;
      _connected.add(true);
    } catch (_) {
      _scheduleRetry();
    }
  }

  void _onFrame(dynamic frame) {
    try {
      final json = jsonDecode('$frame');
      if (json is! Map) return;
      final map = Map<String, dynamic>.from(json);
      // 后端消息帧: {event:"message", chat_id:"123", message:{...}}(见 chat/service.go)。
      // ⚠️ 会话号在帧**顶层** chat_id,不在消息对象里;消息对象在 message 键下。
      // 早期字段名 type/data 一并兼容。此前判成 type/data 导致真机消息全被丢弃。
      if (map['event'] == 'spark' || map['type'] == 'spark') {
        _sparks.add(map);
        return;
      }
      final payload = map['message'] ?? map['data'];
      final isMessage = map['event'] == 'message' || map['type'] == 'message';
      if (isMessage && payload is Map) {
        _messages.add(
          ChatMessage.fromJson(
            Map<String, dynamic>.from(payload),
            myId: myId,
            conversationId: map['chat_id']?.toString(),
          ),
        );
      }
    } catch (_) {
      // 坏帧直接丢弃，不能让一条脏数据打断长连接。
    }
  }

  /// 指数退避：1s → 2s → 4s → 8s → 16s，封顶 30s。
  void _scheduleRetry() {
    _connected.add(false);
    _cleanup();
    if (_manuallyClosed) return;
    final delay = Duration(
      seconds: (1 << _attempt.clamp(0, 5)).clamp(1, 30),
    );
    _attempt++;
    _retryTimer = Timer(delay, _open);
  }

  /// 进后台时主动断开，省电也避免服务端挂着僵尸连接。
  void pause() {
    _manuallyClosed = true;
    _cleanup();
    _connected.add(false);
  }

  void resume() {
    if (_token == null) return;
    _manuallyClosed = false;
    _attempt = 0;
    _open();
  }

  void _cleanup() {
    _sub?.cancel();
    _sub = null;
    _channel?.sink.close();
    _channel = null;
  }

  void send(Map<String, dynamic> payload) {
    _channel?.sink.add(jsonEncode(payload));
  }

  void dispose() {
    _manuallyClosed = true;
    _retryTimer?.cancel();
    _cleanup();
    _messages.close();
    _connected.close();
    _reconnected.close();
  }
}
