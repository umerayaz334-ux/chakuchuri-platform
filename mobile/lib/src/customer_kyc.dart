import 'models.dart';

const kycSlots = <({String kind, String label})>[
  (kind: 'cnic_front', label: 'CNIC front'),
  (kind: 'cnic_back', label: 'CNIC back'),
  (kind: 'selfie', label: 'Selfie'),
];

const verifyIdentityPageId = 'Verify identity';

class CustomerDocument {
  const CustomerDocument({
    required this.id,
    required this.kind,
    required this.fileId,
    required this.originalName,
    required this.status,
    required this.uploadedAt,
    this.label = '',
    this.requestId = '',
  });

  factory CustomerDocument.fromJson(JsonMap json) {
    return CustomerDocument(
      id: readString(json, 'id'),
      kind: readString(json, 'kind'),
      label: readString(json, 'label'),
      fileId: readString(json, 'fileId'),
      originalName: readString(json, 'originalName'),
      status: readString(json, 'status'),
      uploadedAt: readString(json, 'uploadedAt'),
      requestId: readString(json, 'requestId'),
    );
  }

  final String id;
  final String kind;
  final String label;
  final String fileId;
  final String originalName;
  final String status;
  final String uploadedAt;
  final String requestId;

  String get normalizedStatus => status.trim().toLowerCase();
}

class RequestedDocument {
  const RequestedDocument({
    required this.id,
    required this.label,
    required this.kind,
    required this.status,
    required this.createdAt,
    this.note = '',
  });

  factory RequestedDocument.fromJson(JsonMap json) {
    return RequestedDocument(
      id: readString(json, 'id'),
      label: readString(json, 'label'),
      kind: readString(json, 'kind'),
      status: readString(json, 'status'),
      createdAt: readString(json, 'createdAt'),
      note: readString(json, 'note'),
    );
  }

  final String id;
  final String label;
  final String kind;
  final String status;
  final String createdAt;
  final String note;

  String get normalizedStatus => status.trim().toLowerCase();
}

class Customer {
  const Customer({
    required this.id,
    required this.companyName,
    required this.contactName,
    required this.email,
    required this.phone,
    required this.country,
    this.verificationStatus = 'unverified',
    this.verificationStage = '',
    this.identityNote = '',
    this.cnic = '',
    this.documents = const [],
    this.requestedDocuments = const [],
    this.documentsSubmittedAt = '',
    this.verificationInviteOpen = false,
  });

  factory Customer.fromJson(JsonMap json) {
    return Customer(
      id: readString(json, 'id'),
      companyName: readString(json, 'companyName'),
      contactName: readString(json, 'contactName'),
      email: readString(json, 'email'),
      phone: readString(json, 'phone'),
      country: readString(json, 'country'),
      verificationStatus: readString(json, 'verificationStatus', 'unverified'),
      verificationStage: readString(json, 'verificationStage'),
      identityNote: readString(json, 'identityNote'),
      cnic: readString(json, 'cnic'),
      documents: readList(json, 'documents', CustomerDocument.fromJson),
      requestedDocuments: readList(
        json,
        'requestedDocuments',
        RequestedDocument.fromJson,
      ),
      documentsSubmittedAt: readString(json, 'documentsSubmittedAt'),
      verificationInviteOpen: json['verificationInviteOpen'] == true,
    );
  }

  final String id;
  final String companyName;
  final String contactName;
  final String email;
  final String phone;
  final String country;
  final String verificationStatus;
  final String verificationStage;
  final String identityNote;
  final String cnic;
  final List<CustomerDocument> documents;
  final List<RequestedDocument> requestedDocuments;
  final String documentsSubmittedAt;
  final bool verificationInviteOpen;
}

const _verificationStages = {
  'not_started',
  'action_required',
  'in_review',
  'ready_to_verify',
  'verified',
};

const _inactiveRequestStatuses = {'withdrawn', 'cancelled', 'canceled'};

bool isCustomerVerified(String? status) =>
    (status ?? '').trim().toLowerCase() == 'verified';

String verificationLabel(String? status) =>
    isCustomerVerified(status) ? 'Verified' : 'Not verified';

bool isActiveDocumentRequest(RequestedDocument request) =>
    !_inactiveRequestStatuses.contains(request.normalizedStatus);

String customerVerificationStage(Customer? customer) {
  if (customer == null) return 'not_started';
  if (isCustomerVerified(customer.verificationStatus)) return 'verified';

  final reported = customer.verificationStage.trim().toLowerCase();
  if (_verificationStages.contains(reported)) return reported;

  final activeRequests = customer.requestedDocuments
      .where(isActiveDocumentRequest)
      .toList(growable: false);
  final activeRequestIds = activeRequests.map((request) => request.id).toSet();
  final documents = customer.documents.where(
    (document) =>
        document.requestId.isEmpty ||
        activeRequestIds.contains(document.requestId),
  );
  final byRequest = <String, CustomerDocument>{
    for (final document in documents)
      if (document.requestId.isNotEmpty) document.requestId: document,
  };

  if (documents.any(slotNeedsAction)) return 'action_required';
  if (activeRequests.any((request) => slotNeedsAction(byRequest[request.id]))) {
    return 'action_required';
  }
  if (customer.verificationInviteOpen) return 'action_required';
  if (documents.any((document) {
    final status = document.normalizedStatus;
    return status == 'submitted' || status == 'uploaded';
  })) {
    return 'in_review';
  }

  final byKind = <String, CustomerDocument>{
    for (final document in documents)
      if (document.kind != 'other') document.kind: document,
  };
  final requiredAccepted = kycSlots.every(
    (slot) => byKind[slot.kind]?.normalizedStatus == 'accepted',
  );
  final requestsAccepted = activeRequests.every(
    (request) => byRequest[request.id]?.normalizedStatus == 'accepted',
  );
  if (requiredAccepted && requestsAccepted) return 'ready_to_verify';
  if (customer.documentsSubmittedAt.isNotEmpty ||
      documents.any((document) => document.normalizedStatus == 'accepted')) {
    return 'in_review';
  }
  return 'not_started';
}

bool customerNeedsIdentityAction(Customer? customer) =>
    customerVerificationStage(customer) == 'action_required';

bool customerIdentityReviewPending(Customer? customer) {
  final stage = customerVerificationStage(customer);
  return stage == 'in_review' || stage == 'ready_to_verify';
}

bool customerCanStartVerification(Customer? customer) {
  final stage = customerVerificationStage(customer);
  return stage == 'not_started' || stage == 'action_required';
}

bool isEditableDoc(CustomerDocument? doc) {
  if (doc == null) return true;
  final status = doc.normalizedStatus;
  return status == 'draft' || status == 'rejected' || status == 'requested';
}

bool slotNeedsAction(CustomerDocument? doc) {
  if (doc == null) return true;
  final status = doc.normalizedStatus;
  return status == 'draft' || status == 'rejected' || status == 'requested';
}
