// Package splitcmd implements the "split" CLI command.
package splitcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/split"
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
    recorded link repairs and its navigation footer;
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

Each part ends with one plain paragraph linking the parts before and after it,
"Previous: [heading](part.md) · Next: [heading](part.md)", either side omitted at
the ends of the book. It holds no source text, so it is outside the verbatim and
rendering checks, and it is checked to link exactly those neighbours.

--min-bytes N joins a section smaller than N bytes to its neighbour when the two
fit under the limit, for a source whose conversion flattened its headings into
dozens of near-empty sections. It is off by default, and recorded in each part
when set.

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
file that exists with different bytes is a conflict and is not overwritten, and a
part left over from an earlier split of the same source is reported as a leftover
and kept; both are findings, and each names the splitter version that wrote it.
--replace regenerates instead: it overwrites a conflicting file and removes a leftover, but only when the file's own frontmatter says split cut it from this source. A
hand-written file is never touched.

Beside the parts split writes <stem>-00-index.md, the book's entry point: a relative
link to each part in reading order, named for its heading. It is regenerated on every
run and never fetched, and its gnosis_split_index frontmatter is what canonizer's
distill skips, so the book is not distilled a second time through its links.

--originals DIR moves each source split into parts to DIR, at the path it has
relative to --sources (the working directory by default), creating directories as
needed, so whatever reads the sources meets each book once, as its parts. A source
that fits is never moved. The original stays put while any part is in conflict,
and is never written over a different file already at its destination. A source
already under --originals is split back into the matching directory under
--sources, so a moved original can be split again where it is.

