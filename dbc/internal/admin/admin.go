// Package admin implements `dbc admin` subcommands: inspection (and,
// opt-in, remediation) tools for people with elevated access, using the
// caller's own IAM identity rather than any embedded credential. If the
// caller lacks permission for a given query or fix, that action simply
// fails — there's nothing in dbc that grants access beyond what the
// caller already has.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"example.invalid/go/cloudsqlconn"

	"example.invalid/sre/dbc/internal/auth"
	"example.invalid/sre/dbc/internal/proxy"

	"example.invalid/sre/dbc/internal/config"
)

// The two expected IAM groups and the baseline each one should have.
// db-prod-user: SELECT on every business database, nothing more.
// db-prod-admin: the cloudsqlsuperuser built-in role, nothing else needed.
const (
	GroupUserName  = "user@example.invalid"
	GroupAdminName = "user@example.invalid"

	SuperuserRole = "cloudsqlsuperuser"
)

// systemDatabases are excluded when determining "business databases" that
// db-prod-user should have SELECT on.
var systemDatabases = map[string]bool{
	"mysql":              true,
	"sys":                true,
	"information_schema": true,
	"performance_schema": true,
}

// GroupUser is a cloud_iam_group type DB user found on an instance.
type GroupUser struct {
	Name string // e.g. "user@example.invalid"
	Host string
}

// GroupGrants is one group's SHOW GRANTS result, or the reason it couldn't
// be determined (most commonly: caller lacks permission to query it).
type GroupGrants struct {
	Group  GroupUser
	Grants []string
	Err    error
}

// ActionResult is one planned (and, if apply was requested and it's
// Applicable, executed) remediation step.
type ActionResult struct {
	Description string
	// Applicable is false for replica-only findings: still shown so the
	// gap is visible, but never executed even with --apply (replicas
	// mirror the primary via replication; dbc never writes to one).
	Applicable bool
	Applied    bool // true once Apply actually ran this (implies Applicable)
	Err        error
	// Destructive marks an action that removes something (e.g. deleting an
	// account) rather than adding/fixing it — callers (cli) show these
	// separately and gate them behind their own confirmation, not lumped in
	// with routine sync actions.
	Destructive bool
}

// InstanceGrants is the result of inspecting (and optionally fixing) one
// instance.
type InstanceGrants struct {
	Instance  config.Instance
	Groups    []GroupGrants
	Databases []string // business databases; nil if DBErr is set
	DBErr     error
	ListErr   error // set if even listing the instance's users failed
	Actions   []ActionResult
}

type gcloudUser struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Type string `json:"type"`
}

// action is an internal planned step: a description plus the func that
// executes it. applicable false means "diagnostic only" — never run, even
// under --apply (see ActionResult.Applicable).
type action struct {
	description string
	applicable  bool
	destructive bool
	run         func(ctx context.Context) error
}

// listGroups returns the cloud_iam_group type DB users registered on inst,
// using the caller's own gcloud identity.
func listGroups(ctx context.Context, inst config.Instance) ([]GroupUser, error) {
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

	var groups []GroupUser
	for _, u := range users {
		if u.Type == "CLOUD_IAM_GROUP" {
			groups = append(groups, GroupUser{Name: u.Name, Host: u.Host})
		}
	}
	return groups, nil
}

// runQuery opens a short-lived proxy connection to inst and runs sql as
// callerUser, returning each non-empty output line.
func runQuery(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser, sql string) ([]string, error) {
	ln, err := proxy.Listen()
	if err != nil {
		return nil, fmt.Errorf("建立暫時連線失敗: %w", err)
	}
	defer ln.Close()

	go proxy.ServeOnce(ctx, dialer, ln, inst)

	port := ln.Addr().(*net.TCPAddr).Port
	cmd := exec.CommandContext(ctx, "mysql",
		"-h", "0.0.0.0",
		"-P", strconv.Itoa(port),
		"-u", callerUser,
		"--batch", "--skip-column-names",
		"-e", sql,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}

	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// groupPrincipal splits a group's reported name into the user/host form
// MySQL grant statements need (e.g. `db-prod-user`@`example.invalid`) —
// group principals use the domain as host, unlike individual users whose
// host is `%`.
func groupPrincipal(g GroupUser) (user, host string) {
	user, host, ok := strings.Cut(g.Name, "@")
	if !ok {
		return g.Name, g.Host
	}
	return user, host
}

func showGrants(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser string, group GroupUser) ([]string, error) {
	user, host := groupPrincipal(group)
	lines, err := runQuery(ctx, dialer, inst, callerUser, fmt.Sprintf("SHOW GRANTS FOR `%s`@`%s`;", user, host))
	if err != nil {
		return nil, err
	}

	// GRANT USAGE ON *.* is MySQL's placeholder for "this account exists,
	// no privileges" — every principal has it, it's never informative.
	var filtered []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "GRANT USAGE ON *.* TO ") {
			filtered = append(filtered, line)
		}
	}
	return filtered, nil
}

