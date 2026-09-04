package bundle

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/constraint"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/scan"
	"github.com/StevenACoffman/gnosis/internal/standards"
	"github.com/StevenACoffman/skillet/errs"
)

// LoadArchiveStandards reads a bundle's archive gates, falling back to the seed.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: a bundle with no standards file gets the embedded default rather than
// an error. That is not leniency: the seed is the corpus's starting policy, and a
// bundle cloned before `standards/` was introduced would otherwise be unfetchable
// until someone copied a file in. A file that *is* present and is malformed is a
// hard error, because that is somebody's edit and guessing what they meant would
// silently apply gates they did not write.
func LoadArchiveStandards(bundleDir string) (*standards.Archive, error) {
	return loadStandards(bundleDir, "bundle.LoadArchiveStandards",
		standards.ArchiveFileName, standards.DefaultArchive, standards.LoadArchive)
}

// LoadChallengeStandards reads a bundle's challenge window, falling back to the seed.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: the same rule as LoadArchiveStandards — an absent file gets the embedded
// default, and a present file that will not load is a hard error.
func LoadChallengeStandards(bundleDir string) (*standards.Challenge, error) {
	return loadStandards(bundleDir, "bundle.LoadChallengeStandards",
		standards.ChallengeFileName, standards.DefaultChallenge, standards.LoadChallenge)
}

// LoadPromoteStandards reads a bundle's gate thresholds, falling back to the seed.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: the same rule as LoadArchiveStandards — an absent file gets the
// embedded default, and a present file that will not load is a hard error. A
// bundle cloned before promote.toml existed must still promote; a bundle whose
// file is malformed must not silently promote against thresholds nobody wrote.
func LoadPromoteStandards(bundleDir string) (*standards.Promote, error) {
	return loadStandards(bundleDir, "bundle.LoadPromoteStandards",
		standards.PromoteFileName, standards.DefaultPromote, standards.LoadPromote)
}

// LoadSampleStandards reads a bundle's draw seed, falling back to the seed file.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: the same rule as the two above.
func LoadSampleStandards(bundleDir string) (*standards.Sample, error) {
	return loadStandards(bundleDir, "bundle.LoadSampleStandards",
		standards.SampleFileName, standards.DefaultSample, standards.LoadSample)
}

// LoadRetrievalCases reads a corpus's retrieval cases, falling back to the seed.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: the same read-or-seed rule as the others. The seed holds no cases, so a
// bundle that has authored none grades nothing and says so — which is the honest
// answer and the reason an absent file is not an error.
func LoadRetrievalCases(bundleDir string) (*standards.Retrieval, error) {
	return loadStandards(bundleDir, "bundle.LoadRetrievalCases",
		standards.RetrievalFileName, standards.DefaultRetrieval, standards.LoadRetrieval)
}

// loadStandards is the read-or-seed rule the four loaders share.
//
// Requires: name is bundle-relative; seed returns the embedded default; parse is
// the file's loader.
// Ensures: an absent file gets the seed and a present-but-malformed file is a hard
// error, both with the caller's op on the error.
//
// Extracted at the third caller rather than the second. Two copies of an eight-line
// read were tolerable; three would make the rule — which is a decided policy, not
// boilerplate — something a fourth loader could get subtly wrong by copying the
// wrong one of them.
func loadStandards[T any](
	bundleDir, op, name string, seed func() []byte, parse func([]byte) (*T, error),
) (*T, error) {
	src, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(name)))
	if errors.Is(err, fs.ErrNotExist) {
		src = seed()
	} else if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	out, err := parse(src)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// ArchiveGates projects the loaded standards onto what the archive policy needs.
//
// Requires: a is loaded; rules is §9.3's stage 2 and 3 ruleset, or nil.
// Ensures: gates whose ScanText covers every §9.3 stage the ruleset supports.
//
// This is the join §0.1's layering requires: adapters do not import each other,
// so `archive` states its dependency as a value and this shell — which may import
// both — is the one place that knows they correspond. A gate added to `standards`
// and not wired through here is inert, which is why the two are tested together.
//
// The ruleset arrives as a parameter for the same reason: `archive` cannot import
// `scan`, and the scan is a dependency rather than something the policy reaches
// for. A nil ruleset still produces a working ScanText covering stage 1.
func ArchiveGates(a *standards.Archive, rules *scan.Ruleset) archive.Gates {
	return archive.Gates{
		Allowlist:          a.Allowlist.Value,
		PerFileCap:         a.PerFileCap.Value,
		EmbeddedPayloadCap: a.EmbeddedPayloadCap.Value,
		ScanText:           scanTextWith(rules),
	}
}

