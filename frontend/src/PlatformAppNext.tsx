import { accountingInvalidatedEvent, financialScopes } from "./platform/accounting";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  Bell,
  CheckCheck,
  ChevronDown,
  ClipboardList,
  FilePlus2,
  FileText,
  Home,
  LayoutDashboard,
  Languages,
  LogOut,
  Mail,
  Menu,
  MessageSquare,
  Package,
  PanelLeftClose,
  PanelLeftOpen,
  PhoneCall,
  Settings,
  ShieldAlert,
  Truck,
  UserCog,
  Users,
  WalletCards,
  Store,
  X,
  type LucideIcon
} from "lucide-react";
import { apiGet, apiPost, readSession } from "./platform/api";
import { applyCustomerPresence } from "./platform/customer-presence";
import { AdminPortal } from "./platform/admin-portal";
import { adminPages, customerPages, emptyWorkspace, fallbackBlueprint, fallbackFoundation } from "./platform/constants";
import { CustomerPortal, IdentityActionNotice } from "./platform/customer-portal";
import { DirectCallLayer } from "./platform/direct-call-layer";
import { PortalErrorBoundary } from "./platform/error-boundary";
import { PublicEntry } from "./platform/public-entry";
import { PublicDocumentForm, verifyTokenFromPath } from "./platform/public-document-form";
import { PublicDirectoryPage, publicDirectoryFromPath } from "./platform/public-directory";
import {
  VERIFY_IDENTITY_PAGE,
  VerificationBadge,
  VerifiedAvatarMark,
  customerNeedsIdentityAction,
  isCustomerVerified
} from "./platform/customer-documents";
import { ProfileAvatar } from "./platform/profile-avatar";
import { SuspensionGate } from "./platform/suspension-gate";
import {
  applyPlatformTheme,
  cachePlatformSettings,
  normalizePlatformSettings,
  readCachedPlatformSettings,
  readLanguageChoice,
  saveLanguageChoice,
  translateText,
  useAutomaticTranslation,
  type PlatformLanguage
} from "./platform/preferences";
import { pageFromPath, pathForPage, updatePagePath, updatePublicPath } from "./platform/routes";
import type { Blueprint, Customer, Foundation, PageInfo, PlatformSettings, Session, User, Workspace, WorkspacePage } from "./platform/types";
import { messageFromError } from "./platform/utils";
import {
  apiMutationEventName,
  applyWorkspaceChanges,
  changesFromApiMutation,
  connectWorkspaceRealtime,
  userAccessEventName,
  type ApiMutationDetail
} from "./platform/workspace-realtime";

const pageIcons: Record<string, LucideIcon> = {
  Command: LayoutDashboard,
  Home: LayoutDashboard,
  Customers: Users,
  "Users & Access": UserCog,
  Quotations: FileText,
  "Get Quote": FilePlus2,
  Products: Package,
  Orders: ClipboardList,
  Shipping: Truck,
  Payments: WalletCards,
  Directory: Store,
  "App home": Home,
  Messages: MessageSquare,
  Calls: PhoneCall,
  Email: Mail,
  Settings,
  [VERIFY_IDENTITY_PAGE]: ShieldAlert
};

