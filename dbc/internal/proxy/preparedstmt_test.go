package proxy

import (
	"encoding/binary"
	"testing"
)

func TestDecodeParamValueIntegers(t *testing.T) {
	tests := []struct {
		name string
		typ  paramType
		b    []byte
		want string
		n    int
	}{
		{"tiny signed", paramType{id: mysqlTypeTiny}, []byte{0xFF}, "-1", 1},
		{"tiny unsigned", paramType{id: mysqlTypeTiny, unsigned: true}, []byte{0xFF}, "255", 1},
		{"short signed negative", paramType{id: mysqlTypeShort}, []byte{0xFF, 0xFF}, "-1", 2},
		{"short unsigned", paramType{id: mysqlTypeShort, unsigned: true}, []byte{0xFF, 0xFF}, "65535", 2},
		{"year", paramType{id: mysqlTypeYear}, []byte{0xE6, 0x07}, "2022", 2},
		{"long signed negative", paramType{id: mysqlTypeLong}, []byte{0xFF, 0xFF, 0xFF, 0xFF}, "-1", 4},
		{"int24 positive", paramType{id: mysqlTypeInt24}, []byte{0x2A, 0x00, 0x00, 0x00}, "42", 4},
		{"longlong unsigned max", paramType{id: mysqlTypeLongLong, unsigned: true}, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, "18446744073709551615", 8},
		{"longlong signed negative", paramType{id: mysqlTypeLongLong}, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, "-1", 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, n, ok := decodeParamValue(tt.b, tt.typ)
			if !ok {
				t.Fatalf("decodeParamValue: ok=false, want true")
			}
			if v.literal != tt.want || n != tt.n {
				t.Errorf("decodeParamValue = (%q, %d), want (%q, %d)", v.literal, n, tt.want, tt.n)
			}
		})
	}
}

func TestDecodeParamValueTruncated(t *testing.T) {
	tests := []struct {
		name string
		typ  paramType
		b    []byte
	}{
		{"tiny empty", paramType{id: mysqlTypeTiny}, nil},
		{"short 1 byte", paramType{id: mysqlTypeShort}, []byte{0x01}},
		{"long 3 bytes", paramType{id: mysqlTypeLong}, []byte{0x01, 0x02, 0x03}},
		{"longlong 7 bytes", paramType{id: mysqlTypeLongLong}, make([]byte, 7)},
		{"float 3 bytes", paramType{id: mysqlTypeFloat}, []byte{0x01, 0x02, 0x03}},
		{"double 7 bytes", paramType{id: mysqlTypeDouble}, make([]byte, 7)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, ok := decodeParamValue(tt.b, tt.typ); ok {
				t.Fatalf("decodeParamValue: ok=true on truncated input, want false")
			}
		})
	}
}

func TestDecodeParamValueFloatDouble(t *testing.T) {
	f32 := make([]byte, 4)
	binary.LittleEndian.PutUint32(f32, 0x42280000) // 42.0f
	v, n, ok := decodeParamValue(f32, paramType{id: mysqlTypeFloat})
	if !ok || v.literal != "42" || n != 4 {
		t.Fatalf("float decode = (%q, %d, %v), want (\"42\", 4, true)", v.literal, n, ok)
	}

	f64 := make([]byte, 8)
	binary.LittleEndian.PutUint64(f64, 0x4045000000000000) // 42.0
	v, n, ok = decodeParamValue(f64, paramType{id: mysqlTypeDouble})
	if !ok || v.literal != "42" || n != 8 {
		t.Fatalf("double decode = (%q, %d, %v), want (\"42\", 8, true)", v.literal, n, ok)
	}
}

func TestDecodeParamValueNull(t *testing.T) {
	v, n, ok := decodeParamValue(nil, paramType{id: mysqlTypeNull})
	if !ok || v.literal != "NULL" || n != 0 {
		t.Fatalf("NULL decode = (%q, %d, %v), want (\"NULL\", 0, true)", v.literal, n, ok)
	}
}

func TestDecodeParamValueString(t *testing.T) {
	b := append([]byte{5}, "hello"...)
	b = append(b, "trailing"...)
	v, n, ok := decodeParamValue(b, paramType{id: mysqlTypeVarString})
	if !ok || v.literal != "'hello'" || n != 6 {
		t.Fatalf("string decode = (%q, %d, %v), want (\"'hello'\", 6, true)", v.literal, n, ok)
	}
}

