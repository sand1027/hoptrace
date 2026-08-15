"use client";

import { FormEvent, useEffect, useState } from "react";
import type { ProbeAPIRequest } from "@hoptrace/shared-types";
import styles from "./Composer.module.css";

export type ComposerDraft = {
  url: string;
  method: string;
  follow: boolean;
  body: string;
  headersText: string;
  slo: string;
  timeoutMs: number;
};

type Props = {
  onSubmit: (payload: ProbeAPIRequest, opts: { live: boolean }) => Promise<void>;
  loading: boolean;
  draft?: ComposerDraft | null;
  liveStatus?: string | null;
};

const defaults: ComposerDraft = {
  url: "https://httpbin.io/get",
  method: "GET",
  follow: false,
  body: "",
  headersText: "",
  slo: "",
  timeoutMs: 30000,
};

export function Composer({ onSubmit, loading, draft, liveStatus }: Props) {
  const [url, setUrl] = useState(defaults.url);
  const [method, setMethod] = useState(defaults.method);
  const [follow, setFollow] = useState(defaults.follow);
  const [body, setBody] = useState(defaults.body);
  const [headersText, setHeadersText] = useState(defaults.headersText);
  const [slo, setSlo] = useState(defaults.slo);
  const [timeoutMs, setTimeoutMs] = useState(defaults.timeoutMs);
  const [live, setLive] = useState(true);
  const [advanced, setAdvanced] = useState(false);

  useEffect(() => {
    if (!draft) return;
    setUrl(draft.url);
    setMethod(draft.method);
    setFollow(draft.follow);
    setBody(draft.body);
    setHeadersText(draft.headersText);
    setSlo(draft.slo);
    setTimeoutMs(draft.timeoutMs);
    setAdvanced(true);
  }, [draft]);

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

    await onSubmit(
      {
        url,
        method,
        headers,
        body: body || undefined,
        follow_redirects: follow,
        timeout_ms: timeoutMs,
        slo: Object.keys(sloMap).length ? sloMap : undefined,
      },
      { live }
    );
  }

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.bar}>
        <select
          className={styles.method}
          value={method}
          onChange={(e) => setMethod(e.target.value)}
          aria-label="HTTP method"
        >
          {["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"].map(
            (m) => (
              <option key={m} value={m}>
                {m}
              </option>
            )
          )}
        </select>
        <input
          className={styles.url}
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://example.com"
          required
          spellCheck={false}
        />
        <button className={styles.cta} type="submit" disabled={loading}>
          {loading ? "Running…" : "Run"}
        </button>
      </div>

      <div className={styles.toolbar}>
        <label className={styles.toggle}>
          <input
            type="checkbox"
            checked={live}
            onChange={(e) => setLive(e.target.checked)}
          />
          Live stream
        </label>
        <label className={styles.toggle}>
          <input
            type="checkbox"
            checked={follow}
            onChange={(e) => setFollow(e.target.checked)}
          />
          Follow redirects
        </label>
        <button
          type="button"
          className={styles.advBtn}
          onClick={() => setAdvanced((v) => !v)}
          aria-expanded={advanced}
        >
          {advanced ? "Hide options" : "Options"}
        </button>
        {liveStatus && <span className={styles.live}>{liveStatus}</span>}
      </div>

      {advanced && (
        <div className={styles.advanced}>
          <label className={styles.label}>
            Headers
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
          <div className={styles.row}>
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
            <label className={styles.label}>
              SLO
              <input
                className={styles.input}
                value={slo}
                onChange={(e) => setSlo(e.target.value)}
                placeholder="total=2000,ttfb=800"
              />
            </label>
          </div>
        </div>
      )}
    </form>
  );
}

export function payloadToDraft(p: ProbeAPIRequest): ComposerDraft {
  const headersText = Object.entries(p.headers || {})
    .map(([k, v]) => `${k}: ${v}`)
    .join("\n");
  const slo = Object.entries(p.slo || {})
    .map(([k, v]) => `${k}=${v}`)
    .join(",");
  return {
    url: p.url,
    method: p.method || "GET",
    follow: !!p.follow_redirects,
    body: p.body || "",
    headersText,
    slo,
    timeoutMs: p.timeout_ms || 30000,
  };
}
