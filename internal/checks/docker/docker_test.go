package dockercheck

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// fakeDocker serves the Engine API on a Unix socket.
func fakeDocker(t *testing.T) string {
	t.Helper()
	sock := t.TempDir() + "/docker.sock"
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"Id":"abc123","Names":["/my-app"],"State":"running","Status":"Up 1h"}]`))
	})
	mux.HandleFunc("/containers/abc123/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"State":{"Running":true,"Health":{"Status":"healthy"},"RestartCount":0}}`))
	})
	go http.Serve(ln, mux)
	return sock
}

func TestDockerPass(t *testing.T) {
	sock := fakeDocker(t)
	c, err := Factory(config.DockerConfig{Enabled: true, Socket: sock, Timeout: 2 * time.Second,
		Containers: []config.DockerContainer{{Name: "my-app", MaxRestarts: 3}}, RequireHealth: true})
	if err != nil {
		t.Fatal(err)
	}
	res := c[0].Run(context.Background())
	if res.Status != checks.StatusPass {
		t.Fatalf("expected pass, got %s: %s", res.Status, res.Message)
	}
}

func TestDockerMissingContainerFails(t *testing.T) {
	sock := fakeDocker(t)
	c, _ := Factory(config.DockerConfig{Enabled: true, Socket: sock, Timeout: 2 * time.Second,
		Containers: []config.DockerContainer{{Name: "nope"}}})
	res := c[0].Run(context.Background())
	if res.Status != checks.StatusFail {
		t.Fatalf("expected fail, got %s", res.Status)
	}
}

func TestDockerMissingSocketFails(t *testing.T) {
	sock := t.TempDir() + "/nope.sock"
	_ = os.Remove(sock)
	c, _ := Factory(config.DockerConfig{Enabled: true, Socket: sock, Timeout: 2 * time.Second})
	res := c[0].Run(context.Background())
	if res.Status != checks.StatusFail {
		t.Fatalf("expected fail, got %s", res.Status)
	}
}
