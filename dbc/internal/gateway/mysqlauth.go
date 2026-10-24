// dbc-gateway terminates MySQL authentication itself rather than relaying
// the client's real handshake through to the backend: the whole point of
// per-role backend credentials (see config.go's TargetRole) is that a
// caller's actual database privilege comes from which Cloud Identity group
// they're in, not from whatever MySQL password they happen to type — so the
// gateway fakes a successful handshake to the client (it doesn't matter
// what credentials the client supplies; identity was already established by
// the ID token) and separately, for real, authenticates to the backend
// using the stored credential for the role IsMember matched.
//
// Only mysql_native_password and caching_sha2_password are supported —
// these are the two plugins MySQL actually assigns accounts by default
// (5.7 and earlier vs. 8.0+), and both algorithms are fully specified
// (confirmed against MySQL's own protocol docs and cross-checked against
// go-sql-driver/mysql's implementation, already a dependency in this
// module). Anything else is rejected outright rather than guessed at,
// matching this project's established fail-closed posture elsewhere
// (preparedstmt.go's package doc has the same reasoning for a different
// protocol corner).
package gateway

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
)

// MySQL capability flags this file needs.
const (
	clientLongPassword    uint32 = 1 << 0
	clientConnectWithDB   uint32 = 1 << 3
	clientProtocol41      uint32 = 1 << 9
	clientTransactions    uint32 = 1 << 13
	clientMultiStatements uint32 = 1 << 16
	clientMultiResults    uint32 = 1 << 17
	clientPluginAuth      uint32 = 1 << 19
	clientSecureConn      uint32 = 1 << 15
	clientQueryAttribute  uint32 = 1 << 27
)

// fakeCapabilities is a var, not a const: byte(fakeCapabilities>>24) needs
// runtime truncation semantics (Go constant-conversion rules require a
// constant to already be representable in the target type, so a byte(...)
// conversion of a >255 constant is a compile error even though the intent
// is exactly the truncation a var gets for free).
//
// CLIENT_MULTI_STATEMENTS is included on both hops (this same var backs both
// the fake client-facing handshake and the real backend handshake — see
// buildFakeHandshake/buildHandshakeResponse below): without it, a client
// that sends more than one semicolon-separated statement per COM_QUERY (some
// GUI tools batch a whole editor's contents this way) gets a plain SQL
// syntax error from the real backend, since MySQL only accepts a
// multi-statement COM_QUERY payload from a connection that negotiated this
// capability. See queryaudit.go's observeQueryResponsePacket for how the
// audit side copes with a batch producing more than one response.
//
// CLIENT_QUERY_ATTRIBUTES is deliberately NOT included: dbc's query-audit/
// prepared-statement decoding assumes it's off (see preparedstmt.go's
// execCommand, which refuses to guess at a bound EXECUTE's parameters on a
// connection that negotiated it). mysql2 (the Node.js driver Beekeeper
// Studio and others are built on, confirmed live 2026-09-09) claims this
// capability in its Handshake Response unconditionally, regardless of
// whether the server actually offers it — but per MySQL's own protocol docs,
// the capability set that actually governs the session is the *intersection*
// of what both sides offered, not just what the client claims; a
// spec-compliant client is expected to only use wire formats the server
// actually agreed to. authenticateClient below tolerates this over-claim
// (used to reject the whole connection for it) rather than granting it, so
// mysql2 falls back to plain COM_QUERY/COM_STMT_EXECUTE framing exactly as
// if it hadn't claimed the capability at all — no audit/prepared-statement
// code needs to change for this to work.
var fakeCapabilities uint32 = clientLongPassword | clientProtocol41 | clientSecureConn |
	clientTransactions | clientMultiStatements | clientMultiResults | clientPluginAuth

// fakeServerVersion is cosmetic only — clients don't behave differently
// based on it here, since dbc-gateway's fake handshake is never actually
// checked against anything.
const fakeServerVersion = "8.0.0-dbc-gateway"

const utf8mb4GeneralCI = 0x2d

const (
	authMoreDataMarker         byte = 0x01
	cachingSha2RequestPubKey   byte = 0x02
	cachingSha2FastAuthSuccess byte = 0x03
	cachingSha2PerformFullAuth byte = 0x04
	authSwitchRequestMarker    byte = 0xfe
)

