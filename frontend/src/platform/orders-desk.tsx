import { type FormEvent, useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  Ban,
  CalendarClock,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  CircleDollarSign,
  ClipboardList,
  Clock3,
  LoaderCircle,
  MessageSquare,
  PackageCheck,
  Pencil,
  Search,
  SendHorizontal,
  UserRound,
  WalletCards,
  X
} from "lucide-react";
import { apiPost } from "./api";
import type { Customer, Order, Session, Stage } from "./types";
import { date, displayStageLabel, jobCode, linkedQuoteCode, messageFromError, money, nextStage, stashQuoteSelection, takeStoredSelection } from "./utils";

type OrderFilter = "All" | "Active" | "Ready" | "Completed" | "Cancelled";
type OrderDialogMode = "edit" | "cancel";

type OrdersDeskProps = {
  mode: "admin" | "customer";
  orders: Order[];
  customers?: Customer[];
  session: Session;
  onAction: (message: string) => Promise<void>;
  onNavigate: (page: string) => void;
};

const stageOptions = ["Confirmed", "Production", "Quality check", "Ready for delivery", "Completed"];

function useCompactOrdersLayout() {
  const [compact, setCompact] = useState(() => typeof window !== "undefined" && window.matchMedia("(max-width: 820px)").matches);
  useEffect(() => {
    const media = window.matchMedia("(max-width: 820px)");
    const sync = () => setCompact(media.matches);
    sync();
    media.addEventListener("change", sync);
    return () => media.removeEventListener("change", sync);
  }, []);
  return compact;
}

