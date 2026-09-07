import { type FormEvent, useEffect, useMemo, useState } from "react";
import { Check, Copy, ExternalLink, Languages, Palette, RefreshCw, RotateCcw, Save, Wifi } from "lucide-react";
import { apiGet, apiPost } from "./api";
import { normalizePlatformSettings, themePresets } from "./preferences";
import type { PlatformConnectionInfo, PlatformSettings, Session } from "./types";
import { messageFromError } from "./utils";

export function PlatformSettingsPanel({
  settings,
  session,
  onSaved,
  onAction
}: {
  settings: PlatformSettings;
  session: Session;
  onSaved: (settings: PlatformSettings) => void;
  onAction: (message: string) => Promise<void>;
}) {
  const [draft, setDraft] = useState(() => normalizePlatformSettings(settings));
  const [busy, setBusy] = useState(false);
  const [connection, setConnection] = useState<PlatformConnectionInfo | null>(null);
  const [connectionError, setConnectionError] = useState("");
  const [connectionLoading, setConnectionLoading] = useState(true);
  const [connectionRefresh, setConnectionRefresh] = useState(0);
  const [copiedConnection, setCopiedConnection] = useState("");
  const canManage = ["owner", "admin"].includes(session.user.role.toLowerCase()) || session.user.permissions?.includes("platform.settings.manage");
  const changed = useMemo(() => comparable(draft) !== comparable(settings), [draft, settings]);

  useEffect(() => setDraft(normalizePlatformSettings(settings)), [settings]);

  useEffect(() => {
    let active = true;
    setConnectionLoading(true);
    const loadConnection = async () => {
      try {
        const next = await apiGet<PlatformConnectionInfo>("/api/platform/connection", session.token);
        if (!active) return;
        setConnection(next);
        setConnectionError("");
      } catch (error) {
        if (active) setConnectionError(messageFromError(error, "Connection details are unavailable."));
      } finally {
        if (active) setConnectionLoading(false);
      }
    };

    void loadConnection();
    const timer = window.setInterval(loadConnection, 5000);
    const handleOnline = () => void loadConnection();
    window.addEventListener("online", handleOnline);
    return () => {
      active = false;
      window.clearInterval(timer);
      window.removeEventListener("online", handleOnline);
    };
  }, [connectionRefresh, session.token]);

  const copyConnection = async (value: string, key: string) => {
    if (!(await copyText(value))) {
      setConnectionError("The address could not be copied.");
      return;
    }
    setCopiedConnection(key);
    window.setTimeout(() => setCopiedConnection((current) => current === key ? "" : current), 1600);
  };

  const selectPreset = (name: string) => {
    const preset = themePresets.find((item) => item.name === name);
    if (preset) setDraft((current) => ({ ...current, ...preset }));
  };

  const setColor = (key: "primaryColor" | "accentColor" | "surfaceColor", value: string) => {
    setDraft((current) => ({ ...current, [key]: value, themePreset: "Custom" }));
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!canManage) return;
    setBusy(true);
    try {
      const saved = await apiPost<PlatformSettings>("/api/platform/settings", draft, session.token);
      onSaved(saved);
      setDraft(saved);
      await onAction("Platform settings saved.");
    } catch (error) {
      await onAction(messageFromError(error, "Platform settings could not be saved."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="cc-platform-settings" onSubmit={submit}>
      <header className="cc-settings-heading">
        <div>
          <span className="cc-kicker">Platform</span>
          <h2>Brand and language</h2>
        </div>
        <div className="cc-settings-scope" aria-label="Theme coverage">
          <span>Website</span>
          <span>Admin portal</span>
          <span>Customer portal</span>
        </div>
      </header>

      <section className="cc-settings-section">
        <header>
          <span className="cc-settings-section-icon"><Palette size={19} /></span>
          <div>
            <h3>Appearance</h3>
            <small>{draft.themePreset} theme</small>
          </div>
        </header>

        <div className="cc-theme-options" role="radiogroup" aria-label="Theme">
          {themePresets.map((preset) => {
            const selected = draft.themePreset === preset.name;
            return (
              <button
                aria-checked={selected}
                className={selected ? "selected" : ""}
                disabled={!canManage}
                key={preset.name}
                onClick={() => selectPreset(preset.name)}
                role="radio"
                type="button"
              >
                <span className="cc-theme-swatches">
                  <i style={{ background: preset.primaryColor }} />
                  <i style={{ background: preset.accentColor }} />
                  <i style={{ background: preset.surfaceColor }} />
                </span>
                <strong>{preset.name}</strong>
                {selected && <Check size={15} />}
              </button>
            );
          })}
        </div>

        <div className="cc-color-controls">
          <ColorControl disabled={!canManage} label="Primary color" onChange={(value) => setColor("primaryColor", value)} value={draft.primaryColor} />
          <ColorControl disabled={!canManage} label="Accent color" onChange={(value) => setColor("accentColor", value)} value={draft.accentColor} />
          <ColorControl disabled={!canManage} label="Surface color" onChange={(value) => setColor("surfaceColor", value)} value={draft.surfaceColor} />
        </div>

        <div
          className="cc-theme-preview"
          style={{
            "--preview-primary": draft.primaryColor,
            "--preview-accent": draft.accentColor,
            "--preview-surface": draft.surfaceColor
          } as React.CSSProperties}
        >
          <span>Live preview</span>
          <div>
            <i />
            <strong>ChakuChuri.pk</strong>
            <button type="button">Primary action</button>
          </div>
        </div>
      </section>

      <section className="cc-settings-section">
        <header>
          <span className="cc-settings-section-icon"><Languages size={19} /></span>
          <div>
            <h3>Language</h3>
            <small>English / اردو</small>
          </div>
        </header>

        <div className="cc-language-settings">
          <div>
            <span>Default language</span>
            <div className="cc-segmented" role="radiogroup" aria-label="Default language">
              <button aria-checked={draft.defaultLanguage === "en"} className={draft.defaultLanguage === "en" ? "active" : ""} disabled={!canManage} onClick={() => setDraft((current) => ({ ...current, defaultLanguage: "en" }))} role="radio" type="button">English</button>
              <button aria-checked={draft.defaultLanguage === "ur"} className={draft.defaultLanguage === "ur" ? "active" : ""} disabled={!canManage} onClick={() => setDraft((current) => ({ ...current, defaultLanguage: "ur" }))} role="radio" type="button"><span lang="ur" dir="rtl">اردو</span></button>
            </div>
          </div>
          <label className="cc-setting-toggle">
            <span>
              <strong>Show language switch</strong>
              <small>{draft.allowUserLanguageChoice ? "Visible to website and portal users" : "Hidden everywhere; default language is used"}</small>
            </span>
            <input checked={draft.allowUserLanguageChoice} disabled={!canManage} onChange={(event) => setDraft((current) => ({ ...current, allowUserLanguageChoice: event.target.checked }))} type="checkbox" />
            <i aria-hidden="true" />
          </label>
        </div>
      </section>

      <section className="cc-settings-section cc-connection-section">
        <header>
          <span className="cc-settings-section-icon"><Wifi size={19} /></span>
          <div>
            <h3>Device connection</h3>
            <small className="cc-connection-state">
              <i className={connectionError ? "is-offline" : ""} aria-hidden="true" />
              {connectionLoading ? "Checking..." : connectionError ? "Unavailable" : "Connected / auto-updating"}
            </small>
          </div>
          <button
            aria-label="Refresh connection addresses"
            className="cc-icon-button cc-connection-refresh"
            disabled={connectionLoading}
            onClick={() => setConnectionRefresh((current) => current + 1)}
            title="Refresh connection addresses"
            type="button"
          >
            <RefreshCw className={connectionLoading ? "is-spinning" : ""} size={17} />
          </button>
        </header>

        {connectionError && !connection && <div className="cc-connection-error">{connectionError}</div>}

        <div className="cc-connection-list">
          {connection ? (
            <>
              <div className="cc-connection-row is-stable">
                <div className="cc-connection-label">
                  <span>Stable address</span>
                  <small data-no-translate>{connection.hostName}.local</small>
                </div>
                <code data-no-translate>{connection.stableUrl}</code>
                <div className="cc-connection-actions">
                  <a aria-label="Open stable address" href={connection.stableUrl} rel="noreferrer" target="_blank" title="Open stable address">
                    <ExternalLink size={16} />
                  </a>
                  <button aria-label="Copy stable address" onClick={() => void copyConnection(connection.stableUrl, "stable")} title="Copy stable address" type="button">
                    {copiedConnection === "stable" ? <Check size={16} /> : <Copy size={16} />}
                  </button>
                </div>
              </div>

              {connection.addresses.map((address) => (
                <div className="cc-connection-row" key={address.address}>
                  <div className="cc-connection-label">
                    <span>{address.interface}</span>
                    <small data-no-translate>{address.address}</small>
                  </div>
                  <code data-no-translate>{address.url}</code>
                  <div className="cc-connection-actions">
                    <a aria-label={"Open " + address.interface + " address"} href={address.url} rel="noreferrer" target="_blank" title={"Open " + address.interface + " address"}>
                      <ExternalLink size={16} />
                    </a>
                    <button aria-label={"Copy " + address.interface + " address"} onClick={() => void copyConnection(address.url, address.address)} title={"Copy " + address.interface + " address"} type="button">
                      {copiedConnection === address.address ? <Check size={16} /> : <Copy size={16} />}
                    </button>
                  </div>
                </div>
              ))}

              {connection.addresses.length === 0 && (
                <div className="cc-connection-empty">No active private network address detected.</div>
              )}
            </>
          ) : (
            <div className="cc-connection-empty">Checking active network addresses...</div>
          )}
        </div>

        {connection && (
          <div className="cc-connection-foot">
            <span>Frontend port <strong data-no-translate>{connection.port}</strong></span>
            <span>Checked {new Date(connection.updatedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</span>
          </div>
        )}
      </section>
      <footer className="cc-settings-actions">
        {!canManage && <span>Read-only access</span>}
        <button className="cc-button" disabled={!changed || busy} onClick={() => setDraft(normalizePlatformSettings(settings))} type="button">
          <RotateCcw size={16} />
          Reset
        </button>
        <button className="cc-button cc-primary" disabled={!canManage || !changed || busy} type="submit">
          <Save size={16} />
          {busy ? "Saving..." : "Save settings"}
        </button>
      </footer>
    </form>
  );
}

async function copyText(value: string) {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return true;
    }
    const input = document.createElement("textarea");
    input.value = value;
    input.setAttribute("readonly", "");
    input.style.position = "fixed";
    input.style.opacity = "0";
    document.body.appendChild(input);
    input.select();
    const copied = document.execCommand("copy");
    input.remove();
    return copied;
  } catch {
    return false;
  }
}
function ColorControl({ label, value, disabled, onChange }: { label: string; value: string; disabled: boolean; onChange: (value: string) => void }) {
  return (
    <label>
      <span>{label}</span>
      <span className="cc-color-input">
        <input aria-label={label} disabled={disabled} onChange={(event) => onChange(event.target.value)} type="color" value={value} />
        <code data-no-translate>{value.toUpperCase()}</code>
      </span>
    </label>
  );
}

function comparable(settings: PlatformSettings) {
  const normalized = normalizePlatformSettings(settings);
  return JSON.stringify({
    themePreset: normalized.themePreset,
    primaryColor: normalized.primaryColor,
    accentColor: normalized.accentColor,
    surfaceColor: normalized.surfaceColor,
    defaultLanguage: normalized.defaultLanguage,
    allowUserLanguageChoice: normalized.allowUserLanguageChoice
  });
}
