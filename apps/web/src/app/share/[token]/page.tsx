"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import type { ProbeRecord } from "@hoptrace/shared-types";
import { Waterfall } from "@/components/Waterfall";
import { SummaryPanel } from "@/components/SummaryPanel";
import styles from "../../page.module.css";

export default function SharePage() {
  const params = useParams<{ token: string }>();
  const token = params?.token;
  const [rec, setRec] = useState<ProbeRecord | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const res = await fetch(`/api/v1/share/${encodeURIComponent(token)}`);
        if (!res.ok) {
          const body = await res.json().catch(() => ({}));
          throw new Error(body.error || `HTTP ${res.status}`);
        }
        const data = (await res.json()) as ProbeRecord;
        if (!cancelled) setRec(data);
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : "Failed to load share");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [token]);

  return (
    <main className={styles.shell}>
      <header className={styles.top}>
        <div className={styles.brand}>
          <div className={styles.mark}>
            hop<span>trace</span>
          </div>
          <p className={styles.tagline}>Shared probe report</p>
        </div>
        <a className={styles.keyBtn} href="/">
          Open app
        </a>
      </header>

      {loading && <div className={styles.empty}>Loading shared report…</div>}
      {error && <div className={styles.error}>{error}</div>}
      {rec && (
        <div className={styles.main}>
          <SummaryPanel report={rec.report} probeId={rec.id} />
          <Waterfall report={rec.report} />
        </div>
      )}
    </main>
  );
}
