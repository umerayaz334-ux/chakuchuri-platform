import { type FormEvent, useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  Bell,
  Check,
  ImagePlus,
  Info,
  Megaphone,
  Package,
  Plus,
  Sparkles,
  Trash2,
  Upload,
  Users,
  X,
  type LucideIcon
} from "lucide-react";
import { apiPost, apiUploadFile } from "./api";
import { ProductFileThumb } from "./shared";
import type { Customer, CustomerNotice, FeaturedProduct, Session, Workspace } from "./types";
import { messageFromError } from "./utils";

const toneOptions: Array<{ id: CustomerNotice["tone"]; label: string; icon: LucideIcon }> = [
  { id: "info", label: "Info", icon: Info },
  { id: "success", label: "Success", icon: Check },
  { id: "warning", label: "Warning", icon: AlertCircle }
];

const tagPresets = ["Best seller", "Trending", "Upcoming"] as const;

export function AppHomeDesk({
  customers,
  session,
  workspace,
  onAction
}: {
  customers: Customer[];
  session: Session;
  workspace: Workspace;
  onAction: (message: string) => Promise<void>;
}) {
  const [notices, setNotices] = useState<CustomerNotice[]>(() => workspace.notices || []);
  const [featured, setFeatured] = useState<FeaturedProduct[]>(() => workspace.featuredProducts || []);
  const [error, setError] = useState("");

  useEffect(() => {
    setNotices(workspace.notices || []);
  }, [workspace.notices]);

  useEffect(() => {
    setFeatured(workspace.featuredProducts || []);
  }, [workspace.featuredProducts]);

  const activeNotices = notices.filter((item) => item.active).length;
  const activeFeatured = featured.filter((item) => item.active).length;

  return (
    <section className="cc-app-home">
      <div className="cc-app-home-hero">
        <div className="cc-app-home-hero-mark">
          <Megaphone size={20} />
        </div>
        <div className="cc-app-home-hero-copy">
          <span>Customer app</span>
          <strong>Home screen content</strong>
          <p>Publish notices and featured products that appear on the customer app home.</p>
        </div>
        <div className="cc-app-home-hero-stats">
          <div>
            <small>Notices live</small>
            <b>{activeNotices}</b>
          </div>
          <div>
            <small>Featured live</small>
            <b>{activeFeatured}</b>
          </div>
        </div>
      </div>

      {error && (
        <div className="cc-app-home-error" role="alert">
          <AlertCircle size={16} />
          <span>{error}</span>
          <button aria-label="Dismiss error" onClick={() => setError("")} type="button">
            <X size={15} />
          </button>
        </div>
      )}

      <div className="cc-app-home-grid">
        <NoticesPanel
          customers={customers}
          notices={notices}
          onError={setError}
          onNotices={setNotices}
          onAction={onAction}
          session={session}
        />
        <FeaturedPanel
          featured={featured}
          onAction={onAction}
          onError={setError}
          onFeatured={setFeatured}
          session={session}
        />
      </div>
    </section>
  );
}

