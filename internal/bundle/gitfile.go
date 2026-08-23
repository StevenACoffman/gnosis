package bundle

import (
	"errors"
	"io"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"

	"github.com/StevenACoffman/skillet/errs"
)

// FileAtRef reads one bundle-relative file as it stood at a git revision.
//
// Requires: bundleDir is inside a git worktree; ref is anything
// `plumbing.Revision` accepts — a branch, a tag, HEAD, HEAD~3, a full or
// abbreviated hash.
// Ensures: **ENOTFOUND when the revision resolves and the file was not in that
// tree; EINVALID when the revision does not resolve.** Callers distinguish them:
// an absent file is a legitimate state for a bundle that predates it and they fall
// back to a default, while an unresolvable revision is a mistyped argument and
// must not be quietly treated as an empty history. The worktree is not touched.
//
// This exists because §6.2 asks what a threshold *was*, and the previous value of
// a hand-edited file lives in git rather than anywhere gnosis controls. Reading it
// is the whole of what makes a loosening report possible without gnosis owning
// every write to `standards/`.
func FileAtRef(bundleDir, ref, rel string) ([]byte, error) {
	const op = "bundle.FileAtRef"

	repo, err := git.PlainOpenWithOptions(bundleDir,
		&git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": " + bundleDir + " is not inside a git worktree",
		}
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		// EINVALID rather than ENOTFOUND, and the distinction is load-bearing: a
		// caller falls back to a default when a *file* was not in a tree, which is
		// a legitimate state for a bundle predating that file. A revision that does
		// not resolve is a typo in an argument, and treating it as the same code
		// made `--since no-such-ref` compare silently against the seed and report
		// nothing — which the test caught.
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": no such revision: " + ref,
		}
	}
	commit, err := repo.CommitObject(*hash)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return fileFromCommit(op, commit, ref, rel)
}

// fileFromCommit pulls one path out of a commit's tree.
func fileFromCommit(op string, commit *object.Commit, ref, rel string) ([]byte, error) {
	f, err := commit.File(rel)
	if err != nil {
		if errors.Is(err, object.ErrFileNotFound) {
			return nil, &errs.Error{
				Code:    errs.ENOTFOUND,
				Message: op + ": " + rel + " was not in the tree at " + ref,
			}
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	reader, err := f.Reader()
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = reader.Close() }()

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return body, nil
}
