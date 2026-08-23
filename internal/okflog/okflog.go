// Package okflog reads and writes log.md, the corpus's own history (OKF §9).
//
// # Why this is not the audit trail
//
// gnosis keeps two records of what happened and they are not two renderings of one
// thing. §10.7.4 draws the line: **decisions are committed, observations are
// cached.**
//
//	              log.md                      audit.jsonl
//	what     what the corpus decided     what this process did
//	written  by a person, or by a        automatically, on every write
//	         command that changed policy
//	tier     1 — committed, merged       3 — per-user, gitignored
//	audience a colleague reading         whoever is debugging, or asking who
//	         history in six months
//
// A colleague pulling the repository needs to know that the per-file cap was
// raised and why; they do not need to know that this laptop rebuilt its index
// eleven times. Merging the second into git would produce a conflict on every pull
// and tell nobody anything.
//
// §6.2 is the case that makes log.md load-bearing rather than decorative: a
// threshold moved in the finding-reducing direction MUST be recorded here with the
// finding count before and after. Git already has the diff; what git cannot show
// is whether a threshold was wrong or merely inconvenient, and that is exactly
// what nobody reconstructs a year later.
//
// Everything here is pure. The caller supplies the date.
package okflog

import (
	"strings"
	"time"
)

// dateLayout is OKF §9's heading form.
const dateLayout = "2006-01-02"

// Entry is one dated section of the log.
type Entry struct {
	// Date is the section's heading date.
	Date string

	// Lines are the entry's body, without the heading and without the blank line
	// that follows it.
	Lines []string
}

// Parse splits a log into its dated sections.
//
// Requires: nothing; an empty log yields no entries.
// Ensures: entries in the order they appear. Content before the first date
// heading — a title, a note about the file — is preserved by Render and is not an
// entry, because it is not dated and OKF §9's form is what makes an entry an entry.
//
// A heading that is not a date is kept as body text rather than rejected. Refusing
// would make this function fail on a log the `log-format` check is already
// designed to report, and a linter and a parser disagreeing about the same file is
// worse than either being strict.
func Parse(src string) (preamble []string, entries []Entry) {
	var current *Entry
	for _, line := range strings.Split(src, "\n") {
		date, ok := headingDate(line)
		if ok {
			entries = append(entries, Entry{Date: date})
			current = &entries[len(entries)-1]
			continue
		}
		if current == nil {
			preamble = append(preamble, line)
			continue
		}
		current.Lines = append(current.Lines, line)
	}
	return preamble, entries
}

// headingDate reports the date of an OKF §9 entry heading.
func headingDate(line string) (string, bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(line), "## ")
	if !found {
		return "", false
	}
	rest = strings.TrimSpace(rest)
	if _, err := time.Parse(dateLayout, rest); err != nil {
		return "", false
	}
	return rest, true
}

// Add inserts a note under the given date, creating the section if absent.
//
// Requires: on is the date to file under; note is one line.
// Ensures: pure — the same log and note always produce the same result, so a
// caller previewing an entry sees what will be written. A new section is inserted
// **newest first**, because a log read top-down should begin with what just
// happened; an existing section gains the note at its end, because within one day
// the order events happened in is the order worth keeping.
//
// The date is a parameter rather than read from a clock here. That keeps this
// package pure, and it also means a caller backfilling an entry for last Tuesday
// does not have to fight the function.
func Add(src, on, note string) string {
	preamble, entries := Parse(src)

	for i := range entries {
		if entries[i].Date == on {
			entries[i].Lines = appendNote(entries[i].Lines, note)
			return Render(preamble, entries)
		}
	}
	fresh := Entry{Date: on, Lines: []string{"", note}}
	return Render(preamble, append([]Entry{fresh}, entries...))
}

// appendNote adds a line at the end of a section's body, keeping exactly one blank
// line before it when the body already has content.
func appendNote(lines []string, note string) []string {
	trimmed := lines
	for len(trimmed) > 0 && strings.TrimSpace(trimmed[len(trimmed)-1]) == "" {
		trimmed = trimmed[:len(trimmed)-1]
	}
	if len(trimmed) == 0 {
		return []string{"", note}
	}
	return append(trimmed, note)
}

// Render writes a log back out.
//
// Requires: nothing.
// Ensures: exactly one blank line between sections and a single trailing newline,
// so two runs over one log produce byte-identical output and a diff shows only
// what changed.
func Render(preamble []string, entries []Entry) string {
	var b strings.Builder

	writeTrimmed(&b, preamble)
	for i := range entries {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("## ")
		b.WriteString(entries[i].Date)
		b.WriteString("\n")
		writeTrimmed(&b, entries[i].Lines)
	}
	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		return ""
	}
	return out + "\n"
}

// writeTrimmed writes lines with trailing blanks removed, so rendering does not
// accumulate empty lines each time a log is round-tripped.
func writeTrimmed(b *strings.Builder, lines []string) {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	for _, line := range lines[:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}
}

// Since returns the entries dated on or after a date.
//
// Requires: on is in the OKF §9 form, or empty for all entries.
// Ensures: string comparison rather than date parsing, which is correct for
// ISO-8601 and is why OKF chose that form. An entry whose heading is not a date is
// not an entry at all and never appears here.
func Since(entries []Entry, on string) []Entry {
	if on == "" {
		return entries
	}
	out := make([]Entry, 0, len(entries))
	for i := range entries {
		if entries[i].Date >= on {
			out = append(out, entries[i])
		}
	}
	return out
}
