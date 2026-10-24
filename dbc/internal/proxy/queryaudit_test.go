package proxy

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"example.invalid/go/logging"

	"example.invalid/sre/dbc/internal/config"
)

func encodePacket(seq byte, payload []byte) []byte {
	n := len(payload)
	packet := make([]byte, 4+n)
	packet[0] = byte(n)
	packet[1] = byte(n >> 8)
	packet[2] = byte(n >> 16)
	packet[3] = seq
	copy(packet[4:], payload)
	return packet
}

func TestReadPacket(t *testing.T) {
	payload := append([]byte{comQuery}, []byte("SELECT 1")...)
	raw := encodePacket(7, payload)

	gotRaw, gotPayload, err := readPacket(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("readPacket: %v", err)
	}
	if !bytes.Equal(gotRaw, raw) {
		t.Errorf("raw = %v, want %v", gotRaw, raw)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("payload = %v, want %v", gotPayload, payload)
	}
}

func TestReadPacketEmptyPayload(t *testing.T) {
	raw := encodePacket(0, nil)
	gotRaw, gotPayload, err := readPacket(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("readPacket: %v", err)
	}
	if len(gotPayload) != 0 {
		t.Errorf("payload = %v, want empty", gotPayload)
	}
	if !bytes.Equal(gotRaw, raw) {
		t.Errorf("raw = %v, want %v", gotRaw, raw)
	}
}

func TestReadPacketShortRead(t *testing.T) {
	// Header claims 10 bytes of payload but only 3 are actually present.
	raw := []byte{10, 0, 0, 0, 'a', 'b', 'c'}
	_, _, err := readPacket(bytes.NewReader(raw))
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("err = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestBuildErrPacketRoundTrip(t *testing.T) {
	packet := buildErrPacket(3, 1235, "HY000", "no thanks")

	length := int(packet[0]) | int(packet[1])<<8 | int(packet[2])<<16
	if length != len(packet)-4 {
		t.Fatalf("length header = %d, want %d", length, len(packet)-4)
	}
	if packet[3] != 3 {
		t.Fatalf("seq = %d, want 3", packet[3])
	}

	payload := packet[4:]
	if payload[0] != 0xFF {
		t.Fatalf("payload[0] = %#x, want 0xFF", payload[0])
	}
	code := int(payload[1]) | int(payload[2])<<8
	if code != 1235 {
		t.Fatalf("code = %d, want 1235", code)
	}
	if payload[3] != '#' {
		t.Fatalf("sqlstate marker = %q, want '#'", payload[3])
	}
	if got := string(payload[4:9]); got != "HY000" {
		t.Fatalf("sqlstate = %q, want HY000", got)
	}
	if got := string(payload[9:]); got != "no thanks" {
		t.Fatalf("message = %q, want %q", got, "no thanks")
	}
}

func TestParseErrMessage(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    string
	}{
		{
			name:    "with sqlstate marker",
			payload: append([]byte{0xFF, 0x10, 0x00, '#'}, append([]byte("HY000"), "boom"...)...),
			want:    "boom",
		},
		{
			name:    "without sqlstate marker",
			payload: append([]byte{0xFF, 0x10, 0x00}, "boom"...),
			want:    "boom",
		},
		{
			name:    "too short",
			payload: []byte{0xFF, 0x10},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseErrMessage(tt.payload); got != tt.want {
				t.Errorf("parseErrMessage(%v) = %q, want %q", tt.payload, got, tt.want)
			}
		})
	}
}

// fakeConn adapts an io.Reader/io.Writer pair to net.Conn for testing the
// relay loops, which only use Read/Write.
type fakeConn struct {
	io.Reader
	io.Writer
}

