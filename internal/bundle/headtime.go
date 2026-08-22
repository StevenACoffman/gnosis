package bundle

import (
	"errors"
	"time"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"

	"github.com/StevenACoffman/skillet/errs"
)

// HeadTime is when the bundle's current commit was made.
//
// Requires: nothing. A directory that is not a git worktree, and a worktree with
// no commits, both return the zero time and no error.
// Ensures: the committer time of HEAD, in UTC. **The zero time means "no answer",
// not "long ago"**, and a caller comparing it against anything must treat it that
// way — reporting a trail as stale because the bundle is not a git repository
// would fire on every corpus that is not one.
//
// The committer time rather than the author time: an author date can be rewritten
// by a rebase or set by hand, and what this compares against is when a write
// actually reached this clone.
//
// It is HEAD rather than the newest commit touching the bundle subtree, which is
// what §15 asks for. Walking the history for a path-filtered maximum costs a
// traversal on every `doctor` run and answers a subtly different question — a
// commit touching only `README.md` still means somebody was writing here, and the
// trail should have a row near it if gnosis did the writing. If that turns out to
// produce false reports, the path filter is the fix and it belongs behind the
// finding rather than in front of it.
func HeadTime(bundleDir string) (time.Time, error) {
	const op = "bundle.HeadTime"

	repo, err := git.PlainOpenWithOptions(bundleDir,
		&git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		// Not a worktree is not a failure. §4.6 has gnosis working over a git
		// repository and does not require one to exist before anything else can be
		// diagnosed, and `doctor` is the command that must run in the broken cases.
		//
		// The condition is named rather than "any error", which the first draft
		// did and a linter caught. Swallowing every error here would have hidden a
		// permission failure as "no answer" — indistinguishable from a directory
		// that is legitimately not a repository, in the one command whose job is
		// to tell a reader which.
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return time.Time{}, nil
		}
		return time.Time{}, &errs.Error{Op: op, Err: err}
	}

	ref, err := repo.Head()
	if err != nil {
		// An initialised repository with no commits: HEAD points at an unborn
		// branch. That is an ordinary state for a bundle somebody just created.
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return time.Time{}, nil
		}
		return time.Time{}, &errs.Error{Op: op, Err: err}
	}
	commit, err := repo.CommitObject(plumbing.NewHash(ref.Hash().String()))
	if err != nil {
		return time.Time{}, &errs.Error{Op: op, Err: err}
	}
	return commit.Committer.When.UTC(), nil
}
