// Audit rule inspection/remediation for `dbc admin audit-rules`: compares
// each instance's Cloud SQL MySQL audit rules (mysql.cloudsql_*_audit_rule)
// against a fixed baseline, and can reset an instance back to that baseline.
//
// DQL is intentionally not covered here — Cloud SQL only audits activity on
// the primary instance, never on a replica (confirmed against the official
// docs and by testing), so a DQL rule here would be blind to everything run
// against a replica. Read-query logging is instead planned as a separate
// dbc-proxy-level mechanism that works regardless of primary/replica.
package admin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"example.invalid/go/cloudsqlconn"

	"example.invalid/sre/dbc/internal/auth"
	"example.invalid/sre/dbc/internal/config"
)

// auditRuleSpec is one expected baseline rule.
type auditRuleSpec struct {
	description                   string
	users, db, obj, ops, opResult string
}

// expectedAuditRules is the full baseline every instance should have.
//
// The exclusion below exists because Cloud SQL's own HA/replication health
// check writes to mysql.heartbeat every few seconds as root — confirmed by
// inspecting real audit log entries — which would otherwise flood the DML
// log with a purely internal operation that isn't a real business write.
// Scoped to that one table specifically, not the whole mysql schema: mysql
// also holds mysql.user/mysql.db/etc — genuinely sensitive tables that
// should stay fully audited, not swept away by a blanket schema-wide
// exclusion aimed at killing one noisy table.
var expectedAuditRules = []auditRuleSpec{
	{"全員 DML/DDL/DCL", "*", "*", "*", "dml,ddl,dcl", "B"},
	{"排除 Cloud SQL HA 健康檢查心跳（mysql.heartbeat）的 DML",
		"*", "mysql", "heartbeat", "dml", "E"},
}

// AuditRule is one row of mysql.cloudsql_list_audit_rule's output.
type AuditRule struct {
	ID, Username, DB, Object, Ops, OpResult string
}

// InstanceAuditRules is the result of inspecting (and optionally resetting)
// one instance's audit rules.
type InstanceAuditRules struct {
	Instance config.Instance
	Rules    []AuditRule
	ListErr  error // set if querying the current rules failed (e.g. caller lacks permission)
	Actions  []ActionResult
}

// listAuditRules runs mysql.cloudsql_list_audit_rule and parses its output.
// The CALL's own result set (rule rows, 6 tab-separated fields each) and the
// trailing `SELECT @outval, @outmsg` (always exactly 2 fields) share one
// --skip-column-names output, so rows are told apart by field count —
// there's no other reliable delimiter in batch mode.
//
// The status output vars matter here in a way they don't for showGrants:
// testing against production found that cloudsql_create/delete/list_audit_rule
// can "succeed" (exit code 0, no error text) while still failing logically
// (e.g. a malformed user@host pattern) — the only sign is a non-zero outval.
func listAuditRules(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser string) ([]AuditRule, error) {
	lines, err := runQuery(ctx, dialer, inst, callerUser,
		"CALL mysql.cloudsql_list_audit_rule('*', @outval, @outmsg); SELECT @outval, @outmsg;")
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("查詢稽核規則沒有回傳任何內容")
	}

	last := strings.Split(lines[len(lines)-1], "\t")
	if len(last) == 2 && last[0] != "0" {
		return nil, fmt.Errorf("%s", last[1])
	}

	var rules []AuditRule
	for _, line := range lines[:len(lines)-1] {
		if f := strings.Split(line, "\t"); len(f) == 6 {
			rules = append(rules, AuditRule{ID: f[0], Username: f[1], DB: f[2], Object: f[3], Ops: f[4], OpResult: f[5]})
		}
	}
	return rules, nil
}

// deleteAuditRules removes the given rule IDs via mysql.cloudsql_delete_audit_rule.
func deleteAuditRules(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser string, ids []string) error {
	sql := fmt.Sprintf("CALL mysql.cloudsql_delete_audit_rule('%s',1,@outval,@outmsg); SELECT @outval, @outmsg;",
		strings.Join(ids, ","))
	return runAuditStatusQuery(ctx, dialer, inst, callerUser, sql)
}

// createAuditRule adds one baseline rule via mysql.cloudsql_create_audit_rule.
func createAuditRule(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser string, spec auditRuleSpec, reloadMode int) error {
	sql := fmt.Sprintf("CALL mysql.cloudsql_create_audit_rule('%s','%s','%s','%s','%s',%d,@outval,@outmsg); SELECT @outval, @outmsg;",
		spec.users, spec.db, spec.obj, spec.ops, spec.opResult, reloadMode)
	return runAuditStatusQuery(ctx, dialer, inst, callerUser, sql)
}

// runAuditStatusQuery runs sql (expected to end with `SELECT @outval, @outmsg;`)
// and turns a non-zero outval into an error — exit code alone isn't reliable
// for these stored procedures (see listAuditRules's comment).
func runAuditStatusQuery(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser, sql string) error {
	lines, err := runQuery(ctx, dialer, inst, callerUser, sql)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return fmt.Errorf("沒有回傳執行狀態")
	}
	last := strings.Split(lines[len(lines)-1], "\t")
	if len(last) == 2 && last[0] != "0" {
		return fmt.Errorf("%s", last[1])
	}
	return nil
}

