package proxy

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	_ "example.invalid/go-sql-driver/mysql"

	"example.invalid/sre/dbc/internal/config"
)

// TestPreparedStatementEndToEnd drives a *real* go-sql-driver/mysql client
// (which, unlike the fabricated packets everywhere else in this file, sends
// genuine COM_STMT_PREPARE/EXECUTE unless told not to — the default is
// native prepared statements, interpolateParams=false) through dbc's actual
// proxy pipe against a real MySQL server, and checks both that results come
// back uncorrupted and that the audit log captured the correctly
// reconstructed SQL. Everything else in this package tests the decoder
// against hand-built bytes; this is the one test that proves those bytes
// match what a real client and a real server actually exchange.
//
// Skipped unless DBC_E2E_MYSQL_DSN names a reachable "user:pass@host:port"
// — this needs a real MySQL server, so it's opt-in rather than part of the
// normal `go test ./...` run. See README's development section for how to
// stand one up (e.g. a throwaway `docker run mysql:8.0`).
func TestPreparedStatementEndToEnd(t *testing.T) {
	dsn := os.Getenv("DBC_E2E_MYSQL_DSN")
	if dsn == "" {
		t.Skip("DBC_E2E_MYSQL_DSN not set; skipping (needs a real MySQL server — see test doc)")
	}
	user, pass, addr, ok := parseTestDSN(dsn)
	if !ok {
		t.Fatalf("DBC_E2E_MYSQL_DSN = %q, want \"user:pass@host:port\"", dsn)
	}

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	proxyAddr := ln.Addr().String()

	aud := &queryAuditor{
		logger:  &fakeAuditLogger{},
		account: "user@example.invalid",
		inst:    config.Instance{Host: addr, Project: "e2e", Region: "e2e", Name: "e2e"},
	}
	logger := aud.logger.(*fakeAuditLogger)

	acceptErrCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			acceptErrCh <- err
			return
		}
		acceptErrCh <- pipe(context.Background(), nil, conn, aud.inst, aud, "")
	}()

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/testdb?parseTime=true", user, pass, proxyAddr))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1) // keep everything on one physical connection, one pipe()

	if _, err := db.Exec("DROP TABLE IF EXISTS e2e_widgets"); err != nil {
		t.Fatalf("setup DROP TABLE: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE e2e_widgets (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(100),
		qty INT,
		price DECIMAL(10,2),
		created_at DATETIME,
		note TEXT
	)`); err != nil {
		t.Fatalf("setup CREATE TABLE: %v", err)
	}

	// A real prepared INSERT with a mix of types dbc's decoder supports:
	// string, int, decimal, datetime, and NULL.
	res, err := db.Exec(
		"INSERT INTO e2e_widgets (name, qty, price, created_at, note) VALUES (?, ?, ?, ?, ?)",
		"Widget?'s \\ test", 7, "19.99", time.Date(2024, 3, 15, 13, 5, 9, 0, time.UTC), nil,
	)
	if err != nil {
		t.Fatalf("prepared INSERT: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}

	var name, note sql.NullString
	var qty int
	var price string
	var createdAt time.Time
	err = db.QueryRow("SELECT name, qty, price, created_at, note FROM e2e_widgets WHERE id = ?", id).
		Scan(&name, &qty, &price, &createdAt, &note)
	if err != nil {
		t.Fatalf("prepared SELECT: %v", err)
	}

	// The row must have arrived through the proxy uncorrupted — this is
	// the core "results aren't corrupted by dbc's involvement" assertion.
	if name.String != "Widget?'s \\ test" || qty != 7 || price != "19.99" || note.Valid {
		t.Fatalf("row = (name=%q qty=%d price=%q note.Valid=%v), want (\"Widget?'s \\\\ test\", 7, \"19.99\", false)",
			name.String, qty, price, note.Valid)
	}
	if !createdAt.Equal(time.Date(2024, 3, 15, 13, 5, 9, 0, time.UTC)) {
		t.Fatalf("created_at = %v, want 2024-03-15 13:05:09 UTC", createdAt)
	}

	// Reuse the same prepared statement across two Execs — real-world proof
	// that the new_params_bind_flag=0 (cached type reuse) path, and
	// COM_STMT_CLOSE on Close(), both work against an actual driver and not
	// just the hand-built bytes the rest of this package's tests use.
	stmt, err := db.Prepare("UPDATE e2e_widgets SET qty = ? WHERE id = ?")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := stmt.Exec(10, id); err != nil {
		t.Fatalf("first stmt.Exec: %v", err)
	}
	if _, err := stmt.Exec(20, id); err != nil {
		t.Fatalf("second stmt.Exec: %v", err)
	}
	if err := stmt.Close(); err != nil {
		t.Fatalf("stmt.Close: %v", err)
	}

	var qtyAfter int
	if err := db.QueryRow("SELECT qty FROM e2e_widgets WHERE id = ?", id).Scan(&qtyAfter); err != nil {
		t.Fatalf("verify UPDATE: %v", err)
	}
	if qtyAfter != 20 {
		t.Fatalf("qty after two reused-statement UPDATEs = %d, want 20 (both executes must have applied correctly)", qtyAfter)
	}

	db.Close()
	<-acceptErrCh // pipe() returns nil after both relay directions finish.

	// Now check what actually landed in the audit log for the INSERT.
	var insertQuery string
	var insertSuccess bool
	for _, e := range logger.entries {
		p, ok := e.Payload.(map[string]any)
		if !ok {
			continue
		}
		if q, _ := p["query"].(string); strings.HasPrefix(q, "INSERT INTO e2e_widgets") {
			insertQuery, _ = p["query"].(string)
			insertSuccess, _ = p["success"].(bool)
		}
	}
	if insertQuery == "" {
		t.Fatal("no audit entry found for the INSERT")
	}
	if !insertSuccess {
		t.Error("INSERT audit entry recorded as failed, want success")
	}
	// price is quoted: go-sql-driver/mysql sends a bound Go string on the
	// wire as MYSQL_TYPE_STRING regardless of the destination column's SQL
	// type — the driver picks the wire type from the Go value's type, not
	// the schema, and the "19.99" argument above is a Go string. MySQL
	// itself does the string->DECIMAL cast server-side. Confirmed live
	// against a real driver — the quoted literal here is what genuinely
	// went out on the wire, not a decoding mistake.
	want := `INSERT INTO e2e_widgets (name, qty, price, created_at, note) VALUES ('Widget?\'s \\ test', 7, '19.99', '2024-03-15 13:05:09', NULL)`
	if insertQuery != want {
		t.Errorf("audited INSERT query =\n%q\nwant\n%q", insertQuery, want)
	}
}

// parseTestDSN parses "user:pass@host:port" — deliberately not a general
// MySQL DSN parser, just enough for this test's own env var.
func parseTestDSN(dsn string) (user, pass, addr string, ok bool) {
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return "", "", "", false
	}
	cred, addr := dsn[:at], dsn[at+1:]
	user, pass, ok = strings.Cut(cred, ":")
	if !ok || addr == "" {
		return "", "", "", false
	}
	return user, pass, addr, true
}
