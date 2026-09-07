import 'dart:async';

import 'package:flutter/material.dart';

import 'api_client.dart';
import 'app_notifications.dart';
import 'customer_kyc.dart';
import 'models.dart';
import 'screens/auth_screen.dart';
import 'screens/customer_shell.dart';
import 'screens/welcome_screen.dart';
import 'screens/suspension_screen.dart';
import 'theme.dart';
import 'workspace_changes.dart';
import 'workspace_realtime.dart';

class ChakuChuriMobileApp extends StatefulWidget {
  const ChakuChuriMobileApp({super.key, this.api});
  final ApiClient? api;

  @override
  State<ChakuChuriMobileApp> createState() => _ChakuChuriMobileAppState();
}

class _ChakuChuriMobileAppState extends State<ChakuChuriMobileApp>
    with WidgetsBindingObserver {
  late final ApiClient _api = widget.api ?? ApiClient();
  Session? _session;
  Workspace _workspace = Workspace.empty();
  Customer? _customer;
  Timer? _refreshTimer;
  bool _loadingWorkspace = false;
  bool _showWelcome = true;
  bool _startOnSignup = false;
  String _notice = '';
  bool _restoring = true;
  int _sessionEpoch = 0;
  WorkspaceRealtimeClient? _realtime;
  Future<void>? _workspaceRequest;
  final List<JsonMap> _bufferedChanges = [];
  Timer? _resyncTimer;
  Timer? _identityTimer;
  Timer? _noticeTimer;
  Timer? _accountingTimer;
  bool _accountingLoading = false;
  int _accountingVersion = 0;
  bool _identityLoading = false;
  bool _identityAgain = false;
  bool _snapshotNeeded = true;
  int _activeActions = 0;
  String? _notificationRoute;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    AppNotifications.onRoute = (route) {
      if (mounted) setState(() => _notificationRoute = route);
    };
    _api.onMutation = (path, data) {
      final changes = changesFromMutation(path, data);
      _receiveChanges(changes);
      if (changes.isEmpty || mutationNeedsSnapshot(path)) {
        _scheduleResync();
      }
    };
    unawaited(AppNotifications.init());
    unawaited(_restoreSession());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _refreshTimer?.cancel();
    _realtime?.close();
    _resyncTimer?.cancel();
    _identityTimer?.cancel();
    _noticeTimer?.cancel();
    _accountingTimer?.cancel();
    _api.onMutation = null;
    AppNotifications.onRoute = null;
    _sessionEpoch++;
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (_session == null) return;
    if (state == AppLifecycleState.resumed) {
      if (_realtime == null) _startRealtime();
      unawaited(_loadIdentity());
    } else if (state == AppLifecycleState.paused &&
        !_workspace.calls.any(
          (call) => call.status == 'In call' || call.status == 'Ringing',
        )) {
      _realtime?.close();
      _realtime = null;
      _refreshTimer?.cancel();
    }
  }

  Future<void> _restoreSession() async {
    final session = await _api.restoreSession();
    if (!mounted) return;
    setState(() => _restoring = false);
    if (session != null) await _afterLogin(session);
  }

  void _startRealtime() {
    _realtime?.close();
    _realtime = WorkspaceRealtimeClient(
      api: _api,
      onChanges: _receiveChanges,
      onResync: _scheduleResync,
    )..connect();
    _refreshTimer?.cancel();
    _refreshTimer = Timer.periodic(const Duration(seconds: 30), (_) {
      if (_realtime?.connected != true || _snapshotNeeded) {
        unawaited(_loadWorkspace(silent: true));
        unawaited(_loadIdentity());
      }
    });
  }

  void _scheduleResync() {
    _snapshotNeeded = true;
    _resyncTimer ??= Timer(const Duration(milliseconds: 150), () {
      _resyncTimer = null;
      unawaited(_loadWorkspace(silent: true));
    });
  }

  void _scheduleAccounting() {
    _accountingVersion++;
    if (_accountingLoading || _accountingTimer != null) return;
    _accountingTimer = Timer(const Duration(milliseconds: 150), () async {
      _accountingTimer = null;
      if (!mounted || _session == null || _session!.user.isSuspended) return;
      _accountingLoading = true;
      final epoch = _sessionEpoch;
      final version = _accountingVersion;
      try {
        final metrics = await _api.accountingMetrics();
        if (mounted && epoch == _sessionEpoch && version == _accountingVersion) {
          setState(() => _workspace = _workspace.copyWith(metrics: {..._workspace.metrics, ...metrics}));
        }
      } catch (_) {
        // Keep the last authoritative total while offline.
      } finally {
        _accountingLoading = false;
        if (mounted && epoch == _sessionEpoch && version != _accountingVersion) _scheduleAccounting();
      }
    });
  }

  void _receiveChanges(List<JsonMap> changes) {
    if (!mounted || _session == null || changes.isEmpty) return;
    if (changes.any((change) => {'manufacturing', 'shipping', 'payments', 'ledger'}.contains(change['scope']))) _scheduleAccounting();
    for (final change in changes) {
      if (change['operation'] != 'invalidate') continue;
      if (change['scope'] == 'users' || change['scope'] == 'customers') {
        _identityTimer ??= Timer(const Duration(milliseconds: 200), () {
          _identityTimer = null;
          unawaited(_loadIdentity());
        });
      } else if (workspaceCollections.contains(change['scope'])) {
        _scheduleResync();
      }
    }
    if (_session!.user.isSuspended) return;
    if (_workspaceRequest != null) _bufferedChanges.addAll(changes);
    final next = applyWorkspaceChanges(_workspace, changes);
    if (identical(next, _workspace)) return;
    setState(() => _workspace = next);
    // Startup realtime events can arrive before the initial snapshot. Seeding
    // notifications from that partial state makes older records look new when
    // the complete workspace arrives.
    if (!_snapshotNeeded) {
      unawaited(
        AppNotifications.syncWorkspace(next, userId: _session!.user.id),
      );
    }
  }

  Future<void> _loadIdentity() async {
    if (_session == null) return;
    if (_identityLoading) {
      _identityAgain = true;
      return;
    }
    _identityLoading = true;
    final epoch = _sessionEpoch;
    var identityConfirmed = false;
    try {
      final user = await _api.currentUser();
      if (!mounted || epoch != _sessionEpoch) return;
      identityConfirmed = true;
      final wasSuspended = _session!.user.isSuspended;
      unawaited(_api.updateSessionUser(user));
      setState(() {
        _session = _session!.copyWith(user: user);
        if (user.isSuspended) {
          _workspace = Workspace.empty();
          _customer = null;
          _bufferedChanges.clear();
          _snapshotNeeded = true;
        }
      });
      if (user.isSuspended) return;
      if (wasSuspended) _scheduleResync();
      final customer = await _api.myCustomer(customerId: user.customerId);
      if (!mounted || epoch != _sessionEpoch) return;
      setState(() {
        _customer = customer;
      });
    } on ApiException catch (error) {
      if (mounted &&
          epoch == _sessionEpoch &&
          error.code == 'not_authenticated') {
        if (identityConfirmed) {
          // Suspension may begin between /auth/me and the customer request.
          _identityAgain = true;
        } else {
          _logout();
        }
      }
    } catch (_) {
      // The last known profile remains usable while the connection recovers.
    } finally {
      if (epoch == _sessionEpoch) {
        _identityLoading = false;
        if (_identityAgain) {
          _identityAgain = false;
          unawaited(_loadIdentity());
        }
      }
    }
  }

  Future<void> _afterLogin(Session session) async {
    _sessionEpoch++;
    setState(() {
      _session = session;
      _showWelcome = false;
      _notice = '';
    });
    AppNotifications.reset();
    _startRealtime();
    unawaited(AppNotifications.requestPermission());
    await _loadIdentity();
    await _loadWorkspace();
  }

  Future<void> _loadWorkspace({bool silent = false}) {
    if (_session == null || _session!.user.isSuspended)
      return Future<void>.value();
    if (_workspaceRequest != null) return _workspaceRequest!;
    final epoch = _sessionEpoch;
    _bufferedChanges.clear();
    final request = _fetchWorkspace(epoch, silent: silent);
    _workspaceRequest = request;
    return request.whenComplete(() {
      if (epoch == _sessionEpoch) _workspaceRequest = null;
    });
  }

  Future<void> _fetchWorkspace(int epoch, {required bool silent}) async {
    if (!silent) setState(() => _loadingWorkspace = true);
    try {
      final next = await _api.workspace();
      if (!mounted || epoch != _sessionEpoch) return;
      if (_session!.user.isSuspended) return;
      final merged = applyWorkspaceChanges(next, _bufferedChanges);
      _bufferedChanges.clear();
      _snapshotNeeded = false;
      setState(() {
        _workspace = merged;
      });
      unawaited(
        AppNotifications.syncWorkspace(merged, userId: _session!.user.id),
      );
    } on ApiException catch (error) {
      if (!mounted || epoch != _sessionEpoch) return;
      if (error.code == 'not_authenticated') {
        await _loadIdentity();
      } else if (!silent) {
        _showNotice(error.message);
      }
    } catch (_) {
      if (mounted && epoch == _sessionEpoch && !silent) {
        _showNotice('Could not connect. Check your connection and try again.');
      }
    } finally {
      if (mounted && epoch == _sessionEpoch)
        setState(() => _loadingWorkspace = false);
    }
  }

  Future<void> _loadMoreWorkspace(String scope) async {
    if (_session == null) return;
    final current = _workspace.pagination[scope];
    if (current == null || !current.hasMore) return;
    final epoch = _sessionEpoch;
    try {
      final payload = await _api.workspacePage(scope, current.loaded);
      if (!mounted || epoch != _sessionEpoch) return;
      final pagination = {
        ..._workspace.pagination,
        scope: PageInfo.fromJson(asMap(payload['pagination'])),
      };
      setState(() {
        switch (scope) {
          case 'products':
            _workspace = _workspace.copyWith(
              products: _mergeRows(
                _workspace.products,
                readList(payload, 'items', Product.fromJson),
                (row) => row.id,
              ),
              pagination: pagination,
            );
            break;
          case 'quotations':
            _workspace = _workspace.copyWith(
              quotations: _mergeRows(
                _workspace.quotations,
                readList(payload, 'items', Quotation.fromJson),
                (row) => row.id,
              ),
              pagination: pagination,
            );
            break;
          case 'manufacturing':
            _workspace = _workspace.copyWith(
              manufacturing: _mergeRows(
                _workspace.manufacturing,
                readList(payload, 'items', ManufacturingOrder.fromJson),
                (row) => row.id,
              ),
              pagination: pagination,
            );
            break;
          case 'shipping':
            _workspace = _workspace.copyWith(
              shipping: _mergeRows(
                _workspace.shipping,
                readList(payload, 'items', ShippingRequest.fromJson),
                (row) => row.id,
              ),
              pagination: pagination,
            );
            break;
          case 'payments':
            _workspace = _workspace.copyWith(
              payments: _mergeRows(
                _workspace.payments,
                readList(payload, 'items', Payment.fromJson),
                (row) => row.id,
              ),
              pagination: pagination,
            );
            break;
          case 'ledger':
            _workspace = _workspace.copyWith(
              ledger: _mergeRows(
                _workspace.ledger,
                readList(payload, 'items', LedgerEntry.fromJson),
                (row) => row.id,
              ),
              pagination: pagination,
            );
            break;
          case 'conversations':
            _workspace = _workspace.copyWith(
              conversations: _mergeRows(
                _workspace.conversations,
                readList(payload, 'items', Conversation.fromJson),
                (row) => row.id,
              ),
              pagination: pagination,
            );
            break;
          case 'calls':
            _workspace = _workspace.copyWith(
              calls: _mergeRows(
                _workspace.calls,
                readList(payload, 'items', CallRequest.fromJson),
                (row) => row.id,
              ),
              pagination: pagination,
            );
            break;
        }
      });
    } on ApiException catch (error) {
      if (mounted && epoch == _sessionEpoch) _showNotice(error.message);
    }
  }

  void _showNotice(String value) {
    _noticeTimer?.cancel();
    setState(() => _notice = value);
    _noticeTimer = Timer(const Duration(seconds: 4), () {
      if (mounted) setState(() => _notice = '');
    });
  }

  Future<void> _runAction(
    Future<void> Function() action,
    String success,
  ) async {
    if (_session == null || _session!.user.isSuspended) return;
    final epoch = _sessionEpoch;
    setState(() => _activeActions++);
    try {
      await action();
      if (!mounted || epoch != _sessionEpoch) return;
      _showNotice(success);
    } on ApiException catch (error) {
      if (!mounted || epoch != _sessionEpoch) return;
      _showNotice(error.message);
    } catch (_) {
      if (!mounted || epoch != _sessionEpoch) return;
      _showNotice('Action could not be completed.');
    } finally {
      if (mounted && epoch == _sessionEpoch) {
        setState(() {
          if (_activeActions > 0) _activeActions--;
        });
      }
    }
  }

  void _logout() {
    _sessionEpoch++;
    _realtime?.close();
    _realtime = null;
    _resyncTimer?.cancel();
    _resyncTimer = null;
    _identityTimer?.cancel();
    _identityTimer = null;
    _noticeTimer?.cancel();
    _workspaceRequest = null;
    _bufferedChanges.clear();
    _identityLoading = false;
    _identityAgain = false;
    _snapshotNeeded = true;
    final api = _api;
    _refreshTimer?.cancel();
    unawaited(api.logout());
    AppNotifications.reset();
    setState(() {
      _session = null;
      _loadingWorkspace = false;
      _notificationRoute = null;
      _workspace = Workspace.empty();
      _customer = null;
      _notice = '';
      _showWelcome = true;
      _startOnSignup = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'ChakuChuri.pk',
      theme: buildTheme(),
      home: _restoring
          ? const Scaffold(body: Center(child: CircularProgressIndicator()))
          : _session != null
          ? _session!.user.isSuspended
                ? SuspensionScreen(
                    key: ValueKey(_session!.user.id),
                    user: _session!.user,
                    onSignOut: _logout,
                    onCheckStatus: _loadIdentity,
                    onContact: (message) async {
                      final epoch = _sessionEpoch;
                      final user = await _api.contactAboutSuspension(message);
                      if (!mounted || epoch != _sessionEpoch) return;
                      await _api.updateSessionUser(user);
                      if (!mounted || epoch != _sessionEpoch) return;
                      setState(() => _session = _session!.copyWith(user: user));
                    },
                  )
                : CustomerShell(
                    api: _api,
                    session: _session!,
                    workspace: _workspace,
                    customer: _customer,
                    loading: _loadingWorkspace,
                    busy: _activeActions > 0,
                    notice: _notice,
                    notificationRoute: _notificationRoute,
                    onNotificationRouteHandled: () =>
                        setState(() => _notificationRoute = null),
                    onAction: _runAction,
                    onLogout: _logout,
                    onRefresh: () => _loadWorkspace(),
                    onLoadMore: _loadMoreWorkspace,
                    onCustomerUpdated: (customer) {
                      setState(() => _customer = customer);
                    },
                    onUserUpdated: (user) {
                      if (_session == null) return;
                      unawaited(_api.updateSessionUser(user));
                      setState(() {
                        _session = _session!.copyWith(user: user);
                        _notice = 'Personal account settings saved.';
                      });
                    },
                  )
          : _showWelcome
          ? WelcomeScreen(
              onGetStarted: () => setState(() {
                _showWelcome = false;
                _startOnSignup = true;
              }),
              onSignIn: () => setState(() {
                _showWelcome = false;
                _startOnSignup = false;
              }),
            )
          : AuthScreen(
              api: _api,
              startOnSignup: _startOnSignup,
              onBaseUrlChanged: _api.updateBaseUrl,
              onLogin: _afterLogin,
            ),
    );
  }
}

List<T> _mergeRows<T>(
  List<T> current,
  List<T> incoming,
  String Function(T) id,
) {
  final seen = current.map(id).toSet();
  return [...current, ...incoming.where((row) => seen.add(id(row)))];
}
