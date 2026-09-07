import { type FormEvent, useEffect, useRef, useState } from "react";
import { ArrowDownLeft, ArrowRight, ArrowUpRight, AlertTriangle, CheckCircle2, Clock3, Download, FileImage, Info, LoaderCircle, Megaphone, MessageCircle, Plus, Printer, Receipt, ShieldAlert, ShieldCheck, Truck, Upload, Wallet, X } from "lucide-react";
import { apiPost, apiUploadFile } from "./api";
import { accountPosition } from "./accounting";
import { MessengerDesk } from "./messenger-desk";
import { CustomerOrdersPage } from "./customer-orders";
import { CustomerQuotesPage } from "./customer-quotes";
import { ShippingDesk as ShippingOperationsDesk } from "./shipping-desk";
import { CustomerDirectoryPage } from "./trade-directory";
import { ProfileSettings } from "./profile-settings";
import {
  CustomerDocumentsPanel,
  VERIFY_IDENTITY_PAGE,
  customerNeedsIdentityAction,
  customerVerificationStage
} from "./customer-documents";
import {
  Empty,
  Panel,
  ProductFileThumb,
  RateCards,
  ShippingRows
} from "./shared";
import type { Customer, CustomerNotice, FeaturedProduct, LedgerEntry, Order, Payment, Product, Quotation, Session, Shipment, User, Workspace } from "./types";
import { date, digits, displayStageLabel, messageFromError, money, orderDisplayName, orderImageIds, paymentAllocationRows } from "./utils";

export function CustomerPortal({
  page,
  session,
  customer,
  workspace,
  onAction,
  onNavigate,
  onNotice,
  onUserChange,
  onCustomerUpdated
}: {
  page: string;
  session: Session;
  customer?: Customer;
  customers?: Customer[];
  workspace: Workspace;
  onAction: (message: string) => Promise<void>;
  onNavigate: (page: string) => void;
  onNotice: (notice: string) => void;
  onUserChange: (user: User) => void;
  onCustomerUpdated?: (customer: Customer) => void;
}) {
  const needsIdentity = customerNeedsIdentityAction(customer);
  const identityStage = customerVerificationStage(customer);

  return (
    <div className="cc-page-stack">
      {page === "Home" && (
        <CustomerHome
          workspace={workspace}
          session={session}
          onAction={onAction}
          onNavigate={onNavigate}
        />
      )}
      {page === VERIFY_IDENTITY_PAGE && customer && needsIdentity && (
        <CustomerDocumentsPanel
          customer={customer}
          mode="customer"
          onAction={onAction}
          onUpdated={(next) => {
            onCustomerUpdated?.(next);
            if (!customerNeedsIdentityAction(next)) {
              void onAction("We are verifying your documents. If anything further is needed, we will get back to you.");
            }
          }}
          session={session}
        />
      )}
      {page === VERIFY_IDENTITY_PAGE && (!customer || !needsIdentity) && (
        <section className="cc-docs-panel cc-identity-readonly">
          <header className="cc-docs-panel-head">
            <span className={`cc-identity-readonly-icon ${identityStage}`}>
              {identityStage === "verified" ? <ShieldCheck size={22} /> : <Clock3 size={22} />}
            </span>
            <div>
              <span>Identity</span>
              <strong>
                {identityStage === "verified"
                  ? "Account verified"
                  : identityStage === "ready_to_verify"
                    ? "Verification in review"
                    : identityStage === "in_review"
                      ? "Verification in review"
                      : "Verification has not started"}
              </strong>
              <p>
                {identityStage === "verified"
                  ? "Your company account is verified. There is no document upload page to reopen."
                  : identityStage === "ready_to_verify"
                    ? "We are verifying your documents. If anything further is needed, we will get back to you."
                    : identityStage === "in_review"
                      ? "We are verifying your documents. If anything further is needed, we will get back to you."
                      : "Start verification from Settings when you are ready."}
              </p>
            </div>
          </header>
        </section>
      )}

      {page === "Get Quote" && (
        <CustomerQuotesPage
          onAction={onAction}
          onNavigate={onNavigate}
          quotes={workspace.quotations}
          session={session}
        />
      )}
      {page === "Products" && (
        <section className="cc-work-grid">
          <Panel title="My product records" label="Catalog" action="Add" onAction={() => onNotice("Use the compact product form below.")}>
            <ProductManager products={workspace.products} session={session} onAction={onAction} />
          </Panel>
          <Panel title="Inventory health" label="Summary" action="Stock" onAction={() => onNotice("Stock belongs to the customer account.")}>
            <InventoryPulse products={workspace.products} />
          </Panel>
        </section>
      )}
      {page === "Orders" && (
        <CustomerOrdersPage
          orders={workspace.manufacturing}
          quotations={workspace.quotations}
          session={session}
        />
      )}
      {page === "Shipping" && (
        <ShippingOperationsDesk
          mode="customer"
          onAction={onAction}
          session={session}
          shipments={workspace.shipping}
        />
      )}
      {page === "Payments" && (
        <PaymentsPage onAction={onAction} session={session} workspace={workspace} />
      )}
      {page === "Directory" && <CustomerDirectoryPage onAction={onAction} session={session} />}
      {page === "Messages" && <MessengerDesk mode="customer" onAction={onAction} session={session} workspace={workspace} />}
      {page === "Settings" && (
        <ProfileSettings
          customer={customer}
          onAction={onAction}
          onCustomerUpdated={onCustomerUpdated}
          onNavigate={onNavigate}
          onSaved={onUserChange}
          session={session}
          verificationStatus={customer?.verificationStatus}
        />
      )}
    </div>
  );
}

export function IdentityActionNotice({ onOpen }: { onOpen: () => void }) {
  return (
    <button className="cc-identity-action" onClick={onOpen} type="button">
      <ShieldAlert size={16} />
      <span>
        <strong>Action required</strong>
        <small>Identity check</small>
      </span>
    </button>
  );
}

function homeGreeting(name: string) {
  const hour = new Date().getHours();
  const time = hour < 12 ? "Good morning" : hour < 17 ? "Good afternoon" : "Good evening";
  const first = name.trim().split(/\s+/)[0] || "";
  return first ? `${time}, ${first}` : time;
}

