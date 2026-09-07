import { type FormEvent, useEffect, useMemo, useRef, useState } from "react";
import {
  Building2,
  CheckCircle2,
  Clock3,
  Download,
  Eye,
  FileImage,
  LoaderCircle,
  Plus,
  Printer,
  ReceiptText,
  Search,
  Upload,
  X
} from "lucide-react";
import { apiBlob, apiPost, apiUploadFile } from "./api";
import type { Customer, LedgerEntry, Order, Payment, Session } from "./types";
import { date, messageFromError, money } from "./utils";

type AccountingDeskProps = {
  customers: Customer[];
  ledger: LedgerEntry[];
  orders: Order[];
  payments: Payment[];
  session: Session;
  onAction: (message: string) => Promise<void>;
};

type LedgerRow = LedgerEntry & { balance: number };
type DeskTab = "statement" | "proofs";

export function AccountingDesk({ customers, ledger, orders: _orders, payments, session, onAction }: AccountingDeskProps) {
  const [customerId, setCustomerId] = useState("all");
  const [period, setPeriod] = useState("all");
  const [search, setSearch] = useState("");
  const [busyId, setBusyId] = useState("");
  const [tab, setTab] = useState<DeskTab>("statement");
  const [addOpen, setAddOpen] = useState(false);

  const customerMap = useMemo(() => new Map(customers.map((customer) => [customer.id, customer])), [customers]);
  const periodStart = useMemo(() => getPeriodStart(period), [period]);
  const inScope = (rowCustomerId: string, timestamp: string) =>
    (customerId === "all" || rowCustomerId === customerId) && (!periodStart || new Date(timestamp).getTime() >= periodStart);

  const periodLedger = useMemo(
    () => ledger.filter((entry) => inScope(entry.customerId, entry.postedAt)),
    [customerId, ledger, periodStart]
  );
  const scopedPayments = useMemo(
    () => payments.filter((payment) => inScope(payment.customerId, payment.createdAt)),
    [customerId, payments, periodStart]
  );

  const query = search.trim().toLowerCase();
  const matchesQuery = (...values: Array<string | undefined>) =>
    !query || values.some((value) => (value || "").toLowerCase().includes(query));

  const statementRows = useMemo(() => {
    const balances = new Map<string, number>();
    return [...ledger]
      .filter((entry) => customerId === "all" || entry.customerId === customerId)
      .sort((left, right) => left.postedAt.localeCompare(right.postedAt))
      .map((entry) => {
        const balance = (balances.get(entry.customerId) || 0) + entry.debit - entry.credit;
        balances.set(entry.customerId, balance);
        return { ...entry, balance };
      })
      .filter((entry) => !periodStart || new Date(entry.postedAt).getTime() >= periodStart)
      .filter((entry) =>
        matchesQuery(entry.entryNo, entry.sourceId, entry.sourceType, entry.note, customerName(customerMap, entry.customerId))
      )
      .sort((left, right) => right.postedAt.localeCompare(left.postedAt));
  }, [customerId, customerMap, ledger, periodStart, search]);

  const proofPayments = useMemo(
    () =>
      [...scopedPayments]
        .filter((payment) =>
          matchesQuery(
            payment.id,
            payment.type,
            payment.note,
            payment.proofName,
            payment.status,
            customerName(customerMap, payment.customerId)
          )
        )
        .sort((left, right) => right.createdAt.localeCompare(left.createdAt)),
    [customerMap, scopedPayments, search]
  );

  const chargeSources = new Set(["manufacturing_order", "manufacturing_adjustment", "manufacturing_cancellation", "shipping_request", "shipping_adjustment", "shipping_cancellation"]);
  const charges = periodLedger.filter((entry) => chargeSources.has(entry.sourceType)).reduce((sum, entry) => sum + entry.debit - entry.credit, 0);
  const received = periodLedger.reduce((sum, entry) => sum + (entry.sourceType === "payment" ? entry.credit : entry.sourceType === "payment_reversal" ? -entry.debit : 0), 0);
  const pending = scopedPayments.filter((payment) => payment.status === "Waiting confirmation").reduce((sum, payment) => sum + payment.amount, 0);
  const balanceByCustomer = ledger.reduce((balances, entry) => {
    if (customerId !== "all" && entry.customerId !== customerId) return balances;
    balances.set(entry.customerId, (balances.get(entry.customerId) || 0) + entry.debit - entry.credit);
    return balances;
  }, new Map<string, number>());
  const outstanding = Array.from(balanceByCustomer.values()).reduce((sum, balance) => sum + Math.max(balance, 0), 0);
  const advance = Array.from(balanceByCustomer.values()).reduce((sum, balance) => sum + Math.max(-balance, 0), 0);
  const balanceLabel = outstanding > 0 ? "Due" : advance > 0 ? "Advance" : "Settled";
  const balanceAmount = outstanding > 0 ? outstanding : advance;
  const selectedCustomer = customerId === "all" ? null : customerMap.get(customerId);

  const confirmPayment = async (payment: Payment) => {
    setBusyId(payment.id);
    try {
      await apiPost<{ payment: Payment }>(`/api/workflow/payments/${payment.id}/confirm`, {}, session.token);
      await onAction("Payment confirmed and the customer ledger was updated.");
    } catch (error) {
      await onAction(messageFromError(error, "Payment could not be confirmed."));
    } finally {
      setBusyId("");
    }
  };

  const reversePayment = async (payment: Payment) => {
    const reason = window.prompt(`Reason for reversing ${payment.id}`)?.trim();
    if (!reason) return;
    setBusyId(payment.id);
    try {
      await apiPost<{ payment: Payment }>(`/api/workflow/payments/${payment.id}/reverse`, { reason, confirmation: payment.id }, session.token);
      await onAction("Payment reversed and the customer ledger was recalculated.");
    } catch (error) {
      await onAction(messageFromError(error, "Payment could not be reversed."));
    } finally {
      setBusyId("");
    }
  };

  const exportStatement = () => {
    const columns = ["Date", "Customer", "Entry", "Source", "Description", "Debit", "Credit", "Balance", "Currency"];
    const rows = statementRows.map((entry) => [
      entry.postedAt,
      customerName(customerMap, entry.customerId),
      entry.entryNo,
      `${entry.sourceType} / ${entry.sourceId}`,
      entry.note,
      entry.debit,
      entry.credit,
      entry.balance,
      entry.currency || "PKR"
    ]);
    const csv = [columns, ...rows].map((row) => row.map(csvCell).join(",")).join("\r\n");
    const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `statement-${selectedCustomer ? filePart(selectedCustomer.companyName) : "all-customers"}.csv`;
    anchor.click();
    URL.revokeObjectURL(url);
    void onAction("Statement CSV downloaded.");
  };

  const printStatement = () => {
    setTab("statement");
    document.body.classList.add("cc-print-accounting");
    const cleanUp = () => document.body.classList.remove("cc-print-accounting");
    window.addEventListener("afterprint", cleanUp, { once: true });
    window.setTimeout(() => {
      window.print();
      window.setTimeout(cleanUp, 500);
    }, 80);
  };

  return (
    <section className="cc-accounting-desk">
      <div className="cc-accounting-actions">
        <button onClick={() => setAddOpen(true)} title="Record a payment for a customer" type="button">
          <Plus size={17} /> Add payment
        </button>
        <button onClick={exportStatement} title="Download statement as CSV" type="button">
          <Download size={17} /> Export CSV
        </button>
        <button className="primary" onClick={printStatement} title="Print or save statement as PDF" type="button">
          <Printer size={17} /> Print / PDF
        </button>
      </div>

      <div className="cc-accounting-summary">
        <AccountingMetric label="Charges" value={money(charges)} />
        <AccountingMetric label="Received" value={money(received)} />
        <AccountingMetric label={balanceLabel} value={money(balanceAmount)} />
        <AccountingMetric label="Pending" value={money(pending)} />
      </div>

      <section className="cc-accounting-workspace">
        <div className="cc-accounting-toolbar">
          <label className="cc-accounting-search">
            <Search size={17} />
            <input
              aria-label="Search accounting records"
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search reference, proof, note or customer"
              value={search}
            />
          </label>
          <label>
            <Building2 size={16} />
            <select aria-label="Filter by customer" onChange={(event) => setCustomerId(event.target.value)} value={customerId}>
              <option value="all">All customers</option>
              {customers.map((customer) => (
                <option key={customer.id} value={customer.id}>
                  {customer.companyName}
                </option>
              ))}
            </select>
          </label>
          <label>
            <Clock3 size={16} />
            <select aria-label="Filter by period" onChange={(event) => setPeriod(event.target.value)} value={period}>
              <option value="all">All time</option>
              <option value="30">Last 30 days</option>
              <option value="90">Last 90 days</option>
              <option value="365">Last 12 months</option>
            </select>
          </label>
        </div>

        <nav className="cc-accounting-tabs" aria-label="Accounting views">
          <button className={tab === "statement" ? "active" : ""} onClick={() => setTab("statement")} type="button">
            Statement <span>{statementRows.length}</span>
          </button>
          <button className={tab === "proofs" ? "active" : ""} onClick={() => setTab("proofs")} type="button">
            Payment proofs <span>{proofPayments.length}</span>
          </button>
        </nav>

        {tab === "statement" ? (
          <StatementFeed rows={statementRows} customerMap={customerMap} />
        ) : (
          <ProofFeed
            busyId={busyId}
            customerMap={customerMap}
            onConfirm={confirmPayment}
            onReverse={reversePayment}
            payments={proofPayments}
            token={session.token}
          />
        )}
      </section>

      {addOpen ? (
        <AdminAddPaymentForm
          customers={customers}
          initialCustomerId={customerId !== "all" ? customerId : ""}
          onAction={onAction}
          onClose={() => setAddOpen(false)}
          session={session}
        />
      ) : null}
    </section>
  );
}

