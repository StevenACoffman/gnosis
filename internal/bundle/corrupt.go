package bundle

import "github.com/StevenACoffman/skillet/errs"

// corrupt reports that bytes on disk are not what they must be.
//
// Requires: op names the operation; what identifies the artifact; detail says
// what is wrong with it.
// Ensures: an EINVALID error whose message marks this as corruption rather than a
// failure to read.
//
// §15 draws the line: **only malformed state, a checksum mismatch, or unreadable
// evidence is corruption. A failed read, a full disk, and a git subprocess that
// died are operational.** They call for opposite responses — one is a retry and the
// other is somebody looking at a file — and collapsing them sends a reader hunting
// for tampering when a volume unmounted, or lets a genuinely corrupt record read as
// a transient failure worth trying again.
//
// **This makes the distinction legible rather than machine-checkable, and that is
// worth admitting.** `errs` has five codes and none of them means "the bytes are
// wrong"; adding a sixth for one consumer is what skillet's own guidance says not
// to do. EINVALID carries the half a caller can act on — no retry of the same input
// will help — and the wording carries the half a person needs. If a second consumer
// ever needs to branch on corruption programmatically, that is the real need a
// sixth code would answer, and TODO records it.
func corrupt(op, what, detail string) error {
	return &errs.Error{
		Code:    errs.EINVALID,
		Message: op + ": corruption in " + what + ": " + detail,
	}
}
