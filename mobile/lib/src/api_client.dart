import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:path_provider/path_provider.dart';

import 'customer_kyc.dart';
import 'models.dart';
import 'session_store.dart';
import 'shipping_rates.dart';
import 'http_transport.dart';

class ApiException implements Exception {
  const ApiException(
    this.message, {
    this.code = 'request_failed',
    this.requestId = '',
  });

  final String message;
  final String code;
  final String requestId;

  @override
  String toString() => message;
}

class UploadedFile {
  const UploadedFile({
    required this.id,
    required this.originalName,
    required this.mimeType,
    required this.byteSize,
  });

  factory UploadedFile.fromJson(JsonMap json) {
    return UploadedFile(
      id: readString(json, 'id'),
      originalName: readString(json, 'originalName'),
      mimeType: readString(json, 'mimeType'),
      byteSize: readInt(json, 'byteSize'),
    );
  }

  final String id;
  final String originalName;
  final String mimeType;
  final int byteSize;
}

class ApiClient {
  ApiClient({
    String? baseUrl,
    SessionStore store = const SessionStore(),
    this.requestTimeout = const Duration(seconds: 20),
    this.uploadTimeout = const Duration(seconds: 60),
  }) : _store = store,
       baseUrl = baseUrl ?? defaultBaseUrl;

  /// Release builds require HTTPS unless ALLOW_CLEARTEXT=true is baked in,
  /// or the host is clearly a local/LAN address for controlled demos.
  static const bool allowCleartextOverride = bool.fromEnvironment(
    'ALLOW_CLEARTEXT',
    defaultValue: false,
  );

  static bool get allowCleartext => !kReleaseMode || allowCleartextOverride;

  static String get defaultBaseUrl {
    const baked = String.fromEnvironment('API_BASE_URL');
    if (baked.trim().isNotEmpty) {
      return baked.trim().replaceAll(RegExp(r'/$'), '');
    }
    if (kReleaseMode && !allowCleartextOverride) {
      return 'https://chakuchuri.pk';
    }
    if (Platform.isAndroid) return 'http://10.0.2.2:8002';
    return 'http://127.0.0.1:8002';
  }

  static bool isPrivateOrLocalHost(String host) {
    final value = host.trim().toLowerCase();
    if (value.isEmpty) return false;
    if (value == 'localhost' ||
        value == '10.0.2.2' ||
        value.endsWith('.local')) {
      return true;
    }
    final parts = value.split('.');
    if (parts.length != 4) return false;
    final a = int.tryParse(parts[0]);
    final b = int.tryParse(parts[1]);
    final c = int.tryParse(parts[2]);
    final d = int.tryParse(parts[3]);
    if (a == null || b == null || c == null || d == null) return false;
    if ([a, b, c, d].any((part) => part < 0 || part > 255)) return false;
    if (a == 10) return true;
    if (a == 127) return true;
    if (a == 192 && b == 168) return true;
    if (a == 172 && b >= 16 && b <= 31) return true;
    return false;
  }

  static void assertTransportAllowed(Uri uri) {
    if (uri.scheme == 'https') return;
    if (uri.scheme != 'http') {
      throw const ApiException(
        'Enter a valid http:// or https:// server address.',
      );
    }
    if (allowCleartext || isPrivateOrLocalHost(uri.host)) return;
    throw const ApiException(
      'Production builds require https://. Use a debug build or LAN address for HTTP demos.',
    );
  }

  final HttpClient _client = HttpClient()
    ..connectionTimeout = const Duration(seconds: 12);
  String baseUrl;
  String token = '';
  final SessionStore _store;
  final Duration requestTimeout;
  final Duration uploadTimeout;
  final Map<String, Future<Uint8List>> _pendingFiles = {};
  final Map<String, Uint8List> _fileCache = {};
  Future<Directory?>? _thumbnailDirectory;
  int _fileCacheBytes = 0;
  Session? _session;
  Future<void> _storageWrite = Future<void>.value();
  bool storageAvailable = true;
  void Function(String path, Object? data)? onMutation;
  int _sessionVersion = 0;

  void _checkSession(int version) {
    if (version != _sessionVersion) {
      throw const ApiException(
        'Your session changed. Please try again.',
        code: 'session_changed',
      );
    }
  }

