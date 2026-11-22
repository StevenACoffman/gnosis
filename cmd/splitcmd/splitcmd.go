// Package splitcmd implements the "split" CLI command.
package splitcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/split"
	"github.com/StevenACoffman/skillet/atomicfile"
	toerr "github.com/StevenACoffman/toerr/errors"
)

// longHelp is the split command's help text.
const longHelp = `Cut a Markdown file into parts no larger than the archive's per-file cap.

A source over standards/archive.toml's per_file_cap is not archived: it falls to
` + "`referenced`" + `, keeping its hash and losing its text, so no quotation from it can
validate. split keeps the cap and the evidence both. Fetch the parts, not the
original.

Parts are cut only at heading lines, shallowest level first — parts of a book,
then chapters, then sections — and a piece is cut further only while it is over
the limit. --level N cuts at that heading level first, so a book can be split by
chapter even where fewer, larger parts would fit.

Three properties are checked on every run, and a source that fails any of them is
refused and nothing is written for it:

  - every part's body is a verbatim slice of the source, apart from its
    recorded link repairs;
  - the parts tile the source, with nothing dropped or duplicated;
  - the parts render to exactly the HTML the source renders to.

Link reference definitions are document-wide, so a part using one defined
elsewhere gets a verbatim copy appended; the rendering check is what proves that
was enough.

In-document links are repaired. A link to #anchor whose heading landed in another
part is pointed at <that part>.md#anchor, and one whose heading's anchor is
numbered differently within its part is renumbered. That is the one change made
to a part's text, so it is recorded: each rewrite is checked to be nothing but a
fragment swapped for an anchor in this split, and the frontmatter counts them. A
fragment naming no heading in the source is left as written and listed.

Each part is written beside its source as <stem>-<NN>-<slug>.md and begins with a
gnosis_split frontmatter block naming the source, its SHA-256, the part's byte and
line ranges, and the headings enclosing it — so a quotation found in a part can be
traced to the place in the original it came from.

Every file split leaves for fetch — each part, or a source small enough to fetch
whole — is put through fetch's own admission decision, and one fetch would refuse
is reported as unfetchable with fetch's finding: a zero-width character and where,
a secret, an injection pattern. Nothing is stripped; the fix belongs in the source.

Every heading the split cuts at is checked for reading as something other than a
heading — a multi-line setext heading (code or output above a line of dashes), a
shell command, an assignment or call, a fragment starting lower-case like the chart
label "# of Sales", or a chapter nested under front matter such as "Preface" — and
each is reported as a suspect with its line. --strict refuses a source with any,
writing nothing for it.

Re-running over an unchanged source is a no-op: identical files are left alone. A
file that exists with different bytes is a conflict and is never overwritten, and a
part left over from an earlier split of the same source is reported as stale and
never deleted. Both are findings; deciding what to do with them is yours.

split writes beside its sources, not into the bundle, so it takes no writer lock
and leaves no audit row. --bundle is read only for the cap.`

// Part states. A part is written only when it is absent; one that exists with
// other bytes is reported, never overwritten.
const (
	stateCreated   = "created"
	stateUnchanged = "unchanged"
	stateConflict  = "conflict"
)

// Config holds the configuration for the split command.
type Config struct {
	*root.Config

	// DryRun plans and compares exactly as a real run does and writes nothing.
	DryRun bool

	// MaxBytes lowers the part size below the bundle's per-file cap. It cannot
	// raise it: a part over the cap falls to `referenced` exactly as the whole
	// source did, so a larger limit would plan files the archive then refuses.
	MaxBytes int

	// Level cuts at headings of this level even where fewer parts would fit.
	Level int

	// Strict refuses a source with a suspect cut heading, writing nothing for it,
	// instead of writing its parts and reporting the suspects.
	Strict bool

	Flags   *ff.FlagSet
	Command *ff.Command

	// gates are fetch's admission gates for this bundle, loaded once per run and
	// read by every source's prediction.
	gates archive.Gates
}

// Source is what became of one source file.
type Source struct {
	Path       string   `json:"path"`
	SHA256     string   `json:"sha256,omitempty"`
	Bytes      int      `json:"bytes"`
	Parts      []Part   `json:"parts,omitempty"`
	Fits       bool     `json:"fits,omitempty"`
	Refused    string   `json:"refused,omitempty"`
	Oversize   []string `json:"oversize,omitempty"`
	Relinked   int      `json:"relinked,omitempty"`
	Unresolved int      `json:"unresolved,omitempty"`
	Stale      []string `json:"stale,omitempty"`

	// Unfetchable are the files this run leaves that fetch will not archive as text.
	Unfetchable []Unfetchable `json:"unfetchable,omitempty"`

	// Suspects are the headings this source was cut at that read as something
	// other than a heading.
	Suspects []split.Suspect `json:"suspects,omitempty"`
}

// Part is one planned file and what happened to it.
type Part struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
	Lines string `json:"lines"`
	State string `json:"state"`
}

// Result is the whole run.
type Result struct {
	Limit   int      `json:"limit"`
	DryRun  bool     `json:"dry_run"`
	Sources []Source `json:"sources"`
}

