package ssrf_test

import (
	"net"
	"testing"

	"github.com/sandeepv/hoptrace/internal/ssrf"
)

func TestGuard_BlocksLocalhostAndPrivateIPs(t *testing.T) {
	g := ssrf.NewGuard(false)
	for _, raw := range []string{
		"http://localhost/x",
		"http://127.0.0.1/x",
		"http://10.0.0.1/x",
		"http://192.168.1.1/x",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/",
		"ftp://example.com/",
	} {
		if err := g.ValidateURL(raw); err == nil {
			t.Fatalf("expected block for %s", raw)
		}
	}
}

func TestGuard_AllowsPublicLiteral(t *testing.T) {
	g := ssrf.NewGuard(false)
	if err := g.ValidateURL("https://8.8.8.8/"); err != nil {
		t.Fatal(err)
	}
}

func TestGuard_FailClosedOnDNSError(t *testing.T) {
	g := ssrf.NewGuard(false)
	g.Lookup = func(host string) ([]net.IP, error) {
		return nil, net.UnknownNetworkError("boom")
	}
	if err := g.ValidateURL("https://evil.example/"); err == nil {
		t.Fatal("expected fail closed on dns error")
	}
}

func TestGuard_BlocksHostnameResolvingPrivate(t *testing.T) {
	g := ssrf.NewGuard(false)
	g.Lookup = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.1.2.3")}, nil
	}
	if err := g.ValidateURL("https://internal.corp/"); err == nil {
		t.Fatal("expected block")
	}
}

func TestGuard_AllowPrivateBypasses(t *testing.T) {
	g := ssrf.NewGuard(true)
	if err := g.ValidateURL("http://127.0.0.1/"); err != nil {
		t.Fatal(err)
	}
}
