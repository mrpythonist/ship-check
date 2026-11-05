// Package config loads, validates and defaults shipcheck.yml.
// Secrets are never stored in committed config: DSNs and passwords are
// expected to come from environment variables, expanded as ${VAR}.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root of shipcheck.yml (version 1).
type Config struct {
	Version      int                `yaml:"version"`
	Project      ProjectConfig      `yaml:"project"`
	Verification VerificationConfig `yaml:"verification"`
	Checks       ChecksConfig       `yaml:"checks"`
	Reporting    ReportingConfig    `yaml:"reporting"`
}

type ProjectConfig struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
}

type VerificationConfig struct {
	Timeout  time.Duration `yaml:"timeout"`
	Parallel bool          `yaml:"parallel"`
	FailFast bool          `yaml:"fail_fast"`
}

type ReportingConfig struct {
	Format  string `yaml:"format"` // terminal | json
	Verbose bool   `yaml:"verbose"`
}

type ChecksConfig struct {
	HTTP     HTTPConfig     `yaml:"http"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Docker   DockerConfig   `yaml:"docker"`
	Workers  WorkersConfig  `yaml:"workers"`
	System   SystemConfig   `yaml:"system"`
}

type HTTPEndpoint struct {
	Name              string        `yaml:"name"`
	URL               string        `yaml:"url"`
	ExpectedStatus    int           `yaml:"expected_status"`
	Timeout           time.Duration `yaml:"timeout"`
	MaxResponseTimeMs int           `yaml:"max_response_time_ms"`
	BodyContains      string        `yaml:"body_contains"`
	BodyNotContains   string        `yaml:"body_not_contains"`
	CheckTLS          bool          `yaml:"check_tls"`
	TLSMinDays        int           `yaml:"tls_min_days"`
	Severity          string        `yaml:"severity"`
}

type HTTPConfig struct {
	Enabled   bool           `yaml:"enabled"`
	Endpoints []HTTPEndpoint `yaml:"endpoints"`
}

type DatabaseConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Driver   string        `yaml:"driver"` // postgres | mysql
	DSN      string        `yaml:"dsn"`
	Host     string        `yaml:"host"`
	Port     int           `yaml:"port"`
	User     string        `yaml:"user"`
	Password string        `yaml:"password"`
	Database string        `yaml:"database"`
	SSLMode  string        `yaml:"sslmode"`
	Timeout  time.Duration `yaml:"timeout"`
	Query    string        `yaml:"query"`
	Severity string        `yaml:"severity"`
}

type RedisConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Address  string        `yaml:"address"`
	Password string        `yaml:"password"`
	DB       int           `yaml:"db"`
	Timeout  time.Duration `yaml:"timeout"`
	Severity string        `yaml:"severity"`
}

type DockerContainer struct {
	Name        string `yaml:"name"`
	MaxRestarts int    `yaml:"max_restarts"`
}

type DockerConfig struct {
	Enabled       bool              `yaml:"enabled"`
	Socket        string            `yaml:"socket"`
	Containers    []DockerContainer `yaml:"containers"`
	RequireHealth bool              `yaml:"require_health"`
	Timeout       time.Duration     `yaml:"timeout"`
	Severity      string            `yaml:"severity"`
}

type WorkerProcess struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	MinCount int    `yaml:"min_count"`
}

type WorkerQueue struct {
	Name      string `yaml:"name"`
	Address   string `yaml:"address"` // redis address, defaults to redis.address
	Key       string `yaml:"key"`
	MaxLength int    `yaml:"max_length"`
}

type WorkersConfig struct {
	Enabled         bool            `yaml:"enabled"`
	Processes       []WorkerProcess `yaml:"processes"`
	HeartbeatFile   string          `yaml:"heartbeat_file"`
	HeartbeatMaxAge time.Duration   `yaml:"heartbeat_max_age"`
	Queues          []WorkerQueue   `yaml:"queues"`
	Timeout         time.Duration   `yaml:"timeout"`
	Severity        string          `yaml:"severity"`
}

type SystemConfig struct {
	Enabled           bool            `yaml:"enabled"`
	DiskPath          string          `yaml:"disk_path"`
	DiskMaxUsagePct   float64         `yaml:"disk_max_usage_percent"`
	MemoryMaxUsagePct float64         `yaml:"memory_max_usage_percent"`
	MaxLoad1m         float64         `yaml:"max_load_1m"`
	Processes         []WorkerProcess `yaml:"processes"`
	Timeout           time.Duration   `yaml:"timeout"`
	Severity          string          `yaml:"severity"`
}

// Default returns a minimal, safe configuration: HTTP health check only,
// everything else disabled until the user opts in.
func Default() *Config {
	return &Config{
		Version:      1,
		Project:      ProjectConfig{Name: "my-app", Environment: "production"},
		Verification: VerificationConfig{Timeout: 30 * time.Second, Parallel: true},
		Checks: ChecksConfig{
			HTTP: HTTPConfig{Enabled: true, Endpoints: []HTTPEndpoint{
				{Name: "API Health", URL: "http://localhost:8080/health", ExpectedStatus: 200, Timeout: 5 * time.Second, MaxResponseTimeMs: 2000},
			}},
		},
		Reporting: ReportingConfig{Format: "terminal"},
	}
}

// Load reads path, expands ${VAR} environment references in the raw file,
// parses YAML and validates. Validation errors name the offending field.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	expanded := os.ExpandEnv(string(raw))
	cfg := Default()
	// Unmarshal over defaults so omitted fields keep sensible values.
	// Note: Default() sets one HTTP endpoint; if the file defines its own
	// endpoints, yaml replaces the slice, which is the desired behaviour.
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Verification.Timeout <= 0 {
		c.Verification.Timeout = 30 * time.Second
	}
	if c.Checks.HTTP.Enabled {
		for i := range c.Checks.HTTP.Endpoints {
			e := &c.Checks.HTTP.Endpoints[i]
			if e.ExpectedStatus == 0 {
				e.ExpectedStatus = 200
			}
			if e.Timeout <= 0 {
				e.Timeout = 5 * time.Second
			}
			if e.Severity == "" {
				e.Severity = "critical"
			}
		}
	}
	if c.Checks.Database.Query == "" {
		c.Checks.Database.Query = "SELECT 1"
	}
	if c.Checks.Database.Timeout <= 0 {
		c.Checks.Database.Timeout = 5 * time.Second
	}
	if c.Checks.Redis.Timeout <= 0 {
		c.Checks.Redis.Timeout = 5 * time.Second
	}
	if c.Checks.Docker.Socket == "" {
		c.Checks.Docker.Socket = "/var/run/docker.sock"
	}
	if c.Checks.Docker.Timeout <= 0 {
		c.Checks.Docker.Timeout = 5 * time.Second
	}
	if c.Checks.Workers.Timeout <= 0 {
		c.Checks.Workers.Timeout = 5 * time.Second
	}
	if c.Checks.System.DiskPath == "" {
		c.Checks.System.DiskPath = "/"
	}
	if c.Checks.System.DiskMaxUsagePct == 0 {
		c.Checks.System.DiskMaxUsagePct = 90
	}
	if c.Checks.System.MemoryMaxUsagePct == 0 {
		c.Checks.System.MemoryMaxUsagePct = 90
	}
	if c.Checks.System.MaxLoad1m == 0 {
		c.Checks.System.MaxLoad1m = 0 // 0 = disabled
	}
	if c.Checks.System.Timeout <= 0 {
		c.Checks.System.Timeout = 5 * time.Second
	}
}

// Validate returns a joined error naming every invalid field found.
func (c *Config) Validate() error {
	var errs []string
	if c.Version != 1 {
		errs = append(errs, "version: must be 1")
	}
	if strings.TrimSpace(c.Project.Name) == "" {
		errs = append(errs, "project.name: required")
	}
	if strings.TrimSpace(c.Project.Environment) == "" {
		errs = append(errs, "project.environment: required")
	}
	if c.Verification.Timeout < 0 {
		errs = append(errs, "verification.timeout: must be >= 0")
	}
	if c.Reporting.Format != "" && c.Reporting.Format != "terminal" && c.Reporting.Format != "json" {
		errs = append(errs, "reporting.format: must be 'terminal' or 'json'")
	}
	if c.Checks.HTTP.Enabled {
		if len(c.Checks.HTTP.Endpoints) == 0 {
			errs = append(errs, "checks.http.endpoints: at least one endpoint is required when http is enabled")
		}
		for i, e := range c.Checks.HTTP.Endpoints {
			p := fmt.Sprintf("checks.http.endpoints[%d]", i)
			if strings.TrimSpace(e.Name) == "" {
				errs = append(errs, p+".name: required")
			}
			u, err := url.Parse(e.URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				errs = append(errs, p+".url: must be a valid http(s) URL")
			}
			if e.ExpectedStatus != 0 && (e.ExpectedStatus < 100 || e.ExpectedStatus > 599) {
				errs = append(errs, p+".expected_status: must be 100-599")
			}
			if e.Timeout < 0 {
				errs = append(errs, p+".timeout: must be >= 0")
			}
			if e.TLSMinDays < 0 {
				errs = append(errs, p+".tls_min_days: must be >= 0")
			}
			if e.Severity != "" && e.Severity != "critical" && e.Severity != "warning" {
				errs = append(errs, p+".severity: must be 'critical' or 'warning'")
			}
		}
	}
	if c.Checks.Database.Enabled {
		d := c.Checks.Database
		if d.Driver != "postgres" && d.Driver != "mysql" {
			errs = append(errs, "checks.database.driver: must be 'postgres' or 'mysql'")
		}
		if d.DSN == "" && d.Host == "" {
			errs = append(errs, "checks.database: set dsn or host when database is enabled")
		}
		if d.Query != "" && strings.Contains(strings.ToUpper(d.Query), "DROP ") {
			errs = append(errs, "checks.database.query: destructive queries are not allowed")
		}
	}
	if c.Checks.Redis.Enabled && strings.TrimSpace(c.Checks.Redis.Address) == "" {
		errs = append(errs, "checks.redis.address: required when redis is enabled")
	}
	if c.Checks.Docker.Enabled {
		for i, cont := range c.Checks.Docker.Containers {
			if strings.TrimSpace(cont.Name) == "" {
				errs = append(errs, fmt.Sprintf("checks.docker.containers[%d].name: required", i))
			}
			if cont.MaxRestarts < 0 {
				errs = append(errs, fmt.Sprintf("checks.docker.containers[%d].max_restarts: must be >= 0", i))
			}
		}
	}
	if c.Checks.Workers.Enabled {
		for i, p := range c.Checks.Workers.Processes {
			if strings.TrimSpace(p.Pattern) == "" {
				errs = append(errs, fmt.Sprintf("checks.workers.processes[%d].pattern: required", i))
			}
			if p.MinCount < 0 {
				errs = append(errs, fmt.Sprintf("checks.workers.processes[%d].min_count: must be >= 0", i))
			}
		}
		for i, q := range c.Checks.Workers.Queues {
			if strings.TrimSpace(q.Key) == "" {
				errs = append(errs, fmt.Sprintf("checks.workers.queues[%d].key: required", i))
			}
			if q.MaxLength < 0 {
				errs = append(errs, fmt.Sprintf("checks.workers.queues[%d].max_length: must be >= 0", i))
			}
		}
		if c.Checks.Workers.HeartbeatMaxAge < 0 {
			errs = append(errs, "checks.workers.heartbeat_max_age: must be >= 0")
		}
	}
	if c.Checks.System.Enabled {
		if c.Checks.System.DiskMaxUsagePct < 0 || c.Checks.System.DiskMaxUsagePct > 100 {
			errs = append(errs, "checks.system.disk_max_usage_percent: must be 0-100")
		}
		if c.Checks.System.MemoryMaxUsagePct < 0 || c.Checks.System.MemoryMaxUsagePct > 100 {
			errs = append(errs, "checks.system.memory_max_usage_percent: must be 0-100")
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// EnabledCategories returns the categories with enabled: true, in a
// stable order, so the engine and doctor can report what will run.
func (c *Config) EnabledCategories() []string {
	var out []string
	if c.Checks.HTTP.Enabled {
		out = append(out, "http")
	}
	if c.Checks.Database.Enabled {
		out = append(out, "database")
	}
	if c.Checks.Redis.Enabled {
		out = append(out, "redis")
	}
	if c.Checks.Docker.Enabled {
		out = append(out, "docker")
	}
	if c.Checks.Workers.Enabled {
		out = append(out, "workers")
	}
	if c.Checks.System.Enabled {
		out = append(out, "system")
	}
	return out
}
