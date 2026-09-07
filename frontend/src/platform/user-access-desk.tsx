import { useEffect, useMemo, useState, type FormEvent } from "react";
import {
  Activity,
  Building2,
  Check,
  ChevronRight,
  Eye,
  EyeOff,
  Globe,
  KeyRound,
  LockKeyhole,
  Mail,
  Pencil,
  Search,
  ShieldAlert,
  ShieldCheck,
  Trash2,
  UserPlus,
  UsersRound,
  X
} from "lucide-react";
import { apiCreateUser, apiDeleteUser, apiListUsers, apiUpdateUser, apiUserOptions, type ManagedUserPayload } from "./api";
import { adminPages, customerPages } from "./constants";
import { VerificationBadge, canReviewCustomerKYC, verificationLabel } from "./customer-documents";
import { ProfileAvatar } from "./profile-avatar";
import { updateCustomerPath } from "./routes";
import type { AccessOptions, Customer, PageInfo, Session, User } from "./types";
import { date, dateTime, messageFromError } from "./utils";
import { userAccessEventName } from "./workspace-realtime";

const emptyAccessOptions: AccessOptions = {
  roles: [],
  pages: [],
  permissions: [],
  roleDefaults: {},
  pageAccessDefaults: {},
  statuses: []
};

const customerPortalPermissions = ["calls.start", "chat.use", "orders.read", "payments.create", "products.manage", "quotes.create", "shipping.create"];
const customerOnlyPermissions = ["calls.start", "chat.use", "orders.read", "payments.create", "quotes.create", "shipping.create"];

function defaultUserForm(options: AccessOptions, role = "Manager"): ManagedUserPayload {
  return {
    name: "",
    email: "",
    password: "",
    role,
    status: "Active",
    customerId: "",
    assignedCustomerIds: [],
    permissions: options.roleDefaults[role] || [],
    pageAccess: options.pageAccessDefaults[role] || [],
    suspensionNotice: ""
  };
}

function isCustomerRole(role: string) {
  return role.toLowerCase() === "customer";
}

function pageLabel(page: string) {
  if (page === "Command" || page === "Home") return "Dashboard";
  return page;
}

function permissionLabel(permission: string) {
  return permission.replace(/\./g, " / ").replace(/\b\w/g, (letter) => letter.toUpperCase());
}

