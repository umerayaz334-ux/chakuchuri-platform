import { type ChangeEvent, type FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { US_STATES } from "./us-states";
import {
  AlertCircle,
  ArrowRight,
  Box,
  Calculator,
  CalendarDays,
  Check,
  CheckCircle2,
  ChevronRight,
  CircleDollarSign,
  Clock3,
  FileSpreadsheet,
  Gauge,
  Globe2,
  LoaderCircle,
  MapPin,
  PackageCheck,
  Pause,
  Play,
  Search,
  ShieldCheck,
  Truck,
  Upload,
  Weight,
  X,
  type LucideIcon
} from "lucide-react";
import type { Customer, Session, Shipment } from "./types";
import { date, messageFromError, money } from "./utils";
import {
  bookShippingRate,
  dispatchShipment,
  getShippingRateSnapshot,
  importShippingRateBook,
  lookupShippingRates,
  previewShippingRateBook,
  setShippingRateBookStatus
} from "./shipping-api";
import type {
  ShippingLookupRequest,
  ShippingLookupResponse,
  ShippingRateBook,
  ShippingRateOption,
  ShippingRateSnapshot
} from "./shipping-types";

type ShippingDeskProps = {
  mode: "admin" | "customer";
  shipments: Shipment[];
  customers?: Customer[];
  session: Session;
  onAction: (message: string) => Promise<void>;
};

export function ShippingDesk({ mode, shipments, customers = [], session, onAction }: ShippingDeskProps) {
  const [snapshot, setSnapshot] = useState<ShippingRateSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const loadRates = async () => {
    setLoading(true);
    setError("");
    try {
      setSnapshot(await getShippingRateSnapshot(session));
    } catch (loadError) {
      setError(messageFromError(loadError, "Shipping rates could not be loaded."));
    } finally {
      setLoading(false);
    }
  };

  const loadMoreRates = async () => {
    if (!snapshot?.pagination.hasMore) return;
    setLoading(true);
    setError("");
    try {
      const next = await getShippingRateSnapshot(session, snapshot.pagination.loaded);
      setSnapshot((current) => current ? {
        ...next,
        books: mergeRateBooks(current.books, next.books)
      } : next);
    } catch (loadError) {
      setError(messageFromError(loadError, "More rate books could not be loaded."));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadRates();
  }, [session.token]);

  if (mode === "admin") {
    return (
      <AdminShippingDesk
        customers={customers}
        error={error}
        loading={loading}
        onAction={onAction}
        onReload={loadRates}
        onLoadMoreRates={loadMoreRates}
        onSnapshot={setSnapshot}
        session={session}
        shipments={shipments}
        snapshot={snapshot}
      />
    );
  }

  return (
    <CustomerShippingDesk
      error={error}
      loading={loading}
      onAction={onAction}
      onReload={loadRates}
      session={session}
      shipments={shipments}
      snapshot={snapshot}
    />
  );
}

function AdminShippingDesk({ snapshot, shipments, customers, session, loading, error, onReload, onLoadMoreRates, onSnapshot, onAction }: {
  snapshot: ShippingRateSnapshot | null;
  shipments: Shipment[];
  customers: Customer[];
  session: Session;
  loading: boolean;
  error: string;
  onReload: () => Promise<void>;
  onLoadMoreRates: () => Promise<void>;
  onSnapshot: (value: ShippingRateSnapshot) => void;
  onAction: (message: string) => Promise<void>;
}) {
  const [tab, setTab] = useState<"shipments" | "rates">("shipments");
  const [uploadOpen, setUploadOpen] = useState(false);
  const transit = shipments.filter((shipment) => shipment.status.toLowerCase().includes("transit")).length;
  const pending = shipments.filter((shipment) => !isClosedShipment(shipment) && !shipment.status.toLowerCase().includes("transit")).length;

  return (
    <section className="cc-shipping-console">
      <div className="cc-shipping-metrics" aria-label="Shipping summary">
        <ShippingMetric detail="Awaiting action" icon={PackageCheck} label="Open requests" value={String(pending)} />
        <ShippingMetric detail="Courier movement" icon={Truck} label="In transit" value={String(transit)} />
        <ShippingMetric detail="Available to customers" icon={Globe2} label="Active services" value={String(snapshot?.activeServices || 0)} />
        <ShippingMetric detail={snapshot?.latestEffective ? `Effective ${shortDate(snapshot.latestEffective)}` : "No active rate book"} icon={CalendarDays} label="Rate version" value={snapshot?.activeBooks ? `${snapshot.activeBooks} active` : "Not set"} />
      </div>

      <div className="cc-shipping-toolbar">
        <div className="cc-shipping-tabs" role="tablist" aria-label="Shipping sections">
          <button aria-selected={tab === "shipments"} className={tab === "shipments" ? "active" : ""} onClick={() => setTab("shipments")} role="tab" type="button"><Truck size={17} /> Shipments <b>{shipments.length}</b></button>
          <button aria-selected={tab === "rates"} className={tab === "rates" ? "active" : ""} onClick={() => setTab("rates")} role="tab" type="button"><FileSpreadsheet size={17} /> Rate books <b>{snapshot?.pagination.total || 0}</b></button>
        </div>
        <button className="cc-button cc-primary cc-shipping-upload-button" onClick={() => setUploadOpen(true)} type="button"><Upload size={17} /> Upload rate sheet</button>
      </div>

      {error && <ShippingError detail={error} onRetry={() => void onReload()} />}
      {tab === "shipments" ? (
        <ShipmentWorkspace admin customers={customers} onAction={onAction} session={session} shipments={shipments} />
      ) : (
        <RateBookLibrary loading={loading} onAction={onAction} onLoadMore={onLoadMoreRates} onSnapshot={onSnapshot} session={session} snapshot={snapshot} />
      )}

      {uploadOpen && (
        <RateBookImportDialog
          onAction={onAction}
          onClose={() => setUploadOpen(false)}
          onImported={(next) => { onSnapshot(next); setTab("rates"); }}
          session={session}
        />
      )}
    </section>
  );
}

function CustomerShippingDesk({ snapshot, shipments, session, loading, error, onReload, onAction }: {
  snapshot: ShippingRateSnapshot | null;
  shipments: Shipment[];
  session: Session;
  loading: boolean;
  error: string;
  onReload: () => Promise<void>;
  onAction: (message: string) => Promise<void>;
}) {
  const [tab, setTab] = useState<"rates" | "shipments">("rates");
  const [booked, setBooked] = useState<Shipment | null>(null);
  const active = shipments.filter((shipment) => !isClosedShipment(shipment)).length;

  return (
    <section className="cc-shipping-console is-customer">
      <div className="cc-shipping-customer-bar">
        <div className="cc-shipping-tabs" role="tablist" aria-label="Shipping sections">
          <button aria-selected={tab === "rates"} className={tab === "rates" ? "active" : ""} onClick={() => setTab("rates")} role="tab" type="button"><Calculator size={17} /> Get rates</button>
          <button aria-selected={tab === "shipments"} className={tab === "shipments" ? "active" : ""} onClick={() => setTab("shipments")} role="tab" type="button"><Truck size={17} /> Shipments <b>{active}</b></button>
        </div>
        {snapshot?.latestEffective && <span className="cc-shipping-version"><ShieldCheck size={15} /> Rates effective {shortDate(snapshot.latestEffective)}</span>}
      </div>

      {error && <ShippingError detail={error} onRetry={() => void onReload()} />}
      {tab === "rates" ? (
        <RateCalculator
          loading={loading}
          onAction={onAction}
          onBooked={(shipment) => { setBooked(shipment); setTab("shipments"); }}
          session={session}
          snapshot={snapshot}
        />
      ) : (
        <ShipmentWorkspace onAction={onAction} selectedHint={booked?.id} session={session} shipments={shipments} />
      )}
    </section>
  );
}

function RateCalculator({ snapshot, session, loading, onBooked, onAction }: {
  snapshot: ShippingRateSnapshot | null;
  session: Session;
  loading: boolean;
  onBooked: (shipment: Shipment) => void;
  onAction: (message: string) => Promise<void>;
}) {
  const [form, setForm] = useState({ postalCode: "", weightKg: "1", packages: "1", dutyMode: "duty_paid" as "duty_paid" | "non_duty_paid", lengthCm: "", widthCm: "", heightCm: "", includePsw: true });
  const [result, setResult] = useState<ShippingLookupResponse | null>(null);
  const [request, setRequest] = useState<ShippingLookupRequest | null>(null);
  const [selected, setSelected] = useState<ShippingRateOption | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const update = (key: keyof typeof form, value: string | boolean) => setForm((current) => ({ ...current, [key]: value }));

  const calculate = async (event: FormEvent) => {
    event.preventDefault();
    const next: ShippingLookupRequest = {
      country: "US",
      postalCode: form.postalCode,
      dutyMode: form.dutyMode,
      weightKg: Number(form.weightKg),
      lengthCm: Number(form.lengthCm || 0),
      widthCm: Number(form.widthCm || 0),
      heightCm: Number(form.heightCm || 0),
      packages: Number(form.packages),
      includePsw: form.includePsw
    };
    if (!/^\d{5}(?:-\d{4})?$/.test(form.postalCode.trim())) {
      setError("Enter a valid five-digit US ZIP code.");
      return;
    }
    if (!Number.isFinite(next.weightKg) || next.weightKg <= 0 || !Number.isInteger(next.packages) || next.packages < 1) {
      setError("Enter a valid package weight and package count.");
      return;
    }
    const anyDimension = Boolean(form.lengthCm.trim() || form.widthCm.trim() || form.heightCm.trim());
    if (anyDimension && (next.lengthCm <= 0 || next.widthCm <= 0 || next.heightCm <= 0)) {
      setError("Dimensions are optional. Fill length, width and height together, or leave all blank.");
      return;
    }
    setBusy(true);
    setError("");
    setSelected(null);
    try {
      const response = await lookupShippingRates(next, session);
      setRequest(next);
      setResult(response);
    } catch (lookupError) {
      setResult(null);
      setError(messageFromError(lookupError, "Rates could not be calculated."));
    } finally {
      setBusy(false);
    }
  };

  if (loading && !snapshot) {
    return <ShippingLoading label="Loading current services" />;
  }
  if (!snapshot?.activeBooks) {
    return <div className="cc-shipping-empty"><FileSpreadsheet size={25} /><strong>Rates are being updated</strong><span>The shipping team will publish the next active rate book shortly.</span></div>;
  }

  return (
    <div className="cc-rate-calculator-layout">
      <form className="cc-rate-calculator" onSubmit={calculate}>
        <header><span className="cc-kicker">Destination and package</span><h2>Calculate shipping</h2></header>

        <div className="cc-shipping-segment" role="group" aria-label="Duty service">
          <button aria-pressed={form.dutyMode === "duty_paid"} className={form.dutyMode === "duty_paid" ? "active" : ""} onClick={() => update("dutyMode", "duty_paid")} type="button">Duty paid</button>
          <button aria-pressed={form.dutyMode === "non_duty_paid"} className={form.dutyMode === "non_duty_paid" ? "active" : ""} onClick={() => update("dutyMode", "non_duty_paid")} type="button">Non-duty paid</button>
        </div>

        <div className="cc-shipping-field-grid">
          <label className="wide"><span>US ZIP code</span><div className="cc-shipping-input"><MapPin size={16} /><input autoComplete="postal-code" inputMode="numeric" maxLength={10} onChange={(event) => update("postalCode", event.target.value)} placeholder="e.g. 10001" required value={form.postalCode} /></div></label>
          <label><span>Weight per box</span><div className="cc-shipping-input"><Weight size={16} /><input inputMode="decimal" min="0.1" onChange={(event) => update("weightKg", event.target.value)} required step="0.1" type="number" value={form.weightKg} /><em>kg</em></div></label>
          <label><span>Packages</span><div className="cc-shipping-input"><Box size={16} /><input inputMode="numeric" min="1" onChange={(event) => update("packages", event.target.value)} required step="1" type="number" value={form.packages} /></div></label>
        </div>

        <fieldset className="cc-dimension-fields">
          <legend>Box dimensions <span>optional · cm</span></legend>
          <p className="cc-dimension-hint">Leave blank to quote by weight only. Fill all three for volumetric weight.</p>
          <label><span>Length</span><input min="0" onChange={(event) => update("lengthCm", event.target.value)} placeholder="Optional" step="0.1" type="number" value={form.lengthCm} /></label>
          <label><span>Width</span><input min="0" onChange={(event) => update("widthCm", event.target.value)} placeholder="Optional" step="0.1" type="number" value={form.widthCm} /></label>
          <label><span>Height</span><input min="0" onChange={(event) => update("heightCm", event.target.value)} placeholder="Optional" step="0.1" type="number" value={form.heightCm} /></label>
        </fieldset>

        <label className="cc-shipping-check"><input checked={form.includePsw} onChange={(event) => update("includePsw", event.target.checked)} type="checkbox" /><span><Check size={13} /> Include PSW handling</span></label>
        {error && <div className="cc-shipping-form-error" role="alert"><AlertCircle size={15} /> {error}</div>}
        <button className="cc-button cc-primary cc-rate-calculate" disabled={busy} type="submit">{busy ? <LoaderCircle className="spin" size={17} /> : <Calculator size={17} />}{busy ? "Calculating" : "Show rates"}</button>
      </form>

      <div className="cc-rate-results" aria-live="polite">
        {!result ? (
          <div className="cc-rate-placeholder"><Gauge size={26} /><strong>Rate results</strong><span>Enter destination and package details.</span></div>
        ) : (
          <>
            <header className="cc-rate-results-head"><div><span>{result.region || "USA"}</span><h2>{result.zone ? `Zone ${result.zone}` : `ZIP ${result.postalPrefix}`}</h2></div><small>{result.options.length} {result.options.length === 1 ? "service" : "services"}</small></header>
            {result.warnings?.map((warning) => <div className="cc-rate-warning" key={warning}><AlertCircle size={16} /><span>{warning}</span></div>)}
            <div className="cc-rate-option-list">
              {result.options.map((option, index) => (
                <RateOptionCard key={option.serviceId} onSelect={() => setSelected(option)} option={option} recommended={index === 0} selected={selected?.serviceId === option.serviceId} />
              ))}
            </div>
          </>
        )}
      </div>

      {selected && request && (
        <ShipmentBookingDialog
          lookup={request}
          onAction={onAction}
          onBooked={onBooked}
          onClose={() => setSelected(null)}
          option={selected}
          session={session}
        />
      )}
    </div>
  );
}

function RateOptionCard({ option, selected, recommended, onSelect }: { option: ShippingRateOption; selected: boolean; recommended: boolean; onSelect: () => void }) {
  return (
    <article className={`cc-rate-option ${selected ? "selected" : ""} ${option.requiresReview ? "needs-review" : ""}`}>
      <header><div className="cc-rate-carrier-mark">{initials(option.carrier)}</div><div><span>{option.carrier}</span><strong>{option.service}</strong></div>{recommended && <em>Best rate</em>}</header>
      <div className="cc-rate-price"><strong>{rateMoney(option.totalAmount, option.currency)}</strong><span>{option.etaMinDays}-{option.etaMaxDays} days</span></div>
      <div className="cc-rate-facts"><span><Weight size={14} /> {formatWeight(option.chargeableWeightKg)} chargeable</span><span><Globe2 size={14} /> {option.region}</span><span><CircleDollarSign size={14} /> {option.rateMode === "per_kg" ? `${rateMoney(option.unitRate, option.currency)}/kg` : `${formatWeight(option.rateWeightKg)} slab`}</span></div>
      {option.charges.length > 0 && <div className="cc-rate-lines"><span>Base <b>{rateMoney(option.baseAmount, option.currency)}</b></span>{option.charges.map((charge) => <span key={charge.label}>{charge.label} <b>{rateMoney(charge.amount, charge.currency)}</b></span>)}</div>}
      {option.requiresReview ? (
        <div className="cc-rate-review"><AlertCircle size={15} /><span>{option.reviewReasons?.[0] || "Shipping team review required."}</span></div>
      ) : (
        <button className="cc-rate-select" onClick={onSelect} type="button">Choose service <ArrowRight size={16} /></button>
      )}
    </article>
  );
}

function ShipmentBookingDialog({ option, lookup, session, onClose, onBooked, onAction }: {
  option: ShippingRateOption;
  lookup: ShippingLookupRequest;
  session: Session;
  onClose: () => void;
  onBooked: (shipment: Shipment) => void;
  onAction: (message: string) => Promise<void>;
}) {
  const [form, setForm] = useState({ recipientName: "", phone: "", addressLine1: "", addressLine2: "", city: "", state: "", postalCode: lookup.postalCode, contents: "" });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const update = (key: keyof typeof form, value: string) => setForm((current) => ({ ...current, [key]: value }));

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape") onClose(); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [onClose]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const shipment = await bookShippingRate({
        lookup: { ...lookup, carrier: option.carrier, serviceId: option.serviceId },
        ...form
      }, session);
      await onAction(`Shipment ${shipment.id} created with ${option.carrier} ${option.service}.`);
      onBooked(shipment);
      onClose();
    } catch (bookingError) {
      setError(messageFromError(bookingError, "Shipment could not be created."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="cc-shipping-dialog-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <section aria-labelledby="cc-book-shipment-title" aria-modal="true" className="cc-shipping-dialog cc-booking-dialog" role="dialog">
        <header className="cc-shipping-dialog-head"><span><Truck size={19} /></span><div><small>Selected service</small><h2 id="cc-book-shipment-title">Create shipment</h2><p>{option.carrier} {option.service} / {rateMoney(option.totalAmount, option.currency)}</p></div><button aria-label="Close" onClick={onClose} title="Close" type="button"><X size={19} /></button></header>
        <form onSubmit={submit}>
          <div className="cc-booking-service-strip"><div><Clock3 size={16} /><span><b>{option.etaMinDays}-{option.etaMaxDays} days</b>Estimated connection</span></div><div><Weight size={16} /><span><b>{formatWeight(option.chargeableWeightKg)}</b>Chargeable weight</span></div><div><MapPin size={16} /><span><b>Zone {option.zone}</b>{option.region}</span></div></div>
          <div className="cc-booking-fields">
            <label><span>Recipient name</span><input autoComplete="name" onChange={(event) => update("recipientName", event.target.value)} required value={form.recipientName} /></label>
            <label><span>Phone</span><input autoComplete="tel" onChange={(event) => update("phone", event.target.value)} required type="tel" value={form.phone} /></label>
            <label className="wide"><span>Address</span><input autoComplete="address-line1" onChange={(event) => update("addressLine1", event.target.value)} required value={form.addressLine1} /></label>
            <label className="wide"><span>Apartment, suite or unit</span><input autoComplete="address-line2" onChange={(event) => update("addressLine2", event.target.value)} value={form.addressLine2} /></label>
            <label><span>City</span><input autoComplete="address-level2" onChange={(event) => update("city", event.target.value)} required value={form.city} /></label>
            <label>
              <span>State</span>
              <select autoComplete="address-level1" onChange={(event) => update("state", event.target.value)} required value={form.state}>
                <option disabled value="">Select state</option>
                {US_STATES.map((state) => (
                  <option key={state.code} value={state.code}>{state.code} — {state.name}</option>
                ))}
              </select>
            </label>
            <label><span>ZIP code</span><input autoComplete="postal-code" onChange={(event) => update("postalCode", event.target.value)} required value={form.postalCode} /></label>
            <label><span>Country</span><input disabled value="United States" /></label>
            <label className="wide"><span>Package contents</span><input onChange={(event) => update("contents", event.target.value)} placeholder="Commercial description" required value={form.contents} /></label>
          </div>
          {error && <div className="cc-shipping-form-error" role="alert"><AlertCircle size={15} /> {error}</div>}
          <footer><span>Final courier adjustments remain visible in shipment history.</span><div><button className="cc-button" onClick={onClose} type="button">Cancel</button><button className="cc-button cc-primary" disabled={busy} type="submit">{busy ? <LoaderCircle className="spin" size={17} /> : <CheckCircle2 size={17} />}{busy ? "Creating" : "Create shipment"}</button></div></footer>
        </form>
      </section>
    </div>
  );
}

function RateBookLibrary({ snapshot, session, loading, onSnapshot, onAction, onLoadMore }: {
  snapshot: ShippingRateSnapshot | null;
  session: Session;
  loading: boolean;
  onSnapshot: (value: ShippingRateSnapshot) => void;
  onAction: (message: string) => Promise<void>;
  onLoadMore: () => Promise<void>;
}) {
  const [busyId, setBusyId] = useState("");
  const [error, setError] = useState("");

  const changeStatus = async (book: ShippingRateBook) => {
    setBusyId(book.id);
    setError("");
    try {
      const next = await setShippingRateBookStatus(book.id, book.status !== "Active", session);
      onSnapshot(next);
      await onAction(`${book.carrier} ${book.name} is now ${book.status === "Active" ? "paused" : "active"}.`);
    } catch (statusError) {
      setError(messageFromError(statusError, "Rate book status could not be changed."));
    } finally {
      setBusyId("");
    }
  };

  if (loading && !snapshot) return <ShippingLoading label="Loading rate books" />;
  if (!snapshot?.books.length) return <div className="cc-shipping-empty"><FileSpreadsheet size={25} /><strong>No rate books</strong><span>Upload an XLSX workbook to publish customer rates.</span></div>;

  return (
    <div className="cc-rate-library">
      {error && <div className="cc-shipping-form-error" role="alert"><AlertCircle size={15} /> {error}</div>}
      <header className="cc-rate-library-head"><div><span className="cc-kicker">Version history</span><h2>Published rate books</h2></div><small>Previous versions remain stored</small></header>
      <div className="cc-rate-book-list">
        {snapshot.books.map((book) => (
          <article className={book.status === "Active" ? "active" : ""} key={book.id}>
            <header><div className="cc-rate-book-mark"><FileSpreadsheet size={20} /></div><div><span>{book.carrier} / v{book.version}</span><strong>{book.name}</strong><small>{book.sourceName || "Uploaded workbook"}</small></div><b className={book.status === "Active" ? "active" : "paused"}>{book.status === "Active" ? <Check size={13} /> : <Pause size={13} />}{book.status}</b></header>
            <div className="cc-rate-book-stats"><span><b>{book.services.length}</b> services</span><span><b>{book.postalPrefixCount + book.specialPrefixCount}</b> ZIP prefixes</span><span><b>{formatWeight(book.maxBoxWeightKg)}</b> max box</span><span><b>{book.dimensionalDivisor.toLocaleString()}</b> dim divisor</span></div>
            <div className="cc-rate-book-services">{book.services.map((service) => <div key={service.id}><span><strong>{service.name}</strong><small>{service.etaMinDays}-{service.etaMaxDays} days</small></span><span><b>{service.fixedRateCount}</b> fixed rates</span><span><b>{service.bandCount}</b> bulk bands</span><em>from {rateMoney(service.minimumAmount, book.currency)}</em></div>)}</div>
            <footer><span>Effective {book.effectiveDate ? shortDate(book.effectiveDate) : "not set"} / imported {book.importedAt ? date(book.importedAt) : "preview"} by {book.importedBy || "admin"}</span><button disabled={busyId === book.id} onClick={() => void changeStatus(book)} type="button">{busyId === book.id ? <LoaderCircle className="spin" size={15} /> : book.status === "Active" ? <Pause size={15} /> : <Play size={15} />}{book.status === "Active" ? "Pause" : "Activate"}</button></footer>
          </article>
        ))}
        {snapshot.pagination.hasMore && (
          <div className="cc-pagination-more">
            <button className="cc-button" disabled={loading} onClick={() => void onLoadMore()} type="button">See more</button>
            <small>{snapshot.pagination.total - snapshot.pagination.loaded} remaining</small>
          </div>
        )}
      </div>
    </div>
  );
}

function mergeRateBooks(current: ShippingRateBook[], incoming: ShippingRateBook[]) {
  const ids = new Set(current.map((book) => book.id));
  return [...current, ...incoming.filter((book) => !ids.has(book.id))];
}

function RateBookImportDialog({ session, onClose, onImported, onAction }: {
  session: Session;
  onClose: () => void;
  onImported: (snapshot: ShippingRateSnapshot) => void;
  onAction: (message: string) => Promise<void>;
}) {
  const [form, setForm] = useState({ carrier: "OnTrac", name: "USA direct rates", currency: "PKR", activate: true });
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ShippingRateBook | null>(null);
  const [busy, setBusy] = useState<"preview" | "import" | "">("");
  const [error, setError] = useState("");
  const picker = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape" && !busy) onClose(); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [busy, onClose]);

  const chooseFile = (event: ChangeEvent<HTMLInputElement>) => {
    const next = event.target.files?.[0] || null;
    setPreview(null);
    setError("");
    if (next && !next.name.toLowerCase().endsWith(".xlsx")) {
      setFile(null);
      setError("Choose an XLSX workbook.");
      return;
    }
    setFile(next);
  };

  const fields = () => ({ carrier: form.carrier, name: form.name, currency: form.currency, activate: String(form.activate) });

  const runPreview = async (event: FormEvent) => {
    event.preventDefault();
    if (!file) { setError("Choose an XLSX workbook."); return; }
    setBusy("preview");
    setError("");
    try {
      const result = await previewShippingRateBook(file, fields(), session);
      setPreview(result.book);
    } catch (previewError) {
      setError(messageFromError(previewError, "Workbook could not be validated."));
    } finally {
      setBusy("");
    }
  };

  const runImport = async () => {
    if (!file || !preview) return;
    setBusy("import");
    setError("");
    try {
      const result = await importShippingRateBook(file, fields(), session);
      onImported(result.snapshot);
      await onAction(`${result.book.carrier} rate book v${result.book.version} imported with ${result.book.services.length} services.`);
      onClose();
    } catch (importError) {
      setError(messageFromError(importError, "Rate book could not be imported."));
    } finally {
      setBusy("");
    }
  };

  return (
    <div className="cc-shipping-dialog-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
      <section aria-labelledby="cc-import-rates-title" aria-modal="true" className="cc-shipping-dialog cc-rate-import-dialog" role="dialog">
        <header className="cc-shipping-dialog-head"><span><FileSpreadsheet size={19} /></span><div><small>{preview ? "Review import" : "New rate book"}</small><h2 id="cc-import-rates-title">Upload rate sheet</h2><p>{preview ? `${preview.services.length} services detected` : "XLSX service, zone and ZIP tables"}</p></div><button aria-label="Close" disabled={Boolean(busy)} onClick={onClose} title="Close" type="button"><X size={19} /></button></header>
        {!preview ? (
          <form onSubmit={runPreview}>
            <div className="cc-rate-import-fields">
              <label><span>Carrier</span><input list="cc-shipping-carriers" onChange={(event) => setForm((current) => ({ ...current, carrier: event.target.value }))} required value={form.carrier} /><datalist id="cc-shipping-carriers"><option value="OnTrac" /><option value="FedEx" /><option value="DHL" /><option value="UPS" /><option value="USPS" /></datalist></label>
              <label><span>Rate book name</span><input onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} required value={form.name} /></label>
              <label><span>Currency</span><select onChange={(event) => setForm((current) => ({ ...current, currency: event.target.value }))} value={form.currency}><option>PKR</option><option>USD</option><option>EUR</option><option>GBP</option></select></label>
              <label className="cc-rate-activate"><input checked={form.activate} onChange={(event) => setForm((current) => ({ ...current, activate: event.target.checked }))} type="checkbox" /><span><Check size={13} /> Activate after import</span></label>
            </div>
            <button className={`cc-rate-file-drop ${file ? "has-file" : ""}`} onClick={() => picker.current?.click()} type="button"><input accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" hidden onChange={chooseFile} ref={picker} type="file" />{file ? <><FileSpreadsheet size={24} /><span><strong>{file.name}</strong><small>{formatBytes(file.size)}</small></span><CheckCircle2 size={19} /></> : <><Upload size={24} /><span><strong>Choose XLSX workbook</strong><small>Maximum file size 12 MB</small></span><ChevronRight size={19} /></>}</button>
            {error && <div className="cc-shipping-form-error" role="alert"><AlertCircle size={15} /> {error}</div>}
            <footer><span>Nothing is published before review.</span><div><button className="cc-button" onClick={onClose} type="button">Cancel</button><button className="cc-button cc-primary" disabled={busy === "preview" || !file} type="submit">{busy === "preview" ? <LoaderCircle className="spin" size={17} /> : <ArrowRight size={17} />}{busy === "preview" ? "Validating" : "Review workbook"}</button></div></footer>
          </form>
        ) : (
          <div className="cc-rate-import-review">
            <div className="cc-import-review-banner"><CheckCircle2 size={20} /><div><strong>Workbook is ready</strong><span>{preview.postalPrefixCount} continental prefixes and {preview.specialPrefixCount} Hawaii/Alaska prefixes</span></div><b>{preview.effectiveDate ? shortDate(preview.effectiveDate) : "Date missing"}</b></div>
            <div className="cc-import-review-services">{preview.services.map((service) => <article key={service.id}><div><span>{preview.carrier}</span><strong>{service.name}</strong><small>{service.etaMinDays}-{service.etaMaxDays} days</small></div><p><span><b>{service.fixedRateCount}</b> fixed rates</span><span><b>{service.bandCount}</b> bulk bands</span><span><b>{rateMoney(service.minimumAmount, preview.currency)}</b> minimum</span></p></article>)}</div>
            <div className="cc-import-review-rules"><span><Gauge size={16} /><b>{preview.dimensionalDivisor.toLocaleString()}</b> dimensional divisor</span><span><Weight size={16} /><b>{formatWeight(preview.maxBoxWeightKg)}</b> max box</span><span><Globe2 size={16} /><b>{preview.zoneCounts.length}</b> standard zones</span><span><CircleDollarSign size={16} /><b>{preview.surcharges.length}</b> charge rules</span></div>
            {preview.warnings?.map((warning) => <div className="cc-rate-warning" key={warning}><AlertCircle size={16} /> {warning}</div>)}
            {error && <div className="cc-shipping-form-error" role="alert"><AlertCircle size={15} /> {error}</div>}
            <footer><span>{form.activate ? "Current active version will be preserved and paused." : "Workbook will be saved as paused."}</span><div><button className="cc-button" disabled={Boolean(busy)} onClick={() => setPreview(null)} type="button">Back</button><button className="cc-button cc-primary" disabled={busy === "import"} onClick={() => void runImport()} type="button">{busy === "import" ? <LoaderCircle className="spin" size={17} /> : <CheckCircle2 size={17} />}{busy === "import" ? "Importing" : form.activate ? "Import and activate" : "Import as paused"}</button></div></footer>
          </div>
        )}
      </section>
    </div>
  );
}

