import 'models.dart';

String? customerMessageNotificationId(Conversation conversation) {
  if (conversation.unreadForCustomer <= 0) return null;
  Message? latest;
  for (final message in conversation.messages) {
    if (message.authorRole.trim().toLowerCase() == 'customer' || message.id.isEmpty) continue;
    final time = DateTime.tryParse(message.createdAt);
    final previous = DateTime.tryParse(latest?.createdAt ?? '');
    if (latest == null || time == null || previous == null || !time.isBefore(previous)) latest = message;
  }
  if (latest == null) return null;
  return 'chat-${conversation.id}-${latest.id}';
}

String? callNotificationId(CallRequest call, String userId) {
  if (call.status == 'Ringing' && call.initiatorUserId == userId) return null;
  if (call.status != 'Ringing' && call.status != 'In call') return null;
  return 'call-${call.id}-${call.status}';
}
