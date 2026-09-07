import 'dart:convert';
import 'dart:io';

import 'package:chakuchuri_mobile/src/api_client.dart';
import 'package:chakuchuri_mobile/src/models.dart';
import 'package:chakuchuri_mobile/src/screens/customer_shell.dart';
import 'package:chakuchuri_mobile/src/shipping_rates.dart';
import 'package:chakuchuri_mobile/src/theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    // Use the SDK's Android fonts so screenshots show real text and icons.
    final configFile = File('.dart_tool/package_config.json').absolute;
    final config = jsonDecode(await configFile.readAsString()) as Map;
    final flutter = (config['packages'] as List).firstWhere(
      (entry) => entry['name'] == 'flutter',
    ) as Map;
    final sdk = Directory.fromUri(
      configFile.uri.resolve(flutter['rootUri'] as String),
    ).parent.parent;
    final fonts = '${sdk.path}/bin/cache/artifacts/material_fonts';
    for (final family in ['Roboto', 'Ahem']) {
      final textLoader = FontLoader(family);
      for (final weight in ['regular', 'medium', 'bold', 'black']) {
        textLoader.addFont(
          File('$fonts/roboto-$weight.ttf')
              .readAsBytes()
              .then(ByteData.sublistView),
        );
      }
      await textLoader.load();
    }
    final iconLoader = FontLoader('MaterialIcons');
    iconLoader.addFont(
      File('$fonts/materialicons-regular.otf')
          .readAsBytes()
          .then(ByteData.sublistView),
    );
    await iconLoader.load();
  });

  testWidgets('customer shell stays polished on a narrow phone', (
    tester,
  ) async {
    await _setPhone(tester, const Size(360, 800));
    await _pumpShell(tester);

    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/customer_dashboard_360.png'),
    );
  });

  testWidgets('dashboard order thumbnail is larger and opens order details', (
    tester,
  ) async {
    await _setPhone(tester, const Size(390, 844));
    final api = _ThumbnailApi();
    final workspace = _workspace.copyWith(
      manufacturing: [
        ManufacturingOrder.fromJson({
          'id': 'MFG-104',
          'productName': 'Product name belongs in details',
          'quantity': 250,
          'status': 'Production',
          'currentStage': 'Finishing',
          'progress': 64,
          'imageFileId': 'photo-104',
        }),
      ],
    );
    await _pumpShell(tester, api: api, workspace: workspace);
    final card = find.byKey(const ValueKey('dashboard-order-MFG-104'));
    final photo = find.descendant(of: card, matching: find.byType(Image));
    expect(photo, findsOneWidget);
    expect(tester.getSize(photo).width, 124);
    expect(tester.getSize(photo).height, 124);
    expect(api.thumbnailRequests, ['photo-104']);
    expect(find.text('Product name belongs in details'), findsNothing);
    expect(find.text('SEP'), findsNothing);
    await tester.tap(card);
    await tester.pumpAndSettle();
    expect(find.byType(BottomSheet), findsOneWidget);
    expect(find.text('Product name belongs in details'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'dashboard carousels preserve reading position and open details',
    (tester) async {
      await _setPhone(tester, const Size(390, 844));
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      final workspace = _workspace.copyWith(
        manufacturing: [
          for (var i = 1; i <= 3; i++)
            ManufacturingOrder.fromJson({
              'id': 'MFG-10$i',
              'productName': 'Production $i',
              'quantity': 20,
              'status': 'Production',
              'progress': 30,
            }),
        ],
      );
      await _pumpShell(tester, workspace: workspace);
      await tester.ensureVisible(find.byTooltip('Next orders'));
      await tester.pumpAndSettle();
      final controller = tester
          .widget<PageView>(find.byType(PageView).first)
          .controller!;
      expect(controller.page, 0);
      await tester.pump(const Duration(seconds: 6));
      expect(controller.page, 0);
      await tester.tap(find.byTooltip('Next orders'));
      await tester.pumpAndSettle();
      expect(controller.page!.round(), 1);
      await tester.pump(const Duration(seconds: 6));
      await tester.pumpAndSettle();
      expect(controller.page!.round(), 1);
      await tester.tap(find.byTooltip('Next orders'));
      await tester.pumpAndSettle();
      expect(controller.page!.round(), 2);
      final card = find.byKey(const ValueKey('dashboard-order-MFG-101'));
      tester.state<ScrollableState>(find.byType(Scrollable).first).position.jumpTo(0);
      await tester.pumpAndSettle();
      await tester.tap(card);
      await tester.pumpAndSettle();
      expect(find.byType(BottomSheet), findsOneWidget);
      final pageBeforeSheet = controller.page;
      await tester.pump(const Duration(seconds: 6));
      await tester.pumpAndSettle();
      expect(controller.page, pageBeforeSheet);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('dashboard handles large text and reduced motion', (
    tester,
  ) async {
    await _setPhone(tester, const Size(360, 800));
    tester.platformDispatcher.textScaleFactorTestValue = 1.8;
    tester.platformDispatcher.accessibilityFeaturesTestValue =
        const FakeAccessibilityFeatures(disableAnimations: true);
    addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
    addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);
    await _pumpShell(tester);
    expect(tester.takeException(), isNull);
    await tester.scrollUntilVisible(
      find.byTooltip('Next quotations'),
      200,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.pumpAndSettle();
    expect(find.byTooltip('Pause quotations carousel'), findsNothing);
    final pageView = tester.widget<PageView>(find.byType(PageView).first);
    await tester.pump(const Duration(seconds: 10));
    final before = pageView.controller!.page!.round();
    expect(before, 0);
    await tester.tap(find.byTooltip('Next quotations'));
    await tester.pumpAndSettle();
    expect(pageView.controller!.page!.round(), 1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('critical customer tabs render at phone size', (tester) async {
    await _setPhone(tester, const Size(390, 844));
    await _pumpShell(tester);

    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/customer_dashboard_390.png'),
    );

    await tester.tap(find.text('Quote'));
    await tester.pumpAndSettle();
    expect(find.text('All'), findsOneWidget);
    expect(find.text('Accepted'), findsOneWidget);
    expect(find.text('Rejected / cancelled'), findsOneWidget);
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/customer_quotes_390.png'),
    );
    await tester.tap(find.text('Request a quote'));
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<TextField>(find.widgetWithText(TextField, 'Quantity'))
          .controller!
          .text,
      isEmpty,
    );
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/customer_quote_form_390.png'),
    );
    await tester.tap(find.byTooltip('Close'));
    await tester.pumpAndSettle();

    await tester.tap(find.descendant(of: find.byType(NavigationBar), matching: find.text('Orders')));
    await tester.pumpAndSettle();
    expect(find.text('Progress, expected dates and balances.'), findsNothing);
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/customer_orders_390.png'),
    );

    await tester.tap(find.text('Ship'));
    await tester.pumpAndSettle();
    expect(find.text('Logistics'), findsNothing);
    expect(find.textContaining('Compare live services'), findsNothing);
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/customer_shipping_390.png'),
    );
    await tester.tap(find.text('Pay'));
    await tester.pumpAndSettle();
    expect(find.text('Your account'), findsNothing);
    expect(find.text('Balance, payment proofs and statement.'), findsNothing);
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/customer_payments_390.png'),
    );
    await tester.tap(find.text('Upload payment proof'));
    await tester.pumpAndSettle();
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/customer_payment_form_390.png'),
    );

    expect(find.text('Home'), findsOneWidget);
    expect(find.text('Quote'), findsOneWidget);
    expect(find.text('Orders'), findsOneWidget);
    expect(find.text('Ship'), findsOneWidget);
    expect(find.text('Pay'), findsOneWidget);
  });
  testWidgets(
    'shipping offers open in a popup and remain available after closing',
    (tester) async {
      await _setPhone(tester, const Size(390, 844));
      final api = _ShippingApi();
      await _pumpShell(tester, api: api);
      await tester.tap(find.text('Ship'));
      await tester.pumpAndSettle();
      expect(find.text('Add box dimensions (optional)'), findsOneWidget);
      await tester.enterText(
        find.widgetWithText(TextField, 'ZIP code'),
        '10001',
      );
      await tester.ensureVisible(find.text('Show rates'));
      await tester.tap(find.text('Show rates'));
      await tester.pumpAndSettle();
      expect(api.lastRequest?.postalCode, '10001');
      expect(api.lastRequest?.lengthCm, 0);
      expect(find.byType(BottomSheet), findsOneWidget);
      expect(find.text('Available services'), findsOneWidget);
      expect(find.text('Express'), findsOneWidget);
      await expectLater(
        find.byType(MaterialApp),
        matchesGoldenFile('goldens/customer_shipping_offers_390.png'),
      );
      await tester.tap(find.byTooltip('Close'));
      await tester.pumpAndSettle();
      expect(find.byType(BottomSheet), findsNothing);
      await tester.ensureVisible(find.text('View last offers'));
      await tester.tap(find.text('View last offers'));
      await tester.pumpAndSettle();
      expect(find.text('Available services'), findsOneWidget);
      expect(api.lookupCount, 1);
    },
  );
}

