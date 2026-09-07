import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Activity, BadgeCheck, Camera, Check, Download, Eye, EyeOff, KeyRound, LockKeyhole, Mail, Smartphone, Trash2, UserRound, X } from "lucide-react";
import { apiPost, apiUploadFile } from "./api";
import { VerificationBadge, VERIFY_IDENTITY_PAGE, customerCanStartVerification, customerIdentityReviewPending, customerNeedsIdentityAction, customerVerificationStage } from "./customer-documents";
import { ProfileAvatar } from "./profile-avatar";
import type { Customer, Session, User } from "./types";
import { date, messageFromError } from "./utils";

const ANDROID_APK_URL = "/downloads/chakuchuri-android.apk";
const ANDROID_APK_NAME = "chakuchuri-android.apk";
const ANDROID_APP_VERSION = "0.1.21+2018";
const ANDROID_APK_REVISION = "20260906-204000";
/** Set when the APK in public/downloads is replaced; Last-Modified from HEAD overrides when available. */
const ANDROID_APP_BUILT_AT = "2026-09-06T20:40:00+05:00";
const ANDROID_BUILD_NOTES =
  "Announcement notices use green megaphone icon; in-app Download update; updates over 0.1.20/2017 without uninstall.";

function formatApkBuiltAt(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short"
  });
}

