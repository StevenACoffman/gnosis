package bundle

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/skillet/errs"
)

// maxFetch bounds what a single fetch will read into memory.
//
// It is not the archive's per-file cap and must not be confused with it: the cap
// decides what is *stored* and lives in standards/ where it can be argued with,
// while this decides what a single HTTP response may do to this process. A
// response above it is an error, not a `referenced` disposition, because nothing
// was read and so nothing can be recorded about it.
const maxFetch = 64 << 20

const (
	// FetchFile is a single local file.
	FetchFile FetchKind = "file"

	// FetchDir is a local directory, which yields one candidate per file within.
	FetchDir FetchKind = "dir"

	// FetchURL is an http or https resource.
	FetchURL FetchKind = "url"

	// FetchGit is a git repository, cloned to a temporary tree and then walked as
	// a directory.
	FetchGit FetchKind = "git"
)

// FetchKind is which of the four adapters (§9.2) handled a URI.
//
// There are four and there is deliberately no fifth. Every additional protocol is
// a new failure mode in the path that produces evidence, and the four cover
// everything the corpus has needed: a file, a tree of files, an HTTP resource, and
// a repository.
type FetchKind string

// Fetcher performs the I/O a fetch needs.
//
// It is a struct rather than a bare function so a test can supply its own HTTP
// client without a global, and so the git adapter's clone directory is a stated
// dependency rather than a call to os.TempDir buried three frames down.
type Fetcher struct {
	// HTTP is the client used for FetchURL. A nil client uses http.DefaultClient.
	HTTP *http.Client

	// Root is where a git clone is placed. An empty Root uses the system
	// temporary directory.
	Root string
}

// Fetch resolves one URI into the candidates the archive should decide on.
//
// Requires: uri is a local path, an http(s) URL, or a git remote.
// Ensures: returns at least one candidate on success — a directory or repository
// yields one per file, walked in lexical order so two runs over one tree produce
// the same sequence and therefore the same records. Reports EINVALID for a URI no
// adapter claims, rather than guessing.
//
// Reading a source is never the same event as judging it. Nothing here decides a
// disposition: an unreadable source is an error and a readable one is a candidate,
// and archive.Decide is the only thing that says which of the three it becomes.
func (f *Fetcher) Fetch(ctx context.Context, uri string) ([]archive.Candidate, error) {
	const op = "bundle.Fetcher.Fetch"

	switch classify(uri) {
	case FetchURL:
		c, err := f.fetchURL(ctx, uri)
		if err != nil {
			return nil, err
		}
		return []archive.Candidate{c}, nil
	case FetchGit:
		return f.fetchGit(ctx, uri)
	case FetchDir:
		return fetchDir(uri)
	case FetchFile:
		c, err := fetchFile(uri, uri)
		if err != nil {
			return nil, err
		}
		return []archive.Candidate{c}, nil
	default:
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": no adapter for " + uri,
		}
	}
}

// classify picks the adapter for a URI, and reports "" when none applies.
//
// A local path is classified by what is actually on disk rather than by its
// shape, because a directory and a file are indistinguishable as strings and
// getting it wrong would archive a directory listing as evidence.
func classify(uri string) FetchKind {
	switch {
	case looksLikeGit(uri):
		return FetchGit
	case strings.HasPrefix(uri, "http://"), strings.HasPrefix(uri, "https://"):
		return FetchURL
	}
	info, err := os.Stat(strings.TrimPrefix(uri, "file://"))
	switch {
	case err != nil:
		return ""
	case info.IsDir():
		return FetchDir
	default:
		return FetchFile
	}
}

// fetchFile reads one local file. The uri is carried separately from the path so
// a file walked out of a clone records the remote it came from, not the temporary
// directory it happened to land in.
func fetchFile(uri, path string) (archive.Candidate, error) {
	const op = "bundle.fetchFile"

	data, err := os.ReadFile(strings.TrimPrefix(path, "file://"))
	if err != nil {
		return archive.Candidate{}, &errs.Error{Op: op, Err: err}
	}
	return archive.Candidate{
		URI:       uri,
		Bytes:     data,
		Extension: strings.ToLower(filepath.Ext(path)),
	}, nil
}

// fetchDir walks a directory, yielding one candidate per file.
//
// Requires: dir exists.
// Ensures: lexical order, so two runs produce the same sequence. Directories that
// are not evidence are skipped by name: `.git` because it is the repository's own
// bookkeeping, and `.gnosis` because it is derived state that would otherwise be
// archived as though it were a source.
func fetchDir(dir string) ([]archive.Candidate, error) {
	const op = "bundle.fetchDir"

	root := strings.TrimPrefix(dir, "file://")
	var out []archive.Candidate
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		c, ferr := fetchFile(path, path)
		if ferr != nil {
			return ferr
		}
		out = append(out, c)
		return nil
	})
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	if len(out) == 0 {
		return nil, &errs.Error{Code: errs.EINVALID, Message: op + ": no files under " + dir}
	}
	return out, nil
}

// skipDir reports whether a directory holds bookkeeping rather than evidence.
func skipDir(name string) bool {
	return name == ".git" || name == stateDir
}

// fetchURL retrieves one http(s) resource.
//
// Ensures: a non-2xx response is an error naming the status, not a candidate. A
// 404 page is a document, and archiving it would store the error page as evidence
// under the URI of the thing that is missing — which is exactly the failure §4.1
// describes, arriving through the front door.
func (f *Fetcher) fetchURL(ctx context.Context, uri string) (archive.Candidate, error) {
	const op = "bundle.Fetcher.fetchURL"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, http.NoBody)
	if err != nil {
		return archive.Candidate{}, &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	client := f.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return archive.Candidate{}, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return archive.Candidate{}, &errs.Error{
			Op:      op,
			Message: op + ": " + uri + ": " + resp.Status,
		}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFetch+1))
	if err != nil {
		return archive.Candidate{}, &errs.Error{Op: op, Err: err}
	}
	if len(data) > maxFetch {
		return archive.Candidate{}, &errs.Error{
			Op:      op,
			Message: op + ": " + uri + ": response exceeds the fetch limit",
		}
	}
	return archive.Candidate{
		URI:       uri,
		Bytes:     data,
		MediaType: mediaType(resp.Header.Get("Content-Type")),
		Extension: urlExtension(uri),
	}, nil
}

// mediaType strips the parameters from a Content-Type, keeping the type itself.
// The charset is not recorded: the text test decides encoding, and a recorded
// charset that disagreed with the bytes would be provenance nobody could trust.
func mediaType(header string) string {
	t, _, _ := strings.Cut(header, ";")
	return strings.ToLower(strings.TrimSpace(t))
}

// urlExtension is the extension of a URL's path, ignoring query and fragment.
//
// It is derived from the path rather than from Content-Type because the extension
// gates the allowlist, and a server's content type is the server's to choose.
func urlExtension(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return strings.ToLower(filepath.Ext(u.Path))
}
