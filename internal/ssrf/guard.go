package ssrf

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// LookupFunc resolves a hostname to IPs (injectable for tests).
type LookupFunc func(host string) ([]net.IP, error)

// Guard blocks probes to private / link-local / metadata addresses.
type Guard struct {
	AllowPrivate bool
	Lookup       LookupFunc
}

func NewGuard(allowPrivate bool) *Guard {
	return &Guard{
		AllowPrivate: allowPrivate,
		Lookup:       net.LookupIP,
	}
}

// ValidateURL returns an error if the URL host is blocked or resolves to a blocked address.
// DNS failures fail closed (blocked) unless AllowPrivate is set.
func (g *Guard) ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url missing host")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "" && scheme != "http" && scheme != "https" {
		return fmt.Errorf("ssrf: blocked scheme %s", scheme)
	}
	if g.AllowPrivate {
		return nil
	}

	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		lower == "metadata.google.internal" || lower == "metadata" {
		return fmt.Errorf("ssrf: blocked host %s", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("ssrf: blocked ip %s", ip)
		}
		return nil
	}

	lookup := g.Lookup
	if lookup == nil {
		lookup = net.LookupIP
	}
	ips, err := lookup(host)
	if err != nil {
		return fmt.Errorf("ssrf: dns lookup failed for %s (fail closed): %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("ssrf: host %s resolved to no addresses", host)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("ssrf: host %s resolves to blocked ip %s", host, ip)
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		// CGNAT / documentation ranges often abused
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}
