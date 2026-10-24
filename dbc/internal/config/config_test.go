package config

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	input := `# comment
projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - port: 3401
        role: replica
        instance: mysql84-new-replica
  tp-wz3-tpower:
    region: asia-southeast1
    instances:
      - port: 3506
        role: primary
        instance: chatrrom-online
  gff-yes2win:
    region: asia-southeast1
    instances:
      - port: 3508
        role: primary
        instance: chatrrom-online
`
	instances, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(instances) != 3 {
		t.Fatalf("got %d instances, want 3", len(instances))
	}

	// Results are sorted by port regardless of map iteration order.
	if instances[0].Port != 3401 || instances[1].Port != 3506 || instances[2].Port != 3508 {
		t.Fatalf("expected instances sorted by port, got ports %d, %d, %d", instances[0].Port, instances[1].Port, instances[2].Port)
	}

	// Same instance name, different projects: must stay distinguishable by
	// Key(), not collide just because Name matches.
	byTpower, _ := ByPort(instances, 3506)
	byGff, _ := ByPort(instances, 3508)
	if byTpower.Key() == byGff.Key() {
		t.Fatalf("expected distinct keys for same-named instances in different projects, got %q twice", byTpower.Key())
	}

	inst, ok := ByPort(instances, 3401)
	if !ok {
		t.Fatal("expected to find port 3401")
	}
	if got, want := inst.ConnName(), "tp-wz3-rd:asia-east1:mysql84-new-replica"; got != want {
		t.Fatalf("ConnName() = %q, want %q", got, want)
	}
	if inst.Region != "asia-east1" {
		t.Fatalf("expected region inherited from project, got %q", inst.Region)
	}
}

func TestParseDuplicatePort(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - {port: 3401, role: replica, instance: a}
      - {port: 3401, role: replica, instance: b}
`
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("expected error for duplicate port")
	}
}

func TestParseDuplicateKey(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - {port: 3401, role: replica, instance: a}
      - {port: 3402, role: replica, instance: a}
`
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("expected error for duplicate project:instance key")
	}
}

func TestParseBadRole(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - {port: 3401, role: standby, instance: a}
`
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestParseMissingField(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - {port: 3401, role: replica}
`
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("expected error for missing instance field")
	}
}

func TestParseMissingRegion(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    instances:
      - {port: 3401, role: replica, instance: a}
`
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("expected error for missing project region")
	}
}

func TestParseUnknownField(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - {port: 3401, role: replica, instance: a, typo: oops}
`
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestFilterByPort(t *testing.T) {
	instances := []Instance{
		{Port: 3401, Role: "replica"},
		{Port: 3501, Role: "primary"},
	}

	got, err := Filter(instances, "", 3401)
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(got) != 1 || got[0].Port != 3401 {
		t.Fatalf("Filter by port = %+v, want single instance on 3401", got)
	}

	if _, err := Filter(instances, "", 9999); err == nil {
		t.Fatal("expected error for unknown port")
	}
}

func TestFilterByRole(t *testing.T) {
	instances := []Instance{
		{Port: 3401, Role: "replica"},
		{Port: 3402, Role: "replica"},
		{Port: 3501, Role: "primary"},
	}

	got, err := Filter(instances, "primary", 0)
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(got) != 1 || got[0].Port != 3501 {
		t.Fatalf("Filter by role = %+v, want single primary instance", got)
	}
}

func TestParseNoReplicaOnReplicaRejected(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - {port: 3401, role: replica, no_replica: true, instance: a}
`
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("expected error for no_replica on a replica entry")
	}
}

func TestDailySet(t *testing.T) {
	instances := []Instance{
		{Port: 3401, Role: "replica"},
		{Port: 3501, Role: "primary"},                    // has a replica alternative, excluded
		{Port: 3504, Role: "primary", NoReplica: true},   // chat db, no alternative, included
		{Port: 3502, Role: "primary", AlwaysStart: true}, // has a replica, but opted in anyway
	}

	got := DailySet(instances)
	if len(got) != 3 {
		t.Fatalf("DailySet = %+v, want 3 instances (3401, 3504, 3502)", got)
	}
	for _, i := range got {
		if i.Port != 3401 && i.Port != 3504 && i.Port != 3502 {
			t.Fatalf("DailySet included unexpected port %d", i.Port)
		}
	}
}

func TestHasWarnablePrimary(t *testing.T) {
	if HasWarnablePrimary([]Instance{{Role: "primary", NoReplica: true}}) {
		t.Fatal("no_replica primary should not be warnable")
	}
	if !HasWarnablePrimary([]Instance{{Role: "primary"}}) {
		t.Fatal("ordinary primary should be warnable")
	}
	if HasWarnablePrimary([]Instance{{Role: "replica"}}) {
		t.Fatal("replica should not be warnable")
	}
	// AlwaysStart still has a real replica alternative — unlike NoReplica,
	// it should NOT suppress the "please use the replica" warning.
	if !HasWarnablePrimary([]Instance{{Role: "primary", AlwaysStart: true}}) {
		t.Fatal("always_start primary (a real replica alternative exists) should still be warnable")
	}
}

func TestParseBareMetalHost(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - port: 3701
        role: primary
        instance: baremetal-test
        host: "0.0.0.0:3306"
`
	instances, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("got %d instances, want 1", len(instances))
	}

	inst := instances[0]
	if !inst.IsBareMetal() {
		t.Fatal("expected IsBareMetal() to be true when host is set")
	}
	// ConnName is what dbc actually dials and what shows up in the
	// query-audit log's connName field — for bare metal that must be the
	// literal host address, not a synthesized project:region:instance
	// string that doesn't correspond to any real Cloud SQL resource.
	if got, want := inst.ConnName(), "0.0.0.0:3306"; got != want {
		t.Fatalf("ConnName() = %q, want %q", got, want)
	}
}

