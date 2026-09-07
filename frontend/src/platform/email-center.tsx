import { type FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  Activity,
  AlertCircle,
  CalendarClock,
  Check,
  ChevronRight,
  CircleOff,
  Clock3,
  Eye,
  FileText,
  Inbox,
  Mail,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
  Send,
  Settings2,
  ShieldCheck,
  Sparkles,
  TestTube2,
  X,
  Zap,
  type LucideIcon
} from "lucide-react";
import { apiGet, apiPost } from "./api";
import type {
  EmailAutomationRule,
  EmailDelivery,
  EmailPreview,
  EmailSnapshot,
  EmailTemplate,
  Session
} from "./types";
import { messageFromError } from "./utils";

type EmailTab = "overview" | "automations" | "templates" | "deliveries" | "settings";

const tabs: Array<{ id: EmailTab; label: string; icon: LucideIcon }> = [
  { id: "overview", label: "Overview", icon: Activity },
  { id: "automations", label: "Automations", icon: Zap },
  { id: "templates", label: "Templates", icon: FileText },
  { id: "deliveries", label: "Deliveries", icon: Inbox },
  { id: "settings", label: "Settings", icon: Settings2 }
];

export function EmailCenter({
  session,
  onAction
}: {
  session: Session;
  onAction: (message: string) => Promise<void>;
}) {
  const [snapshot, setSnapshot] = useState<EmailSnapshot | null>(null);
  const [tab, setTab] = useState<EmailTab>("overview");
  const [loading, setLoading] = useState(true);
  const [historyBusy, setHistoryBusy] = useState(false);
  const [error, setError] = useState("");
  const canManage = canManageEmail(session);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setSnapshot(normalizeEmailSnapshot(await apiGet<EmailSnapshot>("/api/email", session.token)));
    } catch (loadError) {
      setError(messageFromError(loadError, "Email operations could not be loaded."));
    } finally {
      setLoading(false);
    }
  }, [session.token]);

  useEffect(() => {
    void load();
  }, [load]);

  const accept = async (next: EmailSnapshot, notice: string) => {
    setSnapshot(normalizeEmailSnapshot(next));
    setError("");
    await onAction(notice);
  };

  const loadMoreDeliveries = async () => {
    const page = snapshot?.pagination.deliveries;
    if (!page?.hasMore || historyBusy) return;
    setHistoryBusy(true);
    setError("");
    try {
      const next = normalizeEmailSnapshot(await apiGet<EmailSnapshot>(
        `/api/email?scope=deliveries&offset=${page.loaded}&limit=20`,
        session.token
      ));
      setSnapshot((current) => current ? {
        ...current,
        deliveries: mergeEmailDeliveries(current.deliveries, next.deliveries),
        pagination: { ...current.pagination, deliveries: next.pagination.deliveries }
      } : next);
    } catch (loadError) {
      setError(messageFromError(loadError, "More delivery history could not be loaded."));
    } finally {
      setHistoryBusy(false);
    }
  };

  if (loading && !snapshot) {
    return (
      <section className="cc-email-state" aria-live="polite">
        <RefreshCw className="spin" size={22} />
        <div><strong>Loading email operations</strong><span>Checking templates, automations and delivery activity.</span></div>
      </section>
    );
  }

  if (!snapshot) {
    return (
      <section className="cc-email-state is-error" role="alert">
        <AlertCircle size={22} />
        <div><strong>Email Center is unavailable</strong><span>{error || "Please try again."}</span></div>
        <button className="cc-email-icon-button" onClick={() => void load()} title="Retry" type="button"><RefreshCw size={18} /></button>
      </section>
    );
  }

  return (
    <section className="cc-email-center">
      <div className="cc-email-health">
        <div className={`cc-email-provider-mark ${snapshot.settings.configured ? "is-ready" : ""}`}><Mail size={19} /></div>
        <div className="cc-email-health-copy">
          <strong>{snapshot.settings.mode === "Development" ? "Development mail capture is active" : `${snapshot.settings.provider} is connected`}</strong>
          <span>{snapshot.settings.fromName} &lt;{snapshot.settings.fromEmail}&gt;</span>
        </div>
        <div className="cc-email-health-meta">
          <span className="cc-email-state-pill is-ready"><Check size={13} /> {snapshot.settings.configured ? "Ready" : "Needs setup"}</span>
          <small>{snapshot.metrics.queued + snapshot.metrics.scheduled} awaiting delivery</small>
        </div>
      </div>

      {error && <div className="cc-email-inline-error" role="alert"><AlertCircle size={16} /><span>{error}</span><button aria-label="Dismiss error" onClick={() => setError("")} type="button"><X size={15} /></button></div>}

      <nav className="cc-email-tabs" aria-label="Email Center sections">
        {tabs.map((item) => {
          const Icon = item.icon;
          return (
            <button aria-current={tab === item.id ? "page" : undefined} className={tab === item.id ? "active" : ""} key={item.id} onClick={() => setTab(item.id)} type="button">
              <Icon size={17} /><span>{item.label}</span>
              {item.id === "deliveries" && snapshot.metrics.failed > 0 && <b>{snapshot.metrics.failed}</b>}
            </button>
          );
        })}
      </nav>

      <div className="cc-email-view">
        {tab === "overview" && <EmailOverview onOpen={setTab} snapshot={snapshot} />}
        {tab === "automations" && <AutomationView canManage={canManage} onAccept={accept} onError={setError} rules={snapshot.rules} session={session} />}
        {tab === "templates" && <TemplateView canManage={canManage} onAccept={accept} onError={setError} session={session} snapshot={snapshot} />}
        {tab === "deliveries" && <DeliveryView canManage={canManage} historyBusy={historyBusy} onAccept={accept} onError={setError} onLoadMore={loadMoreDeliveries} session={session} snapshot={snapshot} />}
        {tab === "settings" && <EmailSettingsView canManage={canManage} onAccept={accept} onError={setError} session={session} snapshot={snapshot} />}
      </div>
    </section>
  );
}

