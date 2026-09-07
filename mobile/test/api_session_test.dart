import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:chakuchuri_mobile/src/api_client.dart';
import 'package:chakuchuri_mobile/src/session_store.dart';
import 'package:flutter_test/flutter_test.dart';

class MemorySessionStore extends SessionStore {
  SessionStoreData? saved;
  @override
  Future<void> write(SessionStoreData data) async { saved = data; }
  @override
  Future<SessionStoreData?> read() async => saved;
}

void main() {
  test('sign-out cancels a pending anonymous login', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    final arrived = Completer<void>();
    final release = Completer<void>();
    server.listen((request) async {
      await request.drain<void>();
      arrived.complete();
      await release.future;
      request.response.write(jsonEncode({'ok': true, 'data': {
        'token': 'late-session', 'expiresAt': '2099-01-01T00:00:00Z', 'user': {'id': 'a'},
      }}));
      await request.response.close();
    });
    final api = ApiClient(baseUrl: 'http://127.0.0.1:' + server.port.toString(), store: MemorySessionStore());
    final pending = api.login('a', 'password');
    final rejected = expectLater(pending, throwsA(isA<ApiException>().having((e) => e.code, 'code', 'session_changed')));
    await arrived.future;
    await api.logout();
    release.complete();
    await rejected;
    expect(api.token, isEmpty);
  });

  test('late mutation from the old account is rejected before publishing', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    final arrived = Completer<void>();
    final release = Completer<void>();
    var loginCount = 0;
    String? quoteAuthorization;
    server.listen((request) async {
      await request.drain<void>();
      Object data = <String, dynamic>{};
      if (request.uri.path == '/api/auth/login') {
        loginCount++;
        data = {'token': 'session-$loginCount', 'expiresAt': '2099-01-01T00:00:00Z',
          'user': {'id': 'user-$loginCount', 'status': 'Active', 'role': 'Customer'}};
      } else {
        quoteAuthorization = request.headers.value(HttpHeaders.authorizationHeader);
        arrived.complete();
        await release.future;
        data = {'id': 'private-old-quote'};
      }
      request.response.headers.contentType = ContentType.json;
      request.response.write(jsonEncode({'ok': true, 'data': data}));
      await request.response.close();
    });
    final api = ApiClient(baseUrl: 'http://127.0.0.1:' + server.port.toString(), store: MemorySessionStore());
    await api.login('a', 'password');
    final published = <String>[];
    api.onMutation = (path, _) { if (path.contains('/quotes')) published.add(path); };
    final pending = api.rejectQuote('old');
    final rejected = expectLater(pending, throwsA(isA<ApiException>().having((e) => e.code, 'code', 'session_changed')));
    await arrived.future;
    await api.login('b', 'password');
    release.complete();
    await rejected;
    expect(quoteAuthorization, 'Bearer session-1');
    expect(api.token, 'session-2');
    expect(published, isEmpty);
  });

  test('file fallback never retries with credentials from a new server session', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    final arrived = Completer<void>();
    final release = Completer<void>();
    var requests = 0;
    server.listen((request) async {
      requests++;
      arrived.complete();
      await release.future;
      request.response.statusCode = 404;
      await request.response.close();
    });
    final api = ApiClient(baseUrl: 'http://127.0.0.1:' + server.port.toString(), store: MemorySessionStore());
    final pending = api.fetchFileBytes('old');
    final rejected = expectLater(pending, throwsA(isA<ApiException>().having((e) => e.code, 'code', 'session_changed')));
    await arrived.future;
    api.updateBaseUrl('http://localhost:' + server.port.toString());
    release.complete();
    await rejected;
    expect(requests, 1);
  });
}
