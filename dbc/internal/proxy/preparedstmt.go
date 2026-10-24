// Prepared-statement support for dbc's query-audit proxy: COM_STMT_PREPARE/
// EXECUTE are allowed through and audited with the actual bound parameter
// values substituted into the SQL text — not just unblocked with a gap.
//
// Wire-format details below were verified against MySQL's official protocol
// docs (example.invalid/doc/dev/mysql-server) and, for the binary temporal
// encoding specifically, against the server's own TimeCodec source.
//
// Anything dbc can't safely decode is rejected as a single EXECUTE (a local
// ERR packet, the command never reaches the server) rather than guessed at:
// an unsupported parameter type, CLIENT_QUERY_ATTRIBUTES negotiated on this
// connection (changes COM_STMT_EXECUTE's layout — a separate, more involved
// format dbc doesn't implement), or a `?`-placeholder/parameter-count
// mismatch. Not knowing an unsupported type's encoded length means every
// parameter after it in the same value block can't be located either, so
// "decode what you can and mark the rest opaque" would silently misalign
// and mislabel every later parameter — a confidently wrong audit record,
// which is worse than the outright rejection this replaces.
package proxy

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	comStmtExecute      byte = 0x17
	comStmtSendLongData byte = 0x18
	comStmtClose        byte = 0x19
	comStmtReset        byte = 0x1a
	comStmtFetch        byte = 0x1c
	comResetConnection  byte = 0x1f
	comChangeUser       byte = 0x11
)

// MySQL binary protocol parameter type IDs, as seen in COM_STMT_EXECUTE's
// type block (page_protocol_com_stmt_execute.html). Only the subset dbc
// knows how to decode losslessly is listed — anything else falls into
// decodeParamValue's default case and is rejected (see package doc).
const (
	mysqlTypeTiny       = 0x01
	mysqlTypeShort      = 0x02
	mysqlTypeLong       = 0x03
	mysqlTypeFloat      = 0x04
	mysqlTypeDouble     = 0x05
	mysqlTypeNull       = 0x06
	mysqlTypeTimestamp  = 0x07
	mysqlTypeLongLong   = 0x08
	mysqlTypeInt24      = 0x09
	mysqlTypeDate       = 0x0a
	mysqlTypeTime       = 0x0b
	mysqlTypeDateTime   = 0x0c
	mysqlTypeYear       = 0x0d
	mysqlTypeVarChar    = 0x0f
	mysqlTypeNewDecimal = 0xf6
	mysqlTypeTinyBlob   = 0xf9
	mysqlTypeMediumBlob = 0xfa
	mysqlTypeLongBlob   = 0xfb
	mysqlTypeBlob       = 0xfc
	mysqlTypeVarString  = 0xfd
	mysqlTypeString     = 0xfe

	// paramUnsignedFlag is the high bit of a COM_STMT_EXECUTE parameter
	// type's second byte.
	paramUnsignedFlag = 0x80
)

// paramType is one bound parameter's declared type, as sent by the client
// in COM_STMT_EXECUTE's type block (or cached from an earlier EXECUTE on
// the same statement — see preparedStatement.paramTypes).
type paramType struct {
	id       byte
	unsigned bool
}

// preparedStatement tracks what dbc has learned about one COM_STMT_PREPARE
// on this connection: the SQL template it was prepared from, how many `?`
// placeholders it has, and (once known) the parameter types to use when
// decoding COM_STMT_EXECUTE's bound values.
type preparedStatement struct {
	sql        string
	paramCount int

	// paramTypes is nil until the client sends type info at least once
	// (new_params_bind_flag=1 on some EXECUTE). A later EXECUTE with the
	// flag unset reuses this — MySQL drivers commonly bind types once and
	// omit them on repeat executes of the same statement.
	paramTypes []paramType

	// longData tracks, by parameter index, how many bytes have been sent
	// via COM_STMT_SEND_LONG_DATA since the last EXECUTE/RESET for that
	// parameter. Per protocol, a long-data-supplied parameter is excluded
	// entirely from EXECUTE's value block (it's not NULL, but its value
	// already lives server-side from the earlier SEND_LONG_DATA calls) —
	// dbc doesn't attempt to reconstruct or log the actual content (usually
	// large BLOBs, and Cloud Logging caps a single entry's size anyway);
	// the reconstructed SQL gets an explicit "not recorded" marker instead
	// so the audit record's incompleteness is visible, not silent.
	longData map[int]int
}

// decodedValue is a parsed COM_STMT_EXECUTE parameter value, already
// rendered as literal SQL text to splice into the reconstructed query (see
// substitutePlaceholders) — a quoted/escaped string, a bare numeric
// literal, or the literal NULL. This is diagnostic text for the audit log,
// not guaranteed to be valid, re-executable SQL (e.g. under
// NO_BACKSLASH_ESCAPES the escaping here wouldn't round-trip).
type decodedValue struct {
	literal string
}