class _ShippingApi extends ApiClient {
  ShippingLookupRequest? lastRequest;
  int lookupCount = 0;

  @override
  Future<ShippingRateSnapshot> shippingRateSnapshot() async =>
      const ShippingRateSnapshot(books: [], activeBooks: 1, activeServices: 1);

  @override
  Future<ShippingLookupResponse> lookupShippingRates(
    ShippingLookupRequest request,
  ) async {
    lookupCount++;
    lastRequest = request;
    return ShippingLookupResponse.fromJson({
      'postalPrefix': '100',
      'zone': '1',
      'region': 'New York',
      'options': [
        {
          'serviceId': 'express',
          'rateBookId': 'book-1',
          'carrier': 'DHL',
          'service': 'Express',
          'currency': 'PKR',
          'totalAmount': 12500,
          'baseAmount': 12500,
          'etaMinDays': 3,
          'etaMaxDays': 5,
          'chargeableWeightKg': 1,
          'packages': 1,
        },
      ],
    });
  }
}

Future<void> _setPhone(WidgetTester tester, Size size) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
}

Future<void> _pumpShell(
  WidgetTester tester, {
  ApiClient? api,
  Workspace? workspace,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      debugShowCheckedModeBanner: false,
      theme: _goldenTheme(),
      home: CustomerShell(
        dashboardDate: DateTime(2026, 9, 6, 9),
        api: api ?? ApiClient(baseUrl: 'http://127.0.0.1:8002'),
        session: _session,
        workspace: workspace ?? _workspace,
        loading: false,
        busy: false,
        notice: '',
        onAction: _ignoreAction,
        onLogout: _noop,
        onRefresh: _refresh,
        onLoadMore: (_) async {},
        onUserUpdated: (_) {},
      ),
    ),
  );
  await tester.pumpAndSettle();
}

