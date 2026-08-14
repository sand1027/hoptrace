"use client";

import { useState } from "react";
import type { ProbeAPIResponse, ProbeReport } from "@hoptrace/shared-types";
import { Composer } from "@/components/Composer";
import { Waterfall } from "@/components/Waterfall";
import { SummaryPanel } from "@/components/SummaryPanel";
import styles from "./page.module.css";

export default function HomePage() {
  const [report, setReport] = useState<ProbeReport | null>(null);
  const [probeId, setProbeId] = useState<string | undefined>();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function runProbe(payload: Record<string, unknown>) {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/v1/probes", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      const data = (await res.json()) as ProbeAPIResponse & { error?: string };
      if (!res.ok && !data.report) {
        throw new Error(data.error || `HTTP ${res.status}`);
      }
      setReport(data.report);
      setProbeId(data.id);
      if (data.error) setError(data.error);
    } catch (e) {
      setReport(null);
      setError(e instanceof Error ? e.message : "Probe failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className={styles.shell}>
      <header className={styles.brand}>
        <div className={styles.mark}>hoptrace</div>
        <p className={styles.tagline}>
          Dissect every hop — DNS, connect, TLS, wait, transfer.
        </p>
      </header>

      <section className={styles.workspace}>
        <Composer onSubmit={runProbe} loading={loading} />
        <div className={styles.results}>
          {error && <div className={styles.error}>{error}</div>}
          {report ? (
            <>
              <SummaryPanel report={report} probeId={probeId} />
              <Waterfall report={report} />
            </>
          ) : (
            <div className={styles.empty}>
              {loading
                ? "Probing…"
                : "Enter a URL and run a probe to see the waterfall."}
            </div>
          )}
        </div>
      </section>
    </main>
  );
}
