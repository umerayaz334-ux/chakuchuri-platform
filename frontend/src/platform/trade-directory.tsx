import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Building2, Check, MapPin, MessageCircle, Phone, Plus, Search, Store, X } from "lucide-react";
import { apiBlob, apiGet, apiPost, apiUploadFile } from "./api";
import type { PageInfo, Session } from "./types";
import { messageFromError } from "./utils";

export type DirectoryCategory = {
  id: string;
  name: string;
  slug: string;
  active: boolean;
  sortOrder: number;
  createdAt: string;
};

export type DirectoryListing = {
  id: string;
  name: string;
  categoryIds: string[];
  phone: string;
  whatsapp: string;
  address: string;
  city: string;
  note: string;
  photoFileId?: string;
  status: string;
  submittedByUserId?: string;
  customerId?: string;
  reviewedByUserId?: string;
  reviewedAt?: string;
  reviewNote?: string;
  createdAt: string;
  updatedAt: string;
};

type ListingForm = {
  name: string;
  categoryIds: string[];
  phone: string;
  whatsapp: string;
  address: string;
  city: string;
  note: string;
  photoFileId: string;
};

const emptyForm = (): ListingForm => ({
  name: "",
  categoryIds: [],
  phone: "",
  whatsapp: "",
  address: "",
  city: "Wazirabad",
  note: "",
  photoFileId: ""
});

function categoryNames(listing: DirectoryListing, categories: DirectoryCategory[]) {
  const map = new Map(categories.map((row) => [row.id, row.name]));
  return listing.categoryIds.map((id) => map.get(id) || id).filter(Boolean);
}

function digitsOnly(value: string) {
  return value.replace(/\D/g, "");
}

function whatsappHref(phoneOrWhatsapp: string) {
  const digits = digitsOnly(phoneOrWhatsapp);
  if (!digits) return "";
  const international = digits.startsWith("0") ? `92${digits.slice(1)}` : digits;
  return `https://wa.me/${international}`;
}

function mapsHref(address: string, city: string) {
  const query = [address, city || "Wazirabad", "Pakistan"].filter(Boolean).join(", ");
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(query)}`;
}

function listingInitials(name: string) {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (!parts.length) return "W";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0] || ""}${parts[1][0] || ""}`.toUpperCase();
}

