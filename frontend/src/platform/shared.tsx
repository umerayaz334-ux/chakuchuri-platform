import { Fragment, type ReactNode, useEffect, useRef, useState } from "react";
import { CheckCheck, Download, FileImage, FileText } from "lucide-react";
import { apiBlob } from "./api";
import type {
  Conversation,
  Customer,
  Foundation,
  Metric,
  Module,
  Order,
  Payment,
  LedgerEntry,
  MessageAttachment,
  Quotation,
  Rate,
  Shipment,
  Stage,
  Update,
  UserPresence,
  WorkItem,
  Workspace
} from "./types";
import { date, displayStageLabel, money, orderImageIds, resolveCustomerPresence, resolvePresenceStatus } from "./utils";
import { usePresenceNow } from "./use-presence-now";

export function MetricGrid({ metrics }: { metrics: Metric[] }) {
  return <section className="cc-metric-grid">{metrics.map((metric) => <MetricTile key={metric.label} metric={metric} />)}</section>;
}

export function MetricTile({ metric }: { metric: Metric }) {
  return (
    <article className="cc-metric-tile">
      <span>{metric.label}</span>
      <strong>{metric.value}</strong>
      <small>{metric.delta}</small>
    </article>
  );
}

export function Panel({
  title,
  label,
  action,
  onAction,
  children
}: {
  title: string;
  label: string;
  action?: string;
  onAction?: () => void;
  children: ReactNode;
}) {
  return (
    <div className="cc-panel">
      <header className="cc-panel-title">
        <div>
          <span>{label}</span>
          <h2>{title}</h2>
        </div>
        {action && onAction && <button onClick={onAction} type="button">{action}</button>}
      </header>
      {children}
    </div>
  );
}

export function Empty({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="cc-empty-state">
      <strong>{title}</strong>
      <small>{detail}</small>
    </div>
  );
}

