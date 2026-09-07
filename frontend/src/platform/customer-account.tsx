import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  Activity,
  ArrowLeft,
  CalendarClock,
  ChevronRight,
  CircleDollarSign,
  FileText,
  Mail,
  MapPin,
  MessageSquare,
  PackageCheck,
  Phone,
  PhoneCall,
  Radio,
  Search,
  Truck,
  UserRound,
  UsersRound,
  WalletCards,
  type LucideIcon
} from "lucide-react";
import { apiPost } from "./api";
import { CustomerDocumentsPanel, VerificationBadge, canReviewCustomerKYC } from "./customer-documents";
import { ProfileAvatar } from "./profile-avatar";
import { customerIdFromPath, updateCustomerPath, updatePagePath } from "./routes";
import { Empty } from "./shared";
import type { Call, Customer, Session, Workspace } from "./types";
import { usePresenceNow } from "./use-presence-now";
import { date, findCustomerPresence, money, presenceDetail, resolveCustomerPresence } from "./utils";

type AccountTab = "Overview" | "Documents" | "Orders" | "Shipping" | "Payments";

export function CustomerAccounts({
  customers,
  workspace,
  session,
  onAction,
  onNavigate,
  onCustomerUpdated
}: {
  customers: Customer[];
  workspace: Workspace;
  session: Session;
  onAction: (message: string) => Promise<void>;
  onNavigate: (page: string) => void;
  onCustomerUpdated?: (customer: Customer) => void;
}) {
  const [selectedId, setSelectedId] = useState(() => customerIdFromPath(window.location.pathname));
  const [tab, setTab] = useState<AccountTab>("Overview");
  const [calling, setCalling] = useState(false);
  const [callError, setCallError] = useState("");
  const now = usePresenceNow();
  const canReviewKYC = canReviewCustomerKYC(session);

  useEffect(() => {
    const sync = () => {
      setSelectedId(customerIdFromPath(window.location.pathname));
      const params = new URLSearchParams(window.location.search);
      const nextTab = params.get("tab");
      if (nextTab === "Documents" && canReviewKYC) setTab("Documents");
      else setTab("Overview");
    };
    sync();
    window.addEventListener("popstate", sync);
    window.addEventListener("cc-route-change", sync);
    return () => {
      window.removeEventListener("popstate", sync);
      window.removeEventListener("cc-route-change", sync);
    };
  }, [canReviewKYC]);

  const customer = customers.find((row) => row.id === selectedId);
  const orders = workspace.manufacturing.filter((row) => row.customerId === selectedId);
  const quotes = workspace.quotations.filter((row) => row.customerId === selectedId);
  const shipments = workspace.shipping.filter((row) => row.customerId === selectedId);
  const payments = workspace.payments.filter((row) => row.customerId === selectedId);
  const ledger = workspace.ledger.filter((row) => row.customerId === selectedId);
  const conversation = workspace.conversations.find((row) => row.customerId === selectedId);
  const activeCall = workspace.calls.find(
    (row) => row.customerId === selectedId && (row.status === "Ringing" || row.status === "In call")
  );

  const openAccount = (customerId: string) => {
    setSelectedId(customerId);
    setTab("Overview");
    updateCustomerPath(customerId);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const closeAccount = () => {
    setSelectedId("");
    setTab("Overview");
    updatePagePath("Customers");
  };

  const openMessages = () => {
    if (!customer) return;
    sessionStorage.setItem("cc_message_customer_id", customer.id);
    onNavigate("Messages");
  };

  const startCall = async () => {
    if (!customer || activeCall || calling) return;
    const live = resolveCustomerPresence(workspace.presence, customer);
    if (!live.online) {
      setCallError("Customer is offline.");
      return;
    }
    setCalling(true);
    setCallError("");
    try {
      await apiPost<Call>(
        "/api/workflow/calls",
        {
          customerId: customer.id,
          conversationId: conversation?.id || "",
          subject: "Audio call",
          recipientName: customer.contactName || customer.companyName
        },
        session.token
      );
      await onAction("Calling " + (customer.contactName || customer.companyName) + "...");
    } catch (error) {
      setCallError(error instanceof Error ? error.message : "Call could not be started.");
    } finally {
      setCalling(false);
    }
  };

  if (!selectedId) {
    return <CustomerDirectory customers={customers} onOpen={openAccount} session={session} workspace={workspace} />;
  }

  if (!customer) {
    return (
      <section className="cc-customer-account">
        <button className="cc-account-back" onClick={closeAccount} type="button"><ArrowLeft size={17} /> Customers</button>
        <Empty title="Customer account not found" detail="The account may have been removed or your access changed." />
      </section>
    );
  }

  const orderDue = orders.filter((row) => !["Completed", "Cancelled"].includes(row.status)).reduce((sum, row) => sum + row.balanceDue, 0);
  const ledgerBalance = ledger.reduce((sum, entry) => sum + entry.debit - entry.credit, 0);
  const due = ledger.length > 0 ? Math.max(0, ledgerBalance) : orderDue;
  const advance = ledger.length > 0 ? Math.max(0, -ledgerBalance) : 0;
  const balanceLabel = due > 0 ? "Balance due" : advance > 0 ? "Advance" : "Account clear";
  const balanceAmount = due > 0 ? due : advance;
  const activeOrders = orders.filter((row) => !["Completed", "Cancelled"].includes(row.status)).length;
  const openQuotes = quotes.filter((row) => !["Accepted", "Rejected"].includes(row.status)).length;
  const openShipments = shipments.filter((row) => !["Delivered", "Cancelled"].includes(row.status)).length;
  const person = customer.contactName || customer.companyName;
  const livePresence = findCustomerPresence(workspace.presence, customer.id);
  const profileImageFileId = livePresence?.profileImageFileId;
  const status = resolveCustomerPresence(workspace.presence, customer, now);
  const activity = [
    ...orders.map((row) => ({ id: row.id, type: "Manufacturing", title: row.productName, detail: row.currentStage || row.status, at: row.createdAt })),
    ...quotes.map((row) => ({ id: row.id, type: "Quotation", title: row.productName, detail: row.status, at: row.createdAt })),
    ...shipments.map((row) => ({ id: row.id, type: "Shipping", title: row.destination, detail: row.status, at: row.createdAt })),
    ...payments.map((row) => ({ id: row.id, type: "Payment", title: row.type, detail: row.status, at: row.createdAt }))
  ].sort((a, b) => (b.at || "").localeCompare(a.at || "")).slice(0, 7);

  return (
    <section className="cc-customer-account">
      <header className="cc-account-header">
        <button className="cc-account-back" onClick={closeAccount} type="button"><ArrowLeft size={17} /> Customers</button>
        <div className="cc-account-identity">
          <ProfileAvatar className="cc-account-avatar" token={session.token} user={{ name: customer.companyName, profileImageFileId }} />
          <div>
            <span className="cc-account-eyebrow">Customer account</span>
            <h2>{customer.companyName} <VerificationBadge size="md" status={customer.verificationStatus} /></h2>
            <p><i className={status.online ? "cc-live-dot" : "cc-idle-dot"} />{status.label} / {customer.country}</p>
          </div>
        </div>
        <div className="cc-account-actions">
          <button onClick={openMessages} type="button"><MessageSquare size={17} /> Message</button>
          <button className="primary" disabled={Boolean(activeCall) || calling || !status.online} onClick={startCall} title={activeCall ? activeCall.status : !status.online ? "Customer is offline" : "Call customer"} type="button">
            <PhoneCall size={17} /> {activeCall ? activeCall.status : calling ? "Calling" : !status.online ? "Offline" : "Call"}
          </button>
        </div>
      </header>

      {callError && <div className="cc-account-alert">{callError}</div>}

      <div className="cc-account-kpis">
        <AccountMetric label={balanceLabel} value={money(balanceAmount)} detail={ledger.length ? "From account ledger" : payments.length + " payments recorded"} />
        <AccountMetric label="Active orders" value={String(activeOrders)} detail={orders.length + " manufacturing orders"} />
        <AccountMetric label="Open quotes" value={String(openQuotes)} detail={quotes.length + " total quotations"} />
        <AccountMetric label="Open shipments" value={String(openShipments)} detail={shipments.length + " total shipments"} />
      </div>

      <nav aria-label="Customer account views" className="cc-account-tabs">
        {(["Overview", ...(canReviewKYC ? ["Documents"] : []), "Orders", "Shipping", "Payments"] as AccountTab[]).map((item) => (
          <button aria-current={tab === item ? "page" : undefined} className={tab === item ? "active" : ""} key={item} onClick={() => setTab(item)} type="button">{item}</button>
        ))}
      </nav>

      {tab === "Documents" && canReviewKYC && (
        <CustomerDocumentsPanel
          customer={customer}
          mode="admin"
          onAction={onAction}
          onUpdated={(next) => onCustomerUpdated?.(next)}
          session={session}
        />
      )}

      {tab === "Overview" && (
        <div className="cc-account-columns">
          <AccountSection icon={UserRound} label="Profile" title="Account details">
            <div className="cc-account-facts">
              <AccountFact icon={UserRound} label="Contact" value={person} />
              <AccountFact icon={Mail} label="Email" value={customer.email || "Not provided"} />
              <AccountFact icon={Phone} label="Phone" value={customer.phone || "Not provided"} />
              <AccountFact icon={MapPin} label="Country" value={customer.country || "Not provided"} />
            </div>
            <div className="cc-account-services">
              <span>Enabled services</span>
              <div>{customer.services?.length ? customer.services.map((service) => <b key={service}>{service}</b>) : <small>No services selected</small>}</div>
            </div>
          </AccountSection>
          <AccountSection icon={CalendarClock} label="Timeline" title="Recent activity">
            {activity.length ? <div className="cc-account-activity">{activity.map((item) => (
              <article key={item.type + item.id}><span /><div><strong>{item.title}</strong><small>{item.type} / {item.detail}</small></div><time>{date(item.at)}</time></article>
            ))}</div> : <Empty title="No account activity" detail="Quotes, orders, shipments and payments will appear here." />}
          </AccountSection>
        </div>
      )}

      {tab === "Orders" && (
        <div className="cc-account-columns">
          <AccountSection icon={PackageCheck} label="Production" title="Manufacturing orders">
            <AccountRows emptyTitle="No manufacturing orders" emptyDetail="Accepted quotations will appear here.">
              {orders.map((order) => <AccountRow key={order.id} title={order.productName} detail={order.id + " / " + order.quantity + " pcs / " + order.currentStage} status={order.progress + "%"} amount={money(order.balanceDue)} />)}
            </AccountRows>
          </AccountSection>
          <AccountSection icon={FileText} label="Pricing" title="Quotations">
            <AccountRows emptyTitle="No quotations" emptyDetail="Customer quote requests will appear here.">
              {quotes.map((quote) => <AccountRow key={quote.id} title={quote.productName} detail={quote.id + " / " + quote.quantity + " pcs"} status={quote.status} amount={quote.totalAmount ? money(quote.totalAmount) : "Pricing pending"} />)}
            </AccountRows>
          </AccountSection>
        </div>
      )}

      {tab === "Shipping" && (
        <AccountSection icon={Truck} label="Logistics" title="Shipment history">
          <AccountRows emptyTitle="No shipments" emptyDetail="Manufactured and outside-product shipments will appear here.">
            {shipments.map((shipment) => <AccountRow key={shipment.id} title={shipment.destination} detail={shipment.courier + " / " + shipment.service + " / " + (shipment.tracking || "Tracking pending")} status={shipment.status} amount={shipment.quotedAmount ? money(shipment.quotedAmount) : "Rate pending"} />)}
          </AccountRows>
        </AccountSection>
      )}

      {tab === "Payments" && (
        <div className="cc-account-columns">
          <AccountSection icon={WalletCards} label="Payments" title="Payment records">
            <AccountRows emptyTitle="No payments" emptyDetail="Proof uploads and confirmations will appear here.">
              {payments.map((payment) => <AccountRow key={payment.id} title={payment.type} detail={(payment.proofName || "No proof") + " / " + date(payment.createdAt)} status={payment.status} amount={money(payment.amount)} />)}
            </AccountRows>
          </AccountSection>
          <AccountSection icon={WalletCards} label="Ledger" title="Account statement">
            <AccountRows emptyTitle="No ledger entries" emptyDetail="Charges and confirmed payments will post here.">
              {ledger.map((entry) => <AccountRow key={entry.id} title={entry.note} detail={entry.entryNo + " / " + date(entry.postedAt)} status={entry.sourceType} amount={entry.debit ? money(entry.debit) : "+" + money(entry.credit)} />)}
            </AccountRows>
          </AccountSection>
        </div>
      )}
    </section>
  );
}

type DirectoryMode = "All accounts" | "Online now" | "Balance due" | "Active work";
type DirectorySort = "Recent activity" | "Company A-Z" | "Highest balance";

function CustomerDirectory({ customers, workspace, session, onOpen }: { customers: Customer[]; workspace: Workspace; session: Session; onOpen: (customerId: string) => void }) {
  const now = usePresenceNow();
  const [search, setSearch] = useState("");
  const [mode, setMode] = useState<DirectoryMode>("All accounts");
  const [service, setService] = useState("All services");
  const [sort, setSort] = useState<DirectorySort>("Recent activity");

  const rows = useMemo(() => customers.map((customer) => {
    const orders = workspace.manufacturing.filter((row) => row.customerId === customer.id);
    const quotes = workspace.quotations.filter((row) => row.customerId === customer.id);
    const shipments = workspace.shipping.filter((row) => row.customerId === customer.id);
    const payments = workspace.payments.filter((row) => row.customerId === customer.id);
    const customerLedger = workspace.ledger.filter((row) => row.customerId === customer.id);
    const ledgerBalance = customerLedger.reduce((sum, row) => sum + row.debit - row.credit, 0);
    const orderBalance = orders.filter((row) => !["Completed", "Cancelled"].includes(row.status)).reduce((sum, row) => sum + row.balanceDue, 0);
    const balance = customerLedger.length > 0 ? Math.max(ledgerBalance, 0) : orderBalance;
    const advance = customerLedger.length > 0 ? Math.max(-ledgerBalance, 0) : 0;
    const activeOrders = orders.filter((row) => !["Completed", "Cancelled"].includes(row.status)).length;
    const openQuotes = quotes.filter((row) => !["Accepted", "Rejected"].includes(row.status)).length;
    const openShipments = shipments.filter((row) => !["Delivered", "Cancelled"].includes(row.status)).length;
    const lastActivity = [
      ...orders.map((row) => row.createdAt),
      ...quotes.map((row) => row.createdAt),
      ...shipments.map((row) => row.createdAt),
      ...payments.map((row) => row.createdAt)
    ].filter(Boolean).sort((a, b) => b.localeCompare(a))[0] || "";
    const livePresence = findCustomerPresence(workspace.presence, customer.id);
    return { customer, balance, advance, activeOrders, openQuotes, openShipments, activeWork: activeOrders + openQuotes + openShipments, lastActivity, profileImageFileId: livePresence?.profileImageFileId };
  }), [customers, workspace.ledger, workspace.manufacturing, workspace.payments, workspace.presence, workspace.quotations, workspace.shipping]);

  const services = useMemo(() => Array.from(new Set(customers.flatMap((customer) => customer.services || []))).sort(), [customers]);
  const filteredRows = useMemo(() => {
    const query = search.trim().toLowerCase();
    return rows.filter((row) => {
      const status = resolveCustomerPresence(workspace.presence, row.customer, now);
      const matchesSearch = !query || [row.customer.companyName, row.customer.contactName, row.customer.email, row.customer.phone, row.customer.country, ...(row.customer.services || [])]
        .some((value) => value.toLowerCase().includes(query));
      const matchesMode = mode === "All accounts" ||
        (mode === "Online now" && status.online) ||
        (mode === "Balance due" && row.balance > 0) ||
        (mode === "Active work" && row.activeWork > 0);
      const matchesService = service === "All services" || row.customer.services?.includes(service);
      return matchesSearch && matchesMode && matchesService;
    }).sort((left, right) => {
      if (sort === "Company A-Z") return left.customer.companyName.localeCompare(right.customer.companyName);
      if (sort === "Highest balance") return right.balance - left.balance;
      return (right.lastActivity || "").localeCompare(left.lastActivity || "") || left.customer.companyName.localeCompare(right.customer.companyName);
    });
  }, [mode, now, rows, search, service, sort, workspace.presence]);

  const receivables = rows.reduce((sum, row) => sum + row.balance, 0);
  const activeWork = rows.reduce((sum, row) => sum + row.activeWork, 0);
  const online = rows.filter((row) => resolveCustomerPresence(workspace.presence, row.customer, now).online).length;

  return (
    <section className="cc-customer-directory">
      <header className="cc-directory-heading">
        <div>
          <span>Customer operations</span>
          <h2>Account directory</h2>
          <p>Contacts, services, balances and live work in one view.</p>
        </div>
        <div aria-label="Customer account summary" className="cc-directory-stats">
          <DirectoryStat icon={UsersRound} label="Accounts" value={String(rows.length)} />
          <DirectoryStat icon={Radio} label="Online" value={String(online)} tone="live" />
          <DirectoryStat icon={Activity} label="Open work" value={String(activeWork)} />
          <DirectoryStat icon={CircleDollarSign} label="Receivables" value={money(receivables)} />
        </div>
      </header>

      <div className="cc-directory-toolbar">
        <label className="cc-directory-search">
          <Search size={17} />
          <input aria-label="Search customer accounts" onChange={(event) => setSearch(event.target.value)} placeholder="Search company, contact, email or country" value={search} />
        </label>
        <div aria-label="Account filter" className="cc-directory-segments">
          {(["All accounts", "Online now", "Balance due", "Active work"] as DirectoryMode[]).map((item) => (
            <button aria-pressed={mode === item} className={mode === item ? "active" : ""} key={item} onClick={() => setMode(item)} type="button">{item}</button>
          ))}
        </div>
        <label className="cc-directory-select"><span>Service</span><select aria-label="Filter by service" onChange={(event) => setService(event.target.value)} value={service}><option>All services</option>{services.map((item) => <option key={item}>{item}</option>)}</select></label>
        <label className="cc-directory-select"><span>Sort</span><select aria-label="Sort customers" onChange={(event) => setSort(event.target.value as DirectorySort)} value={sort}><option>Recent activity</option><option>Company A-Z</option><option>Highest balance</option></select></label>
      </div>

      <div className="cc-directory-result-line"><span>{filteredRows.length} {filteredRows.length === 1 ? "account" : "accounts"}</span><small>{search || mode !== "All accounts" || service !== "All services" ? "Filtered view" : "All customer relationships"}</small></div>
      {filteredRows.length ? (
        <div className="cc-directory-table">
          <div aria-hidden="true" className="cc-directory-table-head"><span>Customer</span><span>Services</span><span>Presence</span><span>Account</span><span /></div>
          {filteredRows.map((row) => {
            const status = resolveCustomerPresence(workspace.presence, row.customer, now);
            return (
            <button className="cc-directory-row" key={row.customer.id} onClick={() => onOpen(row.customer.id)} title={`Open ${row.customer.companyName}`} type="button">
              <span className="cc-directory-customer">
                <i className="cc-directory-avatar"><ProfileAvatar token={session.token} user={{ name: row.customer.companyName, profileImageFileId: row.profileImageFileId }} /><b className={status.online ? "online" : ""} /></i>
                <span><strong>{row.customer.companyName}<VerificationBadge compact size="sm" status={row.customer.verificationStatus} /></strong><small>{row.customer.contactName || "Contact not added"} / {row.customer.country || "Country not added"}</small><em>{row.customer.email || row.customer.phone || "Contact details pending"}</em></span>
              </span>
              <span className="cc-directory-services">{row.customer.services?.length ? row.customer.services.slice(0, 2).map((item) => <b key={item}>{item}</b>) : <small>No services</small>}{(row.customer.services?.length || 0) > 2 && <em>+{row.customer.services.length - 2}</em>}</span>
              <span className="cc-directory-activity"><strong>{status.label}</strong><small>{presenceDetail(status)}</small></span>
              <span className="cc-directory-account"><strong>{money(row.balance > 0 ? row.balance : row.advance)}</strong><small>{row.balance > 0 ? "Due" : row.advance > 0 ? "Advance" : "Settled"} / {row.activeOrders} orders / {row.openShipments} shipments / {row.openQuotes} quotes</small></span>
              <span aria-hidden="true" className="cc-directory-row-action"><ChevronRight size={18} /></span>
            </button>
            );
          })}
        </div>
      ) : <div className="cc-directory-empty"><Empty title="No customer accounts match" detail="Try a different search or account filter." /></div>}
    </section>
  );
}

function DirectoryStat({ icon: Icon, label, value, tone = "" }: { icon: LucideIcon; label: string; value: string; tone?: string }) {
  return <div className={tone}><Icon size={16} /><span><small>{label}</small><strong>{value}</strong></span></div>;
}
function AccountMetric({ label, value, detail }: { label: string; value: string; detail: string }) {
  return <article><span>{label}</span><strong>{value}</strong><small>{detail}</small></article>;
}

function AccountFact({ icon: Icon, label, value }: { icon: LucideIcon; label: string; value: string }) {
  return <div><Icon size={17} /><span>{label}</span><strong>{value}</strong></div>;
}

function AccountSection({ icon: Icon, label, title, children }: { icon: LucideIcon; label: string; title: string; children: ReactNode }) {
  return <section className="cc-account-band"><header><div><span>{label}</span><h3>{title}</h3></div><Icon size={20} /></header>{children}</section>;
}

function AccountRows({ children, emptyTitle, emptyDetail }: { children: ReactNode[]; emptyTitle: string; emptyDetail: string }) {
  return <div className="cc-account-list">{children.length ? children : <Empty title={emptyTitle} detail={emptyDetail} />}</div>;
}

function AccountRow({ title, detail, status, amount }: { title: string; detail: string; status: string; amount: string }) {
  return <article className="cc-account-row"><div><strong>{title}</strong><small>{detail}</small></div><span>{status}</span><b>{amount}</b></article>;
}

