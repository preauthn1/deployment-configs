// Query-level audit logging for `dbc start`'s proxy: this sits on top of
// the plain byte-forwarding pipe and, without altering what's forwarded,
// also logs each query to Cloud Logging — INFO for a query that completed
// (whether the SQL itself succeeded or errored doesn't matter here; that's
// a separate concept, see resolvePendingQuery), WARNING for one that a
// dbc-imposed rule rejected.
//
// This exists because Cloud SQL's own DML/DDL/DCL audit rule (see
// internal/admin/audit.go) only ever covers the primary instance — Cloud
// SQL never audits activity on a replica, confirmed against the official
// docs and by testing. Most day-to-day querying goes through a replica
// (`dbc start`'s default), so without this, that traffic would have no
// audit trail at all.
//
// Prepared statements (COM_STMT_PREPARE/COM_STMT_EXECUTE) are allowed
// through and audited with the actual bound parameter values substituted
// into the SQL text — see preparedstmt.go for the wire-format decoding this
// needs and, importantly, the cases dbc rejects rather than guesses at
// (CLIENT_QUERY_ATTRIBUTES on EXECUTE, unsupported parameter types, a
// `?`-count mismatch): those would misalign the whole value block if
// guessed at, producing a confidently wrong audit record — worse than no
// record. This does not affect typing SQL-level PREPARE/EXECUTE at the
// `mysql` CLI prompt — that's ordinary text (COM_QUERY), a different thing
// entirely from the binary wire protocol commands handled here.
package proxy

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"example.invalid/go/logging"

	"example.invalid/sre/dbc/internal/config"
)

const auditLogID = "dbc-query-audit"

const (
	comQuery       byte = 0x03
	comStmtPrepare byte = 0x16
)

// clientQueryAttributes is CLIENT_QUERY_ATTRIBUTES (bit 27 of the 32-bit
// capability_flags negotiated in the client's Handshake Response packet).
// When set, COM_QUERY's payload gains an extra parameter_count/
// parameter_set_count prefix before the query text — confirmed against the
// official MySQL protocol docs after a real client (mysql 8.x) tripped this
// exact case in testing: the captured query text came back prefixed with
// two stray bytes until this was accounted for. COM_STMT_PREPARE is
// unaffected by this capability (also confirmed against the docs) — its
// payload is always just the command byte followed by the query text.
const clientQueryAttributes uint32 = 1 << 27

// errCodeExecuteRejected is the MySQL error code dbc uses when it rejects a
// COM_STMT_EXECUTE it can't safely audit (see preparedstmt.go's package
// doc) — reusing ER_NOT_SUPPORTED_YET rather than inventing a new code.
const errCodeExecuteRejected = 1235

// loggingClients caches one Cloud Logging client per GCP project so
// instances sharing a project (the common case — `dbc start` usually
// serves several per project) share a single client.
var (
	loggingClientsMu sync.Mutex
	loggingClients   = map[string]*logging.Client{}
)

// CheckAuditLogging does a real, synchronous write to project's audit log
// to confirm the caller actually has permission to write it. Serve uses
// this as a startup preflight per project — instances in a project that
// fails this are refused rather than silently degrading to no audit trail
// once queries start flowing.
//
// On success, the created client is cached (keyed by project) for
// newQueryAuditor to reuse — callers must call this before newQueryAuditor
// for the same project.
func CheckAuditLogging(ctx context.Context, project string) error {
	client, err := getOrCreateLoggingClient(ctx, project)
	if err != nil {
		return err
	}
	return client.Logger(auditLogID).LogSync(ctx, logging.Entry{
		Severity: example.invalid,
		Payload:  map[string]any{"event": "dbc start 稽核紀錄權限檢查"},
	})
}

func getOrCreateLoggingClient(ctx context.Context, project string) (*logging.Client, error) {
	loggingClientsMu.Lock()
	defer loggingClientsMu.Unlock()

	if c, ok := loggingClients[project]; ok {
		return c, nil
	}

	client, err := logging.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("建立 Cloud Logging client 失敗（專案 %s）: %w", project, err)
	}
	var notifyOnce sync.Once
	client.OnError = func(err error) {
		notifyOnce.Do(func() {
			log.Printf("⚠️  稽核紀錄寫入異常（專案 %s），本次連線期間可能有部分查詢未記錄到：%v", project, err)
		})
	}
	loggingClients[project] = client
	return client, nil
}