const (
	pluginMySQLNativePassword = [REDACTED]"
	pluginCachingSHA2Password = [REDACTED]"
)

const comInitDB byte = 0x02

// authenticateClient sends a fabricated MySQL handshake to client and
// accepts whatever credentials come back without checking them — see
// package doc for why. It honors one thing from the client's Handshake
// Response: a database name given via CLIENT_CONNECT_WITH_DB (the mechanism
// GUI tools with a "select database" dropdown — Beekeeper Studio and
// similar — use instead of ever sending an explicit `USE db` over the wire;
// found live when one such tool's schema/table sidebar silently stayed
// empty, since with no default schema `WHERE table_schema = database()`
// always matches nothing — not an error, just a legitimately empty result).
// When present, backend (already authenticated by the time this is called —
// see server.go's handleConn ordering) is told to select that database via
// COM_INIT_DB before client is told the handshake succeeded, mirroring how a
// real MySQL server behaves: an unselectable database fails the whole
// login, not just silently proceeds without one.
//
// A client claiming CLIENT_QUERY_ATTRIBUTES is tolerated rather than
// rejected — see fakeCapabilities' doc for why this is deliberately not the
// same as granting it.
//
// Returns an error only for protocol-level failures (I/O, malformed
// packets, or backend refusing the requested database).
func authenticateClient(client, backend net.Conn) error {
	scramble := make([]byte, 20)
	if _, err := rand.Read(scramble); err != nil {
		return fmt.Errorf("產生 scramble 失敗: %w", err)
	}
	if err := writeWirePacket(client, 0, buildFakeHandshake(scramble)); err != nil {
		return fmt.Errorf("送出 handshake 失敗: %w", err)
	}

	seq, payload, err := readWirePacket(client)
	if err != nil {
		return fmt.Errorf("讀取 client handshake response 失敗（%s）: %w", client.RemoteAddr(), err)
	}
	// Diagnostic context for the error paths below: the client's source
	// address and the raw bytes of its Handshake Response — added after a
	// real incident (2026-09-09) where a non-compliant client's request was
	// misparsed as requesting a database named "mysql_native_password"
	// (turned out to be mysql2's auth_plugin_name field, misread because
	// this client's CLIENT_QUERY_ATTRIBUTES claim was, at the time, treated
	// as an outright rejection before its actual database request was ever
	// reached) and there was no way to tell which client, or reconstruct
	// what it actually sent, from the log alone.
	diag := fmt.Sprintf("remote=%s payload=%x", client.RemoteAddr(), payload)

	if db := clientRequestedDatabase(payload); db != "" {
		errPayload, err := selectDatabase(backend, db)
		if err != nil {
			return fmt.Errorf("為 client 要求的資料庫 %q 執行 COM_INIT_DB 失敗: %w [%s]", db, err, diag)
		}
		if errPayload != nil {
			// Relay the backend's real rejection verbatim, in place of the
			// OK a successful handshake would get — this is client-facing
			// information about a database *this specific client* asked
			// for, not a gateway-internal credential detail, so unlike
			// AuthenticateBackend's failures there's no reason to
			// genericize it.
			if werr := writeWirePacket(client, seq+1, errPayload); werr != nil {
				return werr
			}
			return fmt.Errorf("client 要求的資料庫 %q 選取失敗: %s [%s]", db, parseErrMessage(errPayload), diag)
		}
	}

	if err := writeWirePacket(client, seq+1, buildOKPacket()); err != nil {
		return fmt.Errorf("送出 OK 封包失敗: %w", err)
	}
	return nil
}