func (fakeConn) Close() error                       { return nil }
func (fakeConn) LocalAddr() net.Addr                { return nil }
func (fakeConn) RemoteAddr() net.Addr               { return nil }
func (fakeConn) SetDeadline(t time.Time) error      { return nil }
func (fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (fakeConn) SetWriteDeadline(t time.Time) error { return nil }

// fakeAuditLogger records every Entry.Payload passed to Log, so tests can
// assert on what would have reached Cloud Logging without a real client.
type fakeAuditLogger struct {
	entries []logging.Entry
}

func (f *fakeAuditLogger) Log(e logging.Entry) {
	f.entries = append(f.entries, e)
}

// handshakePacket builds a minimal fake Handshake Response packet — only
// the first 4 bytes (capability_flags) matter to the code under test, the
// rest is padding so the packet is a plausible size.
func handshakePacket(capabilities uint32) []byte {
	payload := make([]byte, 32)
	payload[0] = byte(capabilities)
	payload[1] = byte(capabilities >> 8)
	payload[2] = byte(capabilities >> 16)
	payload[3] = byte(capabilities >> 24)
	return encodePacket(1, payload)
}

func TestRelayClientToServerForwardsPreparedStatement(t *testing.T) {
	// PREPARE is allowed through now (not blocked) — see preparedstmt.go.
	// Registering the statement happens on the *response* side
	// (relayServerToClient), so this only checks that the client's PREPARE
	// packet itself reaches the server unmodified and sets pendingPrepare.
	handshake := handshakePacket(0)
	prepare := encodePacket(0, append([]byte{comStmtPrepare}, "SELECT * FROM t WHERE id = ?"...))
	stream := append(append([]byte{}, handshake...), prepare...)
	var serverOut bytes.Buffer

	client := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	server := fakeConn{Reader: bytes.NewReader(nil), Writer: &serverOut}

	aud := &queryAuditor{account: "test", inst: testInstance()}
	err := relayClientToServer(server, client, aud)
	if err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if !bytes.Equal(serverOut.Bytes(), stream) {
		t.Fatalf("server received %v, want %v (prepare must be forwarded, not blocked)", serverOut.Bytes(), stream)
	}
	if aud.pendingPrepare != "SELECT * FROM t WHERE id = ?" {
		t.Fatalf("pendingPrepare = %q, want the prepare template", aud.pendingPrepare)
	}
}

// prepareOKPacket builds a minimal COM_STMT_PREPARE_OK response packet:
// status=0, statement_id, num_columns=0, num_params, reserved=0,
// warning_count=0 — the 12-byte shape resolvePendingPrepare parses.
func prepareOKPacket(seq byte, stmtID uint32, numParams uint16) []byte {
	payload := make([]byte, 12)
	payload[0] = 0x00
	binary.LittleEndian.PutUint32(payload[1:5], stmtID)
	binary.LittleEndian.PutUint16(payload[5:7], 0) // num_columns
	binary.LittleEndian.PutUint16(payload[7:9], numParams)
	return encodePacket(seq, payload)
}

func TestResolvePendingPrepareRegistersStatement(t *testing.T) {
	aud := &queryAuditor{account: "test", inst: testInstance()}
	aud.notePendingPrepare("SELECT * FROM t WHERE id = ?")

	stream := prepareOKPacket(1, 7, 1)
	server := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	client := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
	if err := relayServerToClient(client, server, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}

	if aud.pendingPrepare != "" {
		t.Errorf("pendingPrepare = %q, want cleared", aud.pendingPrepare)
	}
	stmt, ok := aud.statements[7]
	if !ok {
		t.Fatal("expected statement_id=7 to be registered")
	}
	if stmt.sql != "SELECT * FROM t WHERE id = ?" || stmt.paramCount != 1 {
		t.Errorf("registered statement = %+v, want sql/paramCount to match", stmt)
	}
}

func TestResolvePendingPrepareLogsFailureAsWarning(t *testing.T) {
	logger := &fakeAuditLogger{}
	aud := &queryAuditor{logger: logger, account: "test", inst: testInstance()}
	aud.notePendingPrepare("SELECT * FROM nope")

	errPayload := append([]byte{0xFF, 0x2A, 0x04, '#'}, append([]byte("42S02"), "Table 'nope' doesn't exist"...)...)
	stream := encodePacket(1, errPayload)
	server := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	client := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
	if err := relayServerToClient(client, server, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}

	if len(aud.statements) != 0 {
		t.Errorf("statements = %v, want none registered after a failed PREPARE", aud.statements)
	}
	if len(logger.entries) != 1 {
		t.Fatalf("logged %d entries, want 1", len(logger.entries))
	}
	if logger.entries[0].Severity != logging.Warning {
		t.Errorf("severity = %v, want Warning", logger.entries[0].Severity)
	}
}

// TestPrepareThenExecuteFullFlow drives PREPARE and EXECUTE through both
// relay directions in sequence, sharing one queryAuditor — this is the
// closest a unit test gets to the real interleaving pipe() produces without
// standing up actual concurrent goroutines over a live connection (see
// package doc: MySQL's protocol is strictly one command/response at a time,
// so driving this sequentially is faithful to how the real thing behaves).
func TestPrepareThenExecuteFullFlow(t *testing.T) {
	logger := &fakeAuditLogger{}
	aud := &queryAuditor{logger: logger, account: "test", inst: testInstance()}

	// 1. Client sends PREPARE.
	prepareStream := append(handshakePacket(0),
		encodePacket(0, append([]byte{comStmtPrepare}, "SELECT * FROM t WHERE id = ?"...))...)
	if err := relayClientToServer(fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard},
		fakeConn{Reader: bytes.NewReader(prepareStream), Writer: io.Discard}, aud); err != io.EOF {
		t.Fatalf("prepare phase: err = %v, want io.EOF", err)
	}

	// 2. Server responds with PREPARE_OK (statement_id=1, 1 param).
	prepareOK := prepareOKPacket(1, 1, 1)
	if err := relayServerToClient(fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard},
		fakeConn{Reader: bytes.NewReader(prepareOK), Writer: io.Discard}, aud); err != io.EOF {
		t.Fatalf("prepare-response phase: err = %v, want io.EOF", err)
	}

	if _, ok := aud.statements[1]; !ok {
		t.Fatal("expected statement_id=1 registered after PREPARE_OK")
	}

	// 3. Client sends EXECUTE binding id=42. relayClientToServer is called
	// fresh here (a new "connection" in test terms), so it expects its own
	// leading handshake packet just like the real first call would.
	execPayload := execParams(1, []paramType{{id: mysqlTypeLong}}, [][]byte{{42, 0, 0, 0}})
	execStream := append(handshakePacket(0), encodePacket(0, execPayload)...)
	var serverOut bytes.Buffer
	if err := relayClientToServer(fakeConn{Reader: bytes.NewReader(nil), Writer: &serverOut},
		fakeConn{Reader: bytes.NewReader(execStream), Writer: io.Discard}, aud); err != io.EOF {
		t.Fatalf("execute phase: err = %v, want io.EOF", err)
	}

	if !bytes.Equal(serverOut.Bytes(), execStream) {
		t.Fatalf("server received %v, want the EXECUTE forwarded verbatim (plus handshake) %v", serverOut.Bytes(), execStream)
	}
	if aud.pending != "SELECT * FROM t WHERE id = 42" {
		t.Fatalf("pending = %q, want the reconstructed SQL", aud.pending)
	}

	// 4. Server responds OK — this should log the reconstructed SQL as a
	// success.
	okStream := encodePacket(1, []byte{0x00, 0, 0, 0x02, 0, 0, 0})
	if err := relayServerToClient(fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard},
		fakeConn{Reader: bytes.NewReader(okStream), Writer: io.Discard}, aud); err != io.EOF {
		t.Fatalf("execute-response phase: err = %v, want io.EOF", err)
	}

	if len(logger.entries) != 1 {
		t.Fatalf("logged %d entries, want 1", len(logger.entries))
	}
	if logger.entries[0].Severity != example.invalid {
		t.Errorf("severity = %v, want Info", logger.entries[0].Severity)
	}
	payload := logger.entries[0].Payload.(map[string]any)
	if payload["query"] != "SELECT * FROM t WHERE id = 42" {
		t.Errorf("payload[query] = %v, want the reconstructed SQL", payload["query"])
	}
}

