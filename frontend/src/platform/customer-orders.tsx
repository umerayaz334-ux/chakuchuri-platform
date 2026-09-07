import { useEffect, useMemo, useState } from "react";
import {
  Calendar,
  CheckCircle2,
  ChevronRight,
  Circle,
  CircleDot,
  PackageCheck,
  XCircle
} from "lucide-react";
import { ProductFileThumb } from "./shared";
import type { Order, Quotation, Session, Stage, Update } from "./types";
import { date, displayStageLabel, jobCode, money, orderDisplayName, orderImageIds } from "./utils";

type OrderFilter = "All" | "Active" | "Ready" | "Completed" | "Cancelled";

const filters: OrderFilter[] = ["All", "Active", "Ready", "Completed", "Cancelled"];

export function CustomerOrdersPage({
  orders,
  quotations,
  session
}: {
  orders: Order[];
  quotations: Quotation[];
  session: Session;
}) {
  const [filter, setFilter] = useState<OrderFilter>("All");
  const [selectedId, setSelectedId] = useState("");

  const visible = useMemo(() => {
    const sorted = [...orders].sort((left, right) => {
      const recent = latestActivity(right).localeCompare(latestActivity(left));
      return recent !== 0 ? recent : right.id.localeCompare(left.id);
    });
    return sorted.filter((order) => matchesFilter(order, filter));
  }, [filter, orders]);

  const selected = orders.find((order) => order.id === selectedId) || null;

  useEffect(() => {
    if (!selectedId) return;
    if (!orders.some((order) => order.id === selectedId)) setSelectedId("");
  }, [orders, selectedId]);

  useEffect(() => {
    if (!selectedId) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setSelectedId("");
    };
    document.addEventListener("keydown", onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = previous;
    };
  }, [selectedId]);

  return (
    <section className="cc-co">
      <div className="cc-co-filters" role="group" aria-label="Filter orders">
        {filters.map((item) => (
          <button
            aria-pressed={filter === item}
            className={`cc-co-chip${filter === item ? " is-active" : ""}`}
            key={item}
            onClick={() => setFilter(item)}
            type="button"
          >
            {item}
          </button>
        ))}
      </div>

      {orders.length === 0 ? (
        <div className="cc-co-empty">
          <PackageCheck size={24} strokeWidth={1.75} />
          <strong>No manufacturing orders yet</strong>
        </div>
      ) : visible.length === 0 ? (
        <div className="cc-co-empty">
          <PackageCheck size={24} strokeWidth={1.75} />
          <strong>No {filter} orders</strong>
          <p>Try another filter.</p>
        </div>
      ) : (
        <div className="cc-co-list">
          {visible.map((order) => (
            <ModernOrderCard
              key={order.id}
              onOpen={() => setSelectedId(order.id)}
              order={order}
              quotations={quotations}
              token={session.token}
            />
          ))}
        </div>
      )}

      {selected ? (
        <OrderDetailSheet
          onClose={() => setSelectedId("")}
          order={selected}
          quotations={quotations}
          token={session.token}
        />
      ) : null}
    </section>
  );
}

function ModernOrderCard({
  order,
  quotations,
  token,
  onOpen
}: {
  order: Order;
  quotations: Quotation[];
  token: string;
  onOpen: () => void;
}) {
  const photos = orderImageIds(order, quotations);
  const stage = displayStageLabel(order.currentStage || order.status || "Pending");
  const title = orderDisplayName(order, quotations);
  const progress = clampProgress(order.progress);

  return (
    <button className="cc-co-card" onClick={onOpen} type="button">
      <div className="cc-co-card-head">
        {photos[0] ? (
          <ProductFileThumb className="cc-co-thumb" fileId={photos[0]} name={title} token={token} />
        ) : (
          <span className="cc-co-thumb is-empty" aria-hidden>
            <PackageCheck size={22} strokeWidth={1.75} />
          </span>
        )}
        <span className="cc-co-card-copy">
          <em className="cc-co-code">{jobCode(order)}</em>
          <strong>{title}</strong>
          <small>{order.quantity} pieces</small>
        </span>
        <ChevronRight aria-hidden size={18} strokeWidth={2.2} />
      </div>

      <div className="cc-co-stage-row">
        <StatusPill value={stage} />
        <span className="cc-co-progress">
          <i style={{ width: `${progress}%` }} />
        </span>
        <b>{progress}%</b>
      </div>

      <div className="cc-co-card-foot">
        <span>
          <Calendar aria-hidden size={14} strokeWidth={2} />
          Expected {date(order.expectedDate)}
        </span>
        <em className={order.balanceDue > 0 ? "is-due" : "is-clear"}>
          {order.balanceDue > 0 ? `${money(order.balanceDue)} due` : "Account clear"}
        </em>
      </div>
    </button>
  );
}

