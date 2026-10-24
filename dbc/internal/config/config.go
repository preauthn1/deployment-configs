// Package config reads instances.yaml, the list of Cloud SQL instances dbc
// knows how to proxy to.
package config

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Instance describes one instance entry from instances.yaml — normally a
// Cloud SQL instance, or a bare-metal (self-hosted, outside GCP) MySQL
// server if Host is set.
type Instance struct {
	Port    int
	Role    string // "replica" or "primary"
	Project string
	Region  string
	Name    string

	// Host, if set, marks this as a bare-metal MySQL server (a plain
	// "host:port" TCP address) rather than a Cloud SQL instance: dbc just
	// TCP-relays to it instead of dialing through cloudsqlconn's IAM-auth
	// tunnel. Project still names a real GCP project — that's only used as
	// the query-audit log destination (see internal/proxy/queryaudit.go),
	// bare-metal servers have no Cloud SQL Admin API/IAM auth equivalent so
	// `dbc admin grants`/`audit-rules`/`service-accounts` skip them.
	Host string

	// Gateway routes this instance through dbc-gateway instead of dialing a
	// database address directly. The server owns the authoritative audit log.
	Gateway *Gateway

	// NoReplica marks a primary that has no replica counterpart (e.g. the
	// chat DBs), so it's the only way to reach that database at all. `dbc
	// start`'s default (replica-only) set includes these, and the "主庫連線"
	// warning is skipped for them since there's no replica to redirect to.
	NoReplica bool

	// AlwaysStart marks a primary that DOES have a replica counterpart but
	// should still be included in `dbc start`'s default set anyway — unlike
	// NoReplica, this doesn't suppress the "主庫連線" warning (there's a
	// real replica alternative for routine reads; this instance is just
	// deliberately kept connected alongside it).
	AlwaysStart bool

	// DBProdUserCloudSQLSuperuser marks a per-instance exception: on this
	// instance, `dbc admin grants` expects db-prod-user to ALSO hold the
	// cloudsqlsuperuser Cloud SQL role, on top of its normal SELECT-only
	// baseline (default false: db-prod-user having cloudsqlsuperuser is
	// treated as an anomaly and revoked by --apply).
	//
	// GRANTs replicate from primary to replica automatically, but this flag
	// does not — a primary and its replica are separate instances.yaml
	// entries with no code-level link between them. Set it explicitly on
	// BOTH, or two things go wrong: the replica's (accurate, replicated)
	// state gets reported as a spurious anomaly, and if that replica is
	// ever promoted to primary, --apply would actually revoke a role it
	// should keep.
	DBProdUserCloudSQLSuperuser bool
}

// Gateway contains the public endpoint and an allowlisted target ID. It
// deliberately contains no backend database address.
type Gateway struct {
	Endpoint   string
	ServerName string
	Audience   string
	Target     string
}

// IsBareMetal reports whether this is a self-hosted MySQL server (Host is
// set) rather than a Cloud SQL instance.
func (i Instance) IsBareMetal() bool {
	return i.Host != "" || i.Gateway != nil
}

// UsesGateway reports whether dbc must authenticate to dbc-gateway.
func (i Instance) UsesGateway() bool { return i.Gateway != nil }

// ConnName returns the address dbc dials to reach this instance: Host
// verbatim for a bare-metal server, otherwise the Cloud SQL instance
// connection name in the "project:region:instance" form cloudsqlconn
// expects. Also used as the display/label name (`dbc list`, `dbc start`'s
// banner, the query-audit log's connName field).
func (i Instance) ConnName() string {
	if i.Gateway != nil {
		return fmt.Sprintf("%s/%s", i.Gateway.Endpoint, i.Gateway.Target)
	}
	if i.IsBareMetal() {
		return i.Host
	}
	return fmt.Sprintf("%s:%s:%s", i.Project, i.Region, i.Name)
}

// Key uniquely identifies an instance. Instance names alone are not unique
// (e.g. "chatrrom-online" appears under three different projects), so the
// key is always project:instance.
func (i Instance) Key() string {
	return fmt.Sprintf("%s:%s", i.Project, i.Name)
}

const (
	// DefaultPath is where dbc looks for instances.yaml relative to the
	// binary when no override is given.
	DefaultPath = "instances.yaml"

	// EnvPath overrides the instances.yaml location.
	EnvPath = "DBC_INSTANCES_CONF"
)

