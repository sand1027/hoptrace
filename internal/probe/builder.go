package probe

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// RequestBuilder fluently assembles a validated ProbeRequest (Builder pattern).
type RequestBuilder struct {
	req ProbeRequest
	err error
}

// NewRequestBuilder starts a builder with sensible defaults.
func NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{
		req: ProbeRequest{
			Method:       "GET",
			Headers:      map[string]string{},
			Timeout:      30 * time.Second,
			MaxRedirects: 10,
			MaxBodyBytes: 10 << 20,
			SLO:          map[string]float64{},
		},
	}
}

func (b *RequestBuilder) URL(raw string) *RequestBuilder {
	if b.err != nil {
		return b
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		b.err = fmt.Errorf("url is required")
		return b
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		b.err = fmt.Errorf("invalid url: %s", raw)
		return b
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		b.err = fmt.Errorf("unsupported scheme: %s", u.Scheme)
		return b
	}
	b.req.URL = u.String()
	return b
}

func (b *RequestBuilder) Method(m string) *RequestBuilder {
	if b.err != nil {
		return b
	}
	m = strings.ToUpper(strings.TrimSpace(m))
	switch m {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		b.req.Method = m
	default:
		b.err = fmt.Errorf("unsupported method: %s", m)
	}
	return b
}

func (b *RequestBuilder) Header(key, value string) *RequestBuilder {
	if b.err != nil {
		return b
	}
	if b.req.Headers == nil {
		b.req.Headers = map[string]string{}
	}
	b.req.Headers[key] = value
	return b
}

func (b *RequestBuilder) Headers(h map[string]string) *RequestBuilder {
	for k, v := range h {
		b.Header(k, v)
	}
	return b
}

func (b *RequestBuilder) Body(data []byte) *RequestBuilder {
	if b.err != nil {
		return b
	}
	b.req.Body = data
	if len(data) > 0 && b.req.Method == "GET" {
		b.req.Method = "POST"
	}
	return b
}

func (b *RequestBuilder) FollowRedirect(v bool) *RequestBuilder {
	b.req.FollowRedirect = v
	return b
}

func (b *RequestBuilder) Timeout(d time.Duration) *RequestBuilder {
	if b.err != nil {
		return b
	}
	if d <= 0 {
		b.err = fmt.Errorf("timeout must be positive")
		return b
	}
	b.req.Timeout = d
	return b
}

func (b *RequestBuilder) Proxy(raw string) *RequestBuilder {
	b.req.ProxyURL = &raw
	return b
}

func (b *RequestBuilder) IgnoreSSL(v bool) *RequestBuilder {
	b.req.IgnoreSSL = v
	return b
}

func (b *RequestBuilder) CABundle(path string) *RequestBuilder {
	b.req.CABundlePath = path
	return b
}

func (b *RequestBuilder) MaxRedirects(n int) *RequestBuilder {
	if n > 0 {
		b.req.MaxRedirects = n
	}
	return b
}

func (b *RequestBuilder) MaxBodyBytes(n int64) *RequestBuilder {
	if b.err != nil {
		return b
	}
	if n > 0 {
		b.req.MaxBodyBytes = n
	}
	return b
}

func (b *RequestBuilder) SLO(thresholds map[string]float64) *RequestBuilder {
	if b.err != nil {
		return b
	}
	allowed := map[string]struct{}{
		"dns": {}, "connect": {}, "tls": {}, "ttfb": {},
		"wait": {}, "xfer": {}, "total": {},
	}
	for k, v := range thresholds {
		key := strings.ToLower(k)
		if _, ok := allowed[key]; !ok {
			b.err = fmt.Errorf("unknown slo key: %s", k)
			return b
		}
		if v <= 0 {
			b.err = fmt.Errorf("slo %s must be positive", k)
			return b
		}
		b.req.SLO[key] = v
	}
	return b
}

// Build returns the validated ProbeRequest.
func (b *RequestBuilder) Build() (ProbeRequest, error) {
	if b.err != nil {
		return ProbeRequest{}, b.err
	}
	if b.req.URL == "" {
		return ProbeRequest{}, fmt.Errorf("url is required")
	}
	if b.req.IgnoreSSL && b.req.CABundlePath != "" {
		return ProbeRequest{}, fmt.Errorf("--ignore-ssl and --cacert are mutually exclusive")
	}
	// shallow copy headers
	headers := make(map[string]string, len(b.req.Headers))
	for k, v := range b.req.Headers {
		headers[k] = v
	}
	out := b.req
	out.Headers = headers
	slo := make(map[string]float64, len(b.req.SLO))
	for k, v := range b.req.SLO {
		slo[k] = v
	}
	out.SLO = slo
	return out, nil
}
