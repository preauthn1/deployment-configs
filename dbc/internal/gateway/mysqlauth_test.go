package gateway

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
	"testing"
)

// scrambleNative's test vectors were computed independently in Python
// (hashlib), not derived from this file's own implementation, so this is a
// real cross-check rather than a tautology.
func TestScrambleNativeKnownVectors(t *testing.T) {
	scramble := []byte{10, 47, 74, 111, 75, 73, 34, 48, 88, 76, 114, 74, 37, 13, 3, 80, 82, 2, 23, 21}
	tests := []struct {
		password string
		want     string
	}{
		{"secret", "6a149bdd80bda1ebf0fa2bd2cf2e9717fecc34bb"},
		{"secret2", "650f07df353cce5370eea34d580f2e9118818b56"},
	}
	for _, tt := range tests {
		got := scrambleNative(tt.password, scramble)
		if hex.EncodeToString(got) != tt.want {
			t.Errorf("scrambleNative(%q) = %x, want %s", tt.password, got, tt.want)
		}
	}
}

func TestScrambleNativeEmptyPassword(t *testing.T) {
	if got := scrambleNative("", []byte{1, 2, 3}); got != nil {
		t.Errorf("scrambleNative(empty) = %x, want nil", got)
	}
}

// scrambleCachingSHA2's test vectors are go-sql-driver/mysql's own
// (auth_test.go's TestScrambleSHA256Pass) — that library is already a
// dependency of this module and its scramble implementation is
// battle-tested against real MySQL 8 servers, so matching it byte-for-byte
// is a strong correctness signal.
func TestScrambleCachingSHA2KnownVectors(t *testing.T) {
	scramble := []byte{10, 47, 74, 111, 75, 73, 34, 48, 88, 76, 114, 74, 37, 13, 3, 80, 82, 2, 23, 21}
	tests := []struct {
		password string
		want     string
	}{
		{"secret", "f490e76f66d9d86665ce54d98c78d0acfe2fb0b08b423da807144873d30b312c"},
		{"secret2", "abc3934a012cf342e876071c8ee202de51785b430258a7a0138bc79c4d800bc6"},
	}
	for _, tt := range tests {
		got := scrambleCachingSHA2(tt.password, scramble)
		if hex.EncodeToString(got) != tt.want {
			t.Errorf("scrambleCachingSHA2(%q) = %x, want %s", tt.password, got, tt.want)
		}
	}
}

func TestScrambleCachingSHA2EmptyPassword(t *testing.T) {
	if got := scrambleCachingSHA2("", []byte{1, 2, 3}); got != nil {
		t.Errorf("scrambleCachingSHA2(empty) = %x, want nil", got)
	}
}

// TestEncryptPasswordForRSARoundTrips generates a real keypair (this test
// owns both halves, unlike production where dbc-gateway only ever sees the
// backend's public key) and proves decrypting + un-XOR-ing recovers the
// original password — the only way to genuinely verify OAEP encryption
// output, since it's randomized and can't be checked against a fixed
// vector.
func TestEncryptPasswordForRSARoundTrips(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	scramble := []byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
	password := "s3cret-passw0rd"

	ciphertext, err := encryptPasswordForRSA(password, scramble, &key.PublicKey)
	if err != nil {
		t.Fatalf("encryptPasswordForRSA: %v", err)
	}
	if len(ciphertext) != key.Size() {
		t.Fatalf("ciphertext length = %d, want %d (RSA modulus size)", len(ciphertext), key.Size())
	}

	plain, err := rsa.DecryptOAEP(sha1.New(), nil, key, ciphertext, nil)
	if err != nil {
		t.Fatalf("DecryptOAEP: %v", err)
	}
	if len(plain) != len(password)+1 {
		t.Fatalf("decrypted length = %d, want %d (password + NUL)", len(plain), len(password)+1)
	}
	for i := range plain {
		plain[i] ^= scramble[i%len(scramble)]
	}
	if string(plain[:len(password)]) != password || plain[len(password)] != 0 {
		t.Fatalf("recovered plaintext = %q (%v), want %q + NUL", plain[:len(password)], plain, password)
	}
}