// clientRequestedDatabase parses a HandshakeResponse41 payload for the
// optional database name field, present only when CLIENT_CONNECT_WITH_DB
// is set: capability_flags(4), max_packet_size(4), character_set(1),
// filler(23), username(NUL-term), auth_response(1 length byte + data),
// then the NUL-terminated database name. Returns "" if the capability
// wasn't set or the payload is too short/malformed to safely parse —
// never guesses, matching this project's fail-closed posture elsewhere.
func clientRequestedDatabase(payload []byte) string {
	if len(payload) < 4 {
		return ""
	}
	capabilities := uint32(payload[0]) | uint32(payload[1])<<8 | uint32(payload[2])<<16 | uint32(payload[3])<<24
	if capabilities&clientConnectWithDB == 0 {
		return ""
	}
	i := 4 + 4 + 1 + 23 // capability_flags, max_packet_size, character_set, filler
	if i >= len(payload) {
		return ""
	}
	nulIdx := bytes.IndexByte(payload[i:], 0)
	if nulIdx < 0 {
		return ""
	}
	i += nulIdx + 1 // past username
	if i >= len(payload) {
		return ""
	}
	authLen := int(payload[i])
	i += 1 + authLen // past auth_response length byte + data
	if i > len(payload) {
		return ""
	}
	nulIdx = bytes.IndexByte(payload[i:], 0)
	if nulIdx < 0 {
		return string(payload[i:])
	}
	return string(payload[i : i+nulIdx])
}

// selectDatabase sends COM_INIT_DB to backend (already authenticated —
// each new command phase restarts the packet sequence at 0) to switch its
// default schema to db. A non-nil returned payload is the backend's raw
// ERR_Packet (the caller decides what to do with it); a non-nil err is a
// protocol/IO-level failure talking to backend at all.
func selectDatabase(backend net.Conn, db string) ([]byte, error) {
	if err := writeWirePacket(backend, 0, append([]byte{comInitDB}, db...)); err != nil {
		return nil, fmt.Errorf("送出 COM_INIT_DB 失敗: %w", err)
	}
	_, payload, err := readWirePacket(backend)
	if err != nil {
		return nil, fmt.Errorf("讀取 COM_INIT_DB 回應失敗: %w", err)
	}
	if isErrPacket(payload) {
		return payload, nil
	}
	return nil, nil
}

// buildFakeHandshake builds a HandshakeV10 payload advertising
// mysql_native_password (the specific plugin name doesn't matter — the
// client's response is never checked — but it must be a plugin every real
// client library recognizes, so none of them refuse to proceed).
func buildFakeHandshake(scramble []byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte(0x0a)
	buf.WriteString(fakeServerVersion)
	buf.WriteByte(0)
	buf.Write([]byte{0, 0, 0, 0}) // thread id, unused
	buf.Write(scramble[:8])
	buf.WriteByte(0) // filler
	buf.WriteByte(byte(fakeCapabilities))
	buf.WriteByte(byte(fakeCapabilities >> 8))
	buf.WriteByte(utf8mb4GeneralCI)
	buf.Write([]byte{0, 0}) // status_flags
	buf.WriteByte(byte(fakeCapabilities >> 16))
	buf.WriteByte(byte(fakeCapabilities >> 24))
	buf.WriteByte(21) // auth_plugin_data_len: 8-byte part1 + 13-byte part2 (12 + NUL)
	buf.Write(make([]byte, 10))
	buf.Write(scramble[8:20])
	buf.WriteByte(0)
	buf.WriteString(pluginMySQLNativePassword)
	buf.WriteByte(0)
	return buf.Bytes()
}

// authenticateBackend performs a real MySQL handshake with conn (already
// TCP-connected to the backend), authenticating as user/password. Supports
// whichever of mysql_native_password/caching_sha2_password the backend
// advertises; any other plugin is a hard failure — see package doc.
//
// The backend's initial handshake advertises whatever its own
// default_authentication_plugin is configured to — which doesn't
// necessarily match the specific account being authenticated as (the
// server doesn't know which account it'll be until the handshake
// response arrives with a username). When they don't match, the server
// sends back an AuthSwitchRequest telling the client which plugin and
// scramble to actually use instead — confirmed live against a real MySQL 8
// server whose default is mysql_native_password while the target account
// used caching_sha2_password. Only one switch is ever honored (matching
// go-sql-driver/mysql's own behavior) as a bound against a
// misbehaving/malicious backend endlessly requesting further switches.
func authenticateBackend(conn net.Conn, user, password string) error {
	seq, payload, err := readWirePacket(conn)
	if err != nil {
		return fmt.Errorf("讀取後端 handshake 失敗: %w", err)
	}
	if isErrPacket(payload) {
		return fmt.Errorf("後端拒絕連線: %s", parseErrMessage(payload))
	}
	scramble, plugin, err := parseBackendHandshake(payload)
	if err != nil {
		return fmt.Errorf("解析後端 handshake 失敗: %w", err)
	}

	authResponse, err := scrambleFor(plugin, password, scramble)
	if err != nil {
		return err
	}
	response := buildHandshakeResponse(user, authResponse, plugin)
	if err := writeWirePacket(conn, seq+1, response); err != nil {
		return fmt.Errorf("送出後端 handshake response 失敗: %w", err)
	}

	return continueBackendAuth(conn, plugin, password, scramble, true)
}