function DirectoryListingPhoto({
  fileId,
  name,
  token
}: {
  fileId?: string;
  name: string;
  token?: string;
}) {
  const [src, setSrc] = useState("");
  useEffect(() => {
    if (!fileId || !token) {
      setSrc("");
      return;
    }
    let active = true;
    let objectUrl = "";
    apiBlob(`/api/files/${fileId}/thumbnail`, token)
      .catch(() => apiBlob(`/api/files/${fileId}/content`, token))
      .then((blob) => {
        if (!active) return;
        objectUrl = URL.createObjectURL(blob);
        setSrc(objectUrl);
      })
      .catch(() => {
        if (active) setSrc("");
      });
    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [fileId, token]);

  return (
    <div className="cc-trade-photo" aria-hidden={!src}>
      {src ? <img alt={name} src={src} /> : <span>{listingInitials(name)}</span>}
    </div>
  );
}

export function DirectoryBrowseList({
  listings,
  categories,
  token,
  emptyTitle = "No verified listings yet",
  emptyDetail = "Approved shops and makers will appear here."
}: {
  listings: DirectoryListing[];
  categories: DirectoryCategory[];
  token?: string;
  emptyTitle?: string;
  emptyDetail?: string;
}) {
  if (!listings.length) {
    return (
      <div className="cc-trade-empty">
        <Store size={22} />
        <strong>{emptyTitle}</strong>
        <p>{emptyDetail}</p>
      </div>
    );
  }
  return (
    <div className="cc-trade-grid">
      {listings.map((listing) => {
        const callHref = listing.phone ? `tel:${listing.phone}` : "";
        const messageHref = whatsappHref(listing.whatsapp || listing.phone);
        const locationHref = listing.address || listing.city ? mapsHref(listing.address, listing.city) : "";
        return (
          <article className="cc-trade-card" key={listing.id}>
            <DirectoryListingPhoto fileId={listing.photoFileId} name={listing.name} token={token} />
            <div className="cc-trade-card-body">
              <header>
                <strong>{listing.name}</strong>
                <span>{listing.city || "Wazirabad"}</span>
              </header>
              <div className="cc-trade-tags">
                {categoryNames(listing, categories).map((name) => (
                  <b key={name}>{name}</b>
                ))}
              </div>
              {listing.note && <p>{listing.note}</p>}
              {(listing.address || listing.city) && (
                <small className="cc-trade-location">
                  <MapPin size={13} /> {listing.address ? `${listing.address}, ` : ""}
                  {listing.city || "Wazirabad"}
                </small>
              )}
              <div className="cc-trade-actions">
                {callHref && (
                  <a className="call" href={callHref}>
                    <Phone size={14} /> Call
                  </a>
                )}
                {messageHref && (
                  <a className="message" href={messageHref} rel="noreferrer" target="_blank">
                    <MessageCircle size={14} /> Message
                  </a>
                )}
                {locationHref && (
                  <a className="maps" href={locationHref} rel="noreferrer" target="_blank">
                    <MapPin size={14} /> Location
                  </a>
                )}
              </div>
            </div>
          </article>
        );
      })}
    </div>
  );
}

export function DirectoryBrowsePanel({
  token,
  publicMode = false
}: {
  token?: string;
  publicMode?: boolean;
}) {
  const [categories, setCategories] = useState<DirectoryCategory[]>([]);
  const [listings, setListings] = useState<DirectoryListing[]>([]);
  const [query, setQuery] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [pagination, setPagination] = useState<PageInfo>({ loaded: 0, total: 0, hasMore: false });

  const load = async (nextQuery = query, nextCategory = categoryId, offset = 0) => {
    setBusy(true);
    setError("");
    try {
      const path = publicMode
        ? `/api/public/directory/listings?q=${encodeURIComponent(nextQuery)}&category=${encodeURIComponent(nextCategory)}&offset=${offset}&limit=20`
        : `/api/directory/listings?q=${encodeURIComponent(nextQuery)}&category=${encodeURIComponent(nextCategory)}&offset=${offset}&limit=20`;
      const payload = await apiGet<{ listings: DirectoryListing[]; categories: DirectoryCategory[]; pagination: PageInfo }>(path, token);
      setListings((current) => offset ? mergeListings(current, payload.listings || []) : payload.listings || []);
      setPagination(payload.pagination || { loaded: payload.listings?.length || 0, total: payload.listings?.length || 0, hasMore: false });
      setCategories((payload.categories || []).filter((row) => row.active !== false));
    } catch (caught) {
      setError(messageFromError(caught, "Directory could not be loaded."));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    void load("", "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [publicMode, token]);

  return (
    <section className="cc-trade-browse">
      <div className="cc-trade-toolbar">
        <label className="cc-trade-search">
          <Search size={16} />
          <input
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") void load(query, categoryId);
            }}
            placeholder="Search shops, makers, phone…"
            value={query}
          />
        </label>
        <button className="cc-button" disabled={busy} onClick={() => void load(query, categoryId)} type="button">
          Search
        </button>
      </div>
      <div className="cc-trade-chips" role="list">
        <button className={!categoryId ? "active" : ""} onClick={() => { setCategoryId(""); void load(query, ""); }} type="button">
          All
        </button>
        {categories.map((category) => (
          <button
            className={categoryId === category.id ? "active" : ""}
            key={category.id}
            onClick={() => {
              setCategoryId(category.id);
              void load(query, category.id);
            }}
            type="button"
          >
            {category.name}
          </button>
        ))}
      </div>
      {error && <div className="cc-trade-error">{error}</div>}
      <DirectoryBrowseList categories={categories} listings={listings} token={token} />
      {pagination.hasMore && <div className="cc-pagination-more"><button className="cc-button" disabled={busy} onClick={() => void load(query, categoryId, pagination.loaded)} type="button">See more</button><small>{pagination.total - pagination.loaded} remaining</small></div>}
    </section>
  );
}

export function CustomerDirectoryPage({
  session,
  onAction
}: {
  session: Session;
  onAction: (message: string) => Promise<void>;
}) {
  const [tab, setTab] = useState<"browse" | "submit" | "mine">("browse");
  const [categories, setCategories] = useState<DirectoryCategory[]>([]);
  const [mine, setMine] = useState<DirectoryListing[]>([]);
  const [form, setForm] = useState<ListingForm>(emptyForm);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [minePagination, setMinePagination] = useState<PageInfo>({ loaded: 0, total: 0, hasMore: false });

  const refreshMine = async (offset = 0) => {
    const payload = await apiGet<{ listings: DirectoryListing[]; pagination: PageInfo }>(
      `/api/directory/listings/mine?offset=${offset}&limit=20`,
      session.token
    );
    setMine((current) => offset ? mergeListings(current, payload.listings || []) : payload.listings || []);
    setMinePagination(payload.pagination || {
      loaded: payload.listings?.length || 0,
      total: payload.listings?.length || 0,
      hasMore: false
    });
  };

  useEffect(() => {
    void apiGet<{ categories: DirectoryCategory[] }>("/api/directory/categories", session.token)
      .then((payload) => setCategories(payload.categories || []))
      .catch(() => undefined);
    void refreshMine().catch(() => undefined);
  }, [session.token]);

  const toggleCategory = (id: string) => {
    setForm((current) => ({
      ...current,
      categoryIds: current.categoryIds.includes(id)
        ? current.categoryIds.filter((item) => item !== id)
        : [...current.categoryIds, id]
    }));
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!form.name.trim() || !form.phone.trim() || !form.categoryIds.length) {
      setError("Name, phone and at least one category are required.");
      return;
    }
    setBusy("submit");
    setError("");
    try {
      await apiPost("/api/directory/listings", form, session.token);
      setForm(emptyForm());
      await refreshMine(0);
      setTab("mine");
      await onAction("Listing submitted for admin review.");
    } catch (caught) {
      setError(messageFromError(caught, "Listing could not be submitted."));
    } finally {
      setBusy("");
    }
  };

  const onPickPhoto = async (file?: File) => {
    if (!file) return;
    setBusy("photo");
    setError("");
    try {
      const uploaded = await apiUploadFile(
        file,
        {
          customerId: session.user.customerId || "",
          ownerType: "directory_listing",
          ownerId: session.user.id
        },
        session.token
      );
      setForm((current) => ({ ...current, photoFileId: uploaded.id }));
    } catch (caught) {
      setError(messageFromError(caught, "Photo could not be uploaded."));
    } finally {
      setBusy("");
    }
  };

  return (
    <section className="cc-trade-page">
      <header className="cc-trade-head">
        <div>
          <h2>Wazirabad directory</h2>
          <p>Browse verified shops and makers. Submit your listing for review.</p>
        </div>
      </header>
      <div className="cc-trade-tabs" role="tablist">
        <button className={tab === "browse" ? "active" : ""} onClick={() => setTab("browse")} type="button">
          Browse
        </button>
        <button className={tab === "submit" ? "active" : ""} onClick={() => setTab("submit")} type="button">
          Add listing
        </button>
        <button className={tab === "mine" ? "active" : ""} onClick={() => setTab("mine")} type="button">
          My submissions ({minePagination.total})
        </button>
      </div>

      {tab === "browse" && <DirectoryBrowsePanel token={session.token} />}

      {tab === "submit" && (
        <form className="cc-trade-form" onSubmit={submit}>
          {error && <div className="cc-trade-error">{error}</div>}
          <label>
            <span>Shop / person name</span>
            <input onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} required value={form.name} />
          </label>
          <div className="cc-trade-form-row">
            <label>
              <span>Phone</span>
              <input onChange={(event) => setForm((current) => ({ ...current, phone: event.target.value }))} required value={form.phone} />
            </label>
            <label>
              <span>WhatsApp</span>
              <input onChange={(event) => setForm((current) => ({ ...current, whatsapp: event.target.value }))} value={form.whatsapp} />
            </label>
          </div>
          <label>
            <span>Address</span>
            <input onChange={(event) => setForm((current) => ({ ...current, address: event.target.value }))} value={form.address} />
          </label>
          <label>
            <span>City</span>
            <input onChange={(event) => setForm((current) => ({ ...current, city: event.target.value }))} value={form.city} />
          </label>
          <div className="cc-trade-chip-pick">
            <span>Categories</span>
            <div>
              {categories.map((category) => (
                <button
                  className={form.categoryIds.includes(category.id) ? "active" : ""}
                  key={category.id}
                  onClick={() => toggleCategory(category.id)}
                  type="button"
                >
                  {category.name}
                </button>
              ))}
            </div>
          </div>
          <label>
            <span>Short note</span>
            <textarea onChange={(event) => setForm((current) => ({ ...current, note: event.target.value }))} rows={3} value={form.note} />
          </label>
          <label className="cc-trade-file">
            <span>Service / shop photo</span>
            <input accept="image/*" onChange={(event) => void onPickPhoto(event.target.files?.[0])} type="file" />
            {form.photoFileId ? (
              <small>Photo attached — shown on your verified listing.</small>
            ) : (
              <small>Optional. Add a clear photo of your shop, product, or workshop.</small>
            )}
          </label>
          <button className="cc-button cc-primary" disabled={Boolean(busy)} type="submit">
            <Plus size={16} /> {busy === "submit" ? "Submitting…" : "Submit for review"}
          </button>
        </form>
      )}

      {tab === "mine" && (
        <div className="cc-trade-mine">
          {!mine.length && <div className="cc-trade-empty"><strong>No submissions yet</strong><p>Add a listing to get verified in the public directory.</p></div>}
          {mine.map((listing) => (
            <article key={listing.id}>
              <div>
                <strong>{listing.name}</strong>
                <small>{categoryNames(listing, categories).join(" · ")}</small>
              </div>
              <b className={listing.status}>{listing.status}</b>
            </article>
          ))}
          {minePagination.hasMore && (
            <div className="cc-pagination-more">
              <button
                className="cc-button"
                disabled={Boolean(busy)}
                onClick={() => void refreshMine(minePagination.loaded)}
                type="button"
              >
                See more
              </button>
              <small>{minePagination.total - minePagination.loaded} remaining</small>
            </div>
          )}
        </div>
      )}
    </section>
  );
}

