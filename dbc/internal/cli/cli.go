// Package cli wires up the dbc subcommands.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"example.invalid/spf13/cobra"

	"example.invalid/sre/dbc/internal/admin"
	"example.invalid/sre/dbc/internal/auth"
	"example.invalid/sre/dbc/internal/config"
	"example.invalid/sre/dbc/internal/connect"
	"example.invalid/sre/dbc/internal/doctor"
	"example.invalid/sre/dbc/internal/proxy"
	"example.invalid/sre/dbc/internal/upgrade"
)

// Execute runs the dbc CLI.
func Execute(version string) error {
	root := &cobra.Command{
		Use:           "dbc",
		Short:         "連線到公司 Cloud SQL 正式資料庫（IAM 驗證，免記密碼）",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newSetupCmd(),
		newDoctorCmd(version),
		newStartCmd(version),
		newConnectCmd(),
		newListCmd(),
		newUpgradeCmd(version),
		newAdminCmd(),
	)

	return root.Execute()
}

func loadInstances() ([]config.Instance, error) {
	path, err := config.Locate()
	if err != nil {
		return nil, err
	}
	return config.Load(path)
}

// skipBareMetal filters out bare-metal (non-Cloud SQL) instances for `dbc
// admin` subcommands, printing a note for each one skipped. These commands
// are all built on Cloud SQL Admin API calls or Cloud-SQL-only system
// stored procedures — a self-hosted MySQL server has no equivalent, so
// there's nothing meaningful for them to check/apply there.
func skipBareMetal(instances []config.Instance, cmdName string) []config.Instance {
	var kept []config.Instance
	for _, inst := range instances {
		if inst.IsBareMetal() {
			fmt.Printf("ℹ️  %s / %s 是裸機 MySQL，`dbc admin %s` 不支援，已跳過\n", inst.Project, inst.Name, cmdName)
			continue
		}
		kept = append(kept, inst)
	}
	return kept
}

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "檢查依賴、引導 gcloud 登入",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			fmt.Println("檢查 gcloud ...")
			if _, err := auth.Account(ctx); err != nil {
				fmt.Println("尚未登入，開始引導登入流程 (gcloud auth login) ...")
				if err := runInteractive(ctx, "gcloud", "auth", "login"); err != nil {
					return err
				}
			}

			fmt.Println("設定 Application Default Credentials ...")
			if err := auth.EnsureADC(ctx); err != nil {
				if err := runInteractive(ctx, "gcloud", "auth", "application-default", "login"); err != nil {
					return err
				}
			}

			email, err := auth.Account(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("目前登入帳號：%s\n", email)
			fmt.Println("執行 `dbc doctor` 進行完整健康檢查。")
			return nil
		},
	}
}

func newDoctorCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "自助排查連線問題",
		RunE: func(cmd *cobra.Command, args []string) error {
			upgrade.CheckAndNotify(cmd.Context(), version)

			instances, err := loadInstances()
			if err != nil {
				return err
			}

			results := doctor.Run(cmd.Context(), instances)

			failed := false
			for _, r := range results {
				mark := "✅"
				switch {
				case !r.OK && r.Warn:
					mark = "⚠️ "
				case !r.OK:
					mark = "❌"
					failed = true
				}
				if r.Detail != "" {
					fmt.Printf("%s %s — %s\n", mark, r.Name, r.Detail)
				} else {
					fmt.Printf("%s %s\n", mark, r.Name)
				}
			}

			if failed {
				return fmt.Errorf("健康檢查發現問題，請依上方訊息處理")
			}
			return nil
		},
	}
}