// auditLogger is the sliver of *logging.Logger that queryAuditor needs —
// letting tests supply a fake instead of a real Cloud Logging client.
type auditLogger interface {
	Log(logging.Entry)
}

// queryAuditor logs the queries (and any COM_STMT_EXECUTE dbc had to reject
// as unauditable) seen on one proxied connection. `pending`/`pendingPrepare`
// are only ever touched following the strict one-command-at-a-time request/
// response ordering MySQL's protocol guarantees (confirmed against the
// official docs — there's no command pipelining), so despite being read and
// written from both of pipe()'s two directional goroutines, at most one is
// ever "live" at a time and no synchronization is needed for them.
// `statements` doesn't have that luxury — a prepared statement is
// registered from the *server*-response side (relayServerToClient, once
// COM_STMT_PREPARE_OK reports its statement_id) but read and mutated from
// the *client*-command side (relayClientToServer, on every EXECUTE/
// SEND_LONG_DATA/CLOSE/RESET) — so mu guards it.
type queryAuditor struct {
	logger  auditLogger
	account string
	inst    config.Instance

	pending        string // SQL text of the query awaiting a response; "" if none
	pendingPrepare string // SQL template of a COM_STMT_PREPARE awaiting its response; "" if none

	mu         sync.Mutex
	statements map[uint32]*preparedStatement
}

// newQueryAuditor builds an auditor for one connection to inst, reusing the
// Cloud Logging client already validated by CheckAuditLogging for project —
// it returns an error if that project was never checked (or failed), so a
// caller can never end up silently auditing nothing.
func newQueryAuditor(project, account string, inst config.Instance) (*queryAuditor, error) {
	loggingClientsMu.Lock()
	client, ok := loggingClients[project]
	loggingClientsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("尚未驗證 %s 的稽核紀錄權限", project)
	}
	return &queryAuditor{logger: client.Logger(auditLogID), account: account, inst: inst}, nil
}

func (a *queryAuditor) notePendingQuery(query string) {
	a.pending = query
}

// resolvePendingQuery is called with the outcome of whatever query is
// currently pending (if any — a no-op otherwise, e.g. for packets outside
// any query/response exchange like the initial handshake).
func (a *queryAuditor) resolvePendingQuery(success bool, errMsg string) {
	if a.pending == "" {
		return
	}
	query := a.pending
	a.pending = ""
	a.log(success, query, errMsg)
}

// serverMoreResultsExists is SERVER_MORE_RESULTS_EXISTS (bit 3 of an
// OK_Packet's status_flags): set when this OK is not the final response in
// a CLIENT_MULTI_STATEMENTS batch — another statement's response follows.
const serverMoreResultsExists = 0x0008

// moreResultsExist reports whether an OK_Packet's status flags carry
// SERVER_MORE_RESULTS_EXISTS. payload[0] must already be known to be 0x00.
// dbc negotiates CLIENT_PROTOCOL_41 and CLIENT_TRANSACTIONS but not
// CLIENT_SESSION_TRACK, so the OK_Packet's shape is a fixed
// header/affected_rows/last_insert_id/status_flags/warning_count with no
// trailing session-state section to account for.
func moreResultsExist(payload []byte) bool {
	rest := payload[1:]
	_, n, ok := readLenEncInt(rest) // affected_rows
	if !ok {
		return false
	}
	rest = rest[n:]
	_, n, ok = readLenEncInt(rest) // last_insert_id
	if !ok {
		return false
	}
	rest = rest[n:]
	if len(rest) < 2 {
		return false
	}
	statusFlags := uint16(rest[0]) | uint16(rest[1])<<8
	return statusFlags&serverMoreResultsExists != 0
}