function OrderDetailSheet({
  order,
  quotations,
  token,
  onClose
}: {
  order: Order;
  quotations: Quotation[];
  token: string;
  onClose: () => void;
}) {
  const photos = orderImageIds(order, quotations);
  const title = orderDisplayName(order, quotations);
  const progress = clampProgress(order.progress);
  const stageLabel = displayStageLabel(order.currentStage || "Production update pending");
  const latest = latestHistory(order.history);

  return (
    <div className="cc-co-sheet-root">
      <button aria-label="Close order details" className="cc-co-sheet-backdrop" onClick={onClose} type="button" />
      <aside aria-labelledby="cc-co-sheet-title" aria-modal="true" className="cc-co-sheet" role="dialog">
        <div className="cc-co-sheet-handle" aria-hidden />
        <header className="cc-co-sheet-head">
          {photos[0] ? (
            <ProductFileThumb className="cc-co-thumb is-lg" fileId={photos[0]} name={title} token={token} />
          ) : null}
          <div>
            <em className="cc-co-code">{jobCode(order)}</em>
            <h2 id="cc-co-sheet-title">{title}</h2>
            <StatusPill value={order.status || "Pending"} />
          </div>
        </header>

        {photos.length > 1 ? (
          <div className="cc-co-photo-row">
            {photos.slice(0, 8).map((fileId, index) => (
              <ProductFileThumb
                className="cc-co-thumb is-sm"
                fileId={fileId}
                key={fileId}
                name={order.imageNames?.[index] || title}
                token={token}
              />
            ))}
          </div>
        ) : null}

        <div className="cc-co-facts">
          <div>
            <small>QUANTITY</small>
            <strong>{order.quantity} pcs</strong>
          </div>
          <div>
            <small>EXPECTED</small>
            <strong>{date(order.expectedDate)}</strong>
          </div>
          <div>
            <small>BALANCE</small>
            <strong>{money(order.balanceDue)}</strong>
          </div>
        </div>

        <div className="cc-co-sheet-progress">
          <div>
            <strong>{stageLabel}</strong>
            <b>{progress}%</b>
          </div>
          <span className="cc-co-progress is-wide">
            <i style={{ width: `${progress}%` }} />
          </span>
        </div>

        <h3>Production timeline</h3>
        {order.stages.length === 0 ? (
          <div className="cc-co-empty is-soft">
            <strong>Timeline pending</strong>
            <p>Production stages will appear here.</p>
          </div>
        ) : (
          <ul className="cc-co-timeline">
            {order.stages.map((stage) => (
              <StageLine key={`${stage.name}-${stage.status}-${stage.date || ""}`} stage={stage} />
            ))}
          </ul>
        )}

        {latest ? (
          <>
            <h3>Latest update</h3>
            <article className="cc-co-latest">
              <strong>{latest.label}</strong>
              {latest.detail.trim() ? <p>{latest.detail}</p> : null}
              <small>{date(latest.createdAt)}</small>
            </article>
          </>
        ) : null}
      </aside>
    </div>
  );
}

function StageLine({ stage }: { stage: Stage }) {
  const tone = stageTone(stage.status);
  const Icon =
    tone === "done" ? CheckCircle2 : tone === "active" ? CircleDot : tone === "cancelled" ? XCircle : Circle;
  return (
    <li className={`cc-co-stage-line is-${tone}`}>
      <Icon aria-hidden size={18} strokeWidth={2.1} />
      <span>{displayStageLabel(stage.name)}</span>
      <em>{stage.date ? date(stage.date) : stage.status}</em>
    </li>
  );
}

function StatusPill({ value }: { value: string }) {
  return <span className={`cc-co-pill is-${pillTone(value)}`}>{value || "Pending"}</span>;
}

function latestActivity(order: Order) {
  let latest = "";
  for (const update of order.history || []) {
    if ((update.createdAt || "").localeCompare(latest) > 0) latest = update.createdAt;
  }
  return latest || order.id;
}

function latestHistory(history: Update[]) {
  if (!history?.length) return null;
  return history[history.length - 1];
}

function matchesFilter(order: Order, filter: OrderFilter) {
  const status = order.status.toLowerCase();
  if (filter === "Active") return !["completed", "cancelled"].includes(status);
  if (filter === "Ready") return status.includes("ready");
  if (filter === "Completed") return status === "completed";
  if (filter === "Cancelled") return status === "cancelled";
  return true;
}

function clampProgress(value: number) {
  return Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0));
}

function pillTone(value: string) {
  const normalized = value.toLowerCase();
  if (/(reject|cancel|overdue)/.test(normalized)) return "danger";
  if (/(waiting|pending|requested|priced)/.test(normalized)) return "warn";
  if (/(confirm|ready|complete|deliver|paid)/.test(normalized)) return "ok";
  return "info";
}

function stageTone(status: string) {
  const value = status.toLowerCase();
  if (value === "done" || value === "completed") return "done";
  if (value === "active" || value === "in progress") return "active";
  if (value === "cancelled" || value === "canceled") return "cancelled";
  return "pending";
}