func TestParseRSAPublicKeyPEMRoundTrips(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})

	got, err := parseRSAPublicKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("parseRSAPublicKeyPEM: %v", err)
	}
	if got.N.Cmp(key.PublicKey.N) != 0 || got.E != key.PublicKey.E {
		t.Fatal("parsed public key doesn't match the original")
	}
}

func TestParseRSAPublicKeyPEMRejectsGarbage(t *testing.T) {
	if _, err := parseRSAPublicKeyPEM([]byte("not a pem")); err == nil {
		t.Fatal("expected an error for non-PEM input")
	}
}

// buildBackendHandshake constructs a HandshakeV10 payload the way a real
// MySQL server would, for feeding into parseBackendHandshake in tests.
func buildBackendHandshake(scramble []byte, plugin string, pluginAuthCapability bool) []byte {
	if len(scramble) != 20 {
		panic("test helper assumes a 20-byte scramble")
	}
	var caps uint32 = clientProtocol41 | clientSecureConn
	if pluginAuthCapability {
		caps |= clientPluginAuth
	}

	var buf bytes.Buffer
	buf.WriteByte(0x0a)
	buf.WriteString("8.0.99-test")
	buf.WriteByte(0)
	buf.Write([]byte{1, 0, 0, 0}) // thread id
	buf.Write(scramble[:8])
	buf.WriteByte(0) // filler
	buf.WriteByte(byte(caps))
	buf.WriteByte(byte(caps >> 8))
	buf.WriteByte(0x2d) // character_set
	buf.Write([]byte{2, 0})
	buf.WriteByte(byte(caps >> 16))
	buf.WriteByte(byte(caps >> 24))
	if pluginAuthCapability {
		buf.WriteByte(21)
	} else {
		buf.WriteByte(0)
	}
	buf.Write(make([]byte, 10))
	buf.Write(scramble[8:20])
	buf.WriteByte(0)
	if pluginAuthCapability {
		buf.WriteString(plugin)
		buf.WriteByte(0)
	}
	return buf.Bytes()
}

func TestParseBackendHandshakeRoundTrips(t *testing.T) {
	scramble := make([]byte, 20)
	for i := range scramble {
		scramble[i] = byte(i + 1)
	}
	payload := buildBackendHandshake(scramble, pluginCachingSHA2Password, true)

	gotScramble, gotPlugin, err := parseBackendHandshake(payload)
	if err != nil {
		t.Fatalf("parseBackendHandshake: %v", err)
	}
	if !bytes.Equal(gotScramble, scramble) {
		t.Errorf("scramble = %v, want %v", gotScramble, scramble)
	}
	if gotPlugin != pluginCachingSHA2Password {
		t.Errorf("plugin = %q, want %q", gotPlugin, pluginCachingSHA2Password)
	}
}

func TestParseBackendHandshakeNoPluginAuth(t *testing.T) {
	scramble := make([]byte, 20)
	for i := range scramble {
		scramble[i] = byte(i + 1)
	}
	payload := buildBackendHandshake(scramble, "", false)

	gotScramble, gotPlugin, err := parseBackendHandshake(payload)
	if err != nil {
		t.Fatalf("parseBackendHandshake: %v", err)
	}
	if !bytes.Equal(gotScramble, scramble) {
		t.Errorf("scramble = %v, want %v", gotScramble, scramble)
	}
	if gotPlugin != "" {
		t.Errorf("plugin = %q, want empty (capability not advertised)", gotPlugin)
	}
}

func TestParseBackendHandshakeRejectsBadProtocolVersion(t *testing.T) {
	if _, _, err := parseBackendHandshake([]byte{0x09, 0, 0, 0}); err == nil {
		t.Fatal("expected an error for a non-v10 handshake")
	}
}

