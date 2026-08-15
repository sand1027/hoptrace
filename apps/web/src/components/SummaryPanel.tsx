"use client";

import { useState } from "react";
import type { ProbeReport } from "@hoptrace/shared-types";
import { apiHeaders } from "@/lib/api";
import styles from "./SummaryPanel.module.css";

export function SummaryPanel({
  report,
  probeId,
}: {
  report: ProbeReport;
  probeId?: string;
}) {
  const last = report.steps[report.steps.length - 1];
  const slo = report.summary.slo;
  const [shareMsg, setShareMsg] = useState<string | null>(null);
  const [sharing, setSharing] = useState(false);

  function downloadJSON() {
    const blob = new Blob([JSON.stringify(report, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `hoptrace-${probeId || "report"}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }

  async function shareLink() {
    if (!probeId) {
      setShareMsg("Run a probe first to get a shareable link.");
      return;
    }
    setSharing(true);
    setShareMsg(null);
    try {
      const res = await fetch(`/api/v1/probes/${encodeURIComponent(probeId)}/share`, {
        method: "POST",
        headers: apiHeaders(),
      });
      const data = (await res.json()) as { token?: string; error?: string };
      if (!res.ok || !data.token) {
        throw new Error(data.error || `HTTP ${res.status}`);
      }
      const link = `${window.location.origin}/share/${data.token}`;
      await navigator.clipboard.writeText(link);
      setShareMsg("Share link copied.");
    } catch (e) {
      setShareMsg(e instanceof Error ? e.message : "Share failed");
    } finally {
      setSharing(false);
    }
  }

  return (
    <div className={styles.panel}>
      <div className={styles.stats}>
        <Stat label="Total" value={`${report.summary.total_time_ms.toFixed(1)} ms`} />
        <Stat label="Status" value={String(report.summary.final_status || "—")} />
        <Stat label="Steps" value={String(report.total_steps)} />
        <Stat label="Bytes" value={String(report.summary.final_bytes)} />
      </div>

      <div className={styles.details}>
        <div>
          <span className={styles.k}>Final URL</span>
          <code>{report.summary.final_url}</code>
        </div>
        {last?.network.http_version && (
          <div>
            <span className={styles.k}>HTTP</span>
            <code>{last.network.http_version}</code>
          </div>
        )}
        {last?.network.cert_cn && (
          <div>
            <span className={styles.k}>Cert CN</span>
            <code>
              {last.network.cert_cn}
              {last.network.cert_days_left != null
                ? ` · ${last.network.cert_days_left}d left`
                : ""}
            </code>
          </div>
        )}
        {last?.timing.is_estimated && (
          <div>
            <span className={styles.k}>Timing</span>
            <code>estimated (partial trace hooks)</code>
          </div>
        )}
        {slo && (
          <div className={slo.pass ? styles.sloPass : styles.sloFail}>
            <span className={styles.k}>SLO</span>
            <code>
              {slo.pass
                ? "pass"
                : `fail — ${(slo.violations || [])
                    .map((v) => `${v.key}+${v.delta_ms.toFixed(0)}ms`)
                    .join(", ")}`}
            </code>
          </div>
        )}
      </div>

      <div className={styles.actions}>
        <button type="button" className={styles.download} onClick={downloadJSON}>
          Download JSON
        </button>
        <button
          type="button"
          className={styles.download}
          onClick={() => void shareLink()}
          disabled={sharing || !probeId}
        >
          {sharing ? "Sharing…" : "Copy share link"}
        </button>
      </div>
      {shareMsg && <p className={styles.shareMsg}>{shareMsg}</p>}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className={styles.stat}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