export function ProfileSettings({
  session,
  onAction,
  onSaved,
  verificationStatus,
  customer,
  onCustomerUpdated,
  onNavigate
}: {
  session: Session;
  onAction: (message: string) => Promise<void>;
  onSaved: (user: User) => void;
  verificationStatus?: string;
  customer?: Customer;
  onCustomerUpdated?: (customer: Customer) => void;
  onNavigate?: (page: string) => void;
}) {
  const [name, setName] = useState(session.user.name);
  const [email, setEmail] = useState(session.user.email);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPasswords, setShowPasswords] = useState(false);
  const [photo, setPhoto] = useState<File | null>(null);
  const [removePhoto, setRemovePhoto] = useState(false);
  const [photoMenuOpen, setPhotoMenuOpen] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [inviteBusy, setInviteBusy] = useState(false);
  const photoInputRef = useRef<HTMLInputElement>(null);
  const photoMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setName(session.user.name);
    setEmail(session.user.email);
  }, [session.user.email, session.user.name]);

  const preview = useMemo(() => photo ? URL.createObjectURL(photo) : "", [photo]);
  useEffect(() => () => { if (preview) URL.revokeObjectURL(preview); }, [preview]);
  useEffect(() => {
    if (!photoMenuOpen) return;
    const onPointerDown = (event: MouseEvent) => {
      if (photoMenuRef.current && !photoMenuRef.current.contains(event.target as Node)) {
        setPhotoMenuOpen(false);
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setPhotoMenuOpen(false);
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [photoMenuOpen]);
  const sensitiveChange = email.trim().toLowerCase() !== session.user.email.toLowerCase() || Boolean(newPassword);
  const changed = name.trim() !== session.user.name || email.trim().toLowerCase() !== session.user.email.toLowerCase() || Boolean(newPassword || photo || removePhoto);
  const hasPhoto = Boolean((session.user.profileImageFileId || photo) && !removePhoto);

  const choosePhoto = (file?: File) => {
    if (!file) return;
    if (!file.type.startsWith("image/") || file.size > 5 * 1024 * 1024) {
      setError("Choose a JPG, PNG or WebP image up to 5 MB.");
      return;
    }
    setError("");
    setPhoto(file);
    setRemovePhoto(false);
    setPhotoMenuOpen(false);
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!name.trim() || !email.trim()) return setError("Name and login email are required.");
    if (newPassword && newPassword.length < 8) return setError("New password must be at least 8 characters.");
    if (newPassword !== confirmPassword) return setError("New password and confirmation do not match.");
    if (sensitiveChange && !currentPassword) return setError("Enter your current password to change login details.");

    setBusy(true);
    setError("");
    try {
      let profileImageFileId = session.user.profileImageFileId || "";
      if (photo) {
        const uploaded = await apiUploadFile(photo, {
          customerId: session.user.customerId || "",
          ownerType: "user_profile",
          ownerId: session.user.id
        }, session.token);
        profileImageFileId = uploaded.id;
      }
      const user = await apiPost<User>("/api/auth/profile", {
        name: name.trim(),
        email: email.trim().toLowerCase(),
        currentPassword,
        newPassword,
        profileImageFileId,
        removeProfileImage: removePhoto
      }, session.token);
      onSaved(user);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setPhoto(null);
      setRemovePhoto(false);
      await onAction("Personal account settings saved.");
    } catch (caught) {
      setError(messageFromError(caught, "Account settings could not be saved."));
    } finally {
      setBusy(false);
    }
  };

  const reset = () => {
    setName(session.user.name);
    setEmail(session.user.email);
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
    setPhoto(null);
    setRemovePhoto(false);
    setError("");
  };

  const startVerification = async () => {
    if (!customer) return;
    const stage = customerVerificationStage(customer);
    if (stage === "action_required" || stage === "in_review" || stage === "ready_to_verify") {
      onNavigate?.(VERIFY_IDENTITY_PAGE);
      return;
    }
    if (stage !== "not_started") return;
    setInviteBusy(true);
    setError("");
    try {
      const payload = await apiPost<{ customer: Customer }>(
        `/api/customers/${encodeURIComponent(customer.id)}/verification-invite`,
        { note: "Complete your one-time identity form to get verified." },
        session.token
      );
      onCustomerUpdated?.(payload.customer);
      await onAction("Verification form opened.");
      onNavigate?.(VERIFY_IDENTITY_PAGE);
    } catch (caught) {
      const message = messageFromError(caught, "Could not open verification form.");
      if (/in review/i.test(message)) {
        onCustomerUpdated?.(customer);
        await onAction("Documents are already with ChakuChuri for review.");
        return;
      }
      setError(message);
    } finally {
      setInviteBusy(false);
    }
  };

  return (
    <div className="cc-settings-stack">
    <section className="cc-profile-settings">
      <header className="cc-profile-settings-heading is-compact">
        <div className="cc-profile-photo-preview" ref={photoMenuRef}>
          {preview && !removePhoto ? (
            <span className="cc-profile-avatar large"><img alt="New profile preview" src={preview} /></span>
          ) : (
            <ProfileAvatar
              className={removePhoto ? "large removed" : "large"}
              token={session.token}
              user={removePhoto ? { ...session.user, profileImageFileId: "" } : session.user}
            />
          )}
          <button
            aria-expanded={photoMenuOpen}
            aria-haspopup="menu"
            aria-label="Edit profile photo"
            className="cc-profile-photo-edit"
            onClick={() => setPhotoMenuOpen((open) => !open)}
            type="button"
          >
            <Camera size={14} strokeWidth={2.2} />
          </button>
          {photoMenuOpen && (
            <div className="cc-profile-photo-menu" role="menu">
              <button onClick={() => photoInputRef.current?.click()} role="menuitem" type="button">
                <Camera size={15} /> {hasPhoto ? "Replace photo" : "Upload photo"}
              </button>
              {hasPhoto && (
                <button
                  className="danger"
                  onClick={() => {
                    setPhoto(null);
                    setRemovePhoto(true);
                    setPhotoMenuOpen(false);
                  }}
                  role="menuitem"
                  type="button"
                >
                  <Trash2 size={15} /> Remove photo
                </button>
              )}
              <button onClick={() => setPhotoMenuOpen(false)} role="menuitem" type="button">
                <X size={15} /> Cancel
              </button>
            </div>
          )}
          <input
            accept="image/jpeg,image/png,image/webp"
            hidden
            onChange={(event) => {
              choosePhoto(event.target.files?.[0]);
              event.target.value = "";
            }}
            ref={photoInputRef}
            type="file"
          />
        </div>

        <div className="cc-profile-heading-meta">
          <strong className="cc-profile-heading-name">{name || session.user.name}</strong>
          <div className="cc-profile-trust-row">
            {customer && customerIdentityReviewPending(customer) ? (
              <span className={`cc-verify-chip ${customerVerificationStage(customer)}`}>
                In review
              </span>
            ) : (
              verificationStatus && <VerificationBadge size="sm" status={verificationStatus} />
            )}
            {customer && customerCanStartVerification(customer) && (
              <button
                className="cc-verify-chip-action"
                disabled={inviteBusy}
                onClick={() => void startVerification()}
                type="button"
              >
                <BadgeCheck size={14} />
                {inviteBusy ? "Opening…" : customerNeedsIdentityAction(customer) ? "Continue" : "Get verified"}
              </button>
            )}
          </div>
          {customer && customerIdentityReviewPending(customer) && (
            <p className="cc-profile-review-copy">We are verifying your documents. If anything further is needed, we will get back to you.</p>
          )}
        </div>
      </header>

      <form onSubmit={submit}>
        <section className="cc-profile-form-section is-account-fields">
          <div className="cc-profile-fields">
            <label><span>Display name</span><div><UserRound size={16} /><input autoComplete="name" onChange={(event) => setName(event.target.value)} value={name} /></div></label>
            <label><span>Login email / username</span><div><Mail size={16} /><input autoComplete="email" onChange={(event) => setEmail(event.target.value)} type="email" value={email} /></div></label>
            <label><span>Current password {sensitiveChange && <b>Required</b>}</span><div><KeyRound size={16} /><input autoComplete="current-password" onChange={(event) => setCurrentPassword(event.target.value)} type={showPasswords ? "text" : "password"} value={currentPassword} /></div></label>
            <label><span>New password</span><div><LockKeyhole size={16} /><input autoComplete="new-password" minLength={8} onChange={(event) => setNewPassword(event.target.value)} placeholder="At least 8 characters" type={showPasswords ? "text" : "password"} value={newPassword} /></div></label>
            <label><span>Confirm new password</span><div><Check size={16} /><input autoComplete="new-password" onChange={(event) => setConfirmPassword(event.target.value)} type={showPasswords ? "text" : "password"} value={confirmPassword} /><button aria-label={showPasswords ? "Hide passwords" : "Show passwords"} onClick={() => setShowPasswords((value) => !value)} title={showPasswords ? "Hide passwords" : "Show passwords"} type="button">{showPasswords ? <EyeOff size={16} /> : <Eye size={16} />}</button></div></label>
          </div>
          {error && <div className="cc-profile-error">{error}</div>}
          <footer><button disabled={!changed || busy} onClick={reset} type="button">Cancel</button><button className="primary" disabled={!changed || busy} type="submit">{busy ? "Saving..." : "Save"}</button></footer>
        </section>

        <section className="cc-profile-form-section cc-profile-activity">
          <header><span><Activity size={17} /></span><div><strong>Recent login activity</strong></div></header>
          <div>{(session.user.loginActivity || []).slice(0, 5).map((entry) => <article key={entry.at + entry.result}><i className={entry.result.toLowerCase().includes("fail") || entry.result.toLowerCase().includes("block") ? "warning" : "success"} /><span><strong>{entry.result}</strong><small>{entry.detail || "Account activity"}</small></span><time>{date(entry.at)}</time></article>)}{!session.user.loginActivity?.length && <p>No login activity has been recorded yet.</p>}</div>
        </section>
      </form>
    </section>

    <AndroidAppDownload />
    </div>
  );
}

function AndroidAppDownload() {
  const [ready, setReady] = useState<boolean | null>(null);
  const [bytes, setBytes] = useState(0);
  const [lastUpdated, setLastUpdated] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    const check = async () => {
      try {
        const response = await fetch(ANDROID_APK_URL, { method: "HEAD", cache: "no-store" });
        if (!alive) return;
        const length = Number(response.headers.get("content-length") || 0);
        const type = response.headers.get("content-type") || "";
        const ok = response.ok && length > 1000 && type.includes("android");
        setReady(ok);
        setBytes(ok ? length : 0);
        const lm = response.headers.get("last-modified");
        if (alive) setLastUpdated(lm);
      } catch {
        if (alive) setReady(false);
      }
    };
    check();
    const timer = window.setInterval(check, 8000);
    return () => {
      alive = false;
      window.clearInterval(timer);
    };
  }, []);

  const sizeLabel = bytes > 0 ? `${(bytes / (1024 * 1024)).toFixed(1)} MB` : "";
  const updatedLabel = formatApkBuiltAt(lastUpdated || ANDROID_APP_BUILT_AT);

  return (
    <section className="cc-profile-settings cc-mobile-download">
      <header className="cc-profile-settings-heading">
        <div>
          <span>Mobile app</span>
          <h2>Optimized Android test build</h2>
          <p>Install this APK on an Android phone to test the latest ChakuChuri mobile UI.</p>
        </div>
        <div className="cc-profile-trust">
          <Smartphone size={18} />
          <span>
            <strong>Android (64-bit)</strong>
            <small>
              Version {ANDROID_APP_VERSION}
              {sizeLabel ? ` · ${sizeLabel}` : ""}
            </small>
            <small style={{ display: "block", marginTop: 2, opacity: 0.75 }}>Updated {updatedLabel}</small>
          </span>
        </div>
      </header>

      <div className="cc-mobile-download-body">
        <div className="cc-mobile-download-copy">
          <strong>{ANDROID_APK_NAME}</strong>
          {ready === false && (
            <p className="cc-mobile-download-warn">
              No installable APK is available yet. Delete any earlier download — that file was not a real Android package, so Install will not appear.
            </p>
          )}
          {ready && (
            <p>
              Build <strong>{ANDROID_APP_VERSION}</strong>: {ANDROID_BUILD_NOTES} Allow{" "}
              <strong>Notifications</strong> and <strong>Microphone</strong> when asked.
            </p>
          )}
          {ready === null && <p>Checking whether the Android package is ready…</p>}
          <ul>
            <li>
              Phone and PC must be on the same Wi‑Fi. In the app Settings, set Backend URL to{" "}
              <code>http://192.168.1.5:8002</code>. Keep the API running on port 8002.
            </li>
            <li>Install this APK as an update to keep your saved settings. Emulators can use <code>http://10.0.2.2:8002</code>.</li>
          </ul>
        </div>
        {ready ? (
          <a className="cc-mobile-download-button" download={ANDROID_APK_NAME} href={`${ANDROID_APK_URL}?v=${ANDROID_APP_VERSION}-${ANDROID_APK_REVISION}`}>
            <Download size={18} />
            Download Android APK
          </a>
        ) : (
          <button className="cc-mobile-download-button is-disabled" disabled type="button">
            <Download size={18} />
            {ready === null ? "Checking…" : "APK not ready yet"}
          </button>
        )}
      </div>
    </section>
  );
}