export function ProductFileThumb({
  fileId,
  name = "Product",
  token,
  className = ""
}: {
  fileId?: string;
  name?: string;
  token: string;
  className?: string;
}) {
  const [src, setSrc] = useState("");
  useEffect(() => {
    if (!fileId || !token) {
      setSrc("");
      return;
    }
    let active = true;
    let objectUrl = "";
    apiBlob(`/api/files/${fileId}/thumbnail`, token)
      .catch(() => apiBlob(`/api/files/${fileId}/content`, token))
      .then((blob) => {
        if (!active) return;
        objectUrl = URL.createObjectURL(blob);
        setSrc(objectUrl);
      })
      .catch(() => {
        if (active) setSrc("");
      });
    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [fileId, token]);

  return (
    <span className={`cc-product-thumb ${className}`.trim()} aria-hidden={!src}>
      {src ? <img alt={name} src={src} /> : <FileImage size={22} strokeWidth={1.75} />}
    </span>
  );
}

export function OrderGrid({ orders, token = "", quotations = [] }: { orders: Order[]; token?: string; quotations?: Quotation[] }) {
  if (orders.length === 0) return <Empty title="No manufacturing orders" detail="Accepted quotes become production timelines here." />;
  return (
    <div className="cc-card-grid">
      {orders.map((order) => (
        <OrderCard key={order.id} order={order} quotations={quotations} token={token} />
      ))}
    </div>
  );
}

export function OrderCard({
  order,
  token = "",
  quotations = []
}: {
  order: Order;
  token?: string;
  quotations?: Quotation[];
}) {
  const imageId = orderImageIds(order, quotations)[0] || "";
  return (
    <article className="cc-work-card cc-order-card">
      <div className="cc-work-head">
        <div className="cc-order-card-lead">
          {token ? <ProductFileThumb fileId={imageId} name={order.productName} token={token} /> : null}
          <div>
            <span>{order.id}</span>
            <h3>{order.productName}</h3>
            <p>
              {order.quantity} pcs / due {date(order.expectedDate)}
            </p>
          </div>
        </div>
        <strong>{order.status}</strong>
      </div>
      <div className="cc-progress">
        <i style={{ width: `${order.progress}%` }} />
      </div>
      <div className="cc-work-meta">
        <span>{order.progress}% complete</span>
        <span>{money(order.balanceDue)} due</span>
        <span>{displayStageLabel(order.currentStage)}</span>
      </div>
      <Timeline stages={order.stages} />
      <UpdateFeed updates={order.history} />
    </article>
  );
}

export function Timeline({ stages }: { stages: Stage[] }) {
  return (
    <div className="cc-timeline cc-stage-timeline">
      {stages.map((stage) => (
        <div className={stage.status} key={stage.name}>
          <span />
          <div>
            <strong>{stage.name}</strong>
            <small>{stage.date ? date(stage.date) : "Pending"}</small>
          </div>
        </div>
      ))}
    </div>
  );
}

export function UpdateFeed({ updates }: { updates: Update[] }) {
  if (!updates.length) return null;
  return (
    <div className="cc-update-feed">
      {updates.slice(0, 3).map((update) => (
        <article key={update.label + update.createdAt}>
          <div>
            <strong>{update.label}</strong>
            <p>{update.detail}</p>
          </div>
          <small>{date(update.createdAt)}</small>
        </article>
      ))}
    </div>
  );
}

export function ShippingRows({ shipping }: { shipping: Shipment[] }) {
  if (shipping.length === 0) return <Empty title="No shipping requests" detail="Manufactured and outside-product shipping appears here." />;
  return (
    <div className="cc-rows">
      {shipping.map((row) => (
        <div key={row.id}>
          <span>
            {row.id} / {row.type}
          </span>
          <strong>
            {row.courier} {row.service}
          </strong>
          <small>
            {row.destination} / Zone {row.zone} / {money(row.quotedAmount)} / {row.status}
          </small>
        </div>
      ))}
    </div>
  );
}

export function PaymentRows({ payments }: { payments: Payment[] }) {
  if (payments.length === 0) return <Empty title="No payment records" detail="Account payments and shipping proofs appear here." />;
  return (
    <div className="cc-rows">
      {payments.map((payment) => (
        <div key={payment.id}>
          <span>
            {payment.id} / {payment.type}
          </span>
          <strong>{money(payment.amount)}</strong>
          <small>
            {payment.status}
            {payment.allocations?.length ? ` · applied to ${payment.allocations.length} order(s)` : ""}
            {payment.creditLeft ? ` · ${money(payment.creditLeft)} credit` : ""}
            {" / "}
            {payment.proofName || "No proof"}
          </small>
        </div>
      ))}
    </div>
  );
}

export function PaymentSummary({ orders, payments, ledger = [] }: { orders: Order[]; payments: Payment[]; ledger?: LedgerEntry[] }) {
  const ledgerDue = ledger.reduce((sum, entry) => sum + entry.debit - entry.credit, 0);
  const due = ledger.length ? Math.max(ledgerDue, 0) : orders.filter((order) => order.status !== "Cancelled" && order.status !== "Completed").reduce((sum, order) => sum + order.balanceDue, 0);
  const confirmed = ledger.length
    ? ledger.filter((entry) => entry.sourceType === "payment").reduce((sum, entry) => sum + entry.credit, 0)
    : payments.filter((payment) => payment.status === "Confirmed").reduce((sum, payment) => sum + payment.amount, 0);
  const charges = ledger.length ? ledger.reduce((sum, entry) => sum + entry.debit, 0) : due + confirmed;
  const waiting = payments.filter((payment) => payment.status === "Waiting confirmation").reduce((sum, payment) => sum + payment.amount, 0);
  const credit = ledger.length && ledgerDue < 0 ? Math.abs(ledgerDue) : 0;

  return (
    <div className="cc-rate-card cc-payment-summary">
      <div>
        <span>Total charges</span>
        <strong>{money(charges)}</strong>
      </div>
      <div>
        <span>Confirmed paid</span>
        <strong>{money(confirmed)}</strong>
      </div>
      <div>
        <span>Pending proof</span>
        <strong>{money(waiting)}</strong>
      </div>
      <div>
        <span>{credit > 0 ? "Account credit" : "Balance due"}</span>
        <strong>{money(credit > 0 ? credit : due)}</strong>
      </div>
    </div>
  );
}
export function LedgerStatement({ ledger }: { ledger: LedgerEntry[] }) {
  if (ledger.length === 0) return <Empty title="No ledger entries" detail="Confirmed payments and posted charges appear here." />;
  let running = ledger.reduce((sum, entry) => sum + entry.debit - entry.credit, 0);

  return (
    <div className="cc-ledger-statement">
      {ledger.map((entry) => {
        const balanceAfter = running;
        running -= entry.debit - entry.credit;
        return (
          <article key={entry.id || entry.entryNo}>
            <div>
              <span>{date(entry.postedAt)}</span>
              <strong>{entry.note || entry.sourceType}</strong>
              <small>{entry.entryNo} / {entry.sourceId}</small>
            </div>
            <em className={entry.debit > 0 ? "debit" : "credit"}>{entry.debit > 0 ? money(entry.debit) : money(entry.credit)}</em>
            <b>{money(Math.max(balanceAfter, 0))}</b>
          </article>
        );
      })}
    </div>
  );
}

export function ConversationList({ conversations }: { conversations: Conversation[] }) {
  if (conversations.length === 0) return <Empty title="No messages yet" detail="New conversations appear here." />;
  return (
    <div className="cc-conversation-list">
      {conversations.map((conversation) => {
        const unread = Math.max(conversation.unreadForAdmin || 0, conversation.unreadForCustomer || 0);
        return (
          <article key={conversation.id}>
            <header>
              <div>
                <span>{conversation.subject || "Support conversation"}</span>
                <strong>{conversation.id}</strong>
                <small>{conversation.lastMessageAt ? `Last message ${date(conversation.lastMessageAt)}` : "No recent message"}</small>
              </div>
              <div className="cc-thread-state">
                {unread > 0 && <em>{unread} new</em>}
                <b>{conversation.status}</b>
              </div>
            </header>
            <div className="cc-message-flow">
              {conversation.messages.slice(-6).map((message) => {
                const internal = (message.authorRole || message.author).toLowerCase() !== "customer";
                return (
                  <div className={internal ? "admin" : "customer"} key={message.id}>
                    <strong>{message.author}</strong>
                    <p>{message.body}</p>
                    <small>{message.authorRole || (internal ? "Admin" : "Customer")} / {date(message.createdAt)}</small>
                  </div>
                );
              })}
            </div>
          </article>
        );
      })}
    </div>
  );
}

export function MessengerBubbles({
  conversation,
  perspective,
  token,
  unreadCount = 0,
  loadingEarlier = false,
  onLoadEarlier
}: {
  conversation?: Conversation;
  perspective: "admin" | "customer";
  token?: string;
  unreadCount?: number;
  loadingEarlier?: boolean;
  onLoadEarlier?: () => void;
}) {
  const listRef = useRef<HTMLDivElement>(null);
  const [stickyDate, setStickyDate] = useState("");
  const [stickyVisible, setStickyVisible] = useState(false);
  const stickyDateRef = useRef("");
  const hideTimer = useRef<number | null>(null);

  const latestMessageId = conversation?.messages[conversation.messages.length - 1]?.id || "";

  useEffect(() => {
    const scrollToLatest = () => {
      if (listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight;
    };
    const frame = window.requestAnimationFrame(scrollToLatest);
    const timer = window.setTimeout(scrollToLatest, 120);
    return () => {
      window.cancelAnimationFrame(frame);
      window.clearTimeout(timer);
    };
  }, [conversation?.id, latestMessageId]);

  useEffect(() => () => {
    if (hideTimer.current !== null) window.clearTimeout(hideTimer.current);
  }, []);

  useEffect(() => {
    const root = listRef.current;
    if (!root) return;

    const clearCovered = () => {
      root.querySelectorAll<HTMLElement>("[data-chat-day].is-covered").forEach((marker) => {
        marker.classList.remove("is-covered");
      });
    };

    const updateStickyDate = (scrolling: boolean) => {
      const markers = Array.from(root.querySelectorAll<HTMLElement>("[data-chat-day]"));
      if (!markers.length) return;
      const top = root.getBoundingClientRect().top + 16;
      let active = markers[0].dataset.chatDay || "";
      let activeMarker: HTMLElement | null = markers[0];
      for (const marker of markers) {
        const rect = marker.getBoundingClientRect();
        if (rect.top <= top) {
          active = marker.dataset.chatDay || active;
          activeMarker = marker;
        } else {
          break;
        }
      }
      if (active && active !== stickyDateRef.current) {
        stickyDateRef.current = active;
        setStickyDate(active);
      }
      // Only float the chip while scrolling, and hide the matching in-flow pill
      // so you never see two "Today" tags at once.
      if (scrolling && active) {
        markers.forEach((marker) => {
          marker.classList.toggle("is-covered", marker === activeMarker);
        });
        setStickyVisible(true);
      }
    };

    const onScroll = () => {
      updateStickyDate(true);
      if (hideTimer.current !== null) window.clearTimeout(hideTimer.current);
      hideTimer.current = window.setTimeout(() => {
        setStickyVisible(false);
        clearCovered();
      }, 850);
    };

    updateStickyDate(false);
    root.addEventListener("scroll", onScroll, { passive: true });
    return () => root.removeEventListener("scroll", onScroll);
  }, [conversation?.id, conversation?.messages.length]);

  if (!conversation || conversation.messages.length === 0) return <Empty title="No messages yet" detail="Start the conversation from the box below." />;
  const messages = [...conversation.messages].sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  const unreadStart = unreadIncomingStart(messages, unreadCount, perspective);
  let previousDay = "";
  return (
    <div className="cc-message-bubbles-shell">
      <div className={"cc-sticky-date" + (stickyVisible && stickyDate ? " is-visible" : "")} aria-hidden={!stickyVisible}>
        <span>{stickyDate}</span>
      </div>
      <div className="cc-message-bubbles" ref={listRef}>
        {onLoadEarlier && (
          <div className="cc-chat-load-earlier">
            <button disabled={loadingEarlier} onClick={onLoadEarlier} type="button">
              {loadingEarlier ? "Loading..." : "Load earlier messages"}
            </button>
          </div>
        )}
        {messages.map((message, index) => {
          const own = isOwnMessengerMessage(message.authorRole || message.author, perspective);
          const dayKey = messageDayKey(message.createdAt);
          const dayLabel = messageDayLabel(message.createdAt);
          const showDate = Boolean(dayKey) && dayKey !== previousDay;
          if (showDate) previousDay = dayKey;
          const receipt = own ? ownMessageReceipt(messages, message, perspective, conversation) : null;
          return (
            <Fragment key={message.id}>
              {showDate && (
                <div aria-label={dayLabel} className="cc-date-divider" data-chat-day={dayLabel}>
                  <span>{dayLabel}</span>
                </div>
              )}
              {unreadStart === index && (
                <div aria-label="Unread messages" className="cc-unread-divider">
                  <span>Unread messages</span>
                </div>
              )}
              <div className={"cc-message-bubble " + (own ? "own" : "their")}>
                {message.body && <p>{message.body}</p>}
                {message.attachments && message.attachments.length > 0 && (
                  <div className="cc-message-attachments">
                    {message.attachments.map((attachment) => <AuthenticatedAttachment attachment={attachment} key={attachment.fileId} token={token} />)}
                  </div>
                )}
                <small className="cc-msg-meta">
                  <span>{messageStamp(message.createdAt)}</span>
                  {receipt && (
                    <CheckCheck
                      aria-label={receiptLabel(receipt)}
                      className={"cc-msg-ticks " + receipt}
                      size={15}
                      strokeWidth={2.4}
                    />
                  )}
                </small>
              </div>
            </Fragment>
          );
        })}
      </div>
    </div>
  );
}

export function conversationPreview(conversation?: Conversation) {
  if (!conversation || conversation.messages.length === 0) return "No messages yet";
  const message = conversation.messages[conversation.messages.length - 1];
  return message.body || (message.attachments?.some((attachment) => attachment.mimeType.startsWith("image/")) ? "Photo" : "Attachment");
}

function AuthenticatedAttachment({ attachment, token }: { attachment: MessageAttachment; token?: string }) {
  const image = attachment.mimeType.startsWith("image/");
  const [src, setSrc] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!image || !token) return;
    let active = true;
    let objectURL = "";
    apiBlob(attachment.thumbnailUrl || attachment.url, token)
      .then((blob) => {
        if (!active) return;
        objectURL = URL.createObjectURL(blob);
        setSrc(objectURL);
      })
      .catch(() => setSrc(""));
    return () => {
      active = false;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [attachment.fileId, attachment.thumbnailUrl, attachment.url, image, token]);

  const download = async () => {
    if (!token || busy) return;
    setBusy(true);
    try {
      const blob = await apiBlob(attachment.url, token);
      const href = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = href;
      anchor.download = attachment.name;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(href), 1000);
    } finally {
      setBusy(false);
    }
  };

  if (image && src) {
    return (
      <button aria-label={"Download " + attachment.name} className="cc-message-image" onClick={download} title="Download image" type="button">
        <img alt={attachment.name} src={src} />
      </button>
    );
  }

  return (
    <button className="cc-message-file" disabled={busy || !token} onClick={download} title="Download attachment" type="button">
      <FileText size={19} />
      <span><strong>{attachment.name}</strong><small>{formatFileSize(attachment.byteSize)}</small></span>
      <Download size={17} />
    </button>
  );
}

