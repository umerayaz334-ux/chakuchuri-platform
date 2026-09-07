export type Metric = { label: string; value: string; delta: string };

export type LoginActivity = {
  at: string;
  result: string;
  ip: string;
  userAgent: string;
  detail: string;
};

export type KnownIP = {
  ip: string;
  firstSeenAt: string;
  lastSeenAt: string;
  successCount: number;
  failCount?: number;
};

export type User = {
  id: string;
  tenantId: string;
  customerId?: string;
  assignedCustomerIds?: string[];
  name: string;
  email: string;
  role: string;
  status: string;
  permissions: string[];
  pageAccess: string[];
  lastLogin: string;
  lastOnline: string;
  failedLoginCount: number;
  loginActivity?: LoginActivity[];
  knownIps?: KnownIP[];
  profileImageFileId?: string;
  suspensionNotice?: string;
  suspensionContactAt?: string;
  suspensionContactMessage?: string;
  notificationReadIds?: string[];
};

export type AccessOptions = {
  roles: string[];
  pages: string[];
  permissions: string[];
  roleDefaults: Record<string, string[]>;
  pageAccessDefaults: Record<string, string[]>;
  statuses: string[];
};

export type Session = { token: string; user: User; expiresAt: string };
export type FileRecord = {
  id: string;
  tenantId: string;
  customerId?: string;
  ownerType: string;
  ownerId?: string;
  originalName: string;
  storageKey: string;
  url: string;
  thumbnailKey?: string;
  thumbnailUrl?: string;
  mimeType: string;
  byteSize: number;
  width?: number;
  height?: number;
  checksum: string;
  createdAt: string;
};

export type CustomerDocument = {
  id: string;
  kind: string;
  label?: string;
  fileId: string;
  originalName: string;
  status: string;
  uploadedAt: string;
  uploadedByUserId?: string;
  requestId?: string;
  documentRequestId?: string;
};

export type RequestedDocument = {
  id: string;
  label: string;
  kind: string;
  status: string;
  createdAt: string;
  note?: string;
};

export type CustomerVerificationStage = "not_started" | "action_required" | "in_review" | "ready_to_verify" | "verified";

export type Customer = {
  id: string;
  companyName: string;
  contactName: string;
  email: string;
  phone: string;
  country: string;
  services: string[];
  online: boolean;
  lastOnline: string;
  balanceDue: string;
  openOrders: number;
  openShipments: number;
  verificationStatus?: string;
  verifiedAt?: string;
  verifiedByUserId?: string;
  identityNote?: string;
  cnic?: string;
  documents?: CustomerDocument[];
  requestedDocuments?: RequestedDocument[];
  documentsSubmittedAt?: string;
  verificationInviteOpen?: boolean;
  verificationStage?: CustomerVerificationStage;
};

export type Module = {
  name: string;
  owner: string;
  purpose: string;
  status: string;
  dataRules: string[];
};

export type Foundation = {
  phase: string;
  principles: string[];
  modules: Module[];
};

export type Blueprint = {
  productName: string;
  backendStatus: string;
  customerMetrics: Metric[];
  adminMetrics: Metric[];
  orders: unknown[];
  shipping: unknown[];
  calls: unknown[];
};

export type Product = {
  id: string;
  customerId: string;
  sku: string;
  name: string;
  stock: number;
  reserved: number;
  image: string;
  imageFileId?: string;
  updatedAt: string;
};

export type Update = {
  label: string;
  detail: string;
  actor: string;
  createdAt: string;
};

export type Quotation = {
  id: string;
  customerId: string;
  productId?: string;
  productName: string;
  quantity: number;
  imageName?: string;
  imageFileId?: string;
  imageNames?: string[];
  imageFileIds?: string[];
  notes: string;
  steel: string;
  tang: string;
  bladeThickness: string;
  handleMaterial: string;
  sheath: string;
  finish: string;
  status: string;
  totalAmount: number;
  depositRequired: number;
  expectedDate: string;
  adminNote: string;
  createdAt: string;
  history: Update[];
};

export type Stage = { name: string; status: string; date?: string };

export type Order = {
  id: string;
  customerId: string;
  quotationId: string;
  productName: string;
  quantity: number;
  imageName?: string;
  imageFileId?: string;
  imageNames?: string[];
  imageFileIds?: string[];
  status: string;
  currentStage: string;
  progress: number;
  expectedDate: string;
  totalAmount: number;
  depositRequired: number;
  paidAmount: number;
  balanceDue: number;
  stages: Stage[];
  history: Update[];
  createdAt: string;
};

