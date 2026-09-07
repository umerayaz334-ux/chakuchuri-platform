import { useEffect, useMemo, useState, type ChangeEvent } from "react";
import {
  BadgeCheck,
  CheckCircle2,
  Copy,
  ExternalLink,
  FileUp,
  Link2,
  RefreshCw,
  ShieldAlert,
  ShieldCheck,
  Trash2,
  X
} from "lucide-react";
import { apiBlob, apiPost, apiUploadCustomerDocument } from "./api";
import type { Customer, CustomerDocument, Session } from "./types";
import { dateTime, messageFromError } from "./utils";

export const KYC_SLOTS = [
  { kind: "cnic_front", label: "CNIC front" },
  { kind: "cnic_back", label: "CNIC back" },
  { kind: "selfie", label: "Selfie" }
] as const;

export const VERIFY_IDENTITY_PAGE = "Verify identity";

export function canReviewCustomerKYC(session: Session) {
  const role = (session.user.role || "").toLowerCase();
  return role === "owner" || role === "admin" || (session.user.permissions || []).includes("kyc.review");
}

export function verificationLabel(status?: string) {
  return (status || "").toLowerCase() === "verified" ? "Verified" : "Unverified";
}

export function isCustomerVerified(status?: string) {
  return (status || "").toLowerCase() === "verified";
}

function docStatus(doc?: CustomerDocument) {
  return (doc?.status || "").toLowerCase();
}

function isEditableDoc(doc?: CustomerDocument) {
  const status = docStatus(doc);
  return !doc || status === "draft" || status === "rejected" || status === "requested";
}

function isLockedDoc(doc?: CustomerDocument) {
  const status = docStatus(doc);
  return status === "submitted" || status === "accepted" || status === "uploaded";
}

export type CustomerVerificationStage = "not_started" | "action_required" | "in_review" | "ready_to_verify" | "verified";

const inactiveAskStatuses = new Set(["withdrawn", "cancelled", "canceled"]);

function isActiveAskStatus(status?: string) {
  return !inactiveAskStatuses.has((status || "").trim().toLowerCase());
}

function activeAsksFor(customer?: Customer | null) {
  return (customer?.requestedDocuments || []).filter((ask) => isActiveAskStatus(ask.status));
}

function workflowDocumentsFor(customer?: Customer | null) {
  const activeIDs = new Set(activeAsksFor(customer).map((ask) => ask.id));
  return (customer?.documents || []).filter((doc) => !doc.requestId || activeIDs.has(doc.requestId));
}

export function customerVerificationStage(customer?: Customer | null): CustomerVerificationStage {
  if (!customer) return "not_started";
  if (isCustomerVerified(customer.verificationStatus)) return "verified";

  const reported = (customer.verificationStage || "").trim().toLowerCase();
  if (["not_started", "action_required", "in_review", "ready_to_verify", "verified"].includes(reported)) {
    return reported as CustomerVerificationStage;
  }

  const documents = workflowDocumentsFor(customer);
  const asks = activeAsksFor(customer);
  const byRequest = new Map(documents.filter((doc) => doc.requestId).map((doc) => [doc.requestId || "", doc]));
  if (documents.some((doc) => ["draft", "rejected", "requested"].includes(docStatus(doc)))) return "action_required";
  if (asks.some((ask) => {
    const doc = byRequest.get(ask.id);
    return !doc || ["draft", "rejected", "requested"].includes(docStatus(doc));
  })) return "action_required";
  if (customer.verificationInviteOpen) return "action_required";
  if (documents.some((doc) => ["submitted", "uploaded"].includes(docStatus(doc)))) return "in_review";

  const byKind = new Map(documents.filter((doc) => doc.kind !== "other").map((doc) => [doc.kind, doc]));
  const requiredAccepted = KYC_SLOTS.every((slot) => docStatus(byKind.get(slot.kind)) === "accepted");
  const requestsAccepted = asks.every((ask) => docStatus(byRequest.get(ask.id)) === "accepted");
  if (requiredAccepted && requestsAccepted) return "ready_to_verify";
  if (customer.documentsSubmittedAt || documents.some((doc) => docStatus(doc) === "accepted")) return "in_review";
  return "not_started";
}

