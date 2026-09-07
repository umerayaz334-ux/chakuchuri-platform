typedef JsonMap = Map<String, dynamic>;

JsonMap asJsonMap(Object? value) {
  if (value is Map<String, dynamic>) return value;
  if (value is Map) return Map<String, dynamic>.from(value);
  return <String, dynamic>{};
}

String readString(JsonMap json, String key, [String fallback = '']) {
  final value = json[key];
  return value == null ? fallback : value.toString();
}

int readInt(JsonMap json, String key, [int fallback = 0]) {
  final value = json[key];
  if (value is int) return value;
  if (value is num) return value.round();
  return int.tryParse(value?.toString() ?? '') ?? fallback;
}

List<T> readList<T>(JsonMap json, String key, T Function(JsonMap) parser) {
  final value = json[key];
  if (value is! List) return <T>[];
  return value
      .whereType<Map>()
      .map((item) => parser(Map<String, dynamic>.from(item)))
      .toList();
}

List<String> readStringList(JsonMap json, String key) {
  final value = json[key];
  if (value is! List) return const [];
  return value
      .map((item) => item?.toString().trim() ?? '')
      .where((item) => item.isNotEmpty)
      .toList();
}

List<String> mergeImageFileIds({
  String imageFileId = '',
  List<String> imageFileIds = const [],
}) {
  final ids = <String>[];
  for (final id in imageFileIds) {
    final trimmed = id.trim();
    if (trimmed.isNotEmpty && !ids.contains(trimmed)) ids.add(trimmed);
  }
  final primary = imageFileId.trim();
  if (primary.isNotEmpty && !ids.contains(primary)) {
    ids.insert(0, primary);
  }
  return ids;
}

class ApiEnvelope<T> {
  const ApiEnvelope({
    required this.ok,
    required this.requestId,
    this.data,
    this.message,
  });

  final bool ok;
  final String requestId;
  final T? data;
  final String? message;
}

class User {
  const User({
    required this.id,
    required this.name,
    required this.email,
    required this.role,
    required this.status,
    required this.permissions,
    required this.pageAccess,
    this.customerId = '',
    this.lastOnline = '',
    this.profileImageFileId = '',
    this.loginActivity = const [],
    this.suspensionNotice = '',
    this.suspensionContactAt = '',
    this.suspensionContactMessage = '',
  });

  factory User.fromJson(JsonMap json) {
    return User(
      id: readString(json, 'id'),
      name: readString(json, 'name'),
      email: readString(json, 'email'),
      role: readString(json, 'role'),
      status: readString(json, 'status'),
      customerId: readString(json, 'customerId'),
      lastOnline: readString(json, 'lastOnline'),
      profileImageFileId: readString(json, 'profileImageFileId'),
      permissions: (json['permissions'] as List? ?? const [])
          .map((item) => item.toString())
          .toList(),
      pageAccess: (json['pageAccess'] as List? ?? const [])
          .map((item) => item.toString())
          .toList(),
      loginActivity: readList(json, 'loginActivity', LoginActivity.fromJson),
      suspensionNotice: readString(json, 'suspensionNotice'),
      suspensionContactAt: readString(json, 'suspensionContactAt'),
      suspensionContactMessage: readString(json, 'suspensionContactMessage'),
    );
  }

  final String id;
  final String name;
  final String email;
  final String role;
  final String status;
  final String customerId;
  final String lastOnline;
  final String profileImageFileId;
  final List<String> permissions;
  final List<String> pageAccess;
  final List<LoginActivity> loginActivity;
  final String suspensionNotice;
  final String suspensionContactAt;
  final String suspensionContactMessage;
  bool get isSuspended =>
      status.trim().toLowerCase() == 'temporarily suspended';

  JsonMap toJson() => {
    'id': id,
    'name': name,
    'email': email,
    'role': role,
    'status': status,
    'customerId': customerId,
    'lastOnline': lastOnline,
    'profileImageFileId': profileImageFileId,
    'permissions': permissions,
    'pageAccess': pageAccess,
    'loginActivity': loginActivity.map((item) => item.toJson()).toList(),
    'suspensionNotice': suspensionNotice,
    'suspensionContactAt': suspensionContactAt,
    'suspensionContactMessage': suspensionContactMessage,
  };

