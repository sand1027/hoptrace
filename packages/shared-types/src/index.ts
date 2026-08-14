export type Timing = {
  dns_ms: number;
  connect_ms: number;
  tls_ms: number;
  ttfb_ms: number;
  wait_ms: number;
  xfer_ms: number;
  total_ms: number;
  is_estimated: boolean;
};

export type Network = {
  ip: string;
  ip_family: string;
  http_version: string;
  tls_version?: string;
  tls_cipher?: string;
  cert_cn?: string;
  cert_days_left?: number | null;
  tls_verified: boolean;
  tls_custom_ca?: boolean | null;
  proxy_url?: string | null;
  proxy_source?: string | null;
};

export type ResponseMeta = {
  status: number;
  bytes: number;
  content_type?: string | null;
  server?: string | null;
  date?: string | null;
  location?: string | null;
  headers: Record<string, string>;
};

export type StepResult = {
  url: string;
  step_number: number;
  request: {
    method: string;
    headers: Record<string, string>;
    body_bytes: number;
  };
  timing: Timing;
  network: Network;
  response: ResponseMeta | null;
  error: string | null;
  note: string | null;
};

export type SLOViolation = {
  key: string;
  threshold_ms: number;
  actual_ms: number;
  delta_ms: number;
};

export type SLOResult = {
  pass: boolean;
  thresholds_ms: Record<string, number>;
  violations?: SLOViolation[];
};

export type ProbeReport = {
  initial_url: string;
  total_steps: number;
  steps: StepResult[];
  summary: {
    total_time_ms: number;
    final_status: number;
    final_url: string;
    final_bytes: number;
    errors: number;
    slo?: SLOResult | null;
  };
};

export type ProbeAPIRequest = {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  body?: string;
  follow_redirects?: boolean;
  timeout_ms?: number;
  ignore_ssl?: boolean;
  proxy?: string | null;
  slo?: Record<string, number>;
};

export type ProbeAPIResponse = {
  id?: string;
  report: ProbeReport;
  error: string | null;
};