func TestParseBackendHandshakeRejectsTruncated(t *testing.T) {
	full := buildBackendHandshake(make([]byte, 20), pluginMySQLNativePassword, true)
	for _, n := range []int{0, 1, 5, 15, 30} {
		if n > len(full) {
			continue
		}
		if _, _, err := parseBackendHandshake(full[:n]); err == nil {
			t.Fatalf("truncated to %d bytes: expected an error, got none", n)
		}
	}
}

func TestBuildFakeHandshakeAndAuthenticateClient(t *testing.T) {
	clientConn, gatewayConn := net.Pipe()
	defer clientConn.Close()
	defer gatewayConn.Close()
	// No CLIENT_CONNECT_WITH_DB in this test's handshake response, so
	// backend is never touched — an unused pipe end is enough.
	backendConn, _ := net.Pipe()
	defer backendConn.Close()

	done := make(chan error, 1)
	go func() { done <- authenticateClient(gatewayConn, backendConn) }()

	// Act as the "client": read the fake handshake, send a plausible
	// Handshake Response (capabilities without CLIENT_QUERY_ATTRIBUTES),
	// then read the OK.
	_, hsPayload, err := readWirePacket(clientConn)
	if err != nil {
		t.Fatalf("reading fake handshake: %v", err)
	}
	if hsPayload[0] != 0x0a {
		t.Fatalf("handshake protocol version = %#x, want 0x0a", hsPayload[0])
	}

	response := make([]byte, 32)
	caps := uint32(clientProtocol41 | clientSecureConn)
	response[0], response[1], response[2], response[3] = byte(caps), byte(caps>>8), byte(caps>>16), byte(caps>>24)
	if err := writeWirePacket(clientConn, 1, response); err != nil {
		t.Fatalf("writing handshake response: %v", err)
	}

	// authenticateClient's final write (the OK packet) blocks on net.Pipe
	// until read, so this must happen before waiting on <-done — otherwise
	// both sides block forever (the goroutine on the blocked write, this
	// one on <-done).
	_, okPayload, err := readWirePacket(clientConn)
	if err != nil {
		t.Fatalf("reading OK packet: %v", err)
	}
	if !isOKPacket(okPayload) {
		t.Fatalf("final packet = %v, want an OK packet", okPayload)
	}

	if err := <-done; err != nil {
		t.Fatalf("authenticateClient: %v", err)
	}
}

func TestBuildFakeHandshakeAdvertisesMultiStatements(t *testing.T) {
	// GUI tools (Beekeeper Studio and similar) often batch a whole editor's
	// worth of semicolon-separated statements into one COM_QUERY, which
	// requires the server to have negotiated CLIENT_MULTI_STATEMENTS —
	// without it, the real backend rejects the batch with a plain SQL
	// syntax error (see queryaudit.go's observeQueryResponsePacket for how
	// the audit side copes with the resulting multiple responses).
	// buildFakeHandshake returns the raw packet payload (no 4-byte wire
	// header), so it's parsed directly here — not via readWirePacket.
	payload := buildFakeHandshake(make([]byte, 20))
	// Handshake layout: protocol_version(1), server_version(NUL-term),
	// thread_id(4), auth-plugin-data-part1(8), filler(1), then the lower
	// 2 bytes of capability_flags, then upper 2 bytes after
	// character_set(1)+status_flags(2).
	i := bytes.IndexByte(payload[1:], 0) + 2 // past protocol_version+server_version
	i += 4 + 8 + 1                           // thread_id, auth-plugin-data part1, filler
	lower := uint32(payload[i]) | uint32(payload[i+1])<<8
	i += 2 + 1 + 2 // capability_flags_lower, character_set, status_flags
	upper := uint32(payload[i]) | uint32(payload[i+1])<<8
	capabilities := lower | upper<<16
	if capabilities&clientMultiStatements == 0 {
		t.Fatalf("fake handshake capabilities = %#x, want CLIENT_MULTI_STATEMENTS set", capabilities)
	}
}