// continueBackendAuth reads the backend's response to whatever auth data
// was just sent and drives it to completion — see authenticateBackend's doc
// for the AuthSwitchRequest case allowSwitch guards against repeating.
func continueBackendAuth(conn net.Conn, plugin, password string, scramble []byte, allowSwitch bool) error {
	seq, payload, err := readWirePacket(conn)
	if err != nil {
		return fmt.Errorf("讀取後端認證回應失敗: %w", err)
	}
	if isOKPacket(payload) {
		return nil
	}
	if isErrPacket(payload) {
		return fmt.Errorf("後端認證失敗: %s", parseErrMessage(payload))
	}

	if len(payload) > 0 && payload[0] == authSwitchRequestMarker {
		if !allowSwitch {
			return fmt.Errorf("後端要求切換認證外掛超過一次")
		}
		newPlugin, newScramble, err := parseAuthSwitchRequest(payload)
		if err != nil {
			return err
		}
		authResponse, err := scrambleFor(newPlugin, password, newScramble)
		if err != nil {
			return err
		}
		if err := writeWirePacket(conn, seq+1, authResponse); err != nil {
			return fmt.Errorf("送出 auth switch response 失敗: %w", err)
		}
		return continueBackendAuth(conn, newPlugin, password, newScramble, false)
	}

	if plugin != pluginCachingSHA2Password {
		return fmt.Errorf("後端回應了未預期的封包")
	}
	return finishCachingSHA2FullAuth(conn, seq, payload, password, scramble)
}

// scrambleFor computes the challenge-response for plugin, or a clear error
// for anything unsupported — see package doc for why that's fail-closed
// rather than a best-effort guess.
func scrambleFor(plugin, password string, scramble []byte) ([]byte, error) {
	switch plugin {
	case pluginMySQLNativePassword:
		[REDACTED] scrambleNative(password, scramble), nil
	case pluginCachingSHA2Password:
		[REDACTED] scrambleCachingSHA2(password, scramble), nil
	default:
		return nil, fmt.Errorf("後端使用不支援的認證外掛 %q（只支援 %s/%s）", plugin, pluginMySQLNativePassword, pluginCachingSHA2Password)
	}
}

// parseAuthSwitchRequest parses an AuthSwitchRequest payload (leading byte
// authSwitchRequestMarker): a NUL-terminated plugin name followed by the
// new scramble to use, itself sometimes NUL-terminated depending on the
// server — cross-checked against go-sql-driver/mysql's readAuthResult,
// which strips exactly one trailing NUL if present.
func parseAuthSwitchRequest(payload []byte) (plugin string, scramble []byte, err error) {
	rest := payload[1:]
	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return "", nil, fmt.Errorf("auth switch request 缺少 plugin name 結尾")
	}
	plugin = string(rest[:i])
	scramble = rest[i+1:]
	if len(scramble) > 0 && scramble[len(scramble)-1] == 0 {
		scramble = scramble[:len(scramble)-1]
	}
	return plugin, scramble, nil
}

