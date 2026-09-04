package bundle

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/StevenACoffman/skillet/errs"
)

// vcsDir is git's own storage, which is not part of the corpus it versions.
//
// Named beside stateDir rather than inlined at the one comparison, because the two
// exclusions have different reasons and a reader who saw only the code would assume one
// rule with two entries: stateDir is *derived from* the corpus, and this *contains* it.
const vcsDir = ".git"

// Portable reports whether a bundle-relative path is part of the shareable corpus.
//
// Requires: rel is slash-separated and bundle-relative, as fs.WalkDir yields it.
// Ensures: false for everything under .gnosis/ and .git/, true otherwise. Pure.
//
// **The name is §16.3's**: "`gnosis export --format okf` produces a portable bundle for
// sharing outside the team". This is that set, and it is one predicate rather than two
// because `export` and `proof create` are asking the same question — which bytes are the
// corpus, as opposed to which bytes this user's tools happen to have left beside it.
//
// **The exclusion is the whole content, and it is a confidentiality rule rather than a
// tidiness one.** `.gnosis/` holds the audit trail, the prompt cache, the miss log and
// the coverage ledger: what one person asked a model and when. §4.5 makes it derived and
// gitignored for the reasons a cache should not be reviewed, and an export or a proof
// packet that carried it would publish a colleague's session history to whoever the
// bundle was shared with — which is not a thing the recipient asked for or the sender
// would notice.
//
// A prefix match on the segment rather than on the string: `.gnosisrc` is not inside
// `.gnosis/`, and a string prefix would exclude it. This is `underPrefix`'s rule one file
// over, and it is here for the same reason.
func Portable(rel string) bool {
	head, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
	return head != stateDir && head != vcsDir
}

// PortablePaths is every file in the shareable corpus, sorted.
//
// Requires: bundleDir is a bundle root that exists.
// Ensures: bundle-relative slash-separated paths, sorted, with no directories; an empty
// slice for a bundle holding nothing portable. Not pure — it walks a filesystem.
//
// **Sorted, and that is a correctness requirement rather than a courtesy.** A proof
// packet records artifacts in the order it is handed them, so an order that came from
// directory iteration would put different bytes in the packet on every run — and a proof
// that does not reproduce is not one. The same argument makes the export listing stable
// enough to diff.
func PortablePaths(bundleDir string) ([]string, error) {
	const op = "bundle.PortablePaths"

	var out []string
	fsys := os.DirFS(bundleDir)
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, walkErr error) error {
		switch {
		case walkErr != nil:
			return walkErr
		case name == ".":
			return nil
		case !Portable(name):
			// Skipping the directory rather than each file under it: a prompt cache
			// holds one file per critique and walking them to reject them is work
			// whose only product is the same answer.
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		case d.IsDir():
			return nil
		}
		out = append(out, name)
		return nil
	})
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	sort.Strings(out)
	return out, nil
}