function PlatformAppNext() {
  const [session, setSession] = useState<Session | null>(() => readSession());
  const [platformSettings, setPlatformSettings] = useState<PlatformSettings>(readCachedPlatformSettings);
  const [language, setLanguage] = useState<PlatformLanguage>(() => readLanguageChoice(readCachedPlatformSettings(), session?.user.id));
  const [page, setPage] = useState(() => pageFromPath(window.location.pathname, [...adminPages, ...customerPages]) || "Command");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(readSidebarState);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [notice, setNotice] = useState("");
  const noticeTimer = useRef<number | null>(null);
  const [foundation, setFoundation] = useState<Foundation>(fallbackFoundation);
  const [blueprint, setBlueprint] = useState<Blueprint>(fallbackBlueprint);
  const [customerRecords, setCustomers] = useState<Customer[]>([]);
  const [customerPagination, setCustomerPagination] = useState<PageInfo>({ loaded: 0, total: 0, hasMore: false });
  const [workspace, setWorkspace] = useState<Workspace>(emptyWorkspace);
  const [loadingMore, setLoadingMore] = useState(false);
  const customers = useMemo(() => applyCustomerPresence(customerRecords, workspace.presence || []), [customerRecords, workspace.presence]);

  useAutomaticTranslation(language);

  const showNotice = useCallback((message: string) => {
    if (noticeTimer.current !== null) window.clearTimeout(noticeTimer.current);
    setNotice(message);
    noticeTimer.current = message
      ? window.setTimeout(() => {
          setNotice("");
          noticeTimer.current = null;
        }, 4000)
      : null;
  }, []);

  useEffect(() => () => {
    if (noticeTimer.current !== null) window.clearTimeout(noticeTimer.current);
  }, []);

  const acceptPlatformSettings = useCallback((value: PlatformSettings) => {
    setPlatformSettings(normalizePlatformSettings(value));
  }, []);

  const changeLanguage = (next: PlatformLanguage) => {
    setLanguage(next);
    saveLanguageChoice(next, session?.user.id);
  };

  useEffect(() => {
    applyPlatformTheme(platformSettings);
    cachePlatformSettings(platformSettings);
  }, [platformSettings]);

  useEffect(() => {
    setLanguage(readLanguageChoice(platformSettings, session?.user.id));
  }, [platformSettings.defaultLanguage, platformSettings.allowUserLanguageChoice, session?.user.id]);

  const acceptSessionUser = useCallback((user: User) => {
    setSession((current) => {
      if (!current) return current;
      const next = { ...current, user };
      localStorage.setItem("cc_session", JSON.stringify(next));
      return next;
    });
  }, []);

  const loadCustomers = useCallback(async (token?: string) => {
    if (!token) {
      setCustomers([]);
      return;
    }
    try {
      const data = await apiGet<{ customers: Customer[]; pagination?: PageInfo }>("/api/customers?limit=20", token);
      setCustomers(data.customers);
      setCustomerPagination(data.pagination || { loaded: data.customers.length, total: data.customers.length, hasMore: false });
    } catch {
      // Keep the last successful customer list during a temporary interruption.
    }
  }, []);

  const loadBase = useCallback(async (token?: string) => {
    await Promise.all([
      apiGet<Foundation>("/api/foundation").then(setFoundation).catch(() => undefined),
      apiGet<Blueprint>("/api/blueprint").then(setBlueprint).catch(() => undefined),
      apiGet<PlatformSettings>("/api/platform/settings").then(acceptPlatformSettings).catch(() => undefined),
      loadCustomers(token)
    ]);
  }, [acceptPlatformSettings, loadCustomers]);

  const loadWorkspace = useCallback(async (token?: string) => {
    if (!token) {
      setWorkspace(emptyWorkspace);
      return;
    }
    try {
      const payload = await apiGet<Workspace>("/api/workspace", token);
      setWorkspace({
        ...emptyWorkspace,
        ...payload,
        notices: Array.isArray(payload.notices) ? payload.notices : [],
        featuredProducts: Array.isArray(payload.featuredProducts) ? payload.featuredProducts : []
      });
    } catch {
      // Stale data is safer and calmer than blanking the current workspace.
    }
  }, []);

  const loadMore = useCallback(async () => {
    if (!session || loadingMore) return;
    setLoadingMore(true);
    try {
      if (page === "Customers") {
        const data = await apiGet<{ customers: Customer[]; pagination: PageInfo }>(`/api/customers?offset=${customerPagination.loaded}&limit=20`, session.token);
        setCustomers((current) => mergeById(current, data.customers));
        setCustomerPagination(data.pagination);
        return;
      }
      const scopes = scopesForPage(page).filter((scope) => workspace.pagination[scope]?.hasMore);
      const pages = await Promise.all(scopes.map((scope) => {
        const offset = workspace.pagination[scope]?.loaded || 0;
        return apiGet<WorkspacePage>(`/api/workspace/page?scope=${encodeURIComponent(scope)}&offset=${offset}&limit=20`, session.token);
      }));
      setWorkspace((current) => {
        let next = current;
        for (const result of pages) next = mergeWorkspacePage(next, result);
        return next;
      });
    } catch (error) {
      showNotice(messageFromError(error, "Could not load more records."));
    } finally {
      setLoadingMore(false);
    }
  }, [customerPagination.loaded, loadingMore, page, session, showNotice, workspace.pagination]);

  useEffect(() => {
    if (!session || isTemporarilySuspended(session.user.status)) return;
    let active = true;
    let version = 0;
    let running = false;
    let timer: number | undefined;
    const token = session.token;
    const refresh = async () => {
      timer = undefined;
      if (running || !active) return;
      running = true;
      const started = version;
      try {
        const metrics = await apiGet<Workspace["metrics"]>("/api/accounting/metrics", token);
        if (active && started === version) setWorkspace((current) => ({ ...current, metrics: { ...current.metrics, ...metrics } }));
      } catch {
        // Keep the last server-confirmed total while offline.
      } finally {
        running = false;
        if (active && started !== version) timer = window.setTimeout(() => void refresh(), 150);
      }
    };
    const invalidate = () => {
      version++;
      if (!running && timer === undefined) timer = window.setTimeout(() => void refresh(), 150);
    };
    window.addEventListener(accountingInvalidatedEvent, invalidate);
    return () => { active = false; window.clearTimeout(timer); window.removeEventListener(accountingInvalidatedEvent, invalidate); };
  }, [session?.token, session?.user.status]);

  const refreshSessionUser = useCallback(async (token: string) => {
    try {
      const payload = await apiGet<{ user: User }>("/api/auth/me", token);
      acceptSessionUser(payload.user);
    } catch (error) {
      if (messageFromError(error, "") !== "Sign in is required.") return;
      localStorage.removeItem("cc_session");
      setSession(null);
      showNotice("Session expired. Please sign in again.");
    }
  }, [acceptSessionUser, showNotice]);

  useEffect(() => {
    void loadBase();
  }, [loadBase]);

  useEffect(() => {
    if (!session) return;
    const token = session.token;
    const refreshWhenVisible = () => {
      if (document.visibilityState === "visible") void refreshSessionUser(token);
    };
    void refreshSessionUser(token);
    window.addEventListener("online", refreshWhenVisible);
    document.addEventListener("visibilitychange", refreshWhenVisible);
    return () => {
      window.removeEventListener("online", refreshWhenVisible);
      document.removeEventListener("visibilitychange", refreshWhenVisible);
    };
  }, [refreshSessionUser, session?.token]);

  useEffect(() => {
    if (!session || isTemporarilySuspended(session.user.status)) {
      setWorkspace(emptyWorkspace);
      return;
    }
    void loadBase(session.token);
    void loadWorkspace(session.token);
  }, [loadBase, loadWorkspace, session?.token, session?.user.status]);

  useEffect(() => {
    const handleMutation = (event: Event) => {
      const detail = (event as CustomEvent<ApiMutationDetail>).detail;
      if (!detail?.path) return;
      const changes = changesFromApiMutation(detail);
      if (changes.some((change) => financialScopes.has(change.scope)) || detail.path.startsWith("/api/accounting/")) window.dispatchEvent(new Event(accountingInvalidatedEvent));
      if (changes.length) {
        setWorkspace((current) => {
          if (!current.pagination || Object.keys(current.pagination).length === 0) return current;
          return applyWorkspaceChanges(current, changes);
        });
      }
      if (detail.path === "/api/platform/settings") acceptPlatformSettings(detail.data as PlatformSettings);
    };
    window.addEventListener(apiMutationEventName, handleMutation);
    return () => window.removeEventListener(apiMutationEventName, handleMutation);
  }, [acceptPlatformSettings]);

  useEffect(() => {
    if (!session) return;
    const token = session.token;
    return connectWorkspaceRealtime(token, {
      onChanges: (changes) => {
        if (changes.some((change) => financialScopes.has(change.scope))) window.dispatchEvent(new Event(accountingInvalidatedEvent));
        const patches = changes.filter((change) => change.operation !== "invalidate" && change.scope !== "platform");
        if (patches.length) {
          setWorkspace((current) => {
            // Ignore incremental patches until the first full /api/workspace snapshot
            // arrives — otherwise healed order events paint order-sum balance first.
            if (!current.pagination || Object.keys(current.pagination).length === 0) return current;
            return applyWorkspaceChanges(current, patches);
          });
        }

        const platform = changes.find((change) => change.scope === "platform" && change.data);
        if (platform?.data) acceptPlatformSettings(platform.data as PlatformSettings);
        if (changes.some((change) => change.scope === "customers")) void loadCustomers(token);
        if (changes.some((change) => change.scope === "users")) {
          void refreshSessionUser(token);
          window.dispatchEvent(new CustomEvent(userAccessEventName));
        }
      },
      onResync: () => {
        void refreshSessionUser(token);
        void loadCustomers(token);
        if (!isTemporarilySuspended(session.user.status)) void loadWorkspace(token);
      }
    });
  }, [
    acceptPlatformSettings,
    loadCustomers,
    loadWorkspace,
    refreshSessionUser,
    session?.token,
    session?.user.customerId,
    session?.user.role,
    session?.user.status
  ]);

  const pages = useMemo(() => {
    if (!session) return [];
    const fallback = isCustomerRole(session.user.role) ? customerPages : adminPages;
    if (!session.user.pageAccess?.length) return fallback;
    const granted = session.user.pageAccess.flatMap((item) => {
      if (item === "Messages & Calls") return ["Messages", "Calls"];
      if (item === "Manufacturing") return ["Orders"];
      return [item];
    });
    const grantedSet = new Set([...granted, "Settings"]);
    const role = session.user.role.toLowerCase();
    if (role === "owner" || role === "admin" || session.user.permissions.includes("email.read") || session.user.permissions.includes("email.manage")) {
      grantedSet.add("Email");
    }
    // Directory shipped after many accounts were saved; keep it visible for roles that own it by default.
    if (
      role === "owner" ||
      role === "admin" ||
      role === "manager" ||
      role === "support agent" ||
      isCustomerRole(session.user.role) ||
      session.user.permissions.includes("directory.manage")
    ) {
      grantedSet.add("Directory");
    }
    if (role === "owner" || role === "admin" || role === "manager") {
      grantedSet.add("App home");
    }
    return fallback.filter((item) => grantedSet.has(item));
  }, [session]);

  const customer = useMemo(() => customers.find((row) => row.id === session?.user.customerId), [customers, session]);
  const needsIdentityAction = Boolean(session && isCustomerRole(session.user.role) && customerNeedsIdentityAction(customer));
  const showIdentityPage = needsIdentityAction;

  const visiblePages = useMemo(() => {
    if (!showIdentityPage) return pages;
    if (pages.includes(VERIFY_IDENTITY_PAGE)) return pages;
    const settingsIndex = pages.indexOf("Settings");
    if (settingsIndex < 0) return [...pages, VERIFY_IDENTITY_PAGE];
    return [...pages.slice(0, settingsIndex), VERIFY_IDENTITY_PAGE, ...pages.slice(settingsIndex)];
  }, [showIdentityPage, pages]);

  useEffect(() => {
    if (!session || visiblePages.length === 0) return;

    const syncFromLocation = () => {
      const routedPage = pageFromPath(window.location.pathname, visiblePages);
      const nextPage = routedPage || visiblePages[0];
      setPage(nextPage);
      if (!routedPage) updatePagePath(nextPage, true);
    };

    syncFromLocation();
    window.addEventListener("popstate", syncFromLocation);
    return () => window.removeEventListener("popstate", syncFromLocation);
  }, [visiblePages, session?.token]);

  useEffect(() => {
    if (page !== VERIFY_IDENTITY_PAGE || showIdentityPage) return;
    setPage("Home");
    updatePagePath("Home", true);
  }, [showIdentityPage, page]);

  useEffect(() => {
    document.title = session ? `${translateText(pageLabel(page), language)} | ChakuChuri.pk` : "ChakuChuri.pk";
  }, [language, page, session]);

  const navigateToPage = (nextPage: string) => {
    const verifyAllowed =
      nextPage === VERIFY_IDENTITY_PAGE &&
      Boolean(session && isCustomerRole(session.user.role) && customer && showIdentityPage);
    if (!visiblePages.includes(nextPage) && !verifyAllowed) return;
    setPage(nextPage);
    setMobileNavOpen(false);
    updatePagePath(nextPage);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const saveSession = (next: Session) => {
    localStorage.setItem("cc_session", JSON.stringify(next));
    setSession(next);
    showNotice(`Signed in as ${next.user.name}.`);
  };

  const afterAction = async (message: string) => {
    showNotice(message);
  };

  const toggleSidebar = () => {
    setSidebarCollapsed((current) => {
      const next = !current;
      localStorage.setItem("cc_sidebar_collapsed", next ? "1" : "0");
      return next;
    });
  };

  const signOut = () => {
    const token = session?.token;
    localStorage.removeItem("cc_session");
    setSession(null);
    setPage("Command");
    updatePublicPath();
    if (token) {
      void apiPost("/api/auth/logout", {}, token).catch(() => undefined);
    }
  };

  const verifyToken = verifyTokenFromPath(window.location.pathname);
  if (verifyToken) {
    return <PublicDocumentForm token={verifyToken} />;
  }

  if (publicDirectoryFromPath(window.location.pathname) && !session) {
    return <PublicDirectoryPage />;
  }

  if (!session) {
    const allowLanguageChoice = platformSettings.allowUserLanguageChoice && platformSettings.enabledLanguages.length > 1;
    return (
      <PublicEntry
        allowLanguageChoice={allowLanguageChoice}
        blueprint={blueprint}
        foundation={foundation}
        language={language}
        notice={notice}
        onLanguageChange={changeLanguage}
        onNotice={showNotice}
        onSession={saveSession}
      />
    );
  }

  if (isTemporarilySuspended(session.user.status)) {
    return <SuspensionGate onSignOut={signOut} onUserChange={acceptSessionUser} session={session} />;
  }

  const customerRole = isCustomerRole(session.user.role);
  const allowLanguageChoice = platformSettings.allowUserLanguageChoice && platformSettings.enabledLanguages.length > 1;
  const primaryPages = visiblePages.filter((item) => item !== "Settings" && item !== VERIFY_IDENTITY_PAGE);
  const utilityPages = visiblePages.filter((item) => item === VERIFY_IDENTITY_PAGE || item === "Settings");

  return (
    <div className={`cc-portal-shell ${sidebarCollapsed ? "is-collapsed" : ""} ${mobileNavOpen ? "is-mobile-nav-open" : ""}`}>
      {mobileNavOpen && <button aria-label="Close navigation" className="cc-mobile-nav-backdrop" onClick={() => setMobileNavOpen(false)} type="button" />}
      <aside className="cc-portal-sidebar" aria-label="Portal navigation">
        <div className="cc-sidebar-head">
          <div className="cc-brand" title={sidebarCollapsed ? "ChakuChuri.pk" : undefined}>
            <span>CC</span>
            <div className="cc-brand-copy">
              <strong>ChakuChuri.pk</strong>
            </div>
          </div>
          <div className="cc-sidebar-head-actions">
            <NotificationCenter
              customerRole={customerRole}
              onNavigate={navigateToPage}
              token={session.token}
              userId={session.user.id}
              initialReadIds={session.user.notificationReadIds || []}
              workspace={workspace}
            />
            <button
              aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
              className="cc-sidebar-toggle"
              onClick={toggleSidebar}
              title={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
              type="button"
            >
              {sidebarCollapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
            </button>
            <button aria-label="Close navigation" className="cc-mobile-nav-close" onClick={() => setMobileNavOpen(false)} type="button">
              <X size={20} />
            </button>
          </div>
        </div>

        <nav aria-label="Workspace pages">
          {primaryPages.map((item) => (
            <PortalNavLink
              active={page === item}
              collapsed={sidebarCollapsed}
              item={item}
              key={item}
              onNavigate={navigateToPage}
            />
          ))}
        </nav>

        <div className="cc-sidebar-footer">
          {utilityPages.map((item) => (
            <PortalNavLink
              active={page === item}
              collapsed={sidebarCollapsed}
              item={item}
              key={item}
              onNavigate={navigateToPage}
            />
          ))}
          <button className="cc-signout" onClick={signOut} title={sidebarCollapsed ? "Sign out" : undefined} type="button">
            <LogOut size={18} />
            <span className="cc-nav-label">Sign out</span>
          </button>
          <div className="cc-sidebar-account" title={sidebarCollapsed ? `${session.user.name} / ${session.user.role}` : undefined}>
            <span className="cc-avatar-wrap">
              <ProfileAvatar className="cc-sidebar-avatar" token={session.token} user={session.user} />
              {customerRole && sidebarCollapsed && <VerifiedAvatarMark status={customer?.verificationStatus} />}
            </span>
            <div className="cc-account-copy">
              <strong>{session.user.name}</strong>
              <small>{session.user.role}</small>
              {customerRole && !sidebarCollapsed && <VerificationBadge compact size="sm" status={customer?.verificationStatus} />}
            </div>
          </div>
        </div>
      </aside>

      <main className="cc-portal-main">
        <header className="cc-mobile-appbar">
          <button aria-expanded={mobileNavOpen} aria-label="Open navigation" className="cc-mobile-menu" onClick={() => setMobileNavOpen(true)} type="button">
            <Menu size={20} />
          </button>
          <div className="cc-mobile-brand">
            <span>CC</span>
            <div className="cc-mobile-title">
              <strong>
                {page === "Command" || page === "Home" ? greeting(session.user.name) : pageLabel(page)}
              </strong>
              <small>
                {page === "Command" || page === "Home"
                  ? pageLabel(page)
                  : customerRole
                    ? session.user.name
                    : "Company workspace"}
              </small>
            </div>
          </div>
          <div className="cc-mobile-actions">
            {needsIdentityAction && (
              <button
                aria-label="Action required"
                className="cc-identity-action-icon"
                onClick={() => navigateToPage(VERIFY_IDENTITY_PAGE)}
                type="button"
              >
                <ShieldAlert size={18} />
              </button>
            )}
            {allowLanguageChoice && <LanguageSwitch compact language={language} onChange={changeLanguage} />}
            <NotificationCenter customerRole={customerRole} onNavigate={navigateToPage} token={session.token} userId={session.user.id} initialReadIds={session.user.notificationReadIds || []} workspace={workspace} />
          </div>
        </header>

        <header className="cc-portal-topbar">
          <div>
            <span className="cc-kicker">{customerRole ? session.user.name : "Company workspace"}</span>
            <h1>{page === "Command" || page === "Home" ? greeting(session.user.name) : page}</h1>
            <p>{pageDescription(page)}</p>
          </div>
          <div className="cc-topbar-actions">
            {needsIdentityAction && <IdentityActionNotice onOpen={() => navigateToPage(VERIFY_IDENTITY_PAGE)} />}
            {allowLanguageChoice && <LanguageSwitch language={language} onChange={changeLanguage} />}
          </div>
        </header>

        <div className="cc-portal-content">
          <PortalErrorBoundary key={page} page={page}>
            {!customerRole ? (
              <AdminPortal
                foundation={foundation}
                page={page}
                session={session}
                customers={customers}
                workspace={workspace}
                platformSettings={platformSettings}
                onAction={afterAction}
                onPlatformSettingsChange={acceptPlatformSettings}
                onNavigate={navigateToPage}
                onNotice={showNotice}
                onUserChange={acceptSessionUser}
                onCustomerUpdated={(next) => setCustomers((rows) => rows.map((row) => (row.id === next.id ? { ...row, ...next } : row)))}
              />
            ) : (
              <CustomerPortal
                page={page}
                session={session}
                customer={customer}
                customers={customers}
                workspace={workspace}
                onAction={afterAction}
                onNavigate={navigateToPage}
                onNotice={showNotice}
                onUserChange={acceptSessionUser}
                onCustomerUpdated={(next) => setCustomers((rows) => rows.map((row) => (row.id === next.id ? { ...row, ...next } : row)))}
              />
            )}
          </PortalErrorBoundary>
          {hasMoreForPage(page, workspace, customerPagination) && (
            <div className="cc-pagination-more">
              <button className="cc-button" disabled={loadingMore} onClick={() => void loadMore()} type="button">
                <ChevronDown size={17} /> {loadingMore ? "Loading..." : "See more"}
              </button>
              <small>{remainingForPage(page, workspace, customerPagination)} remaining</small>
            </div>
          )}
        </div>
        <DirectCallLayer customers={customers} onAction={afterAction} onRefresh={() => loadWorkspace(session.token)} session={session} workspace={workspace} />
        {notice && <div className="cc-toast">{notice}</div>}
      </main>
    </div>
  );
}

const pageScopes: Record<string, string[]> = {
  Quotations: ["quotations"], Orders: ["manufacturing"], Messages: ["conversations"],
  Shipping: ["shipping"], Payments: ["payments", "ledger", "manufacturing"], Products: ["products"], Calls: ["calls"]
};

function scopesForPage(page: string) { return pageScopes[page] || []; }

function hasMoreForPage(page: string, workspace: Workspace, customers: PageInfo) {
  if (page === "Customers") return customers.hasMore;
  return scopesForPage(page).some((scope) => workspace.pagination[scope]?.hasMore);
}

function remainingForPage(page: string, workspace: Workspace, customers: PageInfo) {
  if (page === "Customers") return Math.max(customers.total - customers.loaded, 0);
  return Math.max(...scopesForPage(page).map((scope) => {
    const info = workspace.pagination[scope];
    return info ? info.total - info.loaded : 0;
  }), 0);
}

function mergeById<T extends { id: string }>(current: T[], incoming: T[]) {
  const seen = new Set(current.map((row) => row.id));
  return [...current, ...incoming.filter((row) => !seen.has(row.id))];
}

function mergeWorkspacePage(workspace: Workspace, result: WorkspacePage): Workspace {
  if (!Object.prototype.hasOwnProperty.call(workspace, result.scope)) return workspace;
  const key = result.scope as keyof Workspace;
  const current = workspace[key];
  if (!Array.isArray(current)) return workspace;
  return {
    ...workspace,
    [key]: mergeById(current as Array<{ id: string }>, result.items),
    pagination: { ...workspace.pagination, [result.scope]: result.pagination }
  } as Workspace;
}

type PortalNotification = {
  id: string;
  title: string;
  detail: string;
  at: string;
  page: string;
  icon: LucideIcon;
};

const notificationReadEventName = "cc:notification-reads";

function NotificationCenter({
  customerRole,
  onNavigate,
  token,
  userId,
  initialReadIds,
  workspace
}: {
  customerRole: boolean;
  onNavigate: (page: string) => void;
  token: string;
  userId: string;
  initialReadIds: string[];
  workspace: Workspace;
}) {
  const root = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [readIds, setReadIds] = useState<string[]>(() => mergeReadIds(initialReadIds, readNotificationIds(userId)));
  const notifications = useMemo(() => portalNotifications(workspace, customerRole), [customerRole, workspace]);
  const unread = notifications.filter((item) => !readIds.includes(item.id));

  useEffect(() => {
    const merged = mergeReadIds(initialReadIds, readNotificationIds(userId));
    setReadIds(merged);
    setOpen(false);
    const local = readNotificationIds(userId);
    if (local.some((id) => !initialReadIds.includes(id))) {
      persistNotificationReads(token, merged);
    }
  }, [userId, token, initialReadIds.join("|")]);

  useEffect(() => {
    const syncReads = (event: Event) => {
      const detail = (event as CustomEvent<{ userId: string; ids: string[] }>).detail;
      if (!detail || detail.userId !== userId) return;
      setReadIds((current) => mergeReadIds(current, detail.ids));
    };
    const syncStorage = (event: StorageEvent) => {
      if (event.key !== notificationStorageKey(userId)) return;
      setReadIds((current) => mergeReadIds(current, readNotificationIds(userId)));
    };
    window.addEventListener(notificationReadEventName, syncReads);
    window.addEventListener("storage", syncStorage);
    return () => {
      window.removeEventListener(notificationReadEventName, syncReads);
      window.removeEventListener("storage", syncStorage);
    };
  }, [userId]);

  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", closeOnOutsideClick);
    document.addEventListener("keydown", closeOnEscape);
    apiGet<{ user: { notificationReadIds?: string[] } }>("/api/auth/me", token)
      .then((payload) => {
        setReadIds((current) => mergeReadIds(current, payload.user?.notificationReadIds || []));
      })
      .catch(() => undefined);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("mousedown", closeOnOutsideClick);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [open, token]);

  const saveReadIds = (ids: string[]) => {
    const unique = mergeReadIds(readIds, readNotificationIds(userId), ids);
    const stored = unique.slice(-200);
    setReadIds(unique);
    localStorage.setItem(notificationStorageKey(userId), JSON.stringify(stored));
    window.dispatchEvent(new CustomEvent(notificationReadEventName, { detail: { userId, ids: stored } }));
    persistNotificationReads(token, stored);
  };

  return (
    <div className="cc-notification-center" ref={root}>
      <button
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-label={`Notifications, ${unread.length} unread`}
        className="cc-notification-trigger"
        onClick={() => setOpen((current) => !current)}
        title="Notifications"
        type="button"
      >
        <Bell size={18} strokeWidth={1.8} />
        {unread.length > 0 && <i className="cc-alert-count">{unread.length > 9 ? "9+" : unread.length}</i>}
      </button>

      {open && (
        <>
          <button aria-label="Close updates" className="cc-notification-backdrop" onClick={() => setOpen(false)} type="button" />
          <section aria-label="Updates" aria-modal="true" className="cc-notification-drawer" role="dialog">
            <header>
              <div>
                <h2>Updates</h2>
                <span>{unread.length ? `${unread.length} unread` : "You're all caught up"}</span>
              </div>
              <div className="cc-notification-drawer-actions">
                {unread.length > 0 && (
                  <button aria-label="Mark all read" className="cc-mark-read" onClick={() => saveReadIds(notifications.map((item) => item.id))} title="Mark all read" type="button">
                    <CheckCheck size={18} />
                  </button>
                )}
                <button aria-label="Close updates" className="cc-notification-close" onClick={() => setOpen(false)} title="Close updates" type="button">
                  <X size={20} />
                </button>
              </div>
            </header>
            <div className="cc-notification-list">
              {notifications.length === 0 ? (
                <div className="cc-notification-empty">
                  <Bell size={22} />
                  <strong>No new notifications</strong>
                </div>
              ) : (
                notifications.map((item) => {
                  const Icon = item.icon;
                  const read = readIds.includes(item.id);
                  return (
                    <button
                      className={read ? "is-read" : ""}
                      key={item.id}
                      onClick={() => {
                        saveReadIds([item.id]);
                        setOpen(false);
                        onNavigate(item.page);
                      }}
                      type="button"
                    >
                      <span className="cc-notification-icon">
                        <Icon size={17} />
                      </span>
                      <span className="cc-notification-copy">
                        <strong>{item.title}</strong>
                        <small>{item.detail}</small>
                        <time>{relativeTime(item.at)}</time>
                      </span>
                      {!read && <i className="cc-unread-dot" />}
                    </button>
                  );
                })
              )}
            </div>
          </section>
        </>
      )}
    </div>
  );
}

function portalNotifications(workspace: Workspace, customerRole: boolean): PortalNotification[] {
  const notifications: PortalNotification[] = [];

  if (customerRole) {
    workspace.quotations
      .filter((quote) => quote.status === "Priced")
      .forEach((quote) =>
        notifications.push({
          id: `quote-priced-${quote.id}`,
          title: "Your quotation is ready",
          detail: `${quote.id} / ${quote.productName}`,
          at: quote.history.at(-1)?.createdAt || quote.createdAt,
          page: "Get Quote",
          icon: FileText
        })
      );
    workspace.manufacturing.forEach((order) => {
      const update = order.history.at(-1);
      if (!update) return;
      notifications.push({
        id: `order-update-${order.id}-${update.createdAt}`,
        title: update.label || "Order updated",
        detail: `${order.id} / ${order.currentStage}`,
        at: update.createdAt,
        page: "Orders",
        icon: ClipboardList
      });
    });
    workspace.payments
      .filter((payment) => payment.status === "Waiting confirmation")
      .forEach((payment) =>
        notifications.push({
          id: `payment-review-${payment.id}`,
          title: "Payment proof under review",
          detail: `${payment.id} / ${payment.type}`,
          at: payment.createdAt,
          page: "Payments",
          icon: WalletCards
        })
      );
    workspace.conversations
      .filter((conversation) => conversation.unreadForCustomer > 0)
      .forEach((conversation) =>
        notifications.push({
          id: `message-customer-${conversation.id}-${conversation.lastMessageAt}`,
          title: `${conversation.unreadForCustomer} new message${conversation.unreadForCustomer === 1 ? "" : "s"}`,
          detail: conversation.subject || "Support conversation",
          at: conversation.lastMessageAt || "",
          page: "Messages",
          icon: MessageSquare
        })
      );
    workspace.calls
      .filter((call) => call.status === "Ringing" || call.status === "In call")
      .forEach((call) =>
        notifications.push({
          id: `call-customer-${call.id}-${call.status}`,
          title: call.status === "In call" ? "Audio call connected" : call.initiatorRole === "Admin" ? "Incoming audio call" : "Calling support",
          detail: call.subject,
          at: call.updatedAt || call.createdAt,
          page: "Messages",
          icon: Bell
        })
      );
  } else {
    workspace.quotations
      .filter((quote) => quote.status === "Requested")
      .forEach((quote) =>
        notifications.push({
          id: `quote-request-${quote.id}`,
          title: "Quotation needs pricing",
          detail: `${quote.id} / ${quote.productName}`,
          at: quote.createdAt,
          page: "Quotations",
          icon: FileText
        })
      );
    workspace.payments
      .filter((payment) => payment.status === "Waiting confirmation")
      .forEach((payment) =>
        notifications.push({
          id: `payment-confirm-${payment.id}`,
          title: "Payment proof needs confirmation",
          detail: `${payment.id} / ${payment.type}`,
          at: payment.createdAt,
          page: "Payments",
          icon: WalletCards
        })
      );
    workspace.shipping
      .filter((shipment) => shipment.status === "Requested" || shipment.status === "Pending")
      .forEach((shipment) =>
        notifications.push({
          id: `shipping-request-${shipment.id}`,
          title: "Shipping request needs review",
          detail: `${shipment.id} / ${shipment.destination}`,
          at: shipment.createdAt,
          page: "Shipping",
          icon: Truck
        })
      );
    workspace.conversations
      .filter((conversation) => conversation.unreadForAdmin > 0)
      .forEach((conversation) =>
        notifications.push({
          id: `message-admin-${conversation.id}-${conversation.lastMessageAt}`,
          title: `${conversation.unreadForAdmin} unread customer message${conversation.unreadForAdmin === 1 ? "" : "s"}`,
          detail: conversation.subject || conversation.customerId,
          at: conversation.lastMessageAt || "",
          page: "Messages",
          icon: MessageSquare
        })
      );
    workspace.calls
      .filter((call) => call.status === "Ringing")
      .forEach((call) =>
        notifications.push({
          id: `call-admin-${call.id}`,
          title: "Incoming audio call",
          detail: call.subject,
          at: call.updatedAt || call.createdAt,
          page: "Calls",
          icon: Bell
        })
      );
  }

  return notifications
    .sort((left, right) => Date.parse(right.at || "") - Date.parse(left.at || ""))
    .slice(0, 12);
}

function mergeReadIds(...lists: Array<string[] | undefined>) {
  return [...new Set(lists.flatMap((list) => list || []).filter((id) => typeof id === "string" && id.trim()))].slice(-400);
}

function persistNotificationReads(token: string, ids: string[]) {
  if (!token) return;
  apiPost<{ user: { notificationReadIds?: string[] } }>("/api/auth/notifications/read", { ids }, token).catch(() => undefined);
}

function notificationStorageKey(userId: string) {
  return `cc_notification_reads_${userId}`;
}

function readNotificationIds(userId: string): string[] {
  try {
    const value = JSON.parse(localStorage.getItem(notificationStorageKey(userId)) || "[]");
    return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
  } catch {
    return [];
  }
}

function relativeTime(value: string) {
  const time = Date.parse(value);
  if (!Number.isFinite(time)) return "Recently";
  const minutes = Math.max(0, Math.round((Date.now() - time) / 60000));
  if (minutes < 1) return "Just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  if (days < 7) return `${days}d ago`;
  return new Date(time).toLocaleDateString(undefined, { month: "short", day: "numeric" });
}
function LanguageSwitch({ language, onChange, compact = false }: { language: PlatformLanguage; onChange: (language: PlatformLanguage) => void; compact?: boolean }) {
  return (
    <div aria-label="Language" className={`cc-language-switch ${compact ? "is-compact" : ""}`} role="group">
      {!compact && <Languages aria-hidden="true" size={16} />}
      <button aria-pressed={language === "en"} className={language === "en" ? "active" : ""} onClick={() => onChange("en")} title="English" type="button">EN</button>
      <button aria-pressed={language === "ur"} className={language === "ur" ? "active" : ""} onClick={() => onChange("ur")} title="Urdu" type="button"><span dir="rtl" lang="ur">اردو</span></button>
    </div>
  );
}
function PortalNavLink({
  item,
  active,
  collapsed,
  onNavigate
}: {
  item: string;
  active: boolean;
  collapsed: boolean;
  onNavigate: (page: string) => void;
}) {
  const Icon = pageIcons[item] || LayoutDashboard;
  const label = pageLabel(item);
  return (
    <a
      aria-current={active ? "page" : undefined}
      aria-label={collapsed ? label : undefined}
      className={`cc-portal-link ${active ? "active" : ""}`}
      href={pathForPage(item)}
      onClick={(event) => {
        if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
        event.preventDefault();
        onNavigate(item);
      }}
      title={collapsed ? label : undefined}
    >
      <Icon aria-hidden="true" size={19} strokeWidth={1.8} />
      <span className="cc-nav-label">{label}</span>
    </a>
  );
}

function pageLabel(page: string) {
  if (page === "Command" || page === "Home") return "Dashboard";
  if (page === "Get Quote") return "Quotations";
  if (page === VERIFY_IDENTITY_PAGE) return "Verify";
  return page;
}

function readSidebarState() {
  return localStorage.getItem("cc_sidebar_collapsed") === "1";
}

function initials(value: string) {
  const letters = value
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() || "")
    .join("");
  return letters || "CC";
}

function greeting(value: string) {
  const hour = new Date().getHours();
  const time = hour < 12 ? "Good morning" : hour < 18 ? "Good afternoon" : "Good evening";
  const firstName = value.trim().split(/\s+/)[0] || "there";
  return `${time}, ${firstName}`;
}

function pageDescription(page: string) {
  const descriptions: Record<string, string> = {
    Command: "What needs you now — quotes, proofs, chats and live floor.",
    Home: "Your balance, orders, quotes and shipping — same command surface as the app.",
    Customers: "Customer accounts, balances and active work.",
    "Users & Access": "Team logins, roles and page permissions.",
    Quotations: "Review requests and prepare customer pricing.",
    "Get Quote": "Request manufacturing pricing for a product.",
    Products: "Manage your catalog, inventory and reserved stock.",
    Orders: "Follow confirmed work through every production stage.",
    Shipping: "Book packages, publish rates and track deliveries.",
    Payments: "Balances, transfer proofs and account history.",
    Directory: "Browse verified Wazirabad shops and makers.",
    "App home": "Customer notices and featured products on the app home.",
    Messages: "Live conversations, photos, files and direct audio calls.",
    Calls: "Incoming, outgoing, answered and missed call history.",
    Email: "Templates, automations and transactional delivery activity.",
    Settings: "Profile, login security and platform preferences.",
    [VERIFY_IDENTITY_PAGE]: "Complete the item below."
  };
  return descriptions[page] || "ChakuChuri operations workspace.";
}

export default PlatformAppNext;


function isTemporarilySuspended(status: string) {
  return status.trim().toLowerCase() === "temporarily suspended";
}

function isCustomerRole(role: string) {
  return role.toLowerCase() === "customer";
}