func businessDatabases(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser string) ([]string, error) {
	lines, err := runQuery(ctx, dialer, inst, callerUser, "SHOW DATABASES;")
	if err != nil {
		return nil, err
	}
	var dbs []string
	for _, line := range lines {
		if !systemDatabases[line] {
			dbs = append(dbs, line)
		}
	}
	return dbs, nil
}

var grantSelectRe = regexp.MustCompile("^GRANT SELECT ON `([^`]+)`\\.\\* TO")

// selectedDatabases extracts the databases a SHOW GRANTS output already
// has a `GRANT SELECT ON `db`.* TO ...` line for.
func selectedDatabases(grants []string) map[string]bool {
	dbs := map[string]bool{}
	for _, line := range grants {
		if m := grantSelectRe.FindStringSubmatch(line); m != nil {
			dbs[m[1]] = true
		}
	}
	return dbs
}

// hasRole reports whether grants already shows role assigned. Cloud SQL
// renders role grants distinctly from privilege grants — no " ON " clause
// — e.g. "GRANT `cloudiamgroup`@`%`,`cloudsqlsuperuser`@`%` TO `x`@`y`".
func hasRole(grants []string, role string) bool {
	needle := "`" + role + "`@"
	for _, line := range grants {
		if strings.HasPrefix(line, "GRANT ") && !strings.Contains(line, " ON ") && strings.Contains(line, needle) {
			return true
		}
	}
	return false
}

// HasSuperuserRole reports whether grants already shows SuperuserRole
// assigned — exported so callers (e.g. cli's verbose grant listing) can
// condense an already-correct db-prod-admin block without re-parsing.
func HasSuperuserRole(grants []string) bool {
	return hasRole(grants, SuperuserRole)
}

