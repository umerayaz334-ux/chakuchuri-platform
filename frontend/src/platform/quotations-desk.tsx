import { type ChangeEvent, type DragEvent, type FormEvent, type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import {
  CalendarClock,
  Check,
  ChevronDown,
  ChevronRight,
  CircleDollarSign,
  FileImage,
  ImagePlus,
  LoaderCircle,
  MessageSquare,
  Minus,
  PackageCheck,
  Phone,
  Plus,
  Search,
  SendHorizontal,
  Trash2,
  UserRound,
  X
} from "lucide-react";
import { apiBlob, apiPost, apiUploadFile } from "./api";
import type { Call, Conversation, Customer, Quotation, Session, UserPresence } from "./types";
import { date, digits, messageFromError, money, resolveCustomerPresence, stashOrderSelection, takeStoredSelection } from "./utils";

type QuoteFilter = "All" | "Needs consultation" | "Priced" | "Accepted" | "Closed";

type QuotationsDeskProps = {
  mode: "admin" | "customer";
  quotes: Quotation[];
  customers?: Customer[];
  calls?: Call[];
  conversations?: Conversation[];
  presence?: UserPresence[];
  session: Session;
  onAction: (message: string) => Promise<void>;
  onNavigate: (page: string) => void;
};

export function QuotationsDesk(props: QuotationsDeskProps) {
  return props.mode === "admin" ? <AdminQuotationsDesk {...props} /> : <CustomerQuotationsDesk {...props} />;
}

function CustomerQuotationsDesk({ quotes, session, onAction, onNavigate }: QuotationsDeskProps) {
  const [selectedId, setSelectedId] = useState(quotes[0]?.id || "");
  const [composerOpen, setComposerOpen] = useState(false);
  const sorted = useMemo(() => [...quotes].sort((left, right) => right.createdAt.localeCompare(left.createdAt)), [quotes]);

  useEffect(() => {
    const stored = takeStoredSelection("cc_select_quote_id");
    if (stored && quotes.some((quote) => quote.id === stored)) {
      setSelectedId(stored);
      return;
    }
    if (selectedId && quotes.some((quote) => quote.id === selectedId)) return;
    setSelectedId(sorted[0]?.id || "");
  }, [quotes, selectedId, sorted]);

  useEffect(() => {
    if (sessionStorage.getItem("cc-open-quote-composer") === "1") {
      sessionStorage.removeItem("cc-open-quote-composer");
      setComposerOpen(true);
    }
  }, []);

  useEffect(() => {
    if (!composerOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setComposerOpen(false);
    };
    document.addEventListener("keydown", onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = previous;
    };
  }, [composerOpen]);

  const selected = quotes.find((quote) => quote.id === selectedId) || sorted[0];

  return (
    <section className="cc-customer-quotes">
      <div className="cc-customer-quotes-toolbar">
        <div>
          <span className="cc-kicker">Quotations</span>
          <h2>My requests</h2>
        </div>
        <button className="cc-quote-open-composer" onClick={() => setComposerOpen(true)} type="button">
          <Plus size={16} strokeWidth={2.4} /> New quote
        </button>
      </div>

      <aside className="cc-customer-quote-list">
        <header>
          <div><span className="cc-kicker">My requests</span><h2>Quotation status</h2></div>
          <span>{quotes.length}</span>
        </header>
        {sorted.length ? (
          <div className="cc-customer-quote-items">
            {sorted.map((quote) => (
              <button className={selected?.id === quote.id ? "selected" : ""} key={quote.id} onClick={() => setSelectedId(quote.id)} type="button">
                <QuoteThumbnail fileId={quoteImageIds(quote)[0]} name={quoteImageNames(quote)[0]} token={session.token} />
                <span><strong>{quoteDisplayName(quote)}</strong><small>{quote.id} / {quote.quantity.toLocaleString()} pieces</small><em>{customerQuoteNextStep(quote)}</em></span>
                <QuoteStatus status={quote.status} />
              </button>
            ))}
          </div>
        ) : (
          <div className="cc-customer-quote-empty">
            <FileImage size={23} />
            <strong>No quote requests yet</strong>
            <small>Tap New quote to send product photos for pricing.</small>
            <button className="cc-quote-open-composer is-empty" onClick={() => setComposerOpen(true)} type="button">
              <Plus size={16} strokeWidth={2.4} /> Request a quotation
            </button>
          </div>
        )}
      </aside>

      {selected && <CustomerQuoteDetail key={selected.id} onAction={onAction} onNavigate={onNavigate} quote={selected} session={session} />}

      {composerOpen && (
        <div
          className="cc-quote-composer-backdrop"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setComposerOpen(false);
          }}
        >
          <section aria-labelledby="cc-quote-composer-title" aria-modal="true" className="cc-quote-composer-dialog" role="dialog">
            <QuoteRequestForm
              onAction={async (message) => {
                await onAction(message);
                setComposerOpen(false);
              }}
              onClose={() => setComposerOpen(false)}
              session={session}
            />
          </section>
        </div>
      )}
    </section>
  );
}

