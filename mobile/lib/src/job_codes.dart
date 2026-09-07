import 'models.dart';

String manufacturingJobCode(ManufacturingOrder order) => order.id.trim();

String manufacturingLinkedQuoteCode(ManufacturingOrder order) {
  final quoteId = order.quotationId.trim();
  if (quoteId.isEmpty) return '';
  if (quoteId == order.id.trim()) return '';
  return quoteId;
}