// TestAuthenticateClientToleratesQueryAttributesClaim covers what used to be
// an outright rejection: mysql2 (Beekeeper Studio's underlying Node.js
// driver, confirmed live 2026-09-09) claims CLIENT_QUERY_ATTRIBUTES
// unconditionally regardless of what the server offers, so every one of its
// connections was rejected at the handshake. Fixed by tolerating rather than
// rejecting the claim — not by granting it (fakeCapabilities still never
// offers it, so the negotiated capability for the session is still off
// regardless of what the client claimed; see that doc for why this is safe
// to just ignore rather than needing to plumb through to the audit layer).
func TestAuthenticateClientToleratesQueryAttributesClaim(t *testing.T) {
	clientConn, gatewayConn := net.Pipe()
	defer clientConn.Close()
	defer gatewayConn.Close()
	backendConn, _ := net.Pipe()
	defer backendConn.Close()

	done := make(chan error, 1)
	go func() { done <- authenticateClient(gatewayConn, backendConn) }()

	if _, _, err := readWirePacket(clientConn); err != nil {
		t.Fatalf("reading fake handshake: %v", err)
	}
	response := make([]byte, 32)
	caps := clientProtocol41 | clientSecureConn | clientQueryAttribute
	response[0] = byte(caps)
	response[1] = byte(caps >> 8)
	response[2] = byte(caps >> 16)
	response[3] = byte(caps >> 24)
	if err := writeWirePacket(clientConn, 1, response); err != nil {
		t.Fatalf("writing handshake response: %v", err)
	}

	_, okPayload, err := readWirePacket(clientConn)
	if err != nil {
		t.Fatalf("reading OK packet: %v", err)
	}
	if !isOKPacket(okPayload) {
		t.Fatalf("final packet = %v, want an OK packet (claiming CLIENT_QUERY_ATTRIBUTES must not be rejected)", okPayload)
	}

	if err := <-done; err != nil {
		t.Fatalf("authenticateClient: %v", err)
	}
}

// handshakeResponseWithDB builds a HandshakeResponse41 payload requesting
// database db via CLIENT_CONNECT_WITH_DB — same field order as
// buildHandshakeResponse, plus the optional database name.
func handshakeResponseWithDB(caps uint32, username, db string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{byte(caps), byte(caps >> 8), byte(caps >> 16), byte(caps >> 24)})
	buf.Write([]byte{0, 0, 0, 0}) // max_packet_size
	buf.WriteByte(utf8mb4GeneralCI)
	buf.Write(make([]byte, 23)) // filler
	buf.WriteString(username)
	buf.WriteByte(0)
	buf.WriteByte(0) // auth_response length: 0 (no password needed here)
	buf.WriteString(db)
	buf.WriteByte(0)
	return buf.Bytes()
}