function EmailOverview({ snapshot, onOpen }: { snapshot: EmailSnapshot; onOpen: (tab: EmailTab) => void }) {
  const rules = snapshot.rules;
  const deliveries = snapshot.deliveries;
  const outbox = snapshot.outbox;
  const metrics = snapshot.metrics;
  const activeRules = rules.filter((rule) => rule.enabled).length;
  const recent = deliveries.slice(0, 6);
  const scheduled = outbox.filter((item) => item.status === "Queued" || item.status === "Scheduled").slice(0, 5);
  const metricCards = [
    { label: "Captured", value: metrics.captured, detail: "Accepted by current provider", icon: Check, tone: "ready" },
    { label: "Waiting", value: metrics.queued + metrics.scheduled, detail: "Queued or scheduled", icon: CalendarClock, tone: "waiting" },
    { label: "Success rate", value: `${Math.round(metrics.successRate)}%`, detail: "Completed attempts", icon: Activity, tone: "neutral" },
    { label: "Needs attention", value: metrics.failed, detail: "Failed deliveries", icon: AlertCircle, tone: metrics.failed ? "danger" : "neutral" }
  ];

  return (
    <div className="cc-email-overview">
      <div className="cc-email-metrics">
        {metricCards.map((metric) => {
          const Icon = metric.icon;
          return <article className={`is-${metric.tone}`} key={metric.label}><div><span>{metric.label}</span><strong>{metric.value}</strong><small>{metric.detail}</small></div><Icon size={19} /></article>;
        })}
      </div>

      <div className="cc-email-overview-grid">
        <section className="cc-email-section">
          <header><div><span>Delivery stream</span><h2>Recent activity</h2></div><button onClick={() => onOpen("deliveries")} type="button">View all <ChevronRight size={15} /></button></header>
          <div className="cc-email-activity-list">
            {recent.length ? recent.map((delivery) => <DeliveryLine delivery={delivery} key={delivery.id} />) : <EmailEmpty icon={Inbox} title="No deliveries yet" detail="Test sends and automated customer emails will appear here." />}
          </div>
        </section>

        <section className="cc-email-section">
          <header><div><span>Outbox</span><h2>Upcoming email</h2></div><button onClick={() => onOpen("automations")} type="button">{activeRules}/{rules.length} active <ChevronRight size={15} /></button></header>
          <div className="cc-email-schedule-list">
            {scheduled.length ? scheduled.map((item) => (
              <div key={item.id}><CalendarClock size={17} /><div><strong>{labelForTrigger(item.trigger)}</strong><span>{item.recipientName || item.recipient}</span></div><time>{relativeTime(item.scheduledAt)}</time></div>
            )) : <EmailEmpty icon={Sparkles} title="Outbox is clear" detail="Nothing is waiting to be sent." />}
          </div>
        </section>
      </div>

      <section className="cc-email-foundation">
        <div><ShieldCheck size={20} /><span><strong>Transactional safety</strong><small>Business actions finish independently from email delivery.</small></span></div>
        <div><RotateCcw size={20} /><span><strong>Duplicate protection</strong><small>Each workflow event has one idempotency key.</small></span></div>
        <div><Clock3 size={20} /><span><strong>Background delivery</strong><small>Due messages are claimed outside customer requests.</small></span></div>
      </section>
    </div>
  );
}

function AutomationView({
  rules,
  session,
  canManage,
  onAccept,
  onError
}: {
  rules: EmailAutomationRule[];
  session: Session;
  canManage: boolean;
  onAccept: (snapshot: EmailSnapshot, notice: string) => Promise<void>;
  onError: (message: string) => void;
}) {
  const groups = useMemo(() => groupBy(rules, (rule) => rule.category), [rules]);
  return (
    <div className="cc-email-automation-view">
      <div className="cc-email-view-heading"><div><span>Workflow rules</span><h2>Customer email automations</h2><p>Choose which operational events create email and when delivery should begin.</p></div><span className="cc-email-readiness"><Zap size={15} /> {rules.filter((rule) => rule.enabled).length} active</span></div>
      {Object.entries(groups).map(([category, rows]) => (
        <section className="cc-email-rule-group" key={category}>
          <header><strong>{category}</strong><span>{rows.filter((row) => row.enabled).length} of {rows.length} active</span></header>
          {rows.map((rule) => <AutomationRow canManage={canManage} key={rule.id} onAccept={onAccept} onError={onError} rule={rule} session={session} />)}
        </section>
      ))}
    </div>
  );
}

