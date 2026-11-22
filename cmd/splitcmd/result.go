package splitcmd

import (
	"fmt"
	"path/filepath"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/split"
	toerr "github.com/StevenACoffman/toerr/errors"
)

// Source is what became of one source file.
type Source struct {
	Path       string     `json:"path"`
	SHA256     string     `json:"sha256,omitempty"`
	Bytes      int        `json:"bytes"`
	Parts      []Part     `json:"parts,omitempty"`
	Fits       bool       `json:"fits,omitempty"`
	Refused    string     `json:"refused,omitempty"`
	Oversize   []string   `json:"oversize,omitempty"`
	Relinked   int        `json:"relinked,omitempty"`
	Unresolved int        `json:"unresolved,omitempty"`
	Leftovers  []Leftover `json:"leftovers,omitempty"`

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

	// WrittenBy is the splitter version that wrote a conflicting file, or 0 when
	// the file is not a part split wrote.
	WrittenBy int `json:"written_by,omitempty"`
}

// Leftover is a part an earlier split of the same source wrote that this split
// does not plan, and whether --replace removed it.
type Leftover struct {
	Path      string `json:"path"`
	WrittenBy int    `json:"written_by"`
	Removed   bool   `json:"removed,omitempty"`
}

// Result is the whole run.
type Result struct {
	Limit   int      `json:"limit"`
	DryRun  bool     `json:"dry_run"`
	Sources []Source `json:"sources"`
}

// findings counts what a person must look at: refusals, conflicts, leftovers,
// and parts still over the limit.
func (r *Result) findings() int {
	n := 0
	for i := range r.Sources {
		s := &r.Sources[i]
		if s.Refused != "" {
			n++
		}
		n += len(s.Oversize) + len(s.Unfetchable) + len(s.Suspects)
		for _, l := range s.Leftovers {
			if !l.Removed {
				n++
			}
		}
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
			_, _ = fmt.Fprintf(c.Stdout, "  %-9s %7d  lines %-13s %s%s\n",
				p.State, p.Bytes, p.Lines, filepath.Base(p.Path), writtenBy(p.WrittenBy))
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
		c.printLeftovers(s.Leftovers)
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

// printLeftovers names each part an earlier split left that this one does not plan.
func (c *Config) printLeftovers(left []Leftover) {
	for _, l := range left {
		what := "leftover  %s: from an earlier split, kept%s\n"
		if l.Removed {
			what = "removed   %s: from an earlier split%s\n"
		}
		_, _ = fmt.Fprintf(c.Stdout, "  "+what, filepath.Base(l.Path), writtenBy(l.WrittenBy))
	}
}

// writtenBy says which splitter wrote a file this run will not touch, and how to
// regenerate it — or nothing for a file split did not write.
func writtenBy(version int) string {
	if version == 0 {
		return ""
	}
	return fmt.Sprintf(" (written by splitter v%d; --replace regenerates it)", version)
}

// printSuspects names each heading the source was cut at that reads as something
// else, with its line, so the source can be corrected before it is split again.
func (c *Config) printSuspects(suspects []split.Suspect) {
	for _, sp := range suspects {
		_, _ = fmt.Fprintf(c.Stdout, "  suspect   line %d %q reads as %s\n",
			sp.Line, sp.Heading, sp.Reason)
	}
}