func newStartCmd(version string) *cobra.Command {
	var primary, all bool
	var port int
	var address string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "掛載 Cloud SQL 代理（前景執行，Ctrl-C 結束）",
		RunE: func(cmd *cobra.Command, args []string) error {
			// dbc start 是長駐指令，用背景 goroutine 檢查版本，不要讓
			// proxy 啟動的時間被這個非必要的檢查拖慢。
			go upgrade.CheckAndNotify(cmd.Context(), version)

			instances, err := loadInstances()
			if err != nil {
				return err
			}

			var selected []config.Instance
			switch {
			case port != 0:
				selected, err = config.Filter(instances, "", port)
			case all:
				selected, err = config.Filter(instances, "", 0)
			case primary:
				selected, err = config.Filter(instances, "primary", 0)
			default:
				selected = config.DailySet(instances)
			}
			if err != nil {
				return err
			}
			if len(selected) == 0 {
				return fmt.Errorf("沒有符合條件的實例")
			}

			if config.HasWarnablePrimary(selected) {
				fmt.Println("⚠️  主庫連線。日常查詢請用從庫（dbc start）。")
				fmt.Println("    需要寫入權限請先加入 user@example.invalid")
			}
			if n := config.NoReplicaCount(selected); n > 0 {
				fmt.Printf("ℹ️  %d 個實例目前沒有從庫，是唯一的存取方式（非提權操作，仍是唯讀），細節見 dbc list 備註欄。\n", n)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			fmt.Println("Cloud SQL 代理啟動中，按 Ctrl-C 結束：")
			return proxy.Serve(ctx, selected, address, version)
		},
	}

	cmd.Flags().BoolVar(&primary, "primary", false, "掛載主庫（35xx）")
	cmd.Flags().BoolVar(&all, "all", false, "掛載所有實例")
	cmd.Flags().IntVar(&port, "port", 0, "只掛載單一 port 的實例")
	cmd.Flags().StringVar(&address, "address", "0.0.0.0", "代理監聽位址（容器連線可用 0.0.0.0）")
	return cmd
}

func newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect PORT",
		Short: "自動帶入帳號並連線 mysql（需先執行 dbc start）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var port int
			if _, err := fmt.Sscanf(args[0], "%d", &port); err != nil {
				return fmt.Errorf("port 必須是數字: %q", args[0])
			}
			instances, err := loadInstances()
			if err != nil {
				return err
			}
			inst, ok := config.ByPort(instances, port)
			if !ok {
				return fmt.Errorf("找不到 port %d，請用 `dbc list` 查看可用的 port", port)
			}
			return connect.Run(cmd.Context(), port, inst.IsBareMetal())
		},
	}
}

func newUpgradeCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "更新到最新版本（取代目前的執行檔）",
		RunE: func(cmd *cobra.Command, args []string) error {
			return upgrade.Run(cmd.Context(), version)
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "顯示 port 對照表",
		RunE: func(cmd *cobra.Command, args []string) error {
			instances, err := loadInstances()
			if err != nil {
				return err
			}

			sorted := append([]config.Instance(nil), instances...)
			sort.Slice(sorted, func(i, j int) bool {
				if sorted[i].Project != sorted[j].Project {
					return sorted[i].Project < sorted[j].Project
				}
				return sorted[i].Port < sorted[j].Port
			})

			currentProject := ""
			for _, i := range sorted {
				if i.Project != currentProject {
					if currentProject != "" {
						fmt.Println()
					}
					fmt.Printf("%s (%s)\n", i.Project, i.Region)
					fmt.Printf("  %-55s %-8s %-6s %s\n", "CONN_NAME", "ROLE", "PORT", "備註")
					currentProject = i.Project
				}
				note := ""
				switch {
				case i.IsBareMetal():
					note = "裸機 MySQL，非 Cloud SQL"
				case i.NoReplica:
					note = "無從庫，dbc start 預設會掛載"
				case i.AlwaysStart:
					note = "有從庫，但 dbc start 預設也會掛載"
				}
				fmt.Printf("  %-55s %-8s %-6d %s\n", i.ConnName(), i.Role, i.Port, note)
			}
			return nil
		},
	}
}

func newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "管理者專用的唯讀檢查工具（用呼叫者自己的 IAM 身分，沒權限就查不到）",
	}
	cmd.AddCommand(newAdminGrantsCmd())
	cmd.AddCommand(newAdminAuditRulesCmd())
	cmd.AddCommand(newAdminServiceAccountsCmd())
	return cmd
}

