import type { Call, Customer, Metric, Order, Payment, Quotation, Shipment, UserPresence, WorkItem, Workspace } from "./types";

export function money(value: number) {
  return `Rs ${Math.round(value || 0).toLocaleString("en-PK")}`;
}

/** Normalize legacy stage labels for display. */
export function displayStageLabel(stage: string) {
  const value = stage.trim();
  if (value.toLowerCase() === "production ready") return "Starting Production";
  return value;
}

/** Product reference photo ids from an order, falling back to linked quotation. */
export function orderImageIds(order: Order, quotations: Quotation[] = []) {
  const fromOrder = [
    ...(order.imageFileIds || []),
    ...(order.imageFileId ? [order.imageFileId] : [])
  ]
    .map((id) => id.trim())
    .filter(Boolean);
  if (fromOrder.length) return Array.from(new Set(fromOrder));
  const quote = quotations.find((row) => row.id === order.quotationId);
  if (!quote) return [];
  return [
    ...(quote.imageFileIds || []),
    ...(quote.imageFileId ? [quote.imageFileId] : [])
  ]
    .map((id) => id.trim())
    .filter(Boolean)
    .filter((id, index, all) => all.indexOf(id) === index);
}

/** Prefer a real product title; skip 1-letter junk names from incomplete quote data. */
export function orderDisplayName(order: Order, quotations: Quotation[] = []) {
  const direct = (order.productName || "").trim();
  if (direct.length > 1 && !/^product consultation$/i.test(direct)) return direct;
  const quote = quotations.find((row) => row.id === order.quotationId);
  const fromQuote = (quote?.productName || "").trim();
  if (fromQuote.length > 1 && !/^product consultation$/i.test(fromQuote)) return fromQuote;
  if (direct) return direct;
  return order.id || "Manufacturing order";
}

/** Public shipping code shown on payments and shipping desks. */
export function shippingCode(shipment: { id?: string }) {
  const id = (shipment.id || "").trim();
  if (!id) return "";
  // SHIP-260906-1234 → Shipping-S1234; keep other ids as-is with Shipping- prefix.
  const match = id.match(/^SHIP-\d{6}-(\d+)$/i) || id.match(/^SHIP-(\d+)$/i) || id.match(/^S-?(\d+)$/i);
  if (match?.[1]) return `Shipping-S${match[1]}`;
  if (/^shipping-/i.test(id)) return id;
  return `Shipping-${id}`;
}

type AllocationCharge = {
  key: string;
  label: string;
  amount: number;
  postedAt: string;
};

/**
 * Uncleared order + shipping charges in ledger time order (oldest first).
 * Payments clear the earliest charges first. Each line shows only what is
 * still unpaid (e.g. charge 200 with 100 paid → Rs 100). Fully cleared lines are omitted.
 */