  Future<HttpBytes> _send(
    String method,
    Uri uri, {
    required String requestToken,
    required int version,
    void Function(HttpClientRequest)? writeBody,
    Map<String, String> headers = const {},
    Duration? timeout,
  }) async {
    assertTransportAllowed(uri);
    try {
      final response = await requestBytes(
        client: _client,
        method: method,
        uri: uri,
        timeout: timeout ?? requestTimeout,
        headers: {
          ...headers,
          if (requestToken.isNotEmpty)
            HttpHeaders.authorizationHeader: 'Bearer $requestToken',
        },
        writeBody: (request) {
          _checkSession(version);
          writeBody?.call(request);
        },
      );
      _checkSession(version);
      return response;
    } on TimeoutException {
      _checkSession(version);
      throw ApiException(
        method == 'GET' ? 'Request timed out. Please try again.' : 'Confirmation timed out. Check the latest status before submitting again.',
        code: 'request_timeout',
      );
    }
  }

  Future<Session?> restoreSession() async {
    try {
      final saved = await _store.read();
      if (saved == null) return null;
      final uri = Uri.tryParse(saved.baseUrl);
      if (uri == null ||
          !uri.hasAuthority ||
          !{'http', 'https'}.contains(uri.scheme))
        return null;
      try {
        assertTransportAllowed(uri);
      } on ApiException {
        await _saveSession(null);
        return null;
      }
      baseUrl = saved.baseUrl;
      final session = saved.session;
      final expiry = DateTime.tryParse(session?.expiresAt ?? '');
      if (session == null ||
          expiry == null ||
          !expiry.isAfter(DateTime.now())) {
        await _saveSession(null);
        return null;
      }
      _session = session;
      _sessionVersion++;
      token = session.token;
      return session;
    } catch (_) {
      storageAvailable = false;
      return null;
    }
  }

  Future<void> _saveSession(Session? session) {
    if (token != (session?.token ?? '')) _sessionVersion++;
    _session = session;
    token = session?.token ?? '';
    final saved = SessionStoreData(baseUrl: baseUrl, session: session);
    _storageWrite = _storageWrite
        .then((_) => _store.write(saved))
        .then((_) {
          storageAvailable = true;
        })
        .catchError((Object _) {
          storageAvailable = false;
        });
    return _storageWrite;
  }

  Future<void> updateSessionUser(User user) async {
    if (_session == null) return;
    await _saveSession(_session!.copyWith(user: user));
  }

  Uri webSocketUri(String path) {
    final uri = Uri.parse('$baseUrl$path');
    if (!{'http', 'https'}.contains(uri.scheme) ||
        !uri.hasAuthority ||
        uri.userInfo.isNotEmpty ||
        !path.startsWith('/')) {
      throw const ApiException('Enter a valid server address.');
    }
    assertTransportAllowed(uri);
    return uri.replace(scheme: uri.scheme == 'https' ? 'wss' : 'ws');
  }

  Future<User> currentUser() => _get<User>(
    '/api/auth/me',
    parser: (data) => User.fromJson(asMap(asMap(data)['user'])),
  );

  Future<User> contactAboutSuspension(String message) => _post<User>(
    '/api/auth/suspension/contact',
    {'message': message.trim()},
    parser: (data) => User.fromJson(asMap(data)),
  );

  void updateBaseUrl(String value) {
    final next = value.trim().replaceAll(RegExp(r'/+$'), '');
    final uri = Uri.tryParse(next);
    if (uri == null ||
        !uri.hasAuthority ||
        uri.userInfo.isNotEmpty ||
        uri.hasQuery ||
        uri.hasFragment ||
        !{'http', 'https'}.contains(uri.scheme)) {
      throw const ApiException(
        'Enter a valid http:// or https:// server address.',
      );
    }
    assertTransportAllowed(uri);
    if (baseUrl == next) return;
    _sessionVersion++;
    baseUrl = next;
    _clearFileCache(clearDisk: true);
    // Credentials belong to one server and must never follow an address change.
    unawaited(_saveSession(null));
  }

