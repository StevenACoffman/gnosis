package bundle

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v6"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/skillet/errs"
)

// fetchGit clones a repository and walks it as a directory.
//
// Requires: uri is a git remote reachable with the ambient credentials.
// Ensures: one candidate per file in the working tree, in lexical order, each
// recording the *remote* it came from rather than the temporary directory it
// briefly lived in. The clone is removed before returning, so a fetch leaves
// nothing behind but records.
//
// The clone is shallow and single-branch. History is not evidence: a quotation is
// checked against the text of a file, and every earlier version of that file is
// either already in tier 0 from a previous fetch or was never cited. Fetching it
// would cost minutes on a large repository to archive nothing.
func (f *Fetcher) fetchGit(ctx context.Context, uri string) ([]archive.Candidate, error) {
	const op = "bundle.Fetcher.fetchGit"

	dir, err := os.MkdirTemp(f.Root, "gnosis-clone-")
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	// Removed even on the error paths: a failed clone leaves a partial tree, and
	// the next fetch would find the directory non-empty and fail differently.
	defer func() { _ = os.RemoveAll(dir) }()

	repo, err := git.PlainCloneContext(ctx, dir, &git.CloneOptions{
		URL:          uri,
		Depth:        1,
		SingleBranch: true,
		Tags:         git.NoTags,
	})
	if err != nil {
		return nil, &errs.Error{Op: op, Message: op + ": clone " + uri, Err: err}
	}

	candidates, err := fetchDir(dir)
	if err != nil {
		return nil, err
	}
	revision := headRevision(repo)
	for i := range candidates {
		candidates[i].URI = repoURI(uri, dir, candidates[i].URI)
		candidates[i].Revision = revision
	}
	return candidates, nil
}

// headRevision is the commit the clone landed on.
//
// Requires: repo is a freshly cloned repository.
// Ensures: a full hex commit hash, or empty when it could not be read.
//
// **Empty rather than an error, and that is the whole of the judgement here.** The
// revision is provenance a person may find useful; the bytes are the evidence. A
// clone that produced a working tree and an unreadable HEAD has delivered the
// evidence, and failing the fetch over the annotation would discard what the command
// was for. Empty is honest — it says the same thing every non-git adapter says.
func headRevision(repo *git.Repository) string {
	head, err := repo.Head()
	if err != nil {
		return ""
	}
	return head.Hash().String()
}

// repoURI rewrites a path inside the clone into a reference to the remote.
//
// The form is `<remote>#<path>`: a fragment, which is URI-legal and which no
// adapter will later mistake for something to fetch on its own.
//
// The commit is deliberately absent, and this is the same argument §4.3.1 makes
// against a timestamp. Including it would make a record's identity depend on the
// repository's activity rather than on the file's: one unrelated push would
// re-record every file in the tree, identical to its predecessor but for a hash
// nobody reads. Which commit a given text came from stays recoverable — the
// content hash is a git blob hash's input, and the repository still has it.
func repoURI(remote, cloneDir, path string) string {
	rel, err := filepath.Rel(cloneDir, path)
	if err != nil {
		// Unreachable for a path the walk produced from cloneDir. Falling back to
		// the bare remote loses the file's location and keeps the record honest
		// about the repository, which is the more important half.
		return remote
	}
	return remote + "#" + filepath.ToSlash(rel)
}

// looksLikeGit reports whether a URI should go to the git adapter.
//
// An https remote is claimed only when it ends in `.git`, because the common case
// for an https URI is a page to read and guessing wrong would clone a website.
func looksLikeGit(uri string) bool {
	switch {
	case strings.HasPrefix(uri, "git@"), strings.HasPrefix(uri, "ssh://"),
		strings.HasPrefix(uri, "git://"):
		return true
	default:
		return strings.HasSuffix(uri, ".git")
	}
}