function NoticesPanel({
  notices,
  customers,
  session,
  onNotices,
  onAction,
  onError
}: {
  notices: CustomerNotice[];
  customers: Customer[];
  session: Session;
  onNotices: (rows: CustomerNotice[]) => void;
  onAction: (message: string) => Promise<void>;
  onError: (message: string) => void;
}) {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [tone, setTone] = useState<CustomerNotice["tone"]>("info");
  const [audience, setAudience] = useState<"all" | "selected">("all");
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [customerQuery, setCustomerQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const [workingId, setWorkingId] = useState("");

  const filteredCustomers = useMemo(() => {
    const query = customerQuery.trim().toLowerCase();
    if (!query) return customers;
    return customers.filter((customer) =>
      `${customer.companyName} ${customer.contactName} ${customer.email} ${customer.id}`.toLowerCase().includes(query)
    );
  }, [customerQuery, customers]);

  const toggleCustomer = (id: string) => {
    setSelectedIds((current) => (current.includes(id) ? current.filter((item) => item !== id) : [...current, id]));
  };

  const create = async (event: FormEvent) => {
    event.preventDefault();
    if (!title.trim() || !body.trim()) {
      onError("Notice title and body are required.");
      return;
    }
    if (audience === "selected" && selectedIds.length === 0) {
      onError("Select at least one customer for a targeted notice.");
      return;
    }
    setBusy(true);
    onError("");
    try {
      const notice = await apiPost<CustomerNotice>(
        "/api/workflow/notices",
        {
          title: title.trim(),
          body: body.trim(),
          tone,
          audience,
          customerIds: audience === "selected" ? selectedIds : [],
          active: true
        },
        session.token
      );
      onNotices([notice, ...notices.filter((row) => row.id !== notice.id)]);
      setTitle("");
      setBody("");
      setTone("info");
      setAudience("all");
      setSelectedIds([]);
      setCustomerQuery("");
      await onAction("Customer notice published.");
    } catch (caught) {
      onError(messageFromError(caught, "Notice could not be created."));
    } finally {
      setBusy(false);
    }
  };

  const setActive = async (notice: CustomerNotice, active: boolean) => {
    setWorkingId(notice.id);
    onError("");
    try {
      const updated = await apiPost<CustomerNotice>(
        `/api/workflow/notices/${encodeURIComponent(notice.id)}/update`,
        { active },
        session.token
      );
      onNotices(notices.map((row) => (row.id === updated.id ? updated : row)));
      await onAction(active ? "Notice activated." : "Notice deactivated.");
    } catch (caught) {
      onError(messageFromError(caught, "Notice could not be updated."));
    } finally {
      setWorkingId("");
    }
  };

  const remove = async (notice: CustomerNotice) => {
    if (!window.confirm(`Delete notice “${notice.title}”?`)) return;
    setWorkingId(notice.id);
    onError("");
    try {
      await apiPost<CustomerNotice>(`/api/workflow/notices/${encodeURIComponent(notice.id)}/delete`, {}, session.token);
      onNotices(notices.filter((row) => row.id !== notice.id));
      await onAction("Notice deleted.");
    } catch (caught) {
      onError(messageFromError(caught, "Notice could not be deleted."));
    } finally {
      setWorkingId("");
    }
  };

  const customerLabel = (id: string) => customers.find((row) => row.id === id)?.companyName || id;

  return (
    <section className="cc-app-home-panel">
      <header>
        <div>
          <span>Broadcast</span>
          <h2>Customer notices</h2>
        </div>
        <em>
          <Bell size={14} /> {notices.length}
        </em>
      </header>

      <form className="cc-app-home-form" onSubmit={create}>
        <label>
          <span>Title</span>
          <input maxLength={120} onChange={(event) => setTitle(event.target.value)} placeholder="Factory update title" required value={title} />
        </label>
        <label className="cc-span">
          <span>Body</span>
          <textarea maxLength={2000} onChange={(event) => setBody(event.target.value)} placeholder="Short message shown on the customer home screen" required rows={3} value={body} />
        </label>
        <div className="cc-app-home-field">
          <span>Tone</span>
          <div className="cc-app-home-chips">
            {toneOptions.map((option) => {
              const Icon = option.icon;
              return (
                <button className={tone === option.id ? "active" : ""} key={option.id} onClick={() => setTone(option.id)} type="button">
                  <Icon size={14} /> {option.label}
                </button>
              );
            })}
          </div>
        </div>
        <div className="cc-app-home-field">
          <span>Audience</span>
          <div className="cc-app-home-chips">
            <button className={audience === "all" ? "active" : ""} onClick={() => setAudience("all")} type="button">
              <Users size={14} /> All customers
            </button>
            <button className={audience === "selected" ? "active" : ""} onClick={() => setAudience("selected")} type="button">
              <Sparkles size={14} /> Selected
            </button>
          </div>
        </div>
        {audience === "selected" && (
          <div className="cc-app-home-customers">
            <label>
              <span>Find customers</span>
              <input onChange={(event) => setCustomerQuery(event.target.value)} placeholder="Search company, contact or email" value={customerQuery} />
            </label>
            <div className="cc-app-home-customer-list" role="listbox" aria-label="Select customers">
              {filteredCustomers.length === 0 ? (
                <p>No matching customers.</p>
              ) : (
                filteredCustomers.map((customer) => {
                  const checked = selectedIds.includes(customer.id);
                  return (
                    <label className={checked ? "is-selected" : ""} key={customer.id}>
                      <input checked={checked} onChange={() => toggleCustomer(customer.id)} type="checkbox" />
                      <span>
                        <strong>{customer.companyName}</strong>
                        <small>{customer.contactName || customer.email}</small>
                      </span>
                    </label>
                  );
                })
              )}
            </div>
            {selectedIds.length > 0 && <small>{selectedIds.length} selected</small>}
          </div>
        )}
        <button className="cc-app-home-submit" disabled={busy} type="submit">
          <Plus size={16} /> {busy ? "Publishing…" : "Publish notice"}
        </button>
      </form>

      <div className="cc-app-home-list">
        {notices.length === 0 ? (
          <div className="cc-app-home-empty">
            <Bell size={18} />
            <div>
              <strong>No notices yet</strong>
              <span>Publish a factory update for the customer home screen.</span>
            </div>
          </div>
        ) : (
          notices.map((notice) => (
            <article className={`cc-app-home-notice is-${notice.tone} ${notice.active ? "is-active" : ""}`} key={notice.id}>
              <div className="cc-app-home-notice-copy">
                <div className="cc-app-home-notice-meta">
                  <span className={`cc-app-home-tone is-${notice.tone}`}>{notice.tone}</span>
                  <span className={notice.active ? "is-live" : "is-off"}>{notice.active ? "Active" : "Inactive"}</span>
                </div>
                <strong>{notice.title}</strong>
                <p>{notice.body}</p>
                <small>
                  {notice.audience === "selected"
                    ? `Selected · ${(notice.customerIds || []).slice(0, 3).map(customerLabel).join(", ")}${(notice.customerIds || []).length > 3 ? ` +${(notice.customerIds || []).length - 3}` : ""}`
                    : "All customers"}
                  {notice.createdBy ? ` · ${notice.createdBy}` : ""}
                </small>
              </div>
              <div className="cc-app-home-row-actions">
                <label className="cc-app-home-toggle" title={notice.active ? "Deactivate" : "Activate"}>
                  <input
                    checked={notice.active}
                    disabled={workingId === notice.id}
                    onChange={(event) => void setActive(notice, event.target.checked)}
                    type="checkbox"
                  />
                  <i aria-hidden="true" />
                </label>
                <button disabled={workingId === notice.id} onClick={() => void remove(notice)} title="Delete notice" type="button">
                  <Trash2 size={15} />
                </button>
              </div>
            </article>
          ))
        )}
      </div>
    </section>
  );
}

function FeaturedPanel({
  featured,
  session,
  onFeatured,
  onAction,
  onError
}: {
  featured: FeaturedProduct[];
  session: Session;
  onFeatured: (rows: FeaturedProduct[]) => void;
  onAction: (message: string) => Promise<void>;
  onError: (message: string) => void;
}) {
  const [name, setName] = useState("");
  const [tagMode, setTagMode] = useState<(typeof tagPresets)[number] | "custom">("Best seller");
  const [customTag, setCustomTag] = useState("");
  const [caption, setCaption] = useState("");
  const [imageFile, setImageFile] = useState<File | null>(null);
  const [fileKey, setFileKey] = useState(0);
  const [busy, setBusy] = useState(false);
  const [workingId, setWorkingId] = useState("");

  const resolvedTag = tagMode === "custom" ? customTag.trim() : tagMode;

  const create = async (event: FormEvent) => {
    event.preventDefault();
    if (!name.trim()) {
      onError("Featured product name is required.");
      return;
    }
    if (!resolvedTag) {
      onError("Add a tag for the featured product.");
      return;
    }
    if (!imageFile) {
      onError("Choose an image for the featured product.");
      return;
    }
    setBusy(true);
    onError("");
    try {
      const uploaded = await apiUploadFile(
        imageFile,
        {
          customerId: session.user.customerId || "",
          ownerType: "product_image"
        },
        session.token
      );
      const item = await apiPost<FeaturedProduct>(
        "/api/workflow/featured",
        {
          name: name.trim(),
          tag: resolvedTag,
          caption: caption.trim(),
          imageFileId: uploaded.id,
          imageName: uploaded.originalName || imageFile.name,
          active: true
        },
        session.token
      );
      onFeatured([item, ...featured.filter((row) => row.id !== item.id)]);
      setName("");
      setTagMode("Best seller");
      setCustomTag("");
      setCaption("");
      setImageFile(null);
      setFileKey((current) => current + 1);
      await onAction("Featured product added.");
    } catch (caught) {
      onError(messageFromError(caught, "Featured product could not be created."));
    } finally {
      setBusy(false);
    }
  };

  const setActive = async (item: FeaturedProduct, active: boolean) => {
    setWorkingId(item.id);
    onError("");
    try {
      const updated = await apiPost<FeaturedProduct>(
        `/api/workflow/featured/${encodeURIComponent(item.id)}/update`,
        { active },
        session.token
      );
      onFeatured(featured.map((row) => (row.id === updated.id ? updated : row)));
      await onAction(active ? "Featured product activated." : "Featured product deactivated.");
    } catch (caught) {
      onError(messageFromError(caught, "Featured product could not be updated."));
    } finally {
      setWorkingId("");
    }
  };

  const remove = async (item: FeaturedProduct) => {
    if (!window.confirm(`Delete featured product “${item.name}”?`)) return;
    setWorkingId(item.id);
    onError("");
    try {
      await apiPost<FeaturedProduct>(`/api/workflow/featured/${encodeURIComponent(item.id)}/delete`, {}, session.token);
      onFeatured(featured.filter((row) => row.id !== item.id));
      await onAction("Featured product deleted.");
    } catch (caught) {
      onError(messageFromError(caught, "Featured product could not be deleted."));
    } finally {
      setWorkingId("");
    }
  };

  return (
    <section className="cc-app-home-panel">
      <header>
        <div>
          <span>Showcase</span>
          <h2>Featured products</h2>
        </div>
        <em>
          <Package size={14} /> {featured.length}
        </em>
      </header>

      <form className="cc-app-home-form" onSubmit={create}>
        <label>
          <span>Name</span>
          <input maxLength={120} onChange={(event) => setName(event.target.value)} placeholder="Product display name" required value={name} />
        </label>
        <div className="cc-app-home-field">
          <span>Tag</span>
          <div className="cc-app-home-chips">
            {tagPresets.map((preset) => (
              <button className={tagMode === preset ? "active" : ""} key={preset} onClick={() => setTagMode(preset)} type="button">
                {preset}
              </button>
            ))}
            <button className={tagMode === "custom" ? "active" : ""} onClick={() => setTagMode("custom")} type="button">
              Custom
            </button>
          </div>
        </div>
        {tagMode === "custom" && (
          <label>
            <span>Custom tag</span>
            <input maxLength={32} onChange={(event) => setCustomTag(event.target.value)} placeholder="e.g. Limited run" required value={customTag} />
          </label>
        )}
        <label className="cc-span">
          <span>Caption</span>
          <textarea maxLength={400} onChange={(event) => setCaption(event.target.value)} placeholder="Short line under the product" rows={2} value={caption} />
        </label>
        <label className="cc-app-home-upload">
          <span>Image</span>
          <div>
            <Upload size={16} />
            <strong>{imageFile ? imageFile.name : "Upload product image"}</strong>
            <small>JPEG, PNG or WebP</small>
            <input
              accept="image/*"
              key={fileKey}
              onChange={(event) => setImageFile(event.target.files?.[0] || null)}
              type="file"
            />
          </div>
        </label>
        <button className="cc-app-home-submit" disabled={busy} type="submit">
          <ImagePlus size={16} /> {busy ? "Adding…" : "Add featured product"}
        </button>
      </form>

      <div className="cc-app-home-list">
        {featured.length === 0 ? (
          <div className="cc-app-home-empty">
            <Package size={18} />
            <div>
              <strong>No featured products</strong>
              <span>Highlight bestsellers and upcoming pieces on the app home.</span>
            </div>
          </div>
        ) : (
          featured.map((item) => (
            <article className={`cc-app-home-feature ${item.active ? "is-active" : ""}`} key={item.id}>
              <ProductFileThumb className="cc-app-home-thumb" fileId={item.imageFileId} name={item.name} token={session.token} />
              <div className="cc-app-home-feature-copy">
                <div className="cc-app-home-notice-meta">
                  <span className="cc-app-home-tag">{item.tag}</span>
                  <span className={item.active ? "is-live" : "is-off"}>{item.active ? "Active" : "Inactive"}</span>
                </div>
                <strong>{item.name}</strong>
                {item.caption ? <p>{item.caption}</p> : null}
                <small>{item.imageName || "Product image"}</small>
              </div>
              <div className="cc-app-home-row-actions">
                <label className="cc-app-home-toggle" title={item.active ? "Deactivate" : "Activate"}>
                  <input
                    checked={item.active}
                    disabled={workingId === item.id}
                    onChange={(event) => void setActive(item, event.target.checked)}
                    type="checkbox"
                  />
                  <i aria-hidden="true" />
                </label>
                <button disabled={workingId === item.id} onClick={() => void remove(item)} title="Delete featured product" type="button">
                  <Trash2 size={15} />
                </button>
              </div>
            </article>
          ))
        )}
      </div>
    </section>
  );
}