func newAdminGrantsCmd() *cobra.Command {
	var apply, verbose bool

	cmd := &cobra.Command{
		Use:   "grants",
		Short: "列出每台實例上的 IAM 群組跟權限，並檢查/修正是否符合預期（db-prod-user 有齊全的 SELECT、db-prod-admin 有 cloudsqlsuperuser）",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			instances, err := loadInstances()
			if err != nil {
				return err
			}
			instances = skipBareMetal(instances, "grants")
			sort.Slice(instances, func(i, j int) bool {
				if instances[i].Project != instances[j].Project {
					return instances[i].Project < instances[j].Project
				}
				return instances[i].Port < instances[j].Port
			})

			// 先永遠用模擬模式跑一次，把現況跟「會做什麼」都印出來，
			// --apply 需要的確認也是基於這次的結果，不是憑空猜。
			results, err := admin.Run(ctx, instances)
			if err != nil {
				return err
			}
			printGrantsReport(results, verbose)

			if !apply {
				return nil
			}

			total := 0
			for _, r := range results {
				for _, a := range r.Actions {
					if a.Applicable {
						total++
					}
				}
			}
			if total == 0 {
				fmt.Println("\n沒有需要修正的項目（從庫上看到的落差不算，那些只能等主庫修正後透過複寫同步）。")
				return nil
			}

			fmt.Printf("\n確定要套用以上 %d 項變更嗎？(y/N): ", total)
			var resp string
			fmt.Scanln(&resp)
			if strings.ToLower(strings.TrimSpace(resp)) != "y" {
				fmt.Println("已取消，沒有套用任何變更。")
				return nil
			}

			applied, err := admin.Apply(ctx, instances)
			if err != nil {
				return err
			}
			printApplyReport(applied)
			return nil
		},
	}

	cmd.Flags().BoolVar(&apply, "apply", false, "確認後真的套用修正（預設只模擬、不會真的改動權限）")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "印出每個群組完整的 GRANT 內容（預設只印摘要跟需要注意的項目）")
	return cmd
}

func newAdminAuditRulesCmd() *cobra.Command {
	var apply, verbose bool

	cmd := &cobra.Command{
		Use:   "audit-rules",
		Short: "檢查每台實例的 Cloud SQL 稽核規則是否符合預期（記錄所有人的 DML/DDL/DCL），並可重置成標準設定",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			instances, err := loadInstances()
			if err != nil {
				return err
			}
			instances = skipBareMetal(instances, "audit-rules")
			sort.Slice(instances, func(i, j int) bool {
				if instances[i].Project != instances[j].Project {
					return instances[i].Project < instances[j].Project
				}
				return instances[i].Port < instances[j].Port
			})

			results, err := admin.RunAuditRules(ctx, instances)
			if err != nil {
				return err
			}
			printAuditRulesReport(results, verbose)

			if !apply {
				return nil
			}

			total := 0
			for _, r := range results {
				for _, a := range r.Actions {
					if a.Applicable {
						total++
					}
				}
			}
			if total == 0 {
				fmt.Println("\n沒有需要修正的項目（複本上看到的落差不算，那些只能等主庫修正後透過複寫同步）。")
				return nil
			}

			fmt.Printf("\n確定要套用以上 %d 項變更嗎？(y/N): ", total)
			var resp string
			fmt.Scanln(&resp)
			if strings.ToLower(strings.TrimSpace(resp)) != "y" {
				fmt.Println("已取消，沒有套用任何變更。")
				return nil
			}

			applied, err := admin.ApplyAuditRules(ctx, instances)
			if err != nil {
				return err
			}
			printAuditApplyReport(applied)
			return nil
		},
	}

	cmd.Flags().BoolVar(&apply, "apply", false, "確認後真的清空並重建成標準規則（預設只模擬、不會真的改動）")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "印出每台實例目前實際的稽核規則清單（預設只印摘要跟需要注意的項目）")
	return cmd
}

