import 'package:chakuchuri_mobile/src/models.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('workspace reads pagination metadata', () {
    final workspace = Workspace.fromJson({
      'quotations': List.generate(20, (index) => {'id': 'q$index'}),
      'pagination': {
        'quotations': {'loaded': 20, 'total': 45, 'hasMore': true},
      },
    });
    expect(workspace.quotations, hasLength(20));
    expect(workspace.pagination['quotations']?.loaded, 20);
    expect(workspace.pagination['quotations']?.total, 45);
    expect(workspace.pagination['quotations']?.hasMore, isTrue);
  });

  test('old API payload remains compatible', () {
    final workspace = Workspace.fromJson(const {});
    expect(workspace.pagination, isEmpty);
  });

  test('conversation reads message pagination metadata', () {
    final conversation = Conversation.fromJson({
      'id': 'conversation-1',
      'messages': List.generate(20, (index) => {'id': 'message-$index'}),
      'messagePagination': {'loaded': 20, 'total': 45, 'hasMore': true},
    });
    expect(conversation.messages, hasLength(20));
    expect(conversation.messagePagination.loaded, 20);
    expect(conversation.messagePagination.total, 45);
    expect(conversation.messagePagination.hasMore, isTrue);
  });
}