// decodeParamValue reads one bound parameter's value from the front of b
// according to typ, returning its literal SQL text and how many bytes it
// consumed. ok is false for any type dbc doesn't implement — see the
// package doc for why callers must treat that as fatal for the whole
// EXECUTE, not just this one parameter.
func decodeParamValue(b []byte, typ paramType) (v decodedValue, n int, ok bool) {
	switch typ.id {
	case mysqlTypeTiny:
		if len(b) < 1 {
			return decodedValue{}, 0, false
		}
		if typ.unsigned {
			return decodedValue{strconv.FormatUint(uint64(b[0]), 10)}, 1, true
		}
		return decodedValue{strconv.FormatInt(int64(int8(b[0])), 10)}, 1, true

	case mysqlTypeShort, mysqlTypeYear:
		if len(b) < 2 {
			return decodedValue{}, 0, false
		}
		u := binary.LittleEndian.Uint16(b)
		if typ.unsigned {
			return decodedValue{strconv.FormatUint(uint64(u), 10)}, 2, true
		}
		return decodedValue{strconv.FormatInt(int64(int16(u)), 10)}, 2, true

	case mysqlTypeLong, mysqlTypeInt24:
		if len(b) < 4 {
			return decodedValue{}, 0, false
		}
		u := binary.LittleEndian.Uint32(b)
		if typ.unsigned {
			return decodedValue{strconv.FormatUint(uint64(u), 10)}, 4, true
		}
		return decodedValue{strconv.FormatInt(int64(int32(u)), 10)}, 4, true

	case mysqlTypeLongLong:
		if len(b) < 8 {
			return decodedValue{}, 0, false
		}
		u := binary.LittleEndian.Uint64(b)
		if typ.unsigned {
			return decodedValue{strconv.FormatUint(u, 10)}, 8, true
		}
		return decodedValue{strconv.FormatInt(int64(u), 10)}, 8, true

	case mysqlTypeFloat:
		if len(b) < 4 {
			return decodedValue{}, 0, false
		}
		f := math.Float32frombits(binary.LittleEndian.Uint32(b))
		return decodedValue{strconv.FormatFloat(float64(f), 'g', -1, 32)}, 4, true

	case mysqlTypeDouble:
		if len(b) < 8 {
			return decodedValue{}, 0, false
		}
		f := math.Float64frombits(binary.LittleEndian.Uint64(b))
		return decodedValue{strconv.FormatFloat(f, 'g', -1, 64)}, 8, true

	case mysqlTypeNull:
		// A parameter explicitly typed NULL contributes no bytes to the
		// value block, independent of the null_bitmap (which the caller
		// already checks before ever reaching here for the normal case).
		return decodedValue{"NULL"}, 0, true

	case mysqlTypeVarChar, mysqlTypeVarString, mysqlTypeString,
		mysqlTypeTinyBlob, mysqlTypeMediumBlob, mysqlTypeLongBlob, mysqlTypeBlob:
		s, n, ok := readLenEncString(b)
		if !ok {
			return decodedValue{}, 0, false
		}
		return decodedValue{quoteSQLString(s)}, n, true

	case mysqlTypeNewDecimal:
		// DECIMAL is sent as its ASCII text form — a bare numeric literal,
		// not a quoted string.
		s, n, ok := readLenEncString(b)
		if !ok {
			return decodedValue{}, 0, false
		}
		return decodedValue{s}, n, true

	case mysqlTypeDate, mysqlTypeDateTime, mysqlTypeTimestamp:
		return decodeDateTime(b)

	case mysqlTypeTime:
		return decodeTime(b)

	default:
		return decodedValue{}, 0, false
	}
}

// readLenEncString reads a MySQL length-encoded string (a length-encoded
// integer followed by that many raw bytes) from the front of b.
func readLenEncString(b []byte) (s string, n int, ok bool) {
	length, prefixN, ok := readLenEncInt(b)
	if !ok {
		return "", 0, false
	}
	rest := b[prefixN:]
	if length > uint64(len(rest)) {
		return "", 0, false
	}
	return string(rest[:length]), prefixN + int(length), true
}

// quoteSQLString renders s as a single-quoted SQL string literal, escaping
// backslashes and single quotes. This is diagnostic text for the audit log
// (see decodedValue's doc) — good enough to read, not a guarantee of valid
// SQL under every sql_mode.
func quoteSQLString(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + replacer.Replace(s) + "'"
}