split writes beside its sources, not into the bundle, so it takes no writer lock
and leaves no audit row. --bundle is read only for the cap.`

// Part states. A part is written only when it is absent; one that exists with
// other bytes is reported, never overwritten.
const (
	stateCreated   = "created"
	stateUnchanged = "unchanged"
	stateReplaced  = "replaced"
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

	// MinBytes joins a section smaller than this to its neighbour when the two fit
	// under the limit. Zero, the default, keeps every section its own part.
	MinBytes int

	// Replace overwrites a part an earlier split of the same source wrote, and
	// removes one this split no longer plans. Files split did not write are never
	// touched.
	Replace bool

	// Strict refuses a source with a suspect cut heading, writing nothing for it,
	// instead of writing its parts and reporting the suspects.
	Strict bool

	// Originals is where a split source's original moves once its parts are
	// written, at the same path relative to it that the source has relative to
	// Sources, so a reader of the sources meets each book once. Empty leaves
	// originals beside their parts.
	Originals string

	// Sources is the directory source paths are taken relative to when Originals is
	// set. It defaults to the working directory.
	Sources string

	Flags   *ff.FlagSet
	Command *ff.Command

	// gates are fetch's admission gates for this bundle, loaded once per run and
	// read by every source's prediction.
	gates archive.Gates
}

// New registers the split command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("split").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.DryRun, 'n', "dry-run", "plan and compare without writing")
	cfg.Flags.IntVar(&cfg.MaxBytes, 0, "max-bytes", 0,
		"largest part in bytes; defaults to, and may not exceed, the archive's per-file cap")
	cfg.Flags.BoolVar(&cfg.Replace, 0, "replace",
		"overwrite or remove parts an earlier split of the same source wrote")
	cfg.Flags.BoolVar(&cfg.Strict, 0, "strict",
		"refuse a source whose split would cut at a heading that reads as code or a label")
	cfg.Flags.IntVar(&cfg.MinBytes, 0, "min-bytes", 0,
		"join a section smaller than this to its neighbour when both fit (0 keeps every section)")
	cfg.Flags.IntVar(&cfg.Level, 0, "level", 0,
		"cut at headings of this level (1-6) even where fewer parts would fit")
	cfg.Flags.StringVar(&cfg.Originals, 0, "originals", "",
		"move each split source's original here, at its path relative to --sources")
	cfg.Flags.StringVar(&cfg.Sources, 0, "sources", ".",
		"directory source paths are relative to, for --originals")
	cfg.Command = &ff.Command{
		Name: "split",
		Usage: "gnosis split [--dry-run] [--max-bytes N] [--level N] " +
			"[--originals DIR [--sources DIR]] <FILE.md>...",
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
	if c.MinBytes < 0 || c.MinBytes >= limit {
		return c.usage(fmt.Errorf("--min-bytes must be at least 0 and below the part limit "+
			"of %d, got %d", limit, c.MinBytes))
	}

	roots, err := c.roots()
	if err != nil {
		return c.usage(err)
	}

	result := &Result{Limit: limit, DryRun: c.DryRun}
	for _, path := range args {
		src, err := c.splitOne(path, limit, roots)
		if err != nil {
			return c.fail(root.ReasonFetchFailed, err)
		}
		result.Sources = append(result.Sources, src)
	}
	return c.report(result)
}

// roots resolves --sources and --originals to absolute paths, so locate can compare
// them with any source path however it was written.
func (c *Config) roots() (placementRoots, error) {
	if c.Originals == "" {
		return placementRoots{}, nil
	}
	sources, err := filepath.Abs(c.Sources)
	if err != nil {
		return placementRoots{}, toerr.WrapWithMessage(err, "resolve --sources")
	}
	originals, err := filepath.Abs(c.Originals)
	if err != nil {
		return placementRoots{}, toerr.WrapWithMessage(err, "resolve --originals")
	}
	if _, inside := within(originals, sources); inside || sources == originals {
		return placementRoots{}, fmt.Errorf("--sources %s must not be inside --originals %s",
			c.Sources, c.Originals)
	}
	return placementRoots{sources: sources, originals: originals}, nil
}

// splitOne plans one source, applies the plan, and moves the original once split.
func (c *Config) splitOne(path string, limit int, roots placementRoots) (Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Source{}, toerr.WrapWithMessage(err, "split: read "+path)
	}
	src := Source{Path: path, Bytes: len(data)}

	abs, err := filepath.Abs(path)
	if err != nil {
		return Source{}, toerr.WrapWithMessage(err, "split: resolve "+path)
	}
	where, refused := locate(abs, roots.sources, roots.originals)
	if refused != "" {
		src.Refused = refused
		return src, nil
	}

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

	if err := c.writeParts(&src, plan, where.partsDir, filepath.Base(path)); err != nil {
		return Source{}, err
	}
	if where.moveTo != "" {
		o, err := c.moveOriginal(path, where.moveTo, data, src.conflicts())
		if err != nil {
			return Source{}, err
		}
		src.Original = &o
	}
	return src, nil
}

// writeParts places plan's parts and its index in dir, predicts what fetch will make
// of the parts, and reports or removes what an earlier split of source left there.
//
// The index is planned like a part, so an earlier index of this source is
// regenerated under --replace rather than mistaken for a leftover. It is not
// predicted: it holds none of the book, so nothing should fetch it.
func (c *Config) writeParts(src *Source, plan *split.Plan, dir, source string) error {
	planned := map[string]bool{}
	files := make([]plannedFile, 0, len(plan.Parts))
	for i := range plan.Parts {
		p := &plan.Parts[i]
		planned[p.Name] = true
		files = append(files, plannedFile{path: filepath.Join(dir, p.Name), content: p.Content})
		src.Relinked += p.Relinked
		src.Unresolved += len(p.Unresolved)
		part, err := c.place(filepath.Join(dir, p.Name), source, p.Content)
		if err != nil {
			return err
		}
		part.Lines = fmt.Sprintf("%d-%d", p.StartLine, p.EndLine)
		src.Parts = append(src.Parts, part)
	}
	planned[plan.Index.Name] = true
	index, err := c.place(filepath.Join(dir, plan.Index.Name), source, plan.Index.Content)
	if err != nil {
		return err
	}
	src.Index = &index

	src.Unfetchable = predict(files, c.gates, c.Rules)

	left, err := leftovers(dir, source, planned)
	if err != nil {
		return err
	}
	if left, err = c.removeLeftovers(left); err != nil {
		return err
	}
	src.Leftovers = left
	return nil
}

// plan runs the planner, turning its refusal into a message.
//
// A refusal is a finding on the source rather than an error for the run: the
// other sources are unaffected, and stopping at the first would hide how many
// share the problem.
func (c *Config) plan(data []byte, source string, limit int) (*split.Plan, string) {
	p, err := split.Split(data, split.Options{
		Source: source, Limit: limit, Level: c.Level, MinBytes: c.MinBytes,
	})
	if err != nil {
		return nil, err.Error()
	}
	return p, ""
}

func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("split: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("split: %w", c.Usage(cause))
}
