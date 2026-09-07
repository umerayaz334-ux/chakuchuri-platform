import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:permission_handler/permission_handler.dart';

import 'models.dart';
import 'notification_keys.dart';

class AppNotifications {
  AppNotifications._();

  static final FlutterLocalNotificationsPlugin _plugin = FlutterLocalNotificationsPlugin();
  static bool _ready = false;
  static Future<void>? _initializing;
  static bool _seeded = false;
  static final Set<String> _seen = <String>{};
  static int _notifyId = 1000;
  static void Function(String route)? onRoute;
  static int _sessionEpoch = 0;
  static Future<void> _work = Future<void>.value();

  static const _androidChannel = AndroidNotificationChannel(
    'chakuchuri_updates',
    'ChakuChuri updates',
    description: 'Quotes, messages, payments, shipping and call alerts.',
    importance: Importance.high,
    playSound: true,
    enableVibration: true,
  );

  static const _callsChannel = AndroidNotificationChannel(
    'chakuchuri_calls',
    'ChakuChuri calls',
    description: 'Incoming and live audio call alerts.',
    importance: Importance.max,
    playSound: true,
    enableVibration: true,
  );

  static Future<void> init() {
    if (_ready) return Future<void>.value();
    return _initializing ??= _initialize().catchError((Object _) {
      // Notification availability must not prevent sign-in or workspace loading.
      _ready = false;
    }).whenComplete(() { _initializing = null; });
  }

  static Future<void> _initialize() async {

    const androidInit = AndroidInitializationSettings('@mipmap/ic_launcher');
    const iosInit = DarwinInitializationSettings(
      requestAlertPermission: true,
      requestBadgePermission: true,
      requestSoundPermission: true,
    );

    await _plugin.initialize(
      InitializationSettings(android: androidInit, iOS: iosInit),
      onDidReceiveNotificationResponse: _onTap,
    );

    final launch = await _plugin.getNotificationAppLaunchDetails();
    final launchResponse = launch?.notificationResponse;
    if (launch?.didNotificationLaunchApp == true && launchResponse != null) {
      _onTap(launchResponse);
    }

    final android = _plugin.resolvePlatformSpecificImplementation<AndroidFlutterLocalNotificationsPlugin>();
    await android?.createNotificationChannel(_androidChannel);
    await android?.createNotificationChannel(_callsChannel);

    _ready = true;
  }

  static void _onTap(NotificationResponse response) {
    final route = response.payload?.trim() ?? '';
    if (route.isEmpty) return;
    onRoute?.call(route);
  }

  static Future<void> requestPermission() async {
    await init();
    if (!_ready) return;
    final android = _plugin.resolvePlatformSpecificImplementation<AndroidFlutterLocalNotificationsPlugin>();
    await android?.requestNotificationsPermission();

    final status = await Permission.notification.status;
    if (!status.isGranted) {
      await Permission.notification.request();
    }
    await _plugin
        .resolvePlatformSpecificImplementation<IOSFlutterLocalNotificationsPlugin>()
        ?.requestPermissions(alert: true, badge: true, sound: true);
  }

  static void reset() {
    _sessionEpoch++;
    _seeded = false;
    _seen.clear();
    _work = _work.then((_) async {
      if (_ready) await _plugin.cancelAll();
    }).catchError((Object _) {});
  }

  static Future<void> syncWorkspace(Workspace workspace, {String userId = ''}) {
    final epoch = _sessionEpoch;
    _work = _work.then((_) => _syncWorkspace(workspace, userId, epoch)).catchError((Object _) {});
    return _work;
  }

  static Future<void> _syncWorkspace(Workspace workspace, String userId, int epoch) async {
    if (epoch != _sessionEpoch) return;
    await init();
    if (!_ready || epoch != _sessionEpoch) return;
    final events = _eventsFrom(workspace, userId: userId);

    if (!_seeded) {
      _seen
        ..clear()
        ..addAll(events.map((event) => event.id));
      _seeded = true;
      return;
    }

    for (final event in events) {
      if (epoch != _sessionEpoch) return;
      if (_seen.contains(event.id)) continue;
      await _show(event);
      if (epoch != _sessionEpoch) return;
      _seen.add(event.id);
    }

    if (_seen.length > 300) {
      final keep = events.map((event) => event.id).toSet();
      _seen.removeWhere((id) => !keep.contains(id) && _seen.length > 200);
    }
  }

