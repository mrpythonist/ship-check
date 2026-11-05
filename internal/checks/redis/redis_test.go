package redischeck

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// fakeRedis is a minimal RESP server supporting PING/AUTH/SELECT/SET/GET/DEL/LLEN.
func fakeRedis(t *testing.T) (addr string, close func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	store := map[string]string{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					// Read RESP array header.
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if !strings.HasPrefix(line, "*") {
						return
					}
					var n int
					if _, err := parseInt(strings.TrimSpace(line[1:]), &n); err != nil {
						return
					}
					args := make([]string, 0, n)
					for i := 0; i < n; i++ {
						h, err := r.ReadString('\n')
						if err != nil {
							return
						}
						var ln2 int
						if _, err := parseInt(strings.TrimSpace(h[1:]), &ln2); err != nil {
							return
						}
						buf := make([]byte, ln2+2)
						if _, err := ioReadFull(r, buf); err != nil {
							return
						}
						args = append(args, string(buf[:ln2]))
					}
					if len(args) == 0 {
						continue
					}
					switch strings.ToUpper(args[0]) {
					case "PING":
						c.Write([]byte("+PONG\r\n"))
					case "AUTH", "SELECT":
						c.Write([]byte("+OK\r\n"))
					case "SET":
						store[args[1]] = args[2]
						c.Write([]byte("+OK\r\n"))
					case "GET":
						if v, ok := store[args[1]]; ok {
							c.Write([]byte("$" + itoa(len(v)) + "\r\n" + v + "\r\n"))
						} else {
							c.Write([]byte("$-1\r\n"))
						}
					case "DEL":
						delete(store, args[1])
						c.Write([]byte(":1\r\n"))
					case "LLEN":
						c.Write([]byte(":3\r\n"))
					default:
						c.Write([]byte("-ERR unknown\r\n"))
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().String(), func() { ln.Close() }
}

func parseInt(s string, out *int) (int, error) {
	var n int
	for _, ch := range s {
		n = n*10 + int(ch-'0')
	}
	*out = n
	return n, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func TestRedisRoundTrip(t *testing.T) {
	addr, close := fakeRedis(t)
	defer close()
	c, err := Factory(config.RedisConfig{Enabled: true, Address: addr, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	res := c[0].Run(context.Background())
	if res.Status != checks.StatusPass {
		t.Fatalf("expected pass, got %s: %s", res.Status, res.Message)
	}
}

func TestRedisUnreachableFails(t *testing.T) {
	c, _ := Factory(config.RedisConfig{Enabled: true, Address: "127.0.0.1:1", Timeout: time.Second})
	res := c[0].Run(context.Background())
	if res.Status != checks.StatusFail {
		t.Fatalf("expected fail, got %s", res.Status)
	}
}

func TestLLEN(t *testing.T) {
	addr, close := fakeRedis(t)
	defer close()
	n, err := LLEN(context.Background(), addr, "", 0, "queues:default", 2*time.Second)
	if err != nil || n != 3 {
		t.Fatalf("LLEN = %d, err %v", n, err)
	}
}
