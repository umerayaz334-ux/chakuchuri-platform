import type {
  Call,
  Conversation,
  CustomerNotice,
  FeaturedProduct,
  LedgerEntry,
  Order,
  Payment,
  Product,
  Quotation,
  Rate,
  Shipment,
  UserPresence,
  Workspace
} from "./types";

export const apiMutationEventName = "cc:api-mutation";
export const userAccessEventName = "cc:user-access-changed";

export type ApiMutationDetail = {
  path: string;
  data: unknown;
};

export type WorkspaceRealtimeChange = {
  scope: string;
  operation: "upsert" | "remove" | "invalidate";
  customerId?: string;
  data?: unknown;
};

type WorkspaceSocketEvent = {
  type: "ready" | "pong" | "workspace.changed";
  revision: number;
  serverTime: string;
  changes?: WorkspaceRealtimeChange[];
};

type RealtimeHandlers = {
  onChanges: (changes: WorkspaceRealtimeChange[]) => void;
  onResync: () => void;
};

export function connectWorkspaceRealtime(token: string, handlers: RealtimeHandlers) {
  let stopped = false;
  let socket: WebSocket | null = null;
  let retryTimer: number | null = null;
  let heartbeatTimer: number | null = null;
  let retryCount = 0;
  let knownRevision: number | null = null;

  const clearConnectionTimers = () => {
    if (retryTimer !== null) window.clearTimeout(retryTimer);
    if (heartbeatTimer !== null) window.clearInterval(heartbeatTimer);
    retryTimer = null;
    heartbeatTimer = null;
  };

  const scheduleReconnect = () => {
    if (stopped || retryTimer !== null) return;
    const delay = Math.min(15_000, 750 * 2 ** Math.min(retryCount, 5));
    retryCount += 1;
    retryTimer = window.setTimeout(() => {
      retryTimer = null;
      open();
    }, delay);
  };

  const handleEvent = (event: MessageEvent) => {
    let payload: WorkspaceSocketEvent;
    try {
      payload = JSON.parse(String(event.data)) as WorkspaceSocketEvent;
    } catch {
      return;
    }
    if (!Number.isFinite(payload.revision)) return;

    if (payload.type === "ready") {
      knownRevision = payload.revision;
      // Revisions are connection-local; always recover across a new subscription.
      handlers.onResync();
      return;
    }
    if (payload.type === "pong") {
      if (knownRevision !== null && payload.revision !== knownRevision) handlers.onResync();
      knownRevision = payload.revision;
      return;
    }
    if (payload.type !== "workspace.changed") return;

    if (knownRevision !== null && payload.revision !== knownRevision + 1) handlers.onResync();
    knownRevision = Math.max(knownRevision || 0, payload.revision);
    if (payload.changes?.length) handlers.onChanges(payload.changes);
  };

  const open = () => {
    if (stopped || typeof WebSocket === "undefined") return;
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    socket = new WebSocket(`${protocol}//${window.location.host}/api/realtime/workspace?token=${encodeURIComponent(token)}`);
    socket.onopen = () => {
      retryCount = 0;
      if (heartbeatTimer !== null) window.clearInterval(heartbeatTimer);
      heartbeatTimer = window.setInterval(() => {
        if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type: "ping" }));
      }, 15_000);
    };
    socket.onmessage = handleEvent;
    socket.onerror = () => socket?.close();
    socket.onclose = () => {
      if (heartbeatTimer !== null) window.clearInterval(heartbeatTimer);
      heartbeatTimer = null;
      socket = null;
      scheduleReconnect();
    };
  };

  const reconnectWhenOnline = () => {
    if (!socket || socket.readyState === WebSocket.CLOSED) open();
  };

  window.addEventListener("online", reconnectWhenOnline);
  open();

  return () => {
    stopped = true;
    clearConnectionTimers();
    window.removeEventListener("online", reconnectWhenOnline);
    socket?.close();
    socket = null;
  };
}

export function applyWorkspaceChanges(current: Workspace, changes: WorkspaceRealtimeChange[]): Workspace {
  let next = current;
  for (const change of changes) {
    if (change.operation === "invalidate" || !change.data) continue;
    switch (change.scope) {
      case "quotations":
        next = { ...next, quotations: updateCollection(next.quotations, change, change.data as Quotation) };
        break;
      case "manufacturing":
        next = { ...next, manufacturing: updateCollection(next.manufacturing, change, change.data as Order) };
        break;
      case "rateSheets":
        next = { ...next, rateSheets: updateCollection(next.rateSheets, change, change.data as Rate) };
        break;
      case "shipping":
        next = { ...next, shipping: updateCollection(next.shipping, change, change.data as Shipment) };
        break;
      case "payments":
        next = { ...next, payments: updateCollection(next.payments, change, change.data as Payment) };
        break;
      case "ledger":
        next = { ...next, ledger: updateCollection(next.ledger, change, change.data as LedgerEntry) };
        break;
      case "products":
        next = { ...next, products: updateCollection(next.products, change, change.data as Product) };
        break;
      case "conversations":
        next = { ...next, conversations: updateCollection(next.conversations, change, change.data as Conversation) };
        break;
      case "calls":
        next = { ...next, calls: updateCollection(next.calls, change, change.data as Call) };
        break;
      case "notices":
        next = { ...next, notices: updateCollection(next.notices || [], change, change.data as CustomerNotice) };
        break;
      case "featured":
        next = { ...next, featuredProducts: updateCollection(next.featuredProducts || [], change, change.data as FeaturedProduct) };
        break;
      case "presence": {
        const person = change.data as UserPresence;
        if (!person?.userId) break;
        const rows = next.presence || [];
        const index = rows.findIndex((row) => row.userId === person.userId);
        if (index < 0) {
          next = { ...next, presence: [person, ...rows] };
        } else {
          const merged = [...rows];
          merged[index] = {
            ...merged[index],
            ...person,
            email: person.email || merged[index].email,
            online: person.online,
            lastOnline: person.lastOnline || merged[index].lastOnline
          };
          next = { ...next, presence: merged };
        }
        break;
      }
    }
  }
  return next === current ? current : { ...next, metrics: deriveMetrics(next) };
}