func TestExecuteRejectedLocallyNotForwardedToServer(t *testing.T) {
	logger := &fakeAuditLogger{}
	aud := &queryAuditor{logger: logger, account: "test", inst: testInstance()}
	aud.statements = map[uint32]*preparedStatement{
		9: {sql: "SELECT ?", paramCount: 1},
	}

	// An unsupported type (BIT, 0x10) forces the fail-closed path.
	execPayload := execParams(9, []paramType{{id: 0x10}}, [][]byte{{1}})
	stream := append(handshakePacket(0), encodePacket(3, execPayload)...)
	var clientOut, serverOut bytes.Buffer
	client := fakeConn{Reader: bytes.NewReader(stream), Writer: &clientOut}
	server := fakeConn{Reader: bytes.NewReader(nil), Writer: &serverOut}

	err := relayClientToServer(server, client, aud)
	if err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}

	if !bytes.Equal(serverOut.Bytes(), handshakePacket(0)) {
		t.Fatalf("server received %v, want just the handshake (rejected EXECUTE must never be forwarded)", serverOut.Bytes())
	}
	if clientOut.Len() == 0 || clientOut.Bytes()[4] != 0xFF {
		t.Fatalf("client response = %v, want a synthesized ERR packet", clientOut.Bytes())
	}
	if clientOut.Bytes()[3] != 4 {
		t.Fatalf("response seq = %d, want 4 (request seq 3 + 1)", clientOut.Bytes()[3])
	}
	if len(logger.entries) != 1 || logger.entries[0].Severity != logging.Warning {
		t.Fatalf("logged entries = %+v, want exactly one Warning", logger.entries)
	}
}

