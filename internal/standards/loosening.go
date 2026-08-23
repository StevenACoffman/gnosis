package standards

import (
	"sort"
	"strconv"
	"strings"
)

// Loosening is one gate that moved in the direction that admits more and reports
// less.
//
// It is not a finding. A loosening is frequently correct — the first cap anybody
// picks is a guess, and the corpus is what corrects it. What it is not is
// invisible: SPEC §6.2 requires that a threshold moved in the finding-reducing
// direction be recorded in log.md with the finding count before and after, and a
// count nobody was asked for is a count nobody produces.
type Loosening struct {
	// Key is the gate's name as it appears in the file.
	Key string `json:"key"`

	// From and To are the old and new values, rendered.
	From string `json:"from"`
	To   string `json:"to"`

	// Rationale is the justification recorded for the new value. It is carried
	// here so the report reads without a second lookup, and so a loosening whose
	// rationale did not change is visible as such.
	Rationale string `json:"rationale"`
}

// CompareArchive reports which gates old → new moved in the loosening direction.
//
// Requires: both are loaded.
// Ensures: the result is sorted by key and is empty when nothing loosened. Gates
// that tightened, and gates that changed only their rationale, are absent: this
// answers one question, and a report that also listed tightenings would bury it.
//
// The direction each gate loosens in is stated here, in Go, and not in the file.
// Were it declared alongside the value, concealing a loosening would take nothing
// more than flipping that field in the same commit.
func CompareArchive(old, cur *Archive) []Loosening {
	if old == nil || cur == nil {
		return nil
	}
	var out []Loosening
	add := func(loosened bool, key, from, to, why string) {
		if loosened {
			out = append(out, Loosening{Key: key, From: from, To: to, Rationale: why})
		}
	}

	// Bigger caps admit more, so bigger is looser.
	for _, g := range []struct {
		key      string
		old, cur Value[int64]
	}{
		{"per_file_cap", old.PerFileCap, cur.PerFileCap},
		{"corpus_budget", old.CorpusBudget, cur.CorpusBudget},
		{"embedded_payload_cap", old.EmbeddedPayloadCap, cur.EmbeddedPayloadCap},
	} {
		add(g.cur.Value > g.old.Value, g.key,
			strconv.FormatInt(g.old.Value, 10), strconv.FormatInt(g.cur.Value, 10),
			g.cur.Rationale)
	}

	// A longer staleness window reports fewer stale sources; a higher warning
	// fraction warns later; a higher in-degree cut makes fewer documents central
	// and so demands stronger evidence of fewer claims.
	add(cur.StalenessDays.Value > old.StalenessDays.Value, "staleness_days",
		strconv.Itoa(old.StalenessDays.Value), strconv.Itoa(cur.StalenessDays.Value),
		cur.StalenessDays.Rationale)
	add(cur.InDegreeCut.Value > old.InDegreeCut.Value, "in_degree_cut",
		strconv.Itoa(old.InDegreeCut.Value), strconv.Itoa(cur.InDegreeCut.Value),
		cur.InDegreeCut.Rationale)
	add(cur.CorpusWarnFraction.Value > old.CorpusWarnFraction.Value, "corpus_warn_fraction",
		formatFraction(old.CorpusWarnFraction.Value), formatFraction(cur.CorpusWarnFraction.Value),
		cur.CorpusWarnFraction.Rationale)

	// An added extension archives sources that previously fell to `referenced`.
	// A removed one is a tightening and is not reported here.
	if added := addedExtensions(old.Allowlist.Value, cur.Allowlist.Value); len(added) > 0 {
		out = append(out, Loosening{
			Key:       "allowlist",
			From:      strings.Join(sorted(old.Allowlist.Value), " "),
			To:        strings.Join(sorted(cur.Allowlist.Value), " "),
			Rationale: cur.Allowlist.Rationale,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// addedExtensions returns the entries present in cur and absent from old.
func addedExtensions(old, cur []string) []string {
	had := make(map[string]bool, len(old))
	for _, e := range old {
		had[e] = true
	}
	var added []string
	for _, e := range cur {
		if !had[e] {
			added = append(added, e)
		}
	}
	return added
}

// sorted returns a sorted copy, so rendering a list never reorders the caller's.
func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// formatFraction renders without an exponent or trailing zeros, so 0.8 reads as
// "0.8" in a log line a person is meant to check.
func formatFraction(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