  Future<Session> login(String email, String password) async {
    final version = _sessionVersion;
    final session = await _post<Session>('/api/auth/login', {
      'email': email.trim(),
      'password': password,
    }, parser: (data) => Session.fromJson(asMap(data)));
    _checkSession(version);
    await _saveSession(session);
    return session;
  }

  Future<void> logout() async {
    _sessionVersion++;
    _clearFileCache(clearDisk: true);
    final oldToken = token;
    final logoutUri = Uri.parse('$baseUrl/api/auth/logout');
    final clearing = _saveSession(null);
    final version = _sessionVersion;
    try {
      if (oldToken.isNotEmpty) {
        await _send(
          'POST',
          logoutUri,
          requestToken: oldToken,
          version: version,
        );
      }
    } catch (_) {
      // Local sign-out should still succeed if the network is down.
    } finally {
      await clearing;
    }
  }

  Future<Session> signup({
    required String companyName,
    required String contactName,
    required String email,
    required String phone,
    required String country,
    required String password,
  }) async {
    final version = _sessionVersion;
    final session = await _post<Session>('/api/auth/signup', {
      'companyName': companyName.trim(),
      'contactName': contactName.trim(),
      'email': email.trim(),
      'phone': phone.trim(),
      'country': country.trim(),
      'password': password,
      'services': ['Manufacturing', 'Shipping', 'Chat', 'Calls'],
    }, parser: (data) => Session.fromJson(asMap(asMap(data)['session'])));
    _checkSession(version);
    await _saveSession(session);
    return session;
  }

  Future<Workspace> workspace() {
    return _get<Workspace>(
      '/api/workspace',
      parser: (data) => Workspace.fromJson(asMap(data)),
    );
  }

  Future<JsonMap> accountingMetrics() {
    return _get<JsonMap>('/api/accounting/metrics', parser: (data) => asMap(data));
  }

  Future<JsonMap> workspacePage(String scope, int offset) {
    final query = Uri(
      queryParameters: {'scope': scope, 'offset': '$offset', 'limit': '20'},
    ).query;
    return _get<JsonMap>(
      '/api/workspace/page?$query',
      parser: (data) => asMap(data),
    );
  }

  Future<List<Customer>> listCustomers() {
    return _get<List<Customer>>(
      '/api/customers',
      parser: (data) => readList(asMap(data), 'customers', Customer.fromJson),
    );
  }

  Future<Customer?> myCustomer({String customerId = ''}) async {
    final rows = await listCustomers();
    final needle = customerId.trim();
    if (needle.isNotEmpty) {
      for (final row in rows) {
        if (row.id == needle) return row;
      }
    }
    return rows.isEmpty ? null : rows.first;
  }

  Future<Customer> attachCustomerDocument({
    required String customerId,
    required String fileId,
    required String kind,
    String label = '',
    String originalName = '',
    String requestId = '',
  }) {
    return _post<Customer>(
      '/api/customers/${Uri.encodeComponent(customerId)}/documents',
      {
        'fileId': fileId,
        'kind': kind,
        'label': label,
        'originalName': originalName,
        'requestId': requestId,
      },
      parser: (data) =>
          Customer.fromJson(asMap(asMap(data)['customer'] ?? data)),
    );
  }

  Future<Customer> deleteCustomerDocument({
    required String customerId,
    required String documentId,
  }) {
    return _post<Customer>(
      '/api/customers/${Uri.encodeComponent(customerId)}/documents/${Uri.encodeComponent(documentId)}/delete',
      {},
      parser: (data) =>
          Customer.fromJson(asMap(asMap(data)['customer'] ?? data)),
    );
  }

  Future<Customer> submitCustomerDocuments(String customerId) {
    return _post<Customer>(
      '/api/customers/${Uri.encodeComponent(customerId)}/documents/submit',
      {},
      parser: (data) =>
          Customer.fromJson(asMap(asMap(data)['customer'] ?? data)),
    );
  }

  Future<Customer> openVerificationInvite(
    String customerId, {
    String note = '',
  }) {
    return _post<Customer>(
      '/api/customers/${Uri.encodeComponent(customerId)}/verification-invite',
      {
        'note': note.trim().isEmpty
            ? 'Complete your one-time identity form to get verified.'
            : note.trim(),
      },
      parser: (data) =>
          Customer.fromJson(asMap(asMap(data)['customer'] ?? data)),
    );
  }