// observeQueryResponsePacket handles one packet of the response to
// whatever query is currently pending. Ordinarily a COM_QUERY carries a
// single statement and the very first response packet fully resolves it —
// but CLIENT_MULTI_STATEMENTS (see mysqlauth.go's package doc for why
// dbc-gateway now negotiates it, and why some GUI tools rely on it to run
// a whole editor's contents at once) lets one COM_QUERY carry several
// semicolon-separated statements, producing one independent outcome per
// statement; only the last one's status flags lack
// SERVER_MORE_RESULTS_EXISTS. dbc logs the whole batch as a single audit
// entry (extractQueryText never splits it either), marked failed if *any*
// statement in the batch errored — not just the first, which would
// silently miss a later failure in the same batch.
//
// A statement whose response is a resultset (starts with a column-count
// packet, neither 0x00 nor 0xFF) is logged as success immediately, without
// waiting to see whether a later statement in the batch fails — the same
// simplification the single-statement path already made before
// multi-statement support existed: dbc only audits whether a query started
// returning data, not the full lifecycle of streaming it home, so
// extending that same tolerance to "the rest of a batch that included a
// resultset" is a deliberate, not accidental, scope decision.
func (a *queryAuditor) observeQueryResponsePacket(payload []byte) {
	if a.pending == "" {
		return
	}
	switch {
	case payload[0] == 0xFF: // ERR — a batch always stops at the first error.
		a.resolvePendingQuery(false, parseErrMessage(payload))
	case payload[0] == 0x00: // OK — may still have more statements to come.
		if moreResultsExist(payload) {
			return
		}
		a.resolvePendingQuery(true, "")
	default: // resultset — see doc comment above.
		a.resolvePendingQuery(true, "")
	}
}

// notePendingPrepare remembers template as awaiting a COM_STMT_PREPARE
// response — see resolvePendingPrepare.
func (a *queryAuditor) notePendingPrepare(template string) {
	a.pendingPrepare = template
}

// resolvePendingPrepare handles the first response packet to a
// COM_STMT_PREPARE this connection sent. ERR means the prepare failed —
// logged as a WARNING, since otherwise a syntax/permission error at PREPARE
// time would never reach the audit trail at all (no EXECUTE ever follows a
// failed PREPARE). Success (COM_STMT_PREPARE_OK) registers the statement so
// a later EXECUTE can find it; PREPARE itself isn't logged as a query — it
// has no data effect, only EXECUTE does.
func (a *queryAuditor) resolvePendingPrepare(payload []byte) {
	if a.pendingPrepare == "" {
		return
	}
	sql := a.pendingPrepare
	a.pendingPrepare = ""

	if len(payload) > 0 && payload[0] == 0xFF {
		a.log(false, sql, "PREPARE 失敗: "+parseErrMessage(payload))
		return
	}
	// COM_STMT_PREPARE_OK: status(1)=0, statement_id(4), num_columns(2),
	// num_params(2), reserved(1), warning_count(2) — 12 bytes minimum.
	if len(payload) < 12 {
		return // malformed OK packet; nothing sensible to register
	}
	stmtID := binary.LittleEndian.Uint32(payload[1:5])
	numParams := int(binary.LittleEndian.Uint16(payload[7:9]))

	a.mu.Lock()
	if a.statements == nil {
		a.statements = map[uint32]*preparedStatement{}
	}
	a.statements[stmtID] = &preparedStatement{sql: sql, paramCount: numParams}
	a.mu.Unlock()
}

// logExecuteRejected records a COM_STMT_EXECUTE dbc rejected locally (never
// forwarded to the server) because it couldn't be safely decoded — see
// preparedstmt.go's package doc for what triggers this.
func (a *queryAuditor) logExecuteRejected(template, reason string) {
	a.log(false, template, "dbc 拒絕這次 EXECUTE（未送出執行）: "+reason)
}

// forgetStatement evicts one statement's tracked state — COM_STMT_CLOSE.
func (a *queryAuditor) forgetStatement(stmtID uint32) {
	a.mu.Lock()
	delete(a.statements, stmtID)
	a.mu.Unlock()
}

// clearLongData drops a statement's accumulated COM_STMT_SEND_LONG_DATA
// tracking — COM_STMT_RESET. Cached parameter types are preserved: the
// MySQL C API docs confirm RESET clears long data and closes any open
// cursor, but doesn't clear parameter bindings.
func (a *queryAuditor) clearLongData(stmtID uint32) {
	a.mu.Lock()
	if stmt, ok := a.statements[stmtID]; ok {
		stmt.longData = nil
	}
	a.mu.Unlock()
}

// forgetAllStatements evicts every tracked statement on this connection —
// COM_RESET_CONNECTION or a COM_CHANGE_USER, both of which release whatever
// prepared statements existed before them server-side. This runs
// optimistically as soon as the client sends the command, without waiting
// for the server's response: if the command then somehow fails, the worst
// case is a subsequent EXECUTE referencing a statement dbc no longer
// tracks, which the existing "unknown statement_id" path already handles
// safely (forward as-is, let the server's own response drive the audit
// entry) — not a correctness risk, just a missed opportunity to fully
// decode that one EXECUTE.
func (a *queryAuditor) forgetAllStatements() {
	a.mu.Lock()
	a.statements = nil
	a.mu.Unlock()
}