func TestClientRequestedDatabase(t *testing.T) {
	withDB := clientProtocol41 | clientSecureConn | clientConnectWithDB
	withoutDB := clientProtocol41 | clientSecureConn

	tests := []struct {
		name    string
		payload []byte
		want    string
	}{
		{"CLIENT_CONNECT_WITH_DB set", handshakeResponseWithDB(withDB, "root", "xsgcasino"), "xsgcasino"},
		{"capability not set", handshakeResponseWithDB(withoutDB, "root", "xsgcasino"), ""},
		{"too short to contain capabilities", []byte{0x01, 0x02}, ""},
		{"empty database name", handshakeResponseWithDB(withDB, "root", ""), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clientRequestedDatabase(tt.payload); got != tt.want {
				t.Errorf("clientRequestedDatabase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuthenticateClientSelectsRequestedDatabase(t *testing.T) {
	clientConn, gatewayConn := net.Pipe()
	defer clientConn.Close()
	defer gatewayConn.Close()
	backendForGateway, backendForTest := net.Pipe()
	defer backendForTest.Close()

	done := make(chan error, 1)
	go func() { done <- authenticateClient(gatewayConn, backendForGateway) }()

	if _, _, err := readWirePacket(clientConn); err != nil {
		t.Fatalf("reading fake handshake: %v", err)
	}
	caps := clientProtocol41 | clientSecureConn | clientConnectWithDB
	if err := writeWirePacket(clientConn, 1, handshakeResponseWithDB(caps, "root", "xsgcasino")); err != nil {
		t.Fatalf("writing handshake response: %v", err)
	}

	// Act as the backend: expect COM_INIT_DB naming the requested database,
	// reply OK.
	_, initDBPayload, err := readWirePacket(backendForTest)
	if err != nil {
		t.Fatalf("reading COM_INIT_DB: %v", err)
	}
	if initDBPayload[0] != comInitDB || string(initDBPayload[1:]) != "xsgcasino" {
		t.Fatalf("COM_INIT_DB payload = %v, want comInitDB+%q", initDBPayload, "xsgcasino")
	}
	if err := writeWirePacket(backendForTest, 1, buildOKPacket()); err != nil {
		t.Fatalf("writing COM_INIT_DB OK: %v", err)
	}

	_, okPayload, err := readWirePacket(clientConn)
	if err != nil {
		t.Fatalf("reading final OK packet: %v", err)
	}
	if !isOKPacket(okPayload) {
		t.Fatalf("final packet = %v, want an OK packet", okPayload)
	}
	if err := <-done; err != nil {
		t.Fatalf("authenticateClient: %v", err)
	}
}

func TestAuthenticateClientRelaysDatabaseSelectionFailure(t *testing.T) {
	clientConn, gatewayConn := net.Pipe()
	defer clientConn.Close()
	defer gatewayConn.Close()
	backendForGateway, backendForTest := net.Pipe()
	defer backendForTest.Close()

	done := make(chan error, 1)
	go func() { done <- authenticateClient(gatewayConn, backendForGateway) }()

	if _, _, err := readWirePacket(clientConn); err != nil {
		t.Fatalf("reading fake handshake: %v", err)
	}
	caps := clientProtocol41 | clientSecureConn | clientConnectWithDB
	if err := writeWirePacket(clientConn, 1, handshakeResponseWithDB(caps, "root", "nope")); err != nil {
		t.Fatalf("writing handshake response: %v", err)
	}

	_, _, err := readWirePacket(backendForTest)
	if err != nil {
		t.Fatalf("reading COM_INIT_DB: %v", err)
	}
	errPayload := append([]byte{0xFF, 0x19, 0x04, '#'}, append([]byte("42000"), "Unknown database 'nope'"...)...)
	if err := writeWirePacket(backendForTest, 1, errPayload); err != nil {
		t.Fatalf("writing COM_INIT_DB error: %v", err)
	}

	// The client must see the real backend rejection, not a generic OK —
	// this is client-facing information about what *this* client asked
	// for, unlike AuthenticateBackend's credential failures.
	_, gotPayload, err := readWirePacket(clientConn)
	if err != nil {
		t.Fatalf("reading final packet: %v", err)
	}
	if !isErrPacket(gotPayload) {
		t.Fatalf("final packet = %v, want an ERR packet", gotPayload)
	}
	if !strings.Contains(parseErrMessage(gotPayload), "Unknown database") {
		t.Errorf("relayed error = %q, want it to mention Unknown database", parseErrMessage(gotPayload))
	}

	if err := <-done; err == nil {
		t.Fatal("authenticateClient: expected an error when backend rejects the requested database, got none")
	}
}

// fakeMySQLBackend runs a minimal, real-enough MySQL server-side handshake
// over conn for exercising authenticateBackend end-to-end without a real
// MySQL server: it sends a HandshakeV10 with the given scramble/plugin,
// verifies the client's scrambled response matches what the real algorithm
// would produce for wantPassword, and — for caching_sha2_password — walks
// through the fast-auth-success or full-authentication (RSA) paths
// depending on fullAuth.
type fakeMySQLBackend struct {
	plugin       string
	scramble     []byte
	wantUser     string
	wantPassword string
	fullAuth     bool // caching_sha2_password only: force the RSA round-trip path
	rsaKey       *rsa.PrivateKey
}

func (f *fakeMySQLBackend) run(conn net.Conn) error {
	if err := writeWirePacket(conn, 0, buildBackendHandshake(f.scramble, f.plugin, true)); err != nil {
		return err
	}
	seq, payload, err := readWirePacket(conn)
	if err != nil {
		return err
	}
	// payload is a HandshakeResponse41; verify username + auth_response.
	rest := payload[4+4+1+23:]
	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return fmt.Errorf("no username terminator")
	}
	gotUser := string(rest[:i])
	rest = rest[i+1:]
	authLen := int(rest[0])
	gotAuthResponse := rest[1 : 1+authLen]

	if gotUser != f.wantUser {
		return fmt.Errorf("username = %q, want %q", gotUser, f.wantUser)
	}

	var wantResponse []byte
	switch f.plugin {
	case pluginMySQLNativePassword:
		[REDACTED] = scrambleNative(f.wantPassword, f.scramble)
	case pluginCachingSHA2Password:
		[REDACTED] = scrambleCachingSHA2(f.wantPassword, f.scramble)
	}
	if !bytes.Equal(gotAuthResponse, wantResponse) {
		return fmt.Errorf("auth response = %x, want %x", gotAuthResponse, wantResponse)
	}

	if f.plugin == pluginMySQLNativePassword || !f.fullAuth {
		if f.plugin == pluginCachingSHA2Password {
			// Real MySQL sends fast-auth-success as its own packet,
			// followed by a separate OK packet — not folded into one.
			if err := writeWirePacket(conn, seq+1, []byte{authMoreDataMarker, cachingSha2FastAuthSuccess}); err != nil {
				return err
			}
			return writeWirePacket(conn, seq+2, buildOKPacket())
		}
		return writeWirePacket(conn, seq+1, buildOKPacket())
	}

	// Full authentication (RSA) path.
	if err := writeWirePacket(conn, seq+1, []byte{authMoreDataMarker, cachingSha2PerformFullAuth}); err != nil {
		return err
	}
	seq, reqPayload, err := readWirePacket(conn)
	if err != nil {
		return err
	}
	if len(reqPayload) != 1 || reqPayload[0] != cachingSha2RequestPubKey {
		return fmt.Errorf("expected a public-key request, got %v", reqPayload)
	}
	der, err := x509.MarshalPKIXPublicKey(&f.rsaKey.PublicKey)
	if err != nil {
		return err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	if err := writeWirePacket(conn, seq+1, append([]byte{authMoreDataMarker}, pemBytes...)); err != nil {
		return err
	}
	seq, encPayload, err := readWirePacket(conn)
	if err != nil {
		return err
	}
	plain, err := rsa.DecryptOAEP(sha1.New(), nil, f.rsaKey, encPayload, nil)
	if err != nil {
		return fmt.Errorf("DecryptOAEP: %w", err)
	}
	for i := range plain {
		plain[i] ^= f.scramble[i%len(f.scramble)]
	}
	if string(plain[:len(plain)-1]) != f.wantPassword || plain[len(plain)-1] != 0 {
		return fmt.Errorf("RSA-recovered password = %q, want %q", plain, f.wantPassword)
	}
	return writeWirePacket(conn, seq+1, buildOKPacket())
}

func TestAuthenticateBackendNativePassword(t *testing.T) {
	gatewayConn, backendConn := net.Pipe()
	defer gatewayConn.Close()
	defer backendConn.Close()

	backend := &fakeMySQLBackend{
		plugin:       pluginMySQLNativePassword,
		scramble:     bytesRange(20, 1),
		wantUser:     "dbc_admin",
		wantPassword: [REDACTED]",
	}
	errCh := make(chan error, 1)
	go func() { errCh <- backend.run(backendConn) }()

	if err := authenticateBackend(gatewayConn, backend.wantUser, backend.wantPassword); err != nil {
		t.Fatalf("authenticateBackend: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("fake backend: %v", err)
	}
}

func TestAuthenticateBackendCachingSHA2FastAuth(t *testing.T) {
	gatewayConn, backendConn := net.Pipe()
	defer gatewayConn.Close()
	defer backendConn.Close()

	backend := &fakeMySQLBackend{
		plugin:       pluginCachingSHA2Password,
		scramble:     bytesRange(20, 7),
		wantUser:     "dbc_readonly",
		wantPassword: [REDACTED]",
	}
	errCh := make(chan error, 1)
	go func() { errCh <- backend.run(backendConn) }()

	if err := authenticateBackend(gatewayConn, backend.wantUser, backend.wantPassword); err != nil {
		t.Fatalf("authenticateBackend: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("fake backend: %v", err)
	}
}

func TestAuthenticateBackendCachingSHA2FullAuthRSA(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	gatewayConn, backendConn := net.Pipe()
	defer gatewayConn.Close()
	defer backendConn.Close()

	backend := &fakeMySQLBackend{
		plugin:       pluginCachingSHA2Password,
		scramble:     bytesRange(20, 3),
		wantUser:     "dbc_admin",
		wantPassword: [REDACTED]",
		fullAuth:     true,
		rsaKey:       key,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- backend.run(backendConn) }()

	if err := authenticateBackend(gatewayConn, backend.wantUser, backend.wantPassword); err != nil {
		t.Fatalf("authenticateBackend: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("fake backend: %v", err)
	}
}

func TestAuthenticateBackendRejectsUnknownPlugin(t *testing.T) {
	gatewayConn, backendConn := net.Pipe()
	defer gatewayConn.Close()
	defer backendConn.Close()

	go func() {
		_ = writeWirePacket(backendConn, 0, buildBackendHandshake(bytesRange(20, 1), "sha256_password", true))
	}()

	if err := authenticateBackend(gatewayConn, "user", "pass"); err == nil {
		t.Fatal("authenticateBackend: expected an error for an unsupported plugin, got none")
	}
}

func TestAuthenticateBackendRejectsBackendErr(t *testing.T) {
	gatewayConn, backendConn := net.Pipe()
	defer gatewayConn.Close()
	defer backendConn.Close()

	go func() {
		errPayload := append([]byte{0xff, 0x10, 0x04, '#'}, append([]byte("28000"), "Access denied"...)...)
		_ = writeWirePacket(backendConn, 0, errPayload)
	}()

	err := authenticateBackend(gatewayConn, "user", "pass")
	if err == nil {
		t.Fatal("authenticateBackend: expected an error when the backend's handshake is itself an ERR, got none")
	}
}

func TestParseAuthSwitchRequestStripsTrailingNUL(t *testing.T) {
	payload := append([]byte{authSwitchRequestMarker}, []byte(pluginCachingSHA2Password)...)
	payload = append(payload, 0)
	scramble := bytesRange(20, 1)
	payload = append(payload, scramble...)
	payload = append(payload, 0) // trailing NUL some servers include

	plugin, gotScramble, err := parseAuthSwitchRequest(payload)
	if err != nil {
		t.Fatalf("parseAuthSwitchRequest: %v", err)
	}
	if plugin != pluginCachingSHA2Password {
		t.Errorf("plugin = %q, want %q", plugin, pluginCachingSHA2Password)
	}
	if !bytes.Equal(gotScramble, scramble) {
		t.Errorf("scramble = %v, want %v", gotScramble, scramble)
	}
}

func TestParseAuthSwitchRequestRejectsMissingNULTerminator(t *testing.T) {
	if _, _, err := parseAuthSwitchRequest([]byte{authSwitchRequestMarker, 'x', 'y', 'z'}); err == nil {
		t.Fatal("expected an error when the plugin name has no NUL terminator")
	}
}

// TestAuthenticateBackendHandlesAuthSwitchRequest reproduces exactly what a
// real MySQL 8 server does when its default_authentication_plugin (declared
// in the initial handshake) doesn't match the account actually being
// authenticated: it sends an AuthSwitchRequest naming the account's real
// plugin and a fresh scramble, and the client must redo the challenge
// response against that instead — confirmed live against a real MySQL 8
// server whose default is mysql_native_password while the target account
// (dbc_admin) used caching_sha2_password.
func TestAuthenticateBackendHandlesAuthSwitchRequest(t *testing.T) {
	gatewayConn, backendConn := net.Pipe()
	defer gatewayConn.Close()
	defer backendConn.Close()

	const user, password = [REDACTED]", "adminpw"
	initialScramble := bytesRange(20, 1)
	switchedScramble := bytesRange(20, 41)

	errCh := make(chan error, 1)
	go func() {
		errCh <- func() error {
			// Initial handshake declares mysql_native_password (the
			// server's configured default) — not what this account
			// actually uses.
			if err := writeWirePacket(backendConn, 0, buildBackendHandshake(initialScramble, pluginMySQLNativePassword, true)); err != nil {
				return err
			}
			seq, payload, err := readWirePacket(backendConn)
			if err != nil {
				return err
			}
			_ = payload // the (native-password-scrambled, and therefore useless) response is ignored, like a real server would.

			switchReq := append([]byte{authSwitchRequestMarker}, []byte(pluginCachingSHA2Password)...)
			switchReq = append(switchReq, 0)
			switchReq = append(switchReq, switchedScramble...)
			if err := writeWirePacket(backendConn, seq+1, switchReq); err != nil {
				return err
			}

			seq, payload, err = readWirePacket(backendConn)
			if err != nil {
				return err
			}
			want := scrambleCachingSHA2(password, switchedScramble)
			if !bytes.Equal(payload, want) {
				return fmt.Errorf("auth switch response = %x, want %x", payload, want)
			}

			if err := writeWirePacket(backendConn, seq+1, []byte{authMoreDataMarker, cachingSha2FastAuthSuccess}); err != nil {
				return err
			}
			return writeWirePacket(backendConn, seq+2, buildOKPacket())
		}()
	}()

	if err := authenticateBackend(gatewayConn, user, password); err != nil {
		t.Fatalf("authenticateBackend: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("fake backend: %v", err)
	}
}

// TestAuthenticateBackendRejectsRepeatedAuthSwitch guards the
// go-sql-driver/mysql-derived safety bound: a backend requesting a second
// plugin switch is treated as a protocol violation, not honored — an
// unbounded switch loop would otherwise be a resource-exhaustion vector for
// a compromised or buggy backend.
func TestAuthenticateBackendRejectsRepeatedAuthSwitch(t *testing.T) {
	gatewayConn, backendConn := net.Pipe()
	defer gatewayConn.Close()
	defer backendConn.Close()

	go func() {
		scramble := bytesRange(20, 1)
		_ = writeWirePacket(backendConn, 0, buildBackendHandshake(scramble, pluginMySQLNativePassword, true))
		if _, _, err := readWirePacket(backendConn); err != nil {
			return
		}
		switchReq := append([]byte{authSwitchRequestMarker}, []byte(pluginCachingSHA2Password)...)
		switchReq = append(switchReq, 0)
		switchReq = append(switchReq, bytesRange(20, 41)...)
		if err := writeWirePacket(backendConn, 1, switchReq); err != nil {
			return
		}
		if _, _, err := readWirePacket(backendConn); err != nil {
			return
		}
		// A second switch request — must be rejected, not honored again.
		secondSwitch := append([]byte{authSwitchRequestMarker}, []byte(pluginMySQLNativePassword)...)
		secondSwitch = append(secondSwitch, 0)
		secondSwitch = append(secondSwitch, bytesRange(20, 81)...)
		_ = writeWirePacket(backendConn, 2, secondSwitch)
	}()

	err := authenticateBackend(gatewayConn, "user", "pass")
	if err == nil {
		t.Fatal("authenticateBackend: expected an error for a second auth switch request, got none")
	}
}

func bytesRange(n, start int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(start + i)
	}
	return b
}