  User copyWith({
    String? name,
    String? email,
    String? role,
    String? status,
    String? customerId,
    String? lastOnline,
    String? profileImageFileId,
    List<String>? permissions,
    List<String>? pageAccess,
    List<LoginActivity>? loginActivity,
  }) {
    return User(
      id: id,
      name: name ?? this.name,
      email: email ?? this.email,
      role: role ?? this.role,
      status: status ?? this.status,
      customerId: customerId ?? this.customerId,
      lastOnline: lastOnline ?? this.lastOnline,
      profileImageFileId: profileImageFileId ?? this.profileImageFileId,
      permissions: permissions ?? this.permissions,
      pageAccess: pageAccess ?? this.pageAccess,
      loginActivity: loginActivity ?? this.loginActivity,
      suspensionNotice: suspensionNotice,
      suspensionContactAt: suspensionContactAt,
      suspensionContactMessage: suspensionContactMessage,
    );
  }
}

class LoginActivity {
  const LoginActivity({
    required this.at,
    required this.result,
    this.ip = '',
    this.userAgent = '',
    this.detail = '',
  });

  factory LoginActivity.fromJson(JsonMap json) {
    return LoginActivity(
      at: readString(json, 'at'),
      result: readString(json, 'result'),
      ip: readString(json, 'ip'),
      userAgent: readString(json, 'userAgent'),
      detail: readString(json, 'detail'),
    );
  }

  final String at;
  final String result;
  final String ip;
  final String userAgent;
  final String detail;

  JsonMap toJson() => {
    'at': at,
    'result': result,
    'ip': ip,
    'userAgent': userAgent,
    'detail': detail,
  };
}

class Session {
  const Session({
    required this.token,
    required this.user,
    required this.expiresAt,
  });

  factory Session.fromJson(JsonMap json) {
    return Session(
      token: readString(json, 'token'),
      user: User.fromJson(
        Map<String, dynamic>.from(json['user'] as Map? ?? const {}),
      ),
      expiresAt: readString(json, 'expiresAt'),
    );
  }

  final String token;
  final User user;
  final String expiresAt;

  JsonMap toJson() => {
    'token': token,
    'user': user.toJson(),
    'expiresAt': expiresAt,
  };

  Session copyWith({User? user, String? expiresAt}) {
    return Session(
      token: token,
      user: user ?? this.user,
      expiresAt: expiresAt ?? this.expiresAt,
    );
  }
}

class Product {
  const Product({
    required this.id,
    required this.sku,
    required this.name,
    required this.stock,
    required this.reserved,
    this.image = '',
  });

  factory Product.fromJson(JsonMap json) {
    return Product(
      id: readString(json, 'id'),
      sku: readString(json, 'sku'),
      name: readString(json, 'name'),
      stock: readInt(json, 'stock'),
      reserved: readInt(json, 'reserved'),
      image: readString(json, 'image'),
    );
  }

  final String id;
  final String sku;
  final String name;
  final int stock;
  final int reserved;
  final String image;
}

class Update {
  const Update({
    required this.label,
    required this.detail,
    required this.actor,
    required this.createdAt,
  });

  factory Update.fromJson(JsonMap json) {
    return Update(
      label: readString(json, 'label'),
      detail: readString(json, 'detail'),
      actor: readString(json, 'actor'),
      createdAt: readString(json, 'createdAt'),
    );
  }

  final String label;
  final String detail;
  final String actor;
  final String createdAt;
}

class Quotation {
  const Quotation({
    required this.id,
    required this.productName,
    required this.quantity,
    required this.status,
    required this.totalAmount,
    required this.depositRequired,
    required this.expectedDate,
    required this.notes,
    required this.history,
    this.imageName = '',
    this.imageFileId = '',
    this.imageNames = const [],
    this.imageFileIds = const [],
  });