export function customerNeedsIdentityAction(customer?: Customer | null) {
  return customerVerificationStage(customer) === "action_required";
}

export function customerCanStartVerification(customer?: Customer | null) {
  const stage = customerVerificationStage(customer);
  return stage === "not_started" || stage === "action_required";
}

export function customerIdentityReviewPending(customer?: Customer | null) {
  const stage = customerVerificationStage(customer);
  return stage === "in_review" || stage === "ready_to_verify";
}

export function VerificationBadge({
  status,
  size = "md",
  compact = false
}: {
  status?: string;
  size?: "sm" | "md";
  compact?: boolean;
}) {
  const normalized = (status || "unverified").toLowerCase() === "verified" ? "verified" : "unverified";
  if (compact && normalized !== "verified") return null;
  const Icon = normalized === "verified" ? BadgeCheck : ShieldCheck;
  return (
    <span className={`cc-verify-badge ${normalized} size-${size}`} title={verificationLabel(normalized)}>
      <Icon aria-hidden="true" size={size === "sm" ? 12 : 14} strokeWidth={2.4} />
      <em>{verificationLabel(normalized)}</em>
    </span>
  );
}

export function VerifiedAvatarMark({ status }: { status?: string }) {
  if (!isCustomerVerified(status)) return null;
  return (
    <i aria-label="Verified account" className="cc-verified-mark" title="Verified account">
      <BadgeCheck size={12} strokeWidth={2.6} />
    </i>
  );
}

