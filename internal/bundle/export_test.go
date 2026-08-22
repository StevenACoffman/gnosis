package bundle

import "github.com/StevenACoffman/gnosis/internal/audit"

// This file is compiled only under `go test`, so nothing here reaches a
// production build, and it adds no exported surface as consumers see the package.
//
// It exists because the property worth testing here cannot be reached from
// outside: verifyAudit's whole job is to notice an append that reported success
// and wrote nothing, and no exported call can produce that state — Audit either
// writes or returns an error. Reaching it means calling the verifier against a
// trail arranged by hand, which is what a caller can never do and a test must.

// VerifyAuditForTest checks whether row is the trail's last line.
func VerifyAuditForTest(bundleDir string, row *audit.Row) error {
	return verifyAudit(bundleDir, row)
}

// ReadTailForTest reads up to n bytes from the end of path.
func ReadTailForTest(path string, n int64) ([]byte, error) {
	return readTail("bundle.ReadTailForTest", path, n)
}