function QuoteRequestForm({
  session,
  onAction,
  onClose
}: {
  session: Session;
  onAction: (message: string) => Promise<void>;
  onClose?: () => void;
}) {
  const [files, setFiles] = useState<File[]>([]);
  const [quantity, setQuantity] = useState("50");
  const [note, setNote] = useState("");
  const [showNote, setShowNote] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [busy, setBusy] = useState(false);
  const [uploadedCount, setUploadedCount] = useState(0);
  const [error, setError] = useState("");
  const picker = useRef<HTMLInputElement>(null);
  const previews = useMemo(() => files.map((file) => ({ file, url: URL.createObjectURL(file) })), [files]);

  useEffect(() => () => previews.forEach((preview) => URL.revokeObjectURL(preview.url)), [previews]);

  const addFiles = (selected: File[]) => {
    const images = selected.filter((file) => file.type.startsWith("image/"));
    if (images.length !== selected.length) {
      setError("Choose PNG, JPG or WebP product photos.");
      return;
    }
    if (images.some((file) => file.size > 2 * 1024 * 1024)) {
      setError("Each photo must be 2 MB or smaller — about a normal phone screenshot. Compress or choose a smaller image.");
      return;
    }
    if (files.length + images.length > 5) {
      setError("You can add up to 5 product photos.");
      return;
    }
    setError("");
    setFiles((current) => [...current, ...images]);
  };

  const chooseFiles = (event: ChangeEvent<HTMLInputElement>) => {
    addFiles(Array.from(event.target.files || []));
    event.target.value = "";
  };

  const dropFiles = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setDragging(false);
    addFiles(Array.from(event.dataTransfer.files));
  };

  const adjustQuantity = (amount: number) => {
    setQuantity(String(Math.max(1, Number(quantity || 0) + amount)));
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const count = Number(quantity);
    if (files.length === 0) {
      setError("Add at least one product photo.");
      return;
    }
    if (!Number.isInteger(count) || count < 1) {
      setError("Enter a valid quantity.");
      return;
    }
    setBusy(true);
    setUploadedCount(0);
    setError("");
    try {
      const uploaded = [];
      for (const file of files) {
        uploaded.push(await apiUploadFile(file, {
          customerId: session.user.customerId || "",
          ownerType: "quotation_reference",
          ownerId: "new-quote"
        }, session.token));
        setUploadedCount((current) => current + 1);
      }
      await apiPost<Quotation>("/api/workflow/quotes", {
        customerId: session.user.customerId,
        productName: "",
        quantity: count,
        imageName: uploaded[0]?.originalName || "",
        imageFileId: uploaded[0]?.id || "",
        imageNames: uploaded.map((file) => file.originalName),
        imageFileIds: uploaded.map((file) => file.id),
        notes: note.trim()
      }, session.token);
      setFiles([]);
      setQuantity("50");
      setNote("");
      setShowNote(false);
      await onAction("Quotation request sent for consultation.");
    } catch (cause) {
      setError(messageFromError(cause, "The quote request could not be submitted."));
    } finally {
      setBusy(false);
      setUploadedCount(0);
    }
  };

  return (
    <form className="cc-photo-quote-form" onSubmit={submit}>
      <header>
        <div>
          <span className="cc-kicker">New request</span>
          <h2 id="cc-quote-composer-title">Request a quotation</h2>
        </div>
        <div className="cc-photo-quote-form-actions">
          <span className="cc-photo-quote-count">{files.length}/5 photos</span>
          {onClose && (
            <button aria-label="Close" className="cc-quote-composer-close" disabled={busy} onClick={onClose} type="button">
              <X size={18} />
            </button>
          )}
        </div>
      </header>

      <input accept="image/png,image/jpeg,image/webp" aria-label="Choose product photos" hidden multiple onChange={chooseFiles} ref={picker} type="file" />
      <div
        className={`cc-photo-dropzone ${dragging ? "dragging" : ""} ${files.length ? "has-files" : ""}`}
        onDragEnter={(event) => { event.preventDefault(); setDragging(true); }}
        onDragLeave={() => setDragging(false)}
        onDragOver={(event) => event.preventDefault()}
        onDrop={dropFiles}
      >
        {previews.length ? (
          <div className="cc-quote-local-previews">
            {previews.map(({ file, url }, index) => (
              <figure key={`${file.name}-${file.lastModified}-${index}`}>
                <img alt={`Product reference ${index + 1}`} src={url} />
                <button aria-label={`Remove ${file.name}`} onClick={() => setFiles((current) => current.filter((_, itemIndex) => itemIndex !== index))} title="Remove photo" type="button"><X size={15} /></button>
                {index === 0 && <figcaption>Cover</figcaption>}
              </figure>
            ))}
            {files.length < 5 && <button className="cc-add-reference" onClick={() => picker.current?.click()} type="button"><ImagePlus size={20} /><span>Add photo</span></button>}
          </div>
        ) : (
          <button className="cc-empty-photo-picker" onClick={() => picker.current?.click()} type="button">
            <span><ImagePlus size={25} /></span>
            <strong>Add product photos</strong>
            <small>PNG, JPG or WebP · up to 5 · max 2 MB each (normal screenshot size)</small>
          </button>
        )}
      </div>
      <p className="cc-quote-photo-hint">
        Keep each photo around a normal phone screenshot — <strong>2 MB max</strong>. Larger camera files will be blocked.
      </p>

      <div className="cc-photo-quote-controls">
        <label className="cc-quantity-control">
          <span>Quantity</span>
          <div>
            <button aria-label="Decrease quantity" onClick={() => adjustQuantity(-1)} type="button"><Minus size={16} /></button>
            <input inputMode="numeric" min="1" onChange={(event) => setQuantity(digits(event.target.value))} required value={quantity} />
            <button aria-label="Increase quantity" onClick={() => adjustQuantity(1)} type="button"><Plus size={16} /></button>
          </div>
        </label>
        <button className="cc-optional-note-trigger" onClick={() => setShowNote((current) => !current)} type="button"><MessageSquare size={16} /> Optional note <ChevronDown className={showNote ? "open" : ""} size={15} /></button>
      </div>

      {showNote && <label className="cc-photo-quote-note"><span>Anything we should know?</span><textarea onChange={(event) => setNote(event.target.value)} placeholder="Packaging, target market or any special request" rows={3} value={note} /></label>}
      {error && <div className="cc-quote-form-error">{error}</div>}
      <button className="cc-submit-photo-quote" disabled={busy || files.length === 0 || Number(quantity) < 1} type="submit">
        {busy ? <><LoaderCircle className="cc-spin" size={18} /> {uploadedCount < files.length ? `Uploading ${uploadedCount + 1} of ${files.length}` : "Submitting request"}</> : <><SendHorizontal size={18} /> Send quotation request</>}
      </button>
    </form>
  );
}