// noteLongData records that stmtID's paramID-th parameter received n more
// bytes via COM_STMT_SEND_LONG_DATA. A reference to an untracked statement
// (unknown to dbc, or paramID out of range) is silently ignored — the
// command still gets forwarded either way; there's just nothing useful to
// track for it, and the server will reject it on its own if it's genuinely
// invalid.
func (a *queryAuditor) noteLongData(stmtID uint32, paramID uint16, n int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	stmt, ok := a.statements[stmtID]
	if !ok || int(paramID) >= stmt.paramCount {
		return
	}
	if stmt.longData == nil {
		stmt.longData = map[int]int{}
	}
	stmt.longData[int(paramID)] += n
}

// execStatement looks up stmtID and, if found, decodes payload against it
// under the auditor's mutex (execCommand mutates the statement's cached
// param types/long-data state on success — see its doc). found reports
// whether dbc has any record of stmtID at all; when it's false, the other
// return values are meaningless and the caller falls back to forwarding the
// EXECUTE unexamined (see relayClientToServer). template is the statement's
// SQL template regardless of execOK, so a rejected EXECUTE can still be
// logged against something recognizable rather than an empty string.
func (a *queryAuditor) execStatement(payload []byte, stmtID uint32, queryAttrsNegotiated bool) (sql, template, failReason string, execOK, found bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	stmt, found := a.statements[stmtID]
	if !found {
		return "", "", "", false, false
	}
	template = stmt.sql
	sql, failReason, execOK = execCommand(payload, stmt, queryAttrsNegotiated)
	return sql, template, failReason, execOK, true
}

func (a *queryAuditor) log(success bool, query, errMsg string) {
	if a.logger == nil {
		return
	}
	severity := example.invalid
	if !success {
		severity = logging.Warning
	}
	payload := map[string]any{
		"account":  a.account,
		"connName": a.inst.ConnName(),
		"query":    query,
		"success":  success,
		"message":  summaryMessage(a.account, a.inst.ConnName(), query, success, errMsg),
	}
	if errMsg != "" {
		payload["errMsg"] = errMsg
	}
	a.logger.Log(logging.Entry{Severity: severity, Payload: payload})
}

// summaryMessageQueryLimit bounds only the query portion of summaryMessage's
// output — the account/connName/status prefix is always shown in full, this
// just keeps the query fragment from swallowing the whole one-line summary.
const summaryMessageQueryLimit = 120

// summaryMessage builds a one-line, human-scannable summary for Cloud
// Logging's Logs Explorer (set jsonPayload.message as a custom summary
// field there — see README). The full, untruncated query is still stored
// separately in the query field; this is purely a shorter preview.
func summaryMessage(account, connName, query string, success bool, errMsg string) string {
	status := "OK"
	if !success {
		status = "FAILED"
		if errMsg != "" {
			status = "FAILED: " + errMsg
		}
	}

	oneLine := strings.Join(strings.Fields(query), " ")
	// Truncate by rune, not byte, so a multi-byte character (Chinese text,
	// an emoji in a string literal, etc.) sitting right at the boundary
	// doesn't get split into invalid UTF-8.
	if runes := []rune(oneLine); len(runes) > summaryMessageQueryLimit {
		oneLine = string(runes[:summaryMessageQueryLimit]) + "..."
	}

	return fmt.Sprintf("%s @ %s [%s] %s", account, connName, status, oneLine)
}

// readPacket reads exactly one MySQL protocol packet from r: a 3-byte
// little-endian length, a 1-byte sequence number, then that many payload
// bytes. Returns the raw bytes (header+payload, for verbatim forwarding)
// and the payload alone (for inspection).
//
// A logical message larger than 2^24-1 bytes spans multiple physical
// packets (continuation packets carry length 0xFFFFFF). Each physical
// packet is still read and forwarded correctly regardless, but content
// extraction only ever looks at one packet's payload, so a single SQL
// statement over ~16MB would be logged truncated — not handled; ordinary
// business queries don't get that large.
func readPacket(r io.Reader) (raw, payload []byte, err error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, nil, err
	}
	length := int(header[0]) | int(header[1])<<8 | int(header[2])<<16
	payload = make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, nil, err
		}
	}
	raw = make([]byte, 4+length)
	copy(raw, header[:])
	copy(raw[4:], payload)
	return raw, payload, nil
}

