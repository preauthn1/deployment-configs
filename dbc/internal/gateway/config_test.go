package gateway

import (
	"strings"
	"testing"
	"time"
)

const validConfig = `
audience: dbc-gateway
tls:
  certificateFile: /tls/tls.crt
  privateKeyFile: /tls/tls.key
targets:
  baremetal-vplus33:
    address: 0.0.0.0:3306
    auditProject: tp-wz3-rd
    roles:
      - groups:
          - user@example.invalid
        mysqlUser: dbc_admin
        mysqlPassword: [REDACTED]
      - groups:
          - user@example.invalid
        mysqlUser: dbc_readonly
        mysqlPassword: [REDACTED]
`

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := ParseConfig(strings.NewReader(validConfig))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if cfg.Listen != ":8443" || cfg.HealthListen != ":8080" {
		t.Fatalf("listeners = (%q, %q), want (:8443, :8080)", cfg.Listen, cfg.HealthListen)
	}
	if cfg.DrainTimeout != 70*time.Minute {
		t.Fatalf("DrainTimeout = %v, want 70m", cfg.DrainTimeout)
	}
}

func TestParseConfigDurations(t *testing.T) {
	raw := validConfig + "authTimeout: 3s\ndialTimeout: 4s\ndrainTimeout: 45m\n"
	cfg, err := ParseConfig(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if cfg.AuthTimeout != 3*time.Second || cfg.DialTimeout != 4*time.Second || cfg.DrainTimeout != 45*time.Minute {
		t.Fatalf("durations = (%v, %v, %v)", cfg.AuthTimeout, cfg.DialTimeout, cfg.DrainTimeout)
	}
}

func TestParseConfigRejectsUnknownField(t *testing.T) {
	_, err := ParseConfig(strings.NewReader(validConfig + "backendHost: 0.0.0.0:3306\n"))
	if err == nil || !strings.Contains(err.Error(), "field backendHost not found") {
		t.Fatalf("ParseConfig() error = %v, want unknown field error", err)
	}
}

func TestParseConfigRejectsInvalidTarget(t *testing.T) {
	raw := strings.Replace(validConfig, "0.0.0.0:3306", "0.0.0.0", 1)
	_, err := ParseConfig(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "host:port") {
		t.Fatalf("ParseConfig() error = %v, want host:port error", err)
	}
}

func TestParseConfigRequiresAtLeastOneRole(t *testing.T) {
	raw := strings.Replace(validConfig, "0.0.0.0:3306\n    auditProject: tp-wz3-rd\n    roles:\n      - groups:\n          - user@example.invalid\n        mysqlUser: dbc_admin\n        mysqlPassword: [REDACTED]      - groups:\n          - user@example.invalid\n        mysqlUser: dbc_readonly\n        mysqlPassword: [REDACTED]",
		"0.0.0.0:3306\n    auditProject: tp-wz3-rd\n", 1)
	_, err := ParseConfig(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "至少必須設定一個 role") {
		t.Fatalf("ParseConfig() error = %v, want a missing-role error", err)
	}
}

func TestParseConfigRequiresGroupsInRole(t *testing.T) {
	raw := strings.Replace(validConfig, "groups:\n          - user@example.invalid\n", "groups: []\n", 1)
	_, err := ParseConfig(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "至少必須設定一個 group") {
		t.Fatalf("ParseConfig() error = %v, want a missing-group error", err)
	}
}

func TestParseConfigRequiresMySQLCredentials(t *testing.T) {
	const raw = `
audience: dbc-gateway
tls:
  certificateFile: /tls/tls.crt
  privateKeyFile: /tls/tls.key
targets:
  baremetal-vplus33:
    address: 0.0.0.0:3306
    auditProject: tp-wz3-rd
    roles:
      - groups:
          - user@example.invalid
        mysqlPassword: [REDACTED]
`
	_, err := ParseConfig(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "mysqlUser/mysqlPassword") {
		t.Fatalf("ParseConfig() error = %v, want a missing-credential error", err)
	}
}

func TestParseConfigRejectsGroupInMultipleRoles(t *testing.T) {
	raw := strings.Replace(validConfig, "user@example.invalid", "user@example.invalid", 1)
	_, err := ParseConfig(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "出現在超過一個 role 裡") {
		t.Fatalf("ParseConfig() error = %v, want a duplicate-group-across-roles error", err)
	}
}