  factory Quotation.fromJson(JsonMap json) {
    return Quotation(
      id: readString(json, 'id'),
      productName: readString(json, 'productName'),
      quantity: readInt(json, 'quantity'),
      status: readString(json, 'status'),
      totalAmount: readInt(json, 'totalAmount'),
      depositRequired: readInt(json, 'depositRequired'),
      expectedDate: readString(json, 'expectedDate'),
      notes: readString(json, 'notes'),
      history: readList(json, 'history', Update.fromJson),
      imageName: readString(json, 'imageName'),
      imageFileId: readString(json, 'imageFileId'),
      imageNames: readStringList(json, 'imageNames'),
      imageFileIds: readStringList(json, 'imageFileIds'),
    );
  }

  final String id;
  final String productName;
  final int quantity;
  final String status;
  final int totalAmount;
  final int depositRequired;
  final String expectedDate;
  final String notes;
  final List<Update> history;
  final String imageName;
  final String imageFileId;
  final List<String> imageNames;
  final List<String> imageFileIds;

  List<String> get resolvedImageFileIds =>
      mergeImageFileIds(imageFileId: imageFileId, imageFileIds: imageFileIds);

  List<String> get resolvedImageNames {
    final names = <String>[...imageNames];
    if (imageName.trim().isNotEmpty && names.isEmpty) {
      return [imageName.trim()];
    }
    return names;
  }
}

class Stage {
  const Stage({required this.name, required this.status, this.date = ''});

  factory Stage.fromJson(JsonMap json) {
    return Stage(
      name: readString(json, 'name'),
      status: readString(json, 'status'),
      date: readString(json, 'date'),
    );
  }

  final String name;
  final String status;
  final String date;
}

class ManufacturingOrder {
  const ManufacturingOrder({
    required this.id,
    required this.productName,
    required this.quantity,
    required this.status,
    required this.currentStage,
    required this.progress,
    required this.expectedDate,
    required this.totalAmount,
    required this.paidAmount,
    required this.balanceDue,
    required this.stages,
    required this.history,
    this.quotationId = '',
    this.imageName = '',
    this.imageFileId = '',
    this.imageNames = const [],
    this.imageFileIds = const [],
  });

  factory ManufacturingOrder.fromJson(JsonMap json) {
    return ManufacturingOrder(
      id: readString(json, 'id'),
      productName: readString(json, 'productName'),
      quantity: readInt(json, 'quantity'),
      status: readString(json, 'status'),
      currentStage: readString(json, 'currentStage'),
      progress: readInt(json, 'progress'),
      expectedDate: readString(json, 'expectedDate'),
      totalAmount: readInt(json, 'totalAmount'),
      paidAmount: readInt(json, 'paidAmount'),
      balanceDue: readInt(json, 'balanceDue'),
      stages: readList(json, 'stages', Stage.fromJson),
      history: readList(json, 'history', Update.fromJson),
      quotationId: readString(json, 'quotationId'),
      imageName: readString(json, 'imageName'),
      imageFileId: readString(json, 'imageFileId'),
      imageNames: readStringList(json, 'imageNames'),
      imageFileIds: readStringList(json, 'imageFileIds'),
    );
  }

  final String id;
  final String productName;
  final int quantity;
  final String status;
  final String currentStage;
  final int progress;
  final String expectedDate;
  final int totalAmount;
  final int paidAmount;
  final int balanceDue;
  final List<Stage> stages;
  final List<Update> history;
  final String quotationId;
  final String imageName;
  final String imageFileId;
  final List<String> imageNames;
  final List<String> imageFileIds;

  List<String> get resolvedImageFileIds =>
      mergeImageFileIds(imageFileId: imageFileId, imageFileIds: imageFileIds);

  List<String> get resolvedImageNames {
    final names = <String>[...imageNames];
    if (imageName.trim().isNotEmpty && names.isEmpty) {
      return [imageName.trim()];
    }
    return names;
  }
}

class RateSheet {
  const RateSheet({
    required this.id,
    required this.courier,
    required this.service,
    required this.zone,
    required this.weight,
    required this.price,
    required this.status,
  });

  factory RateSheet.fromJson(JsonMap json) {
    return RateSheet(
      id: readString(json, 'id'),
      courier: readString(json, 'courier'),
      service: readString(json, 'service'),
      zone: readString(json, 'zone'),
      weight: readString(json, 'weight'),
      price: readInt(json, 'price'),
      status: readString(json, 'status'),
    );
  }