func newAdminServiceAccountsCmd() *cobra.Command {
	var apply, discardOld bool
	var port int

	cmd := &cobra.Command{
		Use:   "service-accounts",
		Short: "依 service-accounts.yaml 建立/同步業務服務帳號（BUILT_IN、cloudsqlsuperuser、密碼輪替），可指定刪除",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			instances, err := loadInstances()
			if err != nil {
				return err
			}
			if port != 0 {
				instances, err = config.Filter(instances, "", port)
				if err != nil {
					return err
				}
			}
			instances = skipBareMetal(instances, "service-accounts")
			sort.Slice(instances, func(i, j int) bool {
				if instances[i].Project != instances[j].Project {
					return instances[i].Project < instances[j].Project
				}
				return instances[i].Port < instances[j].Port
			})

			path, err := admin.LocateServiceAccounts()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("讀取 %s 失敗: %w", path, err)
			}
			specs, err := admin.ParseServiceAccounts(data)
			if err != nil {
				return err
			}
			if len(specs) == 0 {
				fmt.Printf("%s 沒有定義任何帳號。\n", path)
				return nil
			}

			results, err := admin.ServiceAccounts(ctx, instances, specs, false, discardOld)
			if err != nil {
				return err
			}
			printServiceAccountsReport(results, discardOld)

			if !apply {
				return nil
			}

			total, destructive := 0, 0
			for _, r := range results {
				for _, a := range r.Actions {
					if a.Applicable {
						total++
						if a.Destructive {
							destructive++
						}
					}
				}
			}
			if total == 0 {
				fmt.Println("\n沒有需要套用的項目（複本上看到的落差不算，那些只能等主庫修正後透過複寫同步）。")
				return nil
			}

			if destructive > 0 {
				fmt.Printf("\n⚠️  以上包含 %d 項刪除帳號的操作，這無法復原。確定要繼續嗎？(y/N): ", destructive)
				var resp string
				fmt.Scanln(&resp)
				if strings.ToLower(strings.TrimSpace(resp)) != "y" {
					fmt.Println("已取消，沒有套用任何變更。")
					return nil
				}
			}

			fmt.Printf("\n確定要套用以上 %d 項變更嗎？(y/N): ", total)
			var resp string
			fmt.Scanln(&resp)
			if strings.ToLower(strings.TrimSpace(resp)) != "y" {
				fmt.Println("已取消，沒有套用任何變更。")
				return nil
			}

			applied, err := admin.ServiceAccounts(ctx, instances, specs, true, discardOld)
			if err != nil {
				return err
			}
			printServiceAccountsApplyReport(applied)
			return nil
		},
	}

	cmd.Flags().BoolVar(&apply, "apply", false, "確認後真的建立/同步帳號（預設只模擬、不會真的改動）")
	cmd.Flags().BoolVar(&discardOld, "discard-old-password", false, "停用每個帳號的舊密碼（獨立於一般同步，請在確認所有服務都已切到新密碼後才執行）")
	cmd.Flags().IntVar(&port, "port", 0, "只針對單一實例執行（例如測試密碼輪替時，不用動到整個機群）")
	return cmd
}

func printServiceAccountsReport(results []admin.InstanceServiceAccounts, discardOld bool) {
	type issue struct {
		instance string
		lines    []string
	}
	var normal, destructive []issue

	for _, r := range results {
		label := fmt.Sprintf("%s / %s", r.Instance.Project, r.Instance.Name)
		var normalLines, destructiveLines []string
		if r.ListErr != nil {
			normalLines = append(normalLines, fmt.Sprintf("⚠️  無法確認：%v", r.ListErr))
		}
		for _, a := range r.Actions {
			if !a.Applicable {
				normalLines = append(normalLines, "ℹ️  "+a.Description)
				continue
			}
			if a.Destructive {
				destructiveLines = append(destructiveLines, "🗑️  "+a.Description)
			} else {
				normalLines = append(normalLines, "📝 "+a.Description)
			}
		}
		if len(normalLines) > 0 {
			normal = append(normal, issue{label, normalLines})
		}
		if len(destructiveLines) > 0 {
			destructive = append(destructive, issue{label, destructiveLines})
		}
	}

	if len(destructive) > 0 {
		fmt.Println("=== ⚠️ 即將刪除的帳號 ===")
		for _, is := range destructive {
			fmt.Println(is.instance)
			for _, line := range is.lines {
				fmt.Printf("  %s\n", line)
			}
		}
		fmt.Println()
	}

	title := "=== 需要注意 ==="
	if discardOld {
		title = "=== 待停用舊密碼的帳號 ==="
	}
	fmt.Println(title)
	if len(normal) == 0 {
		fmt.Println("沒有發現需要處理的項目。")
	}
	for _, is := range normal {
		fmt.Println(is.instance)
		for _, line := range is.lines {
			fmt.Printf("  %s\n", line)
		}
	}
}

