import { type FormEvent, useState } from "react";
import { apiPost, apiUploadRateSheet } from "./api";
import { AdminDirectoryDesk } from "./trade-directory";
import { AccountingDesk } from "./accounting-desk";
import { BackupPanel } from "./backup-panel";
import { AdminCallHistory } from "./call-history";
import { CustomerAccounts } from "./customer-account";
import { AdminCommandHome } from "./dashboard-desk";
import { AppHomeDesk } from "./app-home-desk";
import { EmailCenter } from "./email-center";
import { PlatformSettingsPanel } from "./settings-panel";
import { ProfileSettings } from "./profile-settings";
import { UserAccessDesk } from "./user-access-desk";
import { MessengerDesk } from "./messenger-desk";
import { OrdersDesk } from "./orders-desk";
import { QuotationsDesk } from "./quotations-desk";
import { ShippingDesk as ShippingOperationsDesk } from "./shipping-desk";
import {
  Empty,
  Panel,
  RateCards,
  ReliabilityPanel,
  ShippingRows
} from "./shared";
import type { Customer, Foundation, PlatformSettings, Quotation, Rate, Session, User, Workspace } from "./types";
import { date, digits, labelFor, messageFromError, money } from "./utils";

export function AdminPortal({
  page,
  session,
  customers,
  workspace,
  platformSettings,
  onAction,
  onPlatformSettingsChange,
  onNavigate,
  onNotice,
  onUserChange,
  onCustomerUpdated
}: {
  page: string;
  session: Session;
  foundation: Foundation;
  customers: Customer[];
  workspace: Workspace;
  platformSettings: PlatformSettings;
  onAction: (message: string) => Promise<void>;
  onPlatformSettingsChange: (settings: PlatformSettings) => void;
  onNavigate: (page: string) => void;
  onNotice: (notice: string) => void;
  onUserChange: (user: User) => void;
  onCustomerUpdated?: (customer: Customer) => void;
}) {
  return (
    <div className="cc-page-stack">
      {page === "Command" && (
        <AdminCommandHome customers={customers} onNavigate={onNavigate} sessionToken={session.token} workspace={workspace} />
      )}
      {page === "Customers" && (
        <CustomerAccounts customers={customers} onAction={onAction} onCustomerUpdated={onCustomerUpdated} onNavigate={onNavigate} session={session} workspace={workspace} />
      )}
      {page === "Users & Access" && (
        <UserAccessDesk customers={customers} onAction={onAction} onNavigate={onNavigate} session={session} />
      )}
      {page === "Quotations" && (
        <QuotationsDesk
          calls={workspace.calls}
          conversations={workspace.conversations}
          customers={customers}
          mode="admin"
          onAction={onAction}
          onNavigate={onNavigate}
          presence={workspace.presence}
          quotes={workspace.quotations}
          session={session}
        />
      )}      {page === "Orders" && (
        <OrdersDesk customers={customers} mode="admin" onAction={onAction} onNavigate={onNavigate} orders={workspace.manufacturing} session={session} />
      )}
      {page === "Shipping" && (
        <ShippingOperationsDesk
          customers={customers}
          mode="admin"
          onAction={onAction}
          session={session}
          shipments={workspace.shipping}
        />
      )}
      {page === "Payments" && <AccountingDesk customers={customers} ledger={workspace.ledger} onAction={onAction} orders={workspace.manufacturing} payments={workspace.payments} session={session} />}
      {page === "Directory" && <AdminDirectoryDesk onAction={onAction} session={session} />}
      {page === "App home" && <AppHomeDesk customers={customers} onAction={onAction} session={session} workspace={workspace} />}
      {page === "Messages" && <MessengerDesk customers={customers} mode="admin" onAction={onAction} session={session} workspace={workspace} />}
      {page === "Calls" && <AdminCallHistory customers={customers} onAction={onAction} session={session} workspace={workspace} />}
      {page === "Email" && <EmailCenter onAction={onAction} session={session} />}
      {page === "Settings" && (
        <>
          <ProfileSettings onAction={onAction} onSaved={onUserChange} session={session} />
          {canManagePlatform(session.user) && (
            <>
              <PlatformSettingsPanel settings={platformSettings} session={session} onAction={onAction} onSaved={onPlatformSettingsChange} />
              <BackupPanel session={session} onNotice={onNotice} />
              <Panel title="Error model" label="Reliability" action="Audit" onAction={() => onNotice("Audit recorder is active on backend actions.")}>
                <ReliabilityPanel />
              </Panel>
            </>
          )}
        </>
      )}
    </div>
  );
}

function AdminQuotationBoard({ quotes, session, onAction }: { quotes: Quotation[]; session: Session; onAction: (message: string) => Promise<void> }) {
  if (quotes.length === 0) return <Empty title="No quotations" detail="Customer quote requests appear here." />;
  return <div className="cc-admin-quote-board">{quotes.map((quote) => <AdminQuoteCard key={quote.id} quote={quote} session={session} onAction={onAction} />)}</div>;
}

