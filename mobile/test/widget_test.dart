import 'package:flutter_test/flutter_test.dart';

import 'package:chakuchuri_mobile/src/app.dart';
import 'package:chakuchuri_mobile/src/api_client.dart';
import 'package:chakuchuri_mobile/src/models.dart';

class SignedOutApi extends ApiClient {
  @override
  Future<Session?> restoreSession() async => null;
}

void main() {
  testWidgets('shows customer auth screen', (tester) async {
    await tester.pumpWidget(ChakuChuriMobileApp(api: SignedOutApi()));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('I already have an account'));
    await tester.tap(find.text('I already have an account'));
    await tester.pumpAndSettle();

    expect(find.text('ChakuChuri.pk'), findsOneWidget);
    expect(find.text('Login'), findsWidgets);
    expect(find.text('Create account'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