function AutomationRow({
  rule,
  session,
  canManage,
  onAccept,
  onError
}: {
  rule: EmailAutomationRule;
  session: Session;
  canManage: boolean;
  onAccept: (snapshot: EmailSnapshot, notice: string) => Promise<void>;
  onError: (message: string) => void;
}) {
  const [enabled, setEnabled] = useState(rule.enabled);
  const [timing, setTiming] = useState(rule.timing);
  const [delayMinutes, setDelayMinutes] = useState(String(rule.delayMinutes));
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setEnabled(rule.enabled);
    setTiming(rule.timing);
    setDelayMinutes(String(rule.delayMinutes));
  }, [rule.id, rule.enabled, rule.timing, rule.delayMinutes, rule.updatedAt]);

  const dirty = enabled !== rule.enabled || timing !== rule.timing || Number(delayMinutes || 0) !== rule.delayMinutes;

  const save = async () => {
    setSaving(true);
    try {
      const nextDelay = timing === "Immediate" ? 0 : Number(delayMinutes) || 0;
      const next = await apiPost<EmailSnapshot>(`/api/email/automations/${encodeURIComponent(rule.id)}`, {
        enabled,
        timing,
        delayMinutes: nextDelay,
        maxReminders: rule.maxReminders
      }, session.token);
      setDelayMinutes(String(nextDelay));
      await onAccept(next, `${rule.label} automation updated.`);
    } catch (error) {
      onError(messageFromError(error, "Automation could not be saved."));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className={`cc-email-rule-row ${enabled ? "is-enabled" : ""}`}>
      <label className="cc-email-toggle" title={enabled ? "Disable automation" : "Enable automation"}>
        <input checked={enabled} disabled={!canManage} onChange={(event) => setEnabled(event.target.checked)} type="checkbox" />
        <i aria-hidden="true" />
      </label>
      <div className="cc-email-rule-copy"><strong>{rule.label}</strong><span>{triggerDescription(rule.trigger)}</span><small>{rule.essential ? "Essential transaction notice" : "Optional notification"}</small></div>
      <label className="cc-email-control"><span>Timing</span><select disabled={!canManage || !enabled} onChange={(event) => setTiming(event.target.value)} value={timing}><option>Immediate</option><option>Delayed</option></select></label>
      <label className="cc-email-control is-delay"><span>Delay</span><div><input disabled={!canManage || !enabled || timing === "Immediate"} min="0" onChange={(event) => setDelayMinutes(event.target.value)} type="number" value={timing === "Immediate" ? "0" : delayMinutes} /><em>min</em></div></label>
      <button className="cc-email-icon-button" disabled={!canManage || !dirty || saving} onClick={() => void save()} title="Save automation" type="button">{saving ? <RefreshCw className="spin" size={17} /> : <Save size={17} />}</button>
    </div>
  );
}

function TemplateView({
  snapshot,
  session,
  canManage,
  onAccept,
  onError
}: {
  snapshot: EmailSnapshot;
  session: Session;
  canManage: boolean;
  onAccept: (snapshot: EmailSnapshot, notice: string) => Promise<void>;
  onError: (message: string) => void;
}) {
  const [selectedId, setSelectedId] = useState(snapshot.templates[0]?.id || "");
  const selected = snapshot.templates.find((template) => template.id === selectedId) || snapshot.templates[0];

  useEffect(() => {
    if (selectedId && snapshot.templates.some((template) => template.id === selectedId)) return;
    setSelectedId(snapshot.templates[0]?.id || "");
  }, [selectedId, snapshot.templates]);

  if (!selected) return <EmailEmpty icon={FileText} title="No templates" detail="Email templates will appear after the foundation loads." />;
  return (
    <div className="cc-email-template-shell">
      <aside className="cc-email-template-list">
        <header><span>Message library</span><strong>{snapshot.templates.length} templates</strong></header>
        {snapshot.templates.map((template) => (
          <button className={template.id === selected.id ? "active" : ""} key={template.id} onClick={() => setSelectedId(template.id)} type="button">
            <span className={`cc-email-language is-${template.language}`}>{template.language.toUpperCase()}</span>
            <span><strong>{template.name}</strong><small>{template.category} / v{template.version}</small></span>
            <ChevronRight size={15} />
          </button>
        ))}
      </aside>
      <TemplateEditor canManage={canManage} key={`${selected.id}:${selected.version}`} onAccept={onAccept} onError={onError} session={session} template={selected} />
    </div>
  );
}