func TestExecuteUnknownStatementIDForwardsAndLogsMarker(t *testing.T) {
	aud := &queryAuditor{account: "test", inst: testInstance()}
	execPayload := execParams(999, nil, nil)
	stream := append(handshakePacket(0), encodePacket(0, execPayload)...)
	var serverOut bytes.Buffer
	client := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	server := fakeConn{Reader: bytes.NewReader(nil), Writer: &serverOut}

	err := relayClientToServer(server, client, aud)
	if err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if !bytes.Equal(serverOut.Bytes(), stream) {
		t.Fatalf("server received %v, want the EXECUTE forwarded as-is (let the server reject it)", serverOut.Bytes())
	}
	if aud.pending == "" {
		t.Fatal("expected a pending marker so the server's response still gets audited")
	}
}

func TestStmtCloseEvictsStatement(t *testing.T) {
	aud := &queryAuditor{account: "test", inst: testInstance()}
	aud.statements = map[uint32]*preparedStatement{5: {sql: "SELECT 1"}}

	closePayload := []byte{comStmtClose, 5, 0, 0, 0}
	stream := append(handshakePacket(0), encodePacket(0, closePayload)...)
	client := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	server := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
	if err := relayClientToServer(server, client, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if _, ok := aud.statements[5]; ok {
		t.Fatal("expected statement_id=5 to be evicted after COM_STMT_CLOSE")
	}
}

func TestStmtResetClearsLongDataButKeepsTypes(t *testing.T) {
	aud := &queryAuditor{account: "test", inst: testInstance()}
	aud.statements = map[uint32]*preparedStatement{
		5: {sql: "SELECT ?", paramCount: 1, paramTypes: []paramType{{id: mysqlTypeLong}}, longData: map[int]int{0: 10}},
	}

	resetPayload := []byte{comStmtReset, 5, 0, 0, 0}
	stream := append(handshakePacket(0), encodePacket(0, resetPayload)...)
	client := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	server := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
	if err := relayClientToServer(server, client, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}

	stmt := aud.statements[5]
	if stmt.longData != nil {
		t.Errorf("longData = %v, want cleared after COM_STMT_RESET", stmt.longData)
	}
	if stmt.paramTypes == nil {
		t.Error("paramTypes must be preserved across COM_STMT_RESET")
	}
}

func TestResetConnectionClearsAllStatements(t *testing.T) {
	for _, cmd := range []byte{comResetConnection, comChangeUser} {
		aud := &queryAuditor{account: "test", inst: testInstance()}
		aud.statements = map[uint32]*preparedStatement{1: {sql: "SELECT 1"}, 2: {sql: "SELECT 2"}}

		stream := append(handshakePacket(0), encodePacket(0, []byte{cmd})...)
		client := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
		server := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
		if err := relayClientToServer(server, client, aud); err != io.EOF {
			t.Fatalf("cmd %#x: err = %v, want io.EOF", cmd, err)
		}
		if len(aud.statements) != 0 {
			t.Errorf("cmd %#x: statements = %v, want all cleared", cmd, aud.statements)
		}
	}
}

// TestStmtFetchIsForwardedWithoutTouchingAuditState covers a cursor-mode
// EXECUTE: when EXECUTE's flags request a server-side cursor, the server
// doesn't return the result set immediately — the client pages through it
// with one or more COM_STMT_FETCH commands instead. FETCH is fetching more
// of a query that was already logged when EXECUTE ran, not a new query, so
// it must be forwarded completely untouched: no new pending state, no
// statement-map mutation, no re-triggering of any audit log entry.
func TestStmtFetchIsForwardedWithoutTouchingAuditState(t *testing.T) {
	logger := &fakeAuditLogger{}
	aud := &queryAuditor{logger: logger, account: "test", inst: testInstance()}
	aud.statements = map[uint32]*preparedStatement{5: {sql: "SELECT * FROM big_table", paramCount: 0}}

	// COM_STMT_FETCH payload: command(1) + statement_id(4) + num_rows(4).
	fetchPayload := []byte{comStmtFetch, 5, 0, 0, 0, 10, 0, 0, 0}
	stream := append(handshakePacket(0), encodePacket(0, fetchPayload)...)
	var serverOut bytes.Buffer
	client := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	server := fakeConn{Reader: bytes.NewReader(nil), Writer: &serverOut}

	if err := relayClientToServer(server, client, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if !bytes.Equal(serverOut.Bytes(), stream) {
		t.Fatalf("server received %v, want the FETCH forwarded verbatim (plus handshake) %v", serverOut.Bytes(), stream)
	}
	if aud.pending != "" {
		t.Errorf("pending = %q, want untouched (FETCH is not a new query)", aud.pending)
	}
	if _, ok := aud.statements[5]; !ok {
		t.Fatal("expected statement_id=5 to remain registered — FETCH must not evict or alter it")
	}

	// A row-data response to the FETCH must not spuriously resolve any
	// pending query or otherwise touch the audit log.
	rowResponse := encodePacket(1, []byte{0x01, 'x'})
	fetchClient := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
	fetchServer := fakeConn{Reader: bytes.NewReader(rowResponse), Writer: io.Discard}
	if err := relayServerToClient(fetchClient, fetchServer, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if len(logger.entries) != 0 {
		t.Errorf("logged %d entries from a FETCH response, want 0", len(logger.entries))
	}
}

func TestRelayClientToServerForwardsQuery(t *testing.T) {
	query := encodePacket(0, append([]byte{comQuery}, "SELECT 1"...))
	stream := append(handshakePacket(0), query...)
	client := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	var serverOut bytes.Buffer
	server := fakeConn{Reader: bytes.NewReader(nil), Writer: &serverOut}

	aud := &queryAuditor{account: "test", inst: testInstance()}
	err := relayClientToServer(server, client, aud)
	if err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if !bytes.Equal(serverOut.Bytes(), stream) {
		t.Fatalf("server received %v, want %v (both handshake and query forwarded verbatim)", serverOut.Bytes(), stream)
	}
	if aud.pending != "SELECT 1" {
		t.Fatalf("pending = %q, want %q", aud.pending, "SELECT 1")
	}
}

func TestRelayClientToServerQueryAttributesPrefix(t *testing.T) {
	// parameter_count=0, parameter_set_count=1, then the query text —
	// the exact shape a real mysql 8.x client sent in production testing
	// once CLIENT_QUERY_ATTRIBUTES was negotiated.
	payload := append([]byte{comQuery, 0x00, 0x01}, "SELECT 1"...)
	query := encodePacket(0, payload)
	stream := append(handshakePacket(clientQueryAttributes), query...)
	client := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	server := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}

	aud := &queryAuditor{account: "test", inst: testInstance()}
	err := relayClientToServer(server, client, aud)
	if err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if aud.pending != "SELECT 1" {
		t.Fatalf("pending = %q, want %q (query-attributes prefix must be stripped)", aud.pending, "SELECT 1")
	}
}

// TestRelayClientToServerPreAuthenticatedAssumesNoQueryAttrs covers
// dbc-gateway's path: there's no handshake packet in this stream to detect
// CLIENT_QUERY_ATTRIBUTES from (gateway's own fake handshake with the
// client never grants that capability regardless of what the client
// claimed — see internal/gateway/mysqlauth.go's fakeCapabilities doc), so
// this always parses COM_QUERY as the plain (no query-attributes prefix)
// shape.
func TestRelayClientToServerPreAuthenticatedAssumesNoQueryAttrs(t *testing.T) {
	payload := append([]byte{comQuery}, "SELECT 1"...)
	stream := encodePacket(0, payload)
	client := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	server := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}

	aud := &queryAuditor{account: "test", inst: testInstance()}
	err := relayClientToServerPreAuthenticated(server, client, aud)
	if err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if aud.pending != "SELECT 1" {
		t.Fatalf("pending = %q, want %q", aud.pending, "SELECT 1")
	}
}

func TestReadLenEncInt(t *testing.T) {
	tests := []struct {
		name      string
		b         []byte
		wantValue uint64
		wantN     int
		wantOK    bool
	}{
		{"single byte", []byte{0x05, 'x'}, 5, 1, true},
		{"0xfc two-byte", []byte{0xfc, 0x01, 0x02}, 0x0201, 3, true},
		{"truncated 0xfc", []byte{0xfc, 0x01}, 0, 0, false},
		{"empty", nil, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, n, ok := readLenEncInt(tt.b)
			if v != tt.wantValue || n != tt.wantN || ok != tt.wantOK {
				t.Errorf("readLenEncInt(%v) = (%d, %d, %v), want (%d, %d, %v)",
					tt.b, v, n, ok, tt.wantValue, tt.wantN, tt.wantOK)
			}
		})
	}
}