export function UserAccessDesk({
  session,
  customers = [],
  onAction,
  onNavigate
}: {
  session: Session;
  customers?: Customer[];
  onAction: (message: string) => Promise<void>;
  onNavigate?: (page: string) => void;
}) {
  const [users, setUsers] = useState<User[]>([]);
  const [options, setOptions] = useState<AccessOptions>(emptyAccessOptions);
  const [focusedId, setFocusedId] = useState(session.user.id);
  const [editing, setEditing] = useState<User | null>(null);
  const [form, setForm] = useState<ManagedUserPayload>(() => defaultUserForm(emptyAccessOptions));
  const [modalOpen, setModalOpen] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState("All roles");
  const [statusFilter, setStatusFilter] = useState("All statuses");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [pagination, setPagination] = useState<PageInfo>({ loaded: 0, total: 0, hasMore: false });

  const load = async (offset = 0, append = false) => {
    setLoading(true);
    setLoadError("");
    try {
      const [nextOptions, page] = await Promise.all([apiUserOptions(session.token), apiListUsers(session.token, { offset, q: search.trim(), role: roleFilter, status: statusFilter })]);
      const nextUsers = page.users;
      setOptions(nextOptions);
      setUsers((current) => append ? mergeUsers(current, nextUsers) : nextUsers);
      setPagination(page.pagination);
      setFocusedId((current) => append || nextUsers.some((user) => user.id === current) ? current : nextUsers[0]?.id || "");
      setForm((current) => current.email ? current : defaultUserForm(nextOptions));
    } catch (error) {
      setLoadError(messageFromError(error, "Users could not be loaded."));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const refresh = () => void load(0).catch(() => undefined);
    const timer = window.setTimeout(refresh, 250);
    window.addEventListener(userAccessEventName, refresh);
    return () => { window.clearTimeout(timer); window.removeEventListener(userAccessEventName, refresh); };
  }, [session.token, search, roleFilter, statusFilter]);

  useEffect(() => {
    if (!modalOpen) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busy) closeModal();
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [busy, modalOpen]);

  const focused = users.find((user) => user.id === focusedId) || users[0];
  const linkedCustomer = focused && isCustomerRole(focused.role)
    ? customers.find((row) => row.id === focused.customerId)
    : undefined;
  const customerRole = isCustomerRole(form.role);
  const pageChoices = (customerRole ? customerPages : adminPages).filter((page) => page !== "Settings" && (!options.pages.length || options.pages.includes(page)));
  const permissionChoices = options.permissions.filter((permission) =>
    customerRole ? customerPortalPermissions.includes(permission) : !customerOnlyPermissions.includes(permission)
  );
  const roles = options.roles.length ? options.roles : ["Owner", "Admin", "Manager", "Customer"];
  const statuses = options.statuses.length ? options.statuses : ["Active", "Temporarily suspended", "Paused", "Blocked"];

  const filteredUsers = useMemo(() => {
    const query = search.trim().toLowerCase();
    return users.filter((user) => {
      const matchesQuery = !query || [user.name, user.email, user.role, user.status].some((value) => value?.toLowerCase().includes(query));
      const matchesRole = roleFilter === "All roles" || user.role === roleFilter;
      const matchesStatus = statusFilter === "All statuses" || user.status === statusFilter;
      return matchesQuery && matchesRole && matchesStatus;
    });
  }, [roleFilter, search, statusFilter, users]);

  const setRole = (role: string) => {
    setForm((current) => ({
      ...current,
      role,
      customerId: editing && isCustomerRole(role) ? editing.customerId || "" : "",
      assignedCustomerIds: [],
      permissions: options.roleDefaults[role] || [],
      pageAccess: options.pageAccessDefaults[role] || []
    }));
  };

  const toggle = (key: "permissions" | "pageAccess", value: string) => {
    setForm((current) => {
      const values = current[key] || [];
      return { ...current, [key]: values.includes(value) ? values.filter((item) => item !== value) : [...values, value] };
    });
  };

  const openCreate = () => {
    setEditing(null);
    setForm(defaultUserForm(options));
    setShowPassword(false);
    setModalOpen(true);
  };

  const openEdit = (user: User) => {
    setFocusedId(user.id);
    setEditing(user);
    setForm({
      name: user.name,
      email: user.email,
      password: "",
      role: user.role,
      status: user.status || "Active",
      customerId: isCustomerRole(user.role) ? user.customerId || "" : "",
      assignedCustomerIds: [],
      permissions: user.permissions || [],
      pageAccess: user.pageAccess || [],
      suspensionNotice: user.suspensionNotice || ""
    });
    setShowPassword(false);
    setModalOpen(true);
  };

  const closeModal = () => {
    if (busy) return;
    setModalOpen(false);
    setEditing(null);
    setShowPassword(false);
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      const payload = {
        ...form,
        customerId: customerRole ? editing?.customerId || "" : "",
        assignedCustomerIds: []
      };
      if (editing) {
        await apiUpdateUser(editing.id, payload, session.token);
        await onAction("User access updated.");
      } else {
        await apiCreateUser(payload, session.token);
        await onAction("User created.");
      }
      await load();
      setModalOpen(false);
      setEditing(null);
    } catch (error) {
      await onAction(messageFromError(error, "User access could not be saved."));
    } finally {
      setBusy(false);
    }
  };

  const remove = async (user: User) => {
    const typed = window.prompt(`Type DELETE to permanently remove ${user.email}.`);
    if (typed !== "DELETE") return;
    setBusy(true);
    try {
      await apiDeleteUser(user.id, session.token);
      await onAction("User permanently deleted.");
      await load();
    } catch (error) {
      await onAction(messageFromError(error, "User could not be deleted."));
    } finally {
      setBusy(false);
    }
  };

  const activeUsers = users.filter((user) => user.status === "Active").length;
  const internalUsers = users.filter((user) => !isCustomerRole(user.role)).length;
  const customerUsers = users.filter((user) => isCustomerRole(user.role)).length;

  return (
    <section className="cc-access-workspace">
      <header className="cc-access-heading">
        <div>
          <span>Identity and permissions</span>
          <h2>Team directory</h2>
          <p>Control sign-in, portal type and access for every account.</p>
        </div>
        <button className="cc-access-create" onClick={openCreate} type="button"><UserPlus size={17} /> Add user</button>
      </header>

      <div className="cc-access-summary">
        <AccessMetric icon={UsersRound} label="Matching accounts" value={String(pagination.total)} detail="Across both portals" />
        <AccessMetric icon={Check} label="Active" value={String(activeUsers)} detail="Can currently sign in" />
        <AccessMetric icon={ShieldCheck} label="Internal team" value={String(internalUsers)} detail="Owner, admin and manager" />
        <AccessMetric icon={Building2} label="Customers" value={String(customerUsers)} detail="Customer portal accounts" />
      </div>

      <div className="cc-access-toolbar">
        <label className="cc-access-search"><Search size={17} /><input aria-label="Search users" onChange={(event) => setSearch(event.target.value)} placeholder="Search name, email or role" value={search} /></label>
        <div className="cc-access-filters">
          <select aria-label="Filter by role" onChange={(event) => setRoleFilter(event.target.value)} value={roleFilter}>
            <option>All roles</option>
            {roles.map((role) => <option key={role}>{role}</option>)}
          </select>
          <select aria-label="Filter by status" onChange={(event) => setStatusFilter(event.target.value)} value={statusFilter}>
            <option>All statuses</option>
            {statuses.map((status) => <option key={status}>{status}</option>)}
          </select>
        </div>
      </div>

      {loadError && <div className="cc-access-error">{loadError}<button onClick={() => load()} type="button">Try again</button></div>}

      <div className="cc-access-layout">
        <section className="cc-access-directory" aria-label="User accounts">
          <header><span>User</span><span>Access type</span><span>Last activity</span><span>Status</span><span /></header>
          {loading ? <div className="cc-access-empty">Loading user accounts...</div> : filteredUsers.length ? filteredUsers.map((user) => (
            <article
              className={focused?.id === user.id ? "selected" : ""}
              key={user.id}
              onClick={() => setFocusedId(user.id)}
              onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") setFocusedId(user.id); }}
              role="button"
              tabIndex={0}
            >
              <div className="cc-access-person"><ProfileAvatar token={session.token} user={user} /><div><strong>{user.name}</strong><small>{user.email}</small></div></div>
              <div className="cc-access-role"><strong>{user.role}</strong><small>{isCustomerRole(user.role) ? "Customer portal" : `${user.pageAccess?.length || 0} pages`}</small></div>
              <div className="cc-access-last"><strong>{user.lastOnline || user.lastLogin ? date(user.lastOnline || user.lastLogin) : "Never"}</strong><small>{user.failedLoginCount ? `${user.failedLoginCount} failed attempts` : "No sign-in alerts"}</small></div>
              <b className={`cc-user-status ${statusClass(user.status)}`}>{user.status || "Active"}</b>
              <ChevronRight aria-hidden="true" size={17} />
            </article>
          )) : <div className="cc-access-empty">No accounts match these filters.</div>}
          {pagination.hasMore && <div className="cc-pagination-more"><button className="cc-button" disabled={loading} onClick={() => void load(pagination.loaded, true)} type="button">See more</button><small>{pagination.total - pagination.loaded} remaining</small></div>}
        </section>

        <aside className="cc-access-inspector">
          {focused ? (
            <>
              <header>
                <ProfileAvatar className="cc-access-large-avatar" token={session.token} user={focused} />
                <div><small>{focused.role} account</small><h3>{focused.name}</h3><p>{focused.email}</p></div>
                <div className="cc-access-inspector-actions">
                  <button aria-label={`Edit ${focused.name}`} onClick={() => openEdit(focused)} title="Edit account" type="button"><Pencil size={16} /></button>
                  <button aria-label={`Delete ${focused.name}`} className="danger" disabled={focused.id === session.user.id || busy} onClick={() => remove(focused)} title={focused.id === session.user.id ? "You cannot delete your own account" : "Delete account"} type="button"><Trash2 size={16} /></button>
                </div>
              </header>

              <div className="cc-access-security-line"><ShieldCheck size={18} /><div><strong>{focused.status || "Active"}</strong><small>{isCustomerRole(focused.role) ? "Signs in to the customer portal" : "Signs in to the internal portal"}</small></div></div>
              {isCustomerRole(focused.role) && canReviewCustomerKYC(session) && (
                <section className="cc-access-detail-section cc-access-verify">
                  <header><span>Customer verification</span><VerificationBadge status={linkedCustomer?.verificationStatus} /></header>
                  <p>
                    {linkedCustomer
                      ? `${linkedCustomer.companyName} · ${verificationLabel(linkedCustomer.verificationStatus)} · ${(linkedCustomer.documents || []).length} document${(linkedCustomer.documents || []).length === 1 ? "" : "s"}`
                      : focused.customerId
                        ? `Linked customer ${focused.customerId} (details loading or missing)`
                        : "No linked customer account yet"}
                  </p>
                  {linkedCustomer && (
                    <button
                      className="cc-access-open-docs"
                      onClick={() => {
                        updateCustomerPath(linkedCustomer.id);
                        window.history.replaceState(
                          { page: "Customers", customerId: linkedCustomer.id },
                          "",
                          `/customers/${encodeURIComponent(linkedCustomer.id)}?tab=Documents`
                        );
                        window.dispatchEvent(new Event("cc-route-change"));
                        onNavigate?.("Customers");
                      }}
                      type="button"
                    >
                      Open documents
                    </button>
                  )}
                </section>
              )}
              {focused.status === "Temporarily suspended" && (
                <section className="cc-access-suspension-card">
                  <header><ShieldAlert size={17} /><div><strong>Restricted sign-in notice</strong><small>This user can only see the account notice and contact action.</small></div></header>
                  <p>{focused.suspensionNotice || "Your account is temporarily suspended. Contact the administrator for help."}</p>
                  {focused.suspensionContactAt && <div className="cc-access-review-request"><span>Review requested</span><strong>{focused.suspensionContactMessage || "Please review my account."}</strong><small>{date(focused.suspensionContactAt)}</small></div>}
                </section>
              )}

              <section className="cc-access-detail-section">
                <header><span>Page access</span><b>{focused.pageAccess?.length || 0}</b></header>
                <div className="cc-access-token-list">{focused.pageAccess?.length ? focused.pageAccess.map((page) => <span key={page}>{pageLabel(page)}</span>) : <small>No pages assigned</small>}</div>
              </section>

              <section className="cc-access-detail-section">
                <header><span>Action permissions</span><b>{focused.permissions?.length || 0}</b></header>
                <div className="cc-access-permission-list">{focused.permissions?.length ? focused.permissions.map((permission) => <div key={permission}><LockKeyhole size={14} /><span>{permissionLabel(permission)}</span></div>) : <small>No action permissions assigned</small>}</div>
              </section>

              <section className="cc-access-detail-section cc-access-ips">
                <header><span>Known IP addresses</span><Globe size={16} /></header>
                {(focused.knownIps || []).length > 0 ? (focused.knownIps || []).map((entry) => (
                  <article key={entry.ip}>
                    <strong title={entry.ip}>{entry.ip}</strong>
                    <small>Last used {dateTime(entry.lastSeenAt)}</small>
                    <small>First seen {dateTime(entry.firstSeenAt)}</small>
                    <span>{entry.successCount || 0} ok{entry.failCount ? ` · ${entry.failCount} failed` : ""}</span>
                  </article>
                )) : <small>No IP addresses recorded yet. They appear after sign-in.</small>}
              </section>

              <section className="cc-access-detail-section cc-access-activity">
                <header><span>Login activity</span><Activity size={16} /></header>
                {(focused.loginActivity || []).slice(0, 8).map((entry) => (
                  <article key={`${entry.at}-${entry.result}-${entry.ip}`}>
                    <i className={entry.result.toLowerCase().includes("success") ? "success" : "warning"} />
                    <div>
                      <strong>{entry.result}</strong>
                      {entry.ip ? <code className="cc-access-ip" title={entry.ip}>{entry.ip}</code> : null}
                      <small>{entry.detail || entry.userAgent || "Sign-in event"}</small>
                    </div>
                    <time dateTime={entry.at}>{dateTime(entry.at)}</time>
                  </article>
                ))}
                {(focused.loginActivity || []).length === 0 && <small>No login activity recorded yet.</small>}
              </section>
            </>
          ) : <div className="cc-access-empty">Select an account to inspect its access.</div>}
        </aside>
      </div>

      {modalOpen && (
        <div className="cc-access-modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) closeModal(); }} role="presentation">
          <section aria-labelledby="cc-access-modal-title" aria-modal="true" className="cc-access-modal" role="dialog">
            <header>
              <div><span>{editing ? "Edit access" : "New account"}</span><h2 id="cc-access-modal-title">{editing ? editing.name : "Add a user"}</h2><p>Set login credentials and exactly what this person can access.</p></div>
              <button aria-label="Close user form" disabled={busy} onClick={closeModal} title="Close" type="button"><X size={19} /></button>
            </header>

            <form className="cc-access-modal-form" onSubmit={submit}>
              <section className="cc-access-form-section">
                <div className="cc-access-section-title"><span>01</span><div><strong>Account details</strong><small>Identity and secure login</small></div></div>
                <div className="cc-access-form-grid">
                  <label><span>Full name</span><div className="cc-access-input"><UsersRound size={16} /><input autoFocus required onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} placeholder="Enter full name" value={form.name} /></div></label>
                  <label><span>Email / username</span><div className="cc-access-input"><Mail size={16} /><input required type="email" onChange={(event) => setForm((current) => ({ ...current, email: event.target.value }))} placeholder="name@company.com" value={form.email} /></div></label>
                  <label><span>{editing ? "New password (optional)" : "Password"}</span><div className="cc-access-input"><KeyRound size={16} /><input minLength={6} required={!editing} type={showPassword ? "text" : "password"} onChange={(event) => setForm((current) => ({ ...current, password: event.target.value }))} placeholder={editing ? "Keep current password" : "Minimum 6 characters"} value={form.password || ""} /><button aria-label={showPassword ? "Hide password" : "Show password"} onClick={() => setShowPassword((current) => !current)} title={showPassword ? "Hide password" : "Show password"} type="button">{showPassword ? <EyeOff size={16} /> : <Eye size={16} />}</button></div></label>
                  <label><span>Account status</span><select onChange={(event) => setForm((current) => ({ ...current, status: event.target.value }))} value={form.status}>{statuses.map((status) => <option key={status}>{status}</option>)}</select></label>
                </div>
                {form.status === "Temporarily suspended" && (
                  <label className="cc-access-suspension-field">
                    <span><ShieldAlert size={16} /> Suspension notice</span>
                    <textarea onChange={(event) => setForm((current) => ({ ...current, suspensionNotice: event.target.value }))} placeholder="Explain why access is paused and how the user can resolve it." required rows={3} value={form.suspensionNotice || ""} />
                    <small>The user can sign in, read this notice and contact an administrator. All other pages and actions stay locked.</small>
                  </label>
                )}
              </section>

              <section className="cc-access-form-section">
                <div className="cc-access-section-title"><span>02</span><div><strong>Portal and role</strong><small>Customer or internal workspace</small></div></div>
                <div className="cc-access-role-picker">{roles.map((role) => <button className={form.role === role ? "active" : ""} key={role} onClick={() => setRole(role)} type="button"><span>{role}</span><small>{isCustomerRole(role) ? "Own customer portal and account" : role === "Manager" ? "Custom internal page access" : "Internal administration access"}</small>{form.role === role && <Check size={16} />}</button>)}</div>
                <div className="cc-access-portal-note"><ShieldCheck size={18} /><div><strong>{customerRole ? "Customer portal account" : "Internal team account"}</strong><small>{customerRole ? "This login gets its own customer workspace automatically. It is not assigned to another customer." : "Choose the internal pages and actions this team member can use."}</small></div></div>
              </section>

              <section className="cc-access-form-section">
                <div className="cc-access-section-title"><span>03</span><div><strong>Operational page access</strong><small>Personal Settings remains available to every active user</small></div></div>
                <div className="cc-access-choice-grid">{pageChoices.map((page) => <label className={(form.pageAccess || []).includes(page) ? "selected" : ""} key={page}><input checked={(form.pageAccess || []).includes(page)} onChange={() => toggle("pageAccess", page)} type="checkbox" /><span>{pageLabel(page)}</span><Check size={15} /></label>)}</div>
              </section>

              <section className="cc-access-form-section">
                <div className="cc-access-section-title"><span>04</span><div><strong>Action permissions</strong><small>Control what the account can change</small></div></div>
                <div className="cc-access-choice-grid permissions">{permissionChoices.map((permission) => <label className={(form.permissions || []).includes(permission) ? "selected" : ""} key={permission}><input checked={(form.permissions || []).includes(permission)} onChange={() => toggle("permissions", permission)} type="checkbox" /><span>{permissionLabel(permission)}</span><Check size={15} /></label>)}</div>
              </section>

              <footer><button disabled={busy} onClick={closeModal} type="button">Cancel</button><button className="primary" disabled={busy} type="submit">{busy ? "Saving..." : editing ? "Save changes" : "Create account"}</button></footer>
            </form>
          </section>
        </div>
      )}
    </section>
  );
}

function mergeUsers(current: User[], incoming: User[]) {
  const ids = new Set(current.map((user) => user.id));
  return [...current, ...incoming.filter((user) => !ids.has(user.id))];
}

function AccessMetric({ icon: Icon, label, value, detail }: { icon: typeof UsersRound; label: string; value: string; detail: string }) {
  return <article><Icon size={18} /><div><span>{label}</span><strong>{value}</strong><small>{detail}</small></div></article>;
}

function statusClass(value?: string) {
  return (value || "Active").toLowerCase().replace(/\s+/g, "-");
}
