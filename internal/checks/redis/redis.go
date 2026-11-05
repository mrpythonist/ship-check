// Package redischeck verifies Redis with a raw RESP client (no extra
// dependency): PING, then SET/GET/DEL on a uniquely namespaced temporary
// key with a short TTL, so application keys are never touched.
package redischeck

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/internal/security"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// Factory builds a single Redis check.
func Factory(cfg any) ([]checks.Check, error) {
	c, ok := cfg.(config.RedisConfig)
	if !ok {
		return nil, fmt.Errorf("redis: unexpected config type %T", cfg)
	}
	return []checks.Check{&redisCheck{cfg: c}}, nil
}

type redisCheck struct{ cfg config.RedisConfig }

func (c *redisCheck) ID() string       { return "redis:availability" }
func (c *redisCheck) Name() string     { return "Redis Availability" }
func (c *redisCheck) Category() string { return "redis" }
func (c *redisCheck) Description() string {
	return "Verifies Redis answers PING and a namespaced temporary read/write round-trip."
}
func (c *redisCheck) Timeout() time.Duration {
	if c.cfg.Timeout > 0 {
		return c.cfg.Timeout
	}
	return 5 * time.Second
}
func (c *redisCheck) Severity() checks.Severity {
	if c.cfg.Severity == "warning" {
		return checks.SeverityWarning
	}
	return checks.SeverityCritical
}

// client is a minimal RESP client over one TCP connection.
type client struct {
	conn net.Conn
	r    *bufio.Reader
}

func dial(ctx context.Context, address, password string, db int, timeout time.Duration) (*client, error) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	cl := &client{conn: conn, r: bufio.NewReader(conn)}
	if password != "" {
		if _, err := cl.do("AUTH", password); err != nil {
			conn.Close()
			return nil, fmt.Errorf("AUTH failed: %w", err)
		}
	}
	if db != 0 {
		if _, err := cl.do("SELECT", strconv.Itoa(db)); err != nil {
			conn.Close()
			return nil, fmt.Errorf("SELECT failed: %w", err)
		}
	}
	return cl, nil
}

func (c *client) do(args ...string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	if _, err := c.conn.Write([]byte(b.String())); err != nil {
		return "", err
	}
	return c.readReply()
}

func (c *client) readReply() (string, error) {
	line, err := c.r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) < 3 {
		return "", fmt.Errorf("short redis reply %q", line)
	}
	switch line[0] {
	case '+', ':':
		return strings.TrimSpace(line[1:]), nil
	case '-':
		return "", fmt.Errorf("redis error: %s", strings.TrimSpace(line[1:]))
	case '$':
		n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil {
			return "", err
		}
		if n < 0 {
			return "", nil // nil bulk string
		}
		buf := make([]byte, n+2)
		if _, err := ioReadFull(c.r, buf); err != nil {
			return "", err
		}
		return string(buf[:n]), nil
	default:
		return "", fmt.Errorf("unexpected redis reply %q", line)
	}
}

func ioReadFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// LLEN returns the length of a list key; used by the workers queue check.
func LLEN(ctx context.Context, address, password string, db int, key string, timeout time.Duration) (int, error) {
	cl, err := dial(ctx, address, password, db, timeout)
	if err != nil {
		return 0, err
	}
	defer cl.conn.Close()
	s, err := cl.do("LLEN", key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(s)
}

func (c *redisCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "redis", Severity: c.Severity()}
	cl, err := dial(ctx, c.cfg.Address, c.cfg.Password, c.cfg.DB, c.Timeout())
	if err != nil {
		return fail(res, start, fmt.Sprintf("Redis connection to %s failed: %v", c.cfg.Address, err), "Check Redis is running and reachable, and the address in shipcheck.yml is correct.")
	}
	defer cl.conn.Close()
	pong, err := cl.do("PING")
	if err != nil || pong != "PONG" {
		return fail(res, start, fmt.Sprintf("Redis PING failed (got %q, err %v)", pong, err), "Redis accepted the connection but did not answer PING; check the server logs.")
	}
	key := security.ProbeKey()
	if _, err := cl.do("SET", key, "shipcheck", "EX", "10"); err != nil {
		return fail(res, start, fmt.Sprintf("Redis write test failed: %v", err), "Check the Redis user has write permission, or disable the Redis check.")
	}
	got, err := cl.do("GET", key)
	if err != nil || got != "shipcheck" {
		_, _ = cl.do("DEL", key)
		return fail(res, start, fmt.Sprintf("Redis read-back failed (got %q, err %v)", got, err), "Redis wrote but did not return the test value; check persistence / replication.")
	}
	if _, err := cl.do("DEL", key); err != nil {
		// Key has a 10s TTL, so a failed DEL is a warning, not a failure.
		res.Status = checks.StatusWarn
		res.Message = "Redis PING and read/write OK, but the temporary test key could not be deleted (it expires in 10s)"
		res.Recommendation = "No action needed; the key expires automatically."
		res.Duration = time.Since(start)
		return res
	}
	elapsed := time.Since(start)
	res.Status = checks.StatusPass
	res.Duration = elapsed
	res.Message = fmt.Sprintf("Redis PING and namespaced read/write OK in %dms", elapsed.Milliseconds())
	res.Details = map[string]any{"address": c.cfg.Address, "db": c.cfg.DB}
	return res
}

func fail(res checks.Result, start time.Time, msg, rec string) checks.Result {
	res.Status = checks.StatusFail
	res.Duration = time.Since(start)
	res.Message = msg
	res.Recommendation = rec
	return res
}
