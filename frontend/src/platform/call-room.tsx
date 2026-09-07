import { useEffect, useRef, useState } from "react";
import { Mic, MicOff, PhoneOff, ShieldCheck, Volume2, WifiOff } from "lucide-react";
import { apiGet, apiPost } from "./api";
import type { Call, CallSignal, Session } from "./types";

type CallConfig = {
  iceServers?: RTCIceServer[];
  pollMs?: number;
  signalingMode?: string;
  fallbackSignalingMode?: string;
  websocketPath?: string;
  redisFanoutEnabled?: boolean;
  turnConfigured?: boolean;
};

type CallSocketEvent = {
  type: "ready" | "signal" | "signal-sent" | "signals" | "presence" | "call-ended" | "error";
  mode?: string;
  message?: string;
  call?: Call;
  signal?: CallSignal;
  signals?: CallSignal[];
};

function signalPayload(value: RTCSessionDescriptionInit | RTCIceCandidate | { muted?: boolean; reason?: string }) {
  return JSON.parse(JSON.stringify(value));
}

function normalizeIceServers(servers?: RTCIceServer[]): RTCIceServer[] {
  if (!servers?.length) {
    return [
      { urls: ["stun:stun.l.google.com:19302", "stun:stun1.l.google.com:19302"] }
    ];
  }
  return servers.map((server) => {
    const urls = Array.isArray(server.urls) ? server.urls : [server.urls];
    const next: RTCIceServer = { urls: urls.filter(Boolean) };
    if (server.username) next.username = server.username;
    if (server.credential) next.credential = String(server.credential);
    return next;
  });
}

function socketURL(path: string, callId: string, token: string) {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const cleanPath = path.replace(/\/$/, "");
  return `${protocol}//${window.location.host}${cleanPath}/${encodeURIComponent(callId)}?token=${encodeURIComponent(token)}`;
}