// buildErrPacket constructs a MySQL protocol ERR packet (assumes
// CLIENT_PROTOCOL_41, universal today) with the given response sequence
// number, so a rejected command gets a legible client-side error instead of
// a dropped connection.
func buildErrPacket(seq byte, code uint16, sqlState, message string) []byte {
	payload := make([]byte, 0, 1+2+1+5+len(message))
	payload = append(payload, 0xFF)
	payload = append(payload, byte(code), byte(code>>8))
	payload = append(payload, '#')
	payload = append(payload, sqlState...)
	payload = append(payload, message...)

	packet := make([]byte, 4+len(payload))
	packet[0] = byte(len(payload))
	packet[1] = byte(len(payload) >> 8)
	packet[2] = byte(len(payload) >> 16)
	packet[3] = seq
	copy(packet[4:], payload)
	return packet
}

// parseErrMessage best-effort extracts the human-readable message from an
// ERR packet's payload (skips the leading 0xFF, the 2-byte error code, and —
// if present — the '#' + 5-byte SQLSTATE marker).
func parseErrMessage(payload []byte) string {
	i := 3
	if i < len(payload) && payload[i] == '#' {
		i += 6
	}
	if i > len(payload) {
		return ""
	}
	return string(payload[i:])
}

// readLenEncInt reads a MySQL length-encoded integer from the start of b.
// Returns the decoded value, how many bytes it consumed, and whether b was
// long enough to decode (false means truncated/malformed input).
func readLenEncInt(b []byte) (value uint64, n int, ok bool) {
	if len(b) == 0 {
		return 0, 0, false
	}
	switch {
	case b[0] < 0xfb:
		return uint64(b[0]), 1, true
	case b[0] == 0xfc:
		if len(b) < 3 {
			return 0, 0, false
		}
		return uint64(b[1]) | uint64(b[2])<<8, 3, true
	case b[0] == 0xfd:
		if len(b) < 4 {
			return 0, 0, false
		}
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16, 4, true
	case b[0] == 0xfe:
		if len(b) < 9 {
			return 0, 0, false
		}
		var v uint64
		for i := 0; i < 8; i++ {
			v |= uint64(b[1+i]) << (8 * i)
		}
		return v, 9, true
	default: // 0xfb: the NULL marker — not meaningful for a count field
		return 0, 1, false
	}
}

// handshakeQueryAttributes reports whether a client's Handshake Response
// packet (always the very first packet a client sends, regardless of
// whatever auth exchange follows) negotiated CLIENT_QUERY_ATTRIBUTES —
// capability_flags occupies that payload's first 4 bytes no matter what
// else the packet contains.
func handshakeQueryAttributes(payload []byte) bool {
	if len(payload) < 4 {
		return false
	}
	capabilities := uint32(payload[0]) | uint32(payload[1])<<8 | uint32(payload[2])<<16 | uint32(payload[3])<<24
	return capabilities&clientQueryAttributes != 0
}

// extractQueryText returns the SQL text from a COM_QUERY payload
// (payload[0] == comQuery). When queryAttrs is true, the payload carries an
// extra parameter_count/parameter_set_count prefix before the query string;
// when parameters are actually bound (paramCount > 0), decoding them would
// additionally need each attribute's name (COM_QUERY's query attributes are
// named, unlike COM_STMT_EXECUTE's positional parameters) — deliberately
// out of scope (see preparedstmt.go's package doc for the parts of
// CLIENT_QUERY_ATTRIBUTES dbc does and doesn't support), so that case is
// reported as opaque rather than guessed at.
func extractQueryText(payload []byte, queryAttrs bool) string {
	rest := payload[1:]
	if !queryAttrs {
		return string(rest)
	}

	paramCount, n, ok := readLenEncInt(rest)
	if !ok {
		return string(rest)
	}
	rest = rest[n:]

	_, n, ok = readLenEncInt(rest) // parameter_set_count, always 1 when this far
	if !ok {
		return string(rest)
	}
	rest = rest[n:]

	if paramCount > 0 {
		return "（query attributes 已綁定參數，dbc 未解析出實際查詢文字）"
	}
	return string(rest)
}