func TestDecodeParamValueStringEscapesQuoteAndBackslash(t *testing.T) {
	raw := `it's a \path`
	b := append([]byte{byte(len(raw))}, raw...)
	v, _, ok := decodeParamValue(b, paramType{id: mysqlTypeString})
	if !ok {
		t.Fatal("decodeParamValue: ok=false")
	}
	want := `'it\'s a \\path'`
	if v.literal != want {
		t.Errorf("escaped literal = %q, want %q", v.literal, want)
	}
}

func TestDecodeParamValueDecimalIsBareLiteral(t *testing.T) {
	raw := "123.45"
	b := append([]byte{byte(len(raw))}, raw...)
	v, _, ok := decodeParamValue(b, paramType{id: mysqlTypeNewDecimal})
	if !ok || v.literal != "123.45" {
		t.Fatalf("decimal decode = (%q, %v), want (\"123.45\", true), unquoted", v.literal, ok)
	}
}

func TestDecodeParamValueUnsupportedType(t *testing.T) {
	if _, _, ok := decodeParamValue([]byte{1, 2, 3}, paramType{id: 0x10 /* BIT, not implemented */}); ok {
		t.Fatal("decodeParamValue: ok=true for an unimplemented type, want false (fail-closed)")
	}
}

func packDateTime(length byte, year uint16, month, day, hour, minute, second byte, micro uint32) []byte {
	b := []byte{length}
	if length == 0 {
		return b
	}
	yb := make([]byte, 2)
	binary.LittleEndian.PutUint16(yb, year)
	b = append(b, yb...)
	b = append(b, month, day)
	if length >= 7 {
		b = append(b, hour, minute, second)
	}
	if length == 11 {
		mb := make([]byte, 4)
		binary.LittleEndian.PutUint32(mb, micro)
		b = append(b, mb...)
	}
	return b
}

func TestDecodeDateTimeVariants(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
		want string
		n    int
	}{
		{"zero length", packDateTime(0, 0, 0, 0, 0, 0, 0, 0), "'0000-00-00 00:00:00'", 1},
		{"date only (length 4)", packDateTime(4, 2024, 3, 15, 0, 0, 0, 0), "'2024-03-15 00:00:00'", 5},
		{"datetime (length 7)", packDateTime(7, 2024, 3, 15, 13, 5, 9, 0), "'2024-03-15 13:05:09'", 8},
		{"with microseconds (length 11)", packDateTime(11, 2024, 3, 15, 13, 5, 9, 123456), "'2024-03-15 13:05:09.123456'", 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, n, ok := decodeDateTime(tt.b)
			if !ok || v.literal != tt.want || n != tt.n {
				t.Errorf("decodeDateTime = (%q, %d, %v), want (%q, %d, true)", v.literal, n, ok, tt.want, tt.n)
			}
		})
	}
}

func TestDecodeDateTimeInvalidLength(t *testing.T) {
	if _, _, ok := decodeDateTime([]byte{5, 1, 2, 3, 4, 5}); ok {
		t.Fatal("decodeDateTime: ok=true for invalid length byte 5, want false")
	}
}

func TestDecodeDateTimeTruncated(t *testing.T) {
	if _, _, ok := decodeDateTime([]byte{7, 1, 2, 3}); ok {
		t.Fatal("decodeDateTime: ok=true when declared length exceeds available bytes, want false")
	}
}

func packTime(length byte, neg bool, days uint32, hour, minute, second byte, micro uint32) []byte {
	b := []byte{length}
	if length == 0 {
		return b
	}
	negB := byte(0)
	if neg {
		negB = 1
	}
	b = append(b, negB)
	db := make([]byte, 4)
	binary.LittleEndian.PutUint32(db, days)
	b = append(b, db...)
	b = append(b, hour, minute, second)
	if length == 12 {
		mb := make([]byte, 4)
		binary.LittleEndian.PutUint32(mb, micro)
		b = append(b, mb...)
	}
	return b
}

