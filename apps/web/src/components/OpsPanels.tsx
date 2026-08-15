"use client";

import { useState } from "react";
import type { FailureRecord, ScheduleJob } from "@hoptrace/shared-types";
import styles from "./OpsPanels.module.css";

export function SchedulesPanel({
  items,
  loading,
  onRefresh,
  onCreate,
  onDelete,
}: {
  items: ScheduleJob[];
  loading?: boolean;
  onRefresh: () => void;
  onCreate: (name: string, url: string, every: number) => void;
  onDelete: (id: string) => void;
}) {
  const [name, setName] = useState("health");
  const [url, setUrl] = useState("https://httpbin.io/get");
  const [every, setEvery] = useState(60);

  return (
    <aside className={styles.panel}>
      <div className={styles.head}>
        <h2>Schedules</h2>
        <button type="button" onClick={onRefresh}>
          Refresh
        </button>
      </div>
      <div className={styles.form}>
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="name" />
        <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="url" />
        <input
          type="number"
          value={every}
          onChange={(e) => setEvery(Number(e.target.value))}
          placeholder="every sec"
        />
        <button type="button" onClick={() => onCreate(name, url, every)}>
          Add
        </button>
      </div>
      {loading && <div className={styles.empty}>Loading…</div>}
      <ul className={styles.list}>
        {items.map((j) => (
          <li key={j.id}>
            <div>
              <strong>{j.name}</strong>
              <span>
                every {j.interval_sec}s · last {j.last_status ?? "—"}/
                {j.last_total_ms?.toFixed?.(0) ?? "—"}ms
              </span>
            </div>
            <button type="button" onClick={() => onDelete(j.id)}>
              Del
            </button>
          </li>
        ))}
      </ul>
    </aside>
  );
}

export function FailuresPanel({ items }: { items: FailureRecord[] }) {
  return (
    <aside className={styles.panel}>
      <div className={styles.head}>
        <h2>Failures</h2>
      </div>
      {items.length === 0 && <div className={styles.empty}>No recent SLO/errors.</div>}
      <ul className={styles.list}>
        {items.map((f) => (
          <li key={f.id}>
            <div>
              <strong>{f.title}</strong>
              <span>
                {f.status} · {f.total_ms.toFixed(0)}ms · {f.message}
              </span>
            </div>
          </li>
        ))}
      </ul>
    </aside>
  );
}