export type Rate = {
  id: string;
  courier: string;
  service: string;
  zone: string;
  weight: string;
  price: number;
  status: string;
  sourceFileId?: string;
  sourceName?: string;
};

export type Shipment = {
  id: string;
  customerId: string;
  manufacturingId?: string;
  type: string;
  courier: string;
  service: string;
  destination: string;
  zone: string;
  weight: string;
  status: string;
  tracking?: string;
  quotedAmount: number;
  createdAt: string;
  history: Update[];
};

export type Payment = {
  id: string;
  customerId: string;
  manufacturingId?: string;
  shippingId?: string;
  type: string;
  amount: number;
  status: string;
  proofName: string;
  proofFileId?: string;
  note: string;
  createdAt: string;
  confirmedAt?: string;
  reversedAt?: string;
  reversalReason?: string;
  allocations?: Array<{ manufacturingId?: string; shippingId?: string; amount: number }>;
  creditLeft?: number;
};

export type LedgerEntry = {
  id: string;
  customerId: string;
  entryNo: string;
  sourceType: string;
  sourceId: string;
  debit: number;
  credit: number;
  currency: string;
  note: string;
  postedAt: string;
};

export type Message = {
  id: string;
  author: string;
  authorRole?: string;
  body: string;
  attachments?: MessageAttachment[];
  createdAt: string;
};

export type MessageAttachment = {
  fileId: string;
  name: string;
  mimeType: string;
  byteSize: number;
  url: string;
  thumbnailUrl?: string;
};

export type Conversation = {
  id: string;
  customerId: string;
  subject?: string;
  status: string;
  lastMessageAt?: string;
  lastAuthor?: string;
  unreadForAdmin: number;
  unreadForCustomer: number;
  messages: Message[];
  messagePagination: PageInfo;
};

export type ConversationMessagePage = {
  conversationId: string;
  messages: Message[];
  pagination: PageInfo;
};

export type Call = {
  id: string;
  customerId: string;
  conversationId?: string;
  subject: string;
  status: string;
  callType?: string;
  roomId?: string;
  initiatorUserId?: string;
  initiatorName: string;
  initiatorRole: string;
  recipientName: string;
  answeredByUserId?: string;
  answeredBy?: string;
  createdAt: string;
  updatedAt?: string;
  ringExpiresAt?: string;
  startedAt?: string;
  endedAt?: string;
  durationSeconds: number;
  position?: number;
  priority?: string;
  queueMessage?: string;
  phone?: string;
  lastPage?: string;
  note?: string;
  requestedBy?: string;
  assignedTo?: string;
  customerLastSeenAt?: string;
  adminLastSeenAt?: string;
  history: Update[];
};

export type CallSignal = {
  id: string;
  callId: string;
  signalNo: number;
  senderId: string;
  senderRole: string;
  signalType: "offer" | "answer" | "ice" | "candidate" | "media-state" | "reconnect-request";
  payload: RTCSessionDescriptionInit & RTCIceCandidateInit & { muted?: boolean; reason?: string };
  createdAt: string;
};
export type UserPresence = {
  userId: string;
  customerId?: string;
  profileImageFileId?: string;
  name: string;
  email?: string;
  role: string;
  status: string;
  online: boolean;
  lastOnline: string;
  lastLogin: string;
  activeSessions: number;
};
export type PlatformSettings = {
  themePreset: string;
  primaryColor: string;
  accentColor: string;
  surfaceColor: string;
  defaultLanguage: "en" | "ur";
  enabledLanguages: Array<"en" | "ur">;
  allowUserLanguageChoice: boolean;
  updatedAt?: string;
  updatedBy?: string;
};
export type PlatformConnectionAddress = {
  interface: string;
  address: string;
  url: string;
};

export type PlatformConnectionInfo = {
  hostName: string;
  stableUrl: string;
  port: number;
  addresses: PlatformConnectionAddress[];
  updatedAt: string;
};
export type BackupRecord = {
  name: string;
  size: number;
  createdAt: string;
  fileCount: number;
  status: "Verified" | "Invalid" | string;
  dataSource: string;
  offsiteCopied?: boolean;
};

