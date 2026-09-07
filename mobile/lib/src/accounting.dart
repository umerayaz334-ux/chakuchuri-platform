import 'models.dart';

class AccountPosition {
  AccountPosition(Workspace workspace) : metrics = workspace.metrics;
  final JsonMap metrics;
  bool get ready => metrics.containsKey('ledgerBalance');
  int get balance => readInt(metrics, 'ledgerBalance');
  int get due => balance > 0 ? balance : 0;
  int get credit => balance < 0 ? -balance : 0;
  int get charges => readInt(metrics, 'totalCharges');
  int get adjustments => readInt(metrics, 'totalAdjustments');
  int get received => readInt(metrics, 'totalReceived');
  int get pending => readInt(metrics, 'pendingPaymentAmount');
}
