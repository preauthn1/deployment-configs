package gateway

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCertificateReloaderKeepsLastValidCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	writeTestCertificate(t, certPath, keyPath, 1)

	reloader, err := NewCertificateReloader(certPath, keyPath)
	if err != nil {
		t.Fatalf("NewCertificateReloader() error = %v", err)
	}
	before, err := reloader.TLSConfig().GetCertificate(nil)
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}
	if err := os.WriteFile(certPath, []byte("invalid"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := reloader.Reload(); err == nil {
		t.Fatal("Reload() error = nil, want invalid certificate error")
	}
	after, err := reloader.TLSConfig().GetCertificate(nil)
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}
	if before != after {
		t.Fatal("invalid reload replaced the last valid certificate")
	}
}

func TestCertificateWatchReloadsRenewedSecret(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	writeTestCertificate(t, certPath, keyPath, 1)
	reloader, err := NewCertificateReloader(certPath, keyPath)
	if err != nil {
		t.Fatalf("NewCertificateReloader() error = %v", err)
	}
	before, _ := reloader.TLSConfig().GetCertificate(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go reloader.Watch(ctx, 5*time.Millisecond, nil)
	writeTestCertificate(t, certPath, keyPath, 2)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		after, _ := reloader.TLSConfig().GetCertificate(nil)
		if after != before {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("certificate was not reloaded")
}

func writeTestCertificate(t *testing.T, certPath, keyPath string, serial int64) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "dbc-gateway.local"},
		DNSNames: []string{"dbc-gateway.local"}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
}