function AccountingMetric({ label, value }: { label: string; value: string }) {
  return (
    <article>
      <small>{label}</small>
      <strong>{value}</strong>
    </article>
  );
}

function AdminAddPaymentForm({
  customers,
  initialCustomerId,
  session,
  onAction,
  onClose
}: {
  customers: Customer[];
  initialCustomerId: string;
  session: Session;
  onAction: (message: string) => Promise<void>;
  onClose: () => void;
}) {
  const sortedCustomers = useMemo(
    () => [...customers].sort((left, right) => left.companyName.localeCompare(right.companyName)),
    [customers]
  );
  const [customerId, setCustomerId] = useState(initialCustomerId || sortedCustomers[0]?.id || "");
  const [amount, setAmount] = useState("");
  const [note, setNote] = useState("");
  const [proofFile, setProofFile] = useState<File | null>(null);
  const [fileKey, setFileKey] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const filePicker = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busy) onClose();
    };
    document.addEventListener("keydown", onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = previous;
    };
  }, [busy, onClose]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const value = Number(amount);
    if (!customerId) {
      setError("Select a customer.");
      return;
    }
    if (!Number.isFinite(value) || value < 1) {
      setError("Enter a valid amount.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      let proofName = "";
      let proofFileId = "";
      if (proofFile) {
        const uploaded = await apiUploadFile(
          proofFile,
          { customerId, ownerType: "payment_proof", ownerId: customerId },
          session.token
        );
        proofName = uploaded.originalName || proofFile.name;
        proofFileId = uploaded.id;
      }
      const adminNote = note.trim() ? `Admin added · ${note.trim()}` : "Admin added";
      const payment = await apiPost<Payment>(
        "/api/workflow/payments",
        {
          customerId,
          type: "Admin payment",
          amount: Math.round(value),
          proofName,
          proofFileId,
          note: adminNote
        },
        session.token
      );
      try {
        await apiPost<{ payment: Payment }>(`/api/workflow/payments/${payment.id}/confirm`, {}, session.token);
        const company = sortedCustomers.find((row) => row.id === customerId)?.companyName || customerId;
        await onAction(`Payment ${money(payment.amount)} added to ${company}.`);
      } catch {
        await onAction(`Payment ${money(payment.amount)} created and waiting confirmation.`);
      }
      onClose();
    } catch (submitError) {
      setError(messageFromError(submitError, "Could not add payment."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div
      className="cc-admin-pay-root"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget && !busy) onClose();
      }}
    >
      <form aria-labelledby="cc-admin-pay-title" className="cc-admin-pay-sheet" onSubmit={submit}>
        <header className="cc-admin-pay-head">
          <div>
            <span className="cc-kicker">Payments</span>
            <h2 id="cc-admin-pay-title">Add payment</h2>
            <p>Credit a customer account. Screenshot is optional.</p>
          </div>
          <button aria-label="Close" disabled={busy} onClick={onClose} type="button">
            <X size={18} />
          </button>
        </header>

        <label className="cc-admin-pay-field">
          <span>Customer</span>
          <select
            disabled={busy}
            onChange={(event) => setCustomerId(event.target.value)}
            required
            value={customerId}
          >
            <option disabled value="">
              Select customer
            </option>
            {sortedCustomers.map((customer) => (
              <option key={customer.id} value={customer.id}>
                {customer.companyName}
              </option>
            ))}
          </select>
        </label>

        <label className="cc-admin-pay-field">
          <span>Amount (Rs)</span>
          <input
            disabled={busy}
            inputMode="numeric"
            min="1"
            onChange={(event) => setAmount(event.target.value)}
            placeholder="e.g. 50000"
            required
            step="1"
            type="number"
            value={amount}
          />
        </label>

        <label className="cc-admin-pay-field">
          <span>Note</span>
          <textarea
            disabled={busy}
            onChange={(event) => setNote(event.target.value)}
            placeholder="Bank transfer, cash, adjustment…"
            rows={3}
            value={note}
          />
        </label>

        <div className="cc-admin-pay-field">
          <span>Screenshot <em>(optional)</em></span>
          <input
            accept="image/*,.pdf"
            disabled={busy}
            key={fileKey}
            onChange={(event) => setProofFile(event.target.files?.[0] || null)}
            ref={filePicker}
            type="file"
          />
          <button
            className="cc-admin-pay-file"
            disabled={busy}
            onClick={() => filePicker.current?.click()}
            type="button"
          >
            <Upload size={16} />
            {proofFile ? proofFile.name : "Choose image or PDF"}
          </button>
          {proofFile ? (
            <button
              className="cc-admin-pay-clear"
              disabled={busy}
              onClick={() => {
                setProofFile(null);
                setFileKey((current) => current + 1);
              }}
              type="button"
            >
              Remove file
            </button>
          ) : null}
        </div>

        {error ? <div className="cc-admin-pay-error" role="alert">{error}</div> : null}

        <footer className="cc-admin-pay-actions">
          <button disabled={busy} onClick={onClose} type="button">
            Cancel
          </button>
          <button className="primary" disabled={busy || !customerId} type="submit">
            {busy ? <LoaderCircle className="spin" size={16} /> : <Plus size={16} />}
            {busy ? "Saving…" : "Add payment"}
          </button>
        </footer>
      </form>
    </div>
  );
}