export function paymentAllocationRows(workspace: Workspace): Array<{ id: string; label: string; amount: number }> {
  const ordersById = new Map(workspace.manufacturing.map((order) => [order.id, order]));
  const shipmentsById = new Map(workspace.shipping.map((row) => [row.id, row]));
  const byKey = new Map<string, AllocationCharge>();

  const chronological = [...workspace.ledger].sort((left, right) => {
    const byTime = (left.postedAt || "").localeCompare(right.postedAt || "");
    return byTime !== 0 ? byTime : left.id.localeCompare(right.id);
  });

  for (const entry of chronological) {
    const resolved = resolveAllocationCharge(entry, ordersById, shipmentsById);
    if (!resolved) continue;
    // Only original charge debits / adjustments — payment credits are applied in FIFO below.
    if ((entry.sourceType || "").trim().toLowerCase() === "payment") continue;
    const net = (entry.debit || 0) - (entry.credit || 0);
    if (!net) continue;
    const existing = byKey.get(resolved.key);
    if (existing) {
      existing.amount += net;
      if (entry.postedAt && (!existing.postedAt || entry.postedAt < existing.postedAt)) {
        existing.postedAt = entry.postedAt;
      }
      continue;
    }
    byKey.set(resolved.key, {
      key: resolved.key,
      label: resolved.label,
      amount: net,
      postedAt: entry.postedAt || ""
    });
  }

  for (const order of workspace.manufacturing) {
    const key = `order:${order.id}`;
    if (byKey.has(key) || order.status === "Cancelled" || order.totalAmount < 1) continue;
    byKey.set(key, {
      key,
      label: orderAllocationLabel(order),
      amount: order.totalAmount,
      postedAt: order.createdAt || ""
    });
  }
  for (const shipment of workspace.shipping) {
    const key = `shipping:${shipment.id}`;
    if (byKey.has(key) || ["Cancelled", "Rejected"].includes(shipment.status) || shipment.quotedAmount < 1) {
      continue;
    }
    byKey.set(key, {
      key,
      label: shippingCode(shipment),
      amount: shipment.quotedAmount,
      postedAt: shipment.createdAt || ""
    });
  }

  const charges = [...byKey.values()]
    .filter((charge) => charge.amount > 0)
    .sort((left, right) => {
      const byTime = (left.postedAt || "").localeCompare(right.postedAt || "");
      return byTime !== 0 ? byTime : left.key.localeCompare(right.key);
    });

  const chargeTotal = charges.reduce((sum, charge) => sum + charge.amount, 0);
  const ledgerBalance = workspace.ledger.reduce((sum, entry) => sum + (entry.debit || 0) - (entry.credit || 0), 0);
  const orderBalance = workspace.manufacturing
    .filter((order) => order.status !== "Cancelled")
    .reduce((sum, order) => sum + Math.max(0, Number(order.balanceDue) || 0), 0);
  // Prefer live account due so partial ledgers still clear oldest-first correctly.
  const accountDue =
    workspace.ledger.length > 0 ? Math.max(0, ledgerBalance) : Math.max(0, orderBalance);
  const paymentCredits = workspace.ledger.length
    ? workspace.ledger
        .filter((entry) => (entry.sourceType || "").trim().toLowerCase() === "payment")
        .reduce((sum, entry) => sum + Math.max(0, entry.credit || 0), 0)
    : workspace.payments
        .filter((payment) => payment.status === "Confirmed")
        .reduce((sum, payment) => sum + payment.amount, 0);

  // Clear oldest charges first using the stronger of payment credits vs (charges − account due).
  const clearedFromBalance = Math.max(0, chargeTotal - accountDue);
  let remainingCredit = Math.max(paymentCredits, clearedFromBalance);
  if (remainingCredit > chargeTotal) remainingCredit = chargeTotal;

  const rows: Array<{ id: string; label: string; amount: number }> = [];
  for (const charge of charges) {
    const applied = Math.min(remainingCredit, charge.amount);
    remainingCredit -= applied;
    const due = charge.amount - applied;
    // Only not-fully-cleared lines; amount is what is still unpaid after oldest-first allocation.
    if (due > 0) rows.push({ id: charge.key, label: charge.label, amount: due });
  }
  return rows;
}

function resolveAllocationCharge(
  entry: { sourceType: string; sourceId: string },
  ordersById: Map<string, Order>,
  shipmentsById: Map<string, Shipment>
): { key: string; label: string } | null {
  const type = (entry.sourceType || "").trim().toLowerCase();
  const sourceId = (entry.sourceId || "").trim();
  if (!sourceId) return null;

  if (type === "manufacturing_order") {
    const order = ordersById.get(sourceId);
    return { key: `order:${sourceId}`, label: orderAllocationLabel(order || { id: sourceId, productName: "" }) };
  }
  if (type === "manufacturing_adjustment") {
    for (const order of ordersById.values()) {
      if (sourceId === order.id || sourceId.startsWith(`${order.id}-`)) {
        return { key: `order:${order.id}`, label: orderAllocationLabel(order) };
      }
    }
    return null;
  }
  if (type === "shipping_request") {
    const shipment = shipmentsById.get(sourceId);
    return { key: `shipping:${sourceId}`, label: shippingCode(shipment || { id: sourceId }) };
  }
  return null;
}

