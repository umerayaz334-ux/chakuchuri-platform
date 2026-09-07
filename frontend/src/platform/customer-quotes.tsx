import { type FormEvent, useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  ClipboardList,
  Clock3,
  History,
  ImagePlus,
  LoaderCircle,
  Package,
  PackageCheck,
  Plus,
  X
} from "lucide-react";
import { apiPost, apiUploadFile } from "./api";
import { ProductFileThumb } from "./shared";
import type { Quotation, Session } from "./types";
import { date, digits, messageFromError, money } from "./utils";

type QuoteFilter = "All" | "Accepted" | "Rejected / cancelled";

const filters: QuoteFilter[] = ["All", "Accepted", "Rejected / cancelled"];

export function CustomerQuotesPage({
  quotes,
  session,
  onAction,
  onNavigate
}: {
  quotes: Quotation[];
  session: Session;
  onAction: (message: string) => Promise<void>;
  onNavigate: (page: string) => void;
}) {
  const [filter, setFilter] = useState<QuoteFilter>("All");
  const [composerOpen, setComposerOpen] = useState(false);
  const [busyId, setBusyId] = useState("");

  const sorted = useMemo(() => {
    return [...quotes].sort((left, right) => {
      const recent = latestActivity(right).localeCompare(latestActivity(left));
      return recent !== 0 ? recent : right.id.localeCompare(left.id);
    });
  }, [quotes]);

  const review = quotes.filter((quote) => quote.status === "Requested").length;
  const priced = quotes.filter((quote) => quote.status === "Priced").length;
  const accepted = quotes.filter((quote) => quote.status === "Accepted").length;

  const visible = useMemo(() => {
    return sorted.filter((quote) => {
      if (filter === "Accepted") return quote.status === "Accepted";
      if (filter === "Rejected / cancelled") return quote.status === "Rejected" || quote.status === "Cancelled";
      return true;
    });
  }, [filter, sorted]);

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

  const act = async (quote: Quotation, action: "accept" | "reject") => {
    setBusyId(`${quote.id}:${action}`);
    try {
      await apiPost(`/api/workflow/quotes/${quote.id}/${action}`, {}, session.token);
      await onAction(action === "accept" ? "Order created from quote." : "Quote rejected.");
      if (action === "accept") onNavigate("Orders");
    } catch (error) {
      await onAction(messageFromError(error, action === "accept" ? "Could not accept quote." : "Could not reject quote."));
    } finally {
      setBusyId("");
    }
  };

  return (
    <section className="cc-cq">
      <div className="cc-cq-metrics" aria-label="Quotation summary">
        <div>
          <strong className="is-blue">{review}</strong>
          <span>In review</span>
        </div>
        <div>
          <strong className="is-amber">{priced}</strong>
          <span>Ready</span>
        </div>
        <div>
          <strong className="is-green">{accepted}</strong>
          <span>Orders</span>
        </div>
      </div>

      <button className="cc-cq-request" onClick={() => setComposerOpen(true)} type="button">
        <Plus size={18} strokeWidth={2.4} />
        Request a quote
      </button>

      <header className="cc-cq-section-head">
        <h2>Your requests</h2>
        <span>{visible.length}</span>
      </header>

      <div className="cc-cq-filters" role="group" aria-label="Filter quotations">
        {filters.map((item) => (
          <button
            aria-pressed={filter === item}
            className={`cc-cq-chip${filter === item ? " is-active" : ""}`}
            key={item}
            onClick={() => setFilter(item)}
            type="button"
          >
            {item}
          </button>
        ))}
      </div>

      {quotes.length === 0 ? (
        <div className="cc-cq-empty">
          <ClipboardList size={24} strokeWidth={1.75} />
          <strong>No quotes yet</strong>
          <p>Request pricing with a product photo and quantity.</p>
        </div>
      ) : visible.length === 0 ? (
        <div className="cc-cq-empty">
          <ClipboardList size={24} strokeWidth={1.75} />
          <strong>Nothing here</strong>
          <p>No quotations match this filter.</p>
        </div>
      ) : (
        <div className="cc-cq-list">
          {visible.map((quote) => (
            <QuoteCard
              busyId={busyId}
              key={quote.id}
              onAccept={quote.status === "Priced" ? () => void act(quote, "accept") : undefined}
              onOpenOrder={quote.status === "Accepted" ? () => onNavigate("Orders") : undefined}
              onReject={quote.status === "Priced" ? () => void act(quote, "reject") : undefined}
              quote={quote}
              token={session.token}
            />
          ))}
        </div>
      )}

      {composerOpen ? (
        <div
          className="cc-cq-sheet-root"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setComposerOpen(false);
          }}
        >
          <button aria-label="Close quote request" className="cc-cq-sheet-backdrop" onClick={() => setComposerOpen(false)} type="button" />
          <aside aria-labelledby="cc-cq-composer-title" aria-modal="true" className="cc-cq-sheet" role="dialog">
            <div className="cc-cq-sheet-handle" aria-hidden />
            <QuoteComposer
              onAction={async (message) => {
                await onAction(message);
                setComposerOpen(false);
              }}
              onClose={() => setComposerOpen(false)}
              session={session}
            />
          </aside>
        </div>
      ) : null}
    </section>
  );
}

