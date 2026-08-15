package dns

import (
	"context"
	"net"
	"testing"
)

func TestSystemResolver_ResolveLocalhost(t *testing.T) {
	r := NewSystemResolver()
	ip, family, ms, err := r.Resolve(context.Background(), "localhost", "80")
	if err != nil {
		t.Fatal(err)
	}
	if net.ParseIP(ip) == nil {
		t.Fatalf("bad ip %q", ip)
	}
	if family != "IPv4" && family != "IPv6" {
		t.Fatalf("family=%s", family)
	}
	if ms < 0 {
		t.Fatalf("ms=%v", ms)
	}
}