  final String id;
  final String courier;
  final String service;
  final String zone;
  final String weight;
  final int price;
  final String status;
}

class ShippingRequest {
  const ShippingRequest({
    required this.id,
    required this.type,
    required this.courier,
    required this.service,
    required this.destination,
    required this.zone,
    required this.weight,
    required this.status,
    required this.quotedAmount,
    required this.createdAt,
    this.tracking = '',
  });

  factory ShippingRequest.fromJson(JsonMap json) {
    return ShippingRequest(
      id: readString(json, 'id'),
      type: readString(json, 'type'),
      courier: readString(json, 'courier'),
      service: readString(json, 'service'),
      destination: readString(json, 'destination'),
      zone: readString(json, 'zone'),
      weight: readString(json, 'weight'),
      status: readString(json, 'status'),
      tracking: readString(json, 'tracking'),
      quotedAmount: readInt(json, 'quotedAmount'),
      createdAt: readString(json, 'createdAt'),
    );
  }

  final String id;
  final String type;
  final String courier;
  final String service;
  final String destination;
  final String zone;
  final String weight;
  final String status;
  final String tracking;
  final int quotedAmount;
  final String createdAt;
}

class Payment {
  const Payment({
    required this.id,
    required this.type,
    required this.amount,
    required this.status,
    required this.proofName,
    required this.createdAt,
    this.manufacturingId = '',
    this.shippingId = '',
  });

  factory Payment.fromJson(JsonMap json) {
    return Payment(
      id: readString(json, 'id'),
      type: readString(json, 'type'),
      amount: readInt(json, 'amount'),
      status: readString(json, 'status'),
      proofName: readString(json, 'proofName'),
      manufacturingId: readString(json, 'manufacturingId'),
      shippingId: readString(json, 'shippingId'),
      createdAt: readString(json, 'createdAt'),
    );
  }

  final String id;
  final String type;
  final int amount;
  final String status;
  final String proofName;
  final String manufacturingId;
  final String shippingId;
  final String createdAt;
}

class LedgerEntry {
  const LedgerEntry({
    required this.id,
    required this.sourceType,
    required this.debit,
    required this.credit,
    required this.note,
    required this.postedAt,
  });

  factory LedgerEntry.fromJson(JsonMap json) {
    return LedgerEntry(
      id: readString(json, 'id'),
      sourceType: readString(json, 'sourceType'),
      debit: readInt(json, 'debit'),
      credit: readInt(json, 'credit'),
      note: readString(json, 'note'),
      postedAt: readString(json, 'postedAt'),
    );
  }

  final String id;
  final String sourceType;
  final int debit;
  final int credit;
  final String note;
  final String postedAt;
}

class MessageAttachment {
  const MessageAttachment({
    required this.fileId,
    required this.name,
    required this.mimeType,
    required this.byteSize,
    this.url = '',
    this.thumbnailUrl = '',
  });

  factory MessageAttachment.fromJson(JsonMap json) {
    return MessageAttachment(
      fileId: readString(json, 'fileId'),
      name: readString(json, 'name'),
      mimeType: readString(json, 'mimeType'),
      byteSize: readInt(json, 'byteSize'),
      url: readString(json, 'url'),
      thumbnailUrl: readString(json, 'thumbnailUrl'),
    );
  }

  final String fileId;
  final String name;
  final String mimeType;
  final int byteSize;
  final String url;
  final String thumbnailUrl;

  bool get isImage => mimeType.toLowerCase().startsWith('image/');
}

class Message {
  const Message({
    required this.id,
    required this.author,
    required this.authorRole,
    required this.body,
    required this.createdAt,
    this.attachments = const [],
  });

  factory Message.fromJson(JsonMap json) {
    return Message(
      id: readString(json, 'id'),
      author: readString(json, 'author'),
      authorRole: readString(json, 'authorRole'),
      body: readString(json, 'body'),
      createdAt: readString(json, 'createdAt'),
      attachments: readList(json, 'attachments', MessageAttachment.fromJson),
    );
  }