function StatementFeed({
  customerMap,
  rows
}: {
  customerMap: Map<string, Customer>;
  rows: LedgerRow[];
}) {
  return (
    <div className="cc-statement-view">
      <header className="cc-statement-title">
        <span>Account statement</span>
        <small>
          {rows.length} rows · Generated {date(new Date().toISOString())}
        </small>
      </header>
      {rows.length ? (
        <div className="cc-accounting-table-wrap">
          <div className="cc-accounting-table statement" role="table">
            <div className="head" role="row">
              <span>Date</span>
              <span>Customer</span>
              <span>Reference</span>
              <span>Description</span>
              <span>Debit</span>
              <span>Credit</span>
              <span>Balance</span>
            </div>
            {rows.map((entry) => (
              <LedgerStatementRow customerMap={customerMap} entry={entry} key={entry.id || entry.entryNo} />
            ))}
          </div>
        </div>
      ) : (
        <AccountingEmpty
          icon={ReceiptText}
          title="No statement entries"
          detail="Change the customer, period or search filters to find accounting activity."
        />
      )}
    </div>
  );
}

function ProofFeed({
  busyId,
  customerMap,
  payments,
  token,
  onConfirm,
  onReverse
}: {
  busyId: string;
  customerMap: Map<string, Customer>;
  payments: Payment[];
  token: string;
  onConfirm: (payment: Payment) => Promise<void>;
  onReverse: (payment: Payment) => Promise<void>;
}) {
  const [previewPayment, setPreviewPayment] = useState<Payment | null>(null);

  return (
    <div className="cc-proof-view">
      <header className="cc-statement-title">
        <span>Payment proofs</span>
        <small>{payments.length} proofs</small>
      </header>
      {payments.length ? (
        <div className="cc-proof-list" role="list">
          <header>
            <span>Proof</span>
            <span>Customer</span>
            <span>Amount</span>
            <span>Details</span>
            <span>Status</span>
            <span>Actions</span>
          </header>
          {payments.map((payment) => {
            const waiting = payment.status === "Waiting confirmation";
            const reversed = payment.status === "Reversed";
            return (
              <article key={payment.id} role="listitem">
                <button
                  aria-label={`Preview proof ${payment.id}`}
                  className="cc-proof-thumb-btn"
                  disabled={!payment.proofFileId}
                  onClick={() => setPreviewPayment(payment)}
                  title="Preview payment proof"
                  type="button"
                >
                  <ProofPreview payment={payment} token={token} />
                </button>
                <div>
                  <strong>{customerName(customerMap, payment.customerId)}</strong>
                  <small>
                    {date(payment.createdAt)} · {payment.id}
                  </small>
                </div>
                <div>
                  <strong>{money(payment.amount)}</strong>
                  <small>{payment.type || "Payment"}</small>
                </div>
                <div>
                  <strong>{payment.note || payment.proofName || "Payment proof"}</strong>
                  <small>{payment.proofName || "Attached file"}</small>
                </div>
                <span className={`cc-payment-status ${waiting ? "waiting" : reversed ? "reversed" : "confirmed"}`}>
                  {waiting ? "Waiting confirmation" : payment.status || "Confirmed"}
                </span>
                <div className="cc-proof-actions">
                  <button
                    aria-label={`Preview proof ${payment.id}`}
                    disabled={!payment.proofFileId}
                    onClick={() => setPreviewPayment(payment)}
                    title="Preview payment proof"
                    type="button"
                  >
                    <Eye size={15} />
                  </button>
                  {waiting && (
                    <button
                      className="confirm"
                      disabled={busyId === payment.id}
                      onClick={() => void onConfirm(payment)}
                      type="button"
                    >
                      {busyId === payment.id ? <LoaderCircle className="cc-spin" size={15} /> : <CheckCircle2 size={15} />}
                      Confirm
                    </button>
                  )}
                  {payment.status === "Confirmed" && (
                    <button
                      className="confirm"
                      disabled={busyId === payment.id}
                      onClick={() => void onReverse(payment)}
                      title="Reverse this confirmed payment with an audited ledger entry"
                      type="button"
                    >
                      Reverse
                    </button>
                  )}
                </div>
              </article>
            );
          })}
        </div>
      ) : (
        <AccountingEmpty
          icon={FileImage}
          title="No payment proofs"
          detail="Uploaded transfer proofs for the selected filters will show here."
        />
      )}
      {previewPayment && <ProofLightbox onClose={() => setPreviewPayment(null)} payment={previewPayment} token={token} />}
    </div>
  );
}