function CustomerQuoteDetail({ quote, session, onAction, onNavigate }: { quote: Quotation; session: Session; onAction: (message: string) => Promise<void>; onNavigate: (page: string) => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const formal = quote.status !== "Requested";

  const act = async (action: "accept" | "reject") => {
    setBusy(true);
    setError("");
    try {
      await apiPost<unknown>(`/api/workflow/quotes/${quote.id}/${action}`, {}, session.token);
      await onAction(action === "accept" ? "Quotation accepted. Your order is now open." : "Quotation declined.");
      if (action === "accept") {
        stashOrderSelection(quote.id);
        onNavigate("Orders");
      }
    } catch (cause) {
      setError(messageFromError(cause, "The quotation could not be updated."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <article className="cc-customer-quote-detail">
      <header>
        <div><span className="cc-kicker">{quote.id}</span><h2>{quoteDisplayName(quote)}</h2><p>{quote.quantity.toLocaleString()} pieces / submitted {date(quote.createdAt)}</p></div>
        <QuoteStatus status={quote.status} />
      </header>
      <QuotePhotoGrid quote={quote} token={session.token} />
      {formal ? (
        <>
          <div className="cc-customer-quote-price">
            <div><small>Per piece</small><strong>{money(unitPriceForQuote(quote))}</strong></div>
            <div><small>Total quote</small><strong>{money(quote.totalAmount)}</strong></div>
            <div><small>Expected ready</small><strong>{quote.expectedDate ? date(quote.expectedDate) : "To confirm"}</strong></div>
          </div>
          <QuoteSpecification quote={quote} />
          {quote.adminNote && <div className="cc-quote-admin-note"><MessageSquare size={16} /><p><strong>Note from ChakuChuri</strong><span>{quote.adminNote}</span></p></div>}
          {quote.status === "Priced" && <div className="cc-customer-quote-actions"><button className="cc-button cc-primary" disabled={busy} onClick={() => void act("accept")} type="button"><PackageCheck size={16} /> Proceed to order</button><button className="cc-button cc-danger" disabled={busy} onClick={() => void act("reject")} type="button"><Trash2 size={16} /> Decline</button></div>}
          {quote.status === "Accepted" && (
            <div className="cc-customer-quote-actions">
              <button
                className="cc-button cc-primary"
                onClick={() => {
                  stashOrderSelection(quote.id);
                  onNavigate("Orders");
                }}
                type="button"
              >
                <PackageCheck size={16} /> Open order {quote.id}
              </button>
            </div>
          )}
        </>
      ) : (
        <div className="cc-consultation-waiting"><Phone size={19} /><div><strong>Consultation pending</strong><span>Our team will review the photos and contact you to confirm product details.</span></div></div>
      )}
      {error && <div className="cc-quote-form-error">{error}</div>}
    </article>
  );
}

function AdminQuotationsDesk({ quotes, customers = [], calls = [], conversations = [], presence = [], session, onAction, onNavigate }: QuotationsDeskProps) {
  const [selectedId, setSelectedId] = useState(quotes[0]?.id || "");
  const [filter, setFilter] = useState<QuoteFilter>("All");
  const [search, setSearch] = useState("");
  const customerNames = useMemo(() => new Map(customers.map((customer) => [customer.id, customer.companyName])), [customers]);
  const sorted = useMemo(() => [...quotes].sort(compareQuotes), [quotes]);
  const visible = useMemo(() => sorted.filter((quote) => {
    if (!matchesQuoteFilter(quote, filter)) return false;
    const query = search.trim().toLowerCase();
    if (!query) return true;
    return [quote.id, quote.productName, quote.status, customerNames.get(quote.customerId) || ""].some((value) => value.toLowerCase().includes(query));
  }), [customerNames, filter, search, sorted]);

  useEffect(() => {
    const stored = takeStoredSelection("cc_select_quote_id");
    if (stored && quotes.some((quote) => quote.id === stored)) {
      setSelectedId(stored);
      return;
    }
    if (selectedId && quotes.some((quote) => quote.id === selectedId)) return;
    setSelectedId(sorted[0]?.id || "");
  }, [quotes, selectedId, sorted]);

  const selected = quotes.find((quote) => quote.id === selectedId) || visible[0];
  const requested = quotes.filter((quote) => quote.status === "Requested").length;
  const priced = quotes.filter((quote) => quote.status === "Priced").length;
  const accepted = quotes.filter((quote) => quote.status === "Accepted").length;
  const pipeline = quotes.filter((quote) => !["Rejected"].includes(quote.status)).reduce((sum, quote) => sum + quote.totalAmount, 0);

  return (
    <section className="cc-admin-quotes-desk">
      <div className="cc-quote-summary">
        <QuoteMetric icon={Phone} label="Needs consultation" value={String(requested)} />
        <QuoteMetric icon={CircleDollarSign} label="Awaiting decision" value={String(priced)} />
        <QuoteMetric icon={PackageCheck} label="Converted" value={String(accepted)} />
        <QuoteMetric icon={CalendarClock} label="Quoted pipeline" value={money(pipeline)} />
      </div>
      <div className="cc-admin-quotes-workspace">
        <aside className="cc-admin-quote-index">
          <header><div><span className="cc-kicker">Quote inbox</span><strong>{visible.length} requests</strong></div></header>
          <label className="cc-orders-search"><Search size={17} /><input aria-label="Search quotations" onChange={(event) => setSearch(event.target.value)} placeholder="Search quote or customer" value={search} /></label>
          <div className="cc-orders-filters" role="group" aria-label="Filter quotations">
            {(["All", "Needs consultation", "Priced", "Accepted", "Closed"] as QuoteFilter[]).map((item) => <button aria-pressed={filter === item} className={filter === item ? "active" : ""} key={item} onClick={() => setFilter(item)} type="button">{item}</button>)}
          </div>
          <div className="cc-admin-quote-list">
            {visible.length ? visible.map((quote) => (
              <button className={selected?.id === quote.id ? "selected" : ""} key={quote.id} onClick={() => setSelectedId(quote.id)} type="button">
                <QuoteThumbnail fileId={quoteImageIds(quote)[0]} name={quoteImageNames(quote)[0]} token={session.token} />
                <span><span><strong>{quoteDisplayName(quote)}</strong><em>{quote.id}</em></span><small>{customerNames.get(quote.customerId) || quote.customerId}</small><b>{quote.quantity.toLocaleString()} pieces</b></span>
                <QuoteStatus status={quote.status} />
                <ChevronRight size={15} />
              </button>
            )) : <div className="cc-order-list-empty">No quotations match this filter.</div>}
          </div>
        </aside>
        <div className="cc-admin-quote-detail">
          {selected ? <AdminQuoteConsultation
            activeCall={calls.some((call) => call.customerId === selected.customerId && ["Ringing", "In call"].includes(call.status))}
            conversationId={conversations.find((conversation) => conversation.customerId === selected.customerId)?.id || ""}
            customerName={customerNames.get(selected.customerId) || selected.customerId}
            customerOnline={resolveCustomerPresence(presence, customers.find((customer) => customer.id === selected.customerId) || { id: selected.customerId }).online}
            key={selected.id}
            onAction={onAction}
            onNavigate={onNavigate}
            quote={selected}
            session={session}
          /> : <div className="cc-order-detail-empty"><FileImage size={24} /><strong>Select a quotation</strong></div>}
        </div>
      </div>
    </section>
  );
}

function AdminQuoteConsultation({ quote, customerName, conversationId, activeCall, customerOnline, session, onAction, onNavigate }: {
  quote: Quotation;
  customerName: string;
  conversationId: string;
  activeCall: boolean;
  customerOnline: boolean;
  session: Session;
  onAction: (message: string) => Promise<void>;
  onNavigate: (page: string) => void;
}) {
  const [form, setForm] = useState(() => quoteForm(quote));
  const [busy, setBusy] = useState<"call" | "quote" | "">("");
  const [error, setError] = useState("");
  const canPrice = quote.status === "Requested" || quote.status === "Priced";
  const unitPrice = Number(form.unitPrice);
  const totalAmount = unitPrice > 0 ? unitPrice * quote.quantity : 0;

  useEffect(() => setForm(quoteForm(quote)), [quote.id]);

  const setField = (key: keyof ReturnType<typeof quoteForm>, value: string) => {
    setForm((current) => ({ ...current, [key]: key === "unitPrice" || key === "depositRequired" ? digits(value) : value }));
  };

  const openMessages = () => {
    sessionStorage.setItem("cc_message_customer_id", quote.customerId);
    onNavigate("Messages");
  };

  const startCall = async () => {
    if (busy || activeCall || !customerOnline) return;
    setBusy("call");
    setError("");
    try {
      await apiPost<Call>("/api/workflow/calls", {
        customerId: quote.customerId,
        conversationId,
        subject: `Quote consultation / ${quote.id}`,
        recipientName: customerName
      }, session.token);
      await onAction(`Calling ${customerName} for ${quote.id}...`);
    } catch (cause) {
      setError(messageFromError(cause, "The consultation call could not be started."));
    } finally {
      setBusy("");
    }
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!form.productName.trim() || unitPrice < 1 || totalAmount < 1 || !form.expectedDate) {
      setError("Complete the product name, unit price and expected date.");
      return;
    }
    setBusy("quote");
    setError("");
    try {
      await apiPost<Quotation>(`/api/workflow/quotes/${quote.id}/price`, {
        ...form,
        unitPrice,
        totalAmount,
        depositRequired: Number(form.depositRequired) || 0
      }, session.token);
      await onAction("Formal quotation sent to the customer.");
    } catch (cause) {
      setError(messageFromError(cause, "The formal quotation could not be sent."));
    } finally {
      setBusy("");
    }
  };

  return (
    <>
      <header className="cc-admin-quote-detail-head">
        <div><span className="cc-kicker">{quote.id}</span><h2>{quoteDisplayName(quote)}</h2><p>{customerName} / {quote.quantity.toLocaleString()} pieces / {date(quote.createdAt)}</p></div>
        <QuoteStatus status={quote.status} />
        <div className="cc-quote-contact-actions">
          <button aria-label="Open customer messages" className="cc-icon-button" onClick={openMessages} title="Open messages" type="button"><MessageSquare size={18} /></button>
          <button aria-label={activeCall ? "Call already active" : !customerOnline ? "Customer is offline" : "Start consultation call"} className="cc-quote-call-button" disabled={busy !== "" || activeCall || !customerOnline} onClick={() => void startCall()} title={activeCall ? "Call already active" : !customerOnline ? "Customer is offline" : "Start consultation call"} type="button">{busy === "call" ? <LoaderCircle className="cc-spin" size={17} /> : <Phone size={17} />}<span>{activeCall ? "In call" : !customerOnline ? "Offline" : "Call customer"}</span></button>
        </div>
      </header>

      <div className="cc-quote-reference-band">
        <div><span className="cc-kicker">References</span><h3>Customer photos</h3>{quote.notes && <p>{quote.notes}</p>}</div>
        <QuotePhotoGrid quote={quote} token={session.token} />
      </div>

      <form className="cc-consultation-form" onSubmit={submit}>
        <header><div><span className="cc-kicker">Consultation record</span><h3>Product specification</h3></div><span>Saved with formal quote</span></header>
        <div className="cc-consultation-fields">
          <QuoteField className="wide" label="Product name"><input disabled={!canPrice} onChange={(event) => setField("productName", event.target.value)} placeholder="Confirmed product name" required value={form.productName} /></QuoteField>
          <QuoteField label="Steel"><select disabled={!canPrice} onChange={(event) => setField("steel", event.target.value)} value={form.steel}>{["D2", "1095 carbon", "440C", "Damascus", "Stainless", "Customer supplied"].map((item) => <option key={item}>{item}</option>)}</select></QuoteField>
          <QuoteField label="Tang"><select disabled={!canPrice} onChange={(event) => setField("tang", event.target.value)} value={form.tang}>{["Full tang", "Hidden tang", "Partial tang", "Rat-tail tang", "Not applicable"].map((item) => <option key={item}>{item}</option>)}</select></QuoteField>
          <QuoteField label="Blade thickness"><input disabled={!canPrice} onChange={(event) => setField("bladeThickness", event.target.value)} placeholder="e.g. 3.2 mm" value={form.bladeThickness} /></QuoteField>
          <QuoteField label="Handle"><input disabled={!canPrice} onChange={(event) => setField("handleMaterial", event.target.value)} placeholder="Material and color" value={form.handleMaterial} /></QuoteField>
          <QuoteField label="Sheath"><select disabled={!canPrice} onChange={(event) => setField("sheath", event.target.value)} value={form.sheath}>{["Leather", "Kydex", "Nylon", "Wood", "None", "Custom"].map((item) => <option key={item}>{item}</option>)}</select></QuoteField>
          <QuoteField label="Finish"><select disabled={!canPrice} onChange={(event) => setField("finish", event.target.value)} value={form.finish}>{["Satin", "Mirror polish", "Stonewashed", "Black oxide", "Brut de forge", "Custom"].map((item) => <option key={item}>{item}</option>)}</select></QuoteField>
        </div>

        <div className="cc-quote-pricing-block">
          <header><div><span className="cc-kicker">Commercials</span><h3>Price and schedule</h3></div>{totalAmount > 0 && <span>{quote.quantity.toLocaleString()} pieces / {money(totalAmount)}</span>}</header>
          <div>
            <QuoteField label="Price per piece"><span className="cc-money-input"><b>Rs</b><input disabled={!canPrice} inputMode="numeric" onChange={(event) => setField("unitPrice", event.target.value)} required value={form.unitPrice} /></span></QuoteField>
            <QuoteField label="Total quote"><output className="cc-quote-calculated-total">{totalAmount > 0 ? money(totalAmount) : "Rs 0"}</output></QuoteField>
            <QuoteField label="Deposit required"><span className="cc-money-input"><b>Rs</b><input disabled={!canPrice} inputMode="numeric" onChange={(event) => setField("depositRequired", event.target.value)} value={form.depositRequired} /></span></QuoteField>
            <QuoteField label="Expected ready"><input disabled={!canPrice} onChange={(event) => setField("expectedDate", event.target.value)} required type="date" value={form.expectedDate} /></QuoteField>
          </div>
          <QuoteField className="quote-note" label="Customer-facing note"><textarea disabled={!canPrice} onChange={(event) => setField("adminNote", event.target.value)} placeholder="Agreed packing, tolerances, delivery assumptions or next action" rows={3} value={form.adminNote} /></QuoteField>
        </div>

        {error && <div className="cc-quote-form-error">{error}</div>}
        {canPrice && <footer><span><Check size={15} /> Photos and consultation details stay attached to this quote.</span><button className="cc-button cc-primary" disabled={busy !== ""} type="submit">{busy === "quote" ? <LoaderCircle className="cc-spin" size={17} /> : <SendHorizontal size={17} />}{quote.status === "Priced" ? "Update formal quote" : "Send formal quote"}</button></footer>}
      </form>

      {quote.status === "Accepted" && (
        <div className="cc-customer-quote-actions" style={{ marginTop: 12 }}>
          <button
            className="cc-button cc-primary"
            onClick={() => {
              stashOrderSelection(quote.id);
              onNavigate("Orders");
            }}
            type="button"
          >
            <PackageCheck size={16} /> Open order {quote.id}
          </button>
        </div>
      )}

      {quote.history.length > 0 && <section className="cc-quote-history"><header><span className="cc-kicker">History</span><h3>Quotation activity</h3></header>{quote.history.slice(0, 6).map((update, index) => <article key={`${update.createdAt}-${index}`}><span><UserRound size={14} /></span><div><strong>{update.label}</strong><p>{update.detail}</p><small>{update.actor}</small></div><time>{date(update.createdAt)}</time></article>)}</section>}
    </>
  );
}

function QuotePhotoGrid({ quote, token }: { quote: Quotation; token: string }) {
  const ids = quoteImageIds(quote);
  const names = quoteImageNames(quote);
  if (!ids.length) return <div className="cc-quote-no-photo"><FileImage size={21} /><span>{names[0] || "No reference photo"}</span></div>;
  return <div className={`cc-quote-photo-grid count-${Math.min(ids.length, 5)}`}>{ids.slice(0, 5).map((id, index) => <QuotePhoto fileId={id} key={id} name={names[index] || `Reference ${index + 1}`} token={token} />)}</div>;
}

function QuotePhoto({ fileId, name, token }: { fileId: string; name: string; token: string }) {
  const [src, setSrc] = useState("");
  useEffect(() => {
    if (!fileId) return;
    let active = true;
    let objectUrl = "";
    apiBlob(`/api/files/${fileId}/thumbnail`, token).catch(() => apiBlob(`/api/files/${fileId}/content`, token)).then((blob) => {
      if (!active) return;
      objectUrl = URL.createObjectURL(blob);
      setSrc(objectUrl);
    }).catch(() => setSrc(""));
    return () => { active = false; if (objectUrl) URL.revokeObjectURL(objectUrl); };
  }, [fileId, token]);
  return <figure>{src ? <img alt={name} src={src} /> : <span><FileImage size={22} /></span>}<figcaption>{name}</figcaption></figure>;
}

function QuoteThumbnail({ fileId, name, token }: { fileId?: string; name?: string; token: string }) {
  if (!fileId) return <span className="cc-quote-thumbnail"><FileImage size={18} /></span>;
  return <span className="cc-quote-thumbnail"><QuotePhoto fileId={fileId} name={name || "Product reference"} token={token} /></span>;
}

function QuoteSpecification({ quote }: { quote: Quotation }) {
  const rows = [["Steel", quote.steel], ["Tang", quote.tang], ["Blade", quote.bladeThickness], ["Handle", quote.handleMaterial], ["Sheath", quote.sheath], ["Finish", quote.finish]].filter(([, value]) => value);
  if (!rows.length) return null;
  return <div className="cc-quote-specification">{rows.map(([label, value]) => <div key={label}><small>{label}</small><strong>{value}</strong></div>)}</div>;
}

function QuoteField({ label, className = "", children }: { label: string; className?: string; children: ReactNode }) {
  return <label className={className}><span>{label}</span>{children}</label>;
}

function QuoteStatus({ status }: { status: string }) {
  return <span className={`cc-quote-status ${status.toLowerCase().replace(/\s+/g, "-")}`}>{status === "Requested" ? "Consult" : status}</span>;
}

function QuoteMetric({ icon: Icon, label, value }: { icon: typeof Phone; label: string; value: string }) {
  return <div><span><Icon size={17} /></span><p><small>{label}</small><strong>{value}</strong></p></div>;
}

function quoteForm(quote: Quotation) {
  return {
    productName: quote.productName === "Product consultation" ? "" : quote.productName,
    steel: quote.steel || "D2",
    tang: quote.tang || "Full tang",
    bladeThickness: quote.bladeThickness || "",
    handleMaterial: quote.handleMaterial || "",
    sheath: quote.sheath || "Leather",
    finish: quote.finish || "Satin",
    unitPrice: quote.totalAmount && quote.quantity ? String(Math.round(quote.totalAmount / quote.quantity)) : "",
    depositRequired: quote.depositRequired ? String(quote.depositRequired) : "",
    expectedDate: quote.expectedDate || "",
    adminNote: quote.adminNote || ""
  };
}

function quoteImageIds(quote: Quotation) {
  return quote.imageFileIds?.length ? quote.imageFileIds : quote.imageFileId ? [quote.imageFileId] : [];
}

function quoteImageNames(quote: Quotation) {
  return quote.imageNames?.length ? quote.imageNames : quote.imageName ? [quote.imageName] : [];
}

function quoteDisplayName(quote: Quotation) {
  if (!quote.productName || quote.productName === "Product consultation") return "Product consultation";
  return quote.productName;
}

function customerQuoteNextStep(quote: Quotation) {
  if (quote.status === "Requested") return "Consultation pending";
  if (quote.status === "Priced") return "Review and approve";
  if (quote.status === "Accepted") return "Converted to order";
  return quote.status;
}

function compareQuotes(left: Quotation, right: Quotation) {
  const leftTime = Date.parse(left.createdAt);
  const rightTime = Date.parse(right.createdAt);
  if (Number.isFinite(leftTime) && Number.isFinite(rightTime) && leftTime !== rightTime) return rightTime - leftTime;
  return right.createdAt.localeCompare(left.createdAt);
}

function unitPriceForQuote(quote: Quotation) {
  return quote.quantity > 0 ? Math.round(quote.totalAmount / quote.quantity) : 0;
}

function matchesQuoteFilter(quote: Quotation, filter: QuoteFilter) {
  if (filter === "Needs consultation") return quote.status === "Requested";
  if (filter === "Priced") return quote.status === "Priced";
  if (filter === "Accepted") return quote.status === "Accepted";
  if (filter === "Closed") return quote.status === "Rejected";
  return true;
}