// scanTextWith is §9.3's admission scan, as the archive's policy needs it.
//
// Requires: rules may be nil, in which case only stage 1 runs.
// Ensures: ReasonNone for text that passes every stage the ruleset supports, and
// otherwise the reason for the first thing found. Pure, and safe to share.
//
// It reports **one** reason, and that is right for a disposition: a source carrying
// both a zero-width character and an AWS key is refused either way. What it loses is
// the detail, which is why `ScanFindings` exists beside it — the reason goes on the
// record and the full set goes in the report. Hidden characters are checked first
// because they are the stage whose constants are not arguable at all, so it is the
// reason least likely to be disputed when a source is refused.
//
// A secret and an injection produce different reasons, and that distinction earns
// its keep: an injected instruction is somebody attacking this corpus, and a
// committed credential is somebody's own mistake upstream that now needs rotating.
// Collapsing them into one reason would send a reader to the wrong response.
func scanTextWith(rules *scan.Ruleset) func(string) archive.RejectReason {
	return func(text string) archive.RejectReason {
		if len(scan.Hidden(text)) > 0 {
			return archive.ReasonHiddenCharacters
		}
		matches := rules.Patterns(text)
		// A secret outranks an injection when a source carries both. The reason is
		// what a reader does next: an injected instruction means this source is not
		// admitted, and a leaked credential means somebody has to rotate a key
		// whatever happens to the source.
		for _, m := range matches {
			if m.Category == scan.CategorySecret {
				return archive.ReasonSecret
			}
		}
		if len(matches) > 0 {
			return archive.ReasonInjectionPattern
		}
		return archive.ReasonNone
	}
}

// ScanFindings is every §9.3 finding in text, rendered.
//
// Requires: rules may be nil, in which case only stage 1's findings are reported.
// Ensures: one line per hidden-character class and one per matching rule, in the
// order the scanners produce them — which is sorted, so two runs describe one text
// identically. Empty rather than nil for clean text. Pure.
//
// It exists because `scanTextWith` reduces the scan to one `RejectReason` for the
// record, which is right for a disposition and loses the detail: a source carrying
// three classes reports one. The reason answers "what became of this"; this answers
// "what is in it", and a reader deciding whether to fix the source or rotate a key
// needs the second.
//
// **It renders through `scan.Describe`, which the candidate scan also uses.** A
// second renderer would let `fetch` and the promote gate describe one problem two
// ways, so an author who saw a finding from one and something else from the other
// would have to work out that they were the same. That is the Repetition red flag
// with a consequence attached.
//
// The bytes are scanned a second time here rather than the decision's findings being
// carried out of `archive`. That keeps `archive.Decide` pure and its signature about
// dispositions, and the two cannot disagree because both go through the same
// ruleset — the cost is one more pass over a file bounded by the per-file cap, and
// only for a source that was refused.
func ScanFindings(rules *scan.Ruleset, text string) []string {
	return scan.Describe(scan.Hidden(text), rules.Patterns(text))
}

// LoadIndicators reads a bundle's indicator words, falling back to the seed.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: the same rule as the gate loaders — an absent file gets the embedded
// default, and a present file that will not load is a hard error.
func LoadIndicators(bundleDir string) (*standards.Indicators, error) {
	return loadStandards(bundleDir, "bundle.LoadIndicators",
		standards.IndicatorsFileName, standards.DefaultIndicators, standards.LoadIndicators)
}

// dependentMarkers is the word list segmentation refuses cuts on.
//
// It returns nil rather than an error when the list will not load, and that is the
// one place this file is lenient on purpose. A malformed indicator file must not
// stop a reply being admitted: without the words, segmentation behaves exactly as it
// did before they existed — it cuts where the copula test allows — which is a
// coarser corpus rather than a wrong one. `doctor` is where a broken standards file
// is reported.
func dependentMarkers(bundleDir string) []string {
	in, err := LoadIndicators(bundleDir)
	if err != nil {
		return nil
	}
	return in.Words(standards.RoleReason)
}

// LoadStrengths reads a bundle's claim-strength markers, falling back to the seed.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: the same rule as the gate loaders — an absent file gets the embedded default,
// and a present file that will not load is a hard error.
func LoadStrengths(bundleDir string) (*standards.Strengths, error) {
	return loadStandards(bundleDir, "bundle.LoadStrengths",
		standards.StrengthFileName, standards.DefaultStrengths, standards.LoadStrengths)
}