function formatFileSize(bytes: number) {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return Math.round(bytes / 1024) + " KB";
  return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}

export function conversationInitials(name: string) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("") || "CC";
}

export function isOwnMessengerMessage(role: string, perspective: "admin" | "customer") {
  const normalized = role.toLowerCase();
  const fromCustomer = normalized === "customer" || normalized.includes("buyer");
  return perspective === "customer" ? fromCustomer : !fromCustomer;
}

export function ownMessageReceipt(
  messages: Conversation["messages"],
  message: Conversation["messages"][number],
  perspective: "admin" | "customer",
  conversation: Conversation
): "delivered" | "read" {
  const own = messages.filter((item) => isOwnMessengerMessage(item.authorRole || item.author, perspective));
  const index = own.findIndex((item) => item.id === message.id);
  const peerUnread = perspective === "customer" ? conversation.unreadForAdmin || 0 : conversation.unreadForCustomer || 0;
  if (index < 0) return "delivered";
  return own.length - index <= peerUnread ? "delivered" : "read";
}

function receiptLabel(status: "delivered" | "read") {
  return status === "read" ? "Read" : "Delivered";
}

function unreadIncomingStart(
  messages: Conversation["messages"],
  unreadCount: number,
  perspective: "admin" | "customer"
) {
  if (unreadCount <= 0) return -1;
  let remaining = unreadCount;
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (isOwnMessengerMessage(message.authorRole || message.author, perspective)) continue;
    remaining -= 1;
    if (remaining === 0) return index;
  }
  return messages.length ? 0 : -1;
}

