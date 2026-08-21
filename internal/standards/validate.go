package standards

import (
	"sort"
	"strings"

	"github.com/StevenACoffman/skillet/errs"
)

// validate rejects gates outside the range in which they mean anything.
//
// The checks here are deliberately weak: they catch a value that cannot be
// intended, not one that is unwise. Whether 256 KiB is the right cap is a
// judgment the rationale records and a reviewer makes; whether a cap of zero is
// meant is not, because it would archive nothing and report no error.
//
// Requires: a is decoded.
// Ensures: returns EINVALID naming every problem found, sorted, so one run
// surfaces the whole edit.
func (a *Archive) validate(op string) error {
	var bad []string
	positive := map[string]int64{
		"per_file_cap":         a.PerFileCap.Value,
		"corpus_budget":        a.CorpusBudget.Value,
		"embedded_payload_cap": a.EmbeddedPayloadCap.Value,
		"staleness_days":       int64(a.StalenessDays.Value),
	}
	for name, v := range positive {
		if v <= 0 {
			bad = append(bad, name+" must be positive")
		}
	}
	if f := a.CorpusWarnFraction.Value; f <= 0 || f > 1 {
		bad = append(bad, "corpus_warn_fraction must be in (0, 1]")
	}
	if a.InDegreeCut.Value < 0 {
		bad = append(bad, "in_degree_cut must not be negative")
	}
	if a.PerFileCap.Value > a.CorpusBudget.Value {
		bad = append(bad, "per_file_cap exceeds corpus_budget, so one file could exhaust it")
	}
	bad = append(bad, a.badExtensions()...)
	if strings.TrimSpace(a.HTMLExtractor.Value) == "" {
		bad = append(bad, "html_extractor must name the pinned extractor")
	}
	// An unversioned extractor is worse than none: every extracted record would
	// claim a provenance it cannot distinguish from the next release's.
	if strings.TrimSpace(a.HTMLExtractorVersion.Value) == "" {
		bad = append(bad, "html_extractor_version must pin a version")
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	return &errs.Error{Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; ")}
}

// badExtensions reports allowlist entries that cannot match a filename.
//
// An entry missing its leading dot is the common typo and the expensive one: the
// suffix test would never fire, so every source of that kind falls silently to
// `referenced` and the corpus loses durable evidence without reporting anything.
func (a *Archive) badExtensions() []string {
	if len(a.Allowlist.Value) == 0 {
		return []string{"allowlist is empty, so nothing could ever be archived"}
	}
	var bad []string
	seen := map[string]bool{}
	for _, ext := range a.Allowlist.Value {
		switch {
		case !strings.HasPrefix(ext, "."):
			bad = append(bad, "allowlist entry "+quote(ext)+" has no leading dot")
		case len(ext) == 1:
			bad = append(bad, "allowlist entry \".\" names no extension")
		case ext != strings.ToLower(ext):
			// Matching is lower-cased, so an upper-case entry is dead weight that
			// reads as though it covers a case it does not.
			bad = append(bad, "allowlist entry "+quote(ext)+" must be lower-case")
		case seen[ext]:
			bad = append(bad, "allowlist entry "+quote(ext)+" is listed twice")
		}
		seen[ext] = true
	}
	return bad
}

func quote(s string) string { return `"` + s + `"` }
