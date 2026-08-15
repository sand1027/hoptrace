"use client";

import { useCallback, useEffect, useState } from "react";
import type {
  FailureRecord,
  ProbeAPIRequest,
  ProbeAPIResponse,
  ProbeRecord,
  ProbeReport,
  ProbeStreamEvent,
  SavedProbe,
  ScheduleJob,
  StepResult,
} from "@hoptrace/shared-types";
import {
  Composer,
  ComposerDraft,
  payloadToDraft,
} from "@/components/Composer";
import { Waterfall } from "@/components/Waterfall";
import { SummaryPanel } from "@/components/SummaryPanel";
import { HistoryPanel } from "@/components/HistoryPanel";
import { LibraryPanel, savedToPayload } from "@/components/LibraryPanel";
import { ComparePanel } from "@/components/ComparePanel";
import { FailuresPanel, SchedulesPanel } from "@/components/OpsPanels";
import { apiHeaders, setApiKey, wsURL } from "@/lib/api";
import styles from "./page.module.css";

export default function HomePage() {
  const [report, setReport] = useState<ProbeReport | null>(null);
  const [probeId, setProbeId] = useState<string | undefined>();
  const [error, setError] = useState<string | null>(null);
  const [authHint, setAuthHint] = useState<string | null>(null);
  const [apiKeyDraft, setApiKeyDraft] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [loading, setLoading] = useState(false);
  const [liveStatus, setLiveStatus] = useState<string | null>(null);
  const [history, setHistory] = useState<ProbeRecord[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [saved, setSaved] = useState<SavedProbe[]>([]);
  const [savedLoading, setSavedLoading] = useState(false);
  const [draft, setDraft] = useState<ComposerDraft | null>(null);
  const [lastPayload, setLastPayload] = useState<ProbeAPIRequest | null>(null);
  const [compareLeft, setCompareLeft] = useState<string>("");
  const [compareRight, setCompareRight] = useState<string>("");
  const [schedules, setSchedules] = useState<ScheduleJob[]>([]);
  const [failures, setFailures] = useState<FailureRecord[]>([]);
  const [railTab, setRailTab] = useState<"history" | "library" | "ops">(
    "history",
  );

  const noteAuth = useCallback((status: number) => {
    if (status === 401) {
      setAuthHint(
        "API requires auth (-require-auth). Paste your key below, or set NEXT_PUBLIC_HOPTRACE_API_KEY.",
      );
    }
  }, []);

  const loadSchedules = useCallback(async () => {
    try {
      const res = await fetch("/api/v1/schedules", { headers: apiHeaders() });
      if (res.status === 401) noteAuth(401);
      if (res.ok) setSchedules(await res.json());
    } catch (e) {
      console.error(e);
    }
  }, [noteAuth]);

  const loadFailures = useCallback(async () => {
    try {
      const res = await fetch("/api/v1/failures", { headers: apiHeaders() });
      if (res.status === 401) noteAuth(401);
      if (res.ok) setFailures(await res.json());
    } catch (e) {
      console.error(e);
    }
  }, [noteAuth]);

  const loadHistory = useCallback(async () => {
    setHistoryLoading(true);
    try {
      const res = await fetch("/api/v1/probes?limit=30", {
        headers: apiHeaders(),
      });
      if (!res.ok) {
        noteAuth(res.status);
        setHistory([]);
        return;
      }
      setAuthHint(null);
      const data = (await res.json()) as ProbeRecord[];
      setHistory(Array.isArray(data) ? data : []);
    } catch (e) {
      console.error(e);
    } finally {
      setHistoryLoading(false);
    }
  }, [noteAuth]);

  const loadSaved = useCallback(async () => {
    setSavedLoading(true);
    try {
      const res = await fetch("/api/v1/saved", { headers: apiHeaders() });
      if (!res.ok) {
        noteAuth(res.status);
        setSaved([]);
        return;
      }
      const data = (await res.json()) as SavedProbe[];
      setSaved(Array.isArray(data) ? data : []);
    } catch (e) {
      console.error(e);
    } finally {
      setSavedLoading(false);
    }
  }, [noteAuth]);

  useEffect(() => {
    void loadHistory();
    void loadSaved();
    void loadSchedules();
    void loadFailures();
  }, [loadHistory, loadSaved, loadSchedules, loadFailures]);

  function applyApiKey() {
    setApiKey(apiKeyDraft);
    setAuthHint(null);
    setShowKey(false);
    void loadHistory();
    void loadSaved();
    void loadSchedules();
    void loadFailures();
  }

  async function runREST(payload: ProbeAPIRequest) {
    const res = await fetch("/api/v1/probes", {
      method: "POST",
      headers: apiHeaders(),
      body: JSON.stringify(payload),
    });
    if (res.status === 401) noteAuth(401);
    const data = (await res.json()) as ProbeAPIResponse & { error?: string };
    if (!res.ok && !data.report) {
      throw new Error(data.error || `HTTP ${res.status}`);
    }
    setReport(data.report);
    setProbeId(data.id);
    if (data.error) setError(data.error);
  }

  function runLive(payload: ProbeAPIRequest): Promise<void> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(wsURL());
      const steps: StepResult[] = [];

      ws.onopen = () => {
        setLiveStatus("connected — probing…");
        ws.send(JSON.stringify(payload));
      };

      ws.onmessage = (msg) => {
        const ev = JSON.parse(msg.data) as ProbeStreamEvent;
        if (ev.kind === "step_start") {
          setLiveStatus(`step ${ev.step} starting…`);
        }
        if (ev.kind === "phase" && ev.phase) {
          setLiveStatus(`step ${ev.step}: ${ev.phase} ${ev.ms?.toFixed?.(1) ?? ""}ms`);
        }
        if (ev.kind === "step_done" && ev.result) {
          steps.push(ev.result);
          setReport({
            initial_url: payload.url,
            total_steps: steps.length,
            steps: [...steps],
            summary: {
              total_time_ms: steps.reduce((s, x) => s + x.timing.total_ms, 0),
              final_status: ev.result.response?.status || 0,
              final_url: ev.result.url,
              final_bytes: ev.result.response?.bytes || 0,
              errors: steps.filter((s) => s.error).length,
            },
          });
        }
        if (ev.kind === "complete" || ev.kind === "done") {
          if (ev.report) setReport(ev.report);
          if (ev.id) setProbeId(ev.id);
          if (ev.error) setError(typeof ev.error === "string" ? ev.error : null);
          setLiveStatus("complete");
          ws.close();
          resolve();
        }
        if (ev.kind === "error" && !ev.report) {
          setLiveStatus(null);
          reject(new Error(ev.error || "stream error"));
          ws.close();
        }
      };

      ws.onerror = () => {
        reject(new Error("WebSocket failed — is the API running on :8080?"));
      };
    });
  }

  async function runProbe(payload: ProbeAPIRequest, opts: { live: boolean }) {
    setLoading(true);
    setError(null);
    setLiveStatus(null);
    setLastPayload(payload);
    try {
      if (opts.live) {
        await runLive(payload);
      } else {
        await runREST(payload);
      }
      void loadHistory();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Probe failed");
    } finally {
      setLoading(false);
    }
  }

  function selectHistory(rec: ProbeRecord) {
    setReport(rec.report);
    setProbeId(rec.id);
    setError(null);
  }

  async function saveCurrent(name: string, description: string) {
    if (!lastPayload && !report) {
      setError("Run a probe first, then save it as a template.");
      return;
    }
    const base = lastPayload || {
      url: report!.initial_url,
      method: "GET",
    };
    const body = {
      name,
      description,
      url: base.url,
      method: base.method || "GET",
      headers: base.headers || {},
      body: base.body || "",
      follow_redirects: !!base.follow_redirects,
      timeout_ms: base.timeout_ms || 30000,
      ignore_ssl: !!base.ignore_ssl,
      proxy: base.proxy,
      slo: base.slo,
    };
    const res = await fetch("/api/v1/saved", {
      method: "POST",
      headers: apiHeaders(),
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      noteAuth(res.status);
      const err = await res.json();
      setError(err.error || "save failed");
      return;
    }
    void loadSaved();
  }

  async function runSaved(name: string) {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`/api/v1/saved/${encodeURIComponent(name)}/run`, {
        method: "POST",
        headers: apiHeaders(),
      });
      if (res.status === 401) noteAuth(401);
      const data = (await res.json()) as ProbeAPIResponse & { error?: string };
      if (!res.ok && !data.report) throw new Error(data.error || "run failed");
      setReport(data.report);
      setProbeId(data.id);
      void loadHistory();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Run failed");
    } finally {
      setLoading(false);
    }
  }

  async function deleteSaved(name: string) {
    await fetch(`/api/v1/saved/${encodeURIComponent(name)}`, {
      method: "DELETE",
      headers: apiHeaders(),
    });
    void loadSaved();
  }

  return (
    <main className={styles.shell}>
      <header className={styles.top}>
        <div className={styles.brand}>
          <div className={styles.mark}>
            hop<span>trace</span>
          </div>
          <p className={styles.tagline}>HTTP phase profiler</p>
        </div>
        <button
          type="button"
          className={styles.keyBtn}
          onClick={() => setShowKey((v) => !v)}
        >
          API key
        </button>
      </header>

      {(authHint || showKey) && (
        <div className={styles.authRow}>
          {authHint && <p className={styles.authHint}>{authHint}</p>}
          <div className={styles.authForm}>
            <input
              className={styles.authInput}
              type="password"
              placeholder="X-API-Key (ht_…)"
              value={apiKeyDraft}
              onChange={(e) => setApiKeyDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") applyApiKey();
              }}
            />
            <button
              type="button"
              className={styles.authBtn}
              onClick={applyApiKey}
            >
              Save
            </button>
          </div>
        </div>
      )}

      <Composer
        onSubmit={runProbe}
        loading={loading}
        draft={draft}
        liveStatus={liveStatus}
      />

      <section className={styles.workspace}>
        <div className={styles.main}>
          {error && <div className={styles.error}>{error}</div>}
          {report ? (
            <>
              <SummaryPanel report={report} probeId={probeId} />
              <Waterfall report={report} />
              <ComparePanel
                history={history}
                leftId={compareLeft || history[1]?.id}
                rightId={compareRight || history[0]?.id}
                onChangeLeft={setCompareLeft}
                onChangeRight={setCompareRight}
              />
            </>
          ) : (
            <div className={styles.empty}>
              <p className={styles.emptyTitle}>
                {loading ? "Tracing hops…" : "Ready when you are"}
              </p>
              <p className={styles.emptyHint}>
                {loading
                  ? "Watch DNS → connect → TLS → wait → transfer unfold."
                  : "Enter a URL above and run a probe to see the waterfall."}
              </p>
              {!loading && (
                <div className={styles.phases}>
                  {["DNS", "Connect", "TLS", "Wait", "Transfer"].map((p) => (
                    <span key={p} className={styles.phasePill}>
                      {p}
                    </span>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        <aside className={styles.rail}>
          <div className={styles.tabs} role="tablist">
            {(
              [
                ["history", "History"],
                ["library", "Library"],
                ["ops", "Ops"],
              ] as const
            ).map(([id, label]) => (
              <button
                key={id}
                type="button"
                role="tab"
                aria-selected={railTab === id}
                className={railTab === id ? styles.tabActive : styles.tab}
                onClick={() => setRailTab(id)}
              >
                {label}
              </button>
            ))}
          </div>

          {railTab === "history" && (
            <HistoryPanel
              items={history}
              selectedId={probeId}
              onSelect={selectHistory}
              onRefresh={() => void loadHistory()}
              loading={historyLoading}
            />
          )}
          {railTab === "library" && (
            <LibraryPanel
              items={saved}
              loading={savedLoading}
              onRefresh={() => void loadSaved()}
              onRun={(n) => void runSaved(n)}
              onLoad={(sp) => {
                setDraft(payloadToDraft(savedToPayload(sp)));
                setLastPayload(savedToPayload(sp));
              }}
              onDelete={(n) => void deleteSaved(n)}
              onSaveCurrent={(n, d) => void saveCurrent(n, d)}
            />
          )}
          {railTab === "ops" && (
            <>
              <SchedulesPanel
                items={schedules}
                onRefresh={() => void loadSchedules()}
                onCreate={async (name, url, every) => {
                  await fetch("/api/v1/schedules", {
                    method: "POST",
                    headers: apiHeaders(),
                    body: JSON.stringify({
                      name,
                      url,
                      method: "GET",
                      interval_sec: every,
                      enabled: true,
                    }),
                  });
                  void loadSchedules();
                }}
                onDelete={async (id) => {
                  await fetch(`/api/v1/schedules/${id}`, {
                    method: "DELETE",
                    headers: apiHeaders(),
                  });
                  void loadSchedules();
                }}
              />
              <FailuresPanel items={failures} />
            </>
          )}
        </aside>
      </section>
    </main>
  );
}