export function CustomerDocumentsPanel({
  customer,
  session,
  mode,
  onUpdated,
  onAction
}: {
  customer: Customer;
  session: Session;
  mode: "admin" | "customer";
  onUpdated: (customer: Customer) => void;
  onAction: (message: string) => Promise<void>;
}) {
  const admin = mode === "admin";
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [note, setNote] = useState(customer.identityNote || "");
  const [cnic, setCnic] = useState(customer.cnic || "");
  const [otherLabel, setOtherLabel] = useState("");
  const [askLabel, setAskLabel] = useState("");
  const [linkUrl, setLinkUrl] = useState("");
  const documents = customer.documents || [];
  const allAsks = customer.requestedDocuments || [];
  const activeAsks = allAsks.filter((ask) => isActiveAskStatus(ask.status));
  const activeAskIDs = new Set(activeAsks.map((ask) => ask.id));
  const workflowDocuments = documents.filter((doc) => !doc.requestId || activeAskIDs.has(doc.requestId));
  const openAsks = activeAsks.filter((ask) => ask.status.toLowerCase() === "open");

  useEffect(() => {
    setNote(customer.identityNote || "");
    setCnic(customer.cnic || "");
  }, [customer.cnic, customer.identityNote, customer.id]);

  const byKind = useMemo(() => {
    const map = new Map<string, CustomerDocument>();
    documents.forEach((doc) => {
      if (doc.kind !== "other") map.set(doc.kind, doc);
    });
    return map;
  }, [documents]);
  const extras = documents.filter((doc) => doc.kind === "other" && !doc.requestId);
  const askDocs = useMemo(() => {
    const byRequest = new Map<string, CustomerDocument>();
    documents.forEach((doc) => {
      if (doc.requestId) byRequest.set(doc.requestId, doc);
    });
    return byRequest;
  }, [documents]);

  const slotNeedsAction = (doc?: CustomerDocument) => {
    if (!doc) return true;
    const status = docStatus(doc);
    return status === "draft" || status === "rejected" || status === "requested";
  };

  const customerSlots = KYC_SLOTS.filter((slot) => admin || slotNeedsAction(byKind.get(slot.kind)));

  const requiredReady = KYC_SLOTS.every((slot) => {
    const doc = byKind.get(slot.kind);
    if (!doc) return false;
    const status = docStatus(doc);
    if (status === "accepted" || status === "submitted" || status === "uploaded") return true;
    if (status === "rejected") return false;
    return status === "draft" && Boolean(doc.fileId);
  });
  const asksReady = openAsks.every((ask) => {
    const doc = askDocs.get(ask.id);
    const status = docStatus(doc);
    if (status === "accepted" || status === "submitted" || status === "uploaded") return true;
    return Boolean(doc?.fileId) && status === "draft";
  });
  const hasDraftChanges = workflowDocuments.some((doc) => docStatus(doc) === "draft");
  const canSubmit = requiredReady && asksReady && hasDraftChanges;

  const upload = async (kind: string, file: File, label = "", requestId = "") => {
    setBusy(requestId || kind);
    setError("");
    try {
      const payload = await apiUploadCustomerDocument(
        file,
        { customerId: customer.id, kind, label, requestId },
        session.token
      );
      onUpdated(payload.customer);
      await onAction(`${label || kind} added. Replace anytime before Submit.`);
      if (kind === "other" && !requestId) setOtherLabel("");
    } catch (caught) {
      setError(messageFromError(caught, "Document could not be uploaded."));
    } finally {
      setBusy("");
    }
  };

  const onPick = (kind: string, event: ChangeEvent<HTMLInputElement>, label = "", requestId = "") => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (file) void upload(kind, file, label, requestId);
  };

  const removeDoc = async (docId: string) => {
    setBusy(docId);
    setError("");
    try {
      const payload = await apiPost<{ customer: Customer }>(
        `/api/customers/${encodeURIComponent(customer.id)}/documents/${encodeURIComponent(docId)}/delete`,
        {},
        session.token
      );
      onUpdated(payload.customer);
      await onAction("Document removed.");
    } catch (caught) {
      setError(messageFromError(caught, "Document could not be removed."));
    } finally {
      setBusy("");
    }
  };

  const reviewDoc = async (docId: string, status: "accepted" | "rejected") => {
    if (!admin) return;
    setBusy(`${docId}-${status}`);
    setError("");
    try {
      const payload = await apiPost<{ customer: Customer }>(
        `/api/customers/${encodeURIComponent(customer.id)}/documents/${encodeURIComponent(docId)}/review`,
        {
          status,
          note: status === "rejected" ? note || "Please re-upload a clearer copy of this document." : note
        },
        session.token
      );
      onUpdated(payload.customer);
      await onAction(status === "accepted" ? "Document accepted." : "Document rejected — customer can replace it.");
    } catch (caught) {
      setError(messageFromError(caught, "Document review failed."));
    } finally {
      setBusy("");
    }
  };

  const submitPackage = async () => {
    if (admin) return;
    setBusy("submit");
    setError("");
    try {
      const payload = await apiPost<{ customer: Customer }>(
        `/api/customers/${encodeURIComponent(customer.id)}/documents/submit`,
        {},
        session.token
      );
      onUpdated(payload.customer);
      await onAction("Submitted for review. This form is closed until ChakuChuri asks again.");
    } catch (caught) {
      setError(messageFromError(caught, "Add all required files before submitting."));
    } finally {
      setBusy("");
    }
  };

  const setVerification = async (status: "verified" | "unverified") => {
    if (!admin) return;
    setBusy(status);
    setError("");
    try {
      const payload = await apiPost<{ customer: Customer }>(
        `/api/customers/${encodeURIComponent(customer.id)}/verification`,
        { status, note, cnic },
        session.token
      );
      onUpdated(payload.customer);
      await onAction(`Account marked ${verificationLabel(status).toLowerCase()}.`);
    } catch (caught) {
      setError(messageFromError(caught, "Verification could not be updated."));
    } finally {
      setBusy("");
    }
  };

  const createSecureLink = async () => {
    if (!admin) return;
    setBusy("link");
    setError("");
    try {
      const payload = await apiPost<{ url: string; customer?: Customer }>(
        `/api/customers/${encodeURIComponent(customer.id)}/document-requests`,
        {
          kinds: ["cnic_front", "cnic_back", "selfie", "other"],
          expiresInDays: 7,
          note: note || "Please complete identity verification for your ChakuChuri account."
        },
        session.token
      );
      setLinkUrl(payload.url);
      if (payload.customer) onUpdated(payload.customer);
      await navigator.clipboard?.writeText(payload.url).catch(() => undefined);
      await onAction("Secure verification link created and copied. Any earlier active link is now closed.");
    } catch (caught) {
      setError(messageFromError(caught, "Verification link could not be created."));
    } finally {
      setBusy("");
    }
  };

  const askDocuments = async () => {
    if (!admin) return;
    const label = askLabel.trim();
    if (!label) return;
    setBusy("ask");
    setError("");
    try {
      const payload = await apiPost<{ customer: Customer }>(
        `/api/customers/${encodeURIComponent(customer.id)}/document-asks`,
        { labels: [label], note: note || `Please upload: ${label}` },
        session.token
      );
      onUpdated(payload.customer);
      setAskLabel("");
      await onAction(`Asked for “${label}”. Customer will see it on the form.`);
    } catch (caught) {
      setError(messageFromError(caught, "Could not request additional document."));
    } finally {
      setBusy("");
    }
  };

  const withdrawAsk = async (askId: string, label: string) => {
    if (!admin || !window.confirm(`Withdraw the ${label} requirement? Its audit history will be retained.`)) return;
    setBusy(`withdraw-${askId}`);
    setError("");
    try {
      const payload = await apiPost<{ customer: Customer }>(
        `/api/customers/${encodeURIComponent(customer.id)}/document-asks/${encodeURIComponent(askId)}/withdraw`,
        {},
        session.token
      );
      onUpdated(payload.customer);
      await onAction(`${label} is no longer required.`);
    } catch (caught) {
      setError(messageFromError(caught, "Could not withdraw this requirement."));
    } finally {
      setBusy("");
    }
  };

  const rejectedCount = workflowDocuments.filter((doc) => docStatus(doc) === "rejected").length;
  const remainingCount = customerSlots.length + (admin ? 0 : openAsks.filter((ask) => slotNeedsAction(askDocs.get(ask.id))).length);
  const needsReviewCount = workflowDocuments.filter((doc) => docStatus(doc) === "submitted").length;
  const acceptedDocsCount = workflowDocuments.filter((doc) => docStatus(doc) === "accepted").length;
  const waitingAskCount = openAsks.filter((ask) => !askDocs.get(ask.id) || docStatus(askDocs.get(ask.id)) === "rejected").length;
  const requiredAccepted = KYC_SLOTS.every((slot) => docStatus(byKind.get(slot.kind)) === "accepted");
  const requestedAccepted = activeAsks.every((ask) => docStatus(askDocs.get(ask.id)) === "accepted");
  const verificationReady = requiredAccepted && requestedAccepted;
  const verified = isCustomerVerified(customer.verificationStatus);
  const stageLabels: Record<CustomerVerificationStage, string> = {
    not_started: "Not started",
    action_required: "Action required",
    in_review: "In review",
    ready_to_verify: "Ready to verify",
    verified: "Verified"
  };
  const caseStage = stageLabels[customerVerificationStage(customer)];
  const verificationBlocker = !requiredAccepted
    ? "Accept CNIC front, CNIC back and selfie first."
    : !requestedAccepted
      ? "Accept every additional requested document first."
      : "All checks are complete. The account can be verified.";

  if (admin) {
    return (
      <div className="cc-docs-panel cc-docs-admin-desk">
        <header className="cc-docs-panel-head">
          <div>
            <span>Identity documents</span>
            <strong>Document review</strong>
            <p>Preview submissions, accept or reject per file, and ask for extra documents by name.</p>
          </div>
          <div className="cc-docs-head-status">
            <span className={`cc-docs-case-state ${caseStage.toLowerCase().replaceAll(" ", "-")}`}>{caseStage}</span>
            <VerificationBadge status={customer.verificationStatus} />
          </div>
        </header>

        <div className="cc-docs-summary">
          <article className={needsReviewCount ? "warn" : ""}>
            <span>Needs review</span>
            <strong>{needsReviewCount}</strong>
          </article>
          <article>
            <span>Waiting from customer</span>
            <strong>{waitingAskCount + KYC_SLOTS.filter((slot) => !byKind.get(slot.kind) || docStatus(byKind.get(slot.kind)) === "rejected").length}</strong>
          </article>
          <article className="ok">
            <span>Accepted</span>
            <strong>{acceptedDocsCount}</strong>
          </article>
          <article>
            <span>Extra asks</span>
            <strong>{activeAsks.length}</strong>
          </article>
        </div>

        {error && <div className="cc-docs-error">{error}</div>}

        <section className="cc-docs-section">
          <header className="cc-docs-section-head">
            <div>
              <strong>Required KYC</strong>
              <small>CNIC front, CNIC back and selfie</small>
            </div>
          </header>
          <div className="cc-docs-slots">
            {KYC_SLOTS.map((slot) => {
              const doc = byKind.get(slot.kind);
              const status = docStatus(doc);
              return (
                <DocumentSlot
                  adminReview
                  busy={busy.startsWith(doc?.id || slot.kind)}
                  canView
                  doc={doc}
                  key={slot.kind}
                  label={slot.label}
                  onAccept={doc && (status === "submitted" || status === "rejected") ? () => void reviewDoc(doc.id, "accepted") : undefined}
                  onReject={doc && (status === "submitted" || status === "accepted") ? () => void reviewDoc(doc.id, "rejected") : undefined}
                  onRemove={doc && isEditableDoc(doc) ? () => void removeDoc(doc.id) : undefined}
                  token={session.token}
                />
              );
            })}
          </div>
        </section>

        <section className="cc-docs-section">
          <header className="cc-docs-section-head">
            <div>
              <strong>Additional requests</strong>
              <small>Ask by document name — customer submissions appear here for review</small>
            </div>
          </header>

          <div className="cc-docs-ask-row">
            <input onChange={(event) => setAskLabel(event.target.value)} placeholder="e.g. Trade license, NTN certificate" value={askLabel} />
            <button disabled={Boolean(busy) || !askLabel.trim()} onClick={() => void askDocuments()} type="button">
              <RefreshCw size={16} /> {busy === "ask" ? "Asking…" : "Ask customer"}
            </button>
          </div>

          <div className="cc-docs-slots">
            {activeAsks.length === 0 && extras.length === 0 && (
              <div className="cc-docs-empty-ask">No active additional requirements.</div>
            )}
            {activeAsks.map((ask) => {
              const doc = askDocs.get(ask.id);
              const status = docStatus(doc);
              const waiting = !doc || status === "rejected";
              return (
                <DocumentSlot
                  adminReview
                  askStatus={ask.status}
                  busy={busy.startsWith(doc?.id || ask.id)}
                  canView
                  doc={doc}
                  key={ask.id}
                  label={ask.label}
                  onAccept={doc && (status === "submitted" || status === "rejected") ? () => void reviewDoc(doc.id, "accepted") : undefined}
                  onReject={doc && (status === "submitted" || status === "accepted") ? () => void reviewDoc(doc.id, "rejected") : undefined}
                  onRemove={doc && isEditableDoc(doc) ? () => void removeDoc(doc.id) : undefined}
                  token={session.token}
                  onWithdraw={() => void withdrawAsk(ask.id, ask.label)}
                  waitingLabel={waiting ? (status === "rejected" ? "Rejected — waiting for re-upload" : "Waiting for customer upload") : undefined}
                />
              );
            })}
            {extras.map((doc) => (
              <DocumentSlot
                adminReview
                busy={busy.startsWith(doc.id)}
                canView
                doc={doc}
                key={doc.id}
                label={doc.label || "Other document"}
                onAccept={docStatus(doc) === "submitted" || docStatus(doc) === "rejected" ? () => void reviewDoc(doc.id, "accepted") : undefined}
                onReject={docStatus(doc) === "submitted" || docStatus(doc) === "accepted" ? () => void reviewDoc(doc.id, "rejected") : undefined}
                onRemove={() => void removeDoc(doc.id)}
                token={session.token}
              />
            ))}
          </div>

          <label className="cc-docs-upload-row">
            <input onChange={(event) => setOtherLabel(event.target.value)} placeholder="Staff upload label (optional)" value={otherLabel} />
            <span className={"cc-docs-file-btn " + (busy === "other" ? "busy" : "")}>
              <FileUp size={16} />
              Upload for customer
              <input accept="image/jpeg,image/png,image/webp,application/pdf,.jpg,.jpeg,.png,.webp,.pdf" disabled={Boolean(busy)} hidden onChange={(event) => onPick("other", event, otherLabel || "Other document")} type="file" />
            </span>
          </label>
        </section>

        <section className="cc-docs-section cc-docs-admin-tools">
          <header className="cc-docs-section-head">
            <div>
              <strong>Account actions</strong>
              <small>Verification status, customer note and invite link</small>
            </div>
          </header>
          <div className={`cc-docs-readiness ${verificationReady ? "ready" : "blocked"}`}>
            {verificationReady ? <CheckCircle2 size={18} /> : <ShieldAlert size={18} />}
            <div>
              <strong>{verificationReady ? "Review complete" : "Verification locked"}</strong>
              <small>{verificationBlocker}</small>
            </div>
          </div>
          <div className="cc-docs-admin-grid">
            <label>
              <span>CNIC number</span>
              <input onChange={(event) => setCnic(event.target.value)} placeholder="Optional CNIC / NICOP" value={cnic} />
            </label>
            <label className="wide">
              <span>Customer-facing note</span>
              <textarea onChange={(event) => setNote(event.target.value)} placeholder="Shown with Action required" rows={3} value={note} />
            </label>
          </div>
          <div className="cc-docs-admin-actions">
            <button
              disabled={Boolean(busy) || !verificationReady || verified}
              onClick={() => void setVerification("verified")}
              title={verified ? "Account is already verified" : verificationBlocker}
              type="button"
            >
              <ShieldCheck size={16} /> Mark verified
            </button>
            <button className="ghost" disabled={Boolean(busy) || !verified} onClick={() => void setVerification("unverified")} type="button">
              Mark unverified
            </button>
            <button disabled={Boolean(busy) || verified} onClick={() => void createSecureLink()} type="button">
              <Link2 size={16} /> {busy === "link" ? "Creating..." : "Create secure link"}
            </button>
          </div>
          {linkUrl && (
            <div className="cc-docs-link">
              <code>{linkUrl}</code>
              <button onClick={() => void navigator.clipboard?.writeText(linkUrl)} type="button" title="Copy again">
                <Copy size={15} />
              </button>
            </div>
          )}
        </section>
      </div>
    );
  }

  return (
    <div className="cc-docs-panel">
      <header className="cc-docs-panel-head">
        <div>
          <span>Identity documents</span>
          <strong>Verification form</strong>
          <p>Only documents that still need you are shown. Accepted files stay hidden and locked.</p>
        </div>
        <em className="cc-docs-progress">
          {remainingCount > 0 ? `${remainingCount} to finish` : "Ready"}
          {acceptedDocsCount > 0 ? ` · ${acceptedDocsCount} accepted` : ""}
        </em>
      </header>

      <div className={"cc-docs-guide " + (rejectedCount ? "rejected" : remainingCount === 0 ? "unverified" : "unverified")}>
        {rejectedCount
          ? "Replace the rejected file(s) below, then Submit again. Accepted documents stay locked and are not shown."
          : remainingCount === 0
            ? "Nothing left to upload. Your documents are with ChakuChuri for review."
            : openAsks.length && customerSlots.length === 0
              ? "Upload the requested document(s) below, then Submit."
              : acceptedDocsCount > 0
                ? "Finish the remaining item(s) below, then Submit. Accepted documents are already locked."
                : "Attach each required file, replace freely, then Submit for review."}
      </div>

      {error && <div className="cc-docs-error">{error}</div>}

      <div className="cc-docs-slots">
        {customerSlots.map((slot) => {
          const doc = byKind.get(slot.kind);
          const editable = isEditableDoc(doc);
          return (
            <DocumentSlot
              busy={busy.startsWith(doc?.id || slot.kind)}
              canView={docStatus(doc) === "draft"}
              doc={doc}
              key={slot.kind}
              label={slot.label}
              onPick={editable ? (event) => onPick(slot.kind, event) : undefined}
              onRemove={doc && docStatus(doc) === "draft" ? () => void removeDoc(doc.id) : undefined}
              token={session.token}
              uploadLabel={doc ? "Replace" : "Upload"}
            />
          );
        })}
      </div>

      {openAsks.length > 0 && (
        <div className="cc-docs-extra">
          <div className="cc-docs-extra-head">
            <strong>Requested by ChakuChuri</strong>
            <small>Upload these, then Submit once</small>
          </div>
          <div className="cc-docs-slots">
            {openAsks.filter((ask) => slotNeedsAction(askDocs.get(ask.id))).map((ask) => {
              const doc = askDocs.get(ask.id);
              return (
                <DocumentSlot
                  busy={busy.startsWith(doc?.id || ask.id)}
                  canView={docStatus(doc) === "draft"}
                  doc={doc}
                  key={ask.id}
                  label={ask.label}
                  onPick={isEditableDoc(doc) ? (event) => onPick("other", event, ask.label, ask.id) : undefined}
                  onRemove={doc && docStatus(doc) === "draft" ? () => void removeDoc(doc.id) : undefined}
                  token={session.token}
                  uploadLabel={doc ? "Replace" : "Upload"}
                />
              );
            })}
          </div>
        </div>
      )}

      <div className="cc-docs-submit-bar">
        <div>
          <strong>Submit package</strong>
          <small>
            {remainingCount === 0
              ? "No files left to send. Accepted and submitted documents stay locked."
              : "Only the files below are sent. Already accepted documents stay locked."}
          </small>
        </div>
        <button
          className="cc-docs-submit-btn"
          disabled={Boolean(busy) || !canSubmit || remainingCount === 0}
          onClick={() => void submitPackage()}
          type="button"
        >
          {busy === "submit" ? "Submitting…" : remainingCount === 0 ? "Nothing to submit" : "Submit for review"}
        </button>
      </div>

      {customer.identityNote && (
        <p className="cc-docs-note"><BadgeCheck size={16} /> {customer.identityNote}</p>
      )}
    </div>
  );
}