  final String id;
  final String author;
  final String authorRole;
  final String body;
  final String createdAt;
  final List<MessageAttachment> attachments;
}

class Conversation {
  const Conversation({
    required this.id,
    required this.subject,
    required this.status,
    required this.lastMessageAt,
    required this.unreadForCustomer,
    required this.messages,
    this.customerId = '',
    this.unreadForAdmin = 0,
    this.messagePagination = const PageInfo(
      loaded: 0,
      total: 0,
      hasMore: false,
    ),
  });

  factory Conversation.fromJson(JsonMap json) {
    return Conversation(
      id: readString(json, 'id'),
      customerId: readString(json, 'customerId'),
      subject: readString(json, 'subject', 'Support conversation'),
      status: readString(json, 'status'),
      lastMessageAt: readString(json, 'lastMessageAt'),
      unreadForCustomer: readInt(json, 'unreadForCustomer'),
      unreadForAdmin: readInt(json, 'unreadForAdmin'),
      messages: readList(json, 'messages', Message.fromJson),
      messagePagination: PageInfo.fromJson(
        Map<String, dynamic>.from(
          json['messagePagination'] as Map? ?? const {},
        ),
      ),
    );
  }

  final String id;
  final String customerId;
  final String subject;
  final String status;
  final String lastMessageAt;
  final int unreadForCustomer;
  final int unreadForAdmin;
  final List<Message> messages;
  final PageInfo messagePagination;
}

class ConversationMessagePage {
  const ConversationMessagePage({
    required this.conversationId,
    required this.messages,
    required this.pagination,
  });

  factory ConversationMessagePage.fromJson(JsonMap json) =>
      ConversationMessagePage(
        conversationId: readString(json, 'conversationId'),
        messages: readList(json, 'messages', Message.fromJson),
        pagination: PageInfo.fromJson(
          Map<String, dynamic>.from(json['pagination'] as Map? ?? const {}),
        ),
      );

  final String conversationId;
  final List<Message> messages;
  final PageInfo pagination;
}

class CallRequest {
  const CallRequest({
    required this.id,
    required this.customerId,
    required this.conversationId,
    required this.subject,
    required this.status,
    required this.callType,
    required this.roomId,
    required this.initiatorUserId,
    required this.initiatorName,
    required this.initiatorRole,
    required this.recipientName,
    required this.answeredByUserId,
    required this.answeredBy,
    required this.createdAt,
    required this.updatedAt,
    required this.ringExpiresAt,
    required this.startedAt,
    required this.endedAt,
    required this.durationSeconds,
    required this.customerLastSeenAt,
    required this.adminLastSeenAt,
    required this.history,
  });

  factory CallRequest.fromJson(JsonMap json) {
    return CallRequest(
      id: readString(json, 'id'),
      customerId: readString(json, 'customerId'),
      conversationId: readString(json, 'conversationId'),
      subject: readString(json, 'subject', 'Audio call'),
      status: readString(json, 'status'),
      callType: readString(json, 'callType', 'audio'),
      roomId: readString(json, 'roomId'),
      initiatorUserId: readString(json, 'initiatorUserId'),
      initiatorName: readString(json, 'initiatorName'),
      initiatorRole: readString(json, 'initiatorRole'),
      recipientName: readString(json, 'recipientName'),
      answeredByUserId: readString(json, 'answeredByUserId'),
      answeredBy: readString(json, 'answeredBy'),
      createdAt: readString(json, 'createdAt'),
      updatedAt: readString(json, 'updatedAt'),
      ringExpiresAt: readString(json, 'ringExpiresAt'),
      startedAt: readString(json, 'startedAt'),
      endedAt: readString(json, 'endedAt'),
      durationSeconds: readInt(json, 'durationSeconds'),
      customerLastSeenAt: readString(json, 'customerLastSeenAt'),
      adminLastSeenAt: readString(json, 'adminLastSeenAt'),
      history: readList(json, 'history', Update.fromJson),
    );
  }

