// Package gateway 實作 dbc-gateway server。
package gateway

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultListen       = ":8443"
	defaultHealthListen = ":8080"
	defaultAuthTimeout  = 10 * time.Second
	defaultDialTimeout  = 10 * time.Second
	defaultDrainTimeout = 70 * time.Minute
)

// Config 是 dbc-gateway 的 server-side 設定。backend address 只會從 Targets 取得，
// client 只能提交 target ID，不能指定任意網路位置。
type Config struct {
	Listen       string                  `yaml:"listen"`
	HealthListen string                  `yaml:"healthListen"`
	Audience     string                  `yaml:"audience"`
	TLS          TLSConfig               `yaml:"tls"`
	Targets      map[string]TargetConfig `yaml:"targets"`
	AuthTimeout  time.Duration           `yaml:"-"`
	DialTimeout  time.Duration           `yaml:"-"`
	DrainTimeout time.Duration           `yaml:"-"`
}

// TLSConfig 指向 cert-manager Secret 掛載的 PEM 檔案。
type TLSConfig struct {
	CertificateFile string `yaml:"certificateFile"`
	PrivateKeyFile  string `yaml:"privateKeyFile"`
}

// TargetConfig 是一個允許代理的 MySQL backend。
type TargetConfig struct {
	Address      string       `yaml:"address"`
	AuditProject string       `yaml:"auditProject"`
	Roles        []TargetRole `yaml:"roles"`
}

// TargetRole 把一組 Cloud Identity 群組對應到一組後端 MySQL 憑證——群組本身
// 決定了實際的資料庫權限（例如 db-prod-admin 對應一個有完整權限的帳號、
// db-prod-user 對應一個只有 SELECT 的帳號），不是由呼叫端自己提供的帳密
// 決定，見 mysqlauth.go 的套件說明。
//
// Roles 是有序清單：Authorizer 會依序找第一個成員資格符合的 role 並採用
// 它的憑證，找到就不再往後找。同一個人若符合多個 role，只會用最先符合
// 的那一個——權限高的 role 要排在前面（例如 db-prod-admin 要排在
// db-prod-user 之前），不然同時屬於兩個群組的人會被排在後面、權限較低
// 的 role 定住。
type TargetRole struct {
	Groups        []string `yaml:"groups"`
	MySQLUser     string   `yaml:"mysqlUser"`
	MySQLPassword string   `yaml:"mysqlPassword"`
}

type rawConfig struct {
	Listen       string                  `yaml:"listen"`
	HealthListen string                  `yaml:"healthListen"`
	Audience     string                  `yaml:"audience"`
	TLS          TLSConfig               `yaml:"tls"`
	Targets      map[string]TargetConfig `yaml:"targets"`
	AuthTimeout  string                  `yaml:"authTimeout"`
	DialTimeout  string                  `yaml:"dialTimeout"`
	DrainTimeout string                  `yaml:"drainTimeout"`
}

// ParseConfig 解析並嚴格驗證 gateway YAML。未知欄位視為錯誤，避免拼字錯誤被靜默忽略。
func ParseConfig(r io.Reader) (*Config, error) {
	var raw rawConfig
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析 dbc-gateway 設定失敗: %w", err)
	}

	cfg := &Config{
		Listen: raw.Listen, HealthListen: raw.HealthListen, Audience: raw.Audience,
		TLS: raw.TLS, Targets: raw.Targets,
		AuthTimeout: defaultAuthTimeout, DialTimeout: defaultDialTimeout, DrainTimeout: defaultDrainTimeout,
	}
	if cfg.Listen == "" {
		cfg.Listen = defaultListen
	}
	if cfg.HealthListen == "" {
		cfg.HealthListen = defaultHealthListen
	}

	var err error
	if cfg.AuthTimeout, err = parseDuration("authTimeout", raw.AuthTimeout, cfg.AuthTimeout); err != nil {
		return nil, err
	}
	if cfg.DialTimeout, err = parseDuration("dialTimeout", raw.DialTimeout, cfg.DialTimeout); err != nil {
		return nil, err
	}
	if cfg.DrainTimeout, err = parseDuration("drainTimeout", raw.DrainTimeout, cfg.DrainTimeout); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func parseDuration(field, raw string, fallback time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s 必須是正數 duration，得到 %q", field, raw)
	}
	return value, nil
}

// Validate 驗證所有會影響信任邊界的必要設定。
func (c *Config) Validate() error {
	if c.Audience == "" {
		return fmt.Errorf("audience 不可為空")
	}
	if c.TLS.CertificateFile == "" || c.TLS.PrivateKeyFile == "" {
		return fmt.Errorf("tls.certificateFile 與 tls.privateKeyFile 都必須設定")
	}
	if len(c.Targets) == 0 {
		return fmt.Errorf("至少必須設定一個 target")
	}
	for id, target := range c.Targets {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("target ID 不可為空")
		}
		if _, _, err := net.SplitHostPort(target.Address); err != nil {
			return fmt.Errorf("target %q 的 address 必須是 host:port: %w", id, err)
		}
		if target.AuditProject == "" {
			return fmt.Errorf("target %q 缺少 auditProject", id)
		}
		if len(target.Roles) == 0 {
			return fmt.Errorf("target %q 至少必須設定一個 role", id)
		}
		seenGroups := map[string]bool{}
		for i, role := range target.Roles {
			if len(role.Groups) == 0 {
				return fmt.Errorf("target %q 的第 %d 個 role 至少必須設定一個 group", id, i+1)
			}
			if role.MySQLUser == "" || role.MySQLPassword == "" {
				return fmt.Errorf("target %q 的第 %d 個 role 缺少 mysqlUser/mysqlPassword", id, i+1)
			}
			for _, group := range role.Groups {
				group = strings.TrimSpace(group)
				if group == "" {
					return fmt.Errorf("target %q 包含空白的 group", id)
				}
				// 同一個群組出現在超過一個 role 裡：其中一個 role 永遠選
				// 不到（Authorizer 只採用最先符合的那個），這是死設定，
				// 直接拒絕載入而不是讓它悄悄存在。
				if seenGroups[group] {
					return fmt.Errorf("target %q 的群組 %q 出現在超過一個 role 裡", id, group)
				}
				seenGroups[group] = true
			}
		}
	}
	return nil
}