function AdminQuoteCard({ quote, session, onAction }: { quote: Quotation; session: Session; onAction: (message: string) => Promise<void> }) {
  const [open, setOpen] = useState(quote.status === "Requested");
  const [form, setForm] = useState({
    totalAmount: quote.totalAmount ? String(quote.totalAmount) : "",
    depositRequired: quote.depositRequired ? String(quote.depositRequired) : "",
    expectedDate: quote.expectedDate,
    steel: quote.steel || "D2",
    tang: quote.tang || "Full tang",
    bladeThickness: quote.bladeThickness || "3.0 mm",
    handleMaterial: quote.handleMaterial || "Rosewood",
    sheath: quote.sheath || "Leather",
    finish: quote.finish || "Satin",
    adminNote: quote.adminNote || ""
  });

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    await apiPost<Quotation>(
      `/api/workflow/quotes/${quote.id}/price`,
      { ...form, totalAmount: Number(form.totalAmount), depositRequired: Number(form.depositRequired) },
      session.token
    );
    setOpen(false);
    await onAction("Quotation priced and sent to customer.");
  };

  const setField = (key: keyof typeof form, value: string) => {
    setForm((current) => ({ ...current, [key]: key === "totalAmount" || key === "depositRequired" ? digits(value) : value }));
  };

  return (
    <article className="cc-admin-quote-card">
      <header>
        <div>
          <span>
            {quote.id} / {quote.quantity} pcs
          </span>
          <strong>{quote.productName}</strong>
          <small>{quote.notes || "No notes"}</small>
        </div>
        <button className="cc-button" onClick={() => setOpen((current) => !current)} type="button">
          {open ? "Close" : "Open"}
        </button>
      </header>
      <div className="cc-quote-prices">
        <div>
          <span>Status</span>
          <strong>{quote.status}</strong>
        </div>
        <div>
          <span>Total</span>
          <strong>{quote.totalAmount ? money(quote.totalAmount) : "Waiting"}</strong>
        </div>
        <div>
          <span>Deposit</span>
          <strong>{money(quote.depositRequired)}</strong>
        </div>
        <div>
          <span>Ready</span>
          <strong>{quote.expectedDate ? date(quote.expectedDate) : "Not set"}</strong>
        </div>
      </div>
      {open && (
        <form className="cc-modern-form cc-admin-price-form" onSubmit={submit}>
          {(["totalAmount", "depositRequired", "expectedDate", "steel", "tang", "bladeThickness", "handleMaterial", "sheath", "finish"] as (keyof typeof form)[]).map((key) => (
            <label key={key}>
              {labelFor(key)}
              <input type={key === "expectedDate" ? "date" : "text"} required={key === "totalAmount"} onChange={(event) => setField(key, event.target.value)} value={form[key]} />
            </label>
          ))}
          <label className="cc-span">
            Admin note
            <input onChange={(event) => setField("adminNote", event.target.value)} value={form.adminNote} />
          </label>
          <button className="cc-button cc-primary cc-span" type="submit">
            Send quote
          </button>
        </form>
      )}
    </article>
  );
}

function RateSheetDesk({ rates, session, onAction }: { rates: Rate[]; session: Session; onAction: (message: string) => Promise<void> }) {
  const [form, setForm] = useState({ courier: "FedEx", service: "Duty paid premium", status: "Active" });
  const [sheetFile, setSheetFile] = useState<File | null>(null);
  const [fileKey, setFileKey] = useState(0);
  const [busy, setBusy] = useState(false);

  const setField = (key: keyof typeof form, value: string) => {
    setForm((current) => ({ ...current, [key]: value }));
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!sheetFile) {
      await onAction("Choose a CSV or XLSX rate sheet first.");
      return;
    }
    setBusy(true);
    try {
      const result = await apiUploadRateSheet(sheetFile, form, session.token);
      setSheetFile(null);
      setFileKey((current) => current + 1);
      await onAction(`${result.count} rate rows imported from ${result.source.originalName}.`);
    } catch (error) {
      await onAction(messageFromError(error, "Rate sheet import failed."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="cc-rate-sheet-desk">
      <form className="cc-rate-sheet-import" onSubmit={submit}>
        <label>
          Courier
          <input onChange={(event) => setField("courier", event.target.value)} value={form.courier} />
        </label>
        <label>
          Service
          <input onChange={(event) => setField("service", event.target.value)} value={form.service} />
        </label>
        <label>
          Status
          <select onChange={(event) => setField("status", event.target.value)} value={form.status}>
            <option>Active</option>
            <option>Paused</option>
          </select>
        </label>
        <label className="cc-span">
          CSV / XLSX sheet
          <input key={fileKey} accept=".csv,.xlsx" onChange={(event) => setSheetFile(event.target.files?.[0] || null)} type="file" />
        </label>
        <button className="cc-button cc-primary cc-span" disabled={busy || !sheetFile} type="submit">
          {busy ? "Importing..." : "Import sheet rows"}
        </button>
      </form>
      <RateCards rates={rates} />
    </div>
  );
}

function canManagePlatform(user: User) {
  const role = user.role.toLowerCase();
  return role === "owner" || role === "admin" || user.permissions.includes("platform.settings.manage");
}