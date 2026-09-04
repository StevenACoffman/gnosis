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

	repo, ref, err := headRef(op, bundleDir)
	if err != nil || ref == nil {
		return time.Time{}, err
	}
	commit, err := repo.CommitObject(plumbing.NewHash(ref.Hash().String()))
	if err != nil {
		return time.Time{}, &errs.Error{Op: op, Err: err}
	}
	return commit.Committer.When.UTC(), nil
}

// HeadSHA is the commit the bundle currently sits on.
//
// Requires: nothing, as HeadTime requires nothing.
// Ensures: the full hex hash of HEAD, or **the empty string with no error** for a
// directory that is not a worktree or a worktree with no commits.
//
// The empty string rather than an error, because that is precisely what its one caller
// needs: `proof.Create` records no provenance when it is handed an empty SHA, and a
// proof packet verifies on its bytes regardless of the commit it names. A bundle that
// is not under version control can still be proved; it just cannot say where it came
// from, and refusing to prove it would make version control a precondition for
// integrity rather than for provenance.
func HeadSHA(bundleDir string) (string, error) {
	const op = "bundle.HeadSHA"

	_, ref, err := headRef(op, bundleDir)
	if err != nil || ref == nil {
		return "", err
	}
	return ref.Hash().String(), nil
}

// headRef resolves HEAD, reporting "no answer" as a nil reference and no error.
//
// Requires: op names the calling operation, for the error.
// Ensures: a nil reference and no error when bundleDir is not a worktree, or is one with
// no commits. The repository is returned with it so a caller wanting the commit object
// does not open the same worktree twice. Not pure — it reads a repository.
//
// **The two tolerated cases are named rather than swallowed as "any error".** The first
// draft of HeadTime caught everything, and a linter caught that: it would have hidden a
// permission failure as "no answer", indistinguishable from a directory that is
// legitimately not a repository — inside `doctor`, whose whole job is telling a reader
// which of the two they have.
func headRef(op, bundleDir string) (*git.Repository, *plumbing.Reference, error) {
	repo, err := git.PlainOpenWithOptions(bundleDir,
		&git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		// Not a worktree is not a failure. §4.6 has gnosis working over a git
		// repository and does not require one to exist before anything else can be
		// diagnosed, and `doctor` is the command that must run in the broken cases.
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, nil, nil
		}
		return nil, nil, &errs.Error{Op: op, Err: err}
	}
	ref, err := repo.Head()
	if err != nil {
		// An initialised repository with no commits: HEAD points at an unborn
		// branch. That is an ordinary state for a bundle somebody just created.
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, nil, nil
		}
		return nil, nil, &errs.Error{Op: op, Err: err}
	}
	return repo, ref, nil
}