  Future<Quotation> createQuote({
    required String productId,
    required String productName,
    required int quantity,
    required String notes,
    String imageName = '',
    String imageFileId = '',
  }) {
    return _post<Quotation>('/api/workflow/quotes', {
      'productId': productId,
      'productName': productName.trim(),
      'quantity': quantity,
      'notes': notes.trim(),
      'imageName': imageName.trim(),
      'imageFileId': imageFileId.trim(),
    }, parser: (data) => Quotation.fromJson(asMap(data)));
  }

  Future<UploadedFile> uploadFile({
    required List<int> bytes,
    required String filename,
    required String ownerType,
    String ownerId = '',
    String customerId = '',
  }) async {
    final quotationImage =
        ownerType == 'quotation_photo' || ownerType == 'quotation_reference';
    final maxBytes = quotationImage ? 2 * 1024 * 1024 : 10 * 1024 * 1024;
    if (bytes.isEmpty || bytes.length > maxBytes) {
      throw ApiException(
        quotationImage
            ? 'Quotation photos must be 2 MB or smaller — about a normal phone screenshot. Compress or choose a smaller image.'
            : 'Choose a file smaller than 10 MB.',
        code: 'invalid_file',
      );
    }

    final boundary = '----chakuchuri-${DateTime.now().microsecondsSinceEpoch}';
    final version = _sessionVersion;
    final requestToken = token;
    final response = await _send(
      'POST',
      Uri.parse('$baseUrl/api/files'),
      requestToken: requestToken,
      version: version,
      timeout: uploadTimeout,
      headers: {
        HttpHeaders.acceptHeader: 'application/json',
        HttpHeaders.contentTypeHeader:
            'multipart/form-data; boundary=$boundary',
      },
      writeBody: (request) {
        void addField(String name, String value) {
          if (value.trim().isEmpty) return;
          request.write('--$boundary\r\n');
          request.write('Content-Disposition: form-data; name="$name"\r\n\r\n');
          request.write('${value.trim()}\r\n');
        }

        addField('ownerType', ownerType);
        addField('ownerId', ownerId);
        addField('customerId', customerId);
        final safeName = filename.replaceAll(RegExp(r'[\r\n"]'), '_');
        request.write('--$boundary\r\n');
        request.write(
          'Content-Disposition: form-data; name="file"; filename="$safeName"\r\n',
        );
        request.write('Content-Type: ${_contentMimeType(safeName)}\r\n\r\n');
        request.add(bytes);
        request.write('\r\n--$boundary--\r\n');
      },
    );
    return _decodeResponse<UploadedFile>(
      response,
      version: version,
      parser: (data) => UploadedFile.fromJson(asMap(data)),
    );
  }

  Future<ManufacturingOrder> acceptQuote(String id) {
    return _post<ManufacturingOrder>(
      '/api/workflow/quotes/$id/accept',
      {},
      parser: (data) => ManufacturingOrder.fromJson(asMap(data)),
    );
  }

  Future<Quotation> rejectQuote(String id) {
    return _post<Quotation>(
      '/api/workflow/quotes/$id/reject',
      {},
      parser: (data) => Quotation.fromJson(asMap(data)),
    );
  }

  Future<ShippingRequest> createShipping({
    required String type,
    required String manufacturingId,
    required String courier,
    required String service,
    required String destination,
    required String zone,
    required String weight,
  }) {
    return _post<ShippingRequest>('/api/workflow/shipping', {
      'type': type,
      'manufacturingId': manufacturingId,
      'courier': courier,
      'service': service,
      'destination': destination.trim(),
      'zone': zone.trim(),
      'weight': weight.trim(),
    }, parser: (data) => ShippingRequest.fromJson(asMap(data)));
  }

  Future<ShippingRateSnapshot> shippingRateSnapshot() {
    return _get<ShippingRateSnapshot>(
      '/api/shipping-rates',
      parser: (data) => ShippingRateSnapshot.fromJson(asMap(data)),
    );
  }

  Future<ShippingLookupResponse> lookupShippingRates(
    ShippingLookupRequest request,
  ) {
    return _post<ShippingLookupResponse>(
      '/api/shipping-rates/lookup',
      request.toJson(),
      parser: (data) => ShippingLookupResponse.fromJson(asMap(data)),
    );
  }