function orderAllocationLabel(order: { id?: string; productName?: string }) {
  const id = (order.id || "").trim() || "order";
  const name = (order.productName || "").trim();
  return name ? `Order-${id} ${name}` : `Order-${id}`;
}

/** Public job code shown on orders (same as quote id for new accepts). */
export function jobCode(order: Order) {
  return (order.id || "").trim();
}

/** Linked quotation code when it differs from the order id (legacy MFG-* rows). */
export function linkedQuoteCode(order: Order) {
  const quoteId = (order.quotationId || "").trim();
  if (!quoteId) return "";
  if (quoteId === (order.id || "").trim()) return "";
  return quoteId;
}

export function stashOrderSelection(orderId: string) {
  const id = orderId.trim();
  if (id) sessionStorage.setItem("cc_select_order_id", id);
}

export function stashQuoteSelection(quoteId: string) {
  const id = quoteId.trim();
  if (id) sessionStorage.setItem("cc_select_quote_id", id);
}

export function takeStoredSelection(key: string) {
  const value = (sessionStorage.getItem(key) || "").trim();
  if (value) sessionStorage.removeItem(key);
  return value;
}

export function date(value: string) {
  if (!value) return "Not set";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" });
}

export function dateTime(value: string) {
  if (!value) return "Not set";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString("en-GB", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: true
  });
}

const presenceOnlineWindowMs = 90_000;

/** Online only when recent activity AND not explicitly forced offline (logout). */
export function isPresenceOnline(lastOnline: string, onlineFlag?: boolean, now = Date.now()) {
  if (onlineFlag === false) return false;
  const parsed = Date.parse(lastOnline);
  if (!Number.isFinite(parsed)) return false;
  const age = now - parsed;
  if (age < -5_000) return false;
  const normalized = age < 0 ? 0 : age;
  return normalized <= presenceOnlineWindowMs;
}

export function lastSeenLabel(lastOnline: string, onlineFlag?: boolean, now = Date.now()) {
  if (isPresenceOnline(lastOnline, onlineFlag, now)) return "online";
  if (!lastOnline || lastOnline === "Never" || lastOnline === "Not tracked yet") return "never online";
  const parsed = new Date(lastOnline);
  if (Number.isNaN(parsed.getTime())) {
    return `last seen ${lastOnline}`;
  }
  const diff = Math.max(0, now - parsed.getTime());
  const seconds = Math.floor(diff / 1000);
  if (seconds < 8) return "last seen just now";
  if (seconds < 60) return "last seen a few seconds ago";
  const mins = Math.floor(diff / 60_000);
  if (mins < 60) {
    const safeMins = Math.max(1, mins);
    return `last seen ${safeMins} minute${safeMins === 1 ? "" : "s"} ago`;
  }
  const hours = Math.floor(diff / 3_600_000);
  if (hours < 24) {
    return `last seen ${hours} hour${hours === 1 ? "" : "s"} ago`;
  }

  const sameDay = parsed.toDateString() === new Date(now).toDateString();
  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);
  const time = parsed.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
  if (sameDay) return `last seen today at ${time}`;
  if (parsed.toDateString() === yesterday.toDateString()) return `last seen yesterday at ${time}`;
  return `last seen ${parsed.toLocaleDateString("en-GB", { day: "2-digit", month: "short" })} at ${time}`;
}

/** Prefer live presence row over stale customer.online flags. */
export function resolvePresenceStatus(
  presence: { online?: boolean; lastOnline?: string } | undefined,
  fallback?: { online?: boolean; lastOnline?: string },
  now = Date.now()
) {
  const lastOnline = presence?.lastOnline || fallback?.lastOnline || "";
  const onlineFlag = presence ? presence.online : fallback?.online;
  const online = isPresenceOnline(lastOnline, onlineFlag, now);
  return {
    online,
    lastOnline,
    label: online ? "online" : lastSeenLabel(lastOnline, onlineFlag === false ? false : onlineFlag, now)
  };
}