function ProofLightbox({
  payment,
  token,
  onClose
}: {
  payment: Payment;
  token: string;
  onClose: () => void;
}) {
  const [src, setSrc] = useState("");
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!payment.proofFileId) {
      setLoading(false);
      setFailed(true);
      return;
    }
    let active = true;
    let objectUrl = "";
    setLoading(true);
    setFailed(false);
    setSrc("");

    const looksLikeImage = (blob: Blob) => {
      if (blob.type.startsWith("image/")) return true;
      const name = (payment.proofName || "").toLowerCase();
      return /\.(jpe?g|png|gif|webp|bmp|heic)$/i.test(name) && blob.size > 0 && !blob.type.includes("json");
    };

    apiBlob(`/api/files/${payment.proofFileId}/content`, token)
      .catch(() => apiBlob(`/api/files/${payment.proofFileId}/thumbnail`, token))
      .then((blob) => {
        if (!active) return;
        if (!looksLikeImage(blob)) {
          setFailed(true);
          setLoading(false);
          return;
        }
        objectUrl = URL.createObjectURL(blob);
        setSrc(objectUrl);
        setLoading(false);
      })
      .catch(() => {
        if (!active) return;
        setFailed(true);
        setLoading(false);
      });
    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [payment.proofFileId, payment.proofName, token]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="cc-proof-lightbox" onClick={onClose} role="presentation">
      <div
        aria-label={`Preview ${payment.proofName || payment.id}`}
        aria-modal="true"
        className="cc-proof-lightbox-card"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
      >
        <header>
          <div>
            <strong>{payment.proofName || payment.id}</strong>
            <small>
              {money(payment.amount)} · {date(payment.createdAt)}
            </small>
          </div>
          <button aria-label="Close preview" onClick={onClose} title="Close" type="button">
            <X size={16} />
          </button>
        </header>
        <div className="cc-proof-lightbox-body">
          {loading && <LoaderCircle className="cc-spin" size={28} />}
          {!loading && failed && (
            <div className="cc-proof-lightbox-empty">
              <FileImage size={28} />
              <p>Could not load this image preview.</p>
            </div>
          )}
          {!loading && src && <img alt={payment.proofName || "Payment proof"} src={src} />}
        </div>
      </div>
    </div>
  );
}