// relayClientToServer forwards raw packets from client to server. It
// inspects the first packet (the Handshake Response) to learn whether
// CLIENT_QUERY_ATTRIBUTES was negotiated, tracks prepared-statement
// lifecycle commands (see preparedstmt.go), and remembers COM_QUERY/
// COM_STMT_EXECUTE text so the matching response can be logged by
// relayServerToClient. Returns when client (or the write to server) errors,
// which for a normal disconnect is just io.EOF-flavored.
func relayClientToServer(server, client net.Conn, aud *queryAuditor) error {
	return relayClientToServerFrom(server, client, aud, true, false)
}

// relayClientToServerPreAuthenticated is like relayClientToServer, but for
// a connection where MySQL-level authentication on both sides has already
// completed by some other means before this loop starts (see
// internal/gateway/mysqlauth.go) — there's no handshake packet left to
// inspect, so it skips straight to command-phase parsing with
// queryAttrs fixed false (the gateway's fake client-facing handshake never
// grants CLIENT_QUERY_ATTRIBUTES, so this assumption holds regardless of
// what the client claimed — see that package's fakeCapabilities doc).
func relayClientToServerPreAuthenticated(server, client net.Conn, aud *queryAuditor) error {
	return relayClientToServerFrom(server, client, aud, false, false)
}

func relayClientToServerFrom(server, client net.Conn, aud *queryAuditor, first, queryAttrs bool) error {
	for {
		raw, payload, err := readPacket(client)
		if err != nil {
			return err
		}

		switch {
		case first:
			first = false
			queryAttrs = handshakeQueryAttributes(payload)

		case len(payload) > 0 && payload[0] == comStmtPrepare:
			aud.notePendingPrepare(string(payload[1:]))

		case len(payload) > 0 && payload[0] == comQuery:
			aud.notePendingQuery(extractQueryText(payload, queryAttrs))

		case len(payload) >= 5 && payload[0] == comStmtExecute:
			stmtID := binary.LittleEndian.Uint32(payload[1:5])
			sql, template, failReason, ok, found := aud.execStatement(payload, stmtID, queryAttrs)
			switch {
			case !found:
				// Not a statement dbc ever saw PREPARE for — forward as-is
				// and let the server's own response drive the audit entry
				// (almost certainly an ERR, "Unknown prepared statement
				// handler", but that's still worth a WARNING, not silence).
				aud.notePendingQuery(fmt.Sprintf("(EXECUTE 參照未知的 statement_id=%d)", stmtID))
			case !ok:
				aud.logExecuteRejected(template, failReason)
				errPacket := buildErrPacket(raw[3]+1, errCodeExecuteRejected, "HY000", "dbc proxy 無法安全稽核這次 EXECUTE："+failReason)
				if _, err := client.Write(errPacket); err != nil {
					return err
				}
				continue // never forwarded to the server
			default:
				aud.notePendingQuery(sql)
			}

		case len(payload) >= 7 && payload[0] == comStmtSendLongData:
			stmtID := binary.LittleEndian.Uint32(payload[1:5])
			paramID := binary.LittleEndian.Uint16(payload[5:7])
			aud.noteLongData(stmtID, paramID, len(payload)-7)

		case len(payload) >= 5 && payload[0] == comStmtClose:
			aud.forgetStatement(binary.LittleEndian.Uint32(payload[1:5]))

		case len(payload) >= 5 && payload[0] == comStmtReset:
			aud.clearLongData(binary.LittleEndian.Uint32(payload[1:5]))

		case len(payload) > 0 && (payload[0] == comResetConnection || payload[0] == comChangeUser):
			aud.forgetAllStatements()
		}

		if _, err := server.Write(raw); err != nil {
			return err
		}
	}
}

// relayServerToClient forwards raw packets from server to client. A
// pending COM_STMT_PREPARE response is handled specially (it needs to parse
// the statement_id/param count out of a successful response, not just a
// success/fail bit — see resolvePendingPrepare); otherwise, if a query is
// currently pending, it inspects the first response packet's leading byte —
// 0xFF means an ERR packet (failure), anything else means success (OK, or
// the start of a result set) — logs the outcome, and clears the pending
// state so later packets of the same (possibly multi-packet) response are
// just forwarded without re-triggering.
func relayServerToClient(client, server net.Conn, aud *queryAuditor) error {
	for {
		raw, payload, err := readPacket(server)
		if err != nil {
			return err
		}

		if len(payload) > 0 {
			switch {
			case aud.pendingPrepare != "":
				aud.resolvePendingPrepare(payload)
			default:
				aud.observeQueryResponsePacket(payload)
			}
		}

		if _, err := client.Write(raw); err != nil {
			return err
		}
	}
}