export type BackupPolicy = {
  automatic: boolean;
  interval: string;
  retention: number;
  dataSource: string;
  offsiteDir?: string;
  offsiteConfigured?: boolean;
  offsiteRequired?: boolean;
};

export type BackupOverview = {
  backups: BackupRecord[];
  policy: BackupPolicy;
};
export type EmailSenderSettings = {
  provider: string;
  mode: string;
  fromName: string;
  fromEmail: string;
  replyTo: string;
  domainStatus: string;
  webhookStatus: string;
  configured: boolean;
  lastVerifiedAt?: string;
  updatedAt?: string;
  updatedBy?: string;
  smtpHost?: string;
  smtpPort?: number;
  smtpUsername?: string;
  smtpImplicitTls?: boolean;
  passwordConfigured?: boolean;
};

export type EmailTemplate = {
  id: string;
  key: string;
  name: string;
  category: string;
  language: string;
  subject: string;
  body: string;
  variables: string[];
  enabled: boolean;
  version: number;
  updatedAt: string;
  updatedBy: string;
};

export type EmailAutomationRule = {
  id: string;
  trigger: string;
  label: string;
  category: string;
  templateKey: string;
  timing: string;
  enabled: boolean;
  essential: boolean;
  delayMinutes: number;
  maxReminders: number;
  lastTriggered?: string;
  updatedAt: string;
  updatedBy: string;
};

export type EmailOutboxItem = {
  id: string;
  deliveryId: string;
  eventKey: string;
  trigger: string;
  customerId?: string;
  entityId?: string;
  recipient: string;
  recipientName: string;
  templateKey: string;
  templateLanguage: string;
  status: string;
  scheduledAt: string;
  createdAt: string;
  updatedAt: string;
  lastError?: string;
  payload: Record<string, string>;
  attempts: number;
};

export type EmailDelivery = {
  id: string;
  outboxId: string;
  eventKey: string;
  trigger: string;
  customerId?: string;
  entityId?: string;
  recipient: string;
  recipientName: string;
  templateKey: string;
  templateLanguage: string;
  subject: string;
  status: string;
  provider: string;
  createdAt: string;
  scheduledAt: string;
  providerMessageId?: string;
  sentAt?: string;
  lastError?: string;
  templateVersion: number;
  attempts: number;
};

export type EmailMetrics = {
  queued: number;
  scheduled: number;
  captured: number;
  failed: number;
  cancelled: number;
  successRate: number;
  lastActivity?: string;
};

export type EmailSnapshot = {
  settings: EmailSenderSettings;
  templates: EmailTemplate[];
  rules: EmailAutomationRule[];
  outbox: EmailOutboxItem[];
  deliveries: EmailDelivery[];
  metrics: EmailMetrics;
  pagination: Record<string, PageInfo>;
};

export type EmailPreview = {
  subject: string;
  body: string;
  htmlBody?: string;
  missingVariables: string[];
  templateVersion: number;
};
export type CustomerNotice = {
  id: string;
  title: string;
  body: string;
  tone: "info" | "success" | "warning";
  audience: "all" | "selected";
  customerIds: string[];
  active: boolean;
  createdAt: string;
  updatedAt: string;
  createdBy: string;
};

export type FeaturedProduct = {
  id: string;
  name: string;
  tag: string;
  caption: string;
  imageFileId: string;
  imageName: string;
  productId?: string;
  sortOrder: number;
  active: boolean;
  createdAt: string;
  updatedAt: string;
  createdBy: string;
};

export type Workspace = {
  products: Product[];
  quotations: Quotation[];
  manufacturing: Order[];
  rateSheets: Rate[];
  shipping: Shipment[];
  payments: Payment[];
  ledger: LedgerEntry[];
  conversations: Conversation[];
  calls: Call[];
  presence: UserPresence[];
  notices: CustomerNotice[];
  featuredProducts: FeaturedProduct[];
  metrics: Record<string, number | string>;
  pagination: Record<string, PageInfo>;
};

export type PageInfo = { loaded: number; total: number; hasMore: boolean };

export type WorkspacePage = {
  scope: string;
  items: Array<{ id: string }>;
  pagination: PageInfo;
};

export type WorkItem = {
  id: string;
  title: string;
  subtitle: string;
  status: string;
  due: string;
  amount: string;
  progress: number;
  meta: string[];
};

export type Envelope<T> = {
  ok: boolean;
  requestId: string;
  data?: T;
  error?: { code: string; message: string };
};