function QuoteCard({
  quote,
  token,
  busyId,
  onAccept,
  onReject,
  onOpenOrder
}: {
  quote: Quotation;
  token: string;
  busyId: string;
  onAccept?: () => void;
  onReject?: () => void;
  onOpenOrder?: () => void;
}) {
  const photos = quoteImageIds(quote);
  const names = quoteImageNames(quote);
  const inReview = quote.status === "Requested";
  const hasPrice = quote.totalAmount > 0;
  const unit = hasPrice && quote.quantity > 0 ? Math.round(quote.totalAmount / quote.quantity) : 0;
  const latest = latestUpdate(quote);
  const accepting = busyId === `${quote.id}:accept`;
  const rejecting = busyId === `${quote.id}:reject`;

  return (
    <article className="cc-cq-card">
      <div className="cc-cq-card-head">
        {photos[0] ? (
          <ProductFileThumb className="cc-cq-thumb" fileId={photos[0]} name={names[0] || quoteTitle(quote)} token={token} />
        ) : (
          <span className="cc-cq-thumb is-empty" aria-hidden>
            <ClipboardList size={26} strokeWidth={1.75} />
          </span>
        )}
        <div className="cc-cq-card-copy">
          <div className="cc-cq-card-idrow">
            <em>{quote.id}</em>
            <span className={`cc-cq-pill is-${pillTone(inReview ? "In review" : quote.status)}`}>
              {inReview ? "In review" : quote.status}
            </span>
          </div>
          <strong>{quoteTitle(quote)}</strong>
          <small>
            <Package size={14} strokeWidth={2} />
            {quote.quantity} pieces
          </small>
        </div>
      </div>

      {photos.length > 1 ? (
        <div className="cc-cq-photo-row">
          {photos.slice(0, 6).map((fileId, index) => (
            <ProductFileThumb
              className="cc-cq-thumb is-sm"
              fileId={fileId}
              key={fileId}
              name={names[index] || quoteTitle(quote)}
              token={token}
            />
          ))}
        </div>
      ) : null}

      {hasPrice ? (
        <div className="cc-cq-facts">
          <div>
            <small>PER UNIT</small>
            <strong>{money(unit)}</strong>
          </div>
          <div>
            <small>TOTAL</small>
            <strong>{money(quote.totalAmount)}</strong>
          </div>
          <div>
            <small>READY</small>
            <strong>{quote.expectedDate ? date(quote.expectedDate) : "Pending"}</strong>
          </div>
        </div>
      ) : inReview ? (
        <div className="cc-cq-review">
          <Clock3 size={18} strokeWidth={2} />
          <span>Our team is reviewing your request.</span>
        </div>
      ) : null}

      {latest?.detail ? (
        <div className="cc-cq-history">
          <History size={15} strokeWidth={2} />
          <span>{latest.detail}</span>
          <em>{date(latest.createdAt)}</em>
        </div>
      ) : null}

      {onOpenOrder ? (
        <div className="cc-cq-actions is-single">
          <button className="cc-cq-btn is-outline" onClick={onOpenOrder} type="button">
            <PackageCheck size={16} strokeWidth={2.2} />
            View order
          </button>
        </div>
      ) : null}

      {onAccept ? (
        <div className={`cc-cq-actions${onReject ? "" : " is-single"}`}>
          <button className="cc-cq-btn is-primary" disabled={Boolean(busyId)} onClick={onAccept} type="button">
            {accepting ? <LoaderCircle className="cc-spin" size={16} /> : <Check size={16} strokeWidth={2.4} />}
            Accept quote
          </button>
          {onReject ? (
            <button
              aria-label="Reject quotation"
              className="cc-cq-btn is-icon-danger"
              disabled={Boolean(busyId)}
              onClick={onReject}
              type="button"
            >
              {rejecting ? <LoaderCircle className="cc-spin" size={16} /> : <X size={18} strokeWidth={2.4} />}
            </button>
          ) : null}
        </div>
      ) : null}
    </article>
  );
}