export function OrdersDesk({ mode, orders, customers = [], session, onAction, onNavigate }: OrdersDeskProps) {
  const admin = mode === "admin";
  const compact = useCompactOrdersLayout();
  const [selectedId, setSelectedId] = useState(orders[0]?.id || "");
  const [filter, setFilter] = useState<OrderFilter>("All");
  const [search, setSearch] = useState("");

  const customerNames = useMemo(
    () => new Map(customers.map((customer) => [customer.id, customer.companyName])),
    [customers]
  );

  useEffect(() => {
    const stored = takeStoredSelection("cc_select_order_id");
    if (stored && orders.some((order) => order.id === stored)) {
      setSelectedId(stored);
    }
  }, [orders]);
  const sortedOrders = useMemo(() => [...orders].sort(compareOrders), [orders]);
  const visibleOrders = useMemo(() => {
    const query = search.trim().toLowerCase();
    return sortedOrders.filter((order) => {
      if (!matchesFilter(order, filter)) return false;
      if (!query) return true;
      return [order.id, order.productName, order.status, order.currentStage, customerNames.get(order.customerId) || ""]
        .some((value) => value.toLowerCase().includes(query));
    });
  }, [customerNames, filter, search, sortedOrders]);

  useEffect(() => {
    if (compact) return;
    if (selectedId && visibleOrders.some((order) => order.id === selectedId)) return;
    setSelectedId(visibleOrders[0]?.id || "");
  }, [compact, selectedId, visibleOrders]);

  const selected = visibleOrders.find((order) => order.id === selectedId);
  const active = orders.filter((order) => isActive(order)).length;
  const ready = orders.filter((order) => order.status.toLowerCase().includes("ready")).length;
  const overdue = orders.filter(isOverdue).length;
  const balance = orders.filter((order) => isActive(order)).reduce((total, order) => total + Math.max(order.balanceDue, 0), 0);

  const selectOrder = (orderId: string) => {
    if (compact) {
      setSelectedId((current) => (current === orderId ? "" : orderId));
      return;
    }
    setSelectedId(orderId);
  };

  if (orders.length === 0) {
    return (
      <section className="cc-orders-empty">
        <PackageCheck size={28} />
        <h2>No orders yet</h2>
        <p>Accepted quotations will become production orders here.</p>
        {!admin && <button className="cc-button cc-primary" onClick={() => onNavigate("Get Quote")} type="button">Request a quote</button>}
      </section>
    );
  }

  return (
    <section className="cc-orders-desk">
      <div className="cc-orders-summary" aria-label="Order summary">
        <OrderMetric icon={ClipboardList} label="Active" value={String(active)} detail="In production" />
        <OrderMetric icon={PackageCheck} label="Ready" value={String(ready)} detail="Ready to dispatch" />
        <OrderMetric icon={CalendarClock} label="Overdue" value={String(overdue)} detail={overdue ? "Needs attention" : "On schedule"} warning={overdue > 0} />
        <OrderMetric icon={CircleDollarSign} label="Balance due" value={money(balance)} detail="Across all orders" />
      </div>

      <div className={`cc-orders-workspace ${compact ? "is-compact" : ""}`}>
        <aside className="cc-orders-index">
          <header>
            <div>
              <span className="cc-kicker">Order book</span>
              <strong>{visibleOrders.length} {visibleOrders.length === 1 ? "order" : "orders"}</strong>
            </div>
          </header>

          <label className="cc-orders-search">
            <Search aria-hidden="true" size={17} />
            <input aria-label="Search orders" onChange={(event) => setSearch(event.target.value)} placeholder="Search ID, product or customer" value={search} />
          </label>

          <div className="cc-orders-filters" role="group" aria-label="Filter orders">
            {(["All", "Active", "Ready", "Completed", "Cancelled"] as OrderFilter[]).map((item) => (
              <button aria-pressed={filter === item} className={filter === item ? "active" : ""} key={item} onClick={() => setFilter(item)} type="button">{item}</button>
            ))}
          </div>

          <div className="cc-order-list">
            {visibleOrders.length ? visibleOrders.map((order) => {
              const open = selected?.id === order.id;
              return (
                <div className={`cc-order-accordion ${open ? "is-open" : ""}`} key={order.id}>
                  <button
                    aria-current={open ? "true" : undefined}
                    aria-expanded={compact ? open : undefined}
                    className={`cc-order-list-item ${open ? "selected" : ""}`}
                    onClick={() => selectOrder(order.id)}
                    type="button"
                  >
                    <span className={`cc-order-state-dot ${statusTone(order)}`} aria-hidden="true" />
                    <span className="cc-order-list-copy">
                      <span><strong>{order.productName}</strong><em>{jobCode(order)}</em></span>
                      {admin && <small>{customerNames.get(order.customerId) || order.customerId}</small>}
                      {linkedQuoteCode(order) && <small>Quote {linkedQuoteCode(order)}</small>}
                      <span className="cc-order-mini-progress"><i style={{ width: `${clampProgress(order.progress)}%` }} /></span>
                      <span className="cc-order-list-meta"><small>{displayStageLabel(order.currentStage || order.status)}</small><b>{deadlineLabel(order.expectedDate)}</b></span>
                    </span>
                    {compact ? (
                      <ChevronDown aria-hidden="true" className={open ? "is-open" : ""} size={16} />
                    ) : (
                      <ChevronRight aria-hidden="true" size={16} />
                    )}
                  </button>
                  {compact && open && (
                    <div className="cc-order-inline-detail">
                      <OrderDetail
                        admin={admin}
                        customerName={customerNames.get(order.customerId) || order.customerId}
                        onAction={onAction}
                        onNavigate={onNavigate}
                        order={order}
                        session={session}
                      />
                    </div>
                  )}
                </div>
              );
            }) : (
              <div className="cc-order-list-empty">No orders match this filter.</div>
            )}
          </div>
        </aside>

        {!compact && (
          <div className="cc-order-detail">
            {selected ? (
              <OrderDetail
                admin={admin}
                customerName={customerNames.get(selected.customerId) || selected.customerId}
                key={selected.id}
                onAction={onAction}
                onNavigate={onNavigate}
                order={selected}
                session={session}
              />
            ) : (
              <div className="cc-order-detail-empty"><PackageCheck size={24} /><strong>Select an order</strong></div>
            )}
          </div>
        )}
      </div>
    </section>
  );
}