// New registers the split command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("split").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.DryRun, 'n', "dry-run", "plan and compare without writing")
	cfg.Flags.IntVar(&cfg.MaxBytes, 0, "max-bytes", 0,
		"largest part in bytes; defaults to, and may not exceed, the archive's per-file cap")
	cfg.Flags.BoolVar(&cfg.Strict, 0, "strict",
		"refuse a source whose split would cut at a heading that reads as code or a label")
	cfg.Flags.IntVar(&cfg.Level, 0, "level", 0,
		"cut at headings of this level (1-6) even where fewer parts would fit")
	cfg.Command = &ff.Command{
		Name:      "split",
		Usage:     "gnosis split [--dry-run] [--max-bytes N] [--level N] <FILE.md>...",
		ShortHelp: "cut a Markdown source too large to archive into parts that are not",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: load the cap, plan each source, compare, write.
func (c *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return c.usage(errors.New("split needs at least one Markdown file; " +
			"try `gnosis split book.md --dry-run`"))
	}
	if c.Level < 0 || c.Level > 6 {
		return c.usage(fmt.Errorf("--level must be between 1 and 6, got %d", c.Level))
	}

	loaded, err := bundle.LoadArchiveStandards(c.Bundle)
	if err != nil {
		return c.fail(root.ReasonStandardsInvalid, err)
	}
	c.gates = bundle.ArchiveGates(loaded, c.Rules)
	limit := int(loaded.PerFileCap.Value)
	switch {
	case c.MaxBytes < 0:
		return c.usage(fmt.Errorf("--max-bytes must be positive, got %d", c.MaxBytes))
	case c.MaxBytes > limit:
		return c.usage(fmt.Errorf("--max-bytes %d exceeds the archive's per-file cap of %d; "+
			"a larger part would be referenced, not archived", c.MaxBytes, limit))
	case c.MaxBytes > 0:
		limit = c.MaxBytes
	}

	result := &Result{Limit: limit, DryRun: c.DryRun}
	for _, path := range args {
		src, err := c.splitOne(path, limit)
		if err != nil {
			return c.fail(root.ReasonFetchFailed, err)
		}
		result.Sources = append(result.Sources, src)
	}
	return c.report(result)
}

// splitOne plans one source and applies the plan.
func (c *Config) splitOne(path string, limit int) (Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Source{}, toerr.WrapWithMessage(err, "split: read "+path)
	}
	src := Source{Path: path, Bytes: len(data)}

	plan, refused := c.plan(data, filepath.Base(path), limit)
	if refused != "" {
		src.Refused = refused
		return src, nil
	}
	src.SHA256 = plan.SHA256
	src.Oversize = plan.Oversize
	src.Suspects = plan.Suspects
	if c.Strict && len(plan.Suspects) > 0 {
		src.Refused = fmt.Sprintf("--strict: %d cut heading(s) read as something other "+
			"than a heading; nothing was written", len(plan.Suspects))
		return src, nil
	}
	if len(plan.Parts) == 0 {
		src.Fits = true
		src.Unfetchable = predict([]plannedFile{{path: path, content: data}}, c.gates, c.Rules)
		return src, nil
	}

	dir := filepath.Dir(path)
	planned := map[string]bool{}
	files := make([]plannedFile, 0, len(plan.Parts))
	for i := range plan.Parts {
		p := &plan.Parts[i]
		planned[p.Name] = true
		files = append(files, plannedFile{path: filepath.Join(dir, p.Name), content: p.Content})
		src.Relinked += p.Relinked
		src.Unresolved += len(p.Unresolved)
		state, err := c.place(filepath.Join(dir, p.Name), p.Content)
		if err != nil {
			return Source{}, err
		}
		src.Parts = append(src.Parts, Part{
			Path:  filepath.Join(dir, p.Name),
			Bytes: len(p.Content),
			Lines: fmt.Sprintf("%d-%d", p.StartLine, p.EndLine),
			State: state,
		})
	}

	src.Unfetchable = predict(files, c.gates, c.Rules)

	stale, err := staleParts(dir, filepath.Base(path), planned)
	if err != nil {
		return Source{}, err
	}
	src.Stale = stale
	return src, nil
}

// plan runs the planner, turning its refusal into a message.
//
// A refusal is a finding on the source rather than an error for the run: the
// other sources are unaffected, and stopping at the first would hide how many
// share the problem.
func (c *Config) plan(data []byte, source string, limit int) (*split.Plan, string) {
	p, err := split.Split(data, split.Options{Source: source, Limit: limit, Level: c.Level})
	if err != nil {
		return nil, err.Error()
	}
	return p, ""
}

// place writes content at path when nothing is there, and reports what it found
// otherwise.
func (c *Config) place(path string, content []byte) (string, error) {
	existing, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(existing, content):
		return stateUnchanged, nil
	case err == nil:
		return stateConflict, nil
	case !errors.Is(err, os.ErrNotExist):
		return "", toerr.WrapWithMessage(err, "split: read "+path)
	}
	if c.DryRun {
		return stateCreated, nil
	}
	if err := atomicfile.WriteFile(path, content, 0o644); err != nil {
		return "", toerr.WrapWithMessage(err, "split: write "+path)
	}
	return stateCreated, nil
}

