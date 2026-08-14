package tlsinfo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/sandeepv/hoptrace/internal/probe"
)

// SocketInspector dials TLS and reads certificate metadata (Strategy).
type SocketInspector struct{}

func NewSocketInspector() *SocketInspector {
	return &SocketInspector{}
}

func (i *SocketInspector) Inspect(ctx context.Context, host, addr string, ignoreSSL bool, caBundle string) (*probe.TLSInfo, float64, error) {
	start := time.Now()

	tlsCfg := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: ignoreSSL, //nolint:gosec // intentional for troubleshooting
		MinVersion:         tls.VersionTLS12,
	}
	if caBundle != "" {
		pem, err := os.ReadFile(caBundle)
		if err != nil {
			return nil, 0, fmt.Errorf("read ca bundle: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, 0, fmt.Errorf("invalid ca bundle: %s", caBundle)
		}
		tlsCfg.RootCAs = pool
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, float64(time.Since(start).Microseconds()) / 1000.0, err
	}
	defer rawConn.Close()

	conn := tls.Client(rawConn, tlsCfg)
	if err := conn.HandshakeContext(ctx); err != nil {
		return nil, float64(time.Since(start).Microseconds()) / 1000.0, err
	}
	defer conn.Close()

	state := conn.ConnectionState()
	info := &probe.TLSInfo{
		Version:  tlsVersionName(state.Version),
		Cipher:   tls.CipherSuiteName(state.CipherSuite),
		Verified: !ignoreSSL,
	}
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		info.CertCN = cert.Subject.CommonName
		if info.CertCN == "" && len(cert.DNSNames) > 0 {
			info.CertCN = cert.DNSNames[0]
		}
		days := int(time.Until(cert.NotAfter).Hours() / 24)
		info.CertDaysLeft = &days
	}

	elapsed := float64(time.Since(start).Microseconds()) / 1000.0
	var _ probe.TLSInspector = i
	return info, elapsed, nil
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "TLSv1.3"
	case tls.VersionTLS12:
		return "TLSv1.2"
	case tls.VersionTLS11:
		return "TLSv1.1"
	case tls.VersionTLS10:
		return "TLSv1.0"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}

// FromConnectionState adapts an existing tls.ConnectionState into TLSInfo (Adapter helper).
func FromConnectionState(state tls.ConnectionState, verified bool) *probe.TLSInfo {
	info := &probe.TLSInfo{
		Version:  tlsVersionName(state.Version),
		Cipher:   tls.CipherSuiteName(state.CipherSuite),
		Verified: verified,
	}
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		info.CertCN = cert.Subject.CommonName
		if info.CertCN == "" && len(cert.DNSNames) > 0 {
			info.CertCN = cert.DNSNames[0]
		}
		days := int(time.Until(cert.NotAfter).Hours() / 24)
		info.CertDaysLeft = &days
	}
	return info
}

// NormalizeHost strips port from host:port if present.
func NormalizeHost(hostport string) string {
	h, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return strings.TrimSpace(hostport)
	}
	return h
}