function OrderDetail({ admin, customerName, order, session, onAction, onNavigate }: {
  admin: boolean;
  customerName: string;
  order: Order;
  session: Session;
  onAction: (message: string) => Promise<void>;
  onNavigate: (page: string) => void;
}) {
  const [stage, setStage] = useState(order.status === "Deposit pending" ? order.status : order.status);
  const [note, setNote] = useState("");
  const [expectedDate, setExpectedDate] = useState(order.expectedDate || "");
  const [busy, setBusy] = useState<"stage" | "update" | "">("");
  const [error, setError] = useState("");
  const [showAll, setShowAll] = useState(false);
  const [dialog, setDialog] = useState<OrderDialogMode | "">("");
  const upcoming = nextStage(order.status);
  const closed = !isActive(order);

  useEffect(() => {
    setStage(order.status === "Deposit pending" ? "Deposit pending" : order.status);
    setExpectedDate(order.expectedDate || "");
  }, [order.expectedDate, order.status]);
  const paidPercent = order.totalAmount > 0 ? Math.min(100, Math.round((order.paidAmount / order.totalAmount) * 100)) : 0;

  const openMessages = () => {
    if (admin) sessionStorage.setItem("cc_message_customer_id", order.customerId);
    onNavigate("Messages");
  };

  const updateStage = async (target = stage) => {
    if (!target || target === order.status || busy) return;
    setBusy("stage");
    setError("");
    try {
      await apiPost<Order>(`/api/workflow/manufacturing/${order.id}/stage`, { stage: target }, session.token);
      await onAction(`Order moved to ${target}.`);
    } catch (cause) {
      setError(messageFromError(cause, "The order stage could not be updated."));
    } finally {
      setBusy("");
    }
  };

  const submitUpdate = async (event: FormEvent) => {
    event.preventDefault();
    if ((!note.trim() && expectedDate === order.expectedDate) || busy) return;
    setBusy("update");
    setError("");
    try {
      await apiPost<Order>(`/api/workflow/manufacturing/${order.id}/update`, { detail: note.trim(), expectedDate }, session.token);
      setNote("");
      await onAction("Production update published.");
    } catch (cause) {
      setError(messageFromError(cause, "The production update could not be published."));
    } finally {
      setBusy("");
    }
  };

  const openLinkedQuote = () => {
    const quoteId = (order.quotationId || order.id || "").trim();
    if (!quoteId) return;
    stashQuoteSelection(quoteId);
    onNavigate(admin ? "Quotations" : "Get Quote");
  };

  const history = showAll ? order.history : order.history.slice(0, 5);

  return (
    <>
      <header className="cc-order-detail-head">
        <div className="cc-order-detail-title">
          <span className="cc-kicker">{jobCode(order)}</span>
          <h2>{order.productName}</h2>
          <p>{admin ? customerName : "Production order"} / {order.quantity.toLocaleString()} pieces</p>
          {order.quotationId ? (
            <button className="cc-order-quote-link" onClick={openLinkedQuote} type="button">
              {linkedQuoteCode(order) ? `Linked quote ${linkedQuoteCode(order)}` : "View quotation"}
            </button>
          ) : null}
        </div>
        <span className={`cc-order-status ${statusTone(order)}`}>{order.status}</span>
        <div className="cc-order-detail-actions">
          <button className="cc-icon-button" aria-label="Open messages" onClick={openMessages} title="Open messages" type="button"><MessageSquare size={18} /></button>
          <button className="cc-icon-button" aria-label="Open payments" onClick={() => onNavigate("Payments")} title="Open payments" type="button"><WalletCards size={18} /></button>
          {admin && !closed && <button className="cc-icon-button" aria-label="Edit accepted order" onClick={() => setDialog("edit")} title="Edit order" type="button"><Pencil size={17} /></button>}
          {admin && !closed && <button className="cc-icon-button cc-order-cancel-trigger" aria-label="Cancel accepted order" onClick={() => setDialog("cancel")} title="Cancel order" type="button"><Ban size={17} /></button>}
        </div>
      </header>

      <div className="cc-order-progress-overview">
        <div className="cc-order-progress-label"><strong>{clampProgress(order.progress)}% complete</strong><span>{displayStageLabel(order.currentStage || order.status)}</span></div>
        <div className="cc-order-progress-track"><i style={{ width: `${clampProgress(order.progress)}%` }} /></div>
      </div>

      <div className="cc-order-facts">
        <OrderFact icon={CalendarClock} label="Expected completion" value={order.expectedDate ? date(order.expectedDate) : "Not scheduled"} detail={deadlineLabel(order.expectedDate)} tone={isOverdue(order) ? "warning" : ""} />
        <OrderFact icon={CircleDollarSign} label="Order value" value={money(order.totalAmount)} detail={`${paidPercent}% paid`} />
        <OrderFact icon={CheckCircle2} label="Paid" value={money(order.paidAmount)} detail={order.paidAmount > 0 ? "Confirmed receipts" : "No payment confirmed"} />
        <OrderFact icon={WalletCards} label="Balance due" value={money(order.balanceDue)} detail={order.status === "Cancelled" ? (order.paidAmount > 0 ? "Cancelled · credit on account" : "Cancelled") : order.balanceDue > 0 ? "Payment outstanding" : "Fully paid"} tone={order.status === "Cancelled" ? "" : order.balanceDue > 0 ? "warning" : "success"} />
      </div>

      <section className="cc-order-timeline-section">
        <header><div><span className="cc-kicker">Production</span><h3>Order timeline</h3></div><span>{completedStageCount(order.stages)} of {order.stages.length} milestones</span></header>
        <OrderTimeline stages={order.stages} />
      </section>

      {admin && !closed && (
        <section className="cc-order-admin-controls">
          {order.status === "Deposit pending" ? (
            <div className="cc-order-stage-control">
              <div><span className="cc-kicker">Workflow</span><h3>Awaiting deposit</h3></div>
              <p className="cc-order-deposit-hint">Production stages unlock after confirmed payments cover the {money(order.depositRequired)} deposit. Customers just upload amount + screenshot; allocation is automatic.</p>
            </div>
          ) : (
          <div className="cc-order-stage-control">
            <div><span className="cc-kicker">Workflow</span><h3>Update production stage</h3></div>
            <div>
              <select aria-label="Production stage" onChange={(event) => setStage(event.target.value)} value={stageOptions.includes(stage) ? stage : stageOptions[0]}>
                {stageOptions.map((item) => <option key={item}>{item}</option>)}
              </select>
              <button className="cc-button cc-primary" disabled={busy !== "" || stage === order.status} onClick={() => void updateStage()} type="button">
                {busy === "stage" ? "Updating..." : "Update stage"}
              </button>
              {upcoming && upcoming !== stage && (
                <button className="cc-button" disabled={busy !== ""} onClick={() => { setStage(upcoming); void updateStage(upcoming); }} type="button">Advance to {upcoming}</button>
              )}
            </div>
          </div>
          )}

          <form className="cc-order-update-form" onSubmit={submitUpdate}>
            <div><span className="cc-kicker">Customer update</span><h3>Publish production note</h3></div>
            <label><span>Expected date</span><input onChange={(event) => setExpectedDate(event.target.value)} type="date" value={expectedDate} /></label>
            <label className="cc-order-note-field"><span>Update</span><textarea onChange={(event) => setNote(event.target.value)} placeholder="Add a concise production update for the customer" rows={2} value={note} /></label>
            <button aria-label="Publish production update" className="cc-icon-button cc-order-send-update" disabled={busy !== "" || (!note.trim() && expectedDate === order.expectedDate)} title="Publish update" type="submit"><SendHorizontal size={18} /></button>
          </form>
          {error && <div className="cc-order-error">{error}</div>}
        </section>
      )}

      <section className="cc-order-activity">
        <header><div><span className="cc-kicker">Activity</span><h3>Order updates</h3></div>{order.history.length > 5 && <button onClick={() => setShowAll((current) => !current)} type="button">{showAll ? "Show less" : `View all ${order.history.length}`}</button>}</header>
        {history.length ? <div className="cc-order-activity-list">{history.map((update, index) => (
          <article key={`${update.createdAt}-${index}`}>
            <span className="cc-order-activity-icon">{index === 0 ? <Clock3 size={15} /> : <Check size={14} />}</span>
            <div><strong>{update.label}</strong><p>{update.detail}</p><small><UserRound size={12} /> {update.actor || "ChakuChuri"}</small></div>
            <time>{dateTime(update.createdAt)}</time>
          </article>
        ))}</div> : <p className="cc-order-no-activity">No production updates yet.</p>}
      </section>

      {dialog && <OrderChangeDialog customerName={customerName} mode={dialog} onAction={onAction} onClose={() => setDialog("")} order={order} session={session} />}
    </>
  );
}

