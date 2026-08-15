package probe

import "time"

// ProbeRequest is the fully-built probe input (constructed via Builder).
type ProbeRequest struct {
	URL            string
	Method         string
	Headers        map[string]string
	Body           []byte
	FollowRedirect bool
	Timeout        time.Duration
	ProxyURL       *string // nil = use env; empty string = disable proxy
	IgnoreSSL      bool
	CABundlePath   string
	MaxRedirects   int
	MaxBodyBytes   int64 // 0 = default (10 MiB)
	SLO            map[string]float64 // phase -> threshold ms
}

// Timing holds phase latencies in milliseconds.
type Timing struct {
	DNSMS       float64 `json:"dns_ms"`
	ConnectMS   float64 `json:"connect_ms"`
	TLSMS       float64 `json:"tls_ms"`
	TTFBMS      float64 `json:"ttfb_ms"`
	WaitMS      float64 `json:"wait_ms"`
	XferMS      float64 `json:"xfer_ms"`
	TotalMS     float64 `json:"total_ms"`
	IsEstimated bool    `json:"is_estimated"`
}

// Network captures connection / TLS metadata.
type Network struct {
	IP           string  `json:"ip"`
	IPFamily     string  `json:"ip_family"`
	HTTPVersion  string  `json:"http_version"`
	TLSVersion   string  `json:"tls_version,omitempty"`
	TLSCipher    string  `json:"tls_cipher,omitempty"`
	CertCN       string  `json:"cert_cn,omitempty"`
	CertDaysLeft *int    `json:"cert_days_left,omitempty"`
	TLSVerified  bool    `json:"tls_verified"`
	TLSCustomCA  *bool   `json:"tls_custom_ca"`
	ProxyURL     *string `json:"proxy_url"`
	ProxySource  *string `json:"proxy_source"`
}

// RequestMeta describes what was sent for a step.
type RequestMeta struct {
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	BodyBytes int               `json:"body_bytes"`
}

// ResponseMeta describes the HTTP response for a step.
type ResponseMeta struct {
	Status      int               `json:"status"`
	Bytes       int64             `json:"bytes"`
	ContentType *string           `json:"content_type"`
	Server      *string           `json:"server"`
	Date        *string           `json:"date"`
	Location    *string           `json:"location"`
	Headers     map[string]string `json:"headers"`
}

// StepResult is one hop in a (possibly redirected) probe.
type StepResult struct {
	URL        string       `json:"url"`
	StepNumber int          `json:"step_number"`
	Request    RequestMeta  `json:"request"`
	Timing     Timing       `json:"timing"`
	Network    Network      `json:"network"`
	Response   *ResponseMeta `json:"response"`
	Error      *string      `json:"error"`
	Note       *string      `json:"note"`
}

// SLOViolation describes a single threshold breach.
type SLOViolation struct {
	Key          string  `json:"key"`
	ThresholdMS  float64 `json:"threshold_ms"`
	ActualMS     float64 `json:"actual_ms"`
	DeltaMS      float64 `json:"delta_ms"`
}

// SLOResult is the summary SLO evaluation.
type SLOResult struct {
	Pass         bool           `json:"pass"`
	ThresholdsMS map[string]float64 `json:"thresholds_ms"`
	Violations   []SLOViolation `json:"violations,omitempty"`
}

// Summary aggregates the full probe.
type Summary struct {
	TotalTimeMS float64    `json:"total_time_ms"`
	FinalStatus int        `json:"final_status"`
	FinalURL    string     `json:"final_url"`
	FinalBytes  int64      `json:"final_bytes"`
	Errors      int        `json:"errors"`
	SLO         *SLOResult `json:"slo,omitempty"`
}

// ProbeReport is the top-level JSON export shape.
type ProbeReport struct {
	InitialURL string       `json:"initial_url"`
	TotalSteps int          `json:"total_steps"`
	Steps      []StepResult `json:"steps"`
	Summary    Summary      `json:"summary"`
}
