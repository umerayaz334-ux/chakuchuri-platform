import 'models.dart';

const workspaceCollections = {
  'products',
  'quotations',
  'manufacturing',
  'rateSheets',
  'shipping',
  'payments',
  'ledger',
  'conversations',
  'calls',
  'presence',
  'notices',
  'featured',
};

Workspace applyWorkspaceChanges(Workspace current, List<JsonMap> changes) {
  var next = current;
  for (final change in changes) {
    final operation = readString(change, 'operation');
    if (operation != 'upsert' && operation != 'remove') continue;
    final data = asJsonMap(change['data']);
    final scope = readString(change, 'scope');
    final key = scope == 'presence' ? 'userId' : 'id';
    if (readString(data, key).isEmpty) continue;

    List<T> update<T>(
      List<T> rows,
      T Function(JsonMap) parse,
      String Function(T) id,
    ) {
      final valueId = readString(data, key);
      if (operation == 'remove') {
        return rows.where((row) => id(row) != valueId).toList();
      }
      final value = parse(data);
      final index = rows.indexWhere((row) => id(row) == valueId);
      if (index < 0) return [value, ...rows];
      return [...rows]..[index] = value;
    }

    next = switch (scope) {
      'products' => next.copyWith(
        products: update(next.products, Product.fromJson, (row) => row.id),
      ),
      'quotations' => next.copyWith(
        quotations: update(
          next.quotations,
          Quotation.fromJson,
          (row) => row.id,
        ),
      ),
      'manufacturing' => next.copyWith(
        manufacturing: update(
          next.manufacturing,
          ManufacturingOrder.fromJson,
          (row) => row.id,
        ),
      ),
      'rateSheets' => next.copyWith(
        rateSheets: update(
          next.rateSheets,
          RateSheet.fromJson,
          (row) => row.id,
        ),
      ),
      'shipping' => next.copyWith(
        shipping: update(
          next.shipping,
          ShippingRequest.fromJson,
          (row) => row.id,
        ),
      ),
      'payments' => next.copyWith(
        payments: update(next.payments, Payment.fromJson, (row) => row.id),
      ),
      'ledger' => next.copyWith(
        ledger: update(next.ledger, LedgerEntry.fromJson, (row) => row.id),
      ),
      'conversations' => next.copyWith(
        conversations: update(
          next.conversations,
          Conversation.fromJson,
          (row) => row.id,
        ),
      ),
      'calls' => next.copyWith(
        calls: update(next.calls, CallRequest.fromJson, (row) => row.id),
      ),
      'presence' => next.copyWith(
        presence: update(
          next.presence,
          UserPresence.fromJson,
          (row) => row.userId,
        ),
      ),
      'notices' => next.copyWith(
        notices: update(next.notices, CustomerNotice.fromJson, (row) => row.id),
      ),
      'featured' => next.copyWith(
        featuredProducts: update(
          next.featuredProducts,
          FeaturedProduct.fromJson,
          (row) => row.id,
        ),
      ),
      _ => next,
    };
  }
  if (identical(next, current)) return current;
  final active = next.manufacturing
      .where((row) => row.status != 'Completed' && row.status != 'Cancelled')
      .toList();
  return next.copyWith(
    metrics: {
      ...next.metrics,
      'pendingQuotes': next.quotations
          .where((row) => row.status == 'Requested' || row.status == 'Priced')
          .length,
      'activeOrders': active.length,
      'pendingPayments': next.payments
          .where((row) => row.status == 'Waiting confirmation')
          .length,
      'openShipments': next.shipping.length,
      'ringingCalls': next.calls.where((row) => row.status == 'Ringing').length,
      'activeCalls': next.calls.where((row) => row.status == 'In call').length,
      'missedCalls': next.calls.where((row) => row.status == 'Missed').length,
      'averageProgress': active.isEmpty
          ? 0
          : active.fold<int>(0, (sum, row) => sum + row.progress) ~/
                active.length,
    },
  );
}

List<JsonMap> changesFromMutation(String path, Object? data) {
  String? scope;
  if (path == '/api/workflow/quotes' ||
      RegExp(r'^/api/workflow/quotes/[^/]+/(price|reject)$').hasMatch(path)) {
    scope = 'quotations';
  } else if (RegExp(r'^/api/workflow/quotes/[^/]+/accept$').hasMatch(path)) {
    scope = 'manufacturing';
  } else if (path == '/api/workflow/messages' ||
      RegExp(r'^/api/workflow/messages/[^/]+/read$').hasMatch(path)) {
    scope = 'conversations';
  } else if (path == '/api/workflow/calls' ||
      RegExp(r'^/api/workflow/calls/[^/]+/(status|end)$').hasMatch(path)) {
    scope = 'calls';
  } else if (path == '/api/workflow/payments') {
    scope = 'payments';
  } else if (path == '/api/workflow/shipping' ||
      path == '/api/shipping-rates/book') {
    scope = 'shipping';
  }
  return scope == null
      ? []
      : [
          {'scope': scope, 'operation': 'upsert', 'data': data},
        ];
}

bool mutationNeedsSnapshot(String path) {
  // Accepting a quote changes both the quote and the newly created order.
  // The response only contains the order, so recover the related quote once.
  return RegExp(r'^/api/workflow/quotes/[^/]+/accept$').hasMatch(path);
}