class _ThumbnailApi extends ApiClient {
  final thumbnailRequests = <String>[];

  @override
  Future<Uint8List> fetchFileBytes(
    String fileId, {
    bool thumbnail = true,
  }) async {
    if (thumbnail) thumbnailRequests.add(fileId);
    return base64Decode(
      'R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7',
    );
  }
}

ThemeData _goldenTheme() {
  final theme = buildTheme();
  // Button styles without a font family otherwise use the test-only Ahem font.
  ButtonStyle font(ButtonStyle style) => style.copyWith(
    textStyle: WidgetStatePropertyAll(
      style.textStyle?.resolve({})?.copyWith(fontFamily: 'Roboto'),
    ),
  );
  return theme.copyWith(
    filledButtonTheme: FilledButtonThemeData(style: font(theme.filledButtonTheme.style!)),
    textButtonTheme: TextButtonThemeData(style: font(theme.textButtonTheme.style!)),
    outlinedButtonTheme: OutlinedButtonThemeData(style: font(theme.outlinedButtonTheme.style!)),
  );
}

Future<void> _ignoreAction(
  Future<void> Function() action,
  String success,
) async {}

Future<void> _refresh() async {}
void _noop() {}

const _session = Session(
  token: 'visual-test',
  expiresAt: '2026-12-31T00:00:00Z',
  user: User(
    id: 'USR-104',
    name: 'Ahsan Export Co.',
    email: 'ahsan@example.com',
    role: 'Customer',
    status: 'Active',
    permissions: [],
    pageAccess: [],
    customerId: 'CUS-104',
  ),
);