// instanceEntry is one instance under a project in instances.yaml.
type instanceEntry struct {
	Port      int    `yaml:"port"`
	Role      string `yaml:"role"`
	NoReplica bool   `yaml:"no_replica"`
	Name      string `yaml:"instance"`
	// Host marks this as a bare-metal (non-Cloud SQL) MySQL server — see
	// Instance.Host.
	Host string `yaml:"host"`
	// Gateway and Host are mutually exclusive so there is no silent direct fallback.
	Gateway *gatewayEntry `yaml:"gateway"`
	// DBProdUserCloudSQLSuperuser — see Instance.DBProdUserCloudSQLSuperuser.
	DBProdUserCloudSQLSuperuser bool `yaml:"db_prod_user_cloudsqlsuperuser"`
	// AlwaysStart — see Instance.AlwaysStart.
	AlwaysStart bool `yaml:"always_start"`
}

type gatewayEntry struct {
	Endpoint   string `yaml:"endpoint"`
	ServerName string `yaml:"server_name"`
	Audience   string `yaml:"audience"`
	Target     string `yaml:"target"`
}

// projectEntry is one project's block in instances.yaml: a shared region
// plus its instances. Every instance under a project lives in that same
// project's region, so region is declared once per project instead of once
// per instance.
type projectEntry struct {
	Region    string          `yaml:"region"`
	Instances []instanceEntry `yaml:"instances"`
}

// file is the top-level shape of instances.yaml: a map of project name to
// projectEntry, grouped for readability when editing.
type file struct {
	Projects map[string]projectEntry `yaml:"projects"`
}

