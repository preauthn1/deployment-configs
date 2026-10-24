package gateway

import (
	"context"
	"net"
	"testing"
	"time"

	"example.invalid/sre/dbc/internal/gatewayproto"
)

// TestHandleConnPerformsRealMySQLAuthTermination wires handleConn up with
// the *real* AuthenticateClient/AuthenticateBackend (unlike newTestServer's
// no-ops, used by server_test.go's tests for other things) and a
// fakeMySQLBackend (from mysqlauth_test.go) standing in for the real
// backend, proving the whole chain — gatewayproto auth frame → Authorize →
// real backend MySQL handshake with the role's stored credential → fake
// client-facing MySQL handshake → command-phase relay — works together,
// not just in isolation.
func TestHandleConnPerformsRealMySQLAuthTermination(t *testing.T) {
	authorizer, _ := testAuthorizer() // "db" target, single role: dbc_readonly
	cfg := authorizer.Config
	cfg.AuthTimeout = 2 * time.Second
	cfg.DialTimeout = 2 * time.Second

	s := NewServer(cfg, authorizer)
	relayed := make(chan struct{})
	s.Relay = func(client, backend net.Conn, authz *Authorization) error {
		close(relayed)
		relayBothWays(client, backend)
		return nil
	}

	backendForGateway, backendForFake := net.Pipe()
	s.DialContext = func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != "0.0.0.0:3306" {
			t.Fatalf("DialContext(%q, %q)", network, address)
		}
		return backendForGateway, nil
	}

	fakeBackend := &fakeMySQLBackend{
		plugin:       pluginMySQLNativePassword,
		scramble:     bytesRange(20, 11),
		wantUser:     "dbc_readonly", // must match testAuthorizer's role credential
		wantPassword: [REDACTED]",
	}
	backendErrCh := make(chan error, 1)
	go func() { backendErrCh <- fakeBackend.run(backendForFake) }()

	clientSide, serverSide := net.Pipe()
	go s.handleConn(context.Background(), serverSide)

	if err := gatewayproto.WriteFrame(clientSide, gatewayproto.ClientAuth{
		Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db",
	}); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	var result gatewayproto.AuthResult
	if err := gatewayproto.ReadFrame(clientSide, &result); err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if !result.OK || result.SessionID == "" {
		t.Fatalf("auth result = %#v", result)
	}

	// From here the client speaks raw MySQL protocol: read the gateway's
	// fake handshake, respond, get an OK, then confirm the command-phase
	// relay actually connects through to the fake backend.
	if _, payload, err := readWirePacket(clientSide); err != nil || len(payload) == 0 || payload[0] != 0x0a {
		t.Fatalf("reading fake client handshake: payload=%v err=%v", payload, err)
	}
	response := make([]byte, 32)
	caps := uint32(clientProtocol41 | clientSecureConn)
	response[0], response[1], response[2], response[3] = byte(caps), byte(caps>>8), byte(caps>>16), byte(caps>>24)
	if err := writeWirePacket(clientSide, 1, response); err != nil {
		t.Fatalf("writing client handshake response: %v", err)
	}
	if _, payload, err := readWirePacket(clientSide); err != nil || !isOKPacket(payload) {
		t.Fatalf("reading fake OK: payload=%v err=%v", payload, err)
	}

	<-relayed
	if err := <-backendErrCh; err != nil {
		t.Fatalf("fake backend: %v", err)
	}

	want := []byte("COM_QUERY-ish bytes")
	if err := writeWirePacket(clientSide, 0, want); err != nil {
		t.Fatalf("writing command-phase packet: %v", err)
	}
	_, got, err := readWirePacket(backendForFake)
	if err != nil {
		t.Fatalf("reading relayed packet at backend: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("backend received %q, want %q", got, want)
	}

	_ = clientSide.Close()
	_ = backendForFake.Close()
}