  final String id;
  final String customerId;
  final String conversationId;
  final String subject;
  final String status;
  final String callType;
  final String roomId;
  final String initiatorUserId;
  final String initiatorName;
  final String initiatorRole;
  final String recipientName;
  final String answeredByUserId;
  final String answeredBy;
  final String createdAt;
  final String updatedAt;
  final String ringExpiresAt;
  final String startedAt;
  final String endedAt;
  final int durationSeconds;
  final String customerLastSeenAt;
  final String adminLastSeenAt;
  final List<Update> history;
}

class CallSignal {
  const CallSignal({
    required this.id,
    required this.callId,
    required this.signalNo,
    required this.senderId,
    required this.senderRole,
    required this.signalType,
    required this.payload,
    required this.createdAt,
  });

  factory CallSignal.fromJson(JsonMap json) {
    return CallSignal(
      id: readString(json, 'id'),
      callId: readString(json, 'callId'),
      signalNo: readInt(json, 'signalNo'),
      senderId: readString(json, 'senderId'),
      senderRole: readString(json, 'senderRole'),
      signalType: readString(json, 'signalType'),
      payload: asJsonMap(json['payload']),
      createdAt: readString(json, 'createdAt'),
    );
  }

  final String id;
  final String callId;
  final int signalNo;
  final String senderId;
  final String senderRole;
  final String signalType;
  final JsonMap payload;
  final String createdAt;
}

class UserPresence {
  const UserPresence({
    this.userId = '',
    required this.name,
    required this.role,
    required this.online,
    required this.lastOnline,
  });

  factory UserPresence.fromJson(JsonMap json) {
    return UserPresence(
      userId: readString(json, 'userId'),
      name: readString(json, 'name'),
      role: readString(json, 'role'),
      online: json['online'] == true,
      lastOnline: readString(json, 'lastOnline'),
    );
  }

  final String userId;
  final String name;
  final String role;
  final bool online;
  final String lastOnline;
}

class CustomerNotice {
  const CustomerNotice({
    required this.id,
    required this.title,
    required this.body,
    required this.tone,
    required this.audience,
    required this.active,
    this.customerIds = const [],
    this.createdAt = '',
    this.updatedAt = '',
    this.createdBy = '',
  });

  factory CustomerNotice.fromJson(JsonMap json) {
    return CustomerNotice(
      id: readString(json, 'id'),
      title: readString(json, 'title'),
      body: readString(json, 'body'),
      tone: readString(json, 'tone', 'info'),
      audience: readString(json, 'audience', 'all'),
      customerIds: readStringList(json, 'customerIds'),
      active: json['active'] != false,
      createdAt: readString(json, 'createdAt'),
      updatedAt: readString(json, 'updatedAt'),
      createdBy: readString(json, 'createdBy'),
    );
  }

  final String id;
  final String title;
  final String body;
  final String tone;
  final String audience;
  final List<String> customerIds;
  final bool active;
  final String createdAt;
  final String updatedAt;
  final String createdBy;
}

class FeaturedProduct {
  const FeaturedProduct({
    required this.id,
    required this.name,
    required this.tag,
    required this.imageFileId,
    this.caption = '',
    this.imageName = '',
    this.productId = '',
    this.sortOrder = 0,
    this.active = true,
    this.createdAt = '',
    this.updatedAt = '',
    this.createdBy = '',
  });

  factory FeaturedProduct.fromJson(JsonMap json) {
    return FeaturedProduct(
      id: readString(json, 'id'),
      name: readString(json, 'name'),
      tag: readString(json, 'tag', 'Featured'),
      caption: readString(json, 'caption'),
      imageFileId: readString(json, 'imageFileId'),
      imageName: readString(json, 'imageName'),
      productId: readString(json, 'productId'),
      sortOrder: readInt(json, 'sortOrder'),
      active: json['active'] != false,
      createdAt: readString(json, 'createdAt'),
      updatedAt: readString(json, 'updatedAt'),
      createdBy: readString(json, 'createdBy'),
    );
  }

  final String id;
  final String name;
  final String tag;
  final String caption;
  final String imageFileId;
  final String imageName;
  final String productId;
  final int sortOrder;
  final bool active;
  final String createdAt;
  final String updatedAt;
  final String createdBy;
}