function TemplateEditor({
  template,
  session,
  canManage,
  onAccept,
  onError
}: {
  template: EmailTemplate;
  session: Session;
  canManage: boolean;
  onAccept: (snapshot: EmailSnapshot, notice: string) => Promise<void>;
  onError: (message: string) => void;
}) {
  const [subject, setSubject] = useState(template.subject);
  const [body, setBody] = useState(template.body);
  const [enabled, setEnabled] = useState(template.enabled);
  const [preview, setPreview] = useState<EmailPreview | null>(null);
  const [testRecipient, setTestRecipient] = useState(session.user.email);
  const [working, setWorking] = useState<"save" | "preview" | "test" | "">("");
  const dirty = subject !== template.subject || body !== template.body || enabled !== template.enabled;

  const showPreview = useCallback(async (useDraft = true) => {
    setWorking("preview");
    try {
      setPreview(await apiPost<EmailPreview>("/api/email/preview", {
        templateKey: template.key,
        language: template.language,
        subject: useDraft ? subject : template.subject,
        body: useDraft ? body : template.body,
        context: {}
      }, session.token));
    } catch (error) {
      onError(messageFromError(error, "Preview could not be created."));
    } finally {
      setWorking("");
    }
  }, [body, onError, session.token, subject, template.body, template.key, template.language, template.subject]);

  useEffect(() => {
    void showPreview(true);
    // Only re-run when the saved template identity changes, not on every keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [template.id, template.version, session.token]);

  const save = async () => {
    setWorking("save");
    try {
      const next = await apiPost<EmailSnapshot>(`/api/email/templates/${encodeURIComponent(template.id)}`, { subject, body, enabled }, session.token);
      await onAccept(next, `${template.name} template saved as a new version.`);
    } catch (error) {
      onError(messageFromError(error, "Template could not be saved."));
    } finally {
      setWorking("");
    }
  };

  const sendTest = async (event: FormEvent) => {
    event.preventDefault();
    setWorking("test");
    try {
      const next = await apiPost<EmailSnapshot>("/api/email/test", { recipient: testRecipient, templateKey: template.key, language: template.language }, session.token);
      await onAccept(next, `Test email captured for ${testRecipient}.`);
    } catch (error) {
      onError(messageFromError(error, "Test email could not be sent."));
    } finally {
      setWorking("");
    }
  };

  return (
    <div className="cc-email-template-editor">
      <header><div><span>{template.category} / {template.language.toUpperCase()}</span><h2>{template.name}</h2><small>Version {template.version} / Updated {formatDateTime(template.updatedAt)}</small></div><label className="cc-email-toggle with-label"><span>{enabled ? "Enabled" : "Disabled"}</span><input checked={enabled} disabled={!canManage} onChange={(event) => setEnabled(event.target.checked)} type="checkbox" /><i /></label></header>
      <div className="cc-email-editor-grid">
        <div className="cc-email-editor-fields">
          <label><span>Subject</span><input disabled={!canManage} maxLength={180} onChange={(event) => setSubject(event.target.value)} value={subject} /></label>
          <label><span>Message content</span><textarea disabled={!canManage} onChange={(event) => setBody(event.target.value)} rows={10} value={body} /></label>
          <p className="cc-email-design-note">{"Customers receive a designed HTML email. Use short paragraphs, Label: value fact lines, and an Open {{portal_url}}/... line for the button."}</p>
          <div className="cc-email-variable-list"><span>Variables</span><div>{(template.variables || []).map((variable) => <code key={variable}>{`{{${variable}}}`}</code>)}</div></div>
        </div>
        <aside className="cc-email-preview-pane">
          <header><span>Inbox preview</span><button className="cc-email-icon-button" disabled={working !== ""} onClick={() => void showPreview(true)} title="Refresh preview" type="button">{working === "preview" ? <RefreshCw className="spin" size={17} /> : <Eye size={17} />}</button></header>
          {preview ? (
            <div className="cc-email-message-preview is-rich">
              <small>SUBJECT</small>
              <strong>{preview.subject}</strong>
              {preview.htmlBody ? (
                <iframe className="cc-email-html-frame" sandbox="" srcDoc={preview.htmlBody} title={`${template.name} preview`} />
              ) : (
                <pre>{preview.body}</pre>
              )}
              {(preview.missingVariables || []).length > 0 && <span className="is-warning">Missing: {preview.missingVariables.join(", ")}</span>}
            </div>
          ) : (
            <EmailEmpty icon={Eye} title="Preview template" detail="Render this message with safe sample customer and order data." />
          )}
          <form className="cc-email-test-form" onSubmit={sendTest}><label><span>Test recipient</span><input disabled={!canManage} onChange={(event) => setTestRecipient(event.target.value)} required type="email" value={testRecipient} /></label><button disabled={!canManage || working !== ""} type="submit"><TestTube2 size={16} /> {working === "test" ? "Capturing..." : "Send test"}</button></form>
        </aside>
      </div>
      <footer><span>{dirty ? "Unsaved changes" : `Saved version ${template.version}`}</span><button disabled={!canManage || !dirty || working !== ""} onClick={() => void save()} type="button">{working === "save" ? <RefreshCw className="spin" size={16} /> : <Save size={16} />} Save new version</button></footer>
    </div>
  );
}

function DeliveryView({
  snapshot,
  session,
  canManage,
  historyBusy,
  onAccept,
  onError,
  onLoadMore
}: {
  snapshot: EmailSnapshot;
  session: Session;
  canManage: boolean;
  historyBusy: boolean;
  onAccept: (snapshot: EmailSnapshot, notice: string) => Promise<void>;
  onError: (message: string) => void;
  onLoadMore: () => Promise<void>;
}) {
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("All");
  const [working, setWorking] = useState("");
  const statuses = ["All", "Captured", "Sending", "Queued", "Scheduled", "Failed", "Cancelled"];
  const deliveries = snapshot.deliveries || [];
  const visible = deliveries.filter((delivery) => {
    const matchesStatus = status === "All" || delivery.status === status;
    const haystack = `${delivery.recipient} ${delivery.recipientName} ${delivery.subject} ${delivery.entityId} ${delivery.trigger}`.toLowerCase();
    return matchesStatus && haystack.includes(query.trim().toLowerCase());
  });

  const act = async (delivery: EmailDelivery, action: "retry" | "cancel") => {
    setWorking(delivery.id);
    try {
      const path = action === "retry" ? `/api/email/deliveries/${encodeURIComponent(delivery.id)}/retry` : `/api/email/outbox/${encodeURIComponent(delivery.outboxId)}/cancel`;
      const next = await apiPost<EmailSnapshot>(path, {}, session.token);
      await onAccept(next, action === "retry" ? "Delivery queued for retry." : "Scheduled email cancelled.");
    } catch (error) {
      onError(messageFromError(error, "Delivery action failed."));
    } finally {
      setWorking("");
    }
  };

  return (
    <div className="cc-email-delivery-view">
      <div className="cc-email-view-heading"><div><span>Message trail</span><h2>Delivery history</h2><p>Search each attempt by customer, email address, subject, event or record ID.</p></div><span className="cc-email-readiness"><Inbox size={15} /> {snapshot.pagination.deliveries?.total || deliveries.length} records</span></div>
      <div className="cc-email-delivery-tools"><label><Search size={17} /><input onChange={(event) => setQuery(event.target.value)} placeholder="Search deliveries" value={query} /></label><div>{statuses.map((item) => <button className={status === item ? "active" : ""} key={item} onClick={() => setStatus(item)} type="button">{item}</button>)}</div></div>
      <div className="cc-email-delivery-table" role="table" aria-label="Email delivery history">
        <div className="cc-email-delivery-head" role="row"><span>Recipient</span><span>Message</span><span>Status</span><span>Activity</span><span aria-label="Actions" /></div>
        {visible.length ? visible.map((delivery) => (
          <div className="cc-email-delivery-row" key={delivery.id} role="row">
            <div><strong>{delivery.recipientName || delivery.recipient}</strong><span>{delivery.recipient}</span></div>
            <div><strong>{delivery.subject}</strong><span>{labelForTrigger(delivery.trigger)} / {delivery.entityId || "Test"}</span></div>
            <span className={`cc-email-state-pill is-${statusClass(delivery.status)}`}>{statusIcon(delivery.status)} {delivery.status}</span>
            <div><strong>{formatDateTime(delivery.sentAt || delivery.createdAt)}</strong><span>{delivery.provider} / {delivery.attempts} attempt{delivery.attempts === 1 ? "" : "s"}</span></div>
            <div className="cc-email-row-actions">{(delivery.status === "Failed" || delivery.status === "Sending") && <button disabled={!canManage || working === delivery.id} onClick={() => void act(delivery, "retry")} title="Retry delivery" type="button"><RotateCcw size={16} /></button>}{(delivery.status === "Queued" || delivery.status === "Scheduled") && <button disabled={!canManage || working === delivery.id} onClick={() => void act(delivery, "cancel")} title="Cancel scheduled email" type="button"><CircleOff size={16} /></button>}</div>
            {delivery.lastError && <p>{delivery.lastError}</p>}
          </div>
        )) : <EmailEmpty icon={Search} title="No matching deliveries" detail="Try a different search or status filter." />}
      </div>
      {snapshot.pagination.deliveries?.hasMore && (
        <div className="cc-pagination-more">
          <button className="cc-button" disabled={historyBusy} onClick={() => void onLoadMore()} type="button">
            {historyBusy ? <RefreshCw className="spin" size={15} /> : null} See more
          </button>
          <small>{snapshot.pagination.deliveries.total - snapshot.pagination.deliveries.loaded} remaining</small>
        </div>
      )}
    </div>
  );
}

function EmailSettingsView({
  snapshot,
  session,
  canManage,
  onAccept,
  onError
}: {
  snapshot: EmailSnapshot;
  session: Session;
  canManage: boolean;
  onAccept: (snapshot: EmailSnapshot, notice: string) => Promise<void>;
  onError: (message: string) => void;
}) {
  const [fromName, setFromName] = useState(snapshot.settings.fromName);
  const [fromEmail, setFromEmail] = useState(snapshot.settings.fromEmail);
  const [replyTo, setReplyTo] = useState(snapshot.settings.replyTo);
  const [provider, setProvider] = useState(snapshot.settings.provider === "smtp" ? "smtp" : "development");
  const [smtpHost, setSmtpHost] = useState(snapshot.settings.smtpHost || "");
  const [smtpPort, setSmtpPort] = useState(String(snapshot.settings.smtpPort || 587));
  const [smtpUsername, setSmtpUsername] = useState(snapshot.settings.smtpUsername || "");
  const [smtpPassword, setSmtpPassword] = useState("");
  const [smtpImplicitTls, setSmtpImplicitTls] = useState(Boolean(snapshot.settings.smtpImplicitTls || snapshot.settings.smtpPort === 465));
  const [clearPassword, setClearPassword] = useState(false);
  const [working, setWorking] = useState<"save" | "verify" | "">("");

  useEffect(() => {
    setFromName(snapshot.settings.fromName);
    setFromEmail(snapshot.settings.fromEmail);
    setReplyTo(snapshot.settings.replyTo);
    setProvider(snapshot.settings.provider === "smtp" ? "smtp" : "development");
    setSmtpHost(snapshot.settings.smtpHost || "");
    setSmtpPort(String(snapshot.settings.smtpPort || 587));
    setSmtpUsername(snapshot.settings.smtpUsername || "");
    setSmtpImplicitTls(Boolean(snapshot.settings.smtpImplicitTls || snapshot.settings.smtpPort === 465));
    setSmtpPassword("");
    setClearPassword(false);
  }, [snapshot.settings]);

  const dirty =
    fromName !== snapshot.settings.fromName ||
    fromEmail !== snapshot.settings.fromEmail ||
    replyTo !== snapshot.settings.replyTo ||
    provider !== (snapshot.settings.provider === "smtp" ? "smtp" : "development") ||
    smtpHost !== (snapshot.settings.smtpHost || "") ||
    Number(smtpPort || 0) !== (snapshot.settings.smtpPort || 587) ||
    smtpUsername !== (snapshot.settings.smtpUsername || "") ||
    smtpImplicitTls !== Boolean(snapshot.settings.smtpImplicitTls || snapshot.settings.smtpPort === 465) ||
    smtpPassword !== "" ||
    clearPassword;

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setWorking("save");
    try {
      const next = await apiPost<EmailSnapshot>("/api/email/settings", {
        fromName,
        fromEmail,
        replyTo,
        provider,
        smtpHost,
        smtpPort: Number(smtpPort) || 587,
        smtpUsername,
        smtpPassword: clearPassword ? "" : smtpPassword,
        smtpImplicitTls: smtpImplicitTls || Number(smtpPort) === 465,
        clearSmtpPassword: clearPassword
      }, session.token);
      setSmtpPassword("");
      setClearPassword(false);
      await onAccept(next, provider === "smtp" ? "SMTP connection settings saved." : "Email sender settings saved.");
    } catch (error) {
      onError(messageFromError(error, "Sender settings could not be saved."));
    } finally {
      setWorking("");
    }
  };

  const verify = async () => {
    setWorking("verify");
    try {
      const next = await apiPost<EmailSnapshot>("/api/email/settings/verify", {}, session.token);
      await onAccept(next, provider === "smtp" ? "SMTP connection verified." : "Development sink verified.");
    } catch (error) {
      onError(messageFromError(error, "Email provider could not be verified."));
    } finally {
      setWorking("");
    }
  };

  return (
    <div className="cc-email-settings-view">
      <div className="cc-email-view-heading">
        <div>
          <span>Delivery configuration</span>
          <h2>Sender and connection</h2>
          <p>Set the public from-address and choose Development sink or live SMTP delivery.</p>
        </div>
        <button className="cc-email-verify-button" disabled={!canManage || working !== "" || dirty} onClick={() => void verify()} type="button">
          {working === "verify" ? <RefreshCw className="spin" size={16} /> : <ShieldCheck size={16} />} Verify connection
        </button>
      </div>
      <div className="cc-email-settings-grid">
        <form className="cc-email-sender-form" onSubmit={save}>
          <header><Mail size={19} /><div><strong>Sender identity</strong><span>Shown in every customer inbox</span></div></header>
          <label><span>From name</span><input disabled={!canManage} onChange={(event) => setFromName(event.target.value)} required value={fromName} /></label>
          <label><span>From email</span><input disabled={!canManage} onChange={(event) => setFromEmail(event.target.value)} required type="email" value={fromEmail} /></label>
          <label><span>Reply-to email</span><input disabled={!canManage} onChange={(event) => setReplyTo(event.target.value)} required type="email" value={replyTo} /></label>
          <label>
            <span>Delivery mode</span>
            <select disabled={!canManage} onChange={(event) => setProvider(event.target.value)} value={provider}>
              <option value="development">Development sink (local capture)</option>
              <option value="smtp">SMTP (real inbox delivery)</option>
            </select>
          </label>
          {provider === "smtp" && (
            <div className="cc-email-smtp-fields">
              <label><span>SMTP host</span><input disabled={!canManage} onChange={(event) => setSmtpHost(event.target.value)} placeholder="smtp.gmail.com" required value={smtpHost} /></label>
              <label><span>SMTP port</span><input disabled={!canManage} min={1} max={65535} onChange={(event) => {
                const next = event.target.value;
                setSmtpPort(next);
                if (Number(next) === 465) setSmtpImplicitTls(true);
              }} required type="number" value={smtpPort} /></label>
              <label><span>Username</span><input disabled={!canManage} onChange={(event) => setSmtpUsername(event.target.value)} placeholder="usually your email" value={smtpUsername} /></label>
              <label>
                <span>Password {snapshot.settings.passwordConfigured ? <b>Saved — leave blank to keep</b> : <b>Required for most hosts</b>}</span>
                <input autoComplete="new-password" disabled={!canManage || clearPassword} onChange={(event) => setSmtpPassword(event.target.value)} placeholder={snapshot.settings.passwordConfigured ? "••••••••" : "App password or SMTP password"} type="password" value={smtpPassword} />
              </label>
              <label className="cc-email-check-row">
                <input checked={smtpImplicitTls} disabled={!canManage} onChange={(event) => setSmtpImplicitTls(event.target.checked)} type="checkbox" />
                <span>Implicit TLS (port 465)</span>
              </label>
              {snapshot.settings.passwordConfigured && (
                <label className="cc-email-check-row">
                  <input checked={clearPassword} disabled={!canManage} onChange={(event) => {
                    setClearPassword(event.target.checked);
                    if (event.target.checked) setSmtpPassword("");
                  }} type="checkbox" />
                  <span>Clear saved password</span>
                </label>
              )}
            </div>
          )}
          <footer>
            <span>{dirty ? "Unsaved changes" : `Last saved ${formatDateTime(snapshot.settings.updatedAt)}`}</span>
            <button disabled={!canManage || !dirty || working !== ""} type="submit">{working === "save" ? <RefreshCw className="spin" size={16} /> : <Save size={16} />} Save</button>
          </footer>
        </form>
        <section className="cc-email-provider-details">
          <header><Settings2 size={19} /><div><strong>Provider status</strong><span>{snapshot.settings.mode}</span></div></header>
          <dl>
            <div><dt>Provider</dt><dd>{snapshot.settings.provider === "smtp" ? "SMTP" : "Development sink"}</dd></div>
            {snapshot.settings.provider === "smtp" && <div><dt>Host</dt><dd>{snapshot.settings.smtpHost || "—"}:{snapshot.settings.smtpPort || "—"}</dd></div>}
            {snapshot.settings.provider === "smtp" && <div><dt>Username</dt><dd>{snapshot.settings.smtpUsername || "—"}</dd></div>}
            {snapshot.settings.provider === "smtp" && <div><dt>Password</dt><dd>{snapshot.settings.passwordConfigured ? "Saved on server" : "Not set"}</dd></div>}
            <div><dt>Domain</dt><dd>{snapshot.settings.domainStatus}</dd></div>
            <div><dt>Webhooks</dt><dd>{snapshot.settings.webhookStatus}</dd></div>
            <div><dt>Last verified</dt><dd>{snapshot.settings.lastVerifiedAt ? formatDateTime(snapshot.settings.lastVerifiedAt) : "Not verified yet"}</dd></div>
          </dl>
          <div className="cc-email-production-note">
            <ShieldCheck size={18} />
            <div>
              <strong>{snapshot.settings.provider === "smtp" ? "SMTP delivery active" : "Development sink active"}</strong>
              <span>
                {snapshot.settings.provider === "smtp"
                  ? "Save settings, then Verify connection. Password stays on the server and is never shown again in the browser."
                  : "Emails are captured in Delivery history only. Switch to SMTP above to send to real inboxes."}
              </span>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}

function DeliveryLine({ delivery }: { delivery: EmailDelivery }) {
  return <div><span className={`cc-email-delivery-icon is-${statusClass(delivery.status)}`}>{statusIcon(delivery.status)}</span><div><strong>{delivery.subject}</strong><span>{delivery.recipientName || delivery.recipient}</span></div><span className={`cc-email-state-pill is-${statusClass(delivery.status)}`}>{delivery.status}</span><time>{relativeTime(delivery.sentAt || delivery.createdAt)}</time></div>;
}

function EmailEmpty({ icon: Icon, title, detail }: { icon: LucideIcon; title: string; detail: string }) {
  return <div className="cc-email-empty"><Icon size={20} /><div><strong>{title}</strong><span>{detail}</span></div></div>;
}

function statusIcon(status: string) {
  if (status === "Captured" || status === "Delivered" || status === "Sent") return <Check size={13} />;
  if (status === "Failed") return <AlertCircle size={13} />;
  if (status === "Cancelled") return <CircleOff size={13} />;
  return <Clock3 size={13} />;
}

function statusClass(status: string) {
  switch (status.toLowerCase()) {
    case "captured": case "delivered": case "sent": return "ready";
    case "failed": return "danger";
    case "cancelled": return "muted";
    default: return "waiting";
  }
}

function labelForTrigger(trigger: string) {
  const labels: Record<string, string> = {
    "quote.priced": "Quotation ready",
    "quote.accepted": "Quotation accepted",
    "order.amended": "Order amended",
    "order.ready": "Order ready",
    "order.cancelled": "Order cancelled",
    "payment.due": "Payment reminder",
    "payment.confirmed": "Payment confirmed",
    "shipment.dispatched": "Shipment dispatched",
    "test.send": "Test email"
  };
  return labels[trigger] || trigger.replace(/\./g, " ");
}

function triggerDescription(trigger: string) {
  const descriptions: Record<string, string> = {
    "quote.priced": "Send when formal per-unit pricing is published.",
    "quote.accepted": "Confirm the new manufacturing order.",
    "order.amended": "Explain verified changes to an accepted order.",
    "order.ready": "Tell the customer production is ready for delivery.",
    "order.cancelled": "Confirm cancellation and accounting reversal.",
    "payment.due": "Remind the customer about an open order balance.",
    "payment.confirmed": "Acknowledge a confirmed transfer and ledger credit.",
    "shipment.dispatched": "Share courier and tracking details."
  };
  return descriptions[trigger] || "Send a transactional customer notice.";
}

function formatDateTime(value?: string) {
  if (!value) return "Not yet";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString("en-PK", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

function relativeTime(value?: string) {
  if (!value) return "Not yet";
  const parsed = new Date(value).getTime();
  if (Number.isNaN(parsed)) return value;
  const seconds = Math.round((parsed - Date.now()) / 1000);
  const formatter = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
  if (Math.abs(seconds) < 60) return formatter.format(seconds, "second");
  const minutes = Math.round(seconds / 60);
  if (Math.abs(minutes) < 60) return formatter.format(minutes, "minute");
  const hours = Math.round(minutes / 60);
  if (Math.abs(hours) < 24) return formatter.format(hours, "hour");
  return formatter.format(Math.round(hours / 24), "day");
}

function groupBy<T>(values: T[] | null | undefined, key: (value: T) => string) {
  return (values || []).reduce<Record<string, T[]>>((groups, value) => {
    const group = key(value);
    groups[group] = [...(groups[group] || []), value];
    return groups;
  }, {});
}

function normalizeEmailSnapshot(raw: EmailSnapshot | null | undefined): EmailSnapshot {
  const metrics = raw?.metrics;
  return {
    settings: {
      fromName: raw?.settings?.fromName || "ChakuChuri",
      fromEmail: raw?.settings?.fromEmail || "noreply@chakuchuri.local",
      replyTo: raw?.settings?.replyTo || raw?.settings?.fromEmail || "noreply@chakuchuri.local",
      provider: raw?.settings?.provider || "development",
      mode: raw?.settings?.mode || "Development",
      configured: Boolean(raw?.settings?.configured),
      domainStatus: raw?.settings?.domainStatus || "Local development",
      webhookStatus: raw?.settings?.webhookStatus || "Not required",
      lastVerifiedAt: raw?.settings?.lastVerifiedAt,
      updatedAt: raw?.settings?.updatedAt || "",
      updatedBy: raw?.settings?.updatedBy || ""
    },
    templates: Array.isArray(raw?.templates)
      ? raw!.templates.map((template) => ({
          ...template,
          variables: Array.isArray(template.variables) ? template.variables : []
        }))
      : [],
    rules: Array.isArray(raw?.rules) ? raw!.rules : [],
    outbox: Array.isArray(raw?.outbox) ? raw!.outbox : [],
    deliveries: Array.isArray(raw?.deliveries) ? raw!.deliveries : [],
    pagination: raw?.pagination || {},
    metrics: {
      queued: metrics?.queued || 0,
      scheduled: metrics?.scheduled || 0,
      captured: metrics?.captured || 0,
      failed: metrics?.failed || 0,
      cancelled: metrics?.cancelled || 0,
      successRate: metrics?.successRate || 0,
      lastActivity: metrics?.lastActivity
    }
  };
}

function mergeEmailDeliveries(current: EmailDelivery[], incoming: EmailDelivery[]) {
  const ids = new Set(current.map((delivery) => delivery.id));
  return [...current, ...incoming.filter((delivery) => !ids.has(delivery.id))];
}

function canManageEmail(session: Session) {
  const role = session.user.role.toLowerCase();
  return role === "owner" || role === "admin" || (session.user.permissions || []).includes("email.manage");
}