func printServiceAccountsApplyReport(results []admin.InstanceServiceAccounts) {
	currentProject := ""
	for _, r := range results {
		var shown []admin.ActionResult
		for _, a := range r.Actions {
			if a.Applicable {
				shown = append(shown, a)
			}
		}
		if len(shown) == 0 {
			continue
		}
		if r.Instance.Project != currentProject {
			if currentProject != "" {
				fmt.Println()
			}
			fmt.Printf("%s (%s)\n", r.Instance.Project, r.Instance.Region)
			currentProject = r.Instance.Project
		}
		fmt.Printf("  %s\n", r.Instance.Name)
		for _, a := range shown {
			mark := "✅"
			if a.Destructive {
				mark = "🗑️ "
			}
			if a.Err != nil {
				fmt.Printf("    ❌ %s：%v\n", a.Description, a.Err)
			} else {
				fmt.Printf("    %s %s\n", mark, a.Description)
			}
		}
	}
}

func printGrantsReport(results []admin.InstanceGrants, verbose bool) {
	printAttentionSummary(results)
	printStatusSummary(results)

	if !verbose {
		fmt.Println("\n（加 --verbose 看每個群組完整的 GRANT 內容）")
		return
	}

	fmt.Println("\n=== 詳細清單 ===")
	currentProject := ""
	for _, r := range results {
		if r.Instance.Project != currentProject {
			if currentProject != "" {
				fmt.Println()
			}
			fmt.Printf("%s (%s)\n", r.Instance.Project, r.Instance.Region)
			currentProject = r.Instance.Project
		}
		fmt.Printf("  %s\n", r.Instance.Name)
		if r.ListErr != nil {
			fmt.Printf("    ⚠️  無法確認：%v\n", r.ListErr)
			continue
		}
		if len(r.Groups) == 0 {
			fmt.Println("    （沒有 IAM 群組）")
		}
		for _, g := range r.Groups {
			fmt.Printf("    %s\n", g.Group.Name)
			if g.Err != nil {
				fmt.Printf("      ⚠️  無法確認：%v\n", g.Err)
				continue
			}
			// db-prod-admin 的內容在正常情況下永遠一樣（cloudiamgroup +
			// cloudsqlsuperuser），沒有異常就濃縮成一行，不用每台實例都
			// 重複印兩行一模一樣的 GRANT。
			if g.Group.Name == admin.GroupAdminName && admin.HasSuperuserRole(g.Grants) {
				fmt.Printf("      ✅ 正常（%s）\n", admin.SuperuserRole)
				continue
			}
			for _, line := range g.Grants {
				suffix := ""
				// db-prod-user's SELECT list is informative and varies per
				// instance, so it's always printed in full (unlike
				// db-prod-admin's fixed grant set above) — but a
				// cloudsqlsuperuser line here would otherwise look like a
				// tool bug or a security incident. Flag it as the
				// deliberate per-instance exception it is.
				if g.Group.Name == admin.GroupUserName && r.Instance.DBProdUserCloudSQLSuperuser && admin.HasSuperuserRole([]string{line}) {
					suffix = "  ← 此實例的例外設定，非異常"
				}
				fmt.Printf("      %s%s\n", line, suffix)
			}
		}
		for _, a := range r.Actions {
			if a.Applicable {
				fmt.Printf("    📝 [模擬] %s\n", a.Description)
			} else {
				fmt.Printf("    ℹ️  %s\n", a.Description)
			}
		}
	}
}

