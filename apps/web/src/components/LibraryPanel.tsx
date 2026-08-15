"use client";

import { useState } from "react";
import type { ProbeAPIRequest, SavedProbe } from "@hoptrace/shared-types";
import styles from "./LibraryPanel.module.css";

type Props = {
  items: SavedProbe[];
  loading?: boolean;
  onRefresh: () => void;
  onRun: (name: string) => void;
  onLoad: (sp: SavedProbe) => void;
  onDelete: (name: string) => void;
  onSaveCurrent: (name: string, desc: string) => void;
};

export function LibraryPanel({
  items,
  loading,
  onRefresh,
  onRun,
  onLoad,
  onDelete,
  onSaveCurrent,
}: Props) {
  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");

  return (
    <aside className={styles.panel}>
      <div className={styles.head}>
        <h2>Library</h2>
        <button type="button" className={styles.refresh} onClick={onRefresh}>
          Refresh
        </button>
      </div>

      <div className={styles.saveRow}>
        <input
          className={styles.input}
          placeholder="template name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <button
          type="button"
          className={styles.saveBtn}
          onClick={() => {
            if (!name.trim()) return;
            onSaveCurrent(name.trim(), desc.trim());
            setName("");
            setDesc("");
          }}
        >
          Save current
        </button>
      </div>
      <input
        className={styles.input}
        placeholder="description (optional)"
        value={desc}
        onChange={(e) => setDesc(e.target.value)}
      />

      {loading && <div className={styles.empty}>Loading…</div>}
      {!loading && items.length === 0 && (
        <div className={styles.empty}>No saved templates yet.</div>
      )}

      <ul className={styles.list}>
        {items.map((sp) => (
          <li key={sp.name} className={styles.item}>
            <div className={styles.meta}>
              <strong>{sp.name}</strong>
              <span>
                {sp.method} · {sp.url}
              </span>
            </div>
            <div className={styles.actions}>
              <button type="button" onClick={() => onRun(sp.name)}>
                Run
              </button>
              <button type="button" onClick={() => onLoad(sp)}>
                Load
              </button>
              <button type="button" onClick={() => onDelete(sp.name)}>
                Del
              </button>
            </div>
          </li>
        ))}
      </ul>
    </aside>
  );
}

export function savedToPayload(sp: SavedProbe): ProbeAPIRequest {
  return {
    url: sp.url,
    method: sp.method,
    headers: sp.headers,
    body: sp.body,
    follow_redirects: sp.follow_redirects,
    timeout_ms: sp.timeout_ms,
    ignore_ssl: sp.ignore_ssl,
    proxy: sp.proxy,
    slo: sp.slo,
  };
}