// assignRoles calls gcloud sql users assign-roles. host is passed through
// as an explicit --host flag when non-empty — required for BUILT_IN
// accounts (confirmed the hard way: omitting it 404s, even though the
// account itself was created with host '%') but must stay omitted for
// CLOUD_IAM_GROUP callers, which resolve correctly without it.
func assignRoles(ctx context.Context, inst config.Instance, userType, groupName, host, roles string, revokeExisting bool) error {
	args := []string{"sql", "users", "assign-roles", groupName,
		"--type=" + userType,
		"--instance=" + inst.Name,
		"--project=" + inst.Project,
		"--database-roles=" + roles,
	}
	if host != "" {
		args = append(args, "--host="+host)
	}
	if revokeExisting {
		args = append(args, "--revoke-existing-roles")
	}
	out, err := exec.CommandContext(ctx, "gcloud", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func createGroupUser(ctx context.Context, inst config.Instance, groupName string) error {
	out, err := exec.CommandContext(ctx, "gcloud", "sql", "users", "create", groupName,
		"--instance="+inst.Name, "--project="+inst.Project, "--type=cloud_iam_group",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// planActions compares ig's inspected state against the expected baseline
// for db-prod-user/db-prod-admin and returns the fixes needed. inst/dialer/
// callerUser are captured so the returned actions know how to execute
// themselves.
func planActions(dialer *cloudsqlconn.Dialer, callerUser string, ig InstanceGrants) []action {
	if ig.ListErr != nil {
		return nil // can't plan without knowing what groups actually exist
	}
	inst := ig.Instance

	// Replicas only ever mirror the primary: GRANTs on the primary
	// replicate automatically (this is exactly how db-prod-user's original
	// SELECT grants ended up on replicas in the first place), and replicas
	// run read_only=ON so a direct write would fail there anyway. So the
	// gap is still computed and shown for a replica (useful to see —
	// usually means the primary itself isn't fixed yet, or replication is
	// lagging) but marked non-applicable: --apply skips it unconditionally.
	applicable := inst.Role != "replica"
	note := ""
	if !applicable {
		note = "（從庫透過複寫從主庫同步，dbc 不會直接對從庫執行）"
	}

	byName := map[string]GroupGrants{}
	for _, g := range ig.Groups {
		byName[g.Group.Name] = g
	}

	var actions []action

	userUser, userHost := groupPrincipal(GroupUser{Name: GroupUserName})

	if gg, ok := byName[GroupUserName]; ok {
		if gg.Err == nil {
			have := selectedDatabases(gg.Grants)
			if ig.DBErr == nil {
				for _, db := range ig.Databases {
					if have[db] {
						continue
					}
					db := db
					actions = append(actions, action{
						description: fmt.Sprintf("GRANT SELECT ON `%s`.* TO `%s`@`%s`%s", db, userUser, userHost, note),
						applicable:  applicable,
						run: func(ctx context.Context) error {
							_, err := runQuery(ctx, dialer, inst, callerUser,
								fmt.Sprintf("GRANT SELECT ON `%s`.* TO `%s`@`%s`;", db, userUser, userHost))
							return err
						},
					})
				}
			}
			// Normally db-prod-user having cloudsqlsuperuser is an anomaly
			// to revoke — but some instances are configured (per-instance,
			// in instances.yaml) as a deliberate exception where
			// db-prod-user should ALSO hold it, on top of its usual
			// SELECT-only baseline. Only this one check branches on that;
			// the SELECT gap-filling above is unconditional either way.
			hasSuperuser := hasRole(gg.Grants, SuperuserRole)
			switch {
			case inst.DBProdUserCloudSQLSuperuser && !hasSuperuser:
				actions = append(actions, action{
					description: fmt.Sprintf("補上 %s 的 %s（此實例設定為例外）%s", GroupUserName, SuperuserRole, note),
					applicable:  applicable,
					run: func(ctx context.Context) error {
						return assignRoles(ctx, inst, "CLOUD_IAM_GROUP", GroupUserName, "", SuperuserRole, false)
					},
				})
			case !inst.DBProdUserCloudSQLSuperuser && hasSuperuser:
				actions = append(actions, action{
					description: fmt.Sprintf("撤銷 %s 意外持有的 %s%s", GroupUserName, SuperuserRole, note),
					applicable:  applicable,
					run: func(ctx context.Context) error {
						return assignRoles(ctx, inst, "CLOUD_IAM_GROUP", GroupUserName, "", "", true)
					},
				})
			}
		}
	} else {
		actions = append(actions, action{
			description: fmt.Sprintf("建立 %s（這台實例上還沒有這個群組；建立後請重新執行以補齊 SELECT 授權）%s", GroupUserName, note),
			applicable:  applicable,
			run: func(ctx context.Context) error {
				return createGroupUser(ctx, inst, GroupUserName)
			},
		})
	}

	if gg, ok := byName[GroupAdminName]; ok {
		if gg.Err == nil && !hasRole(gg.Grants, SuperuserRole) {
			actions = append(actions, action{
				description: fmt.Sprintf("補上 %s 的 %s%s", GroupAdminName, SuperuserRole, note),
				applicable:  applicable,
				run: func(ctx context.Context) error {
					return assignRoles(ctx, inst, "CLOUD_IAM_GROUP", GroupAdminName, "", SuperuserRole, false)
				},
			})
		}
	} else {
		actions = append(actions, action{
			description: fmt.Sprintf("建立 %s 並掛上 %s（這台實例上還沒有這個群組）%s", GroupAdminName, SuperuserRole, note),
			applicable:  applicable,
			run: func(ctx context.Context) error {
				if err := createGroupUser(ctx, inst, GroupAdminName); err != nil {
					return err
				}
				return assignRoles(ctx, inst, "CLOUD_IAM_GROUP", GroupAdminName, "", SuperuserRole, false)
			},
		})
	}

	return actions
}

// Run inspects every instance in instances — which IAM groups exist, what
// each grants, and which business databases exist — and computes the
// remediation plan (InstanceGrants.Actions) without executing anything.
func Run(ctx context.Context, instances []config.Instance) ([]InstanceGrants, error) {
	return inspect(ctx, instances, false)
}

// Apply does the same inspection as Run, then actually executes each
// planned action and records the outcome in InstanceGrants.Actions.
func Apply(ctx context.Context, instances []config.Instance) ([]InstanceGrants, error) {
	return inspect(ctx, instances, true)
}

func inspect(ctx context.Context, instances []config.Instance, apply bool) ([]InstanceGrants, error) {
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

	// Instances are independent — inspecting them concurrently turns "sum of
	// every instance's latency" into "roughly the slowest one", since each
	// gcloud call and Cloud SQL connection dominates with its own network
	// round-trip. Bounded so a large instances.yaml doesn't fire off dozens
	// of gcloud/mysql subprocesses at once. Sharing one dialer across
	// goroutines mirrors what `dbc start` (proxy.Serve) already does in
	// production — a single dialer serving many concurrent Dial calls.
	const concurrency = 8
	sem := make(chan struct{}, concurrency)
	results := make([]InstanceGrants, len(instances))
	var wg sync.WaitGroup
	for i, inst := range instances {
		wg.Add(1)
		go func(i int, inst config.Instance) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = inspectOne(ctx, dialer, inst, callerUser, apply)
		}(i, inst)
	}
	wg.Wait()

	return results, nil
}

// inspectOne does the full inspection (and, if apply, remediation) for a
// single instance. Safe to call concurrently for different instances
// sharing the same dialer.
func inspectOne(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser string, apply bool) InstanceGrants {
	groups, err := listGroups(ctx, inst)
	if err != nil {
		return InstanceGrants{Instance: inst, ListErr: err}
	}

	ig := InstanceGrants{Instance: inst}
	for _, g := range groups {
		grants, err := showGrants(ctx, dialer, inst, callerUser, g)
		ig.Groups = append(ig.Groups, GroupGrants{Group: g, Grants: grants, Err: err})
	}

	dbs, dbErr := businessDatabases(ctx, dialer, inst, callerUser)
	ig.Databases, ig.DBErr = dbs, dbErr

	for _, a := range planActions(dialer, callerUser, ig) {
		ar := ActionResult{Description: a.description, Applicable: a.applicable, Destructive: a.destructive}
		if apply && a.applicable {
			ar.Applied = true
			ar.Err = a.run(ctx)
		}
		ig.Actions = append(ig.Actions, ar)
	}

	return ig
}