// countActions splits an instance's actions into ones dbc would actually
// fix (Applicable, e.g. on a primary) versus informational-only findings
// (e.g. a replica gap that just mirrors its primary).
func countActions(actions []admin.ActionResult) (applicable, informational int) {
	for _, a := range actions {
		if a.Applicable {
			applicable++
		} else {
			informational++
		}
	}
	return applicable, informational
}

// printStatusSummary prints one compact line per instance: how many fixes
// are needed, or a plain OK.
func printStatusSummary(results []admin.InstanceGrants) {
	width := 0
	for _, r := range results {
		label := fmt.Sprintf("%s / %s", r.Instance.Project, r.Instance.Name)
		if len(label) > width {
			width = len(label)
		}
	}

	fmt.Println("\n=== 摘要 ===")
	for _, r := range results {
		label := fmt.Sprintf("%s / %s", r.Instance.Project, r.Instance.Name)
		applicable, informational := countActions(r.Actions)
		switch {
		case r.ListErr != nil:
			fmt.Printf("%-*s  ⚠️  無法確認\n", width, label)
		case applicable > 0:
			fmt.Printf("%-*s  📝 %d 項待修正\n", width, label, applicable)
		case informational > 0:
			fmt.Printf("%-*s  ℹ️  %d 項僅供參考（從庫，等主庫修正後會自動同步）\n", width, label, informational)
		default:
			fmt.Printf("%-*s  ✅ 正常\n", width, label)
		}
	}
}

