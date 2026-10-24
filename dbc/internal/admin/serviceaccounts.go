// Service account (BUILT_IN, password-authenticated) provisioning and
// password rotation for `dbc admin service-accounts`.
//
// Passwords are never diffable — Cloud SQL has no way to read back an
// account's current password, unlike GRANTs or audit rules — so unlike
// grants/audit-rules, the password-sync step is unconditional: it runs
// every time --apply is used, not just when a gap is detected. Rotation
// uses MySQL's dual-password support (ALTER USER ... RETAIN CURRENT
// PASSWORD, confirmed working against real Cloud SQL) so the old password
// keeps working until DiscardOldPasswords is run separately — that's a
// deliberate, manual step, since only a human knows when every consumer
// has actually switched to the new password.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"example.invalid/go/cloudsqlconn"
	"gopkg.in/yaml.v3"

	"example.invalid/sre/dbc/internal/auth"
	"example.invalid/sre/dbc/internal/config"
)

const (
	// ServiceAccountsDefaultPath is where dbc looks for service-accounts.yaml,
	// relative to the executable — mirrors config.DefaultPath's resolution.
	// This file holds plaintext passwords and must never be committed; the
	// repo's .gitignore excludes it.
	ServiceAccountsDefaultPath = "service-accounts.yaml"

	// ServiceAccountsEnvPath overrides the service-accounts.yaml location.
	ServiceAccountsEnvPath = "DBC_SERVICE_ACCOUNTS_CONF"
)

// ServiceAccountSpec is one desired account from service-accounts.yaml.
type ServiceAccountSpec struct {
	Name     string
	Password string // empty when Absent
	Absent   bool
}

type serviceAccountEntry struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
	State    string `yaml:"state"`
}

type serviceAccountsFile struct {
	Accounts []serviceAccountEntry `yaml:"accounts"`
}

// LocateServiceAccounts finds service-accounts.yaml the same way
// config.Locate finds instances.yaml: env var, then next to the executable,
// then the current directory.
func LocateServiceAccounts() (string, error) {
	if p := os.Getenv(ServiceAccountsEnvPath); p != "" {
		return p, nil
	}

	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		candidate := filepath.Join(filepath.Dir(exe), ServiceAccountsDefaultPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if _, err := os.Stat(ServiceAccountsDefaultPath); err == nil {
		return ServiceAccountsDefaultPath, nil
	}

	return "", fmt.Errorf("找不到 %s，請在 dbc 執行檔同目錄建立這個檔案（內含密碼，不要 commit 進 git），或設定 %s 指向它",
		ServiceAccountsDefaultPath, ServiceAccountsEnvPath)
}

// ParseServiceAccounts decodes service-accounts.yaml's content.
func ParseServiceAccounts(data []byte) ([]ServiceAccountSpec, error) {
	var f serviceAccountsFile
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("解析 %s 失敗: %w", ServiceAccountsDefaultPath, err)
	}

	seen := map[string]bool{}
	specs := make([]ServiceAccountSpec, 0, len(f.Accounts))
	for _, e := range f.Accounts {
		if e.Name == "" {
			return nil, fmt.Errorf("有帳號沒有填 name")
		}
		if seen[e.Name] {
			return nil, fmt.Errorf("帳號 %s 重複定義", e.Name)
		}
		seen[e.Name] = true

		var absent bool
		switch e.State {
		case "", "present":
			absent = false
		case "absent":
			absent = true
		default:
			return nil, fmt.Errorf("帳號 %s 的 state 值無效: %q（只能是空白或 absent）", e.Name, e.State)
		}

		if !absent && e.Password == "" {
			return nil, fmt.Errorf("帳號 %s 沒有設定 password（state 不是 absent 就一定要有密碼）", e.Name)
		}

		specs = append(specs, ServiceAccountSpec{Name: e.Name, Password: [REDACTED], Absent: absent})
	}
	return specs, nil
}

// InstanceServiceAccounts is the result of inspecting (and optionally
// syncing) one instance's service accounts.
type InstanceServiceAccounts struct {
	Instance config.Instance
	ListErr  error
	Actions  []ActionResult
}