function messageDayKey(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  return `${parsed.getFullYear()}-${parsed.getMonth()}-${parsed.getDate()}`;
}

function messageDayLabel(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value || "Unknown date";
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const startOfMessage = new Date(parsed.getFullYear(), parsed.getMonth(), parsed.getDate());
  const dayDiff = Math.round((startOfToday.getTime() - startOfMessage.getTime()) / 86_400_000);
  if (dayDiff === 0) return "Today";
  if (dayDiff === 1) return "Yesterday";
  return parsed.toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric" });
}

function messageStamp(value: string) {
  if (!value) return "";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}
export function SupportPresence({ presence }: { presence: UserPresence[] }) {
  const now = usePresenceNow();
  const visible = presence.slice(0, 5);
  if (visible.length === 0) {
    return (
      <div className="cc-presence-strip">
        <div>
          <i />
          <span>Support team</span>
          <strong>Presence loading</strong>
        </div>
      </div>
    );
  }
  return (
    <div className="cc-presence-strip">
      {visible.map((person) => {
        const status = resolvePresenceStatus(person, undefined, now);
        return (
          <div key={person.userId}>
            <i className={status.online ? "online" : ""} />
            <span>{person.role}</span>
            <strong>{person.name}</strong>
            <small>{status.label}</small>
          </div>
        );
      })}
    </div>
  );
}
export function RateCards({ rates }: { rates: Rate[] }) {
  if (rates.length === 0) return <Empty title="No active rates" detail="Admin rate sheets will appear here." />;
  return (
    <div className="cc-rate-card">
      {rates.map((rate) => (
        <div key={rate.id}>
          <span>
            {rate.courier} / Zone {rate.zone}
          </span>
          <strong>{money(rate.price)}</strong>
          <small>
            {rate.service} / {rate.weight}{rate.sourceName ? ` / ${rate.sourceName}` : ""}
          </small>
        </div>
      ))}
    </div>
  );
}