// printAttentionSummary prints a compact, scannable list of everything
// that needs a human's attention — action items and query failures —
// before the full (much longer) grant dump, so problems aren't buried.
func printAttentionSummary(results []admin.InstanceGrants) {
	type issue struct {
		instance string
		lines    []string
	}
	var issues []issue

	for _, r := range results {
		label := fmt.Sprintf("%s / %s", r.Instance.Project, r.Instance.Name)
		var lines []string
		if r.ListErr != nil {
			lines = append(lines, fmt.Sprintf("⚠️  無法確認：%v", r.ListErr))
		}
		for _, g := range r.Groups {
			if g.Err != nil {
				lines = append(lines, fmt.Sprintf("⚠️  %s 無法確認：%v", g.Group.Name, g.Err))
			}
		}
		for _, a := range r.Actions {
			if a.Applicable {
				lines = append(lines, "📝 "+a.Description)
			}
		}
		if len(lines) > 0 {
			issues = append(issues, issue{instance: label, lines: lines})
		}
	}

	if len(issues) == 0 {
		fmt.Println("=== 需要注意 ===")
		fmt.Println("沒有發現異常，所有查得到的實例都符合預期。")
		fmt.Println()
		return
	}

	fmt.Println("=== 需要注意 ===")
	for _, is := range issues {
		fmt.Println(is.instance)
		for _, line := range is.lines {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()
}

func printApplyReport(results []admin.InstanceGrants) {
	currentProject := ""
	for _, r := range results {
		// 只回報真的被 --apply 動過的項目；non-applicable（從庫）的項目
		// 從來就沒被執行，不該顯示成 ✅ 完成，直接跳過不列。
		applicable, _ := countActions(r.Actions)
		if applicable == 0 {
			continue
		}
		if r.Instance.Project != currentProject {
			if currentProject != "" {
				fmt.Println()
			}
			fmt.Printf("%s (%s)\n", r.Instance.Project, r.Instance.Region)
			currentProject = r.Instance.Project
		}
		fmt.Printf("  %s\n", r.Instance.Name)
		for _, a := range r.Actions {
			if !a.Applicable {
				continue
			}
			if a.Err != nil {
				fmt.Printf("    ❌ %s：%v\n", a.Description, a.Err)
			} else {
				fmt.Printf("    ✅ %s\n", a.Description)
			}
		}
	}
}

func printAuditRulesReport(results []admin.InstanceAuditRules, verbose bool) {
	printAuditAttentionSummary(results)
	printAuditStatusSummary(results)

	if !verbose {
		fmt.Println("\n（加 --verbose 看每台實例目前實際的稽核規則清單）")
		return
	}

	fmt.Println("\n=== 詳細清單 ===")
	currentProject := ""
	for _, r := range results {
		if r.Instance.Project != currentProject {
			if currentProject != "" {
				fmt.Println()
			}
			fmt.Printf("%s (%s)\n", r.Instance.Project, r.Instance.Region)
			currentProject = r.Instance.Project
		}
		fmt.Printf("  %s\n", r.Instance.Name)
		if r.ListErr != nil {
			fmt.Printf("    ⚠️  無法確認：%v\n", r.ListErr)
			continue
		}
		if len(r.Rules) == 0 {
			fmt.Println("    （沒有任何稽核規則）")
		}
		for _, rule := range r.Rules {
			fmt.Printf("    [id=%s] %s / db=%s obj=%s / %s / %s\n",
				rule.ID, rule.Username, rule.DB, rule.Object, rule.Ops, rule.OpResult)
		}
		for _, a := range r.Actions {
			if a.Applicable {
				fmt.Printf("    📝 [模擬] %s\n", a.Description)
			} else {
				fmt.Printf("    ℹ️  %s\n", a.Description)
			}
		}
	}
}

func printAuditStatusSummary(results []admin.InstanceAuditRules) {
	width := 0
	for _, r := range results {
		label := fmt.Sprintf("%s / %s", r.Instance.Project, r.Instance.Name)
		if len(label) > width {
			width = len(label)
		}
	}

	fmt.Println("\n=== 摘要 ===")
	for _, r := range results {
		label := fmt.Sprintf("%s / %s", r.Instance.Project, r.Instance.Name)
		applicable, informational := countActions(r.Actions)
		switch {
		case r.ListErr != nil:
			fmt.Printf("%-*s  ⚠️  無法確認\n", width, label)
		case applicable > 0:
			fmt.Printf("%-*s  📝 待重置\n", width, label)
		case informational > 0:
			fmt.Printf("%-*s  ℹ️  僅供參考（複本，等主庫修正後會自動同步；複本本身不稽核）\n", width, label)
		default:
			fmt.Printf("%-*s  ✅ 正常\n", width, label)
		}
	}
}

func printAuditAttentionSummary(results []admin.InstanceAuditRules) {
	type issue struct {
		instance string
		lines    []string
	}
	var issues []issue

	for _, r := range results {
		label := fmt.Sprintf("%s / %s", r.Instance.Project, r.Instance.Name)
		var lines []string
		if r.ListErr != nil {
			lines = append(lines, fmt.Sprintf("⚠️  無法確認：%v", r.ListErr))
		}
		for _, a := range r.Actions {
			if a.Applicable {
				lines = append(lines, "📝 "+a.Description)
			}
		}
		if len(lines) > 0 {
			issues = append(issues, issue{instance: label, lines: lines})
		}
	}

	if len(issues) == 0 {
		fmt.Println("=== 需要注意 ===")
		fmt.Println("沒有發現異常，所有查得到的實例都符合預期。")
		fmt.Println()
		return
	}

	fmt.Println("=== 需要注意 ===")
	for _, is := range issues {
		fmt.Println(is.instance)
		for _, line := range is.lines {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()
}

func printAuditApplyReport(results []admin.InstanceAuditRules) {
	currentProject := ""
	for _, r := range results {
		applicable, _ := countActions(r.Actions)
		if applicable == 0 {
			continue
		}
		if r.Instance.Project != currentProject {
			if currentProject != "" {
				fmt.Println()
			}
			fmt.Printf("%s (%s)\n", r.Instance.Project, r.Instance.Region)
			currentProject = r.Instance.Project
		}
		fmt.Printf("  %s\n", r.Instance.Name)
		for _, a := range r.Actions {
			if !a.Applicable {
				continue
			}
			if a.Err != nil {
				fmt.Printf("    ❌ %s：%v\n", a.Description, a.Err)
			} else {
				fmt.Printf("    ✅ %s\n", a.Description)
			}
		}
	}
}

func runInteractive(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
