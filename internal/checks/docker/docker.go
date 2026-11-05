// Package dockercheck verifies the Docker daemon and required containers
// over the Docker Engine API on the local Unix socket. It never starts,
// stops, or restarts anything: verification is read-only.
package dockercheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// Factory builds a single Docker check.
func Factory(cfg any) ([]checks.Check, error) {
	c, ok := cfg.(config.DockerConfig)
	if !ok {
		return nil, fmt.Errorf("docker: unexpected config type %T", cfg)
	}
	return []checks.Check{&dockerCheck{cfg: c}}, nil
}

type dockerCheck struct{ cfg config.DockerConfig }

func (c *dockerCheck) ID() string       { return "docker:containers" }
func (c *dockerCheck) Name() string     { return "Docker Containers" }
func (c *dockerCheck) Category() string { return "docker" }
func (c *dockerCheck) Description() string {
	return "Verifies the Docker daemon answers and required containers are running and healthy."
}
func (c *dockerCheck) Timeout() time.Duration {
	if c.cfg.Timeout > 0 {
		return c.cfg.Timeout
	}
	return 5 * time.Second
}
func (c *dockerCheck) Severity() checks.Severity {
	if c.cfg.Severity == "warning" {
		return checks.SeverityWarning
	}
	return checks.SeverityCritical
}

type containerSummary struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
}

type containerInspect struct {
	State struct {
		Running bool `json:"Running"`
		Health  *struct {
			Status string `json:"Status"`
		} `json:"Health"`
		RestartCount int `json:"RestartCount"`
	} `json:"State"`
}

func (c *dockerCheck) client(ctx context.Context) *http.Client {
	socket := c.cfg.Socket
	if socket == "" {
		socket = "/var/run/docker.sock"
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		},
	}
	return &http.Client{Transport: transport, Timeout: c.Timeout()}
}

func (c *dockerCheck) get(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.client(ctx).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return body, resp.StatusCode, nil
}

func (c *dockerCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "docker", Severity: c.Severity()}
	if _, status, err := c.get(ctx, "/_ping"); err != nil || status != http.StatusOK {
		return fail(res, start, fmt.Sprintf("Docker daemon did not answer on %s: %v (status %d)", c.cfg.Socket, err, status), "Check Docker is running and the socket path in shipcheck.yml is correct; the user running ShipCheck needs socket access.")
	}
	body, status, err := c.get(ctx, "/containers/json?all=1")
	if err != nil || status != http.StatusOK {
		return fail(res, start, fmt.Sprintf("could not list containers: %v (status %d)", err, status), "Check the Docker API version and socket permissions.")
	}
	var containers []containerSummary
	if err := json.Unmarshal(body, &containers); err != nil {
		return fail(res, start, fmt.Sprintf("could not parse container list: %v", err), "Check the Docker daemon version; ShipCheck expects the standard Engine API.")
	}
	byName := map[string]containerSummary{}
	running := 0
	for _, cont := range containers {
		for _, n := range cont.Names {
			byName[strings.TrimPrefix(n, "/")] = cont
		}
		if cont.State == "running" {
			running++
		}
	}
	res.Details = map[string]any{"containers_total": len(containers), "containers_running": running}
	if len(c.cfg.Containers) == 0 {
		res.Status = checks.StatusPass
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("Docker daemon OK, %d container(s) running", running)
		return res
	}
	for _, want := range c.cfg.Containers {
		cont, ok := byName[want.Name]
		if !ok {
			return fail(res, start, fmt.Sprintf("required container %q was not found", want.Name), fmt.Sprintf("Check the container name and whether it was removed or renamed; run `docker ps -a` to list containers."))
		}
		if cont.State != "running" {
			return fail(res, start, fmt.Sprintf("container %q is %s, not running", want.Name, cont.State), fmt.Sprintf("Start the container with `docker start %s` and check its logs.", want.Name))
		}
		// Inspect for health + restart count (read-only).
		inspectBody, st, err := c.get(ctx, "/containers/"+cont.ID+"/json")
		if err == nil && st == http.StatusOK {
			var insp containerInspect
			if json.Unmarshal(inspectBody, &insp) == nil {
				if c.cfg.RequireHealth && insp.State.Health != nil && insp.State.Health.Status != "healthy" {
					return fail(res, start, fmt.Sprintf("container %q health is %q", want.Name, insp.State.Health.Status), fmt.Sprintf("Check `docker inspect %s` and the container logs for the failing healthcheck.", want.Name))
				}
				if want.MaxRestarts > 0 && insp.State.RestartCount > want.MaxRestarts {
					return fail(res, start, fmt.Sprintf("container %q restarted %d times (max %d)", want.Name, insp.State.RestartCount, want.MaxRestarts), fmt.Sprintf("Check `docker logs %s`; repeated restarts usually mean a crash loop.", want.Name))
				}
			}
		}
	}
	elapsed := time.Since(start)
	res.Status = checks.StatusPass
	res.Duration = elapsed
	res.Message = fmt.Sprintf("Docker daemon OK, all %d required container(s) running", len(c.cfg.Containers))
	return res
}

func fail(res checks.Result, start time.Time, msg, rec string) checks.Result {
	res.Status = checks.StatusFail
	res.Duration = time.Since(start)
	res.Message = msg
	res.Recommendation = rec
	return res
}