// hasAuditRule reports whether rules already contains an exact match for spec.
func hasAuditRule(rules []AuditRule, spec auditRuleSpec) bool {
	for _, r := range rules {
		if r.Username == spec.users && r.DB == spec.db && r.Object == spec.obj &&
			r.Ops == spec.ops && r.OpResult == spec.opResult {
			return true
		}
	}
	return false
}

// auditRulesMatch reports whether current is exactly the expected baseline —
// same count, and every expected rule present. Any drift (missing, extra, or
// differently-shaped rules) counts as a mismatch; there's no partial diff.
func auditRulesMatch(current []AuditRule, expected []auditRuleSpec) bool {
	if len(current) != len(expected) {
		return false
	}
	for _, spec := range expected {
		if !hasAuditRule(current, spec) {
			return false
		}
	}
	return true
}

// resetAuditRules deletes every rule currently on inst (if any) and recreates
// the full baseline from scratch. A failure partway through is reported, not
// rolled back — a re-run of --apply will pick up wherever it left off.
func resetAuditRules(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser string, currentIDs []string) error {
	if len(currentIDs) > 0 {
		if err := deleteAuditRules(ctx, dialer, inst, callerUser, currentIDs); err != nil {
			return fmt.Errorf("清空舊規則失敗: %w", err)
		}
	}
	for i, spec := range expectedAuditRules {
		reload := 0
		if i == len(expectedAuditRules)-1 {
			reload = 1 // 只有最後一條才真的觸發 reload，前面延後生效
		}
		if err := createAuditRule(ctx, dialer, inst, callerUser, spec, reload); err != nil {
			return fmt.Errorf("建立規則「%s」失敗: %w", spec.description, err)
		}
	}
	return nil
}

// planAuditRuleActions compares iar's inspected state against the baseline
// and returns the single reset action needed, if any.
func planAuditRuleActions(dialer *cloudsqlconn.Dialer, callerUser string, iar InstanceAuditRules) []action {
	if iar.ListErr != nil {
		return nil // can't plan without knowing what's actually there
	}
	if auditRulesMatch(iar.Rules, expectedAuditRules) {
		return nil
	}

	inst := iar.Instance

	// Cloud SQL only audits activity on the primary — a replica's rules are
	// purely a replication side-effect of the primary's, never enforced
	// there directly. So a replica gap is still surfaced (usually means the
	// primary itself needs fixing, or replication hasn't caught up yet) but
	// never auto-applied.
	applicable := inst.Role != "replica"
	note := ""
	if !applicable {
		note = "（複本本身不會產生稽核紀錄，這裡只是複寫同步的結果，dbc 不會直接對複本執行）"
	}

	ids := make([]string, len(iar.Rules))
	for i, r := range iar.Rules {
		ids[i] = r.ID
	}

	return []action{{
		description: fmt.Sprintf("清空目前 %d 條規則，重建成預期的 %d 條 baseline%s", len(iar.Rules), len(expectedAuditRules), note),
		applicable:  applicable,
		run: func(ctx context.Context) error {
			return resetAuditRules(ctx, dialer, inst, callerUser, ids)
		},
	}}
}

// RunAuditRules inspects every instance's audit rules against the baseline
// without changing anything.
func RunAuditRules(ctx context.Context, instances []config.Instance) ([]InstanceAuditRules, error) {
	return inspectAuditRules(ctx, instances, false)
}

// ApplyAuditRules does the same inspection as RunAuditRules, then actually
// resets any instance whose rules don't match the baseline.
func ApplyAuditRules(ctx context.Context, instances []config.Instance) ([]InstanceAuditRules, error) {
	return inspectAuditRules(ctx, instances, true)
}

func inspectAuditRules(ctx context.Context, instances []config.Instance, apply bool) ([]InstanceAuditRules, error) {
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
	results := make([]InstanceAuditRules, len(instances))
	var wg sync.WaitGroup
	for i, inst := range instances {
		wg.Add(1)
		go func(i int, inst config.Instance) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = inspectAuditRulesOne(ctx, dialer, inst, callerUser, apply)
		}(i, inst)
	}
	wg.Wait()

	return results, nil
}

func inspectAuditRulesOne(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, callerUser string, apply bool) InstanceAuditRules {
	rules, err := listAuditRules(ctx, dialer, inst, callerUser)
	if err != nil {
		return InstanceAuditRules{Instance: inst, ListErr: err}
	}

	iar := InstanceAuditRules{Instance: inst, Rules: rules}
	for _, a := range planAuditRuleActions(dialer, callerUser, iar) {
		ar := ActionResult{Description: a.description, Applicable: a.applicable, Destructive: a.destructive}
		if apply && a.applicable {
			ar.Applied = true
			ar.Err = a.run(ctx)
		}
		iar.Actions = append(iar.Actions, ar)
	}

	return iar
}