function OrderChangeDialog({ mode, order, customerName, session, onAction, onClose }: {
  mode: OrderDialogMode;
  order: Order;
  customerName: string;
  session: Session;
  onAction: (message: string) => Promise<void>;
  onClose: () => void;
}) {
  const [step, setStep] = useState<"details" | "confirm">("details");
  const [productName, setProductName] = useState(order.productName);
  const [quantity, setQuantity] = useState(String(order.quantity));
  const [unitPrice, setUnitPrice] = useState(String(order.quantity > 0 ? Math.round(order.totalAmount / order.quantity) : 0));
  const [expectedDate, setExpectedDate] = useState(order.expectedDate || "");
  const [reason, setReason] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const quantityNumber = Number(quantity);
  const unitPriceNumber = Number(unitPrice);
  const totalAmount = quantityNumber > 0 && unitPriceNumber > 0 ? quantityNumber * unitPriceNumber : 0;
  const difference = totalAmount - order.totalAmount;

  useEffect(() => {
    const previousOverflow = document.body.style.overflow;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busy) onClose();
    };
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [busy, onClose]);

  const close = () => {
    if (!busy) onClose();
  };

  const review = () => {
    setError("");
    if (!reason.trim()) {
      setError("Add the reason for this change. It will be saved in the order audit trail.");
      return;
    }
    if (mode === "edit") {
      if (!productName.trim() || !Number.isInteger(quantityNumber) || quantityNumber < 1 || !Number.isInteger(unitPriceNumber) || unitPriceNumber < 1 || !expectedDate) {
        setError("Complete the product name, quantity, unit price and expected date.");
        return;
      }
      if (productName.trim() === order.productName && quantityNumber === order.quantity && totalAmount === order.totalAmount && expectedDate === order.expectedDate) {
        setError("Change at least one order detail before continuing.");
        return;
      }
    }
    setStep("confirm");
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (step === "details") {
      review();
      return;
    }
    if (confirmation.trim() !== order.id) {
      setError(`Type ${order.id} exactly to confirm.`);
      return;
    }
    setBusy(true);
    setError("");
    try {
      if (mode === "edit") {
        await apiPost<Order>(`/api/workflow/manufacturing/${order.id}/edit`, {
          productName: productName.trim(),
          quantity: quantityNumber,
          unitPrice: unitPriceNumber,
          expectedDate,
          reason: reason.trim(),
          confirmation: confirmation.trim()
        }, session.token);
        await onAction("Order updated. The customer can see the revised details now.");
      } else {
        await apiPost<Order>(`/api/workflow/manufacturing/${order.id}/cancel`, {
          reason: reason.trim(),
          confirmation: confirmation.trim()
        }, session.token);
        await onAction("Order cancelled and its accounting charge reversed.");
      }
      onClose();
    } catch (cause) {
      setError(messageFromError(cause, mode === "edit" ? "The order could not be updated." : "The order could not be cancelled."));
    } finally {
      setBusy(false);
    }
  };

  const editMode = mode === "edit";
  return (
    <div className="cc-order-dialog-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) close(); }}>
      <section aria-describedby="cc-order-dialog-description" aria-labelledby="cc-order-dialog-title" aria-modal="true" className={`cc-order-dialog ${editMode ? "edit" : "cancel"}`} role="dialog">
        <header className="cc-order-dialog-head">
          <span className="cc-order-dialog-icon">{editMode ? <Pencil size={19} /> : <Ban size={19} />}</span>
          <div><span className="cc-kicker">{step === "details" ? "Step 1 of 2" : "Step 2 of 2"}</span><h2 id="cc-order-dialog-title">{editMode ? "Edit accepted order" : "Cancel order"}</h2><p id="cc-order-dialog-description">{order.id} / {customerName}</p></div>
          <button aria-label="Close" className="cc-icon-button" disabled={busy} onClick={close} title="Close" type="button"><X size={18} /></button>
        </header>

        <form onSubmit={submit}>
          <div className="cc-order-dialog-body">
            {step === "details" ? (
              editMode ? <>
                <div className="cc-order-dialog-fields">
                  <label className="wide"><span>Product name</span><input autoFocus onChange={(event) => setProductName(event.target.value)} value={productName} /></label>
                  <label><span>Quantity</span><input inputMode="numeric" onChange={(event) => setQuantity(numbersOnly(event.target.value))} value={quantity} /></label>
                  <label><span>Price per piece</span><span className="cc-order-money-field"><b>Rs</b><input inputMode="numeric" onChange={(event) => setUnitPrice(numbersOnly(event.target.value))} value={unitPrice} /></span></label>
                  <label><span>Expected completion</span><input onChange={(event) => setExpectedDate(event.target.value)} type="date" value={expectedDate} /></label>
                  <label className="wide"><span>Reason for change</span><textarea onChange={(event) => setReason(event.target.value)} placeholder="Explain what the customer requested and who approved it" rows={3} value={reason} /></label>
                </div>
                <div className="cc-order-calculation"><span>{quantityNumber > 0 ? quantityNumber.toLocaleString() : "0"} pieces x {money(unitPriceNumber || 0)}</span><strong>{money(totalAmount)}</strong><em className={difference < 0 ? "credit" : difference > 0 ? "debit" : ""}>{difference === 0 ? "No value change" : `${difference > 0 ? "+" : ""}${money(difference)} ledger adjustment`}</em></div>
              </> : <>
                <div className="cc-order-cancel-warning"><AlertTriangle size={22} /><div><strong>This closes the order for both sides</strong><p>Production controls will lock, the order charge will be reversed, and the customer will see the cancellation reason immediately.</p></div></div>
                <div className="cc-order-cancel-summary"><span><small>Order value</small><strong>{money(order.totalAmount)}</strong></span><span><small>Confirmed payments</small><strong>{money(order.paidAmount)}</strong></span><span><small>Open balance</small><strong>{money(order.balanceDue)}</strong></span></div>
                <label className="cc-order-cancel-reason"><span>Cancellation reason</span><textarea autoFocus onChange={(event) => setReason(event.target.value)} placeholder="State the customer request or business reason" rows={4} value={reason} /></label>
              </>
            ) : (
              <div className="cc-order-confirm-step">
                <div className="cc-order-review-title"><CheckCircle2 size={20} /><div><strong>Review before confirming</strong><p>This action is audited and shared with the customer.</p></div></div>
                {editMode ? <div className="cc-order-review-list">
                  <OrderReviewRow after={productName.trim()} before={order.productName} label="Product" />
                  <OrderReviewRow after={`${quantityNumber.toLocaleString()} pieces`} before={`${order.quantity.toLocaleString()} pieces`} label="Quantity" />
                  <OrderReviewRow after={money(unitPriceNumber)} before={money(order.quantity > 0 ? Math.round(order.totalAmount / order.quantity) : 0)} label="Unit price" />
                  <OrderReviewRow after={money(totalAmount)} before={money(order.totalAmount)} label="Order total" />
                  <OrderReviewRow after={expectedDate} before={order.expectedDate || "Not set"} label="Expected date" />
                </div> : <div className="cc-order-accounting-notice"><strong>{money(order.totalAmount)} order charge will be reversed</strong><p>{order.paidAmount > 0 ? `${money(order.paidAmount)} in confirmed payments will remain as customer credit until it is refunded or applied elsewhere.` : "There are no confirmed payments to refund or reallocate."}</p></div>}
                <div className="cc-order-review-reason"><small>Audit reason</small><p>{reason}</p></div>
                <label className="cc-order-confirm-field"><span>Type <b>{order.id}</b> to confirm</span><input autoComplete="off" autoFocus onChange={(event) => setConfirmation(event.target.value)} placeholder={order.id} value={confirmation} /></label>
              </div>
            )}
            {error && <div className="cc-order-dialog-error" role="alert">{error}</div>}
          </div>

          <footer className="cc-order-dialog-footer">
            {step === "confirm" ? <button className="cc-button" disabled={busy} onClick={() => { setError(""); setConfirmation(""); setStep("details"); }} type="button">Back</button> : <span>Changes are saved to order history and the audit log.</span>}
            <div><button className="cc-button" disabled={busy} onClick={close} type="button">Close</button><button className={`cc-button ${editMode ? "cc-primary" : "cc-order-danger-button"}`} disabled={busy} type="submit">{busy ? <LoaderCircle className="cc-spin" size={17} /> : step === "details" ? <ChevronRight size={17} /> : editMode ? <Check size={17} /> : <Ban size={17} />}{step === "details" ? (editMode ? "Review changes" : "Review cancellation") : (editMode ? "Apply verified changes" : "Cancel order")}</button></div>
          </footer>
        </form>
      </section>
    </div>
  );
}