// TestHandleConnToleratesClientQueryAttributesClaim proves a client that
// claims CLIENT_QUERY_ATTRIBUTES (mysql2 — Beekeeper Studio's underlying
// driver — does this unconditionally, confirmed live 2026-09-09) still
// completes the full handshake and reaches Relay normally, end to end
// through the real handleConn wiring — not just authenticateClient in
// isolation. See mysqlauth.go's fakeCapabilities doc for why this is
// tolerated rather than granted.
func TestHandleConnToleratesClientQueryAttributesClaim(t *testing.T) {
	authorizer, _ := testAuthorizer()
	cfg := authorizer.Config
	cfg.AuthTimeout = 2 * time.Second
	cfg.DialTimeout = 2 * time.Second

	s := NewServer(cfg, authorizer)
	relayed := make(chan struct{})
	s.Relay = func(client, backend net.Conn, authz *Authorization) error {
		close(relayed)
		relayBothWays(client, backend)
		return nil
	}

	backendForGateway, backendForFake := net.Pipe()
	s.DialContext = func(_ context.Context, network, address string) (net.Conn, error) {
		return backendForGateway, nil
	}
	fakeBackend := &fakeMySQLBackend{
		plugin:       pluginMySQLNativePassword,
		scramble:     bytesRange(20, 11),
		wantUser:     "dbc_readonly",
		wantPassword: [REDACTED]",
	}
	backendErrCh := make(chan error, 1)
	go func() { backendErrCh <- fakeBackend.run(backendForFake) }()

	clientSide, serverSide := net.Pipe()
	go s.handleConn(context.Background(), serverSide)

	if err := gatewayproto.WriteFrame(clientSide, gatewayproto.ClientAuth{
		Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db",
	}); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	var result gatewayproto.AuthResult
	if err := gatewayproto.ReadFrame(clientSide, &result); err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if !result.OK {
		t.Fatalf("auth result = %#v", result)
	}

	if _, payload, err := readWirePacket(clientSide); err != nil || len(payload) == 0 || payload[0] != 0x0a {
		t.Fatalf("reading fake client handshake: payload=%v err=%v", payload, err)
	}
	response := make([]byte, 32)
	caps := clientProtocol41 | clientSecureConn | clientQueryAttribute
	response[0], response[1], response[2], response[3] = byte(caps), byte(caps>>8), byte(caps>>16), byte(caps>>24)
	if err := writeWirePacket(clientSide, 1, response); err != nil {
		t.Fatalf("writing client handshake response: %v", err)
	}
	if _, payload, err := readWirePacket(clientSide); err != nil || !isOKPacket(payload) {
		t.Fatalf("reading fake OK: payload=%v err=%v (claiming CLIENT_QUERY_ATTRIBUTES must not be rejected)", payload, err)
	}

	<-relayed
	if err := <-backendErrCh; err != nil {
		t.Fatalf("fake backend: %v", err)
	}

	_ = clientSide.Close()
	_ = backendForFake.Close()
}

// TestHandleConnRejectsWrongBackendCredential proves a role whose stored
// credential the backend doesn't actually accept surfaces as the same
// generic "database backend unavailable" the dial-failure path uses — not
// a different, more revealing error — and that the client never gets an
// AuthResult{OK:true} in that case.
func TestHandleConnRejectsWrongBackendCredential(t *testing.T) {
	authorizer, _ := testAuthorizer()
	cfg := authorizer.Config
	cfg.AuthTimeout = 2 * time.Second
	cfg.DialTimeout = 2 * time.Second

	s := NewServer(cfg, authorizer)

	backendForGateway, backendForFake := net.Pipe()
	s.DialContext = func(context.Context, string, string) (net.Conn, error) {
		return backendForGateway, nil
	}

	// A minimal backend that accepts any handshake response and then
	// rejects it — simulating a real MySQL server's "Access denied" for a
	// stored credential that's gone stale (e.g. rotated on the backend but
	// not yet updated in gateway.yaml).
	go func() {
		_ = writeWirePacket(backendForFake, 0, buildBackendHandshake(bytesRange(20, 5), pluginMySQLNativePassword, true))
		if _, _, err := readWirePacket(backendForFake); err != nil {
			return
		}
		errPayload := append([]byte{0xff, 0x15, 0x04, '#'}, append([]byte("28000"), "Access denied"...)...)
		_ = writeWirePacket(backendForFake, 2, errPayload)
	}()

	clientSide, serverSide := net.Pipe()
	go s.handleConn(context.Background(), serverSide)

	if err := gatewayproto.WriteFrame(clientSide, gatewayproto.ClientAuth{
		Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db",
	}); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	var result gatewayproto.AuthResult
	if err := gatewayproto.ReadFrame(clientSide, &result); err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if result.OK {
		t.Fatal("expected auth to fail when the backend rejects the role's stored credential")
	}
	if result.Error != "database backend unavailable" {
		t.Fatalf("result.Error = %q, want the generic backend-unavailable message (not a detail leak)", result.Error)
	}

	_ = clientSide.Close()
	_ = backendForFake.Close()
}