// strengthMarkers is the claim-strength lists `coverage` compares against.
//
// Requires: fsys is rooted at the bundle.
// Ensures: the seed's markers when the bundle declares none, and the zero value when a
// file that is present will not parse.
//
// **Read through the fs.FS rather than by path**, like `vocabulary` and unlike the gate
// loaders: `Snapshot` is handed a filesystem so a caller testing with an fstest.MapFS
// gets the same answers as one reading a disk, and a path parameter here would be the
// one input that could not be faked.
//
// A malformed file yields the zero value, which skips the check with a reason rather
// than passing every claim — reporting the absence of the ruler as a clean measurement
// is the one output a health command must never produce. `doctor` reports the parse
// failure.
func strengthMarkers(fsys fs.FS) lint.Strengths {
	raw, err := fs.ReadFile(fsys, standards.StrengthFileName)
	if err != nil {
		raw = standards.DefaultStrengths()
	}
	in, err := standards.LoadStrengths(raw)
	if err != nil {
		return lint.Strengths{}
	}
	return lint.Strengths{
		Universal: in.Words(standards.StrengthUniversal),
		Hedged:    in.Words(standards.StrengthHedged),
	}
}

// registerWords reads a bundle's causal registers, falling back to the seed.
//
// The same shape as strengthMarkers and for the same reason: a file that will not load
// leaves the check unable to run and saying so, rather than running against half a list.
func registerWords(fsys fs.FS) lint.Registers {
	raw, err := fs.ReadFile(fsys, standards.RegistersFileName)
	if err != nil {
		raw = standards.DefaultRegisters()
	}
	in, err := standards.LoadRegisters(raw)
	if err != nil {
		return lint.Registers{}
	}
	return lint.Registers{
		Intervention: in.Words(standards.RegisterIntervention),
		Association:  in.Words(standards.RegisterAssociation),
	}
}

// LoadOperators reads a bundle's operator patterns, falling back to the seed.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: the same rule as the gate loaders — an absent file gets the embedded default,
// and a present file that will not load is a hard error.
func LoadOperators(bundleDir string) (*standards.Operators, error) {
	return loadStandards(bundleDir, "bundle.LoadOperators",
		standards.OperatorsFileName, standards.DefaultOperators, standards.LoadOperators)
}

// OperatorPatterns is the pattern set the constraint parser reads.
//
// Requires: bundleDir is a path.
// Ensures: nil when the file will not load, which makes every claim parse to no
// constraint rather than to a wrong one — the interval and enumeration predicates then
// skip instead of comparing readings nobody can trust. `doctor` reports the parse
// failure.
func OperatorPatterns(bundleDir string) []constraint.Pattern {
	in, err := LoadOperators(bundleDir)
	if err != nil {
		return nil
	}
	out := make([]constraint.Pattern, 0, len(in.Pattern))
	for _, p := range in.Pattern {
		out = append(out, constraint.Pattern{
			ID: p.ID, Phrase: p.Phrase, Op: constraint.OpKind(p.Op),
		})
	}
	return out
}

// indicatorWords is the closed lexical class of §9.4.1, for the checks that read it.
//
// Requires: fsys is rooted at the bundle.
// Ensures: the seed's words when the bundle declares none, and the zero value when a
// present file will not parse — which skips §17.4's check with a reason rather than
// passing every lead.
//
// Read through the fs.FS like the vocabulary and the strength markers, so a caller
// testing with an fstest.MapFS gets the same answers as one reading a disk.
func indicatorWords(fsys fs.FS) lint.Indicators {
	raw, err := fs.ReadFile(fsys, standards.IndicatorsFileName)
	if err != nil {
		raw = standards.DefaultIndicators()
	}
	in, err := standards.LoadIndicators(raw)
	if err != nil {
		return lint.Indicators{}
	}
	return lint.Indicators{
		Reason:     in.Words(standards.RoleReason),
		Conclusion: in.Words(standards.RoleConclusion),
	}
}

// languageMarkers is §10.3's lexical class, for the check that reads it.
//
// Requires: fsys is rooted at the bundle.
// Ensures: the seed's markers when the bundle declares none, and none when a present
// file will not parse — which skips the check with a reason rather than passing every
// document.
func languageMarkers(fsys fs.FS) []lint.LanguageMarker {
	raw, err := fs.ReadFile(fsys, standards.LanguageFileName)
	if err != nil {
		raw = standards.DefaultLanguage()
	}
	in, err := standards.LoadLanguage(raw)
	if err != nil {
		return nil
	}
	out := make([]lint.LanguageMarker, 0, len(in.Marker))
	for _, m := range in.Marker {
		out = append(out, lint.LanguageMarker{Phrase: m.Phrase, Role: m.Role})
	}
	return out
}