function ShipmentWorkspace({ shipments, customers = [], session, admin = false, selectedHint, onAction }: {
  shipments: Shipment[];
  customers?: Customer[];
  session: Session;
  admin?: boolean;
  selectedHint?: string;
  onAction: (message: string) => Promise<void>;
}) {
  const sorted = useMemo(() => [...shipments].sort((left, right) => right.createdAt.localeCompare(left.createdAt)), [shipments]);
  const [selectedId, setSelectedId] = useState(selectedHint || sorted[0]?.id || "");
  const [filter, setFilter] = useState("All");
  const [search, setSearch] = useState("");
  const customerNames = useMemo(() => new Map(customers.map((customer) => [customer.id, customer.companyName])), [customers]);
  const visible = sorted.filter((shipment) => {
    if (filter !== "All" && shipmentStatusGroup(shipment) !== filter) return false;
    const query = search.trim().toLowerCase();
    if (!query) return true;
    return [shipment.id, shipment.courier, shipment.service, shipment.destination, shipment.status, customerNames.get(shipment.customerId) || ""].some((value) => value.toLowerCase().includes(query));
  });
  const selected = visible.find((shipment) => shipment.id === selectedId) || visible[0];

  useEffect(() => {
    if (selectedHint && shipments.some((shipment) => shipment.id === selectedHint)) setSelectedId(selectedHint);
  }, [selectedHint, shipments]);
  useEffect(() => {
    if (!selected || selected.id === selectedId) return;
    setSelectedId(selected.id);
  }, [selected, selectedId]);

  if (!shipments.length) return <div className="cc-shipping-empty"><Truck size={25} /><strong>No shipments yet</strong><span>New shipping requests will appear here.</span></div>;

  return (
    <div className="cc-shipment-workspace">
      <aside className="cc-shipment-index">
        <header><div><span className="cc-kicker">Shipment book</span><strong>{visible.length} {visible.length === 1 ? "shipment" : "shipments"}</strong></div></header>
        <label className="cc-shipment-search"><Search size={16} /><input aria-label="Search shipments" onChange={(event) => setSearch(event.target.value)} placeholder="Search shipment or destination" value={search} /></label>
        <div className="cc-shipment-filters">{["All", "Quoted", "In transit", "Delivered"].map((item) => <button className={filter === item ? "active" : ""} key={item} onClick={() => setFilter(item)} type="button">{item}</button>)}</div>
        <div className="cc-shipment-list">{visible.length ? visible.map((shipment) => <button className={selected?.id === shipment.id ? "selected" : ""} key={shipment.id} onClick={() => setSelectedId(shipment.id)} type="button"><span className={`cc-shipment-dot ${shipmentTone(shipment.status)}`} /><span><strong>{shipment.courier} {shipment.service}</strong><small>{admin ? customerNames.get(shipment.customerId) || shipment.customerId : shipment.id}</small><em>{shipment.status} / {rateMoney(shipment.quotedAmount, "PKR")}</em></span><ChevronRight size={16} /></button>) : <div className="cc-shipment-no-match">No shipments match this view.</div>}</div>
      </aside>
      <div className="cc-shipment-detail">{selected ? <ShipmentDetail admin={admin} customerName={customerNames.get(selected.customerId)} onAction={onAction} session={session} shipment={selected} /> : <div className="cc-rate-placeholder"><Truck size={24} /><strong>Select a shipment</strong></div>}</div>
    </div>
  );
}