// listBuiltInUsers returns the BUILT_IN-type usernames already registered
// on inst.
func listBuiltInUsers(ctx context.Context, inst config.Instance) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "gcloud", "sql", "users", "list",
		"--instance="+inst.Name,
		"--project="+inst.Project,
		"--format=json",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("查詢使用者清單失敗: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	var users []gcloudUser
	if err := json.Unmarshal(out, &users); err != nil {
		return nil, fmt.Errorf("解析使用者清單失敗: %w", err)
	}

	// gcloud sql users list omits "type" entirely for BUILT_IN accounts
	// (confirmed against real output — it's not the literal string
	// "BUILT_IN", the field is simply absent), unlike CLOUD_IAM_GROUP/
	// CLOUD_IAM_GROUP_USER/CLOUD_IAM_USER which always set it explicitly.
	existing := map[string]bool{}
	for _, u := range users {
		if u.Type == "" || u.Type == "BUILT_IN" {
			existing[u.Name] = true
		}
	}
	return existing, nil
}

func builtInGrants(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser, name string) ([]string, error) {
	lines, err := runQuery(ctx, dialer, inst, callerUser, fmt.Sprintf("SHOW GRANTS FOR `%s`@`%%`;", name))
	if err != nil {
		return nil, err
	}
	var filtered []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "GRANT USAGE ON *.* TO ") {
			filtered = append(filtered, line)
		}
	}
	return filtered, nil
}

