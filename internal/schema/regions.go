package schema

import (
	"sort"
	"strings"
)

// The region names. They are constants because they appear in every committed
// AGENTS.md as marker text, so renaming one orphans a region in every bundle: a
// reader's file would keep a region gnosis no longer generates and lose the one it
// does.
const (
	// RegionVocabulary is the corpus's declared `type` keys.
	RegionVocabulary = "vocabulary"

	// RegionCommands is what this binary can do.
	RegionCommands = "commands"

	// RegionIndex is the corpus's documents, grouped by type.
	//
	// It lives in `index.md` rather than the schema document because OKF §3.1
	// reserves that filename for the entry point a reader arrives at, and §8 makes
	// it progressive disclosure. The two documents answer different questions —
	// *what may I write* and *what is here* — and a reader arriving with a question
	// wants the second.
	RegionIndex = "index"
)

// TypeEntry is one declared type, reduced to what an agent needs to choose one.
type TypeEntry struct {
	Key  string
	Desc string

	// Deprecated is the replacement key when the type is soft-deprecated, and empty
	// otherwise.
	//
	// Rendered rather than omitted. §5.8.1's soft-deprecation is announce-then-
	// enforce, and a vocabulary listing that silently dropped a deprecated key would
	// enforce without announcing — an agent would find its documents rejected for
	// using a word the schema had quietly stopped mentioning.
	Deprecated string
}

// CommandEntry is one command and its one-line help.
type CommandEntry struct {
	Name string
	Help string
}

// DocEntry is one concept, reduced to what a reader needs to decide whether to open it.
type DocEntry struct {
	// Type groups the listing. Empty for a document declaring none, which groups
	// under a name saying so rather than silently.
	Type string

	// Title is the link text and Path is bundle-relative, as OKF §6.1 links are.
	Title string
	Path  string
}

// SchemaRegions renders the schema document's generated blocks.
//
// Requires: types are the corpus's declared types and commands are this binary's, both
// as read by the caller.
// Ensures: exactly two regions, vocabulary then commands, each sorted; pure.
//
// # What is generated, and what is refused
//
// The mechanical parts only: the type vocabulary and the command list. §5.7 states the
// restraint and the evidence for it — an ETH Zurich study found *auto-generated*
// context files reduced agent success in five of eight settings — so gnosis "never
// writes the prose that tells an agent how to think". A `workflow` region explaining
// how to ingest would be exactly the prose that study measured, and its absence is the
// feature rather than an omission.
//
// Both lists are sorted, because both come from maps or from a registration order that
// is an implementation detail. A file that reordered itself between two runs over one
// corpus would report drift against itself, which is the property `--check` rests on.
func SchemaRegions(types []TypeEntry, commands []CommandEntry) []Region {
	return []Region{
		{Name: RegionVocabulary, Body: vocabularyBody(types)},
		{Name: RegionCommands, Body: commandsBody(commands)},
	}
}

// IndexRegions renders the index document's generated block.
//
// Requires: docs are the corpus's concepts as read by the caller.
// Ensures: exactly one region; sorted by type then title, so two runs over one corpus
// produce identical bytes. Pure.
//
// **A separate function rather than a third parameter on SchemaRegions.** The two
// documents share the marker mechanism and nothing else: one is generated from the
// vocabulary and this binary, the other from the corpus. A single builder taking all
// three would make every caller pass a nil for the document it is not writing, which is
// a general mechanism carrying two documents' worth of special-purpose knowledge.
func IndexRegions(docs []DocEntry) []Region {
	return []Region{{Name: RegionIndex, Body: indexBody(docs)}}
}

// indexBody renders the corpus grouped by type.
//
// **An empty corpus says so rather than rendering nothing**, and the region is written
// either way. A suppressed region would mean the first `gnosis schema` after the first
// document appeared rewrote a file the reader thought was stable; an empty one reads as
// a generator that failed. Saying "this corpus holds no documents yet" is the same
// distinction `Gains.Any` makes — nothing yet and nobody looked must not render alike.
//
// Title and path only. A listing that carried every field would be the mirror the
// seeded prose warns against, and `gnosis search` is what answers questions about
// content.
func indexBody(docs []DocEntry) string {
	if len(docs) == 0 {
		return "This corpus holds no documents yet."
	}
	sorted := make([]DocEntry, len(docs))
	copy(sorted, docs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Type != sorted[j].Type {
			return sorted[i].Type < sorted[j].Type
		}
		return sorted[i].Title < sorted[j].Title
	})

	var b strings.Builder
	current, started := "", false
	for i := range sorted {
		d := &sorted[i]
		// `started` rather than `current != ""`, because the empty type is a real
		// group: comparing against "" made the first heading vanish exactly when the
		// group was the untyped one, so the documents that most need labelling were
		// the ones rendered bare. Visible on the first run over a real corpus and
		// invisible to a test asserting the entry appears.
		if !started || d.Type != current {
			if started {
				b.WriteString("\n")
			}
			// A type with no key still groups, under a name that says so rather than
			// under an empty heading: a document declaring no type is a conformance
			// finding, and hiding it here would make the listing disagree with `lint`.
			name := d.Type
			if name == "" {
				name = "(no type declared)"
			}
			b.WriteString("**")
			b.WriteString(name)
			b.WriteString("**\n\n")
			current, started = d.Type, true
		}
		b.WriteString("- [")
		b.WriteString(d.Title)
		b.WriteString("](/")
		b.WriteString(d.Path)
		b.WriteString(")\n")
	}
	return b.String()
}

// vocabularyBody renders the type list.
//
// An empty vocabulary says so rather than rendering an empty list. A corpus with no
// declared types is an ordinary state — `init` seeds a starter ontology, but a bundle
// whose file was deleted or failed to load has none — and a bare heading with nothing
// under it reads as a rendering bug rather than as an answer.
func vocabularyBody(types []TypeEntry) string {
	if len(types) == 0 {
		return "This corpus declares no types. `gnosis doctor` reports why."
	}
	sorted := make([]TypeEntry, len(types))
	copy(sorted, types)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	var b strings.Builder
	for i := range sorted {
		t := &sorted[i]
		b.WriteString("- `")
		b.WriteString(t.Key)
		b.WriteString("`")
		if t.Desc != "" {
			b.WriteString(" — ")
			b.WriteString(t.Desc)
		}
		if t.Deprecated != "" {
			// Named as deprecated *and* pointed at its replacement, because
			// "deprecated" alone tells an author to stop and not what to do instead.
			b.WriteString(" **(deprecated; use `")
			b.WriteString(t.Deprecated)
			b.WriteString("`)**")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// commandsBody renders the command list.
func commandsBody(commands []CommandEntry) string {
	if len(commands) == 0 {
		return "No commands were reported, which is a defect in gnosis rather than " +
			"in this corpus."
	}
	sorted := make([]CommandEntry, len(commands))
	copy(sorted, commands)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	var b strings.Builder
	for i := range sorted {
		c := &sorted[i]
		b.WriteString("- `gnosis ")
		b.WriteString(c.Name)
		b.WriteString("`")
		if c.Help != "" {
			b.WriteString(" — ")
			b.WriteString(c.Help)
		}
		b.WriteString("\n")
	}
	return b.String()
}
