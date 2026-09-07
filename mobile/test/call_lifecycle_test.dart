import 'dart:async';
import 'package:chakuchuri_mobile/src/api_client.dart';
import 'package:chakuchuri_mobile/src/direct_call.dart';
import 'package:chakuchuri_mobile/src/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class DelayedCallApi extends ApiClient {
  DelayedCallApi() { token = 'test'; }
  final config = Completer<JsonMap>();
  @override
  Future<JsonMap> callConfig() => config.future;
}

void main() {
  testWidgets('disposing during setup does not open native media afterwards', (tester) async {
    final api = DelayedCallApi();
    final session = Session.fromJson({'token': 'test', 'user': {'id': 'customer', 'role': 'Customer'}});
    await tester.pumpWidget(MaterialApp(home: Scaffold(body: MobileCallRoom(
      api: api, session: session,
      call: CallRequest.fromJson({'id': 'call', 'status': 'In call'}),
      onEnded: () async {},
    ))));
    await tester.pump();
    await tester.pumpWidget(const SizedBox.shrink());
    api.config.complete({});
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
}
