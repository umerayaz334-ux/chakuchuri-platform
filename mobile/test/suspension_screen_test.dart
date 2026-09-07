import 'package:chakuchuri_mobile/src/models.dart';
import 'package:chakuchuri_mobile/src/screens/suspension_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final user = User.fromJson({
    'id': 'c1', 'status': 'Temporarily suspended',
    'suspensionNotice': 'Please contact us about your account.',
    'suspensionContactMessage': 'Please review my access.',
  });

  test('suspension details survive storage and profile copies', () {
    final restored = User.fromJson(user.copyWith(name: 'Updated').toJson());
    expect(restored.isSuspended, isTrue);
    expect(restored.suspensionNotice, user.suspensionNotice);
    expect(restored.suspensionContactMessage, user.suspensionContactMessage);
  });

  testWidgets('suspension screen sends contact and fits a narrow screen', (tester) async {
    tester.view.physicalSize = const Size(360, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    var sent = '';
    var checked = 0;
    var signedOut = false;
    await tester.pumpWidget(MaterialApp(home: SuspensionScreen(
      user: user,
      onContact: (message) async { sent = message; },
      onCheckStatus: () async { checked++; },
      onSignOut: () { signedOut = true; },
    )));
    expect(find.text(user.suspensionNotice), findsOneWidget);
    await tester.tap(find.text('Contact administrator'));
    await tester.pumpAndSettle();
    expect(sent, 'Please review my access.');
    expect(find.text('Message sent to administrator.'), findsOneWidget);
    await tester.tap(find.text('Check status'));
    await tester.pumpAndSettle();
    expect(checked, 1);
    await tester.tap(find.byTooltip('Sign out'));
    expect(signedOut, isTrue);
    expect(tester.takeException(), isNull);
  });
}
