// Command shipcheck is the ShipCheck CLI: production release verification.
package main

import (
	"os"

	"github.com/mrpythonist/ship-check/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
