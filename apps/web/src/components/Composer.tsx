"use client";

import { FormEvent, useState } from "react";
import styles from "./Composer.module.css";

type Props = {
  onSubmit: (payload: Record<string, unknown>) => Promise<void>;
  loading: boolean;
};

export function Composer({ onSubmit, loading }: Props) {
  const [url, setUrl] = useState("https://httpbin.io/get");
  const [method, setMethod] = useState("GET");
  const [follow, setFollow] = useState(false);
  const [body, setBody] = useState("");
  const [headersText, setHeadersText] = useState("");
  const [slo, setSlo] = useState("");
  const [timeoutMs, setTimeoutMs] = useState(30000);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const headers: Record<string, string> = {};
    for (const line of headersText.split("\n")) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      const idx = trimmed.indexOf(":");
      if (idx === -1) continue;
      headers[trimmed.slice(0, idx).trim()] = trimmed.slice(idx + 1).trim();
    }

    const sloMap: Record<string, number> = {};
    if (slo.trim()) {
      for (const part of slo.split(",")) {
        const [k, v] = part.split("=").map((s) => s.trim());
        if (k && v) sloMap[k] = Number(v);
      }
    }

    await onSubmit({
      url,
      method,
      headers,
      body: body || undefined,
      follow_redirects: follow,
      timeout_ms: timeoutMs,
      slo: Object.keys(sloMap).length ? sloMap : undefined,
    });
  }

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <label className={styles.label}>
        URL
        <input
          className={styles.input}
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://example.com"
          required
        />
      </label>

      <div className={styles.row}>
        <label className={styles.label}>
          Method
          <select
            className={styles.input}
            value={method}
            onChange={(e) => setMethod(e.target.value)}
          >
            {["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"].map(
              (m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              )
            )}
          </select>
        </label>
        <label className={styles.label}>
          Timeout (ms)
          <input
            className={styles.input}
            type="number"
            min={1000}
            value={timeoutMs}
            onChange={(e) => setTimeoutMs(Number(e.target.value))}
          />
        </label>
      </div>

      <label className={styles.check}>
        <input
          type="checkbox"
          checked={follow}
          onChange={(e) => setFollow(e.target.checked)}
        />
        Follow redirects
      </label>

      <label className={styles.label}>
        Headers (one Key: Value per line)
        <textarea
          className={styles.textarea}
          rows={3}
          value={headersText}
          onChange={(e) => setHeadersText(e.target.value)}
          placeholder={"Accept: application/json"}
        />
      </label>

      <label className={styles.label}>
        Body
        <textarea
          className={styles.textarea}
          rows={3}
          value={body}
          onChange={(e) => setBody(e.target.value)}
          placeholder='{"hello":"world"}'
        />
      </label>

      <label className={styles.label}>
        SLO (optional)
        <input
          className={styles.input}
          value={slo}
          onChange={(e) => setSlo(e.target.value)}
          placeholder="total=2000,ttfb=800"
        />
      </label>

      <button className={styles.cta} type="submit" disabled={loading}>
        {loading ? "Running…" : "Run probe"}
      </button>
    </form>
  );
}