// staleParts lists files in dir that an earlier split of source produced and this
// plan does not: the same stem, and frontmatter naming the same source.
//
// The frontmatter test is what keeps a hand-written file that happens to share the
// prefix from being reported. Nothing is deleted; a stale part may be the one a
// claim already quotes.
func staleParts(dir, source string, planned map[string]bool) ([]string, error) {
	stem := strings.TrimSuffix(source, ".md")
	matches, err := filepath.Glob(filepath.Join(dir, stem+"-*.md"))
	if err != nil {
		return nil, toerr.WrapWithMessage(err, "split: list "+dir)
	}
	marker := []byte("gnosis_split:\n  source: " + source + "\n")
	var stale []string
	for _, m := range matches {
		if planned[filepath.Base(m)] {
			continue
		}
		data, err := os.ReadFile(m)
		if err != nil {
			return nil, toerr.WrapWithMessage(err, "split: read "+m)
		}
		if bytes.HasPrefix(data, []byte("---\n")) && bytes.Contains(data, marker) {
			stale = append(stale, m)
		}
	}
	sort.Strings(stale)
	return stale, nil
}

// findings counts what a person must look at: refusals, conflicts, stale parts,
// and parts still over the limit.
func (r *Result) findings() int {
	n := 0
	for i := range r.Sources {
		s := &r.Sources[i]
		if s.Refused != "" {
			n++
		}
		n += len(s.Stale) + len(s.Oversize) + len(s.Unfetchable) + len(s.Suspects)
		for _, p := range s.Parts {
			if p.State == stateConflict {
				n++
			}
		}
	}
	return n
}

// report renders the outcome. Findings exit non-zero, as fetch's do, so a script
// splitting a directory of books can tell a clean run from one needing attention.
func (c *Config) report(result *Result) error {
	n := result.findings()
	if c.JSONL {
		var err error
		if n == 0 {
			err = c.EmitOK(result)
		} else {
			err = c.EmitFindings(root.ReasonNeedsHuman,
				fmt.Sprintf("%d split finding(s) need a decision", n), result)
		}
		if err != nil {
			return fmt.Errorf("split: %w", err)
		}
	} else {
		c.print(result)
	}
	if n > 0 {
		return toerr.Wrap(root.ExitError(root.CodeFindings))
	}
	return nil
}

func (c *Config) print(result *Result) {
	for i := range result.Sources {
		s := &result.Sources[i]
		switch {
		case s.Refused != "":
			_, _ = fmt.Fprintf(c.Stdout, "refused  %s\n    %s\n", s.Path, s.Refused)
			c.printSuspects(s.Suspects)
			continue
		case s.Fits:
			_, _ = fmt.Fprintf(c.Stdout, "fits     %s (%d bytes, limit %d): nothing to split\n",
				s.Path, s.Bytes, result.Limit)
			c.printUnfetchable(s.Unfetchable)
			continue
		}
		_, _ = fmt.Fprintf(c.Stdout, "split    %s into %d parts\n", s.Path, len(s.Parts))
		for _, p := range s.Parts {
			_, _ = fmt.Fprintf(c.Stdout, "  %-9s %7d  lines %-13s %s\n",
				p.State, p.Bytes, p.Lines, filepath.Base(p.Path))
		}
		if s.Relinked > 0 || s.Unresolved > 0 {
			_, _ = fmt.Fprintf(c.Stdout,
				"  links     %d pointed at the part holding their heading; "+
					"%d naming no heading left as written\n", s.Relinked, s.Unresolved)
		}
		for _, o := range s.Oversize {
			_, _ = fmt.Fprintf(c.Stdout, "  oversize  %s: no heading left to cut at\n", o)
		}
		c.printUnfetchable(s.Unfetchable)
		c.printSuspects(s.Suspects)
		for _, st := range s.Stale {
			_, _ = fmt.Fprintf(c.Stdout, "  stale     %s: from an earlier split, not deleted\n", st)
		}
	}
	if result.DryRun {
		_, _ = fmt.Fprintln(c.Stdout, "dry run: nothing was written")
	}
}

// printUnfetchable names each file fetch will refuse, with fetch's own findings, so
// the source can be fixed before anything is fetched.
func (c *Config) printUnfetchable(files []Unfetchable) {
	for _, r := range files {
		_, _ = fmt.Fprintf(c.Stdout, "  unfetchable %s: fetch will refuse it (%s)\n",
			filepath.Base(r.Path), r.Reason)
		for _, f := range r.Findings {
			_, _ = fmt.Fprintf(c.Stdout, "      %s\n", f)
		}
	}
}

// printSuspects names each heading the source was cut at that reads as something
// else, with its line, so the source can be corrected before it is split again.
func (c *Config) printSuspects(suspects []split.Suspect) {
	for _, sp := range suspects {
		_, _ = fmt.Fprintf(c.Stdout, "  suspect   line %d %q reads as %s\n",
			sp.Line, sp.Heading, sp.Reason)
	}
}

func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("split: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("split: %w", c.Usage(cause))
}