function ShipmentDetail({ shipment, customerName, admin, session, onAction }: { shipment: Shipment; customerName?: string; admin: boolean; session: Session; onAction: (message: string) => Promise<void> }) {
  const [tracking, setTracking] = useState(shipment.tracking || "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => setTracking(shipment.tracking || ""), [shipment.id, shipment.tracking]);
  const dispatch = async (event: FormEvent) => {
    event.preventDefault();
    if (!tracking.trim()) { setError("Enter the courier tracking number."); return; }
    setBusy(true);
    setError("");
    try {
      await dispatchShipment(shipment.id, tracking.trim(), session);
      await onAction(`${shipment.id} marked in transit.`);
    } catch (dispatchError) {
      setError(messageFromError(dispatchError, "Shipment could not be dispatched."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <article className="cc-shipment-detail-card">
      <header><div><span>{shipment.id}</span><h2>{shipment.courier} {shipment.service}</h2><p>{customerName || shipment.destination.split(" / ")[0]}</p></div><b className={shipmentTone(shipment.status)}>{shipment.status}</b></header>
      <div className="cc-shipment-detail-facts"><div><MapPin size={17} /><span>Destination<strong>{shipment.destination}</strong></span></div><div><Weight size={17} /><span>Rated package<strong>{readableWeight(shipment.weight)}</strong></span></div><div><Globe2 size={17} /><span>Rate zone<strong>{shipment.zone === "Special" ? "Hawaii / Alaska" : `Zone ${shipment.zone}`}</strong></span></div><div><CircleDollarSign size={17} /><span>Quoted amount<strong>{money(shipment.quotedAmount)}</strong></span></div></div>
      <section className="cc-shipment-timeline"><header><span>Shipment activity</span><small>Created {date(shipment.createdAt)}</small></header>{shipment.history?.length ? shipment.history.map((item, index) => <div key={item.label + item.createdAt}><i className={index === 0 ? "current" : ""}>{index === 0 ? <Check size={12} /> : null}</i><span><strong>{item.label}</strong><p>{item.detail}</p><small>{item.actor} / {date(item.createdAt)}</small></span></div>) : <div><i className="current"><Check size={12} /></i><span><strong>Shipping request created</strong><small>{date(shipment.createdAt)}</small></span></div>}</section>
      {shipment.tracking && <div className="cc-shipment-tracking"><span>Tracking number</span><strong>{shipment.tracking}</strong><CheckCircle2 size={18} /></div>}
      {admin && !isClosedShipment(shipment) && !shipment.status.toLowerCase().includes("transit") && <form className="cc-shipment-dispatch" onSubmit={dispatch}><label><span>Courier tracking number</span><input onChange={(event) => setTracking(event.target.value)} placeholder="Enter tracking number" value={tracking} /></label><button className="cc-button cc-primary" disabled={busy} type="submit">{busy ? <LoaderCircle className="spin" size={16} /> : <Truck size={16} />}{busy ? "Updating" : "Mark in transit"}</button></form>}
      {error && <div className="cc-shipping-form-error" role="alert"><AlertCircle size={15} /> {error}</div>}
    </article>
  );
}

function ShippingMetric({ icon: Icon, label, value, detail }: { icon: LucideIcon; label: string; value: string; detail: string }) {
  return <div><span><Icon size={18} /></span><p><small>{label}</small><strong>{value}</strong><em>{detail}</em></p></div>;
}

function ShippingError({ detail, onRetry }: { detail: string; onRetry: () => void }) {
  return <div className="cc-shipping-load-error" role="alert"><AlertCircle size={18} /><span><strong>Shipping data is unavailable</strong><small>{detail}</small></span><button onClick={onRetry} type="button">Retry</button></div>;
}

function ShippingLoading({ label }: { label: string }) {
  return <div className="cc-shipping-loading"><LoaderCircle className="spin" size={20} /><span>{label}</span></div>;
}

function isClosedShipment(shipment: Shipment) {
  const value = shipment.status.toLowerCase();
  return value.includes("delivered") || value.includes("cancelled");
}

function shipmentStatusGroup(shipment: Shipment) {
  const status = shipment.status.toLowerCase();
  if (status.includes("delivered")) return "Delivered";
  if (status.includes("transit") || status.includes("label")) return "In transit";
  return "Quoted";
}

function shipmentTone(status: string) {
  const value = status.toLowerCase();
  if (value.includes("delivered")) return "success";
  if (value.includes("transit")) return "transit";
  if (value.includes("cancel")) return "danger";
  return "quoted";
}

function readableWeight(value: string) {
  const parts = value.split(" / ");
  return parts.slice(0, 1).join("") || value;
}

function formatWeight(value: number) {
  return `${Number(value.toFixed(2))} kg`;
}

function shortDate(value: string) {
  const parsed = new Date(`${value}T00:00:00`);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString("en-PK", { day: "numeric", month: "short", year: "numeric" });
}

function rateMoney(value: number, currency: string) {
  try {
    return new Intl.NumberFormat("en-PK", { style: "currency", currency, maximumFractionDigits: currency === "PKR" ? 0 : 2 }).format(value);
  } catch {
    return `${currency} ${Number(value).toLocaleString()}`;
  }
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function initials(value: string) {
  return value.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join("") || "SR";
}
