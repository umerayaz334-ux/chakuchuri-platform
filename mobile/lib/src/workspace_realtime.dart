import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:web_socket_channel/io.dart';

import 'api_client.dart';
import 'models.dart';

class WorkspaceRealtimeClient {
  WorkspaceRealtimeClient({
    required this.api,
    required this.onChanges,
    required this.onResync,
  });

  final ApiClient api;
  final void Function(List<JsonMap>) onChanges;
  final void Function() onResync;
  IOWebSocketChannel? _socket;
  StreamSubscription<dynamic>? _subscription;
  Timer? _retry;
  Timer? _heartbeat;
  bool _closed = false;
  bool _connected = false;
  int _attempt = 0;
  int? _revision;
  DateTime _lastEvent = DateTime.now();
  final _random = Random();

  bool get connected => _connected;

  void connect() {
    if (_closed || _socket != null || api.token.isEmpty) return;
    _retry?.cancel();
    _retry = null;
    try {
      final socket = IOWebSocketChannel.connect(
        api.webSocketUri('/api/realtime/workspace'),
        headers: {HttpHeaders.authorizationHeader: ['Bearer', api.token].join(' ')},
        connectTimeout: const Duration(seconds: 8),
      );
      _socket = socket;
      _subscription = socket.stream.listen(
        (raw) {
          if (_closed || _socket != socket) return;
          try {
            final event = asJsonMap(jsonDecode(raw is String ? raw : utf8.decode(List<int>.from(raw))));
            _lastEvent = DateTime.now();
            final revision = event['revision'];
            if (revision is! int) return;
            final type = readString(event, 'type');
            if (type == 'ready') {
              _connected = true;
              _attempt = 0;
              _revision = revision;
              // Subscribe before fetching the snapshot so no startup event is lost.
              onResync();
            } else if (type == 'workspace.changed') {
              if (_revision != null && revision != _revision! + 1) onResync();
              _revision = revision;
              onChanges(readList(event, 'changes', (row) => row));
            } else if (type == 'pong' && revision != _revision) {
              _revision = revision;
              onResync();
            }
          } catch (_) {
            onResync();
          }
        },
        onError: (Object _) => _lost(socket),
        onDone: () => _lost(socket),
      );
      unawaited(_watchReady(socket));
    } catch (_) {
      _scheduleReconnect();
    }
  }

  Future<void> _watchReady(IOWebSocketChannel socket) async {
    try {
      await socket.ready;
      if (_closed || _socket != socket) return;
      _lastEvent = DateTime.now();
      _heartbeat = Timer.periodic(const Duration(seconds: 15), (_) {
        if (DateTime.now().difference(_lastEvent) > const Duration(seconds: 45)) {
          _lost(socket);
          return;
        }
        try {
          socket.sink.add(jsonEncode({'type': 'ping'}));
        } catch (_) {
          _lost(socket);
        }
      });
    } catch (_) {
      _lost(socket);
    }
  }

  void _lost(IOWebSocketChannel socket) {
    if (_socket != socket) return;
    _connected = false;
    _socket = null;
    _heartbeat?.cancel();
    _heartbeat = null;
    unawaited(_subscription?.cancel());
    _subscription = null;
    unawaited(socket.sink.close().catchError((Object _) {}));
    _scheduleReconnect();
  }

  void _scheduleReconnect() {
    if (_closed || _retry != null) return;
    final delay = min(15000, 750 * pow(2, min(_attempt++, 5)).toInt());
    _retry = Timer(Duration(milliseconds: delay + _random.nextInt(500)), () {
      _retry = null;
      connect();
    });
  }

  void close() {
    _closed = true;
    _retry?.cancel();
    _heartbeat?.cancel();
    final socket = _socket;
    _socket = null;
    _connected = false;
    unawaited(_subscription?.cancel());
    _subscription = null;
    if (socket != null) unawaited(socket.sink.close().catchError((Object _) {}));
  }
}