  static List<_NotifyEvent> _eventsFrom(Workspace workspace, {required String userId}) {
    final events = <_NotifyEvent>[];

    for (final quote in workspace.quotations.where((row) => row.status == 'Priced')) {
      events.add(
        _NotifyEvent(
          id: 'quote-priced-${quote.id}',
          title: 'Your quotation is ready',
          body: '${quote.productName} · ${quote.id}',
          route: 'Get Quote',
          call: false,
        ),
      );
    }

    for (final payment in workspace.payments.where((row) => row.status == 'Waiting confirmation')) {
      events.add(
        _NotifyEvent(
          id: 'payment-review-${payment.id}',
          title: 'Payment under review',
          body: '${payment.type} · ${payment.id}',
          route: 'Payments',
          call: false,
        ),
      );
    }

    for (final shipment in workspace.shipping.where((row) => row.status == 'In transit' || row.status == 'Delivered')) {
      events.add(
        _NotifyEvent(
          id: 'ship-${shipment.id}-${shipment.status}',
          title: 'Shipment ${shipment.status.toLowerCase()}',
          body: '${shipment.courier} · ${shipment.tracking.isEmpty ? shipment.id : shipment.tracking}',
          route: 'Shipping',
          call: false,
        ),
      );
    }

    for (final conversation in workspace.conversations) {
      final id = customerMessageNotificationId(conversation);
      if (id == null) continue;
      events.add(
        _NotifyEvent(
          id: id,
          title: 'New message',
          body: conversation.subject.isEmpty ? 'Open Messages' : conversation.subject,
          route: 'Messages',
          call: false,
        ),
      );
    }

    for (final order in workspace.manufacturing) {
      final update = order.history.isEmpty ? null : order.history.last;
      if (update == null) continue;
      events.add(
        _NotifyEvent(
          id: 'order-${order.id}-${update.createdAt}-${update.label}',
          title: update.label.isEmpty ? 'Order updated' : update.label,
          body: '${order.productName} · ${order.currentStage}',
          route: 'Orders',
          call: false,
        ),
      );
    }

    for (final call in workspace.calls.where((row) => row.status == 'Ringing' || row.status == 'In call')) {
      final id = callNotificationId(call, userId);
      if (id == null) continue;
      events.add(
        _NotifyEvent(
          id: id,
          title: call.status == 'Ringing' ? 'Incoming audio call' : 'Audio call in progress',
          body: call.subject.isEmpty ? 'ChakuChuri support' : call.subject,
          route: 'Calls',
          call: true,
        ),
      );
    }

    return events;
  }

  static Future<void> _show(_NotifyEvent event) async {
    final channel = event.call ? _callsChannel : _androidChannel;
    await _plugin.show(
      _notifyId++,
      event.title,
      event.body,
      NotificationDetails(
        android: AndroidNotificationDetails(
          channel.id,
          channel.name,
          channelDescription: channel.description,
          importance: event.call ? Importance.max : Importance.high,
          priority: event.call ? Priority.max : Priority.high,
          category: event.call ? AndroidNotificationCategory.call : AndroidNotificationCategory.message,
          playSound: true,
          enableVibration: true,
          visibility: NotificationVisibility.private,
        ),
        iOS: const DarwinNotificationDetails(
          presentAlert: true,
          presentBadge: true,
          presentSound: true,
        ),
      ),
      payload: event.route,
    );
  }
}

class _NotifyEvent {
  const _NotifyEvent({
    required this.id,
    required this.title,
    required this.body,
    required this.route,
    required this.call,
  });

  final String id;
  final String title;
  final String body;
  final String route;
  final bool call;
}