  Future<ShippingRequest> bookShippingRate(ShippingBookingRequest request) {
    return _post<ShippingRequest>(
      '/api/shipping-rates/book',
      request.toJson(),
      parser: (data) => ShippingRequest.fromJson(asMap(data)),
    );
  }

  Future<Payment> createPayment({
    required String type,
    required int amount,
    String manufacturingId = '',
    String shippingId = '',
    required String proofName,
    required String proofFileId,
    required String note,
  }) {
    return _post<Payment>('/api/workflow/payments', {
      'type': type,
      'amount': amount,
      if (manufacturingId.trim().isNotEmpty) 'manufacturingId': manufacturingId,
      if (shippingId.trim().isNotEmpty) 'shippingId': shippingId,
      'proofName': proofName.trim(),
      'proofFileId': proofFileId.trim(),
      'note': note.trim(),
    }, parser: (data) => Payment.fromJson(asMap(data)));
  }

  Future<Conversation> sendMessage(
    String body, {
    String customerId = '',
    List<String> attachmentIds = const [],
  }) {
    return _post<Conversation>('/api/workflow/messages', {
      'body': body.trim(),
      if (customerId.trim().isNotEmpty) 'customerId': customerId.trim(),
      if (attachmentIds.isNotEmpty) 'attachmentIds': attachmentIds,
    }, parser: (data) => Conversation.fromJson(asMap(data)));
  }

  Future<Conversation> markConversationRead(String id) {
    return _post<Conversation>(
      '/api/workflow/messages/${Uri.encodeComponent(id)}/read',
      {},
      parser: (data) => Conversation.fromJson(asMap(data)),
    );
  }

  Future<ConversationMessagePage> conversationMessages(String id, int offset) {
    final query = Uri(queryParameters: {'offset': '$offset', 'limit': '20'})
        .query;
    return _get<ConversationMessagePage>(
      '/api/workflow/messages/${Uri.encodeComponent(id)}/page?$query',
      parser: (data) => ConversationMessagePage.fromJson(asMap(data)),
    );
  }

  Future<User> updateProfile({
    required String name,
    required String email,
    String currentPassword = '',
    String newPassword = '',
    String profileImageFileId = '',
    bool removeProfileImage = false,
  }) {
    return _post<User>('/api/auth/profile', {
      'name': name.trim(),
      'email': email.trim().toLowerCase(),
      'currentPassword': currentPassword,
      'newPassword': newPassword,
      'profileImageFileId': profileImageFileId.trim(),
      'removeProfileImage': removeProfileImage,
    }, parser: (data) => User.fromJson(asMap(data)));
  }

  Future<CallRequest> startCall({
    String conversationId = '',
    String subject = 'Audio call',
    String recipientName = 'ChakuChuri Support',
  }) {
    return _post<CallRequest>('/api/workflow/calls', {
      'conversationId': conversationId.trim(),
      'subject': subject.trim(),
      'recipientName': recipientName.trim(),
    }, parser: (data) => CallRequest.fromJson(asMap(data)));
  }

  Future<CallRequest> updateCallStatus(
    String id,
    String status, {
    String note = '',
  }) {
    return _post<CallRequest>('/api/workflow/calls/$id/status', {
      'status': status,
      'note': note.trim(),
    }, parser: (data) => CallRequest.fromJson(asMap(data)));
  }

  Future<CallRequest> endCall(String id) {
    return _post<CallRequest>(
      '/api/workflow/calls/$id/end',
      {},
      parser: (data) {
        return CallRequest.fromJson(asMap(data));
      },
    );
  }

  Future<CallRequest> heartbeatCall(String id) {
    return _post<CallRequest>(
      '/api/workflow/calls/$id/heartbeat',
      {},
      parser: (data) => CallRequest.fromJson(asMap(data)),
    );
  }

  Future<List<CallSignal>> listCallSignals(String id, {int after = 0}) {
    return _get<List<CallSignal>>(
      '/api/workflow/calls/$id/signals?after=$after',
      parser: (data) => (data as List? ?? const [])
          .map((item) => CallSignal.fromJson(asMap(item)))
          .toList(),
    );
  }

