import { useEffect, useMemo, useState } from "react";
import { LoaderCircle, PhoneIncoming, PhoneOff, PhoneOutgoing, X } from "lucide-react";
import { apiPost } from "./api";
import { ProfileAvatar } from "./profile-avatar";
import { InAppCallRoom } from "./call-room";
import type { Call, Customer, Session, Workspace } from "./types";
import { findCustomerPresence, findSupportPresence, messageFromError } from "./utils";

export function DirectCallLayer({ customers, onAction, onRefresh, session, workspace }: {
  customers: Customer[];
  onAction: (message: string) => Promise<void>;
  onRefresh?: () => Promise<void>;
  session: Session;
  workspace: Workspace;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [nowMs, setNowMs] = useState(() => Date.now());
  const customerRole = session.user.role.toLowerCase() === "customer";
  const calls = useMemo(() => workspace.calls
    .filter((call) => call.status === "Ringing" || call.status === "In call")
    .filter((call) => !isRingExpired(call, nowMs))
    .filter((call) => {
      if (customerRole) return call.customerId === session.user.customerId;
      if (call.status === "Ringing") return call.initiatorRole === "Customer" || call.initiatorUserId === session.user.id;
      return call.initiatorUserId === session.user.id || call.answeredByUserId === session.user.id;
    })
    .sort((left, right) => {
      if (left.status !== right.status) return left.status === "In call" ? -1 : 1;
      return (right.updatedAt || right.createdAt).localeCompare(left.updatedAt || left.createdAt);
    }), [customerRole, nowMs, session.user.customerId, session.user.id, workspace.calls]);
  const call = calls[0];
  const incoming = call ? isIncoming(call, session) : false;
  const customerName = (call ? customers.find((customer) => customer.id === call.customerId)?.companyName : "") || "";
  const peerName = call ? (incoming ? call.initiatorName : call.recipientName || customerName || "Customer") : "";
  const peerPresence = call ? (customerRole
    ? findSupportPresence(workspace.presence, Date.now(), session.user.id)
    : findCustomerPresence(workspace.presence, call.customerId)) : undefined;
  const displayName = peerName || customerName || "ChakuChuri contact";
  const active = call?.status === "In call";

  useEffect(() => {
    if (!call || call.status !== "Ringing") return;
    return startRingtone(incoming ? "incoming" : "outgoing");
  }, [call?.id, call?.status, incoming]);

  useEffect(() => {
    if (!call || call.status !== "Ringing") return;
    const expiresAt = Date.parse(call.ringExpiresAt || "");
    if (!Number.isFinite(expiresAt)) return;
    const delay = Math.max(0, expiresAt - Date.now());
    const timer = window.setTimeout(() => {
      setNowMs(Date.now());
      void onRefresh?.();
      void onAction(incoming ? "Missed call." : "No answer.");
    }, delay + 350);
    return () => window.clearTimeout(timer);
  }, [call?.id, call?.ringExpiresAt, call?.status, incoming, onAction, onRefresh]);

  useEffect(() => {
    if (!call || call.status !== "Ringing" || !incoming) return;
    const previous = document.title;
    let highlight = false;
    const timer = window.setInterval(() => {
      highlight = !highlight;
      document.title = highlight ? `Incoming call · ${displayName}` : previous;
    }, 1100);
    return () => {
      window.clearInterval(timer);
      document.title = previous;
    };
  }, [call?.id, call?.status, displayName, incoming]);

  const update = async (status: "In call" | "Declined" | "Cancelled") => {
    if (!call || busy) return;
    setBusy(true);
    setError("");
    try {
      await apiPost<Call>(`/api/workflow/calls/${call.id}/status`, { status }, session.token);
      await onAction(status === "In call" ? "Call connected." : status === "Declined" ? "Call declined." : "Call cancelled.");
    } catch (caught) {
      setError(messageFromError(caught, "Call action failed."));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    if (!call || call.status !== "Ringing") return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || busy) return;
      event.preventDefault();
      void update(incoming ? "Declined" : "Cancelled");
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, call?.id, call?.status, incoming]);

  if (!call) return null;

  return (
    <div className="cc-direct-call-backdrop">
      <section aria-label="Audio call" aria-modal="true" className={`cc-direct-call-panel ${incoming ? "incoming" : "outgoing"} ${active ? "active" : ""}`} role="dialog">
        {active ? (
          <>
            <div className="cc-active-call-identity">
              <ProfileAvatar className="cc-call-avatar active" token={session.token} user={{ name: displayName, profileImageFileId: peerPresence?.profileImageFileId }} />
              <small>In-app audio</small>
              <h2>{displayName}</h2>
            </div>
            <InAppCallRoom autoStart call={call} mode={customerRole ? "customer" : "admin"} onAction={onAction} session={session} />
          </>
        ) : (
          <>
            <div className="cc-ringing-visual">
              <span className="cc-ringing-wave wave-one" />
              <span className="cc-ringing-wave wave-two" />
              <ProfileAvatar className="cc-call-avatar large" token={session.token} user={{ name: displayName, profileImageFileId: peerPresence?.profileImageFileId }} />
            </div>
            <div className="cc-ringing-copy">
              <span>{incoming ? "Incoming call" : "Calling"}</span>
              <h2>{displayName}</h2>
              <p>{incoming ? "Answer to start encrypted audio" : "Waiting for an answer…"}</p>
              {error && <em>{error}</em>}
            </div>
            <div className="cc-ringing-actions">
              <button aria-label={incoming ? "Decline call" : "Cancel call"} className="cc-call-round-button decline" disabled={busy} onClick={() => void update(incoming ? "Declined" : "Cancelled")} title={incoming ? "Decline (Esc)" : "Cancel (Esc)"} type="button">
                {busy ? <LoaderCircle className="cc-spin" size={24} /> : incoming ? <PhoneOff size={24} /> : <X size={25} />}
                <span>{incoming ? "Decline" : "Cancel"}</span>
              </button>
              {incoming && (
                <button aria-label="Answer call" className="cc-call-round-button answer" disabled={busy} onClick={() => void update("In call")} title="Answer call" type="button">
                  <PhoneIncoming size={24} /><span>Answer</span>
                </button>
              )}
              {!incoming && <PhoneOutgoing aria-hidden="true" className="cc-outgoing-call-mark" size={28} />}
            </div>
          </>
        )}
      </section>
    </div>
  );
}

