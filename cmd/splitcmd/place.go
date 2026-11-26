package splitcmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/split"
	"github.com/StevenACoffman/skillet/atomicfile"
	toerr "github.com/StevenACoffman/toerr/errors"
)

// decide picks what happens to one planned part, given what is on disk at its path.
//
// Requires: exists reports whether a file is at the part's path, and existing is
// its content when it does.
// Ensures: created when nothing is there; unchanged when the bytes already match;
// replaced when replace is set and the file is a part of source; conflict
// otherwise. Pure.
//
// A file is replaced only when its own frontmatter says split cut it from this
// source. A hand-written file, or a part of some other source that happens to share
// the name, is never overwritten, --replace or not.
func decide(planned, existing []byte, exists bool, source string, replace bool) string {
	switch {
	case !exists:
		return stateCreated
	case bytes.Equal(existing, planned):
		return stateUnchanged
	}
	if prov, ok := split.ReadProvenance(existing); ok && prov.Source == source && replace {
		return stateReplaced
	}
	return stateConflict
}

// place carries out decide's choice for one part and reports it, with the splitter
// version that wrote a file it would not overwrite.
func (c *Config) place(path, source string, content []byte) (Part, error) {
	existing, err := os.ReadFile(path)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Part{}, toerr.WrapWithMessage(err, "split: read "+path)
	}

	part := Part{Path: path, Bytes: len(content)}
	part.State = decide(content, existing, exists, source, c.Replace)
	switch part.State {
	case stateCreated, stateReplaced:
		if c.DryRun {
			return part, nil
		}
		if err := atomicfile.WriteFile(path, content, 0o644); err != nil {
			return Part{}, toerr.WrapWithMessage(err, "split: write "+path)
		}
	case stateConflict:
		if prov, ok := split.ReadProvenance(existing); ok {
			part.WrittenBy = prov.Version
		}
	}
	return part, nil
}

// leftovers lists files in dir that an earlier split of source produced and this
// plan does not.
//
// A file counts only when its frontmatter says split cut it from this source, so a
// hand-written file sharing the prefix is never reported, and never removed.
func leftovers(dir, source string, planned map[string]bool) ([]Leftover, error) {
	stem := strings.TrimSuffix(source, ".md")
	matches, err := filepath.Glob(filepath.Join(dir, stem+"-*.md"))
	if err != nil {
		return nil, toerr.WrapWithMessage(err, "split: list "+dir)
	}
	sort.Strings(matches)
	var out []Leftover
	for _, m := range matches {
		if planned[filepath.Base(m)] {
			continue
		}
		data, err := os.ReadFile(m)
		if err != nil {
			return nil, toerr.WrapWithMessage(err, "split: read "+m)
		}
		if prov, ok := split.ReadProvenance(data); ok && prov.Source == source {
			out = append(out, Leftover{Path: m, WrittenBy: prov.Version})
		}
	}
	return out, nil
}

// removeLeftovers deletes the leftovers under --replace, returning them marked removed.
//
// Without --replace nothing is deleted, because a leftover part may be the one a
// claim already quotes; with it, the caller has said the new split supersedes the
// old. A dry run marks them without deleting, so its report matches the real run.
func (c *Config) removeLeftovers(left []Leftover) ([]Leftover, error) {
	if !c.Replace {
		return left, nil
	}
	out := make([]Leftover, 0, len(left))
	for _, l := range left {
		if !c.DryRun {
			if err := os.Remove(l.Path); err != nil {
				return nil, toerr.WrapWithMessage(err, "split: remove "+l.Path)
			}
		}
		l.Removed = true
		out = append(out, l)
	}
	return out, nil
}