// decodeDateTime parses the binary protocol value for DATE/DATETIME/
// TIMESTAMP: a 1-byte length prefix (0, 4, 7, or 11) followed by that many
// bytes of packed fields (page_protocol_binary_resultset.html).
func decodeDateTime(b []byte) (v decodedValue, n int, ok bool) {
	if len(b) < 1 {
		return decodedValue{}, 0, false
	}
	length := int(b[0])
	if length != 0 && length != 4 && length != 7 && length != 11 {
		return decodedValue{}, 0, false
	}
	if len(b) < 1+length {
		return decodedValue{}, 0, false
	}
	if length == 0 {
		return decodedValue{"'0000-00-00 00:00:00'"}, 1, true
	}
	f := b[1 : 1+length]

	year := binary.LittleEndian.Uint16(f[0:2])
	month, day := f[2], f[3]
	var hour, minute, second byte
	var micro uint32
	if length >= 7 {
		hour, minute, second = f[4], f[5], f[6]
	}
	if length == 11 {
		micro = binary.LittleEndian.Uint32(f[7:11])
	}

	s := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", year, month, day, hour, minute, second)
	if micro > 0 {
		s += fmt.Sprintf(".%06d", micro)
	}
	return decodedValue{"'" + s + "'"}, 1 + length, true
}

// decodeTime parses TIME's binary protocol value: a 1-byte length prefix
// (0, 8, or 12), then (if >=8) is_negative/days/hour/minute/second, then
// (if 12) microseconds. TIME can exceed 24 hours and go negative — unlike
// DATE/DATETIME/TIMESTAMP — hence the extra is_negative/days fields;
// confirmed against the server's own TimeCodec encoder source.
func decodeTime(b []byte) (v decodedValue, n int, ok bool) {
	if len(b) < 1 {
		return decodedValue{}, 0, false
	}
	length := int(b[0])
	if length != 0 && length != 8 && length != 12 {
		return decodedValue{}, 0, false
	}
	if len(b) < 1+length {
		return decodedValue{}, 0, false
	}
	if length == 0 {
		return decodedValue{"'00:00:00'"}, 1, true
	}
	f := b[1 : 1+length]

	neg := f[0] != 0
	days := binary.LittleEndian.Uint32(f[1:5])
	hour, minute, second := f[5], f[6], f[7]
	var micro uint32
	if length == 12 {
		micro = binary.LittleEndian.Uint32(f[8:12])
	}

	totalHours := days*24 + uint32(hour)
	sign := ""
	if neg {
		sign = "-"
	}
	s := fmt.Sprintf("%s%02d:%02d:%02d", sign, totalHours, minute, second)
	if micro > 0 {
		s += fmt.Sprintf(".%06d", micro)
	}
	return decodedValue{"'" + s + "'"}, 1 + length, true
}

// substitutePlaceholders splices values into sql's `?` placeholders in
// order, skipping `?` characters that appear inside single/double-quoted
// string literals, backtick-quoted identifiers, or comments (`-- `, `#`,
// `/* ... */`) — a literal `?` inside a string constant must not be
// mistaken for a bind placeholder. Returns ok=false if the number of
// placeholders found doesn't match len(values) exactly: callers must treat
// that as fatal for the whole EXECUTE rather than emit a plausible-looking
// but possibly-misaligned reconstruction (see package doc).
func substitutePlaceholders(sql string, values []decodedValue) (string, bool) {
	runes := []rune(sql)
	var out strings.Builder
	valueIdx := 0
	i := 0
	for i < len(runes) {
		c := runes[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			j := skipQuoted(runes, i, c)
			out.WriteString(string(runes[i:j]))
			i = j
		case c == '-' && i+1 < len(runes) && runes[i+1] == '-' &&
			(i+2 >= len(runes) || isCommentWhitespace(runes[i+2])):
			j := skipLineComment(runes, i)
			out.WriteString(string(runes[i:j]))
			i = j
		case c == '#':
			j := skipLineComment(runes, i)
			out.WriteString(string(runes[i:j]))
			i = j
		case c == '/' && i+1 < len(runes) && runes[i+1] == '*':
			j := skipBlockComment(runes, i)
			out.WriteString(string(runes[i:j]))
			i = j
		case c == '?':
			if valueIdx >= len(values) {
				return "", false
			}
			out.WriteString(values[valueIdx].literal)
			valueIdx++
			i++
		default:
			out.WriteRune(c)
			i++
		}
	}
	if valueIdx != len(values) {
		return "", false
	}
	return out.String(), true
}