final _workspace = Workspace.fromJson({
  'products': [
    {
      'id': 'PRD-12',
      'sku': 'CC-KNF-012',
      'name': 'D2 Bushcraft Knife',
      'stock': 120,
      'reserved': 40,
    },
  ],
  'quotations': [
    {
      'id': 'QT-2026-1042',
      'productName': 'Hand-forged D2 hunting knife',
      'quantity': 250,
      'status': 'Priced',
      'totalAmount': 468000,
      'depositRequired': 140400,
      'expectedDate': '2026-10-02T00:00:00Z',
      'notes': 'Matte finish with leather sheath.',
      'history': [],
    },
    {
      'id': 'QT-2026-1038',
      'productName': 'Custom Viking axe',
      'quantity': 80,
      'status': 'Requested',
      'totalAmount': 0,
      'depositRequired': 0,
      'expectedDate': '',
      'notes': '',
      'history': [],
    },
  ],
  'manufacturing': [
    {
      'id': 'MFG-2026-0084',
      'productName': 'D2 Bushcraft Knife / Walnut Handle',
      'quantity': 250,
      'status': 'Production',
      'currentStage': 'Assembly and handle finishing',
      'progress': 64,
      'expectedDate': '2026-09-18T00:00:00Z',
      'totalAmount': 468000,
      'paidAmount': 140400,
      'balanceDue': 327600,
      'stages': [
        {
          'name': 'Order confirmed',
          'status': 'Completed',
          'date': '2026-08-18',
        },
        {
          'name': 'Deposit received',
          'status': 'Completed',
          'date': '2026-08-19',
        },
        {'name': 'Production', 'status': 'Current', 'date': '2026-08-21'},
        {'name': 'Quality check', 'status': 'Pending', 'date': ''},
        {'name': 'Ready to ship', 'status': 'Pending', 'date': ''},
      ],
      'history': [
        {
          'label': 'Assembly completed',
          'detail': 'Handle finishing is now in progress.',
          'actor': 'Production team',
          'createdAt': '2026-09-02T09:30:00Z',
        },
      ],
    },
  ],
  'rateSheets': [
    {
      'id': 'RATE-1',
      'courier': 'DHL',
      'service': 'Express',
      'zone': 'USA Zone 4',
      'weight': '5 kg',
      'price': 28400,
      'status': 'Active',
    },
    {
      'id': 'RATE-2',
      'courier': 'FedEx',
      'service': 'Priority',
      'zone': 'UK Zone 2',
      'weight': '5 kg',
      'price': 24900,
      'status': 'Active',
    },
  ],
  'shipping': [
    {
      'id': 'SHP-2026-0198',
      'type': 'Manufactured order',
      'courier': 'DHL',
      'service': 'Express',
      'destination': 'Austin, Texas, United States',
      'zone': 'USA Zone 4',
      'weight': '18.5 kg',
      'status': 'In transit',
      'tracking': 'DHL-883104729',
      'quotedAmount': 62400,
      'createdAt': '2026-08-29T10:00:00Z',
    },
    {
      'id': 'SHP-2026-0189',
      'type': 'Outside product',
      'courier': 'FedEx',
      'service': 'Priority',
      'destination': 'Birmingham, United Kingdom',
      'zone': 'UK Zone 2',
      'weight': '8 kg',
      'status': 'Delivered',
      'tracking': 'FDX-6620431',
      'quotedAmount': 38900,
      'createdAt': '2026-08-18T10:00:00Z',
    },
  ],
  'payments': [
    {
      'id': 'PAY-2026-302',
      'type': 'Deposit',
      'amount': 140400,
      'status': 'Confirmed',
      'proofName': 'bank-transfer-aug-19.jpg',
      'manufacturingId': 'MFG-2026-0084',
      'createdAt': '2026-08-19T13:20:00Z',
    },
    {
      'id': 'PAY-2026-318',
      'type': 'Shipping',
      'amount': 62400,
      'status': 'Pending',
      'proofName': 'dhl-transfer-sep-02.png',
      'shippingId': 'SHP-2026-0198',
      'createdAt': '2026-09-02T08:10:00Z',
    },
  ],
  'ledger': [
    {
      'id': 'LED-401',
      'sourceType': 'Manufacturing',
      'debit': 468000,
      'credit': 0,
      'note': 'Manufacturing order MFG-2026-0084',
      'postedAt': '2026-08-18T10:00:00Z',
    },
    {
      'id': 'LED-402',
      'sourceType': 'Payment',
      'debit': 0,
      'credit': 140400,
      'note': 'Confirmed manufacturing deposit',
      'postedAt': '2026-08-19T13:20:00Z',
    },
  ],
  'conversations': [
    {
      'id': 'CON-44',
      'subject': 'Manufacturing support',
      'status': 'Open',
      'lastMessageAt': '2026-09-02T10:30:00Z',
      'unreadForCustomer': 0,
      'messages': [
        {
          'id': 'MSG-1',
          'author': 'Ahsan Export Co.',
          'authorRole': 'Customer',
          'body':
              'Can you confirm whether the first batch has entered finishing?',
          'createdAt': '2026-09-02T10:22:00Z',
        },
        {
          'id': 'MSG-2',
          'author': 'Hira - Customer Care',
          'authorRole': 'Admin',
          'body': 'Yes, assembly is complete and handle finishing started this morning.',
          'createdAt': '2026-09-02T10:30:00Z',
        },
      ],
    },
  ],
  'calls': [
    {
      'id': 'CALL-18',
      'subject': 'Order completion date',
      'status': 'Waiting',
      'position': 2,
      'priority': 'Normal',
      'lastPage': 'Messages',
      'createdAt': '2026-09-02T10:31:00Z',
      'queueMessage': 'You are next after the current support call.',
    },
  ],
  'presence': [
    {
      'name': 'Hira - Customer Care',
      'role': 'Admin',
      'online': true,
      'lastOnline': '2026-09-02T10:31:00Z',
    },
  ],
  'metrics': {},
});