  Future<CallSignal> sendCallSignal(
    String id,
    String signalType,
    JsonMap payload,
  ) {
    return _post<CallSignal>('/api/workflow/calls/$id/signals', {
      'signalType': signalType,
      'payload': payload,
    }, parser: (data) => CallSignal.fromJson(asMap(data)));
  }

  Future<JsonMap> callConfig() {
    return _get<JsonMap>(
      '/api/workflow/calls/config',
      parser: (data) => asMap(data),
    );
  }

  Future<JsonMap> androidBuildInfo() {
    return _get<JsonMap>(
      '/api/mobile/android-build',
      parser: (data) => asMap(data),
    );
  }

  /// Downloads the latest APK from the API into app storage and returns the local path.
  Future<File> downloadAndroidApk({
    void Function(int received, int? total)? onProgress,
  }) async {
    final info = await androidBuildInfo();
    if (info['ready'] != true) {
      throw const ApiException(
        'Android update package is not available on the server yet.',
      );
    }
    final relative = (info['downloadUrl'] as String?)?.trim();
    final path = (relative != null && relative.startsWith('/'))
        ? relative
        : '/downloads/chakuchuri-android.apk';
    final uri = Uri.parse('$baseUrl$path');
    assertTransportAllowed(uri);

    final version = _sessionVersion;
    final request = await _client.getUrl(uri);
    request.headers.set(HttpHeaders.acceptHeader, '*/*');
    if (token.isNotEmpty) {
      request.headers.set(HttpHeaders.authorizationHeader, 'Bearer $token');
    }
    final response = await request.close().timeout(const Duration(minutes: 15));
    _checkSession(version);
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw ApiException(
        'Could not download the Android update (HTTP ${response.statusCode}).',
      );
    }
    final total = response.contentLength >= 0 ? response.contentLength : null;
    final dir = await getTemporaryDirectory();
    final file = File(
      '${dir.path}${Platform.pathSeparator}chakuchuri-android-update.apk',
    );
    final sink = file.openWrite();
    var received = 0;
    try {
      await for (final chunk in response) {
        _checkSession(version);
        sink.add(chunk);
        received += chunk.length;
        onProgress?.call(received, total);
      }
      await sink.flush();
    } finally {
      await sink.close();
    }
    if (received < 1024) {
      throw const ApiException('Downloaded update file was empty or incomplete.');
    }
    return file;
  }

  Future<Uint8List> fetchFileBytes(String fileId, {bool thumbnail = true}) {
    final key = jsonEncode([
      _sessionVersion,
      baseUrl,
      token,
      fileId.trim(),
      thumbnail,
    ]);
    final cached = _fileCache.remove(key);
    if (cached != null) {
      _fileCache[key] = cached;
      return Future<Uint8List>.value(cached);
    }
    final pending = _pendingFiles[key];
    if (pending != null) return pending;
    final request = _loadFileBytes(fileId, thumbnail: thumbnail).then((bytes) {
      _cacheFile(key, bytes);
      return bytes;
    });
    if (_pendingFiles.length >= 32) return request;
    final tracked = request.whenComplete(() {
      // Returning the removed future would make this completion await itself.
      _pendingFiles.remove(key);
    });
    _pendingFiles[key] = tracked;
    return tracked;
  }

  void _cacheFile(String key, Uint8List bytes) {
    const maxCacheBytes = 24 * 1024 * 1024;
    const maxEntries = 96;
    if (bytes.isEmpty || bytes.length > maxCacheBytes ~/ 2) return;
    final previous = _fileCache.remove(key);
    if (previous != null) _fileCacheBytes -= previous.length;
    _fileCache[key] = bytes;
    _fileCacheBytes += bytes.length;
    while (_fileCache.length > maxEntries || _fileCacheBytes > maxCacheBytes) {
      final oldestKey = _fileCache.keys.first;
      final removed = _fileCache.remove(oldestKey);
      if (removed != null) _fileCacheBytes -= removed.length;
    }
  }

  void _clearFileCache({required bool clearDisk}) {
    _pendingFiles.clear();
    _fileCache.clear();
    _fileCacheBytes = 0;
    if (clearDisk) unawaited(_clearDiskThumbnails());
  }

  Future<Directory?> _getThumbnailDirectory() {
    return _thumbnailDirectory ??= () async {
      try {
        final root = await getTemporaryDirectory();
        final directory = Directory(
          '${root.path}${Platform.pathSeparator}cc_thumbnails_v1',
        );
        if (!await directory.exists()) await directory.create(recursive: true);
        return directory;
      } catch (_) {
        return null;
      }
    }();
  }

  String _thumbnailFilename(String id) {
    final safe = id.replaceAll(RegExp(r'[^a-zA-Z0-9_-]'), '_');
    return '${safe.length > 100 ? safe.substring(0, 100) : safe}.thumb';
  }

  Future<Uint8List?> _readDiskThumbnail(String id) async {
    try {
      final directory = await _getThumbnailDirectory();
      if (directory == null) return null;
      final file = File(
        '${directory.path}${Platform.pathSeparator}${_thumbnailFilename(id)}',
      );
      if (!await file.exists()) return null;
      final stat = await file.stat();
      if (DateTime.now().difference(stat.modified) > const Duration(days: 30)) {
        await file.delete();
        return null;
      }
      final bytes = await file.readAsBytes();
      return bytes.isEmpty ? null : bytes;
    } catch (_) {
      return null;
    }
  }

  Future<void> _writeDiskThumbnail(String id, Uint8List bytes) async {
    if (bytes.isEmpty || bytes.length > 2 * 1024 * 1024) return;
    try {
      final directory = await _getThumbnailDirectory();
      if (directory == null) return;
      final file = File(
        '${directory.path}${Platform.pathSeparator}${_thumbnailFilename(id)}',
      );
      await file.writeAsBytes(bytes, flush: false);
      final files = await directory
          .list()
          .where((entry) => entry is File)
          .cast<File>()
          .toList();
      if (files.length <= 220) return;
      final dated = <({File file, DateTime modified})>[];
      for (final item in files) {
        dated.add((file: item, modified: (await item.stat()).modified));
      }
      dated.sort((a, b) => a.modified.compareTo(b.modified));
      for (final item in dated.take(dated.length - 180)) {
        await item.file.delete();
      }
    } catch (_) {
      // A cache write must never block the requested image.
    }
  }

  Future<void> _clearDiskThumbnails() async {
    final directory = await _getThumbnailDirectory();
    try {
      if (directory != null && await directory.exists()) {
        await directory.delete(recursive: true);
      }
    } catch (_) {
      // Cache cleanup is best effort.
    } finally {
      _thumbnailDirectory = null;
    }
  }

  Future<Uint8List> _loadFileBytes(
    String fileId, {
    required bool thumbnail,
  }) async {
    final version = _sessionVersion;
    final id = fileId.trim();
    if (id.isEmpty) {
      throw const ApiException('Missing file id.', code: 'invalid_file');
    }
    if (thumbnail) {
      final cached = await _readDiskThumbnail(id);
      _checkSession(version);
      if (cached != null) return cached;
    }
    final paths = thumbnail
        ? <String>['/api/files/$id/thumbnail', '/api/files/$id/content']
        : <String>['/api/files/$id/content'];
    Object? lastError;
    for (final path in paths) {
      _checkSession(version);
      try {
        final bytes = await _fetchBinary(path);
        if (thumbnail && path.endsWith('/thumbnail')) {
          unawaited(_writeDiskThumbnail(id, bytes));
        }
        return bytes;
      } on ApiException catch (error) {
        _checkSession(version);
        if (error.code != 'file_not_found') rethrow;
        lastError = error;
      }
    }
    if (lastError is ApiException) throw lastError;
    throw ApiException(
      lastError?.toString() ?? 'Unable to load file.',
      code: 'file_fetch_failed',
    );
  }

  Future<Uint8List> _fetchBinary(String path) async {
    final version = _sessionVersion;
    final requestToken = token;
    final response = await _sendRead(
      Uri.parse('$baseUrl$path'),
      requestToken: requestToken,
      version: version,
    );
    final bytes = response.bytes;
    _checkSession(version);
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw ApiException(
        'Unable to load file (${response.statusCode}).',
        code: response.statusCode == 404
            ? 'file_not_found'
            : 'file_fetch_failed',
      );
    }
    if (bytes.isEmpty) {
      throw const ApiException(
        'Empty file response.',
        code: 'file_fetch_failed',
      );
    }
    return bytes;
  }

  Future<T> _get<T>(String path, {required T Function(dynamic data) parser}) {
    return _request<T>('GET', path, parser: parser);
  }

  Future<T> _post<T>(
    String path,
    Object body, {
    required T Function(dynamic data) parser,
  }) {
    return _request<T>(
      'POST',
      path,
      body: body,
      parser: (data) {
        final result = parser(data);
        onMutation?.call(path, data);
        return result;
      },
    );
  }

  Future<T> _request<T>(
    String method,
    String path, {
    Object? body,
    required T Function(dynamic data) parser,
  }) async {
    final version = _sessionVersion;
    final requestToken = token;
    final uri = Uri.parse('$baseUrl$path');
    final headers = {
      HttpHeaders.acceptHeader: 'application/json',
      if (body != null) HttpHeaders.contentTypeHeader: 'application/json',
    };
    final response = method == 'GET'
        ? await _sendRead(
            uri,
            requestToken: requestToken,
            version: version,
            headers: headers,
          )
        : await _send(
            method,
            uri,
            requestToken: requestToken,
            version: version,
            headers: headers,
            writeBody: (request) {
              if (body != null) request.write(jsonEncode(body));
            },
          );
    return _decodeResponse<T>(response, parser: parser, version: version);
  }

  Future<HttpBytes> _sendRead(
    Uri uri, {
    required String requestToken,
    required int version,
    Map<String, String> headers = const {},
  }) async {
    for (var attempt = 0; ; attempt++) {
      try {
        return await _send(
          'GET',
          uri,
          requestToken: requestToken,
          version: version,
          headers: headers,
        );
      } catch (error) {
        final transient = error is SocketException || error is HttpException;
        if (!transient || attempt > 0) rethrow;
        _checkSession(version);
        await Future<void>.delayed(const Duration(milliseconds: 250));
      }
    }
  }

  Future<T> _decodeResponse<T>(
    HttpBytes response, {
    required T Function(dynamic data) parser,
    required int version,
  }) async {
    final raw = utf8.decode(response.bytes);
    _checkSession(version);
    final payload = raw.isEmpty
        ? <String, dynamic>{}
        : jsonDecode(raw) as Map<String, dynamic>;
    final ok =
        payload['ok'] == true &&
        response.statusCode >= 200 &&
        response.statusCode < 300;
    if (!ok) {
      final error = asMap(payload['error']);
      throw ApiException(
        readString(error, 'message', 'Request failed.'),
        code: readString(error, 'code'),
        requestId: readString(payload, 'requestId'),
      );
    }
    return parser(payload['data']);
  }
}