export function ModuleStack({ modules }: { modules: Module[] }) {
  return (
    <div className="cc-module-stack">
      {modules.slice(0, 8).map((module) => (
        <article key={module.name}>
          <span>{module.owner}</span>
          <strong>{module.name}</strong>
          <small>{module.status}</small>
        </article>
      ))}
    </div>
  );
}

export function QueueList({ items }: { items: WorkItem[] }) {
  if (items.length === 0) return <Empty title="No activity" detail="New work appears here as it arrives." />;
  return (
    <div className="cc-queue-list">
      {items.map((item) => (
        <article key={item.id}>
          <div>
            <span>{item.status}</span>
            <strong>{item.title}</strong>
            <small>{item.subtitle}</small>
          </div>
          <em>{item.due}</em>
        </article>
      ))}
    </div>
  );
}

export function SettingsPanel({ foundation }: { foundation: Foundation }) {
  return (
    <div className="cc-settings-panel">
      <strong>{foundation.phase}</strong>
      {foundation.principles.map((principle) => (
        <p key={principle}>{principle}</p>
      ))}
    </div>
  );
}

export function ReliabilityPanel() {
  return (
    <div className="cc-rate-card">
      <div>
        <span>API shape</span>
        <strong>Request IDs</strong>
      </div>
      <div>
        <span>Error safety</span>
        <strong>Panic recovery</strong>
      </div>
      <div>
        <span>Traceability</span>
        <strong>Audit events</strong>
      </div>
    </div>
  );
}

export function CustomerTable({ customers, workspace, onOpen }: { customers: Customer[]; workspace: Workspace; onOpen: (customerId: string) => void }) {
  const now = usePresenceNow();
  if (customers.length === 0) return <Empty title="No customers yet" detail="Signup accounts will appear here." />;
  return (
    <div className="cc-customer-table cc-account-rows">
      {customers.map((customer) => {
        const orders = workspace.manufacturing.filter((order) => order.customerId === customer.id);
        const balance = orders.reduce((sum, order) => sum + order.balanceDue, 0);
        const status = resolveCustomerPresence(workspace.presence, customer, now);

        return (
          <article key={customer.id}>
            <span className={status.online ? "cc-live-dot" : "cc-idle-dot"} />
            <div>
              <strong>{customer.companyName}</strong>
              <small>
                {customer.contactName || customer.email} / {customer.country} / {status.label}
              </small>
            </div>
            <em>{balance ? money(balance) : customer.balanceDue}</em>
            <em>{orders.length || customer.openOrders} orders</em>
            <button onClick={() => onOpen(customer.id)} type="button">View account</button>
          </article>
        );
      })}
    </div>
  );
}