function DocumentSlot({
  label,
  doc,
  token,
  busy,
  canView = false,
  adminReview = false,
  askStatus,
  waitingLabel,
  onPick,
  onRemove,
  onWithdraw,
  onAccept,
  onReject,
  uploadLabel = "Upload"
}: {
  label: string;
  doc?: CustomerDocument;
  token: string;
  busy?: boolean;
  canView?: boolean;
  adminReview?: boolean;
  askStatus?: string;
  waitingLabel?: string;
  onPick?: (event: ChangeEvent<HTMLInputElement>) => void;
  onRemove?: () => void;
  onWithdraw?: () => void;
  onAccept?: () => void;
  onReject?: () => void;
  uploadLabel?: string;
}) {
  const [thumb, setThumb] = useState("");
  const status = docStatus(doc);
  const locked = isLockedDoc(doc);

  useEffect(() => {
    if (!canView || !doc?.fileId) {
      setThumb("");
      return;
    }
    let active = true;
    apiBlob(`/api/files/${doc.fileId}/thumbnail`, token)
      .then((blob) => {
        if (!active) return;
        setThumb(URL.createObjectURL(blob));
      })
      .catch(() => {
        if (active) setThumb("");
      });
    return () => {
      active = false;
    };
  }, [canView, doc?.fileId, token]);

  useEffect(() => () => {
    if (thumb) URL.revokeObjectURL(thumb);
  }, [thumb]);

  const openFile = async () => {
    if (!canView || !doc?.fileId) return;
    try {
      const blob = await apiBlob(`/api/files/${doc.fileId}/content`, token);
      const url = URL.createObjectURL(blob);
      window.open(url, "_blank", "noopener,noreferrer");
      window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch {
      // ignore
    }
  };

  const statusText = waitingLabel
    ? waitingLabel
    : !doc
      ? "Required"
      : status === "rejected"
        ? "Rejected · replace"
        : status === "draft"
          ? "Attached · not submitted"
          : status === "accepted"
            ? "Accepted"
            : "Submitted · in review";

  const detail = waitingLabel
    ? askStatus === "open" ? "Request sent to customer" : `Ask status: ${askStatus || "open"}`
    : !doc
      ? "Upload a clear photo or PDF"
      : canView && doc.originalName
        ? `${doc.originalName} · ${dateTime(doc.uploadedAt)}`
        : status === "draft"
          ? "Ready — you can still replace"
          : status === "rejected"
            ? "Re-upload this file only"
            : status === "accepted"
              ? "Approved — locked"
              : "Locked after submit";

  return (
    <article className={"cc-docs-slot " + (doc ? "filled" : "") + (locked ? " locked" : "") + (status === "rejected" ? " rejected" : "") + (status === "accepted" ? " accepted" : "") + (waitingLabel ? " waiting" : "") + (status === "submitted" ? " review" : "")}>
      <div className="cc-docs-slot-preview">
        {thumb ? <img alt="" src={thumb} /> : waitingLabel ? <RefreshCw size={22} /> : locked || status === "draft" ? <CheckCircle2 size={22} /> : <FileUp size={22} />}
      </div>
      <div>
        <strong>{label}</strong>
        <small>{detail}</small>
        <em className={"cc-docs-status " + (waitingLabel ? "waiting" : status || "empty")}>{statusText}</em>
      </div>
      <div className={"cc-docs-slot-actions " + (adminReview ? "review" : "")}>
        {canView && doc?.fileId && (
          <button onClick={() => void openFile()} type="button" title="Preview file">
            <ExternalLink size={15} />
            {adminReview && <span>Open</span>}
          </button>
        )}
        {onAccept && (
          <button className="ok" disabled={busy} onClick={onAccept} type="button" title="Accept this document">
            <ShieldCheck size={15} />
            {adminReview && <span>Accept</span>}
          </button>
        )}
        {onReject && (
          <button className="warn" disabled={busy} onClick={onReject} type="button" title="Reject this document">
            <ShieldAlert size={15} />
            {adminReview && <span>Reject</span>}
          </button>
        )}
        {onRemove && (
          <button disabled={busy} onClick={onRemove} type="button" title="Remove">
            <Trash2 size={15} />
          </button>
        )}
        {onWithdraw && (
          <button className="neutral" disabled={busy} onClick={onWithdraw} type="button" title="Withdraw requirement">
            <X size={15} />
            {adminReview && <span>Withdraw</span>}
          </button>
        )}
        {onPick && (
          <label className={"cc-docs-file-btn " + (busy ? "busy" : "")}>
            {busy ? "…" : uploadLabel}
            <input accept="image/jpeg,image/png,image/webp,application/pdf,.jpg,.jpeg,.png,.webp,.pdf" disabled={busy} hidden onChange={onPick} type="file" />
          </label>
        )}
        {!onPick && !waitingLabel && locked && <span className="cc-docs-locked-tag">{status === "accepted" ? "Accepted" : "Locked"}</span>}
        {waitingLabel && <span className="cc-docs-locked-tag waiting">Waiting</span>}
      </div>
    </article>
  );
}
