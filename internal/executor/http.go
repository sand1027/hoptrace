package executor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sandeepv/hoptrace/internal/probe"
	"github.com/sandeepv/hoptrace/internal/tlsinfo"
)

// TraceTimings collects httptrace phase timestamps (Adapter target).
type TraceTimings struct {
	DNSStart, DNSDone         time.Time
	ConnectStart, ConnectDone time.Time
	TLSStart, TLSDone         time.Time
	GotConn                   time.Time
	WroteRequest              time.Time
	GotFirstByte              time.Time
	RemoteAddr                string
}

// HTTPExecutor performs a single HTTP request with httptrace timings (Strategy).
type HTTPExecutor struct {
	// DNS timings may be supplied externally when DNS is resolved separately.
}

func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{}
}

type executeResult struct {
	step   probe.StepResult
	client *http.Client
}

func (e *HTTPExecutor) Execute(ctx context.Context, req probe.ProbeRequest, targetURL string) (probe.StepResult, error) {
	res, err := e.execute(ctx, req, targetURL, false)
	if err != nil {
		return res.step, err
	}
	return res.step, nil
}

func (e *HTTPExecutor) execute(ctx context.Context, req probe.ProbeRequest, targetURL string, allowRedirect bool) (executeResult, error) {
	timings := &TraceTimings{}
	var tlsState *tls.ConnectionState

	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { timings.DNSStart = time.Now() },
		DNSDone: func(info httptrace.DNSDoneInfo) {
			timings.DNSDone = time.Now()
		},
		ConnectStart: func(_, _ string) { timings.ConnectStart = time.Now() },
		ConnectDone: func(_, addr string, err error) {
			timings.ConnectDone = time.Now()
			if err == nil {
				timings.RemoteAddr = addr
			}
		},
		TLSHandshakeStart: func() { timings.TLSStart = time.Now() },
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			timings.TLSDone = time.Now()
			if err == nil {
				s := state
				tlsState = &s
			}
		},
		GotConn:              func(httptrace.GotConnInfo) { timings.GotConn = time.Now() },
		WroteRequest:         func(httptrace.WroteRequestInfo) { timings.WroteRequest = time.Now() },
		GotFirstResponseByte: func() { timings.GotFirstByte = time.Now() },
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, targetURL, bodyReader(req.Body))
	if err != nil {
		return executeResult{}, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if len(req.Body) > 0 && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", detectContentType(req.Body))
	}

	httpReq = httpReq.WithContext(httptrace.WithClientTrace(httpReq.Context(), trace))

	transport, proxyURL, proxySource, err := buildTransport(req)
	if err != nil {
		return executeResult{}, err
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   req.Timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			if allowRedirect {
				return nil
			}
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	totalWall := time.Since(start)

	step := probe.StepResult{
		URL: targetURL,
		Request: probe.RequestMeta{
			Method:    req.Method,
			Headers:   copyHeaders(req.Headers),
			BodyBytes: len(req.Body),
		},
		Network: probe.Network{
			ProxyURL:    proxyURL,
			ProxySource: proxySource,
			TLSVerified: !req.IgnoreSSL,
		},
	}
	if req.CABundlePath != "" {
		v := true
		step.Network.TLSCustomCA = &v
	}

	if err != nil {
		msg := err.Error()
		step.Error = &msg
		step.Timing = adaptTimings(timings, totalWall, time.Time{})
		fillNetworkFromTrace(&step.Network, timings, tlsState, req.IgnoreSSL)
		return executeResult{step: step, client: client}, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	end := time.Now()
	if readErr != nil {
		msg := readErr.Error()
		step.Error = &msg
	}

	step.Timing = adaptTimings(timings, totalWall, end)
	fillNetworkFromTrace(&step.Network, timings, tlsState, req.IgnoreSSL)
	step.Network.HTTPVersion = httpVersion(resp.Proto)

	ct := resp.Header.Get("Content-Type")
	server := resp.Header.Get("Server")
	date := resp.Header.Get("Date")
	loc := resp.Header.Get("Location")
	step.Response = &probe.ResponseMeta{
		Status:  resp.StatusCode,
		Bytes:   int64(len(body)),
		Headers: flattenHeaders(resp.Header),
	}
	if ct != "" {
		step.Response.ContentType = &ct
	}
	if server != "" {
		step.Response.Server = &server
	}
	if date != "" {
		step.Response.Date = &date
	}
	if loc != "" {
		step.Response.Location = &loc
	}

	var _ probe.RequestExecutor = e
	return executeResult{step: step, client: client}, nil
}

func adaptTimings(t *TraceTimings, wall time.Duration, bodyEnd time.Time) probe.Timing {
	ms := func(d time.Duration) float64 {
		if d < 0 {
			return 0
		}
		return float64(d.Microseconds()) / 1000.0
	}

	var dns, connect, tlsMs, ttfb, wait, xfer float64
	estimated := false

	if !t.DNSStart.IsZero() && !t.DNSDone.IsZero() {
		dns = ms(t.DNSDone.Sub(t.DNSStart))
	}
	if !t.ConnectStart.IsZero() && !t.ConnectDone.IsZero() {
		connect = ms(t.ConnectDone.Sub(t.ConnectStart))
	}
	if !t.TLSStart.IsZero() && !t.TLSDone.IsZero() {
		tlsMs = ms(t.TLSDone.Sub(t.TLSStart))
	}

	// TTFB = time from start of request until first response byte.
	// Prefer GotFirstByte relative to earliest meaningful start.
	startRef := t.DNSStart
	if startRef.IsZero() {
		startRef = t.ConnectStart
	}
	if startRef.IsZero() {
		startRef = t.GotConn
	}
	if !t.GotFirstByte.IsZero() && !startRef.IsZero() {
		ttfb = ms(t.GotFirstByte.Sub(startRef))
	} else {
		ttfb = ms(wall)
		estimated = true
	}

	// Wait = server processing after request written until first byte.
	if !t.WroteRequest.IsZero() && !t.GotFirstByte.IsZero() {
		wait = ms(t.GotFirstByte.Sub(t.WroteRequest))
	} else if !t.GotConn.IsZero() && !t.GotFirstByte.IsZero() {
		wait = ms(t.GotFirstByte.Sub(t.GotConn))
		estimated = true
	}

	if !t.GotFirstByte.IsZero() && !bodyEnd.IsZero() {
		xfer = ms(bodyEnd.Sub(t.GotFirstByte))
	}

	total := ms(wall)
	if total == 0 && !t.GotFirstByte.IsZero() && !startRef.IsZero() {
		total = ttfb + xfer
	}

	return probe.Timing{
		DNSMS:       dns,
		ConnectMS:   connect,
		TLSMS:       tlsMs,
		TTFBMS:      ttfb,
		WaitMS:      wait,
		XferMS:      xfer,
		TotalMS:     total,
		IsEstimated: estimated,
	}
}

func fillNetworkFromTrace(n *probe.Network, t *TraceTimings, state *tls.ConnectionState, ignoreSSL bool) {
	if t.RemoteAddr != "" {
		host, _, err := net.SplitHostPort(t.RemoteAddr)
		if err == nil {
			n.IP = host
		} else {
			n.IP = t.RemoteAddr
		}
		if ip := net.ParseIP(n.IP); ip != nil {
			if ip.To4() != nil {
				n.IPFamily = "IPv4"
			} else {
				n.IPFamily = "IPv6"
			}
		}
	}
	if state != nil {
		info := tlsinfo.FromConnectionState(*state, !ignoreSSL)
		n.TLSVersion = info.Version
		n.TLSCipher = info.Cipher
		n.CertCN = info.CertCN
		n.CertDaysLeft = info.CertDaysLeft
		n.TLSVerified = info.Verified
	}
}

func buildTransport(req probe.ProbeRequest) (*http.Transport, *string, *string, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: req.IgnoreSSL, //nolint:gosec
		MinVersion:         tls.VersionTLS12,
	}
	if req.CABundlePath != "" {
		pem, err := os.ReadFile(req.CABundlePath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read ca bundle: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, nil, nil, fmt.Errorf("invalid ca bundle")
		}
		tlsCfg.RootCAs = pool
	}

	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     tlsCfg,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	var proxyURL *string
	var proxySource *string

	if req.ProxyURL != nil {
		raw := *req.ProxyURL
		if raw == "" {
			tr.Proxy = nil
			src := "bypassed by --proxy \"\""
			proxySource = &src
		} else {
			u, err := url.Parse(raw)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("invalid proxy url: %w", err)
			}
			tr.Proxy = http.ProxyURL(u)
			proxyURL = &raw
			src := "from arg --proxy"
			proxySource = &src
		}
	} else {
		// Detect env proxy for attribution (best-effort).
		for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
			if v := os.Getenv(key); v != "" {
				proxyURL = &v
				src := "from env " + key
				proxySource = &src
				break
			}
		}
	}

	return tr, proxyURL, proxySource, nil
}

func bodyReader(b []byte) io.Reader {
	if len(b) == 0 {
		return nil
	}
	return strings.NewReader(string(b))
}

func detectContentType(b []byte) string {
	s := strings.TrimSpace(string(b))
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return "application/json"
	}
	if strings.HasPrefix(s, "<") {
		return "application/xml"
	}
	return "application/octet-stream"
}

func copyHeaders(h map[string]string) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}

func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vals := range h {
		out[strings.ToLower(k)] = strings.Join(vals, ", ")
	}
	return out
}

func httpVersion(proto string) string {
	return proto
}
