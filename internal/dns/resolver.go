package dns

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sandeepv/hoptrace/internal/probe"
)

// SystemResolver implements probe.DNSResolver using the OS resolver (Strategy).
type SystemResolver struct {
	Resolver *net.Resolver
}

func NewSystemResolver() *SystemResolver {
	return &SystemResolver{Resolver: net.DefaultResolver}
}

func (r *SystemResolver) Resolve(ctx context.Context, host, port string) (string, string, float64, error) {
	start := time.Now()
	addrs, err := r.Resolver.LookupIPAddr(ctx, host)
	elapsed := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return "", "", elapsed, fmt.Errorf("dns lookup %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return "", "", elapsed, fmt.Errorf("dns lookup %s: no addresses", host)
	}

	// Prefer IPv4 for predictability, fall back to first result.
	var chosen net.IPAddr
	family := "IPv4"
	for _, a := range addrs {
		if a.IP.To4() != nil {
			chosen = a
			family = "IPv4"
			break
		}
	}
	if chosen.IP == nil {
		chosen = addrs[0]
		if chosen.IP.To4() == nil {
			family = "IPv6"
		}
	}

	_ = port // reserved for future Happy Eyeballs / dial hints
	var _ probe.DNSResolver = r
	return chosen.IP.String(), family, elapsed, nil
}