export function InAppCallRoom({
  call,
  session,
  mode,
  autoStart = false,
  onAction
}: {
  call: Call;
  session: Session;
  mode: "admin" | "customer";
  autoStart?: boolean;
  onAction: (message: string) => Promise<void>;
}) {
  const [phase, setPhase] = useState(call.status === "In call" ? "Ready to join" : call.status);
  const [muted, setMuted] = useState(false);
  const [connected, setConnected] = useState(false);
  const [microphoneActive, setMicrophoneActive] = useState(false);
  const [playbackBlocked, setPlaybackBlocked] = useState(false);
  const [connectionQuality, setConnectionQuality] = useState("Connecting");
  const [transport, setTransport] = useState("Preparing");
  const [notice, setNotice] = useState("");
  const [elapsed, setElapsed] = useState(0);
  const peerRef = useRef<RTCPeerConnection | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const pollRef = useRef<number | null>(null);
  const heartbeatRef = useRef<number | null>(null);
  const reconnectRef = useRef<number | null>(null);
  const statsRef = useRef<number | null>(null);
  const configRef = useRef<CallConfig>({ pollMs: 1200 });
  const lastSignalNoRef = useRef(0);
  const handledRef = useRef<Set<string>>(new Set());
  const pendingIceRef = useRef<RTCIceCandidateInit[]>([]);
  const stoppingRef = useRef(false);
  const startingRef = useRef(false);
  const recoveringConnectionRef = useRef(false);
  const recoveringMicrophoneRef = useRef(false);
  const catchUpRef = useRef<number | null>(null);

  const isAdmin = mode === "admin";

  useEffect(() => {
    return () => stopLocal();
  }, []);

  useEffect(() => {
    const recoverVisibleAudio = () => {
      if (document.visibilityState !== "visible" || stoppingRef.current) return;
      const track = streamRef.current?.getAudioTracks()[0];
      if (peerRef.current && (!track || track.readyState === "ended")) void recoverMicrophone();
      if (remoteAudioRef.current?.srcObject) void resumeRemoteAudio();
    };
    document.addEventListener("visibilitychange", recoverVisibleAudio);
    return () => document.removeEventListener("visibilitychange", recoverVisibleAudio);
  }, []);

  const sendSocketMessage = (message: unknown) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) return false;
    socket.send(JSON.stringify(message));
    return true;
  };

  const sendSignal = async (signalType: string, payload: unknown) => {
    if (sendSocketMessage({ type: "signal", signalType, payload })) return;
    await apiPost<CallSignal>(`/api/workflow/calls/${call.id}/signals`, { signalType, payload }, session.token);
  };

  const requestSignalCatchUp = () => {
    if (sendSocketMessage({ type: "catch-up", after: lastSignalNoRef.current })) return;
    void pollSignals();
  };

  const startSignalCatchUp = () => {
    if (catchUpRef.current !== null) return;
    catchUpRef.current = window.setInterval(requestSignalCatchUp, 2000);
  };

  const ensureMediaSupport = () => {
    if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) {
      throw new Error("Microphone access is blocked on this Wi-Fi address. Open the HTTPS device address shown in Settings > Device connection, then allow microphone access.");
    }
    if (typeof RTCPeerConnection === "undefined") {
      throw new Error("This browser does not support in-app audio calls. Update the browser and try again.");
    }
  };

  const wireLocalTrack = (track: MediaStreamTrack) => {
    track.onended = () => {
      setMicrophoneActive(false);
      if (!stoppingRef.current) void recoverMicrophone();
    };
    track.onmute = () => setMicrophoneActive(false);
    track.onunmute = () => setMicrophoneActive(true);
  };

  const openMicrophone = async () => {
    ensureMediaSupport();
    const mediaDevices = navigator.mediaDevices;
    if (!mediaDevices?.getUserMedia) throw new Error("Microphone access is unavailable in this browser.");
    const stream = await mediaDevices.getUserMedia({
      audio: {
        autoGainControl: true,
        channelCount: 1,
        echoCancellation: true,
        noiseSuppression: true
      },
      video: false
    });
    const track = stream.getAudioTracks()[0];
    if (!track) {
      stream.getTracks().forEach((item) => item.stop());
      throw new Error("No microphone was found on this device.");
    }
    wireLocalTrack(track);
    streamRef.current = stream;
    setMicrophoneActive(track.readyState === "live");
    return stream;
  };

  const recoverMicrophone = async () => {
    if (recoveringMicrophoneRef.current || stoppingRef.current || !peerRef.current) return;
    recoveringMicrophoneRef.current = true;
    try {
      const previous = streamRef.current;
      const stream = await openMicrophone();
      const track = stream.getAudioTracks()[0];
      const sender = peerRef.current.getSenders().find((item) => item.track?.kind === "audio");
      if (sender) await sender.replaceTrack(track);
      else peerRef.current.addTrack(track, stream);
      previous?.getTracks().forEach((item) => item.stop());
      setNotice("");
    } catch (error) {
      setMicrophoneActive(false);
      setNotice(describeCallError(error));
    } finally {
      recoveringMicrophoneRef.current = false;
    }
  };

  const playRemoteAudio = async () => {
    const audio = remoteAudioRef.current;
    if (!audio?.srcObject) return;
    audio.muted = false;
    audio.volume = 1;
    try {
      await audio.play();
      setPlaybackBlocked(false);
    } catch {
      setPlaybackBlocked(true);
    }
  };

  const resumeRemoteAudio = async () => {
    await playRemoteAudio();
  };

  const scheduleReconnect = (delay = 1500) => {
    if (stoppingRef.current || reconnectRef.current !== null) return;
    reconnectRef.current = window.setTimeout(() => {
      reconnectRef.current = null;
      void restartConnection();
    }, delay);
  };

  const attachRemoteTrack = (event: RTCTrackEvent) => {
    const audio = remoteAudioRef.current;
    if (!audio) return;
    audio.srcObject = event.streams[0] || new MediaStream([event.track]);
    event.track.onunmute = () => {
      setConnectionQuality("Good");
      void playRemoteAudio();
    };
    event.track.onmute = () => {
      if (!stoppingRef.current) setConnectionQuality("Audio interrupted");
    };
    event.track.onended = () => {
      if (!stoppingRef.current) {
        setConnectionQuality("Reconnecting");
        scheduleReconnect(300);
      }
    };
    void playRemoteAudio();
  };

  const flushPendingIce = async (peer: RTCPeerConnection) => {
    if (!peer.remoteDescription || pendingIceRef.current.length === 0) return;
    const pending = pendingIceRef.current.splice(0);
    for (const candidate of pending) await peer.addIceCandidate(candidate).catch(() => undefined);
  };

  const makeRestartOffer = async (peer: RTCPeerConnection) => {
    if (peer.signalingState !== "stable") {
      scheduleReconnect(1200);
      return;
    }
    peer.restartIce();
    const offer = await peer.createOffer({ iceRestart: true });
    await peer.setLocalDescription(offer);
    await sendSignal("offer", signalPayload(offer));
  };

  const restartConnection = async () => {
    const peer = peerRef.current;
    if (!peer || stoppingRef.current || peer.signalingState === "closed" || recoveringConnectionRef.current) return;
    recoveringConnectionRef.current = true;
    setPhase("Reconnecting");
    setConnectionQuality("Reconnecting");
    try {
      if (isAdmin) await makeRestartOffer(peer);
      else await sendSignal("reconnect-request", { reason: "connection-interrupted" });
    } catch {
      setNotice("Audio connection was interrupted. Reconnecting automatically...");
      scheduleReconnect(1800);
    } finally {
      window.setTimeout(() => {
        recoveringConnectionRef.current = false;
      }, 1800);
    }
  };

  const handleConnectionState = (peer: RTCPeerConnection) => {
    const state = peer.connectionState;
    if (state === "connected") {
      if (reconnectRef.current !== null) window.clearTimeout(reconnectRef.current);
      reconnectRef.current = null;
      recoveringConnectionRef.current = false;
      setConnected(true);
      setPhase("Connected");
      setConnectionQuality("Good");
      setNotice("");
      void playRemoteAudio();
      return;
    }
    if (state === "disconnected" || state === "failed") {
      setConnected(false);
      setPhase("Reconnecting");
      setConnectionQuality("Interrupted");
      scheduleReconnect(state === "failed" ? 100 : 1200);
      return;
    }
    if (state === "connecting" || state === "new") {
      setConnected(false);
      setPhase("Connecting");
      setConnectionQuality("Connecting");
    }
  };

  const startQualityMonitor = (peer: RTCPeerConnection) => {
    if (statsRef.current !== null) window.clearInterval(statsRef.current);
    statsRef.current = window.setInterval(async () => {
      if (peer.connectionState !== "connected") return;
      try {
        const reports = await peer.getStats();
        let packetsReceived: number | null = null;
        let lost = 0;
        let jitter = 0;
        reports.forEach((rawReport) => {
          const report = rawReport as RTCStats & { kind?: string; packetsReceived?: number; packetsLost?: number; jitter?: number };
          if (report.type === "inbound-rtp" && report.kind === "audio") {
            packetsReceived = Number(report.packetsReceived || 0);
            lost = Number(report.packetsLost || 0);
            jitter = Number(report.jitter || 0);
          }
        });
        if (packetsReceived === null) return;
        const total = packetsReceived + Math.max(0, lost);
        const lossPercent = total > 0 ? (Math.max(0, lost) / total) * 100 : 0;
        setConnectionQuality(lossPercent > 8 || jitter > 0.08 ? "Unstable" : lossPercent > 3 || jitter > 0.04 ? "Fair" : "Good");
      } catch {
        // Statistics are advisory; peer-state recovery remains active.
      }
    }, 3500);
  };

  const makePeer = async () => {
    ensureMediaSupport();
    const config = await apiGet<CallConfig>("/api/workflow/calls/config", session.token).catch((): CallConfig => ({ pollMs: 1200 }));
    configRef.current = config;
    const stream = await openMicrophone();
    const iceServers = normalizeIceServers(config.iceServers);
    const peer = new RTCPeerConnection({
      iceCandidatePoolSize: 4,
      iceServers
    });

    peer.onicecandidate = (event) => {
      if (event.candidate) void sendSignal("ice", signalPayload(event.candidate)).catch(() => undefined);
    };
    peer.ontrack = attachRemoteTrack;
    peer.onconnectionstatechange = () => handleConnectionState(peer);
    peer.oniceconnectionstatechange = () => {
      if (peer.iceConnectionState === "failed") scheduleReconnect(100);
      if (peer.iceConnectionState === "disconnected") scheduleReconnect(1200);
    };

    stream.getAudioTracks().forEach((track) => peer.addTrack(track, stream));
    peerRef.current = peer;
    startQualityMonitor(peer);

    const websocketReady = await connectSocket(config);
    if (!websocketReady) startPolling(config.pollMs);
    else startSignalCatchUp();
    heartbeatRef.current = window.setInterval(() => {
      if (sendSocketMessage({ type: "heartbeat" })) return;
      void apiPost<Call>(`/api/workflow/calls/${call.id}/heartbeat`, {}, session.token).catch(() => undefined);
    }, 5000);
    return peer;
  };

  const connectSocket = async (config: CallConfig) => {
    if (config.signalingMode !== "websocket" || !config.websocketPath || typeof WebSocket === "undefined") {
      setTransport("HTTP fallback");
      return false;
    }

    return new Promise<boolean>((resolve) => {
      const socket = new WebSocket(socketURL(config.websocketPath || "/api/realtime/calls", call.id, session.token));
      let settled = false;
      const finish = (ready: boolean) => {
        if (settled) return;
        settled = true;
        window.clearTimeout(timeout);
        resolve(ready);
      };
      const timeout = window.setTimeout(() => {
        setTransport("HTTP fallback");
        socket.close();
        finish(false);
      }, 2500);

      socket.onopen = () => {
        socketRef.current = socket;
        setTransport(config.redisFanoutEnabled ? "WebSocket / Redis" : "WebSocket");
        sendSocketMessage({ type: "catch-up", after: lastSignalNoRef.current });
        finish(true);
      };
      socket.onerror = () => {
        setTransport("HTTP fallback");
        finish(false);
      };
      socket.onmessage = (event) => {
        void handleSocketEvent(event).catch((error) => setNotice(error instanceof Error ? error.message : "Realtime event could not be handled."));
      };
      socket.onclose = () => {
        if (socketRef.current === socket) socketRef.current = null;
        if (!stoppingRef.current && peerRef.current && call.status === "In call") {
          setTransport("HTTP fallback");
          startPolling(configRef.current.pollMs);
        }
      };
    });
  };

  const startPolling = (pollMs = 1200) => {
    if (pollRef.current !== null) return;
    setTransport((current) => (current.startsWith("WebSocket") ? current : "HTTP fallback"));
    pollRef.current = window.setInterval(() => void pollSignals(), Number(pollMs || 1200));
  };

  const start = async () => {
    if (startingRef.current || peerRef.current) return;
    startingRef.current = true;
    try {
      stoppingRef.current = false;
      setNotice("");
      setPhase("Opening microphone");
      setTransport("Connecting");
      const peer = await makePeer();
      if (isAdmin) {
        const offer = await peer.createOffer();
        await peer.setLocalDescription(offer);
        await sendSignal("offer", signalPayload(offer));
      }
      await pollSignals();
      if (peer.connectionState !== "connected") setPhase(isAdmin ? "Ringing…" : "Connecting…");
    } catch (error) {
      setNotice(describeCallError(error));
      setPhase("Could not start");
      setTransport("Unavailable");
      stopLocal();
    } finally {
      startingRef.current = false;
    }
  };

  useEffect(() => {
    if (!autoStart || call.status !== "In call") return;
    const timer = window.setTimeout(() => {
      if (!peerRef.current) void start();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [autoStart, call.id, call.status]);

  useEffect(() => {
    if (!call.startedAt || call.status !== "In call") return;
    const tick = () => setElapsed(Math.max(0, Math.floor((Date.now() - Date.parse(call.startedAt || "")) / 1000)));
    tick();
    const timer = window.setInterval(tick, 1000);
    return () => window.clearInterval(timer);
  }, [call.startedAt, call.status]);

  const pollSignals = async () => {
    const peer = peerRef.current;
    if (!peer) return;
    const signals = await apiGet<CallSignal[]>(`/api/workflow/calls/${call.id}/signals?after=${lastSignalNoRef.current}`, session.token).catch(() => []);
    for (const signal of signals) await handleIncomingSignal(signal);
  };

  const handleSocketEvent = async (message: MessageEvent) => {
    const event = JSON.parse(String(message.data)) as CallSocketEvent;
    if (event.type === "ready") setTransport(event.mode === "websocket" ? "WebSocket" : event.mode || "Realtime");
    if (event.type === "error" && event.message) setNotice(event.message);
    if (event.type === "signal" && event.signal) await handleIncomingSignal(event.signal);
    if (event.type === "signal-sent" && event.signal) lastSignalNoRef.current = Math.max(lastSignalNoRef.current, event.signal.signalNo || 0);
    if (event.type === "signals" && event.signals) {
      for (const signal of event.signals) await handleIncomingSignal(signal);
    }
    if (event.type === "call-ended") {
      if (stoppingRef.current) return;
      setPhase("Call ended");
      setNotice("The call has ended.");
      stopLocal();
      await onAction("Call ended.");
    }
  };

  const handleIncomingSignal = async (signal: CallSignal) => {
    const peer = peerRef.current;
    if (!peer || signal.senderId === session.user.id) return;
    lastSignalNoRef.current = Math.max(lastSignalNoRef.current, signal.signalNo || 0);
    if (handledRef.current.has(signal.id)) return;
    handledRef.current.add(signal.id);

    if (signal.signalType === "offer") {
      if (peer.signalingState === "have-local-offer") await peer.setLocalDescription({ type: "rollback" });
      await peer.setRemoteDescription(signal.payload as RTCSessionDescriptionInit);
      await flushPendingIce(peer);
      const answer = await peer.createAnswer();
      await peer.setLocalDescription(answer);
      await sendSignal("answer", signalPayload(answer));
    }
    if (signal.signalType === "answer" && peer.signalingState === "have-local-offer") {
      await peer.setRemoteDescription(signal.payload as RTCSessionDescriptionInit);
      await flushPendingIce(peer);
    }
    if (signal.signalType === "ice" || signal.signalType === "candidate") {
      const candidate = signal.payload as RTCIceCandidateInit;
      if (peer.remoteDescription) await peer.addIceCandidate(candidate).catch(() => undefined);
      else pendingIceRef.current.push(candidate);
    }
    if (signal.signalType === "media-state" && typeof signal.payload?.muted === "boolean") {
      setNotice(signal.payload.muted ? "The other person muted their microphone." : "");
    }
    if (signal.signalType === "reconnect-request" && isAdmin) {
      recoveringConnectionRef.current = false;
      await restartConnection();
    }
  };

  const toggleMute = async () => {
    const next = !muted;
    streamRef.current?.getAudioTracks().forEach((track) => {
      track.enabled = !next;
    });
    setMuted(next);
    await sendSignal("media-state", { muted: next }).catch(() => undefined);
  };

  const end = async () => {
    try {
      if (sendSocketMessage({ type: "end" })) await new Promise((resolve) => window.setTimeout(resolve, 150));
      else await apiPost<Call>(`/api/workflow/calls/${call.id}/end`, {}, session.token);
    } finally {
      stopLocal();
      await onAction("Call ended.");
    }
  };

  const stopLocal = () => {
    stoppingRef.current = true;
    if (pollRef.current !== null) window.clearInterval(pollRef.current);
    if (catchUpRef.current !== null) window.clearInterval(catchUpRef.current);
    if (heartbeatRef.current !== null) window.clearInterval(heartbeatRef.current);
    if (reconnectRef.current !== null) window.clearTimeout(reconnectRef.current);
    if (statsRef.current !== null) window.clearInterval(statsRef.current);
    pollRef.current = null;
    catchUpRef.current = null;
    heartbeatRef.current = null;
    reconnectRef.current = null;
    statsRef.current = null;
    const socket = socketRef.current;
    if (socket) {
      socket.onclose = null;
      socket.close();
      socketRef.current = null;
    }
    streamRef.current?.getTracks().forEach((track) => track.stop());
    streamRef.current = null;
    peerRef.current?.close();
    peerRef.current = null;
    pendingIceRef.current = [];
    setConnected(false);
    setMicrophoneActive(false);
  };

  const qualityClass = connectionQuality.toLowerCase().replace(/\s+/g, "-");

  return (
    <div className={`cc-in-app-call-room ${connected ? "is-connected" : ""}`}>
      <div className="cc-call-security"><ShieldCheck size={15} /><span>Private in-app audio</span></div>
      <div className="cc-call-live-status" aria-live="polite">
        <i className={connected ? "is-live" : ""} />
        <strong>{friendlyPhase(phase)}</strong>
        <time>{formatElapsed(elapsed)}</time>
      </div>
      <audio autoPlay playsInline ref={remoteAudioRef} />
      {notice && <div className="cc-call-notice"><WifiOff size={17} /><span>{notice}</span></div>}
      {playbackBlocked && (
        <button className="cc-call-audio-recovery" onClick={() => void resumeRemoteAudio()} type="button">
          <Volume2 size={18} /> Tap to hear the call
        </button>
      )}
      <div className="cc-call-health">
        <span className={`quality-${qualityClass}`}><i /> {connected ? `${connectionQuality} audio` : connectionQuality}</span>
        <small>{microphoneActive ? (muted ? "You are muted" : "Mic is on") : "Microphone unavailable"}</small>
      </div>
      <div className="cc-call-room-controls">
        {!autoStart && (
          <button className="cc-call-join-button" disabled={Boolean(peerRef.current) || call.status !== "In call"} onClick={() => void start()} type="button">
            <Mic size={18} /> Join call
          </button>
        )}
        <button aria-label={muted ? "Unmute microphone" : "Mute microphone"} className={`cc-call-control-button ${muted ? "is-muted" : ""}`} disabled={!microphoneActive} onClick={() => void toggleMute()} title={muted ? "Unmute microphone" : "Mute microphone"} type="button">
          {muted ? <MicOff size={23} /> : <Mic size={23} />}
          <span>{muted ? "Unmute" : "Mute"}</span>
        </button>
        <button aria-label="End call" className="cc-call-control-button end" onClick={() => void end()} title="End call" type="button">
          <PhoneOff size={24} />
          <span>End</span>
        </button>
      </div>
      <footer>{friendlyTransport(transport)}</footer>
    </div>
  );
}

function friendlyPhase(phase: string) {
  if (phase === "Ready to join") return "Ready";
  if (phase === "Opening microphone") return "Starting mic…";
  if (phase === "Could not start") return "Could not connect";
  return phase;
}

function friendlyTransport(value: string) {
  if (value.startsWith("WebSocket")) return "Live connection";
  if (value.includes("HTTP") || value.includes("fallback") || value.includes("polling")) return "Backup connection";
  if (value === "Connecting" || value === "Preparing") return "Connecting…";
  if (value === "Unavailable") return "Connection unavailable";
  return value;
}

function describeCallError(error: unknown) {
  if (error instanceof DOMException && error.name === "NotAllowedError") {
    return "Microphone permission was denied. Allow microphone access in the browser and join again.";
  }
  if (error instanceof DOMException && error.name === "NotFoundError") return "No microphone was found on this device.";
  return error instanceof Error ? error.message : "Microphone permission or call setup failed.";
}

function formatElapsed(value: number) {
  const minutes = Math.floor(value / 60);
  return String(minutes).padStart(2, "0") + ":" + String(value % 60).padStart(2, "0");
}