function OrderReviewRow({ label, before, after }: { label: string; before: string; after: string }) {
  const changed = before !== after;
  return <div className={changed ? "changed" : ""}><small>{label}</small><span>{before}</span><ChevronRight size={14} /><strong>{after}</strong></div>;
}

function OrderTimeline({ stages }: { stages: Stage[] }) {
  return (
    <ol className="cc-order-timeline">
      {stages.map((stage, index) => {
        const state = normalizeStageStatus(stage.status);
        return (
          <li className={state} key={`${stage.name}-${index}`}>
            <span className="cc-order-timeline-marker">{state === "done" ? <Check size={14} /> : index + 1}</span>
            <div><strong>{stage.name}</strong><small>{stage.date ? date(stage.date) : state === "active" ? "In progress" : state === "cancelled" ? "Cancelled" : "Pending"}</small></div>
          </li>
        );
      })}
    </ol>
  );
}

function OrderMetric({ icon: Icon, label, value, detail, warning = false }: { icon: typeof ClipboardList; label: string; value: string; detail: string; warning?: boolean }) {
  return <div className={warning ? "warning" : ""}><span><Icon size={17} /></span><p><small>{label}</small><strong>{value}</strong><em>{detail}</em></p></div>;
}

function OrderFact({ icon: Icon, label, value, detail, tone = "" }: { icon: typeof ClipboardList; label: string; value: string; detail: string; tone?: string }) {
  return <div className={tone}><span><Icon size={17} /></span><p><small>{label}</small><strong>{value}</strong><em>{detail}</em></p></div>;
}

