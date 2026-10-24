package admin

import "testing"

func TestSelectedDatabases(t *testing.T) {
	grants := []string{
		"GRANT USAGE ON *.* TO `db-prod-user`@`example.invalid`",
		"GRANT SELECT ON `xsgcasino`.* TO `db-prod-user`@`example.invalid`",
		"GRANT SELECT ON `gffpay`.* TO `db-prod-user`@`example.invalid`",
		"GRANT `cloudiamgroup`@`%` TO `db-prod-user`@`example.invalid`",
	}

	got := selectedDatabases(grants)
	if len(got) != 2 || !got["xsgcasino"] || !got["gffpay"] {
		t.Fatalf("selectedDatabases(%v) = %v, want {xsgcasino, gffpay}", grants, got)
	}
}

func TestSelectedDatabasesEmpty(t *testing.T) {
	grants := []string{"GRANT USAGE ON *.* TO `db-prod-user`@`example.invalid`"}
	got := selectedDatabases(grants)
	if len(got) != 0 {
		t.Fatalf("selectedDatabases(%v) = %v, want empty", grants, got)
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name   string
		grants []string
		role   string
		want   bool
	}{
		{
			name: "role present among multiple",
			grants: []string{
				"GRANT USAGE ON *.* TO `db-prod-admin`@`example.invalid`",
				"GRANT `cloudiamgroup`@`%`,`cloudsqlsuperuser`@`%` TO `db-prod-admin`@`example.invalid`",
			},
			role: "cloudsqlsuperuser",
			want: true,
		},
		{
			name: "role absent",
			grants: []string{
				"GRANT USAGE ON *.* TO `db-prod-user`@`example.invalid`",
				"GRANT `cloudiamgroup`@`%` TO `db-prod-user`@`example.invalid`",
			},
			role: "cloudsqlsuperuser",
			want: false,
		},
		{
			name: "privilege grant line must not false-positive",
			grants: []string{
				"GRANT SELECT ON `cloudsqlsuperuser`.* TO `db-prod-user`@`example.invalid`",
			},
			role: "cloudsqlsuperuser",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasRole(tt.grants, tt.role); got != tt.want {
				t.Fatalf("hasRole(%v, %q) = %v, want %v", tt.grants, tt.role, got, tt.want)
			}
		})
	}
}

func TestGroupPrincipal(t *testing.T) {
	user, host := groupPrincipal(GroupUser{Name: "user@example.invalid"})
	if user != "db-prod-user" || host != "example.invalid" {
		t.Fatalf("groupPrincipal() = (%q, %q), want (db-prod-user, example.invalid)", user, host)
	}
}