function QuoteComposer({
  session,
  onAction,
  onClose
}: {
  session: Session;
  onAction: (message: string) => Promise<void>;
  onClose: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState("");
  const [quantity, setQuantity] = useState("");
  const [productName, setProductName] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const picker = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!file) {
      setPreview("");
      return;
    }
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const count = Number(quantity);
    if (!file || !Number.isInteger(count) || count < 1) {
      setError("Add a product photo and enter the quantity.");
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      setError("Each photo must be 2 MB or smaller.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const uploaded = await apiUploadFile(
        file,
        {
          customerId: session.user.customerId || "",
          ownerType: "quotation_reference",
          ownerId: "new-quote"
        },
        session.token
      );
      await apiPost<Quotation>(
        "/api/workflow/quotes",
        {
          customerId: session.user.customerId,
          productName: productName.trim() || "Product consultation",
          quantity: count,
          imageName: uploaded.originalName || file.name,
          imageFileId: uploaded.id,
          imageNames: [uploaded.originalName || file.name],
          imageFileIds: [uploaded.id],
          notes: note.trim()
        },
        session.token
      );
      await onAction("Quotation request sent for consultation.");
    } catch (cause) {
      setError(messageFromError(cause, "The quote request could not be submitted."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="cc-cq-composer" onSubmit={submit}>
      <header className="cc-cq-composer-head">
        <span className="cc-cq-composer-icon" aria-hidden>
          <ClipboardList size={20} strokeWidth={1.9} />
        </span>
        <div>
          <h2 id="cc-cq-composer-title">Request a quote</h2>
          <p>Add a photo and quantity.</p>
        </div>
        <button aria-label="Close" className="cc-cq-composer-close" disabled={busy} onClick={onClose} type="button">
          <X size={18} />
        </button>
      </header>

      <input
        accept="image/png,image/jpeg,image/webp"
        hidden
        onChange={(event) => {
          const next = event.target.files?.[0] || null;
          setFile(next);
          setError("");
          event.target.value = "";
        }}
        ref={picker}
        type="file"
      />

      <button className={`cc-cq-photo-pick${file ? " has-file" : ""}`} disabled={busy} onClick={() => picker.current?.click()} type="button">
        {preview ? (
          <img alt="Product reference" src={preview} />
        ) : (
          <>
            <ImagePlus size={22} strokeWidth={1.9} />
            <strong>Add product photo</strong>
            <small>Max 2 MB — normal screenshot size</small>
          </>
        )}
      </button>

      <label className="cc-cq-field">
        <span>Quantity</span>
        <div className="cc-cq-qty">
          <input
            disabled={busy}
            inputMode="numeric"
            onChange={(event) => setQuantity(digits(event.target.value))}
            placeholder="0"
            required
            value={quantity}
          />
          <em>pieces</em>
        </div>
      </label>

      <label className="cc-cq-field">
        <span>Product name (optional)</span>
        <input
          disabled={busy}
          onChange={(event) => setProductName(event.target.value)}
          placeholder="e.g. Chef knife set"
          value={productName}
        />
      </label>

      <label className="cc-cq-field">
        <span>Note (optional)</span>
        <input
          disabled={busy}
          onChange={(event) => setNote(event.target.value)}
          placeholder="Anything we should know"
          value={note}
        />
      </label>

      {error ? <div className="cc-cq-error">{error}</div> : null}

      <button className="cc-cq-btn is-primary is-wide" disabled={busy} type="submit">
        {busy ? <LoaderCircle className="cc-spin" size={16} /> : <Plus size={16} strokeWidth={2.4} />}
        {busy ? "Sending request..." : "Send request"}
      </button>
    </form>
  );
}

function quoteImageIds(quote: Quotation) {
  return quote.imageFileIds?.length ? quote.imageFileIds : quote.imageFileId ? [quote.imageFileId] : [];
}

function quoteImageNames(quote: Quotation) {
  return quote.imageNames?.length ? quote.imageNames : quote.imageName ? [quote.imageName] : [];
}

function quoteTitle(quote: Quotation) {
  if (!quote.productName || quote.productName === "Product consultation") return "Product request";
  return quote.productName;
}

function latestActivity(quote: Quotation) {
  let latest = "";
  for (const update of quote.history || []) {
    if ((update.createdAt || "").localeCompare(latest) > 0) latest = update.createdAt;
  }
  return latest || quote.id;
}

function latestUpdate(quote: Quotation) {
  let latest: { detail: string; createdAt: string } | null = null;
  for (const update of quote.history || []) {
    if (!latest || update.createdAt.localeCompare(latest.createdAt) > 0) {
      latest = { detail: update.detail || update.label, createdAt: update.createdAt };
    }
  }
  return latest;
}

function pillTone(value: string) {
  const normalized = value.toLowerCase();
  if (/(reject|cancel)/.test(normalized)) return "danger";
  if (/(review|waiting|pending|requested|priced)/.test(normalized)) return "warn";
  if (/(accept|confirm|ready|complete)/.test(normalized)) return "ok";
  return "info";
}
