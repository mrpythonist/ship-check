// Package httpcheck verifies HTTP endpoints: status, response time,
// body content, and (for HTTPS) TLS certificate expiry.
package httpcheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/internal/security"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// Factory builds one check per configured endpoint.
func Factory(cfg any) ([]checks.Check, error) {
	c, ok := cfg.(config.HTTPConfig)
	if !ok {
		return nil, fmt.Errorf("http: unexpected config type %T", cfg)
	}
	var out []checks.Check
	for _, e := range c.Endpoints {
		e := e
		out = append(out, &endpointCheck{ep: e})
	}
	return out, nil
}

type endpointCheck struct {
	ep config.HTTPEndpoint
}

func (c *endpointCheck) ID() string       { return "http:" + slug(c.ep.Name) }
func (c *endpointCheck) Name() string     { return c.ep.Name }
func (c *endpointCheck) Category() string { return "http" }
func (c *endpointCheck) Description() string {
	return "Verifies an HTTP endpoint returns the expected status within the time limit."
}
func (c *endpointCheck) Timeout() time.Duration {
	if c.ep.Timeout > 0 {
		return c.ep.Timeout
	}
	return 5 * time.Second
}
func (c *endpointCheck) Severity() checks.Severity {
	if c.ep.Severity == "warning" {
		return checks.SeverityWarning
	}
	return checks.SeverityCritical
}

func (c *endpointCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "http", Severity: c.Severity()}
	// TLS certificate check runs first for HTTPS so an expiring cert is
	// reported even if the endpoint itself still answers.
	if strings.HasPrefix(c.ep.URL, "https://") && c.ep.CheckTLS {
		if r := c.checkTLS(ctx); r.Status != checks.StatusPass {
			r.Duration = time.Since(start)
			return r
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ep.URL, nil)
	if err != nil {
		return fail(res, start, fmt.Sprintf("could not build request for %s: %v", security.RedactURL(c.ep.URL), err), "Check the endpoint URL in shipcheck.yml.")
	}
	client := &http.Client{Timeout: c.Timeout()}
	resp, err := client.Do(req)
	if err != nil {
		return fail(res, start, fmt.Sprintf("request to %s failed: %v", security.RedactURL(c.ep.URL), err), "Check the application is running and reachable from this host.")
	}
	defer resp.Body.Close()
	// Read at most 1 MiB: enough to validate body content, small enough
	// that a huge error page cannot exhaust memory or leak into reports.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	elapsed := time.Since(start)
	res.Details = map[string]any{
		"url": security.RedactURL(c.ep.URL), "status": resp.StatusCode,
		"response_time_ms": elapsed.Milliseconds(),
	}
	if resp.StatusCode != c.ep.ExpectedStatus {
		return fail(res, start, fmt.Sprintf("expected HTTP %d, got %d from %s", c.ep.ExpectedStatus, resp.StatusCode, security.RedactURL(c.ep.URL)), "Check the application logs for the failing endpoint; the deployment may be partially broken.")
	}
	if c.ep.MaxResponseTimeMs > 0 && elapsed.Milliseconds() > int64(c.ep.MaxResponseTimeMs) {
		return fail(res, start, fmt.Sprintf("response took %dms, over the %dms limit", elapsed.Milliseconds(), c.ep.MaxResponseTimeMs), "Investigate slow handlers, database queries, or cold starts on this endpoint.")
	}
	if c.ep.BodyContains != "" && !strings.Contains(string(body), c.ep.BodyContains) {
		return fail(res, start, fmt.Sprintf("response body does not contain %q", c.ep.BodyContains), "The endpoint answered but the expected content is missing; the app may be serving an error page with a 200 status.")
	}
	if c.ep.BodyNotContains != "" && strings.Contains(string(body), c.ep.BodyNotContains) {
		return fail(res, start, fmt.Sprintf("response body contains forbidden text %q", c.ep.BodyNotContains), "The endpoint is serving an error page or debug output.")
	}
	res.Status = checks.StatusPass
	res.Duration = elapsed
	res.Message = fmt.Sprintf("HTTP %d from %s in %dms", resp.StatusCode, security.RedactURL(c.ep.URL), elapsed.Milliseconds())
	return res
}

func (c *endpointCheck) checkTLS(ctx context.Context) checks.Result {
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "http", Severity: c.Severity()}
	d := &tls.Dialer{Config: &tls.Config{InsecureSkipVerify: false}}
	conn, err := d.DialContext(ctx, "tcp", hostPort(c.ep.URL))
	if err != nil {
		return fail(res, time.Now(), fmt.Sprintf("TLS handshake failed for %s: %v", security.RedactURL(c.ep.URL), err), "Check the certificate chain and TLS configuration.")
	}
	defer conn.Close()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return fail(res, time.Now(), "TLS handshake did not return a TLS connection", "Check the certificate configuration.")
	}
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return fail(res, time.Now(), "TLS handshake returned no certificate", "Check the certificate configuration.")
	}
	cert := state.PeerCertificates[0]
	days := int(time.Until(cert.NotAfter).Hours() / 24)
	res.Details = map[string]any{"tls_expires": cert.NotAfter.Format(time.RFC3339), "tls_days_left": days}
	minDays := c.ep.TLSMinDays
	if minDays == 0 {
		minDays = 14
	}
	if days < 0 {
		return fail(res, time.Now(), "TLS certificate has expired", "Renew the certificate immediately.")
	}
	if days < minDays {
		res.Status = checks.StatusWarn
		res.Message = fmt.Sprintf("TLS certificate expires in %d days (threshold %d days)", days, minDays)
		res.Recommendation = "Renew the certificate before it expires."
		return res
	}
	res.Status = checks.StatusPass
	res.Message = fmt.Sprintf("TLS certificate valid for %d more days", days)
	return res
}

func fail(res checks.Result, start time.Time, msg, rec string) checks.Result {
	res.Status = checks.StatusFail
	res.Duration = time.Since(start)
	res.Message = msg
	res.Recommendation = rec
	return res
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func hostPort(raw string) string {
	// Strip scheme, keep host; default to 443 for https.
	rest := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
	host := strings.SplitN(rest, "/", 2)[0]
	if !strings.Contains(host, ":") {
		host += ":443"
	}
	return host
}