class Workspace {
  const Workspace({
    required this.products,
    required this.quotations,
    required this.manufacturing,
    required this.rateSheets,
    required this.shipping,
    required this.payments,
    required this.ledger,
    required this.conversations,
    required this.calls,
    required this.presence,
    required this.metrics,
    this.notices = const [],
    this.featuredProducts = const [],
    this.pagination = const {},
  });

  factory Workspace.empty() {
    return const Workspace(
      products: [],
      quotations: [],
      manufacturing: [],
      rateSheets: [],
      shipping: [],
      payments: [],
      ledger: [],
      conversations: [],
      calls: [],
      presence: [],
      metrics: {},
      notices: [],
      featuredProducts: [],
      pagination: {},
    );
  }

  factory Workspace.fromJson(JsonMap json) {
    return Workspace(
      products: readList(json, 'products', Product.fromJson),
      quotations: readList(json, 'quotations', Quotation.fromJson),
      manufacturing: readList(
        json,
        'manufacturing',
        ManufacturingOrder.fromJson,
      ),
      rateSheets: readList(json, 'rateSheets', RateSheet.fromJson),
      shipping: readList(json, 'shipping', ShippingRequest.fromJson),
      payments: readList(json, 'payments', Payment.fromJson),
      ledger: readList(json, 'ledger', LedgerEntry.fromJson),
      conversations: readList(json, 'conversations', Conversation.fromJson),
      calls: readList(json, 'calls', CallRequest.fromJson),
      presence: readList(json, 'presence', UserPresence.fromJson),
      notices: readList(json, 'notices', CustomerNotice.fromJson),
      featuredProducts: readList(
        json,
        'featuredProducts',
        FeaturedProduct.fromJson,
      ),
      metrics: Map<String, dynamic>.from(json['metrics'] as Map? ?? const {}),
      pagination: (json['pagination'] as Map? ?? const {}).map(
        (key, value) => MapEntry('$key', PageInfo.fromJson(asJsonMap(value))),
      ),
    );
  }

  final List<Product> products;
  final List<Quotation> quotations;
  final List<ManufacturingOrder> manufacturing;
  final List<RateSheet> rateSheets;
  final List<ShippingRequest> shipping;
  final List<Payment> payments;
  final List<LedgerEntry> ledger;
  final List<Conversation> conversations;
  final List<CallRequest> calls;
  final List<UserPresence> presence;
  final List<CustomerNotice> notices;
  final List<FeaturedProduct> featuredProducts;
  final JsonMap metrics;
  final Map<String, PageInfo> pagination;

  Workspace copyWith({
    List<Product>? products,
    List<Quotation>? quotations,
    List<ManufacturingOrder>? manufacturing,
    List<RateSheet>? rateSheets,
    List<ShippingRequest>? shipping,
    List<Payment>? payments,
    List<LedgerEntry>? ledger,
    List<Conversation>? conversations,
    List<CallRequest>? calls,
    List<UserPresence>? presence,
    List<CustomerNotice>? notices,
    List<FeaturedProduct>? featuredProducts,
    JsonMap? metrics,
    Map<String, PageInfo>? pagination,
  }) => Workspace(
    products: products ?? this.products,
    quotations: quotations ?? this.quotations,
    manufacturing: manufacturing ?? this.manufacturing,
    rateSheets: rateSheets ?? this.rateSheets,
    shipping: shipping ?? this.shipping,
    payments: payments ?? this.payments,
    ledger: ledger ?? this.ledger,
    conversations: conversations ?? this.conversations,
    calls: calls ?? this.calls,
    presence: presence ?? this.presence,
    notices: notices ?? this.notices,
    featuredProducts: featuredProducts ?? this.featuredProducts,
    metrics: metrics ?? this.metrics,
    pagination: pagination ?? this.pagination,
  );
}

class PageInfo {
  const PageInfo({
    required this.loaded,
    required this.total,
    required this.hasMore,
  });

  factory PageInfo.fromJson(JsonMap json) => PageInfo(
    loaded: readInt(json, 'loaded'),
    total: readInt(json, 'total'),
    hasMore: json['hasMore'] == true,
  );

  final int loaded;
  final int total;
  final bool hasMore;
}
