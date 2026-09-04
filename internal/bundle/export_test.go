package bundle

import (
	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

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

// CritiquableForTest is the population a critic run would draw from, and what each
// prompt would be told about a claim.
//
// It is here for §10.3's blinding, which is a **requirement** rather than a preference:
// the prompt must not carry the existing adjudication, warrant, status, trust tier or
// verification history. `relay.CriticClaim` enforces that by having nowhere to put them,
// and the failure a type cannot prevent is this projection copying one into a field that
// does exist — a warrant appended to the claim text, a status folded into the lead. That
// is a seam a test has to reach, and no exported call reaches it: `CriticPrompts` needs a
// writer, a lock, and a bundle on disk to answer the same question.
func CritiquableForTest(
	docs []Document, path string, sources map[string]lint.SourceVersion,
) ([]criticTarget, int) {
	return critiquable(docs, path, sources)
}

// CriticTargetView is one target's blinded fields, for a test that must not be able to
// see the rest.
//
// It returns the three strings a prompt is built from rather than the target itself,
// which is the same discipline the type under test follows: a helper handing back the
// whole struct would let an assertion pass by reading a field the prompt never sees.
func CriticTargetView(t *criticTarget) (text, lead string, quotes []string) {
	return t.Text, t.Lead, t.Quotes
}