export function findCustomerPresence(
  presence: UserPresence[] | undefined,
  customerId?: string,
  hint?: { email?: string; companyName?: string },
  now = Date.now()
) {
  if (!presence?.length) return undefined;
  const id = (customerId || "").trim();
  const pickBest = (rows: UserPresence[]) => {
    if (!rows.length) return undefined;
    return [...rows].sort((a, b) => {
      const aOnline = isPresenceOnline(a.lastOnline, a.online, now) ? 1 : 0;
      const bOnline = isPresenceOnline(b.lastOnline, b.online, now) ? 1 : 0;
      if (aOnline !== bOnline) return bOnline - aOnline;
      return (b.lastOnline || "").localeCompare(a.lastOnline || "");
    })[0];
  };

  const customersOnly = presence.filter((person) => person.role.toLowerCase() === "customer");
  if (id) {
    const byId = customersOnly.filter((person) => person.customerId === id);
    if (byId.length) return pickBest(byId);
  }

  // Heal ID drift: same company name or email can still map to a live presence row.
  const company = (hint?.companyName || "").trim().toLowerCase();
  if (company) {
    const byName = customersOnly.filter((person) => (person.name || "").trim().toLowerCase() === company);
    if (byName.length) return pickBest(byName);
  }
  const email = (hint?.email || "").trim().toLowerCase();
  if (email) {
    const byEmail = customersOnly.filter((person) => (person.email || "").trim().toLowerCase() === email);
    if (byEmail.length) return pickBest(byEmail);
  }
  return undefined;
}

/** Best support-desk presence row (online first, then most recent). */
export function findSupportPresence(presence: UserPresence[] | undefined, now = Date.now(), excludeUserId?: string) {
  if (!presence?.length) return undefined;
  const exclude = (excludeUserId || "").trim();
  return [...presence]
    .filter((person) => person.role.toLowerCase() !== "customer" && (!exclude || person.userId !== exclude))
    .sort((a, b) => {
      const aOnline = isPresenceOnline(a.lastOnline, a.online, now) ? 1 : 0;
      const bOnline = isPresenceOnline(b.lastOnline, b.online, now) ? 1 : 0;
      if (aOnline !== bOnline) return bOnline - aOnline;
      return (b.lastOnline || "").localeCompare(a.lastOnline || "");
    })[0];
}

export function resolveCustomerPresence(
  presence: UserPresence[] | undefined,
  customer: { id?: string; email?: string; companyName?: string; online?: boolean; lastOnline?: string } | undefined,
  now = Date.now()
) {
  return resolvePresenceStatus(
    findCustomerPresence(presence, customer?.id, { email: customer?.email, companyName: customer?.companyName }, now),
    customer,
    now
  );
}

export function countOnlineCustomers(presence: UserPresence[] | undefined, customers: Customer[], now = Date.now()) {
  return customers.filter((customer) => resolveCustomerPresence(presence, customer, now).online).length;
}

export function presenceDetail(status: { online: boolean; lastOnline: string; label?: string }, whenOnline = "Active now") {
  if (status.online) return whenOnline;
  if (!status.lastOnline || status.lastOnline === "Never" || status.lastOnline === "Not tracked yet") {
    return "No login tracked";
  }
  return dateTime(status.lastOnline);
}

export function digits(value: string) {
  return value.replace(/[^0-9]/g, "");
}

export function labelFor(key: string) {
  return key.replace(/([A-Z])/g, " $1").replace(/^./, (letter) => letter.toUpperCase());
}

export function nextStage(status: string) {
  if (status === "Confirmed") return "Production";
  if (status === "Production") return "Quality check";
  if (status === "Quality check") return "Ready for delivery";
  if (status === "Ready for delivery") return "Completed";
  return "";
}

