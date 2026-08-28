package lint

import (
	"sort"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/schema"
	"github.com/StevenACoffman/skillet/finding"
)

// commandCheck reports a command named in the schema document that does not resolve.
//
// **It is about the prose, not the generated region, and that is the whole reason it is
// not redundant.** `gnosis schema` writes the command region *from* the registry, so a
// name inside it resolves by construction, and a region that has fallen behind is
// already `gnosis schema --check`'s finding. What nothing checks is the part §5.7.1
// guarantees gnosis never touches: an agent-facing document's own prose, where somebody
// wrote "run `gnosis frobnicate`" and the command was later renamed or never existed.
//
// That is the failure §5.7 exists to prevent, one level in. The document is read by
// agents, an agent does what it is told, and an instruction naming a command that does
// not resolve produces a failure the agent cannot diagnose — it will assume its own
// invocation was wrong.
//
// Every finding is therefore in somebody's prose, because the generated region cannot
// produce one.
func commandCheck() Check {
	return Check{
		Name:       "command",
		Categories: []string{"command"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    schemaDocNamesACommand,
		Run:        unresolvedCommands,
	}
}

// schemaDocNamesACommand reports whether the schema document mentions any command.
//
// Requires: nothing.
// Ensures: a reason whenever it declines. Pure.
func schemaDocNamesACommand(snap *Snapshot) (bool, string) {
	switch {
	case len(snap.Commands) == 0:
		return false, "the caller supplied no command list, so nothing can be resolved" +
			" against it"
	case strings.TrimSpace(snap.SchemaDoc) == "":
		return false, "the bundle has no AGENTS.md; `gnosis schema` writes one"
	case len(mentionedCommands(snap.SchemaDoc)) == 0:
		return false, "AGENTS.md names no gnosis command outside its generated regions"
	default:
		return true, ""
	}
}

// unresolvedCommands reports each command named in the prose that is not registered.
func unresolvedCommands(snap *Snapshot) []finding.Diagnostic {
	registered := map[string]bool{}
	for _, c := range snap.Commands {
		registered[c] = true
	}

	var unknown []string
	for _, name := range mentionedCommands(snap.SchemaDoc) {
		if !registered[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return []finding.Diagnostic{}
	}
	sort.Strings(unknown)

	// One finding naming them all: a document with three stale instructions has one
	// problem — it is out of date — and three findings would make the report about the
	// words rather than about the document nobody has revisited.
	return []finding.Diagnostic{{
		Severity: finding.SeverityWarning,
		Category: "command",
		Path:     "AGENTS.md",
		// Phrased so the count sits in a noun phrase and no verb has to agree with
		// it. "names 1 command that do not resolve" was the third subject-verb
		// disagreement of this session, after "1 document declare" and "1 claim name"
		// — a class no substring assertion sees and one run of the command shows.
		Message: "names " + noun(len(unknown), "unresolvable command") + ": " +
			strings.Join(unknown, ", ") +
			" — this is your own prose rather than a generated region, so `gnosis" +
			" schema` will not correct it. An agent reading this document will run" +
			" what it is told and cannot tell your instruction from its own mistake",
		Action: finding.ActionHuman,
	}}
}

// mentionedCommands finds every `gnosis <name>` written in code spans outside the
// generated regions.
//
// Requires: nothing.
// Ensures: names only, deduplicated, in document order. The generated regions are
// removed first, so a name gnosis wrote is never reported against gnosis. Pure.
func mentionedCommands(doc string) []string {
	prose := doc
	for _, region := range []string{schema.RegionCommands, schema.RegionVocabulary} {
		if body, ok := schema.RegionBody(prose, region); ok {
			prose = strings.Replace(prose, body, "", 1)
		}
	}

	seen := map[string]bool{}
	var out []string
	for _, span := range codeSpans(prose) {
		name, ok := commandName(span)
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// codeSpans returns the contents of every backtick span in text.
//
// A backtick scan rather than a Markdown parse: `skillet/markdown` reports code spans
// but mixes them with link destinations, and this needs the spans alone. Scanning for a
// delimiter that cannot nest is a smaller thing to get right than filtering a list that
// answers a different question.
func codeSpans(text string) []string {
	var out []string
	parts := strings.Split(text, "`")
	// Odd indices are inside a span: the split alternates outside, inside, outside.
	for i := 1; i < len(parts); i += 2 {
		out = append(out, parts[i])
	}
	return out
}

// commandName reports the subcommand a span names, if it names one.
//
// Requires: nothing.
// Ensures: false for anything that is not `gnosis <word>` — including bare `gnosis`,
// which names the binary rather than a command. Pure.
func commandName(span string) (string, bool) {
	fields := strings.Fields(span)
	if len(fields) < 2 || fields[0] != "gnosis" {
		return "", false
	}
	name := fields[1]
	if strings.HasPrefix(name, "-") {
		// `gnosis --jsonl` is a flag on the root, not a subcommand.
		return "", false
	}
	return name, true
}