export function changesFromApiMutation({ path, data }: ApiMutationDetail): WorkspaceRealtimeChange[] {
  const upsert = (scope: string, value: unknown): WorkspaceRealtimeChange[] => value ? [{ scope, operation: "upsert", data: value }] : [];

  if (path === "/api/workflow/quotes") return upsert("quotations", data);
  if (/^\/api\/workflow\/quotes\/[^/]+\/(price|reject)$/.test(path)) return upsert("quotations", data);
  if (/^\/api\/workflow\/quotes\/[^/]+\/accept$/.test(path)) return upsert("manufacturing", data);
  if (/^\/api\/workflow\/manufacturing\/[^/]+\/(stage|update|edit|cancel)$/.test(path)) return upsert("manufacturing", data);
  if (path === "/api/workflow/ratesheets") return upsert("rateSheets", data);
  if (path === "/api/workflow/ratesheets/import") {
    const rates = (data as { rates?: Rate[] })?.rates || [];
    return rates.map((rate) => ({ scope: "rateSheets", operation: "upsert", data: rate }));
  }
  if (path === "/api/workflow/shipping") return upsert("shipping", data);
  if (path === "/api/workflow/payments") return upsert("payments", data);
  if (/^\/api\/workflow\/payments\/[^/]+\/confirm$/.test(path)) {
    const body = data as { payment?: Payment; orders?: Order[]; ledger?: LedgerEntry } & Payment;
    const payment = body.payment || body;
    const changes = upsert("payments", payment);
    for (const order of body.orders || []) changes.push(...upsert("manufacturing", order));
    if (body.ledger) changes.push(...upsert("ledger", body.ledger));
    return changes;
  }
  if (path === "/api/workflow/products") return upsert("products", data);
  if (path === "/api/workflow/messages" || /^\/api\/workflow\/messages\/[^/]+\/read$/.test(path)) return upsert("conversations", data);
  if (path === "/api/workflow/calls" || /^\/api\/workflow\/calls\/[^/]+\/(status|end)$/.test(path)) return upsert("calls", data);
  if (path === "/api/workflow/notices" || /^\/api\/workflow\/notices\/[^/]+\/update$/.test(path)) return upsert("notices", data);
  if (/^\/api\/workflow\/notices\/[^/]+\/delete$/.test(path)) return data ? [{ scope: "notices", operation: "remove", data }] : [];
  if (path === "/api/workflow/featured" || /^\/api\/workflow\/featured\/[^/]+\/update$/.test(path)) return upsert("featured", data);
  if (/^\/api\/workflow\/featured\/[^/]+\/delete$/.test(path)) return data ? [{ scope: "featured", operation: "remove", data }] : [];
  return [];
}

function updateCollection<T extends { id: string }>(rows: T[], change: WorkspaceRealtimeChange, value: T): T[] {
  if (!value?.id) return rows;
  if (change.operation === "remove") return rows.filter((row) => row.id !== value.id);
  const index = rows.findIndex((row) => row.id === value.id);
  if (index < 0) return [value, ...rows];
  if (rows[index] === value) return rows;
  const next = [...rows];
  next[index] = value;
  return next;
}

function deriveMetrics(workspace: Workspace): Workspace["metrics"] {
  const activeOrders = workspace.manufacturing.filter((order) => order.status !== "Completed" && order.status !== "Cancelled");
  const prior = workspace.metrics || {};
  return {
    ...prior,
    pendingQuotes: workspace.quotations.filter((quote) => quote.status === "Requested" || quote.status === "Priced").length,
    activeOrders: activeOrders.length,
    openShipments: workspace.shipping.length,
    pendingPayments: workspace.payments.filter((payment) => payment.status === "Waiting confirmation").length,
    // Money totals are updated only from /api/accounting/metrics.
    ringingCalls: workspace.calls.filter((call) => call.status === "Ringing").length,
    missedCalls: workspace.calls.filter((call) => call.status === "Missed").length,
    activeCalls: workspace.calls.filter((call) => call.status === "In call").length,
    averageProgress: activeOrders.length
      ? Math.floor(activeOrders.reduce((total, order) => total + order.progress, 0) / activeOrders.length)
      : 0
  };
}