func createBuiltInUser(ctx context.Context, inst config.Instance, name, password string) error {
	// --host defaults to an empty string (a distinct, separate account from
	// user@'%') if omitted — confirmed the hard way against a real
	// instance, where every subsequent user@'%' statement then failed with
	// "no such grant defined". Always pin it explicitly.
	out, err := exec.CommandContext(ctx, "gcloud", "sql", "users", "create", name,
		"--instance="+inst.Name, "--project="+inst.Project, "--password=[REDACTED], "--host=%",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func dropBuiltInUser(ctx context.Context, inst config.Instance, name string) error {
	out, err := exec.CommandContext(ctx, "gcloud", "sql", "users", "delete", name,
		"--instance="+inst.Name, "--project="+inst.Project, "--host=%", "--quiet",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// mysqlStringLiteral escapes s for embedding in a single-quoted MySQL
// string literal (backslash first, then quote — order matters, escaping
// the quote first would double-escape the backslashes just inserted).
func mysqlStringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

// syncPassword sets name's password to the new value while keeping the old
// one valid (MySQL 8.0 dual-password support) — confirmed working against
// real Cloud SQL with just the cloudsqlsuperuser role, no extra grant
// needed.
func syncPassword(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser, name, password string) error {
	sql := fmt.Sprintf("ALTER USER `%s`@`%%` IDENTIFIED BY '%s' RETAIN CURRENT PASSWORD;", name, mysqlStringLiteral(password))
	_, err := runQuery(ctx, dialer, inst, callerUser, sql)
	return err
}

// DiscardOldPassword retires name's secondary (old) password — call only
// once every consumer has confirmed switching to the new one.
func discardOldPassword(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser, name string) error {
	sql := fmt.Sprintf("ALTER USER `%s`@`%%` DISCARD OLD PASSWORD;", name)
	_, err := runQuery(ctx, dialer, inst, callerUser, sql)
	return err
}

// planServiceAccountActions compares specs against inst's actual accounts
// and returns the create/sync/delete actions needed. discardOld selects
// between the normal sync plan and the separate "retire old passwords"
// plan — the two are never mixed into one --apply run.
func planServiceAccountActions(dialer *cloudsqlconn.Dialer, callerUser string, inst config.Instance, existing map[string]bool, grants map[string][]string, specs []ServiceAccountSpec, discardOld bool) []action {
	applicable := inst.Role != "replica"
	note := ""
	if !applicable {
		note = "（複本會透過複寫同步，dbc 不會直接對複本執行）"
	}

	var actions []action
	for _, spec := range specs {
		spec := spec
		exists := existing[spec.Name]

		if spec.Absent {
			if !exists {
				continue
			}
			actions = append(actions, action{
				description: fmt.Sprintf("刪除帳號 %s%s", spec.Name, note),
				applicable:  applicable,
				destructive: true,
				run: func(ctx context.Context) error {
					return dropBuiltInUser(ctx, inst, spec.Name)
				},
			})
			continue
		}

		if discardOld {
			if !exists {
				continue
			}
			actions = append(actions, action{
				description: fmt.Sprintf("停用帳號 %s 的舊密碼%s", spec.Name, note),
				applicable:  applicable,
				run: func(ctx context.Context) error {
					return discardOldPassword(ctx, dialer, inst, callerUser, spec.Name)
				},
			})
			continue
		}

		if !exists {
			actions = append(actions, action{
				description: fmt.Sprintf("建立帳號 %s、設定密碼、掛上 %s%s", spec.Name, SuperuserRole, note),
				applicable:  applicable,
				run: func(ctx context.Context) error {
					if err := createBuiltInUser(ctx, inst, spec.Name, spec.Password); err != nil {
						return err
					}
					return assignRoles(ctx, inst, "BUILT_IN", spec.Name, "%", SuperuserRole, false)
				},
			})
			continue
		}

		if !hasRole(grants[spec.Name], SuperuserRole) {
			actions = append(actions, action{
				description: fmt.Sprintf("補上帳號 %s 的 %s%s", spec.Name, SuperuserRole, note),
				applicable:  applicable,
				run: func(ctx context.Context) error {
					return assignRoles(ctx, inst, "BUILT_IN", spec.Name, "%", SuperuserRole, false)
				},
			})
		}

		actions = append(actions, action{
			description: fmt.Sprintf("同步帳號 %s 的密碼（保留舊密碼可用）%s", spec.Name, note),
			applicable:  applicable,
			run: func(ctx context.Context) error {
				return syncPassword(ctx, dialer, inst, callerUser, spec.Name, spec.Password)
			},
		})
	}
	return actions
}

// ServiceAccounts inspects every instance's service accounts against specs,
// and — if apply is true — executes the resulting actions. discardOld
// switches the plan from the normal create/sync/delete set to the separate
// "retire every account's old password" set; the two are never combined
// into one call, matching the CLI's two independent flows (ordinary sync
// vs. `--discard-old-password`).
func ServiceAccounts(ctx context.Context, instances []config.Instance, specs []ServiceAccountSpec, apply, discardOld bool) ([]InstanceServiceAccounts, error) {
	email, err := auth.Account(ctx)
	if err != nil {
		return nil, err
	}
	callerUser, err := auth.MySQLUser(email)
	if err != nil {
		return nil, err
	}

	dialer, err := cloudsqlconn.NewDialer(ctx, cloudsqlconn.WithIAMAuthN())
	if err != nil {
		return nil, fmt.Errorf("建立 Cloud SQL dialer 失敗: %w", err)
	}
	defer dialer.Close()

	const concurrency = 8
	sem := make(chan struct{}, concurrency)
	results := make([]InstanceServiceAccounts, len(instances))
	var wg sync.WaitGroup
	for i, inst := range instances {
		wg.Add(1)
		go func(i int, inst config.Instance) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = inspectServiceAccountsOne(ctx, dialer, inst, callerUser, specs, apply, discardOld)
		}(i, inst)
	}
	wg.Wait()

	return results, nil
}

func inspectServiceAccountsOne(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser string, specs []ServiceAccountSpec, apply, discardOld bool) InstanceServiceAccounts {
	existing, err := listBuiltInUsers(ctx, inst)
	if err != nil {
		return InstanceServiceAccounts{Instance: inst, ListErr: err}
	}

	grants := map[string][]string{}
	for _, spec := range specs {
		if spec.Absent || !existing[spec.Name] {
			continue
		}
		g, err := builtInGrants(ctx, dialer, inst, callerUser, spec.Name)
		if err == nil {
			grants[spec.Name] = g
		}
	}

	isa := InstanceServiceAccounts{Instance: inst}
	for _, a := range planServiceAccountActions(dialer, callerUser, inst, existing, grants, specs, discardOld) {
		ar := ActionResult{Description: a.description, Applicable: a.applicable, Destructive: a.destructive}
		if apply && a.applicable {
			ar.Applied = true
			ar.Err = a.run(ctx)
		}
		isa.Actions = append(isa.Actions, ar)
	}
	return isa
}