export function AdminDirectoryDesk({
  session,
  onAction
}: {
  session: Session;
  onAction: (message: string) => Promise<void>;
}) {
  const [categories, setCategories] = useState<DirectoryCategory[]>([]);
  const [listings, setListings] = useState<DirectoryListing[]>([]);
  const [statusFilter, setStatusFilter] = useState("pending");
  const [categoryName, setCategoryName] = useState("");
  const [createForm, setCreateForm] = useState<ListingForm>(emptyForm);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [pagination, setPagination] = useState<PageInfo>({ loaded: 0, total: 0, hasMore: false });

  const load = async (status = statusFilter, offset = 0) => {
    setBusy("load");
    setError("");
    try {
      const payload = await apiGet<{ listings: DirectoryListing[]; categories: DirectoryCategory[]; pagination: PageInfo }>(
        `/api/admin/directory/listings?status=${encodeURIComponent(status)}&offset=${offset}&limit=20`,
        session.token
      );
      setListings((current) => offset ? mergeListings(current, payload.listings || []) : payload.listings || []);
      setCategories(payload.categories || []);
      setPagination(payload.pagination || { loaded: payload.listings?.length || 0, total: payload.listings?.length || 0, hasMore: false });
    } catch (caught) {
      setError(messageFromError(caught, "Directory admin data could not be loaded."));
    } finally {
      setBusy("");
    }
  };

  useEffect(() => {
    void load("pending");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.token]);

  const pendingCount = useMemo(() => listings.filter((row) => row.status === "pending").length, [listings]);

  const review = async (id: string, status: "approved" | "rejected") => {
    setBusy(id);
    try {
      await apiPost(`/api/admin/directory/listings/${encodeURIComponent(id)}/review`, { status }, session.token);
      await onAction(`Listing ${status}.`);
      await load(statusFilter);
    } catch (caught) {
      setError(messageFromError(caught, "Review failed."));
    } finally {
      setBusy("");
    }
  };

  const addCategory = async (event: FormEvent) => {
    event.preventDefault();
    if (!categoryName.trim()) return;
    setBusy("category");
    try {
      await apiPost("/api/admin/directory/categories", { name: categoryName.trim() }, session.token);
      setCategoryName("");
      await onAction("Category added.");
      await load(statusFilter);
    } catch (caught) {
      setError(messageFromError(caught, "Category could not be added."));
    } finally {
      setBusy("");
    }
  };

  const toggleCategory = async (category: DirectoryCategory) => {
    setBusy(category.id);
    try {
      await apiPost(`/api/admin/directory/categories/${encodeURIComponent(category.id)}`, { active: !category.active }, session.token);
      await load(statusFilter);
    } catch (caught) {
      setError(messageFromError(caught, "Category could not be updated."));
    } finally {
      setBusy("");
    }
  };

  const createListing = async (event: FormEvent) => {
    event.preventDefault();
    if (!createForm.name.trim() || !createForm.phone.trim() || !createForm.categoryIds.length) {
      setError("Name, phone and at least one category are required.");
      return;
    }
    setBusy("create");
    try {
      await apiPost("/api/admin/directory/listings", { ...createForm, status: "approved" }, session.token);
      setCreateForm(emptyForm());
      await onAction("Verified listing published.");
      setStatusFilter("approved");
      await load("approved");
    } catch (caught) {
      setError(messageFromError(caught, "Listing could not be created."));
    } finally {
      setBusy("");
    }
  };

  const onAdminPhoto = async (file?: File) => {
    if (!file) return;
    setBusy("photo");
    setError("");
    try {
      const uploaded = await apiUploadFile(
        file,
        {
          customerId: "",
          ownerType: "directory_listing",
          ownerId: session.user.id
        },
        session.token
      );
      setCreateForm((current) => ({ ...current, photoFileId: uploaded.id }));
    } catch (caught) {
      setError(messageFromError(caught, "Photo could not be uploaded."));
    } finally {
      setBusy("");
    }
  };

  return (
    <section className="cc-trade-page cc-trade-admin">
      <header className="cc-trade-head">
        <div>
          <h2>Directory review</h2>
          <p>Verify Wazirabad listings and manage service categories.</p>
        </div>
      </header>
      {error && <div className="cc-trade-error">{error}</div>}

      <div className="cc-trade-admin-grid">
        <section className="cc-trade-panel">
          <header>
            <strong>Listings</strong>
            <div className="cc-trade-tabs compact">
              {(["pending", "approved", "rejected", "all"] as const).map((status) => (
                <button
                  className={statusFilter === status ? "active" : ""}
                  key={status}
                  onClick={() => {
                    setStatusFilter(status);
                    void load(status);
                  }}
                  type="button"
                >
                  {status === "pending" ? `Pending${pendingCount && statusFilter === "pending" ? ` (${pendingCount})` : ""}` : status}
                </button>
              ))}
            </div>
          </header>
          <div className="cc-trade-admin-list">
            {!listings.length && <div className="cc-trade-empty"><strong>No listings</strong><p>Nothing in this filter.</p></div>}
            {listings.map((listing) => (
              <article key={listing.id}>
                <DirectoryListingPhoto fileId={listing.photoFileId} name={listing.name} token={session.token} />
                <div>
                  <strong>{listing.name}</strong>
                  <small>{categoryNames(listing, categories).join(" · ") || "No category"}</small>
                  <small>{listing.phone}{listing.address ? ` · ${listing.address}` : ""}{listing.city ? `, ${listing.city}` : ""}</small>
                  {listing.note && <p>{listing.note}</p>}
                </div>
                <div className="cc-trade-admin-actions">
                  <b className={listing.status}>{listing.status}</b>
                  {listing.status === "pending" && (
                    <>
                      <button className="cc-button cc-primary" disabled={Boolean(busy)} onClick={() => void review(listing.id, "approved")} type="button">
                        <Check size={14} /> Approve
                      </button>
                      <button className="cc-button" disabled={Boolean(busy)} onClick={() => void review(listing.id, "rejected")} type="button">
                        <X size={14} /> Reject
                      </button>
                    </>
                  )}
                </div>
              </article>
            ))}
            {pagination.hasMore && <div className="cc-pagination-more"><button className="cc-button" disabled={Boolean(busy)} onClick={() => void load(statusFilter, pagination.loaded)} type="button">See more</button><small>{pagination.total - pagination.loaded} remaining</small></div>}
          </div>
        </section>

        <aside className="cc-trade-side">
          <section className="cc-trade-panel">
            <header><strong>Categories</strong></header>
            <form className="cc-trade-inline" onSubmit={addCategory}>
              <input onChange={(event) => setCategoryName(event.target.value)} placeholder="New category name" value={categoryName} />
              <button className="cc-button cc-primary" disabled={busy === "category" || !categoryName.trim()} type="submit">
                Add
              </button>
            </form>
            <div className="cc-trade-cat-list">
              {categories.map((category) => (
                <div key={category.id}>
                  <span>{category.name}</span>
                  <button className={category.active ? "on" : "off"} onClick={() => void toggleCategory(category)} type="button">
                    {category.active ? "Active" : "Off"}
                  </button>
                </div>
              ))}
            </div>
          </section>

          <section className="cc-trade-panel">
            <header><strong>Add verified listing</strong></header>
            <form className="cc-trade-form compact" onSubmit={createListing}>
              <input onChange={(event) => setCreateForm((current) => ({ ...current, name: event.target.value }))} placeholder="Shop / person" required value={createForm.name} />
              <input onChange={(event) => setCreateForm((current) => ({ ...current, phone: event.target.value }))} placeholder="Phone" required value={createForm.phone} />
              <input onChange={(event) => setCreateForm((current) => ({ ...current, whatsapp: event.target.value }))} placeholder="WhatsApp (optional)" value={createForm.whatsapp} />
              <input onChange={(event) => setCreateForm((current) => ({ ...current, address: event.target.value }))} placeholder="Address / area" value={createForm.address} />
              <div className="cc-trade-chip-pick">
                <span>Categories</span>
                <div>
                  {categories.filter((row) => row.active).map((category) => (
                    <button
                      className={createForm.categoryIds.includes(category.id) ? "active" : ""}
                      key={category.id}
                      onClick={() =>
                        setCreateForm((current) => ({
                          ...current,
                          categoryIds: current.categoryIds.includes(category.id)
                            ? current.categoryIds.filter((id) => id !== category.id)
                            : [...current.categoryIds, category.id]
                        }))
                      }
                      type="button"
                    >
                      {category.name}
                    </button>
                  ))}
                </div>
              </div>
              <label className="cc-trade-file">
                <span>Service photo</span>
                <input accept="image/*" onChange={(event) => void onAdminPhoto(event.target.files?.[0])} type="file" />
                {createForm.photoFileId && <small>Photo attached</small>}
              </label>
              <button className="cc-button cc-primary" disabled={busy === "create" || busy === "photo"} type="submit">
                <Building2 size={15} /> Publish
              </button>
            </form>
          </section>
        </aside>
      </div>
    </section>
  );
}

function mergeListings(current: DirectoryListing[], incoming: DirectoryListing[]) {
  const ids = new Set(current.map((listing) => listing.id));
  return [...current, ...incoming.filter((listing) => !ids.has(listing.id))];
}