// finishCachingSHA2FullAuth handles the continuation rounds
// caching_sha2_password needs beyond the initial fast-path scramble: either
// an immediate fast-auth success (rare on a first-ever connection, common
// on reconnects once the server has cached the hash) or a full
// authentication round, which — since this connection has no MySQL-level
// TLS of its own (the gateway-to-backend link's confidentiality comes from
// the private/WireGuard network instead, see instances.yaml's comments) —
// must go through RSA: request the backend's public key, obfuscate the
// password against the original scramble, and encrypt it.
func finishCachingSHA2FullAuth(conn net.Conn, seq byte, payload []byte, password string, scramble []byte) error {
	if len(payload) < 2 || payload[0] != authMoreDataMarker {
		return fmt.Errorf("後端 %s 回應格式不符預期", pluginCachingSHA2Password)
	}
	switch payload[1] {
	case cachingSha2FastAuthSuccess:
		_, final, err := readWirePacket(conn)
		if err != nil {
			return fmt.Errorf("讀取後端最終認證結果失敗: %w", err)
		}
		if isOKPacket(final) {
			return nil
		}
		return fmt.Errorf("後端認證失敗: %s", parseErrMessage(final))

	case cachingSha2PerformFullAuth:
		if err := writeWirePacket(conn, seq+1, []byte{cachingSha2RequestPubKey}); err != nil {
			return fmt.Errorf("要求後端公鑰失敗: %w", err)
		}
		seq, keyPayload, err := readWirePacket(conn)
		if err != nil {
			return fmt.Errorf("讀取後端公鑰失敗: %w", err)
		}
		if len(keyPayload) < 2 || keyPayload[0] != authMoreDataMarker {
			return fmt.Errorf("後端公鑰回應格式不符預期")
		}
		pub, err := parseRSAPublicKeyPEM(keyPayload[1:])
		if err != nil {
			return err
		}

		encrypted, err := encryptPasswordForRSA(password, scramble, pub)
		if err != nil {
			return fmt.Errorf("加密密碼失敗: %w", err)
		}
		if err := writeWirePacket(conn, seq+1, encrypted); err != nil {
			return fmt.Errorf("送出加密密碼失敗: %w", err)
		}
		_, final, err := readWirePacket(conn)
		if err != nil {
			return fmt.Errorf("讀取後端最終認證結果失敗: %w", err)
		}
		if isOKPacket(final) {
			return nil
		}
		return fmt.Errorf("後端認證失敗: %s", parseErrMessage(final))

	default:
		return fmt.Errorf("後端 %s 回應了未知狀態 0x%02x", pluginCachingSHA2Password, payload[1])
	}
}

// parseBackendHandshake parses a HandshakeV10 payload, returning the
// 20-byte scramble (auth-plugin-data, part 1 + part 2 concatenated, minus
// part 2's trailing NUL) and the advertised auth plugin name. Field layout
// confirmed against MySQL's protocol docs
// (page_protocol_connection_phase_packets_protocol_handshake_v10.html).
func parseBackendHandshake(payload []byte) (scramble []byte, authPlugin string, err error) {
	if len(payload) < 1 || payload[0] != 0x0a {
		return nil, "", fmt.Errorf("非預期的 handshake 協定版本")
	}
	rest := payload[1:]

	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return nil, "", fmt.Errorf("handshake 缺少 server version 結尾")
	}
	rest = rest[i+1:]

	if len(rest) < 4+8+1+2+1+2+2 {
		return nil, "", fmt.Errorf("handshake 過短")
	}
	rest = rest[4:] // thread_id
	part1 := append([]byte{}, rest[:8]...)
	rest = rest[8:]
	rest = rest[1:] // filler
	capLower := uint32(rest[0]) | uint32(rest[1])<<8
	rest = rest[2:]
	rest = rest[1:] // character_set
	rest = rest[2:] // status_flags
	capHigher := uint32(rest[0]) | uint32(rest[1])<<8
	rest = rest[2:]
	capabilities := capLower | (capHigher << 16)

	if len(rest) < 1+10 {
		return nil, "", fmt.Errorf("handshake 過短（auth-plugin-data-len 之後）")
	}
	authDataLen := int(rest[0])
	rest = rest[1:]
	rest = rest[10:] // reserved

	part2Len := authDataLen - 8
	if part2Len < 13 {
		part2Len = 13
	}
	if len(rest) < part2Len {
		return nil, "", fmt.Errorf("handshake 缺少 auth-plugin-data part 2")
	}
	part2 := rest[:part2Len]
	rest = rest[part2Len:]
	if len(part2) == 0 {
		return nil, "", fmt.Errorf("handshake 的 auth-plugin-data part 2 為空")
	}
	scramble = append(part1, part2[:len(part2)-1]...) // drop part 2's NUL terminator

	if capabilities&clientPluginAuth != 0 {
		if j := bytes.IndexByte(rest, 0); j >= 0 {
			authPlugin = string(rest[:j])
		} else {
			authPlugin = string(rest)
		}
	}
	return scramble, authPlugin, nil
}