function LedgerStatementRow({
  customerMap,
  entry
}: {
  customerMap: Map<string, Customer>;
  entry: LedgerRow;
}) {
  return (
    <div className="row" role="row">
      <time>{date(entry.postedAt)}</time>
      <div>
        <strong>{customerName(customerMap, entry.customerId)}</strong>
        <small>{entry.currency || "PKR"}</small>
      </div>
      <div>
        <strong>{entry.entryNo}</strong>
        <small>{entry.sourceId}</small>
      </div>
      <div className="cc-statement-desc">
        <strong>{entry.note || entry.sourceType}</strong>
        <small>{sourceLabel(entry.sourceType)}</small>
      </div>
      <b className={`debit${entry.debit ? " has-value" : ""}`}>{entry.debit ? money(entry.debit) : "-"}</b>
      <b className={`credit${entry.credit ? " has-value" : ""}`}>{entry.credit ? money(entry.credit) : "-"}</b>
      <b>{money(Math.max(entry.balance, 0))}</b>
    </div>
  );
}

function ProofPreview({ payment, token }: { payment: Payment; token: string }) {
  const [src, setSrc] = useState("");
  useEffect(() => {
    if (!payment.proofFileId) {
      setSrc("");
      return;
    }
    let active = true;
    let objectUrl = "";
    apiBlob(`/api/files/${payment.proofFileId}/thumbnail`, token)
      .then((blob) => {
        if (!active || !blob.type.startsWith("image/")) return;
        objectUrl = URL.createObjectURL(blob);
        setSrc(objectUrl);
      })
      .catch(() => setSrc(""));
    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [payment.proofFileId, token]);
  return src ? <img alt="Payment proof thumbnail" src={src} /> : <span className="cc-proof-file"><FileImage size={18} /></span>;
}

function AccountingEmpty({ icon: Icon, title, detail }: { icon: typeof ReceiptText; title: string; detail: string }) {
  return (
    <div className="cc-accounting-empty">
      <Icon size={24} />
      <strong>{title}</strong>
      <p>{detail}</p>
    </div>
  );
}

function customerName(customers: Map<string, Customer>, customerId: string) {
  return customers.get(customerId)?.companyName || customerId || "Unknown customer";
}

function sourceLabel(source: string) {
  return source.replace(/_/g, " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function getPeriodStart(period: string) {
  if (period === "all") return 0;
  const days = Number(period);
  return Number.isFinite(days) ? Date.now() - days * 24 * 60 * 60 * 1000 : 0;
}

function csvCell(value: string | number) {
  return `"${String(value).replace(/"/g, '""')}"`;
}

function filePart(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "customer";
}