// skipQuoted returns the index just past a quoted region starting at
// runes[start] (runes[start] == quote). Handles both escape styles MySQL
// accepts inside string literals — a backslash before any character, and a
// doubled quote character — but not backslash-escaping inside backtick
// identifiers, where backslash isn't special. An unterminated quote
// consumes to the end of the input rather than looping forever.
func skipQuoted(runes []rune, start int, quote rune) int {
	backslashEscapes := quote != '`'
	i := start + 1
	for i < len(runes) {
		switch {
		case backslashEscapes && runes[i] == '\\':
			i += 2
		case runes[i] == quote:
			if i+1 < len(runes) && runes[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		default:
			i++
		}
	}
	return i
}

func isCommentWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// skipLineComment returns the index of the newline (or end of input) that
// ends a `--`/`#` comment starting at runes[start] — the newline itself is
// left for the caller to copy verbatim.
func skipLineComment(runes []rune, start int) int {
	i := start
	for i < len(runes) && runes[i] != '\n' {
		i++
	}
	return i
}

// skipBlockComment returns the index just past the closing `*/` of a
// `/* ... */` comment starting at runes[start]. An unterminated comment
// consumes to the end of the input.
func skipBlockComment(runes []rune, start int) int {
	i := start + 2
	for i+1 < len(runes) {
		if runes[i] == '*' && runes[i+1] == '/' {
			return i + 2
		}
		i++
	}
	return len(runes)
}

// execCommand parses a COM_STMT_EXECUTE payload (payload[0] == comStmtExecute)
// against the statement it refers to, returning the reconstructed
// "effective SQL" to log and forward normally. ok=false means dbc can't
// safely decode this specific EXECUTE — the caller must reject it locally
// (a synthesized ERR, never forwarded) rather than guess; see the package
// doc for why. On success, any newly-bound parameter types are written into
// stmt, and this statement's long-data markers are cleared (the server
// requires long data to be resupplied per execute — see preparedStatement's
// longData field doc); on failure stmt is left untouched, since the server
// never actually executes anything and any long data it's still holding
// remains valid for a later real EXECUTE. Caller holds the auditor's mutex
// for the duration.
func execCommand(payload []byte, stmt *preparedStatement, queryAttrsNegotiated bool) (sql, failReason string, ok bool) {
	if len(payload) < 10 {
		return "", "EXECUTE 封包過短", false
	}
	// payload[1:5] is statement_id, already consumed by the caller to find
	// stmt. payload[5] is flags (cursor type, unused here); payload[6:10]
	// is iteration_count, always 1 per protocol.
	rest := payload[10:]

	if stmt.paramCount == 0 {
		return stmt.sql, "", true
	}

	if queryAttrsNegotiated {
		return "", "此連線談成 CLIENT_QUERY_ATTRIBUTES，dbc 尚未支援解析帶參數的 EXECUTE（不影響一般查詢）", false
	}

	nullBitmapLen := (stmt.paramCount + 7) / 8
	if len(rest) < nullBitmapLen+1 {
		return "", "EXECUTE 封包過短（null bitmap）", false
	}
	nullBitmap := rest[:nullBitmapLen]
	rest = rest[nullBitmapLen:]

	newParamsBindFlag := rest[0]
	rest = rest[1:]

	var types []paramType
	if newParamsBindFlag == 1 {
		if len(rest) < stmt.paramCount*2 {
			return "", "EXECUTE 封包過短（參數型別）", false
		}
		types = make([]paramType, stmt.paramCount)
		for i := 0; i < stmt.paramCount; i++ {
			types[i] = paramType{id: rest[i*2], unsigned: rest[i*2+1]&paramUnsignedFlag != 0}
		}
		rest = rest[stmt.paramCount*2:]
	} else {
		if stmt.paramTypes == nil {
			return "", "client 沒有先綁定過參數型別（new_params_bind_flag=0 但沒有先前的型別快取）", false
		}
		types = stmt.paramTypes
	}

	values := make([]decodedValue, stmt.paramCount)
	for i := 0; i < stmt.paramCount; i++ {
		isNull := nullBitmap[i/8]&(1<<uint(i%8)) != 0
		if isNull {
			values[i] = decodedValue{"NULL"}
			continue
		}
		if n, isLongData := stmt.longData[i]; isLongData {
			values[i] = decodedValue{fmt.Sprintf("<long data，%d bytes，內容未記錄>", n)}
			continue
		}
		v, n, decOK := decodeParamValue(rest, types[i])
		if !decOK {
			return "", fmt.Sprintf("第 %d 個參數型別 0x%02x 目前不支援解碼", i+1, types[i].id), false
		}
		rest = rest[n:]
		values[i] = v
	}

	sql, subOK := substitutePlaceholders(stmt.sql, values)
	if !subOK {
		return "", "SQL 樣板裡的 `?` 佔位符數量跟參數個數對不上", false
	}

	stmt.paramTypes = types
	stmt.longData = nil
	return sql, "", true
}
