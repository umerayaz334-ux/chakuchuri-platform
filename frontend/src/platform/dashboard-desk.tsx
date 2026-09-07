import { ArrowUpRight, FileText, MessageSquare, Phone, Wallet, type LucideIcon } from "lucide-react";
import { ProductFileThumb } from "./shared";
import type { Customer, Workspace } from "./types";
import { usePresenceNow } from "./use-presence-now";
import { countOnlineCustomers, date, displayStageLabel, money, orderImageIds, resolveCustomerPresence } from "./utils";

export function AdminCommandHome({
  workspace,
  customers,
  sessionToken,
  onNavigate
}: {
  workspace: Workspace;
  customers: Customer[];
  sessionToken: string;
  onNavigate: (page: string) => void;
}) {
  const now = usePresenceNow();
  const nameOf = (customerId: string) => customers.find((row) => row.id === customerId)?.companyName || customerId || "Customer";
  const quotes = workspace.quotations.filter((quote) => quote.status === "Requested");
  const proofs = workspace.payments.filter((payment) => payment.status === "Waiting confirmation");
  const chats = workspace.conversations.filter((conversation) => (conversation.unreadForAdmin || 0) > 0);
  const liveCalls = workspace.calls.filter((call) => call.status === "Ringing" || call.status === "In call");
  const ringing = liveCalls.filter((call) => call.status === "Ringing");
  const online = customers.filter((customer) => resolveCustomerPresence(workspace.presence, customer, now).online);
  const onlineCount = countOnlineCustomers(workspace.presence, customers, now);
  const production = workspace.manufacturing
    .filter((order) => !["Completed", "Cancelled"].includes(order.status))
    .slice(0, 6);
  const actions: OpsAction[] = [
    ...ringing.map((call) => ({
      key: call.id,
      kind: "Call",
      title: nameOf(call.customerId),
      detail: call.subject || "Incoming audio call",
      meta: "Ringing",
      page: "Calls" as const
    })),
    ...quotes.map((quote) => ({
      key: quote.id,
      kind: "Quote",
      title: quote.productName,
      detail: `${nameOf(quote.customerId)} · ${quote.quantity} pcs`,
      meta: date(quote.createdAt),
      page: "Quotations" as const
    })),
    ...proofs.map((payment) => ({
      key: payment.id,
      kind: "Payment",
      title: money(payment.amount),
      detail: `${nameOf(payment.customerId)} · ${payment.type}`,
      meta: date(payment.createdAt),
      page: "Payments" as const
    })),
    ...chats.map((conversation) => ({
      key: conversation.id,
      kind: "Chat",
      title: nameOf(conversation.customerId),
      detail: conversation.subject || "Support conversation",
      meta: `${conversation.unreadForAdmin} new`,
      page: "Messages" as const
    }))
  ];

  return (
    <section className="cc-ops">
      <div className="cc-ops-kpis">
        <OpsKpi icon={FileText} label="Quotes to price" value={quotes.length} hint="Waiting on the desk" alert={quotes.length > 0} onClick={() => onNavigate("Quotations")} />
        <OpsKpi icon={Wallet} label="Proofs to confirm" value={proofs.length} hint="Bank transfers" alert={proofs.length > 0} onClick={() => onNavigate("Payments")} />
        <OpsKpi icon={MessageSquare} label="Unread chats" value={chats.reduce((total, row) => total + (row.unreadForAdmin || 0), 0)} hint={`${chats.length} conversations`} alert={chats.length > 0} onClick={() => onNavigate("Messages")} />
        <OpsKpi icon={Phone} label="Live calls" value={liveCalls.length} hint={ringing.length ? `${ringing.length} ringing` : "None ringing"} alert={ringing.length > 0} onClick={() => onNavigate("Calls")} />
      </div>

      <div className="cc-ops-layout">
        <article className="cc-ops-card">
          <header>
            <div>
              <span>Now</span>
              <h2>Needs your action</h2>
            </div>
            <em>{actions.length}</em>
          </header>
          {actions.length === 0 ? (
            <div className="cc-ops-empty">
              <strong>Desk is clear</strong>
              <small>No quotes, proofs, chats or ringing calls need you right now.</small>
            </div>
          ) : (
            <div className="cc-ops-list">
              {actions.slice(0, 10).map((item) => (
                <button className="cc-ops-row" key={item.key} onClick={() => onNavigate(item.page)} type="button">
                  <span className={"cc-ops-kind " + item.kind.toLowerCase()}>{item.kind}</span>
                  <strong>{item.title}</strong>
                  <small>{item.detail}</small>
                  <b>{item.meta}</b>
                </button>
              ))}
            </div>
          )}
        </article>

        <div className="cc-ops-side">
          <article className="cc-ops-card">
            <header>
              <div>
                <span>Floor</span>
                <h2>Who is live</h2>
              </div>
              <em>{onlineCount}</em>
            </header>
            {liveCalls.length > 0 && (
              <div className="cc-ops-calls">
                {liveCalls.slice(0, 3).map((call) => (
                  <button className="cc-ops-call" key={call.id} onClick={() => onNavigate("Calls")} type="button">
                    <Phone size={14} />
                    <span>
                      <strong>{nameOf(call.customerId)}</strong>
                      <small>{call.status === "In call" ? "In call" : "Ringing"}</small>
                    </span>
                  </button>
                ))}
              </div>
            )}
            {online.length ? (
              <div className="cc-ops-people">
                {online.slice(0, 8).map((customer) => (
                  <button className="cc-ops-chip" key={customer.id} onClick={() => onNavigate("Customers")} type="button">
                    <i />
                    {customer.companyName}
                  </button>
                ))}
              </div>
            ) : (
              <p className="cc-ops-quiet">No customers online.</p>
            )}
          </article>

          <article className="cc-ops-card">
            <header>
              <div>
                <span>Factory</span>
                <h2>In production</h2>
              </div>
              <button onClick={() => onNavigate("Orders")} type="button">
                All <ArrowUpRight size={14} />
              </button>
            </header>
            {production.length ? (
              <div className="cc-ops-list">
                {production.map((order) => {
                  const imageId = orderImageIds(order, workspace.quotations)[0] || "";
                  return (
                    <button className="cc-ops-row is-progress has-thumb" key={order.id} onClick={() => onNavigate("Orders")} type="button">
                      <ProductFileThumb className="is-ops" fileId={imageId} name={order.productName} token={sessionToken} />
                      <strong>{order.productName}</strong>
                      <small>{nameOf(order.customerId)} · {displayStageLabel(order.currentStage || order.status)}</small>
                      <b>{order.progress}%</b>
                      <span className="cc-ops-bar" aria-hidden>
                        <i style={{ width: `${Math.max(6, Math.min(100, order.progress || 0))}%` }} />
                      </span>
                    </button>
                  );
                })}
              </div>
            ) : (
              <p className="cc-ops-quiet">No open manufacturing orders.</p>
            )}
          </article>
        </div>
      </div>
    </section>
  );
}

type OpsAction = {
  key: string;
  kind: string;
  title: string;
  detail: string;
  meta: string;
  page: "Quotations" | "Payments" | "Messages" | "Calls";
};

function OpsKpi({
  icon: Icon,
  label,
  value,
  hint,
  alert,
  onClick
}: {
  icon: LucideIcon;
  label: string;
  value: number;
  hint: string;
  alert?: boolean;
  onClick: () => void;
}) {
  return (
    <button className={"cc-ops-kpi is-button" + (alert ? " is-alert" : "")} onClick={onClick} type="button">
      <span>
        <Icon size={16} />
        {label}
      </span>
      <em>{value}</em>
      <small>{hint}</small>
    </button>
  );
}
