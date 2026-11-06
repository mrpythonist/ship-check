# Screenshots

No screenshots are included yet.

ShipCheck is a terminal tool. The honest captures for this folder would
be terminal recordings of `shipcheck verify` passing and failing, and of
`shipcheck doctor` output. Those captures need a Go toolchain to build
the binary and a running target to verify against. The VM used to
prepare this documentation had no Go toolchain, so no new captures
were made here, and no output was fabricated.

What exists instead:

- The README shows example terminal output from the automated smoke
  test (`test/smoke.sh`), which was run against a local test server
  during v0.1.0 verification.
- The CLI reference in [cli-reference.md](cli-reference.md) shows the
  exact output shapes, taken from the reporting source
  (`internal/reporting/reporting.go`) and the CLI source
  (`internal/cli/cli.go`).

To add a real capture later, build the binary, run `./test/smoke.sh`
(which prints the PASS and FAIL runs), and save the terminal output as
an image or as text. Do not paste invented output here.
