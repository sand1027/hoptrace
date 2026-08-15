function readApiKey(): string {
  if (typeof window !== "undefined") {
    const fromStorage = window.localStorage.getItem("hoptrace_api_key");
    if (fromStorage) return fromStorage;
  }
  return process.env.NEXT_PUBLIC_HOPTRACE_API_KEY || "";
}

export function apiHeaders(extra?: HeadersInit): HeadersInit {
  const h: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const key = readApiKey();
  if (key) {
    h["X-API-Key"] = key;
  }
  if (extra) {
    const e =
      extra instanceof Headers
        ? Object.fromEntries(extra.entries())
        : Array.isArray(extra)
          ? Object.fromEntries(extra)
          : (extra as Record<string, string>);
    Object.assign(h, e);
  }
  return h;
}

export function wsURL(): string {
  const base =
    process.env.NEXT_PUBLIC_HOPTRACE_WS ||
    "ws://127.0.0.1:8080/v1/probes/stream";
  const key = readApiKey();
  if (!key) return base;
  const join = base.includes("?") ? "&" : "?";
  return `${base}${join}api_key=${encodeURIComponent(key)}`;
}

export function hasApiKey(): boolean {
  return Boolean(readApiKey());
}

/** Persist key for browser sessions (also set NEXT_PUBLIC_HOPTRACE_API_KEY). */
export function setApiKey(key: string) {
  if (typeof window === "undefined") return;
  const trimmed = key.trim();
  if (trimmed) window.localStorage.setItem("hoptrace_api_key", trimmed);
  else window.localStorage.removeItem("hoptrace_api_key");
}
