// Package cli implements the ShipCheck command line interface:
// init, verify, doctor, list-checks, version.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/internal/engine"
	"github.com/mrpythonist/ship-check/internal/registry"
	"github.com/mrpythonist/ship-check/internal/reporting"

	"github.com/mrpythonist/ship-check/internal/checks/database"
	"github.com/mrpythonist/ship-check/internal/checks/docker"
	httpcheck "github.com/mrpythonist/ship-check/internal/checks/http"
	redischeck "github.com/mrpythonist/ship-check/internal/checks/redis"
	systemcheck "github.com/mrpythonist/ship-check/internal/checks/system"
	"github.com/mrpythonist/ship-check/internal/checks/workers"
)

// Version is set by the release build (ldflags); defaults to dev.
var Version = "v0.1.0"

// DefaultRegistry wires every built-in check category.
func DefaultRegistry() *registry.Registry {
	r := registry.New()
	_ = r.Register("http", registry.Meta{Name: "HTTP Health", Description: "Endpoint status, response time, body validation, TLS expiry."}, httpcheck.Factory)
	_ = r.Register("database", registry.Meta{Name: "Database Health", Description: "PostgreSQL / MySQL connectivity and read-only query."}, database.Factory)
	_ = r.Register("redis", registry.Meta{Name: "Redis Health", Description: "PING plus namespaced temporary read/write round-trip."}, redischeck.Factory)
	_ = r.Register("docker", registry.Meta{Name: "Docker Health", Description: "Daemon, required containers, health status, restart count."}, dockercheck.Factory)
	_ = r.Register("workers", registry.Meta{Name: "Worker Health", Description: "Worker processes, heartbeat freshness, queue backlog."}, workers.Factory)
	_ = r.Register("system", registry.Meta{Name: "System Health", Description: "Disk, memory, CPU load, required processes."}, systemcheck.Factory)
	return r
}

// Run is the CLI entry point. It returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return engine.ExitConfig
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "shipcheck %s\n", Version)
		return engine.ExitPass
	case "help", "--help", "-h":
		usage(stdout)
		return engine.ExitPass
	case "init":
		return cmdInit(args[1:], stdout, stderr)
	case "verify":
		return cmdVerify(args[1:], stdout, stderr)
	case "doctor":
		return cmdDoctor(args[1:], stdout, stderr)
	case "list-checks":
		return cmdListChecks(stdout)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		usage(stderr)
		return engine.ExitConfig
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `ShipCheck - Don't trust a successful deployment. Verify the release.

Usage:
  shipcheck init [--config shipcheck.yml] [--force]
  shipcheck verify [--config shipcheck.yml] [--json] [--verbose] [--fail-on-warn]
  shipcheck doctor [--config shipcheck.yml]
  shipcheck list-checks
  shipcheck version

Exit codes:
  0  release verified (warnings only, unless --fail-on-warn)
  1  release FAILED (a critical check failed)
  2  configuration / usage error
  3  internal execution error
`)
}

func findConfig(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	for _, name := range []string{"shipcheck.yml", "shipcheck.yaml", ".shipcheck.yml"} {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return "shipcheck.yml"
}

func cmdInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "shipcheck.yml", "path to write the config to")
	force := fs.Bool("force", false, "overwrite an existing config")
	if err := fs.Parse(args); err != nil {
		return engine.ExitConfig
	}
	if _, err := os.Stat(*cfgPath); err == nil && !*force {
		fmt.Fprintf(stderr, "%s already exists. Use --force to overwrite it.\n", *cfgPath)
		return engine.ExitConfig
	}
	if err := os.WriteFile(*cfgPath, []byte(exampleConfig), 0644); err != nil {
		fmt.Fprintf(stderr, "could not write %s: %v\n", *cfgPath, err)
		return engine.ExitError
	}
	fmt.Fprintf(stdout, "Created %s\nEdit the endpoints and credentials (use ${VAR} for secrets), then run:\n  shipcheck verify\n", *cfgPath)
	return engine.ExitPass
}

func cmdVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "path to shipcheck.yml")
	asJSON := fs.Bool("json", false, "machine-readable JSON output")
	verbose := fs.Bool("verbose", false, "show a message for every check")
	failOnWarn := fs.Bool("fail-on-warn", false, "exit 1 when any check warns")
	if err := fs.Parse(args); err != nil {
		return engine.ExitConfig
	}
	path := findConfig(*cfgPath)
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "configuration error: %v\n", err)
		return engine.ExitConfig
	}
	if cfg.Reporting.Format == "json" {
		*asJSON = true
	}
	if cfg.Reporting.Verbose {
		*verbose = true
	}
	eng := &engine.Engine{Reg: DefaultRegistry(), FailOnWarn: *failOnWarn}
	rep, err := eng.Run(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(stderr, "verification could not run: %v\n", err)
		return engine.ExitError
	}
	format := "terminal"
	if *asJSON {
		format = "json"
	}
	if err := reporting.Render(stdout, rep, format, *verbose); err != nil {
		fmt.Fprintf(stderr, "could not render report: %v\n", err)
		return engine.ExitError
	}
	return rep.ExitCode
}

func cmdDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "path to shipcheck.yml")
	if err := fs.Parse(args); err != nil {
		return engine.ExitConfig
	}
	path := findConfig(*cfgPath)
	fmt.Fprintf(stdout, "ShipCheck %s\nConfig: %s\n", Version, path)
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "FAIL config: %v\n", err)
		fmt.Fprintln(stdout, "\nRun `shipcheck init` to create a starter config.")
		return engine.ExitConfig
	}
	fmt.Fprintf(stdout, "PASS config is valid (project %s, environment %s)\n", cfg.Project.Name, cfg.Project.Environment)
	fmt.Fprintf(stdout, "Enabled checks: %s\n", strings.Join(cfg.EnabledCategories(), ", "))
	eng := &engine.Engine{Reg: DefaultRegistry()}
	list, err := eng.BuildChecks(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "FAIL checks: %v\n", err)
		return engine.ExitError
	}
	fmt.Fprintf(stdout, "Discovered %d check(s):\n", len(list))
	for _, c := range list {
		fmt.Fprintf(stdout, "  - [%s] %s (%s, %s)\n", c.Category(), c.Name(), c.ID(), c.Severity())
	}
	// Preflight hints, read-only.
	if cfg.Checks.Docker.Enabled {
		if _, err := os.Stat(cfg.Checks.Docker.Socket); err != nil {
			fmt.Fprintf(stdout, "WARN Docker socket %s is not reachable: %v\n", cfg.Checks.Docker.Socket, err)
		} else {
			fmt.Fprintf(stdout, "PASS Docker socket %s exists\n", cfg.Checks.Docker.Socket)
		}
	}
	if cfg.Checks.Workers.Enabled && cfg.Checks.Workers.HeartbeatFile != "" {
		if _, err := os.Stat(cfg.Checks.Workers.HeartbeatFile); err != nil {
			fmt.Fprintf(stdout, "WARN heartbeat file %s is missing: %v\n", cfg.Checks.Workers.HeartbeatFile, err)
		}
	}
	fmt.Fprintln(stdout, "Doctor finished. Run `shipcheck verify` for a full verification.")
	return engine.ExitPass
}

func cmdListChecks(stdout io.Writer) int {
	r := DefaultRegistry()
	fmt.Fprintln(stdout, "Available verification checks:")
	for _, cat := range r.Categories() {
		m, _ := r.Describe(cat)
		fmt.Fprintf(stdout, "  %s - %s\n      %s\n", cat, m.Name, m.Description)
	}
	fmt.Fprintln(stdout, "Enable a category in shipcheck.yml under checks: to use it.")
	return engine.ExitPass
}

const exampleConfig = `version: 1

project:
  name: my-saas
  environment: production

verification:
  timeout: 30s
  parallel: true
  fail_fast: false

checks:
  http:
    enabled: true
    endpoints:
      - name: API Health
        url: https://example.com/api/health
        expected_status: 200
        timeout: 5s
        max_response_time_ms: 2000
        body_contains: "ok"
        check_tls: true
        tls_min_days: 14

  database:
    enabled: false
    driver: postgres
    # Prefer environment variables for secrets; never commit passwords.
    dsn: ${DATABASE_URL}
    timeout: 5s

  redis:
    enabled: false
    address: 127.0.0.1:6379
    timeout: 5s

  docker:
    enabled: false
    socket: /var/run/docker.sock
    require_health: true
    containers:
      - name: my-saas-app
        max_restarts: 3

  workers:
    enabled: false
    processes:
      - name: Laravel Queue Worker
        pattern: "queue:work"
        min_count: 1
    heartbeat_file: /var/www/my-saas/storage/worker-heartbeat.txt
    heartbeat_max_age: 5m
    queues:
      - name: default
        address: 127.0.0.1:6379
        key: "queues:default"
        max_length: 100

  system:
    enabled: false
    disk_path: /
    disk_max_usage_percent: 90
    memory_max_usage_percent: 90
    max_load_1m: 4.0

reporting:
  format: terminal
  verbose: false
`

// Ensure filepath import is used on all platforms (doctor resolves paths).
var _ = filepath.Join