function matchesFilter(order: Order, filter: OrderFilter) {
  if (filter === "Active") return isActive(order);
  if (filter === "Ready") return order.status.toLowerCase().includes("ready");
  if (filter === "Completed") return order.status.toLowerCase() === "completed";
  if (filter === "Cancelled") return order.status.toLowerCase() === "cancelled";
  return true;
}

function compareOrders(left: Order, right: Order) {
  if (isActive(left) !== isActive(right)) return isActive(left) ? -1 : 1;
  const leftDate = Date.parse(left.expectedDate || "9999-12-31");
  const rightDate = Date.parse(right.expectedDate || "9999-12-31");
  if (leftDate !== rightDate) return leftDate - rightDate;
  return right.createdAt.localeCompare(left.createdAt);
}

function isActive(order: Order) {
  return !["completed", "cancelled"].includes(order.status.toLowerCase());
}

function isOverdue(order: Order) {
  if (!order.expectedDate || !isActive(order)) return false;
  const target = Date.parse(`${order.expectedDate}T23:59:59`);
  return Number.isFinite(target) && target < Date.now();
}

function deadlineLabel(value: string) {
  if (!value) return "Date not set";
  const target = Date.parse(`${value}T23:59:59`);
  if (!Number.isFinite(target)) return "Date not set";
  const days = Math.ceil((target - Date.now()) / 86400000);
  if (days === 0) return "Due today";
  if (days === 1) return "1 day left";
  if (days > 1) return `${days} days left`;
  if (days === -1) return "1 day overdue";
  return `${Math.abs(days)} days overdue`;
}

function statusTone(order: Order) {
  const status = order.status.toLowerCase();
  if (status === "completed") return "complete";
  if (status === "cancelled") return "cancelled";
  if (status.includes("ready")) return "ready";
  if (isOverdue(order)) return "late";
  if (status.includes("deposit")) return "payment";
  return "active";
}

function clampProgress(value: number) {
  return Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0));
}

function normalizeStageStatus(value: string) {
  const status = value.toLowerCase();
  if (status === "done" || status === "completed") return "done";
  if (status === "active" || status === "in progress") return "active";
  if (status === "cancelled" || status === "canceled") return "cancelled";
  return "pending";
}

function completedStageCount(stages: Stage[]) {
  return stages.filter((stage) => normalizeStageStatus(stage.status) === "done").length;
}

function numbersOnly(value: string) {
  return value.replace(/[^0-9]/g, "");
}

function dateTime(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" });
}