func TestExtractQueryTextWithBoundAttributes(t *testing.T) {
	// parameter_count=1: dbc can't decode the binary-encoded value, so it
	// must report this opaquely rather than guess at the query text.
	payload := append([]byte{comQuery, 0x01, 0x01}, "SELECT ?"...)
	got := extractQueryText(payload, true)
	if got == "SELECT ?" || got == "" {
		t.Fatalf("extractQueryText with bound attributes = %q, want an explicit opaque marker, not a guess", got)
	}
}

// okPacket builds a minimal OK_Packet: header 0x00, affected_rows=0,
// last_insert_id=0, status_flags (SERVER_MORE_RESULTS_EXISTS set iff
// moreResults), warning_count=0.
func okPacket(seq byte, moreResults bool) []byte {
	var statusFlags uint16
	if moreResults {
		statusFlags = serverMoreResultsExists
	}
	payload := []byte{0x00, 0x00, 0x00, byte(statusFlags), byte(statusFlags >> 8), 0x00, 0x00}
	return encodePacket(seq, payload)
}

func errPacket(seq byte, msg string) []byte {
	payload := append([]byte{0xFF, 0x00, 0x00, '#'}, append([]byte("HY000"), msg...)...)
	return encodePacket(seq, payload)
}

func resultsetStartPacket(seq byte) []byte {
	// A column-count packet: single length-encoded int, 1 column.
	return encodePacket(seq, []byte{0x01})
}