func TestDecodeTimeVariants(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
		want string
		n    int
	}{
		{"zero length", packTime(0, false, 0, 0, 0, 0, 0), "'00:00:00'", 1},
		{"positive under 24h (length 8)", packTime(8, false, 0, 13, 5, 9, 0), "'13:05:09'", 9},
		{"negative (length 8)", packTime(8, true, 0, 1, 0, 0, 0), "'-01:00:00'", 9},
		{"exceeds 24h via days (length 8)", packTime(8, false, 2, 3, 0, 0, 0), "'51:00:00'", 9},
		{"with microseconds (length 12)", packTime(12, false, 0, 1, 2, 3, 500000), "'01:02:03.500000'", 13},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, n, ok := decodeTime(tt.b)
			if !ok || v.literal != tt.want || n != tt.n {
				t.Errorf("decodeTime = (%q, %d, %v), want (%q, %d, true)", v.literal, n, ok, tt.want, tt.n)
			}
		})
	}
}

func TestDecodeTimeInvalidLength(t *testing.T) {
	if _, _, ok := decodeTime([]byte{4, 1, 2, 3, 4}); ok {
		t.Fatal("decodeTime: ok=true for invalid length byte 4, want false")
	}
}

func TestSubstitutePlaceholdersBasic(t *testing.T) {
	got, ok := substitutePlaceholders("SELECT * FROM t WHERE id = ? AND name = ?",
		[]decodedValue{{"42"}, {"'bob'"}})
	if !ok {
		t.Fatal("substitutePlaceholders: ok=false")
	}
	want := "SELECT * FROM t WHERE id = 42 AND name = 'bob'"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstitutePlaceholdersSkipsQuestionMarkInsideStringLiteral(t *testing.T) {
	got, ok := substitutePlaceholders(`SELECT ? , 'is this a ?' , "another ?"`, []decodedValue{{"1"}})
	if !ok {
		t.Fatal("substitutePlaceholders: ok=false")
	}
	want := `SELECT 1 , 'is this a ?' , "another ?"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstitutePlaceholdersSkipsQuestionMarkInBacktickIdentifier(t *testing.T) {
	got, ok := substitutePlaceholders("SELECT `weird?column` FROM t WHERE id = ?", []decodedValue{{"1"}})
	if !ok {
		t.Fatal("substitutePlaceholders: ok=false")
	}
	want := "SELECT `weird?column` FROM t WHERE id = 1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstitutePlaceholdersSkipsQuestionMarkInComments(t *testing.T) {
	sql := "SELECT ? -- is this a ?\nFROM t # another ?\nWHERE id = ? /* what about ? here */"
	got, ok := substitutePlaceholders(sql, []decodedValue{{"1"}, {"2"}})
	if !ok {
		t.Fatal("substitutePlaceholders: ok=false")
	}
	want := "SELECT 1 -- is this a ?\nFROM t # another ?\nWHERE id = 2 /* what about ? here */"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstitutePlaceholdersHandlesDoubledQuoteEscape(t *testing.T) {
	got, ok := substitutePlaceholders(`SELECT 'it''s a ? test' , ?`, []decodedValue{{"1"}})
	if !ok {
		t.Fatal("substitutePlaceholders: ok=false")
	}
	want := `SELECT 'it''s a ? test' , 1`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstitutePlaceholdersBackslashNotSpecialInBacktick(t *testing.T) {
	// A literal backslash right before the closing backtick must not
	// consume the backtick too — backslash isn't an escape character
	// inside identifiers.
	got, ok := substitutePlaceholders("SELECT `col\\` FROM t WHERE id = ?", []decodedValue{{"1"}})
	if !ok {
		t.Fatal("substitutePlaceholders: ok=false")
	}
	want := "SELECT `col\\` FROM t WHERE id = 1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstitutePlaceholdersUnterminatedQuoteDoesNotHang(t *testing.T) {
	// No `?` anywhere (it's swallowed into the unterminated string, if
	// there were one after it) — matches the 0 values passed, so this
	// legitimately succeeds. The real assertion is that the call returns
	// at all instead of looping forever on the unterminated quote.
	got, ok := substitutePlaceholders(`SELECT 'unterminated`, nil)
	if !ok || got != `SELECT 'unterminated` {
		t.Fatalf("substitutePlaceholders = (%q, %v), want (%q, true)", got, ok, `SELECT 'unterminated`)
	}
}

func TestSubstitutePlaceholdersUnterminatedBlockCommentDoesNotHang(t *testing.T) {
	if _, ok := substitutePlaceholders(`SELECT 1 /* unterminated`, nil); !ok {
		// No placeholders and none expected — this is actually fine either
		// way; the real assertion is that this returns at all.
		return
	}
}

func TestSubstitutePlaceholdersCountMismatchTooFewValues(t *testing.T) {
	if _, ok := substitutePlaceholders("SELECT ?, ?", []decodedValue{{"1"}}); ok {
		t.Fatal("substitutePlaceholders: ok=true with fewer values than placeholders, want false")
	}
}

func TestSubstitutePlaceholdersCountMismatchTooManyValues(t *testing.T) {
	if _, ok := substitutePlaceholders("SELECT ?", []decodedValue{{"1"}, {"2"}}); ok {
		t.Fatal("substitutePlaceholders: ok=true with more values than placeholders, want false")
	}
}

func TestSubstitutePlaceholdersNoPlaceholders(t *testing.T) {
	got, ok := substitutePlaceholders("SELECT NOW()", nil)
	if !ok || got != "SELECT NOW()" {
		t.Fatalf("got (%q, %v), want (\"SELECT NOW()\", true)", got, ok)
	}
}

// execParams builds a COM_STMT_EXECUTE payload for a statement with the
// given (already-typed) parameters, all newly bound (new_params_bind_flag=1),
// none NULL. values must already be in wire-encoded binary form (the raw
// bytes decodeParamValue would consume), one entry per parameter in order.
func execParams(stmtID uint32, types []paramType, values [][]byte) []byte {
	paramCount := len(types)
	payload := []byte{comStmtExecute}
	idb := make([]byte, 4)
	binary.LittleEndian.PutUint32(idb, stmtID)
	payload = append(payload, idb...)
	payload = append(payload, 0)          // flags
	payload = append(payload, 1, 0, 0, 0) // iteration_count = 1

	nullBitmap := make([]byte, (paramCount+7)/8)
	payload = append(payload, nullBitmap...)
	payload = append(payload, 1) // new_params_bind_flag
	for _, ty := range types {
		flag := byte(0)
		if ty.unsigned {
			flag = paramUnsignedFlag
		}
		payload = append(payload, ty.id, flag)
	}
	for _, v := range values {
		payload = append(payload, v...)
	}
	return payload
}

func TestExecCommandBasicIntegerParam(t *testing.T) {
	stmt := &preparedStatement{sql: "SELECT * FROM t WHERE id = ?", paramCount: 1}
	payload := execParams(7, []paramType{{id: mysqlTypeLong}}, [][]byte{{42, 0, 0, 0}})

	sql, failReason, ok := execCommand(payload, stmt, false)
	if !ok {
		t.Fatalf("execCommand failed: %s", failReason)
	}
	if want := "SELECT * FROM t WHERE id = 42"; sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if stmt.paramTypes == nil {
		t.Error("expected paramTypes to be cached after a successful EXECUTE")
	}
}

func TestExecCommandNoParams(t *testing.T) {
	stmt := &preparedStatement{sql: "SELECT NOW()", paramCount: 0}
	payload := []byte{comStmtExecute, 1, 0, 0, 0, 0, 1, 0, 0, 0}
	sql, failReason, ok := execCommand(payload, stmt, false)
	if !ok {
		t.Fatalf("execCommand failed: %s", failReason)
	}
	if sql != "SELECT NOW()" {
		t.Errorf("sql = %q, want %q", sql, "SELECT NOW()")
	}
}

func TestExecCommandNullParam(t *testing.T) {
	stmt := &preparedStatement{sql: "SELECT ?", paramCount: 1}
	payload := execParams(1, []paramType{{id: mysqlTypeLong}}, [][]byte{{}})
	// Set the null bitmap bit for parameter 0 — payload layout: [0]=cmd,
	// [1:5]=stmt_id, [5]=flags, [6:10]=iteration_count, [10]=null_bitmap.
	payload[10] = 1

	sql, failReason, ok := execCommand(payload, stmt, false)
	if !ok {
		t.Fatalf("execCommand failed: %s", failReason)
	}
	if sql != "SELECT NULL" {
		t.Errorf("sql = %q, want %q", sql, "SELECT NULL")
	}
}

func TestExecCommandReusesCachedTypesWhenNotRebound(t *testing.T) {
	stmt := &preparedStatement{sql: "SELECT ?", paramCount: 1, paramTypes: []paramType{{id: mysqlTypeLong}}}
	// Hand-build an EXECUTE with new_params_bind_flag=0 and a raw value,
	// no type block.
	payload := []byte{comStmtExecute, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 99, 0, 0, 0}
	// [10]=null_bitmap(1 byte, all clear), [11]=new_params_bind_flag=0,
	// [12:16]=the LONG value 99.

	sql, failReason, ok := execCommand(payload, stmt, false)
	if !ok {
		t.Fatalf("execCommand failed: %s", failReason)
	}
	if sql != "SELECT 99" {
		t.Errorf("sql = %q, want %q", sql, "SELECT 99")
	}
}

func TestExecCommandFailsWhenNoCachedTypesAndNotRebound(t *testing.T) {
	stmt := &preparedStatement{sql: "SELECT ?", paramCount: 1}
	payload := []byte{comStmtExecute, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0}
	_, _, ok := execCommand(payload, stmt, false)
	if ok {
		t.Fatal("execCommand: ok=true with no cached types and new_params_bind_flag=0, want fail-closed")
	}
}

func TestExecCommandFailsOnQueryAttributes(t *testing.T) {
	stmt := &preparedStatement{sql: "SELECT ?", paramCount: 1}
	payload := execParams(1, []paramType{{id: mysqlTypeLong}}, [][]byte{{1, 0, 0, 0}})
	_, failReason, ok := execCommand(payload, stmt, true)
	if ok {
		t.Fatal("execCommand: ok=true with CLIENT_QUERY_ATTRIBUTES negotiated, want fail-closed")
	}
	if failReason == "" {
		t.Error("expected a non-empty fail reason")
	}
}

func TestExecCommandFailsOnUnsupportedType(t *testing.T) {
	stmt := &preparedStatement{sql: "SELECT ?", paramCount: 1}
	payload := execParams(1, []paramType{{id: 0x10 /* BIT */}}, [][]byte{{1}})
	_, _, ok := execCommand(payload, stmt, false)
	if ok {
		t.Fatal("execCommand: ok=true for an unsupported parameter type, want fail-closed")
	}
	// stmt must be left untouched on failure.
	if stmt.paramTypes != nil {
		t.Error("stmt.paramTypes must not be mutated on a failed EXECUTE")
	}
}

func TestExecCommandLongDataParamSkipsValueBlockAndAligns(t *testing.T) {
	// Three params: normal, long-data, normal. The value block must only
	// contain values for the two normal ones — getting the skip wrong here
	// would misdecode the third parameter, which is exactly the alignment
	// bug this test guards against.
	stmt := &preparedStatement{
		sql:        "SELECT ?, ?, ?",
		paramCount: 3,
		longData:   map[int]int{1: 12},
	}
	types := []paramType{{id: mysqlTypeLong}, {id: mysqlTypeLong}, {id: mysqlTypeLong}}
	payload := execParams(1, types, [][]byte{{10, 0, 0, 0}, {99, 0, 0, 0}})
	// second entry above is really meant for param index 2 (the long-data
	// param at index 1 contributes no bytes at all), so the value block is
	// [param0's 4 bytes][param2's 4 bytes].

	sql, failReason, ok := execCommand(payload, stmt, false)
	if !ok {
		t.Fatalf("execCommand failed: %s", failReason)
	}
	want := "SELECT 10, <long data，12 bytes，內容未記錄>, 99"
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if stmt.longData != nil {
		t.Error("expected longData to be cleared after a successful EXECUTE")
	}
}

func TestExecCommandPreservesLongDataOnFailure(t *testing.T) {
	stmt := &preparedStatement{
		sql:        "SELECT ?, ?",
		paramCount: 2,
		longData:   map[int]int{0: 5},
	}
	// Second param uses an unsupported type, forcing fail-closed.
	types := []paramType{{id: mysqlTypeLong}, {id: 0x10}}
	payload := execParams(1, types, [][]byte{{1}})

	_, _, ok := execCommand(payload, stmt, false)
	if ok {
		t.Fatal("expected fail-closed on unsupported type")
	}
	if _, still := stmt.longData[0]; !still {
		t.Error("longData must be preserved when the EXECUTE is rejected locally (server never saw it)")
	}
}