function readinessText(expectedDate: string) {
  const raw = expectedDate.trim();
  if (!raw) return "ETA not set";
  const stamp = Date.parse(raw);
  if (!Number.isFinite(stamp)) return `Ready ${date(raw)}`;
  const days = Math.ceil((stamp - Date.now()) / 86400000);
  if (days < 0) return "Past due";
  if (days === 0) return "Ready today";
  if (days === 1) return "Ready tomorrow";
  return `Ready in ${days} days`;
}

function isClosedWorkStatus(status: string) {
  return ["Completed", "Cancelled", "Delivered", "Rejected"].includes(status);
}

function workspaceHydrated(workspace: Workspace) {
  return Boolean(workspace.pagination && Object.keys(workspace.pagination).length > 0);
}


function CustomerHome({
  workspace,
  session,
  onAction,
  onNavigate
}: {
  workspace: Workspace;
  session: Session;
  onAction: (message: string) => Promise<void>;
  onNavigate: (page: string) => void;
}) {
  const openOrders = workspace.manufacturing
    .filter((order) => !isClosedWorkStatus(order.status))
    .slice()
    .sort((a, b) => Date.parse(b.createdAt || "") - Date.parse(a.createdAt || ""));
  const pricedQuotes = workspace.quotations
    .filter((quote) => quote.status === "Priced")
    .slice()
    .sort((a, b) => Date.parse(b.createdAt || "") - Date.parse(a.createdAt || ""));
  const shipments = workspace.shipping
    .filter((row) => !isClosedWorkStatus(row.status))
    .slice()
    .sort((a, b) => Date.parse(b.createdAt || "") - Date.parse(a.createdAt || ""));
  const notices = (workspace.notices || []).filter((row) => row.active).slice(0, 5);
  const featured = (workspace.featuredProducts || [])
    .filter((row) => row.active)
    .slice()
    .sort((a, b) => a.sortOrder - b.sortOrder)
    .slice(0, 20);
  const ready = workspaceHydrated(workspace);
  const position = accountPosition(workspace);
  const due = position.due;
  const credit = position.credit;
  const hasCredit = credit > 0;
  const hasDue = due > 0;
  const orderCount = Object.prototype.hasOwnProperty.call(workspace.metrics || {}, "activeOrders")
    ? Number(workspace.metrics.activeOrders || 0)
    : openOrders.length;
  const quoteCount = Object.prototype.hasOwnProperty.call(workspace.metrics || {}, "pendingQuotes")
    ? Number(workspace.metrics.pendingQuotes || 0)
    : workspace.quotations.filter((q) => q.status === "Requested" || q.status === "Priced").length;
  const shipCount = shipments.length;

  return (
    <section className="cc-home">
      <div className="cc-home-atmosphere" aria-hidden>
        <span className="cc-home-blob is-teal" />
        <span className="cc-home-blob is-blue" />
        <span className="cc-home-blob is-mint" />
      </div>

      <div className="cc-home-shell">
        <header className="cc-home-hero">
          <p className="cc-home-greeting">{homeGreeting(session.user.name)}</p>

          <button
            className={`cc-home-balance${hasCredit ? " is-credit" : hasDue ? "" : " is-clear"}`}
            onClick={() => onNavigate("Payments")}
            type="button"
          >
            <span className="cc-home-balance-copy">
              <em>{hasCredit ? "Account credit" : hasDue ? "Balance due" : "Account clear"}</em>
              <strong>{ready ? money(hasCredit ? credit : due) : "—"}</strong>
            </span>
            <span className="cc-home-balance-cta">{ready && hasDue ? "Pay" : "Ledger"}</span>
          </button>

          <div className="cc-home-gauges">
            <HomeGauge
              color="#5ECF9A"
              label="Orders"
              onClick={() => onNavigate("Orders")}
              progress={Math.min(1, Math.max(0.08, orderCount / 12))}
              value={orderCount}
            />
            <HomeGauge
              color="#6BA8C9"
              label="Quotes"
              onClick={() => onNavigate("Get Quote")}
              progress={Math.min(1, Math.max(0.08, quoteCount / 8))}
              value={quoteCount}
            />
            <HomeGauge
              color="#B7C978"
              label="Shipping"
              onClick={() => onNavigate("Shipping")}
              progress={Math.min(1, Math.max(0.08, shipCount / 6))}
              value={shipCount}
            />
          </div>
        </header>

        {notices.length > 0 && (
          <div className="cc-home-notices">
            {notices.map((notice) => (
              <HomeNoticeBanner key={notice.id} notice={notice} />
            ))}
          </div>
        )}

        <div className="cc-home-block is-plain">
          <header className="cc-home-head">
            <div>
              <h2>{openOrders.length ? `Open orders · ${openOrders.length}` : "Open orders"}</h2>
            </div>
            <button className="cc-home-view-all" onClick={() => onNavigate("Orders")} type="button">
              See all <ArrowRight size={14} strokeWidth={2.4} />
            </button>
          </header>
          {openOrders.length ? (
            <HomeOrdersCarousel
              orders={openOrders.slice(0, 20)}
              quotations={workspace.quotations}
              session={session}
              onOpen={() => onNavigate("Orders")}
            />
          ) : (
            <div className="cc-home-empty">
              <p>No active orders yet.</p>
              <button
                className="cc-button cc-primary"
                onClick={() => {
                  sessionStorage.setItem("cc-open-quote-composer", "1");
                  onNavigate("Get Quote");
                }}
                type="button"
              >
                Get a quote
              </button>
            </div>
          )}
        </div>

        <div className="cc-home-actions">
          <button
            className="cc-home-action"
            onClick={() => {
              sessionStorage.setItem("cc-open-quote-composer", "1");
              onNavigate("Get Quote");
            }}
            type="button"
          >
            <span className="cc-home-action-icon">
              <Plus size={18} />
            </span>
            <span>Quote</span>
          </button>
          <button className="cc-home-action" onClick={() => onNavigate("Shipping")} type="button">
            <span className="cc-home-action-icon">
              <Truck size={18} />
            </span>
            <span>Ship</span>
          </button>
          <button className="cc-home-action" onClick={() => onNavigate("Payments")} type="button">
            <span className="cc-home-action-icon">
              <Wallet size={18} />
            </span>
            <span>Pay</span>
          </button>
          <button className="cc-home-action" onClick={() => onNavigate("Messages")} type="button">
            <span className="cc-home-action-icon">
              <MessageCircle size={18} />
            </span>
            <span>Support</span>
          </button>
        </div>

        {pricedQuotes.length > 0 && (
          <div className="cc-home-block is-plain">
            <header className="cc-home-head">
              <div>
                <h2>{pricedQuotes.length === 1 ? "Quote ready" : `Quotes ready · ${pricedQuotes.length}`}</h2>
              </div>
              <button className="cc-home-view-all" onClick={() => onNavigate("Get Quote")} type="button">
                See all <ArrowRight size={14} strokeWidth={2.4} />
              </button>
            </header>
            <div className="cc-home-quote-scroller">
              <div className="cc-home-quote-track">
                {pricedQuotes.slice(0, 12).map((quote) => (
                  <HomeQuoteStatus
                    key={quote.id}
                    quote={quote}
                    session={session}
                    onAction={onAction}
                    onOpen={() => onNavigate("Get Quote")}
                  />
                ))}
              </div>
            </div>
          </div>
        )}

        {featured.length > 0 && (
          <div className="cc-home-block is-plain">
            <header className="cc-home-head">
              <div>
                <h2>Featured</h2>
              </div>
              <button className="cc-home-view-all" onClick={() => onNavigate("Products")} type="button">
                Catalog <ArrowRight size={14} strokeWidth={2.4} />
              </button>
            </header>
            <HomeFeaturedCarousel
              items={featured}
              session={session}
              onOpen={() => onNavigate("Products")}
            />
          </div>
        )}

        {shipments.length > 0 && (
          <div className="cc-home-block is-plain">
            <header className="cc-home-head">
              <div>
                <h2>Shipments</h2>
              </div>
              <button className="cc-home-view-all" onClick={() => onNavigate("Shipping")} type="button">
                See all <ArrowRight size={14} strokeWidth={2.4} />
              </button>
            </header>
            <div className="cc-home-shipments">
              {shipments.slice(0, 2).map((shipment) => (
                <button className="cc-home-shipment" key={shipment.id} onClick={() => onNavigate("Shipping")} type="button">
                  <span className="cc-home-shipment-icon">
                    <Truck size={18} />
                  </span>
                  <span className="cc-home-shipment-copy">
                    <strong>{shipment.courier || shipment.type || shipment.id}</strong>
                    <small>
                      {shipment.destination || "Destination pending"}
                      {shipment.tracking ? ` · ${shipment.tracking}` : ""}
                    </small>
                  </span>
                  <b>{shipment.status}</b>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}

function HomeOrdersCarousel({
  orders,
  quotations,
  session,
  onOpen
}: {
  orders: Order[];
  quotations: Quotation[];
  session: Session;
  onOpen: () => void;
}) {
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const [active, setActive] = useState(0);
  const dotCount = Math.min(orders.length, 8);

  useEffect(() => {
    const node = scrollerRef.current;
    if (!node) return;
    const sync = () => {
      const cards = Array.from(node.querySelectorAll<HTMLElement>(".cc-home-order-card"));
      if (!cards.length) return;
      const left = node.scrollLeft;
      let best = 0;
      let bestDist = Number.POSITIVE_INFINITY;
      cards.forEach((card, index) => {
        const dist = Math.abs(card.offsetLeft - left);
        if (dist < bestDist) {
          bestDist = dist;
          best = index;
        }
      });
      setActive(Math.min(best, Math.max(0, dotCount - 1)));
    };
    sync();
    node.addEventListener("scroll", sync, { passive: true });
    window.addEventListener("resize", sync);
    return () => {
      node.removeEventListener("scroll", sync);
      window.removeEventListener("resize", sync);
    };
  }, [orders, dotCount]);

  const jumpTo = (index: number) => {
    const node = scrollerRef.current;
    if (!node) return;
    const cards = Array.from(node.querySelectorAll<HTMLElement>(".cc-home-order-card"));
    const card = cards[index];
    if (!card) return;
    node.scrollTo({ left: card.offsetLeft, behavior: "smooth" });
  };

  return (
    <div className="cc-home-orders-carousel">
      <div className="cc-home-order-scroller" ref={scrollerRef}>
        <div className="cc-home-order-track">
          {orders.map((order) => {
            const imageId = orderImageIds(order, quotations)[0] || "";
            const progress = Math.max(0, Math.min(100, order.progress || 0));
            const stage = displayStageLabel(order.currentStage || order.status);
            return (
              <div
                className="cc-home-order-card is-compact"
                key={order.id}
                onClick={onOpen}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    onOpen();
                  }
                }}
                role="button"
                tabIndex={0}
              >
                <ProductFileThumb className="is-card-sq" fileId={imageId} name={orderDisplayName(order, quotations)} token={session.token} />
                <span className="cc-home-order-main">
                  <strong className="cc-home-order-code">{order.id}</strong>
                  <span className="cc-home-order-stage">{stage}</span>
                  <span className="cc-home-order-eta">{readinessText(order.expectedDate || "")}</span>
                  <span className="cc-home-order-progress">
                    <span className="cc-home-bar is-animated" aria-hidden>
                      <i style={{ width: `${Math.max(8, progress)}%` }} />
                    </span>
                    <b>{progress}%</b>
                  </span>
                </span>
              </div>
            );
          })}
        </div>
      </div>
      {orders.length > 1 && (
        <div className="cc-home-carousel-dots" aria-label="Order carousel position">
          {Array.from({ length: dotCount }, (_, index) => (
            <button
              aria-label={`Show order ${index + 1}`}
              className={`cc-home-carousel-dot${index === active ? " is-active" : ""}`}
              key={index}
              onClick={() => jumpTo(index)}
              type="button"
            />
          ))}
        </div>
      )}
    </div>
  );
}

function HomeGauge({
  label,
  value,
  progress,
  color,
  onClick
}: {
  label: string;
  value: number;
  progress: number;
  color: string;
  onClick: () => void;
}) {
  const pct = Math.round(Math.min(1, Math.max(0, progress)) * 100);
  return (
    <button
      className="cc-home-gauge"
      onClick={onClick}
      style={{ ["--gauge-color" as string]: color, ["--gauge-p" as string]: String(pct) }}
      type="button"
    >
      <span className="cc-home-gauge-ring">
        <strong>{value}</strong>
      </span>
      <em>{label}</em>
    </button>
  );
}

function HomeNoticeBanner({ notice }: { notice: CustomerNotice }) {
  const tone = notice.tone || "info";
  const Icon = tone === "success" ? Megaphone : tone === "warning" ? AlertTriangle : Info;
  return (
    <article className={`cc-home-notice is-${tone}`}>
      <Icon className="cc-home-notice-icon" size={20} strokeWidth={2.2} />
      <div>
        <strong>{notice.title}</strong>
        {notice.body.trim() ? <p>{notice.body}</p> : null}
      </div>
    </article>
  );
}

function HomeFeaturedCarousel({
  items,
  session,
  onOpen
}: {
  items: FeaturedProduct[];
  session: Session;
  onOpen: () => void;
}) {
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const [active, setActive] = useState(0);
  const dotCount = Math.min(items.length, 8);

  useEffect(() => {
    const node = scrollerRef.current;
    if (!node) return;
    const sync = () => {
      const cards = Array.from(node.querySelectorAll<HTMLElement>(".cc-home-featured"));
      if (!cards.length) return;
      const left = node.scrollLeft;
      let best = 0;
      let bestDist = Number.POSITIVE_INFINITY;
      cards.forEach((card, index) => {
        const dist = Math.abs(card.offsetLeft - left);
        if (dist < bestDist) {
          bestDist = dist;
          best = index;
        }
      });
      setActive(Math.min(best, Math.max(0, dotCount - 1)));
    };
    sync();
    node.addEventListener("scroll", sync, { passive: true });
    window.addEventListener("resize", sync);
    return () => {
      node.removeEventListener("scroll", sync);
      window.removeEventListener("resize", sync);
    };
  }, [items, dotCount]);

  const jumpTo = (index: number) => {
    const node = scrollerRef.current;
    if (!node) return;
    const cards = Array.from(node.querySelectorAll<HTMLElement>(".cc-home-featured"));
    const card = cards[index];
    if (!card) return;
    node.scrollTo({ left: card.offsetLeft, behavior: "smooth" });
  };

  return (
    <div className="cc-home-featured-carousel">
      <div className="cc-home-featured-scroller" ref={scrollerRef}>
        <div className="cc-home-featured-track">
          {items.map((item) => (
            <HomeFeaturedCard key={item.id} item={item} session={session} onOpen={onOpen} />
          ))}
        </div>
      </div>
      {items.length > 1 && (
        <div className="cc-home-carousel-dots" aria-label="Featured carousel position">
          {Array.from({ length: dotCount }, (_, index) => (
            <button
              aria-label={`Show featured item ${index + 1}`}
              className={`cc-home-carousel-dot${index === active ? " is-active" : ""}`}
              key={index}
              onClick={() => jumpTo(index)}
              type="button"
            />
          ))}
        </div>
      )}
    </div>
  );
}

function HomeFeaturedCard({
  item,
  session,
  onOpen
}: {
  item: FeaturedProduct;
  session: Session;
  onOpen: () => void;
}) {
  return (
    <button className="cc-home-featured" onClick={onOpen} type="button">
      <ProductFileThumb className="cc-home-featured-thumb" fileId={item.imageFileId} name={item.name} token={session.token} />
      {item.tag ? <span className="cc-home-featured-tag">{item.tag}</span> : null}
      <strong>{item.name}</strong>
    </button>
  );
}

function QuoteForm({
  products,
  session,
  onAction,
  onNotice
}: {
  products: Product[];
  session: Session;
  onAction: (message: string) => Promise<void>;
  onNotice: (notice: string) => void;
}) {
  const [catalog, setCatalog] = useState(products.length > 0);
  const [productId, setProductId] = useState(products[0]?.id || "");
  const [form, setForm] = useState({ productName: "", quantity: "50", imageName: "", notes: "" });
  const [imageFile, setImageFile] = useState<File | null>(null);
  const [fileKey, setFileKey] = useState(0);

  useEffect(() => {
    if (!productId && products[0]?.id) {
      setProductId(products[0].id);
    }
  }, [productId, products]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    try {
      let uploaded: { id: string; originalName: string } | null = null;
      if (imageFile) {
        onNotice("Uploading product image...");
        uploaded = await apiUploadFile(
          imageFile,
          { customerId: session.user.customerId || "", ownerType: "quotation_photo" },
          session.token
        );
      }

      await apiPost<Quotation>(
        "/api/workflow/quotes",
        {
          customerId: session.user.customerId,
          productId: catalog ? productId : "",
          productName: catalog ? "" : form.productName,
          quantity: Number(form.quantity),
          imageName: uploaded?.originalName || form.imageName,
          imageFileId: uploaded?.id || "",
          notes: form.notes
        },
        session.token
      );
      setForm({ productName: "", quantity: "50", imageName: "", notes: "" });
      setImageFile(null);
      setFileKey((current) => current + 1);
      await onAction("Quote request submitted.");
    } catch (error) {
      onNotice(messageFromError(error, "Quote request failed."));
    }
  };

  return (
    <form className="cc-modern-form cc-quote-shell" onSubmit={submit}>
      <div className="cc-toggle-row cc-span">
        <button className={catalog ? "selected" : ""} disabled={products.length === 0} onClick={() => setCatalog(true)} type="button">
          Existing product
        </button>
        <button className={!catalog ? "selected" : ""} onClick={() => setCatalog(false)} type="button">
          New product
        </button>
      </div>
      {catalog ? (
        <label className="cc-span">
          Product
          <select onChange={(event) => setProductId(event.target.value)} value={productId}>
            {products.map((product) => (
              <option key={product.id} value={product.id}>
                {product.sku} / {product.name}
              </option>
            ))}
          </select>
        </label>
      ) : (
        <label className="cc-span">
          Product or design name
          <input required onChange={(event) => setForm((current) => ({ ...current, productName: event.target.value }))} value={form.productName} />
        </label>
      )}
      <label>
        Quantity
        <input inputMode="numeric" onChange={(event) => setForm((current) => ({ ...current, quantity: digits(event.target.value) }))} required value={form.quantity} />
      </label>
      <label>
        Product image
        <input
          accept="image/png,image/jpeg,image/webp"
          key={fileKey}
          onChange={(event) => {
            const file = event.target.files?.[0] || null;
            setImageFile(file);
            setForm((current) => ({ ...current, imageName: file?.name || "" }));
          }}
          type="file"
        />
      </label>
      <label className="cc-span">
        Notes
        <textarea onChange={(event) => setForm((current) => ({ ...current, notes: event.target.value }))} rows={3} value={form.notes} />
      </label>
      {form.imageName && <div className="cc-file-pill cc-span">Image ready: {form.imageName}</div>}
      <button className="cc-button cc-primary cc-span" type="submit">
        Submit quote request
      </button>
    </form>
  );
}
function QuoteList({ quotes, session, onAction }: { quotes: Quotation[]; session: Session; onAction: (message: string) => Promise<void> }) {
  if (quotes.length === 0) return <Empty title="No quotations" detail="Submitted requests and company prices appear here." />;
  return <div className="cc-quote-list">{quotes.map((quote) => <QuoteCard key={quote.id} quote={quote} session={session} onAction={onAction} />)}</div>;
}

function HomeQuoteStatus({
  quote,
  session,
  onAction,
  onOpen
}: {
  quote: Quotation;
  session: Session;
  onAction: (message: string) => Promise<void>;
  onOpen: () => void;
}) {
  const act = async (action: "accept" | "reject") => {
    await apiPost<unknown>(`/api/workflow/quotes/${quote.id}/${action}`, {}, session.token);
    await onAction(action === "accept" ? "Quotation accepted. Manufacturing order created." : "Quotation rejected.");
  };
  const imageId = quote.imageFileId || quote.imageFileIds?.[0] || "";
  const unit =
    quote.quantity > 0 && quote.totalAmount
      ? Math.round(quote.totalAmount / quote.quantity)
      : 0;

  return (
    <article className="cc-home-quote is-priced">
      <button className="cc-home-quote-main" onClick={onOpen} type="button">
        <ProductFileThumb className="cc-home-quote-thumb" fileId={imageId} name={quote.productName} token={session.token} />
        <div className="cc-home-quote-copy">
          <div className="cc-home-quote-top">
            <span>{quote.id}</span>
            <b>Priced</b>
          </div>
          <strong>{quote.productName}</strong>
          <small>
            <em className="cc-home-quote-qty">{quote.quantity} pcs</em>
            {quote.expectedDate ? ` · ready ${date(quote.expectedDate)}` : ""}
          </small>
        </div>
      </button>
      <div className="cc-home-quote-meta" aria-label="Quote amounts">
        <span>
          <em>Unit</em>
          <b>{unit ? money(unit) : "—"}</b>
        </span>
        <span>
          <em>Total</em>
          <b>{quote.totalAmount ? money(quote.totalAmount) : "—"}</b>
        </span>
      </div>
      <div className="cc-home-quote-actions">
        <button className="cc-button cc-primary" onClick={() => act("accept")} type="button">
          Proceed
        </button>
        <button className="cc-button" onClick={() => act("reject")} type="button">
          Reject
        </button>
      </div>
    </article>
  );
}

function QuoteCard({
  quote,
  session,
  onAction,
  compact = false
}: {
  quote: Quotation;
  session: Session;
  onAction: (message: string) => Promise<void>;
  compact?: boolean;
}) {
  const act = async (action: "accept" | "reject") => {
    await apiPost<unknown>(`/api/workflow/quotes/${quote.id}/${action}`, {}, session.token);
    await onAction(action === "accept" ? "Quotation accepted. Manufacturing order created." : "Quotation rejected.");
  };

  return (
    <article className={compact ? "cc-quote-card compact" : "cc-quote-card"}>
      <div className="cc-work-head">
        <div>
          <span>{quote.id}</span>
          <h3>{quote.productName}</h3>
          <p>
            {quote.quantity} pcs / {quote.imageName || "No image"}
          </p>
        </div>
        <strong>{quote.status}</strong>
      </div>
      <div className="cc-quote-prices">
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
      {!compact && quote.notes && <p className="cc-soft-note">{quote.notes}</p>}
      {quote.status === "Priced" && (
        <div className="cc-row-actions">
          <button className="cc-button cc-primary" onClick={() => act("accept")} type="button">
            Proceed to order
          </button>
          <button className="cc-button cc-danger" onClick={() => act("reject")} type="button">
            Reject
          </button>
        </div>
      )}
    </article>
  );
}

function ProductManager({ products, session, onAction }: { products: Product[]; session: Session; onAction: (message: string) => Promise<void> }) {
  const [form, setForm] = useState({ sku: "", name: "", stock: "0", imageName: "" });
  const [imageFile, setImageFile] = useState<File | null>(null);
  const [fileKey, setFileKey] = useState(0);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    let uploaded: { id: string; originalName: string } | null = null;
    if (imageFile) {
      uploaded = await apiUploadFile(imageFile, { customerId: session.user.customerId || "", ownerType: "product_image" }, session.token);
    }
    await apiPost<Product>(
      "/api/workflow/products",
      {
        customerId: session.user.customerId,
        sku: form.sku,
        name: form.name,
        stock: Number(form.stock),
        image: uploaded?.originalName || form.imageName,
        imageFileId: uploaded?.id || ""
      },
      session.token
    );
    setForm({ sku: "", name: "", stock: "0", imageName: "" });
    setImageFile(null);
    setFileKey((current) => current + 1);
    await onAction("Product saved.");
  };

  return (
    <div className="cc-product-manager">
      <form className="cc-inline-form" onSubmit={submit}>
        <input onChange={(event) => setForm((current) => ({ ...current, sku: event.target.value }))} placeholder="SKU" value={form.sku} />
        <input required onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} placeholder="Product name" value={form.name} />
        <input inputMode="numeric" onChange={(event) => setForm((current) => ({ ...current, stock: digits(event.target.value) }))} placeholder="Stock" value={form.stock} />
        <input
          accept="image/png,image/jpeg,image/webp"
          key={fileKey}
          onChange={(event) => {
            const file = event.target.files?.[0] || null;
            setImageFile(file);
            setForm((current) => ({ ...current, imageName: file?.name || "" }));
          }}
          type="file"
        />
        <button className="cc-button cc-primary" type="submit">
          Save
        </button>
      </form>
      {form.imageName && <div className="cc-file-pill">Image ready: {form.imageName}</div>}
      <ProductRows products={products} />
    </div>
  );
}
function ProductRows({ products }: { products: Product[] }) {
  if (products.length === 0) return <Empty title="No products yet" detail="Add products to reuse them in quotes and shipping." />;
  return (
    <div className="cc-rows">
      {products.map((product) => (
        <div key={product.id}>
          <span>{product.sku}</span>
          <strong>{product.name}</strong>
          <small>
            {product.stock} on hand / {product.reserved} reserved
          </small>
        </div>
      ))}
    </div>
  );
}

function InventoryPulse({ products }: { products: Product[] }) {
  const total = products.reduce((sum, item) => sum + item.stock, 0);
  const reserved = products.reduce((sum, item) => sum + item.reserved, 0);

  return (
    <div className="cc-rate-card">
      <div>
        <span>Total stock</span>
        <strong>{total}</strong>
      </div>
      <div>
        <span>Reserved</span>
        <strong>{reserved}</strong>
      </div>
      <div>
        <span>Available</span>
        <strong>{total - reserved}</strong>
      </div>
    </div>
  );
}

function ShippingDesk({ workspace, session, onAction }: { workspace: Workspace; session: Session; onAction: (message: string) => Promise<void> }) {
  const [form, setForm] = useState({
    type: "Manufactured goods",
    manufacturingId: workspace.manufacturing[0]?.id || "",
    courier: "FedEx",
    service: "Duty paid premium",
    destination: "",
    zone: "7",
    weight: "0-5 kg"
  });

  useEffect(() => {
    if (form.type === "Manufactured goods" && !form.manufacturingId && workspace.manufacturing[0]?.id) {
      setForm((current) => ({ ...current, manufacturingId: workspace.manufacturing[0].id }));
    }
  }, [form.manufacturingId, form.type, workspace.manufacturing]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    await apiPost<Shipment>("/api/workflow/shipping", { ...form, customerId: session.user.customerId }, session.token);
    setForm((current) => ({ ...current, destination: "" }));
    await onAction("Shipping request created with current rate.");
  };

  return (
    <div className="cc-shipping-desk">
      <div className="cc-toggle-row">
        <button className={form.type === "Manufactured goods" ? "selected" : ""} onClick={() => setForm((current) => ({ ...current, type: "Manufactured goods" }))} type="button">
          From ChakuChuri order
        </button>
        <button className={form.type === "Outside product" ? "selected" : ""} onClick={() => setForm((current) => ({ ...current, type: "Outside product", manufacturingId: "" }))} type="button">
          Outside product
        </button>
      </div>
      <form className="cc-modern-form cc-shipping-form" onSubmit={submit}>
        {form.type === "Manufactured goods" && (
          <label>
            Order
            <select onChange={(event) => setForm((current) => ({ ...current, manufacturingId: event.target.value }))} value={form.manufacturingId}>
              {workspace.manufacturing.map((order) => (
                <option key={order.id} value={order.id}>
                  {order.id} / {order.productName}
                </option>
              ))}
            </select>
          </label>
        )}
        <label>
          Courier
          <select onChange={(event) => setForm((current) => ({ ...current, courier: event.target.value }))} value={form.courier}>
            <option>FedEx</option>
            <option>DHL</option>
            <option>UPS</option>
            <option>Other</option>
          </select>
        </label>
        <label>
          Zone
          <select onChange={(event) => setForm((current) => ({ ...current, zone: event.target.value }))} value={form.zone}>
            <option>7</option>
            <option>6</option>
            <option>5</option>
            <option>4</option>
          </select>
        </label>
        <label>
          Weight
          <select onChange={(event) => setForm((current) => ({ ...current, weight: event.target.value }))} value={form.weight}>
            <option>0-5 kg</option>
            <option>5-10 kg</option>
            <option>10-20 kg</option>
          </select>
        </label>
        <label className="cc-span">
          Destination
          <input required onChange={(event) => setForm((current) => ({ ...current, destination: event.target.value }))} value={form.destination} />
        </label>
        <button className="cc-button cc-primary cc-span" type="submit">
          Create shipping request
        </button>
      </form>
      <ShippingRows shipping={workspace.shipping} />
    </div>
  );
}

function PaymentsPage({
  workspace,
  session,
  onAction
}: {
  workspace: Workspace;
  session: Session;
  onAction: (message: string) => Promise<void>;
}) {
  const [uploadOpen, setUploadOpen] = useState(false);
  const [tab, setTab] = useState<"proofs" | "statement">("proofs");

  const dueOrders = workspace.manufacturing.filter(
    (order) => order.balanceDue > 0 && order.status !== "Cancelled" && order.status !== "Completed"
  );
  const position = accountPosition(workspace);
  const accountDue = position.due;
  const credit = position.credit;
  const pending = position.pending;
  const confirmed = position.received;
  const charges = position.charges;
  const recentProofs = [...workspace.payments].sort((left, right) => right.createdAt.localeCompare(left.createdAt)).slice(0, 12);
  const statement = [...workspace.ledger].sort((left, right) => right.postedAt.localeCompare(left.postedAt));
  const settled = charges > 0 ? Math.min(1, Math.max(0, confirmed / charges)) : 0;
  const hasDue = accountDue > 0;
  const hasCredit = credit > 0;
  const allocationRows = paymentAllocationRows(workspace);

  const downloadStatement = () => {
    const rows = statement.map((entry) => [
      entry.postedAt,
      entry.entryNo,
      entry.sourceType,
      entry.sourceId,
      ledgerReferences(entry),
      entry.note,
      entry.debit || 0,
      entry.credit || 0,
      entry.currency || "PKR"
    ]);
    const csv = [["Date", "Entry", "Source type", "Source ID", "Order / shipping / payment references", "Description", "Debit", "Credit", "Currency"], ...rows]
      .map((row) => row.map((value) => `"${String(value).replace(/"/g, '""')}"`).join(","))
      .join("\r\n");
    const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `chakuchuri-statement-${new Date().toISOString().slice(0, 10)}.csv`;
    anchor.click();
    URL.revokeObjectURL(url);
    void onAction("Statement downloaded with order, shipment and payment references.");
  };

  const printStatement = () => {
    document.body.classList.add("cc-print-customer-statement");
    const cleanUp = () => document.body.classList.remove("cc-print-customer-statement");
    window.addEventListener("afterprint", cleanUp, { once: true });
    window.setTimeout(() => {
      window.print();
      window.setTimeout(cleanUp, 700);
    }, 80);
  };

  useEffect(() => {
    if (!uploadOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setUploadOpen(false);
    };
    document.addEventListener("keydown", onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = previous;
    };
  }, [uploadOpen]);

  return (
    <section className="cc-payments-page">
      <div className="cc-payments-hero">
        <div className="cc-payments-hero-top">
          <span className="cc-payments-hero-icon" aria-hidden>
            {hasDue ? <Wallet size={20} strokeWidth={1.9} /> : <ShieldCheck size={20} strokeWidth={1.9} />}
          </span>
          <em>{hasCredit ? "ACCOUNT CREDIT" : hasDue ? "BALANCE DUE" : "ACCOUNT CLEAR"}</em>
        </div>
        <strong>{money(hasCredit ? credit : accountDue)}</strong>
        {charges > 0 ? (
          <div className="cc-payments-hero-progress">
            <span className="cc-payments-hero-bar">
              <i style={{ width: `${Math.round(settled * 100)}%` }} />
            </span>
            <b>{Math.round(settled * 100)}% paid</b>
          </div>
        ) : null}
        {pending > 0 ? (
          <div className="cc-payments-hero-pending">
            <Clock3 size={16} strokeWidth={2} />
            <span>{money(pending)} proof under review</span>
          </div>
        ) : null}
        <button className="cc-payments-hero-cta" onClick={() => setUploadOpen(true)} type="button">
          <Upload size={16} strokeWidth={2.2} />
          {hasDue ? "Upload payment proof" : "Upload proof"}
        </button>
      </div>

      <div className="cc-payments-metrics" aria-label="Payment summary">
        <div>
          <span>Charges</span>
          <strong>{money(charges)}</strong>
        </div>
        <div>
          <span>Paid</span>
          <strong>{money(confirmed)}</strong>
        </div>
        <div>
          <span>Pending</span>
          <strong>{money(pending)}</strong>
        </div>
      </div>

      {allocationRows.length > 0 && accountDue > 0 ? (
        <div className="cc-payments-alloc">
          <strong>Payment Allocation</strong>
          <small>Still unpaid · payments clear oldest charges first</small>
          <ul>
            {allocationRows.map((row) => (
              <li key={row.id}>
                <span>{row.label}</span>
                <em>{money(row.amount)}</em>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      <div className="cc-payments-tabs" role="tablist" aria-label="Payment records">
        <button
          aria-selected={tab === "proofs"}
          className={tab === "proofs" ? "active" : ""}
          onClick={() => setTab("proofs")}
          role="tab"
          type="button"
        >
          <Receipt size={15} strokeWidth={2} />
          Proofs
        </button>
        <button
          aria-selected={tab === "statement"}
          className={tab === "statement" ? "active" : ""}
          onClick={() => setTab("statement")}
          role="tab"
          type="button"
        >
          <Wallet size={15} strokeWidth={2} />
          Statement
        </button>
      </div>

      {tab === "proofs" ? (
        <div className="cc-payments-section">
          <header>
            <h2>Payment proofs</h2>
            <span>{workspace.payments.length}</span>
          </header>
          {recentProofs.length ? (
            <div className="cc-payments-proof-list">
              {recentProofs.map((payment) => (
                <PaymentProofTile key={payment.id} payment={payment} />
              ))}
            </div>
          ) : (
            <div className="cc-payments-empty">
              <Receipt size={24} strokeWidth={1.75} />
              <strong>No proofs yet</strong>
              <p>Upload your transfer screenshot here.</p>
            </div>
          )}
        </div>
      ) : (
        <div className="cc-payments-section">
          <header className="cc-payments-section-head">
            <div>
              <h2>Account statement</h2>
              <span>{workspace.ledger.length} entries</span>
            </div>
            <div className="cc-payments-statement-actions">
              <button aria-label="Download account statement" onClick={downloadStatement} title="Download statement CSV" type="button"><Download size={16} /> <span>Download</span></button>
              <button aria-label="Print account statement" onClick={printStatement} title="Print or save as PDF" type="button"><Printer size={16} /> <span>Print / PDF</span></button>
            </div>
          </header>
          {statement.length ? (
            <div className="cc-payments-ledger-list">
              {statement.map((entry) => (
                <LedgerTile key={entry.id} entry={entry} />
              ))}
            </div>
          ) : (
            <div className="cc-payments-empty">
              <Wallet size={24} strokeWidth={1.75} />
              <strong>No statement entries yet</strong>
            </div>
          )}
        </div>
      )}

      {uploadOpen ? (
        <div
          className="cc-payment-composer-backdrop"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setUploadOpen(false);
          }}
        >
          <section aria-labelledby="cc-payment-composer-title" aria-modal="true" className="cc-payment-composer-dialog" role="dialog">
            <PaymentProofForm
              onAction={async (message) => {
                await onAction(message);
                setUploadOpen(false);
              }}
              onClose={() => setUploadOpen(false)}
              session={session}
              workspace={workspace}
            />
          </section>
        </div>
      ) : null}
    </section>
  );
}

function PaymentProofTile({ payment }: { payment: Payment }) {
  const confirmed = payment.status === "Confirmed";
  const rejected = payment.status === "Rejected";
  const tone = confirmed ? "ok" : rejected ? "danger" : "warn";
  const status = confirmed ? "Confirmed" : rejected ? "Needs attention" : "Under review";
  const detail = confirmed
    ? "Applied to your account"
    : rejected
      ? "Please upload a new proof"
      : "We are checking this payment";
  const Icon = confirmed ? CheckCircle2 : rejected ? AlertTriangle : Clock3;

  return (
    <article className={`cc-payments-proof-tile is-${tone}`}>
      <span className="cc-payments-proof-icon" aria-hidden>
        <Icon size={20} strokeWidth={2} />
      </span>
      <div>
        <strong>{payment.type}</strong>
        <div className="cc-payments-proof-row">
          <em>{money(payment.amount)}</em>
          <b>{status}</b>
        </div>
        <small>
          {detail} / {date(payment.createdAt)}
        </small>
      </div>
    </article>
  );
}

function compactId(value: string): string {
  const clean = value.trim();
  if (!clean) return "";
  const parts = clean.split("-");
  if (parts.length > 1) return `${parts[0]}-${parts[parts.length - 1].slice(-6)}`;
  return clean.length > 6 ? clean.slice(-6) : clean;
}

function ledgerReferences(entry: LedgerEntry): string {
  const source = entry.sourceType.toLowerCase();
  if (source === "payment" || source === "payment_reversal") return `Payment ID: ${compactId(entry.sourceId)}`;
  if (source.includes("manufacturing")) return `Order ID: ${compactId(entry.sourceId)}`;
  if (source.includes("shipping")) return `Shipping ID: ${compactId(entry.sourceId)}`;
  return "";
}function LedgerTile({ entry }: { entry: LedgerEntry }) {
  const references = ledgerReferences(entry);
  const debit = entry.debit > 0;
  const amount = debit ? entry.debit : entry.credit;
  return (
    <article className={`cc-payments-ledger-tile ${debit ? "is-debit" : "is-credit"}`}>
      <span className="cc-payments-ledger-icon" aria-hidden>
        {debit ? <ArrowUpRight size={16} strokeWidth={2.2} /> : <ArrowDownLeft size={16} strokeWidth={2.2} />}
      </span>
      <div>
        <strong>{entry.note}</strong>
        <small>{date(entry.postedAt)}{references ? ` · ${references}` : ""}</small>
      </div>
      <em>
        {debit ? "-" : "+"}
        {money(amount)}
      </em>
    </article>
  );
}

function PaymentProofForm({
  workspace,
  session,
  onAction,
  onClose
}: {
  workspace: Workspace;
  session: Session;
  onAction: (message: string) => Promise<void>;
  onClose: () => void;
}) {
  const accountDue = accountPosition(workspace).due;
  const [form, setForm] = useState({ amount: "", proofName: "", note: "" });
  const [proofFile, setProofFile] = useState<File | null>(null);
  const [fileKey, setFileKey] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const filePicker = useRef<HTMLInputElement>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const amount = Number(form.amount);
    if (!Number.isFinite(amount) || amount < 1) {
      setError("Enter a valid amount.");
      return;
    }
    if (!proofFile) {
      setError("Choose a transfer screenshot.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const uploaded = await apiUploadFile(
        proofFile,
        { customerId: session.user.customerId || "", ownerType: "payment_proof", ownerId: session.user.customerId || "" },
        session.token
      );
      await apiPost<Payment>(
        "/api/workflow/payments",
        {
          customerId: session.user.customerId,
          type: "Account payment",
          amount,
          proofName: uploaded.originalName || form.proofName,
          proofFileId: uploaded.id,
          note: form.note
        },
        session.token
      );
      setForm({ amount: "", proofName: "", note: "" });
      setProofFile(null);
      setFileKey((current) => current + 1);
      await onAction("Payment proof uploaded. After confirmation it will clear open balances automatically.");
    } catch (submitError) {
      setError(messageFromError(submitError, "Could not upload payment proof."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="cc-payment-composer" onSubmit={submit}>
      <header className="cc-payment-composer-head">
        <div>
          <span className="cc-kicker">Payments</span>
          <h2 id="cc-payment-composer-title">Upload payment proof</h2>
        </div>
        <button aria-label="Close" className="cc-payment-composer-close" disabled={busy} onClick={onClose} type="button">
          <X size={18} />
        </button>
      </header>

      {accountDue > 0 && (
        <div className="cc-payment-composer-due">
          <span>BALANCE DUE</span>
          <strong>{money(accountDue)}</strong>
        </div>
      )}

      <div className="cc-payment-composer-fields">
        <label className="cc-payment-composer-field">
          <span>Amount paid</span>
          <div className="cc-payment-amount-input">
            <em>Rs</em>
            <input
              disabled={busy}
              inputMode="numeric"
              placeholder={accountDue > 0 ? String(accountDue) : "0"}
              required
              onChange={(event) => setForm((current) => ({ ...current, amount: digits(event.target.value) }))}
              value={form.amount}
            />
          </div>
        </label>

        <div className="cc-payment-composer-field">
          <span>Transfer screenshot</span>
          <input
            accept="image/png,image/jpeg,image/webp,application/pdf"
            disabled={busy}
            hidden
            key={fileKey}
            onChange={(event) => {
              const file = event.target.files?.[0] || null;
              setProofFile(file);
              setForm((current) => ({ ...current, proofName: file?.name || "" }));
              setError("");
            }}
            ref={filePicker}
            type="file"
          />
          <button
            className={`cc-payment-file-drop ${proofFile ? "has-file" : ""}`}
            disabled={busy}
            onClick={() => filePicker.current?.click()}
            type="button"
          >
            <FileImage size={22} />
            <span>
              <strong>{proofFile ? proofFile.name : "Choose screenshot or PDF"}</strong>
              <small>{proofFile ? "Tap to replace" : "PNG, JPG, WEBP or PDF"}</small>
            </span>
          </button>
        </div>

        <label className="cc-payment-composer-field">
          <span>Note (optional)</span>
          <input
            disabled={busy}
            onChange={(event) => setForm((current) => ({ ...current, note: event.target.value }))}
            placeholder="Bank reference or transfer date"
            value={form.note}
          />
        </label>
      </div>

      {error && <div className="cc-payment-composer-error">{error}</div>}

      <div className="cc-payment-composer-actions">
        <button className="cc-button" disabled={busy} onClick={onClose} type="button">
          Cancel
        </button>
        <button className="cc-button cc-primary" disabled={busy} type="submit">
          {busy ? (
            <>
              <LoaderCircle className="cc-spin" size={16} /> Uploading…
            </>
          ) : (
            <>
              <Upload size={16} /> Submit proof
            </>
          )}
        </button>
      </div>
    </form>
  );
}
