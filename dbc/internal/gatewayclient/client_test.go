package gatewayclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"example.invalid/sre/dbc/internal/gatewayproto"
)

type staticTokenSource struct {
	token    string
	err      error
	audience string
}

func (s *staticTokenSource) Token(_ context.Context, audience string) (string, error) {
	s.audience = audience
	return s.token, s.err
}

func TestDialUsesTLSAndAuthenticatesBeforeReturning(t *testing.T) {
	cert, roots := testCertificate(t, "dbc-gateway.local")
	listener, err := tls.Listen("tcp", "0.0.0.0:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13})
	if err != nil {
		t.Fatalf("tls.Listen() error = %v", err)
	}
	defer listener.Close()

	requestCh := make(chan gatewayproto.ClientAuth, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		var request gatewayproto.ClientAuth
		if gatewayproto.ReadFrame(conn, &request) == nil {
			requestCh <- request
			_ = gatewayproto.WriteFrame(conn, gatewayproto.AuthResult{OK: true, SessionID: "session-1"})
		}
	}()

	tokens := &staticTokenSource{token: [REDACTED]"}
	client := &Client{Config: Config{
		Endpoint: listener.Addr().String(), ServerName: "dbc-gateway.local", Audience: "dbc-gateway",
		Target: "baremetal-vplus33", ClientVersion: "dev", RootCAs: roots,
	}, TokenSource: tokens}
	conn, err := client.Dial(context.Background())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	request := <-requestCh
	if request.IDToken != "id-token" || request.Target != "baremetal-vplus33" || request.ClientVersion != "dev" {
		t.Fatalf("request = %#v", request)
	}
	if tokens.audience != "dbc-gateway" {
		t.Fatalf("token audience = %q", tokens.audience)
	}
}

func TestDialRejectsCertificateForDifferentSNI(t *testing.T) {
	cert, roots := testCertificate(t, "different.local")
	listener, err := tls.Listen("tcp", "0.0.0.0:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13})
	if err != nil {
		t.Fatalf("tls.Listen() error = %v", err)
	}
	defer listener.Close()
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
			var request gatewayproto.ClientAuth
			_ = gatewayproto.ReadFrame(conn, &request)
		}
	}()

	client := &Client{Config: Config{
		Endpoint: listener.Addr().String(), ServerName: "dbc-gateway.local", Audience: "dbc-gateway", Target: "db", RootCAs: roots,
	}, TokenSource: &staticTokenSource{token: [REDACTED]"}}
	if _, err := client.Dial(context.Background()); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("Dial() error = %v, want certificate error", err)
	}
}

func TestConfigRejectsIPAsSNI(t *testing.T) {
	cfg := Config{Endpoint: "0.0.0.0:8443", ServerName: "0.0.0.0", Audience: "dbc-gateway", Target: "db"}
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "不能是 IP") {
		t.Fatalf("validate() error = %v", err)
	}
}

func testCertificate(t *testing.T, dnsName string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: dnsName}, DNSNames: []string{dnsName},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair() error = %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(template)
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	roots = x509.NewCertPool()
	roots.AddCert(parsed)
	return cert, roots
}