// buildHandshakeResponse builds a HandshakeResponse41 payload authenticating
// as username with authResponse (already scrambled for whichever plugin is
// named). No database is selected (CLIENT_CONNECT_WITH_DB unset) — the
// client picks its own schema with a normal `USE`/qualified query once the
// command phase starts, same as it always could.
func buildHandshakeResponse(username string, authResponse []byte, authPluginName string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{byte(fakeCapabilities), byte(fakeCapabilities >> 8), byte(fakeCapabilities >> 16), byte(fakeCapabilities >> 24)})
	buf.Write([]byte{0, 0, 0, 0}) // max_packet_size: 0 (accept server default)
	buf.WriteByte(utf8mb4GeneralCI)
	buf.Write(make([]byte, 23)) // filler
	buf.WriteString(username)
	buf.WriteByte(0)
	// CLIENT_SECURE_CONNECTION (set in fakeCapabilities) without
	// CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA: single length byte + data.
	buf.WriteByte(byte(len(authResponse)))
	buf.Write(authResponse)
	buf.WriteString(authPluginName)
	buf.WriteByte(0)
	return buf.Bytes()
}

// scrambleNative computes mysql_native_password's challenge response:
// SHA1(password) XOR SHA1(scramble + SHA1(SHA1(password))). Cross-checked
// against go-sql-driver/mysql's scramblePassword (already a dependency of
// this module).
func scrambleNative(password string, scramble []byte) []byte {
	if password == "" {
		return nil
	}
	stage1 := sha1.Sum([]byte(password))
	stage2 := sha1.Sum(stage1[:])
	h := sha1.New()
	h.Write(scramble)
	h.Write(stage2[:])
	out := h.Sum(nil)
	for i := range out {
		out[i] ^= stage1[i]
	}
	return out
}

// scrambleCachingSHA2 computes caching_sha2_password's fast-path challenge
// response: SHA256(password) XOR SHA256(SHA256(SHA256(password)) +
// scramble). Cross-checked against go-sql-driver/mysql's
// scrambleSHA256Password.
func scrambleCachingSHA2(password string, scramble []byte) []byte {
	if password == "" {
		return nil
	}
	stage1 := sha256.Sum256([]byte(password))
	stage2 := sha256.Sum256(stage1[:])
	h := sha256.New()
	h.Write(stage2[:])
	h.Write(scramble)
	out := h.Sum(nil)
	for i := range out {
		out[i] ^= stage1[i]
	}
	return out
}

// parseRSAPublicKeyPEM decodes a PEM-encoded RSA public key as returned by
// a backend's caching_sha2_password full-authentication public-key
// exchange.
func parseRSAPublicKeyPEM(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("無法解析後端公鑰 PEM")
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析後端公鑰失敗: %w", err)
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("後端公鑰不是 RSA 公鑰")
	}
	return pub, nil
}

// encryptPasswordForRSA obfuscates password by XOR-ing it (plus a NUL
// terminator) against scramble repeated cyclically, then RSA-OAEP-encrypts
// it with the backend's public key — the exact scheme
// caching_sha2_password's full-authentication path requires when the
// connection isn't already TLS-secured. Cross-checked against
// go-sql-driver/mysql's encryptPassword.
func encryptPasswordForRSA(password string, scramble []byte, pub *rsa.PublicKey) ([]byte, error) {
	plain := make([]byte, len(password)+1)
	copy(plain, password)
	for i := range plain {
		plain[i] ^= scramble[i%len(scramble)]
	}
	return rsa.EncryptOAEP(sha1.New(), rand.Reader, pub, plain, nil)
}
