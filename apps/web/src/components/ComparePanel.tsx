"use client";

import type { ProbeRecord } from "@hoptrace/shared-types";
import styles from "./ComparePanel.module.css";

type Props = {
  history: ProbeRecord[];
  leftId?: string;
  rightId?: string;
  onChangeLeft: (id: string) => void;
  onChangeRight: (id: string) => void;
};

export function ComparePanel({
  history,
  leftId,
  rightId,
  onChangeLeft,
  onChangeRight,
}: Props) {
  const left = history.find((h) => h.id === leftId);
  const right = history.find((h) => h.id === rightId);
  const lt = left?.report.steps.at(-1)?.timing;
  const rt = right?.report.steps.at(-1)?.timing;

  return (
    <section className={styles.panel}>
      <div className={styles.head}>
        <h2>Compare runs</h2>
        <div className={styles.pickers}>
          <select
            value={leftId || ""}
            onChange={(e) => onChangeLeft(e.target.value)}
          >
            <option value="">Run A</option>
            {history.map((h) => (
              <option key={h.id} value={h.id}>
                {h.id} · {h.report.summary.total_time_ms.toFixed(0)}ms
              </option>
            ))}
          </select>
          <select
            value={rightId || ""}
            onChange={(e) => onChangeRight(e.target.value)}
          >
            <option value="">Run B</option>
            {history.map((h) => (
              <option key={h.id} value={h.id}>
                {h.id} · {h.report.summary.total_time_ms.toFixed(0)}ms
              </option>
            ))}
          </select>
        </div>
      </div>

      {lt && rt ? (
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Phase</th>
              <th>A</th>
              <th>B</th>
              <th>Δ</th>
            </tr>
          </thead>
          <tbody>
            {(
              [
                ["dns", lt.dns_ms, rt.dns_ms],
                ["connect", lt.connect_ms, rt.connect_ms],
                ["tls", lt.tls_ms, rt.tls_ms],
                ["wait", lt.wait_ms, rt.wait_ms],
                ["xfer", lt.xfer_ms, rt.xfer_ms],
                ["ttfb", lt.ttfb_ms, rt.ttfb_ms],
                ["total", lt.total_ms, rt.total_ms],
              ] as const
            ).map(([label, a, b]) => (
              <tr key={label}>
                <td>{label}</td>
                <td>{a.toFixed(1)}</td>
                <td>{b.toFixed(1)}</td>
                <td className={b - a > 0 ? styles.worse : styles.better}>
                  {b - a >= 0 ? "+" : ""}
                  {(b - a).toFixed(1)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <div className={styles.empty}>Pick two history runs to compare.</div>
      )}
    </section>
  );
}