func TestParseGateway(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - port: 3701
        role: primary
        instance: baremetal-test
        gateway:
          endpoint: "example.invalid:8443"
          server_name: "example.invalid"
          audience: "https://example.invalid"
          target: "baremetal-test"
`
	instances, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	inst := instances[0]
	if !inst.IsBareMetal() || !inst.UsesGateway() {
		t.Fatalf("gateway instance classification = %+v", inst)
	}
	if got, want := inst.ConnName(), "example.invalid:8443/baremetal-test"; got != want {
		t.Fatalf("ConnName() = %q, want %q", got, want)
	}
}

func TestParseRejectsHostAndGatewayTogether(t *testing.T) {
	input := `projects:
  p:
    region: r
    instances:
      - port: 3701
        role: primary
        instance: db
        host: "0.0.0.0:3306"
        gateway:
          endpoint: "example.invalid:8443"
          server_name: "example.invalid"
          audience: "https://example.invalid"
          target: "db"
`
	if _, err := Parse(strings.NewReader(input)); err == nil || !strings.Contains(err.Error(), "不可同時設定") {
		t.Fatalf("Parse() error = %v", err)
	}
}

func TestParseRejectsGatewayIPServerName(t *testing.T) {
	input := `projects:
  p:
    region: r
    instances:
      - port: 3701
        role: primary
        instance: db
        gateway:
          endpoint: "0.0.0.0:8443"
          server_name: "0.0.0.0"
          audience: "dbc-gateway"
          target: "db"
`
	if _, err := Parse(strings.NewReader(input)); err == nil || !strings.Contains(err.Error(), "DNS 名稱") {
		t.Fatalf("Parse() error = %v", err)
	}
}

func TestConnNameCloudSQLIsNotBareMetal(t *testing.T) {
	inst := Instance{Project: "p", Region: "r", Name: "n"}
	if inst.IsBareMetal() {
		t.Fatal("an instance with no Host should not be considered bare metal")
	}
	if got, want := inst.ConnName(), "p:r:n"; got != want {
		t.Fatalf("ConnName() = %q, want %q", got, want)
	}
}

func TestParseDBProdUserCloudSQLSuperuserFlag(t *testing.T) {
	input := `projects:
  tp-wz3-rd:
    region: asia-east1
    instances:
      - port: 3502
        role: primary
        instance: flagged
        db_prod_user_cloudsqlsuperuser: true
      - port: 3503
        role: primary
        instance: unflagged
`
	instances, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	flagged, ok := ByPort(instances, 3502)
	if !ok || !flagged.DBProdUserCloudSQLSuperuser {
		t.Fatalf("expected port 3502 to have DBProdUserCloudSQLSuperuser=true, got %+v", flagged)
	}

	unflagged, ok := ByPort(instances, 3503)
	if !ok || unflagged.DBProdUserCloudSQLSuperuser {
		t.Fatalf("expected port 3503 to default to DBProdUserCloudSQLSuperuser=false, got %+v", unflagged)
	}
}