function isIncoming(call: Call, session: Session) {
  if (call.initiatorUserId) return call.initiatorUserId !== session.user.id;
  const currentRole = session.user.role.toLowerCase() === "customer" ? "customer" : "admin";
  return call.initiatorRole.toLowerCase() !== currentRole;
}

function isRingExpired(call: Call, nowMs: number) {
  if (call.status !== "Ringing" || !call.ringExpiresAt) return false;
  const expiresAt = Date.parse(call.ringExpiresAt);
  return Number.isFinite(expiresAt) && expiresAt <= nowMs;
}

type AudioWindow = Window & typeof globalThis & { webkitAudioContext?: typeof AudioContext };
let sharedAudioContext: AudioContext | null = null;

function startRingtone(direction: "incoming" | "outgoing") {
  let stopped = false;
  const AudioContextConstructor = window.AudioContext || (window as AudioWindow).webkitAudioContext;
  const pulse = () => {
    if (stopped || !AudioContextConstructor) return;
    try {
      sharedAudioContext ||= new AudioContextConstructor();
      const context = sharedAudioContext;
      const play = () => {
        const now = context.currentTime;
        if (direction === "incoming") {
          const bursts = [0, 0.55];
          bursts.forEach((offset) => {
            ([784, 988] as const).forEach((frequency, voice) => {
              const oscillator = context.createOscillator();
              const gain = context.createGain();
              const startAt = now + offset;
              oscillator.type = voice === 0 ? "triangle" : "sine";
              oscillator.frequency.value = frequency;
              gain.gain.setValueAtTime(0.0001, startAt);
              gain.gain.exponentialRampToValueAtTime(0.1, startAt + 0.03);
              gain.gain.exponentialRampToValueAtTime(0.0001, startAt + 0.38);
              oscillator.connect(gain);
              gain.connect(context.destination);
              oscillator.start(startAt);
              oscillator.stop(startAt + 0.42);
            });
          });
        } else {
          ([440, 480] as const).forEach((frequency) => {
            const oscillator = context.createOscillator();
            const gain = context.createGain();
            oscillator.type = "sine";
            oscillator.frequency.value = frequency;
            gain.gain.setValueAtTime(0.0001, now);
            gain.gain.exponentialRampToValueAtTime(0.055, now + 0.05);
            gain.gain.setValueAtTime(0.055, now + 0.85);
            gain.gain.exponentialRampToValueAtTime(0.0001, now + 1.15);
            oscillator.connect(gain);
            gain.connect(context.destination);
            oscillator.start(now);
            oscillator.stop(now + 1.2);
          });
        }
      };
      if (context.state === "suspended") void context.resume().then(play).catch(() => undefined);
      else play();
    } catch {
      // Visual ringing remains available when autoplay audio is blocked.
    }
  };
  pulse();
  const interval = window.setInterval(pulse, direction === "incoming" ? 2200 : 3200);
  if (direction === "incoming") navigator.vibrate?.([180, 120, 180, 120, 180]);
  return () => {
    stopped = true;
    window.clearInterval(interval);
    navigator.vibrate?.(0);
  };
}
