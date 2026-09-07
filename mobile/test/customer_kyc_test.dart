import 'package:chakuchuri_mobile/src/customer_kyc.dart';
import 'package:flutter_test/flutter_test.dart';

Map<String, dynamic> customerJson({
  String verificationStage = '',
  List<Map<String, dynamic>> documents = const [],
  List<Map<String, dynamic>> requests = const [],
}) {
  return {
    'id': 'cust_test',
    'companyName': 'Test Exporter',
    'contactName': 'Test Customer',
    'email': 'customer@example.com',
    'phone': '',
    'country': 'Pakistan',
    'verificationStatus': 'unverified',
    'verificationStage': verificationStage,
    'documents': documents,
    'requestedDocuments': requests,
  };
}

Map<String, dynamic> document(
  String id,
  String kind,
  String status, {
  String requestId = '',
}) {
  return {
    'id': id,
    'kind': kind,
    'fileId': 'file_$id',
    'originalName': '$kind.jpg',
    'status': status,
    'uploadedAt': '2026-09-05T08:00:00Z',
    'requestId': requestId,
  };
}

void main() {
  test('uses backend ready_to_verify stage as a passive review state', () {
    final customer = Customer.fromJson(
      customerJson(verificationStage: 'ready_to_verify'),
    );

    expect(customerVerificationStage(customer), 'ready_to_verify');
    expect(customerNeedsIdentityAction(customer), isFalse);
    expect(customerIdentityReviewPending(customer), isTrue);
    expect(customerCanStartVerification(customer), isFalse);
  });

  test('withdrawn document request cannot leave customer waiting', () {
    final customer = Customer.fromJson(
      customerJson(
        documents: [
          document('front', 'cnic_front', 'accepted'),
          document('back', 'cnic_back', 'accepted'),
          document('selfie', 'selfie', 'accepted'),
          document('stale', 'other', 'rejected', requestId: 'ask_old'),
        ],
        requests: [
          {
            'id': 'ask_old',
            'label': 'Old request',
            'kind': 'other',
            'status': 'withdrawn',
            'createdAt': '2026-09-05T08:00:00Z',
          },
        ],
      ),
    );

    expect(customerVerificationStage(customer), 'ready_to_verify');
    expect(customerNeedsIdentityAction(customer), isFalse);
    expect(customerIdentityReviewPending(customer), isTrue);
  });

  test('action_required stage exposes the verification action', () {
    final customer = Customer.fromJson(
      customerJson(verificationStage: 'action_required'),
    );

    expect(customerNeedsIdentityAction(customer), isTrue);
    expect(customerIdentityReviewPending(customer), isFalse);
    expect(customerCanStartVerification(customer), isTrue);
  });
}
