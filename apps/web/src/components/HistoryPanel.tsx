"use client";

import type { ProbeRecord } from "@hoptrace/shared-types";
import styles from "./HistoryPanel.module.css";

type Props = {
  items: ProbeRecord[];
  selectedId?: string;
  onSelect: (rec: ProbeRecord) => void;
  onRefresh: () => void;
  loading?: boolean;
};

export function HistoryPanel({
  items,
  selectedId,
  onSelect,
  onRefresh,
  loading,
}: Props) {
  return (
    <aside className={styles.panel}>
      <div className={styles.head}>
        <h2>History</h2>
        <button type="button" className={styles.refresh} onClick={onRefresh}>
          Refresh
        </button>
      </div>
      {loading && <div className={styles.empty}>Loading…</div>}
      {!loading && items.length === 0 && (
        <div className={styles.empty}>No history yet.</div>
      )}
      <ul className={styles.list}>
        {items.map((item) => {
          const active = item.id === selectedId;
          return (
            <li key={item.id}>
              <button
                type="button"
                className={active ? styles.itemActive : styles.item}
                onClick={() => onSelect(item)}
              >
                <span className={styles.status}>
                  {item.report.summary.final_status || "—"}
                </span>
                <span className={styles.meta}>
                  <span className={styles.ms}>
                    {item.report.summary.total_time_ms.toFixed(0)} ms
                  </span>
                  <span className={styles.when}>
                    {formatWhen(item.created_at)}
                  </span>
                </span>
                <span className={styles.url}>{item.report.initial_url}</span>
              </button>
            </li>
          );
        })}
      </ul>
    </aside>
  );
}

function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