export function actionLabel(stage: string) {
  if (stage === "Production") return "Start manufacturing";
  if (stage === "Quality check") return "Move to QC";
  if (stage === "Ready for delivery") return "Mark ready";
  if (stage === "Completed") return "Mark completed";
  return stage;
}

export function customerMetrics(workspace: Workspace): Metric[] {
  return [
    {
      label: "Active orders",
      value: String(workspace.metrics.activeOrders || 0),
      delta: `${workspace.metrics.averageProgress || 0}% average progress`
    },
    {
      label: "Quotes",
      value: String(workspace.metrics.pendingQuotes || 0),
      delta: "Awaiting price or approval"
    },
    {
      label: "Payment due",
      value: money(Number(workspace.metrics.balanceDue || 0)),
      delta: `${workspace.payments.filter((payment) => payment.status === "Waiting confirmation").length} waiting confirmation`
    },
    {
      label: "Shipments",
      value: String(workspace.shipping.length),
      delta: `${workspace.shipping.filter((row) => row.status === "In transit").length} in transit`
    }
  ];
}

export function adminMetrics(workspace: Workspace, customers: Customer[]): Metric[] {
  return [
    {
      label: "Customers",
      value: String(customers.length),
      delta: `${countOnlineCustomers(workspace.presence, customers)} online now`
    },
    {
      label: "Quote pipeline",
      value: String(workspace.quotations.length),
      delta: `${workspace.quotations.filter((quote) => quote.status === "Requested").length} need pricing`
    },
    {
      label: "Production",
      value: String(workspace.manufacturing.length),
      delta: `${workspace.metrics.averageProgress || 0}% average progress`
    },
    {
      label: "Payment proofs",
      value: String(workspace.payments.filter((payment) => payment.status === "Waiting confirmation").length),
      delta: "Need confirmation"
    }
  ];
}

export function quoteToItem(quote: Quotation): WorkItem {
  return {
    id: quote.id,
    title: quote.productName,
    subtitle: `${quote.quantity} pcs`,
    status: quote.status,
    due: quote.expectedDate ? date(quote.expectedDate) : "Waiting date",
    amount: quote.totalAmount ? money(quote.totalAmount) : "Waiting price",
    progress: quote.status === "Accepted" ? 100 : quote.status === "Priced" ? 66 : 25,
    meta: [quote.steel, quote.tang, quote.finish].filter(Boolean)
  };
}

export function orderToItem(order: Order): WorkItem {
  return {
    id: order.id,
    title: order.productName,
    subtitle: `${order.quantity} pcs`,
    status: order.status,
    due: order.expectedDate ? date(order.expectedDate) : "Date not set",
    amount: money(order.balanceDue),
    progress: order.progress,
    meta: [order.currentStage]
  };
}

export function paymentToItem(payment: Payment): WorkItem {
  return {
    id: payment.id,
    title: payment.type,
    subtitle: payment.proofName || "No proof",
    status: payment.status,
    due: date(payment.createdAt),
    amount: money(payment.amount),
    progress: payment.status === "Confirmed" ? 100 : 40,
    meta: [payment.manufacturingId || payment.shippingId || "Account"]
  };
}

export function callToItem(call: Call): WorkItem {
  const active = call.status === "Ringing" || call.status === "In call";
  return {
    id: call.id,
    title: call.subject,
    subtitle: (call.initiatorName || "Customer") + " to " + (call.recipientName || "Support"),
    status: call.status,
    due: active ? call.status : date(call.endedAt || call.updatedAt || call.createdAt),
    amount: call.durationSeconds ? call.durationSeconds + "s" : "Audio",
    progress: call.status === "Completed" ? 100 : call.status === "In call" ? 70 : call.status === "Ringing" ? 30 : 0,
    meta: [call.customerId, call.answeredBy || call.initiatorRole || "Customer"]
  };
}

export function messageFromError(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
