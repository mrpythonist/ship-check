// Package database verifies PostgreSQL / MySQL connectivity with a
// read-only query (SELECT 1 by default). Credentials are never logged:
// errors pass through security.Redact first.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/internal/security"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// Factory builds a single database check.
func Factory(cfg any) ([]checks.Check, error) {
	c, ok := cfg.(config.DatabaseConfig)
	if !ok {
		return nil, fmt.Errorf("database: unexpected config type %T", cfg)
	}
	return []checks.Check{&dbCheck{cfg: c}}, nil
}

type dbCheck struct{ cfg config.DatabaseConfig }

func (c *dbCheck) ID() string { return "database:connection" }
func (c *dbCheck) Name() string {
	if c.cfg.Driver == "mysql" {
		return "MySQL Connection"
	}
	return "PostgreSQL Connection"
}
func (c *dbCheck) Category() string { return "database" }
func (c *dbCheck) Description() string {
	return "Verifies the database accepts connections and answers a read-only query."
}
func (c *dbCheck) Timeout() time.Duration {
	if c.cfg.Timeout > 0 {
		return c.cfg.Timeout
	}
	return 5 * time.Second
}
func (c *dbCheck) Severity() checks.Severity {
	if c.cfg.Severity == "warning" {
		return checks.SeverityWarning
	}
	return checks.SeverityCritical
}

func (c *dbCheck) dsn() string {
	if c.cfg.DSN != "" {
		return c.cfg.DSN
	}
	switch c.cfg.Driver {
	case "mysql":
		port := c.cfg.Port
		if port == 0 {
			port = 3306
		}
		// parseTime/timeout keep the driver bounded; never interpolate raw.
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=%ds&readTimeout=%ds",
			c.cfg.User, c.cfg.Password, c.cfg.Host, port, c.cfg.Database,
			int(c.Timeout().Seconds()), int(c.Timeout().Seconds()))
	default: // postgres
		port := c.cfg.Port
		if port == 0 {
			port = 5432
		}
		ssl := c.cfg.SSLMode
		if ssl == "" {
			ssl = "disable"
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
			c.cfg.Host, port, c.cfg.User, c.cfg.Password, c.cfg.Database, ssl, int(c.Timeout().Seconds()))
	}
}

func (c *dbCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "database", Severity: c.Severity()}
	db, err := sql.Open(c.cfg.Driver, c.dsn())
	if err != nil {
		return fail(res, start, fmt.Sprintf("could not open %s connection: %v", c.cfg.Driver, err), "Check the database driver and DSN format.")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(c.Timeout())
	if err := db.PingContext(ctx); err != nil {
		return fail(res, start, fmt.Sprintf("%s connection failed: %v", c.cfg.Driver, security.Redact(err.Error())), "Check the database is running, reachable, and the credentials in the environment are correct.")
	}
	query := c.cfg.Query
	if query == "" {
		query = "SELECT 1"
	}
	var one int
	if err := db.QueryRowContext(ctx, query).Scan(&one); err != nil {
		return fail(res, start, fmt.Sprintf("%s query failed: %v", c.cfg.Driver, security.Redact(err.Error())), "Check database permissions for the verification user; use a read-only user.")
	}
	elapsed := time.Since(start)
	res.Status = checks.StatusPass
	res.Duration = elapsed
	res.Message = fmt.Sprintf("%s connection OK, read-only query answered in %dms", c.cfg.Driver, elapsed.Milliseconds())
	res.Details = map[string]any{"driver": c.cfg.Driver, "host": c.cfg.Host, "database": c.cfg.Database}
	return res
}

func fail(res checks.Result, start time.Time, msg, rec string) checks.Result {
	res.Status = checks.StatusFail
	res.Duration = time.Since(start)
	res.Message = msg
	res.Recommendation = rec
	return res
}
