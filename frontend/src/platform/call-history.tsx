import { useMemo, useState } from "react";
import { Clock3, LoaderCircle, Phone, PhoneIncoming, PhoneMissed, PhoneOutgoing, Search } from "lucide-react";
import { apiPost } from "./api";
import { ProfileAvatar } from "./profile-avatar";
import { Empty } from "./shared";
import type { Call, Customer, Session, Workspace } from "./types";
import { findCustomerPresence, messageFromError, resolveCustomerPresence } from "./utils";

type CallFilter = "All" | "Missed" | "Answered" | "Outgoing";

export function AdminCallHistory({
  customers,
  onAction,
  session,
  workspace
}: {
  customers: Customer[];
  onAction: (message: string) => Promise<void>;
  session: Session;
  workspace: Workspace;
}) {
  const [filter, setFilter] = useState<CallFilter>("All");
  const [search, setSearch] = useState("");
  const [callingId, setCallingId] = useState("");
  const [error, setError] = useState("");
  const customerName = (customerId: string) => customers.find((customer) => customer.id === customerId)?.companyName || customerId;
  const rows = useMemo(() => workspace.calls.filter((call) => {
    const outgoing = call.initiatorRole === "Admin";
    const query = search.trim().toLowerCase();
    if (query && !customerName(call.customerId).toLowerCase().includes(query) && !call.subject.toLowerCase().includes(query)) return false;
    if (filter === "Missed") return call.status === "Missed" || call.status === "Declined";
    if (filter === "Answered") return call.status === "Completed" || call.status === "In call";
    if (filter === "Outgoing") return outgoing;
    return true;
  }), [filter, search, workspace.calls, customers]);
  const activeCustomerIds = new Set(workspace.calls.filter((call) => call.status === "Ringing" || call.status === "In call").map((call) => call.customerId));

  const callBack = async (call: Call) => {
    if (callingId || activeCustomerIds.has(call.customerId)) return;
    const customer = customers.find((row) => row.id === call.customerId);
    if (!resolveCustomerPresence(workspace.presence, customer || { id: call.customerId }).online) {
      setError("Customer is offline.");
      return;
    }
    setCallingId(call.id);
    setError("");
    try {
      await apiPost<Call>(
        "/api/workflow/calls",
        { customerId: call.customerId, conversationId: call.conversationId || "", subject: "Audio call", recipientName: customerName(call.customerId) },
        session.token
      );
      await onAction("Calling " + customerName(call.customerId) + "...");
    } catch (caught) {
      setError(messageFromError(caught, "Call could not be started."));
    } finally {
      setCallingId("");
    }
  };

  const completed = workspace.calls.filter((call) => call.status === "Completed");
  const averageDuration = completed.length ? Math.round(completed.reduce((total, call) => total + (call.durationSeconds || 0), 0) / completed.length) : 0;

  return (
    <section className="cc-call-history-page">
      <div className="cc-call-summary-grid">
        <CallSummary icon={Phone} label="Total calls" value={workspace.calls.length} detail="All call activity" />
        <CallSummary icon={PhoneIncoming} label="Answered" value={completed.length} detail="Picked up successfully" />
        <CallSummary icon={PhoneMissed} label="Missed" value={workspace.calls.filter((call) => call.status === "Missed").length} detail="No answer" tone="danger" />
        <CallSummary icon={Clock3} label="Average duration" value={formatDuration(averageDuration)} detail="Answered calls" />
      </div>

      <div className="cc-call-log-toolbar">
        <div>
          <span>Call log</span>
          <strong>Recent calls</strong>
        </div>
        <label className="cc-call-search">
          <Search size={17} />
          <input aria-label="Search calls" onChange={(event) => setSearch(event.target.value)} placeholder="Search person or subject" value={search} />
        </label>
        <div aria-label="Call filters" className="cc-call-filters" role="tablist">
          {(["All", "Missed", "Answered", "Outgoing"] as CallFilter[]).map((item) => (
            <button aria-selected={filter === item} className={filter === item ? "active" : ""} key={item} onClick={() => setFilter(item)} role="tab" type="button">{item}</button>
          ))}
        </div>
      </div>
      {error && <div className="cc-call-log-error">{error}</div>}

      {rows.length === 0 ? <Empty title="No calls found" detail="Call activity matching this view will appear here." /> : (
        <div className="cc-call-log" role="table" aria-label="Call history">
          <div className="cc-call-log-heading" role="row">
            <span>Person</span><span>Direction</span><span>Outcome</span><span>Time</span><span>Duration</span><span aria-label="Actions" />
          </div>
          {rows.map((call) => {
            const outgoing = call.initiatorRole === "Admin";
            const person = customerName(call.customerId);
            const personAccount = findCustomerPresence(workspace.presence, call.customerId);
            const DirectionIcon = outgoing ? PhoneOutgoing : PhoneIncoming;
            const online = resolveCustomerPresence(workspace.presence, customers.find((row) => row.id === call.customerId) || { id: call.customerId }).online;
            return (
              <article className="cc-call-log-row" key={call.id} role="row">
                <div className="cc-call-person" role="cell">
                  <ProfileAvatar token={session.token} user={{ name: person, profileImageFileId: personAccount?.profileImageFileId }} />
                  <div><strong>{person}</strong><small>{call.subject || "Audio call"}</small></div>
                </div>
                <div className="cc-call-direction" role="cell"><DirectionIcon size={17} /><span>{outgoing ? "Outgoing" : "Incoming"}</span></div>
                <div role="cell"><b className={"cc-call-outcome " + statusClass(call.status)}>{statusLabel(call.status)}</b></div>
                <time role="cell">{formatCallTime(call.updatedAt || call.createdAt)}</time>
                <span role="cell">{call.status === "Completed" || call.status === "In call" ? formatDuration(call.durationSeconds || elapsedSeconds(call.startedAt)) : "-"}</span>
                <button
                  aria-label={"Call " + person}
                  className="cc-call-back-button"
                  disabled={Boolean(callingId) || activeCustomerIds.has(call.customerId) || !online}
                  onClick={() => callBack(call)}
                  title={activeCustomerIds.has(call.customerId) ? "Call already active" : !online ? "Customer is offline" : "Call back"}
                  type="button"
                >
                  {callingId === call.id ? <LoaderCircle className="cc-spin" size={18} /> : <Phone size={18} />}
                </button>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}

function CallSummary({ icon: Icon, label, value, detail, tone = "default" }: { icon: typeof Phone; label: string; value: number | string; detail: string; tone?: "default" | "danger" }) {
  return (
    <article className={"cc-call-summary " + tone}>
      <Icon size={19} />
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{detail}</small>
    </article>
  );
}

function statusLabel(status: string) {
  if (status === "Completed") return "Answered";
  if (status === "In call") return "In call";
  if (status === "Ringing") return "Ringing";
  return status;
}

function statusClass(status: string) {
  return status.toLowerCase().replace(/\s+/g, "-");
}

function formatCallTime(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" });
}

function elapsedSeconds(value?: string) {
  if (!value) return 0;
  const started = Date.parse(value);
  return Number.isFinite(started) ? Math.max(0, Math.floor((Date.now() - started) / 1000)) : 0;
}

function formatDuration(value: number) {
  const seconds = Math.max(0, Math.round(value));
  const minutes = Math.floor(seconds / 60);
  const remainder = seconds % 60;
  return minutes ? `${minutes}m ${String(remainder).padStart(2, "0")}s` : `${remainder}s`;
}