// Locate resolves the instances.yaml path to use: $DBC_INSTANCES_CONF if
// set, otherwise instances.yaml next to the running binary, otherwise
// instances.yaml in the current directory.
func Locate() (string, error) {
	if p := os.Getenv(EnvPath); p != "" {
		return p, nil
	}

	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		candidate := filepath.Join(filepath.Dir(exe), DefaultPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if _, err := os.Stat(DefaultPath); err == nil {
		return DefaultPath, nil
	}

	return "", fmt.Errorf("找不到 instances.yaml，請確認它與 dbc 執行檔放在同一目錄，或設定 %s 指向它", EnvPath)
}

// Load parses an instances.yaml file.
func Load(path string) ([]Instance, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("開啟 %s 失敗: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads instances.yaml content: a top-level "projects" map, each
// project declaring a shared region and a list of instances
// (port/role/instance/no_replica fields).
func Parse(r io.Reader) ([]Instance, error) {
	var doc file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("解析 instances.yaml 失敗: %w", err)
	}

	seenPort := map[int]string{}
	seenKey := map[string]bool{}
	var instances []Instance

	// Sort project names so validation errors are stable/reproducible
	// instead of depending on Go's randomized map iteration order.
	projectNames := make([]string, 0, len(doc.Projects))
	for name := range doc.Projects {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	for _, project := range projectNames {
		pe := doc.Projects[project]
		if pe.Region == "" {
			return nil, fmt.Errorf("instances.yaml 專案 %s 缺少 region", project)
		}

		for _, ie := range pe.Instances {
			inst := Instance{
				Port:                        ie.Port,
				Role:                        ie.Role,
				Project:                     project,
				Region:                      pe.Region,
				Name:                        ie.Name,
				Host:                        ie.Host,
				NoReplica:                   ie.NoReplica,
				AlwaysStart:                 ie.AlwaysStart,
				DBProdUserCloudSQLSuperuser: ie.DBProdUserCloudSQLSuperuser,
			}
			if ie.Gateway != nil {
				inst.Gateway = &Gateway{
					Endpoint: ie.Gateway.Endpoint, ServerName: ie.Gateway.ServerName,
					Audience: ie.Gateway.Audience, Target: ie.Gateway.Target,
				}
			}

			if inst.Port == 0 {
				return nil, fmt.Errorf("instances.yaml 專案 %s 有一筆缺少 port", project)
			}
			if inst.Role != "replica" && inst.Role != "primary" {
				return nil, fmt.Errorf("instances.yaml port %d 的 role 必須是 replica 或 primary，得到 %q", inst.Port, inst.Role)
			}
			if inst.Name == "" {
				return nil, fmt.Errorf("instances.yaml port %d 缺少 instance 欄位", inst.Port)
			}
			if inst.NoReplica && inst.Role != "primary" {
				return nil, fmt.Errorf("instances.yaml port %d 是 replica，不應該設定 no_replica", inst.Port)
			}
			if inst.Host != "" && inst.Gateway != nil {
				return nil, fmt.Errorf("instances.yaml port %d 的 host 與 gateway 不可同時設定", inst.Port)
			}
			if inst.Gateway != nil {
				if _, _, err := net.SplitHostPort(inst.Gateway.Endpoint); err != nil {
					return nil, fmt.Errorf("instances.yaml port %d 的 gateway.endpoint 必須是 host:port: %w", inst.Port, err)
				}
				if strings.TrimSpace(inst.Gateway.ServerName) == "" || net.ParseIP(inst.Gateway.ServerName) != nil {
					return nil, fmt.Errorf("instances.yaml port %d 的 gateway.server_name 必須是 TLS 憑證中的 DNS 名稱", inst.Port)
				}
				if strings.TrimSpace(inst.Gateway.Audience) == "" || strings.TrimSpace(inst.Gateway.Target) == "" {
					return nil, fmt.Errorf("instances.yaml port %d 的 gateway.audience 與 gateway.target 不可為空", inst.Port)
				}
			}

			if prev, ok := seenPort[inst.Port]; ok {
				return nil, fmt.Errorf("instances.yaml port %d 重複（已用於 %s）", inst.Port, prev)
			}
			seenPort[inst.Port] = inst.ConnName()

			if seenKey[inst.Key()] {
				return nil, fmt.Errorf("instances.yaml %s 重複", inst.Key())
			}
			seenKey[inst.Key()] = true

			instances = append(instances, inst)
		}
	}

	sort.Slice(instances, func(i, j int) bool { return instances[i].Port < instances[j].Port })

	return instances, nil
}

// ByPort returns the instance listening on the given port, if any.
func ByPort(instances []Instance, port int) (Instance, bool) {
	for _, i := range instances {
		if i.Port == port {
			return i, true
		}
	}
	return Instance{}, false
}

// Filter returns instances matching role ("" means any) and, if port != 0,
// only the single instance on that port.
func Filter(instances []Instance, role string, port int) ([]Instance, error) {
	if port != 0 {
		inst, ok := ByPort(instances, port)
		if !ok {
			return nil, fmt.Errorf("找不到 port %d，請用 `dbc list` 查看可用的 port", port)
		}
		return []Instance{inst}, nil
	}

	if role == "" {
		return instances, nil
	}

	var out []Instance
	for _, i := range instances {
		if i.Role == role {
			out = append(out, i)
		}
	}
	return out, nil
}

// DailySet returns the instances covered by `dbc start`'s default (no
// --primary/--all/--port) mode: every replica, any primary that has no
// replica counterpart at all (NoReplica, since that's the only way to reach
// that database), and any primary explicitly opted in despite having a
// replica (AlwaysStart).
func DailySet(instances []Instance) []Instance {
	var out []Instance
	for _, i := range instances {
		if i.Role == "replica" || i.NoReplica || i.AlwaysStart {
			out = append(out, i)
		}
	}
	return out
}

// HasWarnablePrimary reports whether the set contains a primary that has a
// replica alternative — the case where the "主庫連線" warning is useful
// because the user could have used the replica instead.
func HasWarnablePrimary(instances []Instance) bool {
	for _, i := range instances {
		if i.Role == "primary" && !i.NoReplica {
			return true
		}
	}
	return false
}

// NoReplicaCount returns how many no-replica primaries are in the set, so
// callers can tell the user why a "primary" shows up in a set that's
// otherwise replica-only. Listing them by name isn't useful here — instance
// names repeat across projects (e.g. "xsgcasino-online" exists under
// several projects) — so callers should point the user at `dbc list`'s 備註
// column for the per-instance detail instead.
func NoReplicaCount(instances []Instance) int {
	n := 0
	for _, i := range instances {
		if i.Role == "primary" && i.NoReplica {
			n++
		}
	}
	return n
}
