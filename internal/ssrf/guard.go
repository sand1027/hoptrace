package ssrf

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Guard blocks probes to private / link-local / metadata addresses.
type Guard struct {
	AllowPrivate bool
}

func NewGuard(allowPrivate bool) *Guard {
	return &Guard{AllowPrivate: allowPrivate}
}

// ValidateURL returns an error if the URL host resolves to a blocked address family.
func (g *Guard) ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url missing host")
	}
	if g.AllowPrivate {
		return nil
	}

	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") || lower == "metadata.google.internal" {
		return fmt.Errorf("ssrf: blocked host %s", host)
	}

	// If host is a literal IP, check immediately.
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("ssrf: blocked ip %s", ip)
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		// Let the probe fail naturally later; don't block on DNS failure here.
		return nil
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
	// Cloud metadata
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}
