"use client";

import type { ProbeReport, StepResult } from "@hoptrace/shared-types";
import styles from "./Waterfall.module.css";

const PHASES = [
  { key: "dns_ms", label: "DNS", color: "var(--dns)" },
  { key: "connect_ms", label: "Connect", color: "var(--connect)" },
  { key: "tls_ms", label: "TLS", color: "var(--tls)" },
  { key: "wait_ms", label: "Wait", color: "var(--wait)" },
  { key: "xfer_ms", label: "Transfer", color: "var(--xfer)" },
] as const;

type PhaseKey = (typeof PHASES)[number]["key"];

function phaseValue(step: StepResult, key: PhaseKey): number {
  return step.timing[key] ?? 0;
}

export function Waterfall({ report }: { report: ProbeReport }) {
  const maxTotal = Math.max(
    ...report.steps.map((s) => s.timing.total_ms),
    1
  );

  return (
    <div className={styles.wrap}>
      <div className={styles.head}>
        <h2>Waterfall</h2>
        <div className={styles.legend}>
          {PHASES.map((p) => (
            <span key={p.key} className={styles.legendItem}>
              <i style={{ background: p.color }} />
              {p.label}
            </span>
          ))}
        </div>
      </div>

      <div className={styles.steps}>
        {report.steps.map((step) => {
          const status = step.response?.status ?? "—";
          return (
            <article key={step.step_number} className={styles.step}>
              <header className={styles.stepHead}>
                <span className={styles.stepNum}>#{step.step_number}</span>
                <span className={styles.status}>{status}</span>
                <code className={styles.url}>{step.url}</code>
                <span className={styles.total}>
                  {step.timing.total_ms.toFixed(1)} ms
                </span>
              </header>

              <div className={styles.track}>
                {PHASES.map((p) => {
                  const ms = phaseValue(step, p.key);
                  const width = Math.max((ms / maxTotal) * 100, ms > 0 ? 0.8 : 0);
                  return (
                    <div
                      key={p.key}
                      className={styles.seg}
                      title={`${p.label}: ${ms.toFixed(1)} ms`}
                      style={{
                        width: `${width}%`,
                        background: p.color,
                        opacity: ms > 0 ? 1 : 0.15,
                      }}
                    />
                  );
                })}
              </div>

              <div className={styles.meta}>
                {PHASES.map((p) => (
                  <span key={p.key}>
                    {p.label} {phaseValue(step, p.key).toFixed(1)}
                  </span>
                ))}
                <span>TTFB {step.timing.ttfb_ms.toFixed(1)}</span>
                {step.network.ip && (
                  <span>
                    {step.network.ip} ({step.network.ip_family})
                  </span>
                )}
                {step.network.tls_version && (
                  <span>
                    {step.network.tls_version} · {step.network.tls_cipher}
                  </span>
                )}
              </div>
            </article>
          );
        })}
      </div>
    </div>
  );
}
