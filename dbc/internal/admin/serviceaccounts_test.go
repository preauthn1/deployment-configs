package admin

import (
	"os"
	"testing"
)

func TestParseServiceAccounts(t *testing.T) {
	data := []byte(`
accounts:
  - name: gff_api_frontend
    password: [REDACTED]"
  - name: gff_old_worker
    state: absent
`)
	specs, err := ParseServiceAccounts(data)
	if err != nil {
		t.Fatalf("ParseServiceAccounts: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("len(specs) = %d, want 2", len(specs))
	}
	if specs[0].Name != "gff_api_frontend" || specs[0].Password != "s3cret" || specs[0].Absent {
		t.Errorf("specs[0] = %+v, want {gff_api_frontend s3cret false}", specs[0])
	}
	if specs[1].Name != "gff_old_worker" || !specs[1].Absent {
		t.Errorf("specs[1] = %+v, want Absent=true", specs[1])
	}
}

func TestParseServiceAccountsMissingPassword(t *testing.T) {
	data := []byte(`
accounts:
  - name: gff_no_password
`)
	if _, err := ParseServiceAccounts(data); err == nil {
		t.Fatal("expected error for missing password on a present account, got nil")
	}
}

func TestParseServiceAccountsDuplicateName(t *testing.T) {
	data := []byte(`
accounts:
  - name: gff_dup
    password: "a"
  - name: gff_dup
    password: "b"
`)
	if _, err := ParseServiceAccounts(data); err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

func TestParseServiceAccountsInvalidState(t *testing.T) {
	data := []byte(`
accounts:
  - name: gff_bad_state
    password: "a"
    state: deleted
`)
	if _, err := ParseServiceAccounts(data); err == nil {
		t.Fatal("expected error for invalid state value, got nil")
	}
}

func TestParseServiceAccountsAbsentWithoutPasswordOK(t *testing.T) {
	data := []byte(`
accounts:
  - name: gff_going_away
    state: absent
`)
	specs, err := ParseServiceAccounts(data)
	if err != nil {
		t.Fatalf("ParseServiceAccounts: %v", err)
	}
	if len(specs) != 1 || !specs[0].Absent {
		t.Errorf("specs = %+v, want one Absent entry", specs)
	}
}

// TestExampleFileParses guards against service-accounts.yaml.example (the
// checked-in template — the real file is gitignored, this one isn't)
// drifting out of sync with the schema ParseServiceAccounts actually
// accepts.
func TestExampleFileParses(t *testing.T) {
	data, err := os.ReadFile("../../service-accounts.yaml.example")
	if err != nil {
		t.Fatalf("讀取範本檔失敗: %v", err)
	}
	specs, err := ParseServiceAccounts(data)
	if err != nil {
		t.Fatalf("ParseServiceAccounts(範本檔): %v", err)
	}
	if len(specs) == 0 {
		t.Fatal("範本檔沒有解析出任何帳號")
	}
}

func TestMysqlStringLiteral(t *testing.T) {
	tests := []struct{ in, want string }{
		{`plain`, `plain`},
		{`it's`, `it\'s`},
		{`back\slash`, `back\\slash`},
		{`both\'`, `both\\\'`},
	}
	for _, tt := range tests {
		if got := mysqlStringLiteral(tt.in); got != tt.want {
			t.Errorf("mysqlStringLiteral(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
