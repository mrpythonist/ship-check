package security

import (
	"strings"
	"testing"
)

func TestRedactDSN(t *testing.T) {
	for _, in := range []string{
		"postgres://user:s3cret@localhost:5432/db?sslmode=disable",
		"host=localhost user=u password=s3cret dbname=db",
	} {
		out := Redact(in)
		if strings.Contains(out, "s3cret") {
			t.Fatalf("secret leaked in %q", out)
		}
	}
}

func TestProbeKeyNamespaced(t *testing.T) {
	k := ProbeKey()
	if !strings.HasPrefix(k, "shipcheck:probe:") {
		t.Fatalf("probe key not namespaced: %q", k)
	}
	if ProbeKey() == k {
		t.Fatal("probe keys must be unique")
	}
}