String _contentMimeType(String filename) {
  final lower = filename.toLowerCase();
  if (lower.endsWith('.png')) return 'image/png';
  if (lower.endsWith('.webp')) return 'image/webp';
  if (lower.endsWith('.gif')) return 'image/gif';
  if (lower.endsWith('.pdf')) return 'application/pdf';
  if (lower.endsWith('.txt')) return 'text/plain';
  if (lower.endsWith('.csv')) return 'text/csv';
  if (lower.endsWith('.xls')) return 'application/vnd.ms-excel';
  if (lower.endsWith('.xlsx')) {
    return 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
  }
  if (lower.endsWith('.doc')) return 'application/msword';
  if (lower.endsWith('.docx')) {
    return 'application/vnd.openxmlformats-officedocument.wordprocessingml.document';
  }
  if (lower.endsWith('.ppt')) return 'application/vnd.ms-powerpoint';
  if (lower.endsWith('.pptx')) {
    return 'application/vnd.openxmlformats-officedocument.presentationml.presentation';
  }
  if (lower.endsWith('.zip')) return 'application/zip';
  if (lower.endsWith('.jpg') || lower.endsWith('.jpeg')) return 'image/jpeg';
  return 'application/octet-stream';
}

JsonMap asMap(Object? value) {
  if (value is Map<String, dynamic>) return value;
  if (value is Map) return Map<String, dynamic>.from(value);
  return <String, dynamic>{};
}
