import { useEffect, useMemo, useState, type ChangeEvent } from "react";
import { CheckCircle2, Clock3, FileUp, LockKeyhole, ShieldCheck } from "lucide-react";
import { apiGet } from "./api";
import { KYC_SLOTS } from "./customer-documents";
import { messageFromError } from "./utils";

type PublicRequest = {
  id: string;
  customerId: string;
  companyName?: string;
  kinds: string[];
  status: "active" | "submitted" | "revoked" | "expired";
  note?: string;
  expiresAt: string;
  submittedAt?: string;
};

type SubmittedDoc = {
  kind: string;
  label?: string;
  status: string;
  originalName?: string;
  uploadedAt: string;
  requestId?: string;
};

type RequestedDoc = {
  id: string;
  label: string;
  kind?: string;
  status: string;
};

type PublicPayload = {
  request?: PublicRequest;
  submitted?: SubmittedDoc[];
  requestedDocuments?: RequestedDoc[];
  verificationStatus?: string;
  documentsSubmittedAt?: string;
};

type PublicEnvelope = {
  ok?: boolean;
  error?: { message?: string };
  data?: PublicPayload;
};

const allowedFileTypes = ["image/jpeg", "image/png", "image/webp", "application/pdf"];

export function verifyTokenFromPath(pathname: string) {
  const match = pathname.match(/^\/verify\/([^/?#]+)/i);
  if (!match) return "";
  try {
    return decodeURIComponent(match[1]);
  } catch {
    return "";
  }
}

export function PublicDocumentForm({ token, onDone }: { token: string; onDone?: () => void }) {
  const [request, setRequest] = useState<PublicRequest | null>(null);
  const [submitted, setSubmitted] = useState<SubmittedDoc[]>([]);
  const [requestedDocuments, setRequestedDocuments] = useState<RequestedDoc[]>([]);
  const [verificationStatus, setVerificationStatus] = useState("unverified");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const [otherLabel, setOtherLabel] = useState("");
  const [loading, setLoading] = useState(true);

  const applyPayload = (payload?: PublicPayload) => {
    if (!payload) return;
    if (payload.request) setRequest(payload.request);
    setSubmitted(payload.submitted || []);
    setRequestedDocuments(payload.requestedDocuments || []);
    if (payload.verificationStatus) setVerificationStatus(payload.verificationStatus.toLowerCase());
  };

  const load = async () => {
    setLoading(true);
    try {
      const payload = await apiGet<PublicPayload>(`/api/public/document-requests/${encodeURIComponent(token)}`);
      applyPayload(payload);
      setError("");
    } catch (caught) {
      setError(messageFromError(caught, "This secure link is invalid or no longer active."));
      setRequest(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, [token]);

  const kinds = useMemo(
    () => request?.kinds?.length ? request.kinds : ["cnic_front", "cnic_back", "selfie"],
    [request]
  );
  const slotDoc = (kind: string) => submitted.find((doc) => doc.kind === kind && !doc.requestId);
  const askDoc = (askId: string) => submitted.find((doc) => doc.requestId === askId);
  const openAsks = requestedDocuments.filter((ask) => (ask.status || "").toLowerCase() === "open");
  const requestStatus = request?.status || "active";
  const packageSubmitted = requestStatus === "submitted";
  const locked = verificationStatus === "verified" || requestStatus !== "active";
  const requiredDraftReady = KYC_SLOTS.filter((slot) => kinds.includes(slot.kind)).every((slot) => {
    const doc = slotDoc(slot.kind);
    const status = (doc?.status || "").toLowerCase();
    return Boolean(doc) && status !== "rejected" && ["draft", "submitted", "accepted", "uploaded"].includes(status);
  });
  const asksReady = openAsks.every((ask) => {
    const doc = askDoc(ask.id);
    const status = (doc?.status || "").toLowerCase();
    return Boolean(doc) && status !== "rejected" && ["draft", "submitted", "accepted", "uploaded"].includes(status);
  });
  const hasDraft = submitted.some((doc) => doc.status.toLowerCase() === "draft");
  const otherDrafts = submitted.filter((doc) => doc.kind === "other" && !doc.requestId && doc.status.toLowerCase() === "draft");

  const upload = async (kind: string, file: File, label = "", requestId = "") => {
    if (locked || packageSubmitted) return;
    if (file.size > 10 * 1024 * 1024) {
      setError("Choose a file smaller than 10 MB.");
      return;
    }
    if (file.type && !allowedFileTypes.includes(file.type)) {
      setError("Use a JPEG, PNG, WebP or PDF file.");
      return;
    }
    const existing = requestId ? askDoc(requestId) : slotDoc(kind);
    const status = (existing?.status || "").toLowerCase();
    if (!requestId && kind !== "other" && existing && !["draft", "rejected"].includes(status)) return;

    setBusy(requestId || kind);
    setError("");
    try {
      const body = new FormData();
      body.append("file", file);
      body.append("kind", kind);
      if (label.trim()) body.append("label", label.trim());
      if (requestId) body.append("requestId", requestId);
      const response = await fetch(`/api/public/document-requests/${encodeURIComponent(token)}/upload`, {
        method: "POST",
        body
      });
      const envelope = await readEnvelope(response);
      applyPayload(envelope.data);
      if (kind === "other" && !requestId) setOtherLabel("");
    } catch (caught) {
      setError(messageFromError(caught, "File could not be uploaded."));
    } finally {
      setBusy("");
    }
  };

  const submitPackage = async () => {
    setBusy("submit");
    setError("");
    try {
      const response = await fetch(`/api/public/document-requests/${encodeURIComponent(token)}/submit`, {
        method: "POST"
      });
      const envelope = await readEnvelope(response);
      applyPayload(envelope.data);
      onDone?.();
    } catch (caught) {
      setError(messageFromError(caught, "Add all required files, then submit once."));
    } finally {
      setBusy("");
    }
  };

  const onPick = (kind: string, event: ChangeEvent<HTMLInputElement>, label = "", requestId = "") => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (file) void upload(kind, file, label, requestId);
  };

  if (loading) {
    return <main className="cc-public-docs"><section className="cc-public-docs-card"><p>Loading secure form...</p></section></main>;
  }

  if (!request) {
    return (
      <main className="cc-public-docs">
        <section className="cc-public-docs-card cc-public-docs-complete">
          <span className="cc-public-docs-complete-icon unavailable"><LockKeyhole size={25} /></span>
          <p className="cc-public-docs-kicker">Secure verification</p>
          <h1>Link unavailable</h1>
          <p>{error || "This document link is invalid, expired or replaced by a newer link."}</p>
        </section>
      </main>
    );
  }

  if (locked || packageSubmitted) {
    const heading = verificationStatus === "verified"
      ? "Account already verified"
      : requestStatus === "submitted"
        ? "Submitted for review"
        : "Link closed";
    const detail = verificationStatus === "verified"
      ? "This company account is verified. No more files can be added."
      : requestStatus === "submitted"
        ? "Your identity package is locked and waiting for ChakuChuri review."
        : "This secure link is no longer active. Ask ChakuChuri for a new link.";
    return (
      <main className="cc-public-docs">
        <section className="cc-public-docs-card cc-public-docs-complete">
          <span className="cc-public-docs-complete-icon"><CheckCircle2 size={26} /></span>
          <p className="cc-public-docs-kicker">Secure verification</p>
          <h1>{heading}</h1>
          <p>{detail}</p>
          {request.submittedAt && <small>Submitted {new Date(request.submittedAt).toLocaleString()}</small>}
        </section>
      </main>
    );
  }

  return (
    <main className="cc-public-docs">
      <section className="cc-public-docs-card">
        <div className="cc-public-docs-brand">
          <span>CC</span>
          <div><strong>ChakuChuri.pk</strong><small>Protected document intake</small></div>
        </div>
        <p className="cc-public-docs-kicker">Identity verification</p>
        <h1>{request.companyName || "Customer"}</h1>
        <p>Attach clear images or PDFs. Drafts can be replaced until the final submission.</p>

        <div className="cc-public-docs-meta">
          <span><Clock3 size={17} /><strong>Secure link</strong><small>Expires {new Date(request.expiresAt).toLocaleString()}</small></span>
          <span><FileUp size={17} /><strong>File rules</strong><small>JPEG, PNG, WebP or PDF / 10 MB max</small></span>
          <span><ShieldCheck size={17} /><strong>Final step</strong><small>Submission locks every draft</small></span>
        </div>

        {request.note && <div className="cc-docs-guide unverified">{request.note}</div>}
        {error && <div className="cc-docs-error">{error}</div>}

        <div className="cc-docs-slots">
          {KYC_SLOTS.filter((slot) => kinds.includes(slot.kind)).map((slot) => {
            const existing = slotDoc(slot.kind);
            const status = (existing?.status || "").toLowerCase();
            const canUpload = !existing || status === "draft" || status === "rejected";
            return (
              <article className={`cc-docs-slot ${existing ? "filled" : ""} ${status}`} key={slot.kind}>
                <div className="cc-docs-slot-preview">{existing && status !== "rejected" ? <CheckCircle2 size={22} /> : <FileUp size={22} />}</div>
                <div>
                  <strong>{slot.label}</strong>
                  <small>{!existing ? "Required / add file" : status === "draft" ? `${existing.originalName || "File"} / ready` : status === "rejected" ? "Rejected / upload a clearer copy" : "Locked"}</small>
                </div>
                {canUpload ? (
                  <label className={`cc-docs-file-btn ${busy === slot.kind ? "busy" : ""}`}>
                    {busy === slot.kind ? "..." : existing ? "Replace" : "Upload"}
                    <input accept="image/jpeg,image/png,image/webp,application/pdf,.jpg,.jpeg,.png,.webp,.pdf" disabled={Boolean(busy)} hidden onChange={(event) => onPick(slot.kind, event)} type="file" />
                  </label>
                ) : <span className="cc-docs-locked-tag">Locked</span>}
              </article>
            );
          })}
        </div>

        {openAsks.length > 0 && (
          <div className="cc-docs-extra">
            <div className="cc-docs-extra-head">
              <strong>Requested by ChakuChuri</strong>
              <small>{openAsks.length} required</small>
            </div>
            <div className="cc-docs-slots">
              {openAsks.map((ask) => {
                const existing = askDoc(ask.id);
                const status = (existing?.status || "").toLowerCase();
                const canUpload = !existing || status === "draft" || status === "rejected";
                return (
                  <article className={`cc-docs-slot ${existing ? "filled" : ""} ${status}`} key={ask.id}>
                    <div className="cc-docs-slot-preview">{existing && status !== "rejected" ? <CheckCircle2 size={22} /> : <FileUp size={22} />}</div>
                    <div>
                      <strong>{ask.label || "Extra document"}</strong>
                      <small>{!existing ? "Required / add file" : status === "draft" ? `${existing.originalName || "File"} / ready` : status === "rejected" ? "Rejected / upload a clearer copy" : "Locked"}</small>
                    </div>
                    {canUpload ? (
                      <label className={`cc-docs-file-btn ${busy === ask.id ? "busy" : ""}`}>
                        {busy === ask.id ? "..." : existing ? "Replace" : "Upload"}
                        <input accept="image/jpeg,image/png,image/webp,application/pdf,.jpg,.jpeg,.png,.webp,.pdf" disabled={Boolean(busy)} hidden onChange={(event) => onPick("other", event, ask.label || "Extra document", ask.id)} type="file" />
                      </label>
                    ) : <span className="cc-docs-locked-tag">Locked</span>}
                  </article>
                );
              })}
            </div>
          </div>
        )}

        {kinds.includes("other") && (
          <div className="cc-docs-extra">
            <div className="cc-docs-extra-head">
              <strong>Additional documents</strong>
              <small>{otherDrafts.length ? `${otherDrafts.length} attached` : "Optional"}</small>
            </div>
            <label className="cc-docs-upload-row">
              <input onChange={(event) => setOtherLabel(event.target.value)} placeholder="Document label" value={otherLabel} />
              <span className={`cc-docs-file-btn ${busy === "other" ? "busy" : ""}`}>
                <FileUp size={16} /> {busy === "other" ? "Uploading..." : "Add file"}
                <input accept="image/jpeg,image/png,image/webp,application/pdf,.jpg,.jpeg,.png,.webp,.pdf" disabled={Boolean(busy)} hidden onChange={(event) => onPick("other", event, otherLabel || "Other document")} type="file" />
              </span>
            </label>
          </div>
        )}

        <div className="cc-docs-submit-bar">
          <div><strong>Ready to submit?</strong><small>This action permanently closes this link.</small></div>
          <button className="cc-docs-submit-btn" disabled={Boolean(busy) || !requiredDraftReady || !asksReady || !hasDraft} onClick={() => void submitPackage()} type="button">
            {busy === "submit" ? "Submitting..." : "Submit for review"}
          </button>
        </div>
      </section>
    </main>
  );
}

async function readEnvelope(response: Response): Promise<PublicEnvelope> {
  const raw = await response.text();
  let envelope: PublicEnvelope;
  try {
    envelope = JSON.parse(raw) as PublicEnvelope;
  } catch {
    throw new Error("The server returned an invalid response.");
  }
  if (!response.ok || !envelope.ok || !envelope.data) {
    throw new Error(envelope.error?.message || "Request failed.");
  }
  return envelope;
}
