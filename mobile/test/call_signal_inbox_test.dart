import 'dart:async';
import 'package:chakuchuri_mobile/src/call_signal_inbox.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('peer operations are serialized and duplicate signals apply once', () async {
    final inbox = CallSignalInbox();
    final release = Completer<void>();
    final calls = <String>[];
    final offer = inbox.process('offer', () async { calls.add('offer'); await release.future; });
    final duplicate = inbox.process('offer', () async { calls.add('duplicate'); });
    final ice = inbox.process('ice', () async { calls.add('ice'); });
    await Future<void>.delayed(Duration.zero);
    expect(calls, ['offer']);
    release.complete();
    await Future.wait([offer, duplicate, ice]);
    expect(calls, ['offer', 'ice']);
  });

  test('failed signals can be retried without poisoning the queue', () async {
    final inbox = CallSignalInbox();
    await expectLater(inbox.process('answer', () async { throw StateError('temporary'); }), throwsStateError);
    var applied = 0;
    await inbox.process('answer', () async { applied++; });
    await inbox.process('answer', () async { applied++; });
    expect(applied, 1);
  });

  test('closing discards queued native operations', () async {
    final inbox = CallSignalInbox();
    final release = Completer<void>();
    final first = inbox.process('offer', () => release.future);
    var laterRan = false;
    final later = inbox.process('ice', () async { laterRan = true; });
    await Future<void>.delayed(Duration.zero);
    inbox.close();
    release.complete();
    await Future.wait([first, later]);
    expect(laterRan, isFalse);
  });
}
