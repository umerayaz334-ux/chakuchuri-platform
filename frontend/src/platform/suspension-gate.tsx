import { useState } from "react";
import { AlertTriangle, CheckCircle2, Headphones, LogOut, Mail, SendHorizontal, ShieldAlert } from "lucide-react";
import { apiPost } from "./api";
import type { Session, User } from "./types";
import { date, messageFromError } from "./utils";

export function SuspensionGate({ session, onSignOut, onUserChange }: { session: Session; onSignOut: () => void; onUserChange: (user: User) => void }) {
  const [message, setMessage] = useState(session.user.suspensionContactMessage || "Please review my account and let me know what is required to restore access.");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [sent, setSent] = useState(Boolean(session.user.suspensionContactAt));

  const contact = async () => {
    if (!message.trim() || busy) return;
    setBusy(true);
    setError("");
    try {
      const user = await apiPost<User>("/api/auth/suspension/contact", { message: message.trim() }, session.token);
      onUserChange(user);
      setSent(true);
    } catch (caught) {
      setError(messageFromError(caught, "Your request could not be sent."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="cc-suspension-page">
      <header><div className="cc-suspension-brand"><span>CC</span><strong>ChakuChuri.pk</strong></div><button onClick={onSignOut} type="button"><LogOut size={17} /> Sign out</button></header>
      <section className="cc-suspension-shell">
        <aside><span><ShieldAlert size={30} /></span><small>Account status</small><strong>Temporarily suspended</strong><p>Your login still works, but platform actions are frozen until an administrator restores access.</p><div><span>{initials(session.user.name)}</span><p><strong>{session.user.name}</strong><small>{session.user.email}</small></p></div></aside>
        <div className="cc-suspension-main">
          <div className="cc-suspension-title"><span><AlertTriangle size={20} /></span><div><small>Notice from your administrator</small><h1>Access needs attention</h1></div></div>
          <blockquote>{session.user.suspensionNotice || "Your account is temporarily paused while we review an account matter. Contact the ChakuChuri team below for help."}</blockquote>
          <section className="cc-suspension-contact">
            <header><Headphones size={19} /><div><strong>Contact the admin team</strong><small>Send one clear message about the issue. It will appear in Users &amp; Access.</small></div></header>
            <label><span>Your message</span><textarea maxLength={1000} onChange={(event) => setMessage(event.target.value)} rows={4} value={message} /></label>
            {error && <p className="cc-suspension-error">{error}</p>}
            {sent && <div className="cc-suspension-sent"><CheckCircle2 size={17} /><span><strong>Review request sent</strong><small>{session.user.suspensionContactAt ? `Last sent ${date(session.user.suspensionContactAt)}` : "The admin team has been notified."}</small></span></div>}
            <button className="cc-suspension-send" disabled={busy || !message.trim()} onClick={contact} type="button"><SendHorizontal size={17} /> {busy ? "Sending..." : sent ? "Send updated request" : "Contact administrator"}</button>
          </section>
          <footer><Mail size={16} /><span>Only this support request and sign out are available while your account is suspended.</span></footer>
        </div>
      </section>
    </main>
  );
}

function initials(value: string) {
  return value.trim().split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase() || "").join("") || "CC";
}
