import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:chakuchuri_mobile/src/api_client.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final bodyStarted in [false, true]) {
    test(
      'request deadline covers stalled response body=$bodyStarted and allows recovery',
      () async {
        final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
        addTearDown(() => server.close(force: true));
        var count = 0;
        server.listen((request) async {
          await request.drain<void>();
          count++;
          if (count == 1) {
            if (bodyStarted) {
              request.response.write('{');
              await request.response.flush();
            }
            await Future<void>.delayed(const Duration(seconds: 1));
            await request.response.close();
            return;
          }
          request.response.write(
            jsonEncode({
              'ok': true,
              'data': {
                'user': {'id': 'u'},
              },
            }),
          );
          await request.response.close();
        });
        final api = ApiClient(
          baseUrl: 'http://127.0.0.1:${server.port}',
          requestTimeout: const Duration(milliseconds: 200),
        );
        await expectLater(
          api.currentUser(),
          throwsA(
            isA<ApiException>().having(
              (e) => e.code,
              'code',
              'request_timeout',
            ),
          ),
        );
        expect((await api.currentUser()).id, 'u');
        expect(count, 2);
      },
    );
  }

  test(
    'simultaneous files share a download and completed bytes are cached',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      var count = 0;
      server.listen((request) async {
        count++;
        request.response.add([1, 2, 3]);
        await request.response.close();
      });
      final api = ApiClient(baseUrl: 'http://127.0.0.1:${server.port}');
      final a = api.fetchFileBytes('one');
      final b = api.fetchFileBytes('one');
      expect(identical(a, b), isTrue);
      expect(await a, [1, 2, 3]);
      await b;
      expect(count, 1);
      await api.fetchFileBytes('one');
      expect(count, 1);
    },
  );
}
