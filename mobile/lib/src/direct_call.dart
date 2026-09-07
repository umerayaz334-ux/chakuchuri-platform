import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:web_socket_channel/io.dart';

import 'api_client.dart';
import 'call_signal_inbox.dart';
import 'models.dart';
import 'theme.dart';

class DirectCallOverlay extends StatefulWidget {
  const DirectCallOverlay({
    super.key,
    required this.api,
    required this.session,
    required this.workspace,
    required this.onRefresh,
  });

  final ApiClient api;
  final Session session;
  final Workspace workspace;
  final Future<void> Function() onRefresh;

  @override
  State<DirectCallOverlay> createState() => _DirectCallOverlayState();
}

class _DirectCallOverlayState extends State<DirectCallOverlay> {
  CallRequest? _localCall;
  Timer? _ringTimer;
  Timer? _expireTimer;
  String _ringingCallId = '';
  bool _busy = false;
  String _error = '';

  CallRequest? get _activeCall {
    if (_localCall case final local?) {
      return _isActive(local) ? local : null;
    }
    return _firstActiveCall(widget.workspace.calls);
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _syncRingtone(_activeCall);
      _syncExpireTimer(_activeCall);
    });
  }

  @override
  void didUpdateWidget(covariant DirectCallOverlay oldWidget) {
    super.didUpdateWidget(oldWidget);
    final local = _localCall;
    if (local != null) {
      for (final remote in widget.workspace.calls) {
        if (remote.id == local.id && remote.status == local.status) {
          _localCall = null;
          break;
        }
      }
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _syncRingtone(_activeCall);
      _syncExpireTimer(_activeCall);
    });
  }

  @override
  void dispose() {
    _ringTimer?.cancel();
    _expireTimer?.cancel();
    super.dispose();
  }

  void _syncExpireTimer(CallRequest? call) {
    _expireTimer?.cancel();
    _expireTimer = null;
    if (call == null ||
        call.status != 'Ringing' ||
        call.ringExpiresAt.isEmpty) {
      return;
    }
    final expiresAt = DateTime.tryParse(call.ringExpiresAt)?.toUtc();
    if (expiresAt == null) return;
    final delay =
        expiresAt.difference(DateTime.now().toUtc()) +
        const Duration(milliseconds: 350);
    _expireTimer = Timer(delay.isNegative ? Duration.zero : delay, () {
      if (!mounted) return;
      setState(() {});
      unawaited(widget.onRefresh());
    });
  }

  void _syncRingtone(CallRequest? call) {
    final incoming =
        call != null &&
        call.status == 'Ringing' &&
        call.initiatorUserId != widget.session.user.id;
    if (!incoming) {
      _ringTimer?.cancel();
      _ringTimer = null;
      _ringingCallId = '';
      return;
    }
    if (_ringingCallId == call.id && _ringTimer != null) return;
    _ringTimer?.cancel();
    _ringingCallId = call.id;
    _playRing();
    _ringTimer = Timer.periodic(const Duration(seconds: 3), (_) => _playRing());
  }

  void _playRing() {
    SystemSound.play(SystemSoundType.alert);
    HapticFeedback.mediumImpact();
  }

  Future<void> _changeStatus(CallRequest call, String status) async {
    if (_busy) return;
    setState(() {
      _busy = true;
      _error = '';
    });
    try {
      final updated = await widget.api.updateCallStatus(call.id, status);
      if (!mounted) return;
      setState(() => _localCall = updated);
      await widget.onRefresh();
    } on ApiException catch (error) {
      if (mounted) setState(() => _error = error.message);
    } catch (_) {
      if (mounted) {
        setState(() => _error = 'The call action could not be completed.');
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final call = _activeCall;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _syncRingtone(call);
    });
    if (call == null) return const SizedBox.shrink();

    if (call.status == 'In call') {
      return Material(
        color: AppColors.ink,
        child: MobileCallRoom(
          key: ValueKey(call.id),
          api: widget.api,
          session: widget.session,
          call: call,
          onEnded: widget.onRefresh,
        ),
      );
    }

    final incoming = call.initiatorUserId != widget.session.user.id;
    final person = incoming
        ? _fallback(call.initiatorName, 'ChakuChuri support')
        : _fallback(call.recipientName, 'ChakuChuri support');

    return Material(
      color: const Color(0xF214201B),
      child: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(28),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    incoming ? 'INCOMING CALL' : 'AUDIO CALL',
                    style: TextStyle(
                      color: Colors.white.withValues(alpha: 0.55),
                      fontSize: 11,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                  const SizedBox(height: 30),
                  Container(
                    width: 104,
                    height: 104,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: AppColors.primary,
                      shape: BoxShape.circle,
                      border: Border.all(
                        color: Colors.white.withValues(alpha: 0.18),
                        width: 8,
                      ),
                    ),
                    child: Text(
                      _initials(person),
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 28,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                  ),
                  const SizedBox(height: 22),
                  Text(
                    person,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    textAlign: TextAlign.center,
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 27,
                      height: 1.12,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    incoming ? 'Incoming audio call' : 'Calling...',
                    style: const TextStyle(
                      color: Color(0xFFBFCBC5),
                      fontSize: 15,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    call.subject,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    textAlign: TextAlign.center,
                    style: const TextStyle(
                      color: Color(0xFF8FA099),
                      fontSize: 12,
                    ),
                  ),
                  if (_error.isNotEmpty) ...[
                    const SizedBox(height: 18),
                    Text(
                      _error,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        color: Color(0xFFFFB4AB),
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ],
                  const SizedBox(height: 52),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      _RoundCallControl(
                        label: incoming ? 'Decline' : 'Cancel',
                        icon: Icons.call_end_rounded,
                        color: AppColors.red,
                        disabled: _busy,
                        onPressed: () => _changeStatus(
                          call,
                          incoming ? 'Declined' : 'Cancelled',
                        ),
                      ),
                      if (incoming) ...[
                        const SizedBox(width: 52),
                        _RoundCallControl(
                          label: 'Answer',
                          icon: Icons.call_rounded,
                          color: AppColors.primary,
                          disabled: _busy,
                          onPressed: () => _changeStatus(call, 'In call'),
                        ),
                      ],
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class MobileCallRoom extends StatefulWidget {
  const MobileCallRoom({
    super.key,
    required this.api,
    required this.session,
    required this.call,
    required this.onEnded,
  });

  final ApiClient api;
  final Session session;
  final CallRequest call;
  final Future<void> Function() onEnded;

  @override
  State<MobileCallRoom> createState() => _MobileCallRoomState();
}

class _MobileCallRoomState extends State<MobileCallRoom> {
  RTCPeerConnection? _peer;
  MediaStream? _localStream;
  MediaStream? _remoteStream;
  WebSocketChannel? _socket;
  StreamSubscription<dynamic>? _socketSubscription;
  Timer? _pollTimer;
  Timer? _catchUpTimer;
  Timer? _heartbeatTimer;
  Timer? _elapsedTimer;
  Timer? _noticeTimer;
  final CallSignalInbox _signals = CallSignalInbox();
  Timer? _socketRetry;
  Timer? _iceRetry;
  JsonMap _callConfig = {};
  int _socketAttempts = 0;
  bool _socketConnecting = false;
  bool _polling = false;
  bool _disposed = false;
  DateTime _lastSocketEvent = DateTime.now();
  bool get _alive =>
      mounted &&
      !_ending &&
      !_disposed &&
      widget.api.token == widget.session.token;
  final List<RTCIceCandidate> _pendingCandidates = <RTCIceCandidate>[];
  int _lastSignalNo = 0;
  int _elapsed = 0;
  int _pollMs = 1200;
  bool _socketReady = false;
  bool _starting = false;
  bool _ending = false;
  bool _muted = false;
  bool _speaker = false;
  bool _iceRestarting = false;
  int _iceRestartAttempts = 0;
  String _phase = 'Preparing call';
  String _notice = '';

  bool get _isAdmin =>
      widget.session.user.role.trim().toLowerCase() != 'customer';

  @override
  void initState() {
    super.initState();
    _startElapsedClock();
    WidgetsBinding.instance.addPostFrameCallback((_) => _start());
  }

  @override
  void dispose() {
    _disposeLocal();
    super.dispose();
  }

  void _setState(VoidCallback action) {
    if (mounted) setState(action);
  }

  void _startElapsedClock() {
    void tick() {
      final started = DateTime.tryParse(widget.call.startedAt);
      if (started == null) return;
      final next = DateTime.now().toUtc().difference(started.toUtc()).inSeconds;
      _setState(() => _elapsed = next < 0 ? 0 : next);
    }

    tick();
    _elapsedTimer = Timer.periodic(const Duration(seconds: 1), (_) => tick());
  }

  Future<void> _start() async {
    if (!_alive || _starting || _peer != null) return;
    _starting = true;
    try {
      _setState(() {
        _phase = 'Opening microphone';
        _notice = '';
      });
      final config = await widget.api.callConfig().catchError(
        (_) => <String, dynamic>{},
      );
      if (!_alive) return;
      _callConfig = config;
      _pollMs = readInt(config, 'pollMs', 1200);
      final rawIceServers = config['iceServers'];
      final iceServers = rawIceServers is List
          ? rawIceServers.whereType<Map>().map((item) {
              final mapped = Map<String, dynamic>.from(item);
              final urls = mapped['urls'];
              if (urls is String) {
                mapped['urls'] = <String>[urls];
              } else if (urls is List) {
                mapped['urls'] = urls.map((value) => '$value').toList();
              }
              return mapped;
            }).toList()
          : <Map<String, dynamic>>[
              <String, dynamic>{
                'urls': <String>[
                  'stun:stun.l.google.com:19302',
                  'stun:stun1.l.google.com:19302',
                ],
              },
            ];

      final peer = await createPeerConnection(<String, dynamic>{
        'iceServers': iceServers,
        'sdpSemantics': 'unified-plan',
      });
      if (!_alive) {
        await peer.close();
        await peer.dispose();
        return;
      }
      _peer = peer;
      peer.onIceCandidate = (candidate) {
        if (_alive && candidate.candidate?.isNotEmpty == true) {
          unawaited(
            _sendSignal(
              'ice',
              Map<String, dynamic>.from(candidate.toMap() as Map),
            ).catchError((Object _) {
              _usePollingFallback();
            }),
          );
        }
      };
      peer.onTrack = (event) {
        if (!_alive) return;
        if (event.streams.isNotEmpty) {
          _remoteStream = event.streams.first;
          for (final track in _remoteStream!.getAudioTracks()) {
            track.enabled = true;
          }
        }
      };
      peer.onConnectionState = (state) {
        if (!_alive) return;
        if (state == RTCPeerConnectionState.RTCPeerConnectionStateConnected) {
          _iceRestarting = false;
          _iceRestartAttempts = 0;
          _iceRetry?.cancel();
          _setState(() => _phase = 'Connected');
          unawaited(
            Helper.setSpeakerphoneOn(_speaker).catchError((Object _) {}),
          );
        } else if (state ==
            RTCPeerConnectionState.RTCPeerConnectionStateFailed) {
          _setState(() => _phase = 'Reconnecting');
          unawaited(_restartIce());
        } else if (state ==
            RTCPeerConnectionState.RTCPeerConnectionStateDisconnected) {
          _setState(() => _phase = 'Reconnecting');
          unawaited(_restartIce());
        } else if (state ==
            RTCPeerConnectionState.RTCPeerConnectionStateConnecting) {
          _setState(() => _phase = 'Connecting');
        }
      };
      peer.onIceConnectionState = (state) {
        if (!_alive) return;
        if (state == RTCIceConnectionState.RTCIceConnectionStateFailed ||
            state == RTCIceConnectionState.RTCIceConnectionStateDisconnected) {
          _setState(() => _phase = 'Reconnecting');
          unawaited(_restartIce());
        } else if (state ==
            RTCIceConnectionState.RTCIceConnectionStateConnected) {
          _iceRestarting = false;
          _iceRestartAttempts = 0;
          _iceRetry?.cancel();
          if (_phase == 'Reconnecting') {
            _setState(() => _phase = 'Connected');
          }
        }
      };

      await Helper.ensureAudioSession().catchError((_) {});
      if (!_alive) return;
      final micAllowed = await _ensureMicrophonePermission();
      if (!_alive) return;
      if (!micAllowed) {
        throw const ApiException(
          'Microphone permission is required for audio calls. Allow microphone access in phone settings, then try again.',
          code: 'microphone_denied',
        );
      }
      final localStream = await navigator.mediaDevices.getUserMedia(
        <String, dynamic>{
          'audio': <String, dynamic>{
            'echoCancellation': true,
            'noiseSuppression': true,
            'autoGainControl': true,
          },
          'video': false,
        },
      );
      if (!_alive) {
        for (final track in localStream.getTracks()) {
          await track.stop();
        }
        await localStream.dispose();
        return;
      }
      _localStream = localStream;
      for (final track in _localStream!.getAudioTracks()) {
        await peer.addTrack(track, _localStream!);
      }

      final connected = await _connectSocket(config);
      if (!_alive) return;
      if (!connected) _startPolling();
      _heartbeatTimer = Timer.periodic(const Duration(seconds: 5), (_) {
        if (!_sendSocket(<String, dynamic>{'type': 'heartbeat'})) {
          unawaited(
            widget.api
                .heartbeatCall(widget.call.id)
                .then<void>((_) {})
                .catchError((Object _) {}),
          );
        }
      });

      if (_isAdmin) {
        final offer = await peer.createOffer(<String, dynamic>{
          'offerToReceiveAudio': true,
          'offerToReceiveVideo': false,
        });
        await peer.setLocalDescription(offer);
        await _sendSignal(
          'offer',
          Map<String, dynamic>.from(offer.toMap() as Map),
        );
      }
      await _pollSignals();
      _setState(() {
        _phase = _isAdmin ? 'Calling customer' : 'Joining call';
      });
    } catch (error) {
      _setState(() {
        _phase = 'Could not connect';
        _notice = _friendlyCallError(error);
      });
      _disposeLocal();
    } finally {
      _starting = false;
    }
  }

  Future<bool> _connectSocket(JsonMap config) async {
    if (!_alive || _socketConnecting) return false;
    final mode = readString(config, 'signalingMode');
    final path = readString(config, 'websocketPath', '/api/realtime/calls');
    if (mode != 'websocket' || path.isEmpty) return false;
    _socketConnecting = true;
    try {
      await _closeSocket();
      if (!_alive) return false;
      final socketUri = widget.api.webSocketUri(
        '${path.replaceAll(RegExp(r'/$'), '')}/${Uri.encodeComponent(widget.call.id)}',
      );
      final channel = IOWebSocketChannel.connect(
        socketUri,
        headers: {
          HttpHeaders.authorizationHeader: 'Bearer ${widget.session.token}',
        },
        connectTimeout: const Duration(seconds: 8),
      );
      _socket = channel;
      _socketSubscription = channel.stream.listen(
        (data) {
          if (_alive && identical(_socket, channel))
            unawaited(_handleSocketEvent(data));
        },
        onError: (_) {
          if (identical(_socket, channel)) _usePollingFallback();
        },
        onDone: () {
          if (identical(_socket, channel)) _usePollingFallback();
        },
      );
      await channel.ready.timeout(const Duration(milliseconds: 2500));
      if (!_alive ||
          !identical(_socket, channel) ||
          channel.closeCode != null) {
        await _closeSocket();
        return false;
      }
      _socketReady = true;
      _lastSocketEvent = DateTime.now();
      _socketAttempts = 0;
      _socketRetry?.cancel();
      _socketRetry = null;
      _pollTimer?.cancel();
      _pollTimer = null;
      _sendSocket(<String, dynamic>{
        'type': 'catch-up',
        'after': _lastSignalNo,
      });
      _startSignalCatchUp();
      return true;
    } catch (_) {
      await _closeSocket();
      _scheduleSocketRetry();
      return false;
    } finally {
      _socketConnecting = false;
    }
  }

  void _scheduleSocketRetry() {
    if (!_alive ||
        _socketRetry != null ||
        readString(_callConfig, 'signalingMode') != 'websocket')
      return;
    final delay = (1 << _socketAttempts.clamp(0, 4)).clamp(1, 15);
    _socketAttempts++;
    _socketRetry = Timer(Duration(seconds: delay), () {
      _socketRetry = null;
      if (_socketConnecting) {
        _scheduleSocketRetry();
        return;
      }
      if (_alive) unawaited(_connectSocket(_callConfig));
    });
  }

  void _usePollingFallback() {
    if (!_alive || _peer == null) return;
    _socketReady = false;
    _catchUpTimer?.cancel();
    _catchUpTimer = null;
    _startPolling();
    _scheduleSocketRetry();
  }

  void _startPolling() {
    if (!_alive || _pollTimer != null) return;
    _pollTimer = Timer.periodic(
      Duration(milliseconds: _pollMs.clamp(700, 5000)),
      (_) => unawaited(_pollSignals()),
    );
  }

  void _startSignalCatchUp() {
    if (_catchUpTimer != null) return;
    _catchUpTimer = Timer.periodic(const Duration(seconds: 2), (_) {
      if (_ending || _peer == null) return;
      if (DateTime.now().difference(_lastSocketEvent) >
          const Duration(seconds: 20)) {
        _usePollingFallback();
        return;
      }
      if (_sendSocket(<String, dynamic>{
        'type': 'catch-up',
        'after': _lastSignalNo,
      })) {
        return;
      }
      unawaited(_pollSignals());
    });
  }

  Future<void> _pollSignals() async {
    if (_peer == null || !_alive || _polling) return;
    _polling = true;
    try {
      final signals = await widget.api
          .listCallSignals(widget.call.id, after: _lastSignalNo)
          .timeout(const Duration(seconds: 12));
      for (final signal in signals) {
        if (!_alive) return;
        await _handleIncomingSignal(signal);
        if (signal.signalNo > _lastSignalNo) _lastSignalNo = signal.signalNo;
      }
    } catch (_) {
      if (!_socketReady) {}
    } finally {
      _polling = false;
    }
  }

  Future<void> _handleSocketEvent(dynamic raw) async {
    if (!_alive) return;
    _lastSocketEvent = DateTime.now();
    try {
      final decoded = jsonDecode(
        raw is String ? raw : utf8.decode(List<int>.from(raw as List)),
      );
      final event = asJsonMap(decoded);
      final type = readString(event, 'type');
      if (type == 'ready') {
      } else if (type == 'signal' || type == 'signal-sent') {
        final value = event['signal'];
        if (value is Map) {
          final signal = CallSignal.fromJson(Map<String, dynamic>.from(value));
          if (type == 'signal') {
            await _handleIncomingSignal(signal);
          }
        }
      } else if (type == 'signals') {
        final values = event['signals'];
        if (values is List) {
          for (final value in values.whereType<Map>()) {
            final signal = CallSignal.fromJson(
              Map<String, dynamic>.from(value),
            );
            await _handleIncomingSignal(signal);
            if (signal.signalNo > _lastSignalNo)
              _lastSignalNo = signal.signalNo;
          }
        }
      } else if (type == 'call-ended') {
        if (_ending) return;
        _ending = true;
        _setState(() => _phase = 'Call ended');
        await _finishLocally();
      } else if (type == 'error') {
        _showNotice(
          readString(event, 'message', 'Realtime call connection changed.'),
        );
      }
    } catch (_) {
      _usePollingFallback();
    }
  }

  Future<void> _handleIncomingSignal(CallSignal signal) async {
    await _signals.process(signal.id, () => _applyIncomingSignal(signal));
  }

  Future<void> _applyIncomingSignal(CallSignal signal) async {
    final peer = _peer;
    if (!_alive || peer == null || signal.senderId == widget.session.user.id)
      return;

    if (signal.signalType == 'offer') {
      final description = _descriptionFrom(signal.payload);
      if (description == null) return;
      await peer.setRemoteDescription(description);
      await _flushCandidates();
      final answer = await peer.createAnswer(<String, dynamic>{
        'offerToReceiveAudio': true,
        'offerToReceiveVideo': false,
      });
      await peer.setLocalDescription(answer);
      await _sendSignal(
        'answer',
        Map<String, dynamic>.from(answer.toMap() as Map),
      );
      _setState(() => _phase = 'Connecting');
      return;
    }
    if (signal.signalType == 'answer') {
      final state = await peer.getSignalingState();
      final description = _descriptionFrom(signal.payload);
      if (state == RTCSignalingState.RTCSignalingStateHaveLocalOffer &&
          description != null) {
        await peer.setRemoteDescription(description);
        await _flushCandidates();
      }
      return;
    }
    if (signal.signalType == 'ice' || signal.signalType == 'candidate') {
      final candidate = _candidateFrom(signal.payload);
      if (candidate == null) return;
      final remote = await peer.getRemoteDescription();
      if (remote == null) {
        _pendingCandidates.add(candidate);
      } else {
        await peer.addCandidate(candidate);
      }
      return;
    }
    if (signal.signalType == 'media-state') {
      final muted = signal.payload['muted'] == true;
      _showNotice(
        muted
            ? 'The other microphone is muted.'
            : 'The other microphone is active.',
      );
    }
  }

  Future<void> _flushCandidates() async {
    final peer = _peer;
    if (peer == null) return;
    for (final candidate in List<RTCIceCandidate>.from(_pendingCandidates)) {
      try {
        await peer.addCandidate(candidate);
      } catch (_) {
        // A later catch-up can provide a replacement candidate.
      }
    }
    _pendingCandidates.clear();
  }

  Future<void> _restartIce() async {
    final peer = _peer;
    if (peer == null || !_alive || _iceRestarting) return;
    if (_iceRestartAttempts >= 3) {
      _setState(() => _phase = 'Connection needs retry');
      return;
    }
    _iceRestarting = true;
    _setState(() => _phase = 'Reconnecting');
    _iceRestartAttempts += 1;
    _iceRetry?.cancel();
    _iceRetry = Timer(const Duration(seconds: 8), () {
      if (!_alive) return;
      _iceRestarting = false;
      unawaited(_restartIce());
    });
    try {
      await peer.restartIce();
      // Only the caller (admin) renegotiates; the customer answers the new offer.
      if (_isAdmin) {
        if (await peer.getSignalingState() ==
            RTCSignalingState.RTCSignalingStateHaveLocalOffer) {
          await peer.setLocalDescription(RTCSessionDescription('', 'rollback'));
        }
        final offer = await peer.createOffer(<String, dynamic>{
          'iceRestart': true,
          'offerToReceiveAudio': true,
          'offerToReceiveVideo': false,
        });
        await peer.setLocalDescription(offer);
        await _sendSignal(
          'offer',
          Map<String, dynamic>.from(offer.toMap() as Map),
        );
      }
    } catch (_) {
      _iceRestarting = false;
      if (_iceRestartAttempts >= 3) {
        _setState(() => _phase = 'Connection needs retry');
      } else {
        // The recovery timer retries even if signaling succeeds but media does not.
      }
    }
  }

  RTCSessionDescription? _descriptionFrom(JsonMap payload) {
    final sdp = readString(payload, 'sdp');
    final type = readString(payload, 'type');
    if (sdp.isEmpty || type.isEmpty) return null;
    return RTCSessionDescription(sdp, type);
  }

  RTCIceCandidate? _candidateFrom(JsonMap payload) {
    final candidate = readString(payload, 'candidate');
    if (candidate.isEmpty) return null;
    final rawIndex = payload['sdpMLineIndex'];
    final lineIndex = rawIndex is int
        ? rawIndex
        : int.tryParse(rawIndex?.toString() ?? '');
    return RTCIceCandidate(candidate, payload['sdpMid']?.toString(), lineIndex);
  }

  Future<void> _sendSignal(String type, JsonMap payload) async {
    if (!_alive) return;
    if (_sendSocket(<String, dynamic>{
      'type': 'signal',
      'signalType': type,
      'payload': payload,
    })) {
      return;
    }
    await widget.api.sendCallSignal(widget.call.id, type, payload);
  }

  bool _sendSocket(JsonMap payload) {
    if (!_socketReady || _socket == null) return false;
    try {
      _socket!.sink.add(jsonEncode(payload));
      return true;
    } catch (_) {
      _usePollingFallback();
      return false;
    }
  }

  Future<void> _toggleMute() async {
    final next = !_muted;
    for (final track
        in _localStream?.getAudioTracks() ?? <MediaStreamTrack>[]) {
      track.enabled = !next;
    }
    _setState(() => _muted = next);
    try {
      await _sendSignal('media-state', <String, dynamic>{'muted': next});
    } catch (_) {
      _showNotice('Microphone changed locally.');
    }
  }

  Future<void> _toggleSpeaker() async {
    final next = !_speaker;
    try {
      await Helper.setSpeakerphoneOn(next);
      _setState(() => _speaker = next);
    } catch (_) {
      _showNotice('Audio will use the available output device.');
    }
  }

  Future<void> _end() async {
    if (_ending) return;
    _ending = true;
    _setState(() => _phase = 'Ending call');
    try {
      if (_sendSocket(<String, dynamic>{'type': 'end'})) {
        await Future<void>.delayed(const Duration(milliseconds: 180));
      } else {
        await widget.api.endCall(widget.call.id);
      }
    } catch (_) {
      _showNotice('The call was closed on this device.');
    }
    await _finishLocally();
  }

  Future<void> _finishLocally() async {
    _disposeLocal();
    if (mounted) await widget.onEnded();
  }

  void _showNotice(String value) {
    _noticeTimer?.cancel();
    _setState(() => _notice = value);
    _noticeTimer = Timer(const Duration(seconds: 3), () {
      _setState(() => _notice = '');
    });
  }

  Future<void> _closeSocket() async {
    _socketReady = false;
    final subscription = _socketSubscription;
    _socketSubscription = null;
    final socket = _socket;
    _socket = null;
    await subscription?.cancel();
    if (socket != null) await socket.sink.close();
  }

  void _disposeMediaOnly() {
    final stream = _localStream;
    _localStream = null;
    if (stream != null) {
      for (final track in stream.getTracks()) {
        track.stop();
      }
      unawaited(stream.dispose());
    }
    final remote = _remoteStream;
    _remoteStream = null;
    if (remote != null) unawaited(remote.dispose());
    final peer = _peer;
    _peer = null;
    if (peer != null) {
      unawaited(peer.close().then((_) => peer.dispose()));
    }
  }

  void _disposeLocal() {
    _disposed = true;
    _signals.close();
    _socketRetry?.cancel();
    _iceRetry?.cancel();
    _pollTimer?.cancel();
    _catchUpTimer?.cancel();
    _heartbeatTimer?.cancel();
    _elapsedTimer?.cancel();
    _noticeTimer?.cancel();
    _pollTimer = null;
    _catchUpTimer = null;
    _heartbeatTimer = null;
    _elapsedTimer = null;
    _noticeTimer = null;
    final subscription = _socketSubscription;
    _socketSubscription = null;
    if (subscription != null) unawaited(subscription.cancel());
    final socket = _socket;
    _socket = null;
    _socketReady = false;
    if (socket != null) unawaited(socket.sink.close());
    _disposeMediaOnly();
  }

  @override
  Widget build(BuildContext context) {
    final person = _isAdmin
        ? _fallback(widget.call.recipientName, 'Customer')
        : _fallback(widget.call.answeredBy, 'ChakuChuri support');
    return SafeArea(
      child: Stack(
        children: [
          Positioned.fill(
            child: ColoredBox(
              color: AppColors.ink,
              child: CustomPaint(painter: const _CallBackdropPainter()),
            ),
          ),
          Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(24, 40, 24, 30),
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 440),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      'AUDIO CALL',
                      style: TextStyle(
                        color: Colors.white.withValues(alpha: 0.55),
                        fontSize: 11,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    const SizedBox(height: 30),
                    CircleAvatar(
                      radius: 52,
                      backgroundColor: AppColors.primary,
                      child: Text(
                        _initials(person),
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 28,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                    ),
                    const SizedBox(height: 22),
                    Text(
                      person,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 27,
                        height: 1.12,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      _phase,
                      style: TextStyle(
                        color: _phase == 'Connected'
                            ? const Color(0xFF63D6A7)
                            : const Color(0xFFBFCBC5),
                        fontSize: 15,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 5),
                    Text(
                      _formatDuration(_elapsed),
                      style: const TextStyle(
                        color: Color(0xFF8FA099),
                        fontSize: 14,
                        fontFeatures: <FontFeature>[
                          FontFeature.tabularFigures(),
                        ],
                      ),
                    ),
                    if (_notice.isNotEmpty) ...[
                      const SizedBox(height: 18),
                      Text(
                        _notice,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          color: Color(0xFFFFD6A1),
                          fontSize: 12,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ],
                    if (_phase == 'Connection needs retry')
                      TextButton.icon(
                        onPressed: () {
                          _iceRestartAttempts = 0;
                          _iceRestarting = false;
                          unawaited(_restartIce());
                        },
                        icon: const Icon(Icons.refresh),
                        label: const Text('Retry connection'),
                      ),
                    const SizedBox(height: 48),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        _RoundCallControl(
                          label: _muted ? 'Unmute' : 'Mute',
                          icon: _muted
                              ? Icons.mic_off_rounded
                              : Icons.mic_rounded,
                          color: const Color(0xFF35443D),
                          onPressed: _toggleMute,
                        ),
                        const SizedBox(width: 28),
                        _RoundCallControl(
                          label: 'End',
                          icon: Icons.call_end_rounded,
                          color: AppColors.red,
                          disabled: _ending,
                          onPressed: _end,
                        ),
                        const SizedBox(width: 28),
                        _RoundCallControl(
                          label: 'Speaker',
                          icon: _speaker
                              ? Icons.volume_up_rounded
                              : Icons.volume_down_rounded,
                          color: _speaker
                              ? AppColors.primary
                              : const Color(0xFF35443D),
                          onPressed: _toggleSpeaker,
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _RoundCallControl extends StatelessWidget {
  const _RoundCallControl({
    required this.label,
    required this.icon,
    required this.color,
    required this.onPressed,
    this.disabled = false,
  });

  final String label;
  final IconData icon;
  final Color color;
  final VoidCallback onPressed;
  final bool disabled;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 76,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox.square(
            dimension: 62,
            child: IconButton.filled(
              tooltip: label,
              onPressed: disabled ? null : onPressed,
              style: IconButton.styleFrom(
                backgroundColor: color,
                disabledBackgroundColor: color.withValues(alpha: 0.35),
                foregroundColor: Colors.white,
                shape: const CircleBorder(),
              ),
              icon: Icon(icon, size: 27),
            ),
          ),
          const SizedBox(height: 9),
          Text(
            label,
            maxLines: 1,
            style: const TextStyle(
              color: Color(0xFFE4ECE8),
              fontSize: 12,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
      ),
    );
  }
}

class _CallBackdropPainter extends CustomPainter {
  const _CallBackdropPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = Colors.white.withValues(alpha: 0.025)
      ..strokeWidth = 1;
    const gap = 42.0;
    for (double x = 0; x < size.width; x += gap) {
      canvas.drawLine(Offset(x, 0), Offset(x, size.height), paint);
    }
    for (double y = 0; y < size.height; y += gap) {
      canvas.drawLine(Offset(0, y), Offset(size.width, y), paint);
    }
  }

  @override
  bool shouldRepaint(covariant _CallBackdropPainter oldDelegate) => false;
}

CallRequest? _firstActiveCall(List<CallRequest> calls) {
  for (final call in calls) {
    if (_isActive(call)) return call;
  }
  return null;
}

bool _isActive(CallRequest call) {
  if (call.status == 'In call') return true;
  if (call.status != 'Ringing') return false;
  if (call.ringExpiresAt.isEmpty) return true;
  final expiresAt = DateTime.tryParse(call.ringExpiresAt)?.toUtc();
  if (expiresAt == null) return true;
  return expiresAt.isAfter(DateTime.now().toUtc());
}

String _fallback(String value, String fallback) {
  return value.trim().isEmpty ? fallback : value.trim();
}

String _initials(String value) {
  final parts = value
      .trim()
      .split(RegExp(r'\s+'))
      .where((part) => part.isNotEmpty)
      .take(2)
      .toList();
  if (parts.isEmpty) return 'CC';
  return parts.map((part) => part[0].toUpperCase()).join();
}

String _formatDuration(int seconds) {
  final minutes = seconds ~/ 60;
  final remainder = seconds % 60;
  return '${minutes.toString().padLeft(2, '0')}:${remainder.toString().padLeft(2, '0')}';
}

String _friendlyCallError(Object error) {
  if (error is ApiException) return error.message;
  final text = error.toString().toLowerCase();
  if (text.contains('permission') || text.contains('notallowed')) {
    return 'Microphone access is required for audio calls.';
  }
  return 'The audio call could not connect. Check the network and try again.';
}

Future<bool> _ensureMicrophonePermission() async {
  var status = await Permission.microphone.status;
  if (status.isGranted) return true;
  status = await Permission.microphone.request();
  if (status.isGranted) return true;
  if (status.isPermanentlyDenied) {
    await openAppSettings();
  }
  return false;
}
