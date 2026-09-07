import 'package:chakuchuri_mobile/src/models.dart';
import 'package:chakuchuri_mobile/src/notification_keys.dart';
import 'package:flutter_test/flutter_test.dart';

Conversation chat(String message, {int unread = 1, bool ownReply = false}) => Conversation.fromJson({
  'id': 'conversation', 'unreadForCustomer': unread,
  'messages': [
    {'id': message, 'authorRole': 'Admin', 'createdAt': '2026-09-05T12:00:00Z'},
    if (ownReply) {'id': 'own', 'authorRole': 'Customer', 'createdAt': '2026-09-05T12:01:00Z'},
  ],
});

void main() {
  test('new message with repeated unread count has a new notification identity', () {
    expect(customerMessageNotificationId(chat('one')), isNot(customerMessageNotificationId(chat('two'))));
    expect(customerMessageNotificationId(chat('one', unread: 0)), isNull);
    expect(customerMessageNotificationId(chat('one', ownReply: true)), customerMessageNotificationId(chat('one')));
  });

  test('call heartbeat changes do not create duplicate incoming call alerts', () {
    final call = {'id': 'c1', 'status': 'Ringing', 'initiatorUserId': 'admin', 'updatedAt': 'one'};
    final first = callNotificationId(CallRequest.fromJson(call), 'customer');
    call['updatedAt'] = 'two';
    expect(callNotificationId(CallRequest.fromJson(call), 'customer'), first);
    expect(callNotificationId(CallRequest.fromJson(call), 'admin'), isNull);
    call['status'] = 'Ended';
    expect(callNotificationId(CallRequest.fromJson(call), 'customer'), isNull);
  });
}
