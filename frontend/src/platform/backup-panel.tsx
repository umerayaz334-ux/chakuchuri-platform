import { useEffect, useState } from "react";
import { CheckCircle2, Cloud, DatabaseBackup, Download, HardDrive, RefreshCw, ShieldCheck } from "lucide-react";
import { apiBlob, apiGet, apiPost } from "./api";
import type { BackupOverview, BackupRecord, Session } from "./types";
import { messageFromError } from "./utils";

export function BackupPanel({ session, onNotice }: { session: Session; onNotice: (message: string) => void }) {
  const [overview, setOverview] = useState<BackupOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [downloading, setDownloading] = useState("");
  const [error, setError] = useState("");
  const canManage = ["owner", "admin"].includes(session.user.role.toLowerCase()) || session.user.permissions?.includes("backups.manage");

  const load = async () => {
    if (!canManage) return;
    setLoading(true);
    try {
      setOverview(await apiGet<BackupOverview>("/api/admin/backups", session.token));
      setError("");
    } catch (loadError) {
      setError(messageFromError(loadError, "Backups could not be loaded."));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, [session.token, canManage]);

  const createBackup = async () => {
    setCreating(true);
    try {
      const created = await apiPost<BackupRecord>("/api/admin/backups", {}, session.token);
      onNotice(`Verified backup created: ${created.name}`);
      await load();
    } catch (createError) {
      const message = messageFromError(createError, "Backup could not be created.");
      setError(message);
      onNotice(message);
    } finally {
      setCreating(false);
    }
  };

  const downloadBackup = async (backup: BackupRecord) => {
    setDownloading(backup.name);
    try {
      const blob = await apiBlob(`/api/admin/backups/${encodeURIComponent(backup.name)}/content`, session.token);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = backup.name;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      onNotice(`Backup download started: ${backup.name}`);
    } catch (downloadError) {
      const message = messageFromError(downloadError, "Backup could not be downloaded.");
      setError(message);
      onNotice(message);
    } finally {
      setDownloading("");
    }
  };

  if (!canManage) {
    return (
      <section className="cc-platform-backups">
        <header className="cc-backup-heading">
          <span className="cc-settings-section-icon"><DatabaseBackup size={19} /></span>
          <div><h2>Backups and recovery</h2><small>Owner or backup-manager access required</small></div>
        </header>
      </section>
    );
  }

  const backups = overview?.backups || [];
  const policy = overview?.policy;

  return (
    <section className="cc-platform-backups">
      <header className="cc-backup-heading">
        <span className="cc-settings-section-icon"><DatabaseBackup size={19} /></span>
        <div>
          <span className="cc-kicker">Data safety</span>
          <h2>Backups and recovery</h2>
        </div>
        <div className="cc-backup-actions">
          <button aria-label="Refresh backups" className="cc-icon-button" disabled={loading || creating} onClick={() => void load()} title="Refresh backups" type="button">
            <RefreshCw className={loading ? "is-spinning" : ""} size={17} />
          </button>
          <button className="cc-button cc-primary" disabled={creating} onClick={() => void createBackup()} type="button">
            <DatabaseBackup size={16} />
            {creating ? "Creating..." : "Create backup"}
          </button>
        </div>
      </header>

      {policy && (
        <div className="cc-backup-policy">
          <div><ShieldCheck size={17} /><span><small>Schedule</small><strong>{policy.automatic ? `Every ${policy.interval}` : "Manual"}</strong></span></div>
          <div><HardDrive size={17} /><span><small>Retention</small><strong>{policy.retention} archives</strong></span></div>
          <div><CheckCircle2 size={17} /><span><small>Source</small><strong>{policy.dataSource}</strong></span></div>
          <div>
            <Cloud size={17} />
            <span>
              <small>Offsite</small>
              <strong data-no-translate>
                {policy.offsiteConfigured ? policy.offsiteDir : policy.offsiteRequired ? "Required — not configured" : "Not configured"}
              </strong>
            </span>
          </div>
        </div>
      )}

      {error && <div className="cc-backup-error">{error}</div>}

      <div className="cc-backup-list">
        <div className="cc-backup-list-head">
          <span>Archive</span><span>Contents</span><span>Status</span><span aria-hidden="true" />
        </div>
        {backups.map((backup, index) => (
          <div className={`cc-backup-row ${index === 0 ? "is-latest" : ""}`} key={backup.name}>
            <div className="cc-backup-name">
              <strong data-no-translate>{backup.name}</strong>
              <small>
                {new Date(backup.createdAt).toLocaleString()}
                {backup.offsiteCopied ? " · Offsite copy" : ""}
              </small>
            </div>
            <div className="cc-backup-size"><strong>{formatBytes(backup.size)}</strong><small>{backup.fileCount} files</small></div>
            <span className={`cc-backup-status ${backup.status === "Verified" ? "is-verified" : "is-invalid"}`}>
              {backup.status === "Verified" && <CheckCircle2 size={14} />}{backup.status}
            </span>
            <button
              aria-label={`Download ${backup.name}`}
              disabled={backup.status !== "Verified" || downloading === backup.name}
              onClick={() => void downloadBackup(backup)}
              title="Download verified backup"
              type="button"
            >
              <Download size={16} />
            </button>
          </div>
        ))}
        {!loading && backups.length === 0 && <div className="cc-backup-empty">No backup archives yet.</div>}
        {loading && backups.length === 0 && <div className="cc-backup-empty">Checking backup integrity...</div>}
      </div>
    </section>
  );
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}