func TestObserveQueryResponsePacketSingleStatementResultset(t *testing.T) {
	// Regression: a plain SELECT's response (starts with a column-count
	// packet) still resolves as success on the very first packet, exactly
	// like before multi-statement support existed.
	logger := &fakeAuditLogger{}
	aud := &queryAuditor{logger: logger, account: "test", inst: testInstance()}
	aud.notePendingQuery("SELECT 1")

	stream := resultsetStartPacket(1)
	server := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	client := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
	if err := relayServerToClient(client, server, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if aud.pending != "" {
		t.Errorf("pending = %q, want cleared", aud.pending)
	}
	if len(logger.entries) != 1 {
		t.Fatalf("logged %d entries, want 1", len(logger.entries))
	}
	if logger.entries[0].Severity != example.invalid {
		t.Errorf("severity = %v, want Info (success)", logger.entries[0].Severity)
	}
}

func TestObserveQueryResponsePacketSingleStatementErr(t *testing.T) {
	logger := &fakeAuditLogger{}
	aud := &queryAuditor{logger: logger, account: "test", inst: testInstance()}
	aud.notePendingQuery("SELECT * FROM nope")

	stream := errPacket(1, "Table 'nope' doesn't exist")
	server := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	client := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
	if err := relayServerToClient(client, server, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if len(logger.entries) != 1 || logger.entries[0].Severity != logging.Warning {
		t.Fatalf("logged entries = %+v, want exactly one Warning", logger.entries)
	}
}

func TestObserveQueryResponsePacketMultiStatementAllSucceed(t *testing.T) {
	// "UPDATE a; UPDATE b;" — two OK packets, only the second lacks
	// SERVER_MORE_RESULTS_EXISTS. Must resolve exactly once, as success,
	// only after the second.
	logger := &fakeAuditLogger{}
	aud := &queryAuditor{logger: logger, account: "test", inst: testInstance()}
	aud.notePendingQuery("UPDATE a SET x=1; UPDATE b SET x=2")

	stream := append(okPacket(1, true), okPacket(2, false)...)
	server := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	client := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
	if err := relayServerToClient(client, server, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if aud.pending != "" {
		t.Errorf("pending = %q, want cleared after the final OK", aud.pending)
	}
	if len(logger.entries) != 1 {
		t.Fatalf("logged %d entries, want exactly 1 (the whole batch as one entry)", len(logger.entries))
	}
	if logger.entries[0].Severity != example.invalid {
		t.Errorf("severity = %v, want Info (both statements succeeded)", logger.entries[0].Severity)
	}
}

func TestObserveQueryResponsePacketMultiStatementLaterStatementFails(t *testing.T) {
	// "UPDATE a; UPDATE nope;" — first OK has more-results set, second
	// statement fails outright (a batch stops at its first error). This is
	// the exact gap a naive "resolve on the first packet" implementation
	// would miss: the first statement's success must not mask the second's
	// failure.
	logger := &fakeAuditLogger{}
	aud := &queryAuditor{logger: logger, account: "test", inst: testInstance()}
	aud.notePendingQuery("UPDATE a SET x=1; UPDATE nope SET x=2")

	stream := append(okPacket(1, true), errPacket(2, "Table 'nope' doesn't exist")...)
	server := fakeConn{Reader: bytes.NewReader(stream), Writer: io.Discard}
	client := fakeConn{Reader: bytes.NewReader(nil), Writer: io.Discard}
	if err := relayServerToClient(client, server, aud); err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if len(logger.entries) != 1 {
		t.Fatalf("logged %d entries, want exactly 1", len(logger.entries))
	}
	if logger.entries[0].Severity != logging.Warning {
		t.Errorf("severity = %v, want Warning — the batch's second statement failed", logger.entries[0].Severity)
	}
}

func TestMoreResultsExist(t *testing.T) {
	tests := []struct {
		name string
		ok   []byte
		want bool
	}{
		{"more results set", okPacket(1, true)[4:], true},
		{"final result", okPacket(1, false)[4:], false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := moreResultsExist(tt.ok); got != tt.want {
				t.Errorf("moreResultsExist(%v) = %v, want %v", tt.ok, got, tt.want)
			}
		})
	}
}

func TestSummaryMessageSuccess(t *testing.T) {
	got := summaryMessage("user@example.invalid", "proj:region:inst", "SELECT 1", true, "")
	want := "user@example.invalid @ proj:region:inst [OK] SELECT 1"
	if got != want {
		t.Errorf("summaryMessage = %q, want %q", got, want)
	}
}

func TestSummaryMessageFailureWithErr(t *testing.T) {
	got := summaryMessage("user@example.invalid", "proj:region:inst", "SELECT 1", false, "Table 'x' doesn't exist")
	want := "user@example.invalid @ proj:region:inst [FAILED: Table 'x' doesn't exist] SELECT 1"
	if got != want {
		t.Errorf("summaryMessage = %q, want %q", got, want)
	}
}

func TestSummaryMessageFailureWithoutErr(t *testing.T) {
	got := summaryMessage("user@example.invalid", "proj:region:inst", "SELECT 1", false, "")
	want := "user@example.invalid @ proj:region:inst [FAILED] SELECT 1"
	if got != want {
		t.Errorf("summaryMessage = %q, want %q", got, want)
	}
}

func TestSummaryMessageCollapsesWhitespace(t *testing.T) {
	got := summaryMessage("a", "b", "SELECT\n  1,\n  2\nFROM t", true, "")
	want := "a @ b [OK] SELECT 1, 2 FROM t"
	if got != want {
		t.Errorf("summaryMessage = %q, want %q", got, want)
	}
}

func TestSummaryMessageTruncatesQueryOnly(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := summaryMessage("a", "b", long, true, "")

	prefix := "a @ b [OK] "
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("summaryMessage = %q, want prefix %q intact (only the query is truncated)", got, prefix)
	}
	queryPart := strings.TrimPrefix(got, prefix)
	if !strings.HasSuffix(queryPart, "...") {
		t.Fatalf("truncated query part = %q, want a trailing ...", queryPart)
	}
	if got := len([]rune(strings.TrimSuffix(queryPart, "..."))); got != 120 {
		t.Errorf("truncated query rune length = %d, want 120", got)
	}
}

func TestSummaryMessageTruncatesByRuneNotByte(t *testing.T) {
	// 130 multi-byte characters: a byte-index truncation at 120 would slice
	// through the middle of a rune and produce invalid UTF-8.
	long := strings.Repeat("中", 130)
	got := summaryMessage("a", "b", long, true, "")
	if !utf8.ValidString(got) {
		t.Fatalf("summaryMessage produced invalid UTF-8: %q", got)
	}
}

func testInstance() config.Instance {
	return config.Instance{Port: 3999, Role: "replica", Project: "p", Region: "r", Name: "n"}
}
