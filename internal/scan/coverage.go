package scan

// The four stages SPEC §9.3 specifies, in the order it lists them.
const (
	StageHidden    = "hidden-characters"
	StageInjection = "injection-patterns"
	StageSecrets   = "secrets"
	StageOversize  = "oversize"
)

// Coverage is which of §9.3's stages a scan actually performed.
//
// It exists because "no findings" and "§9.3 satisfied" are different claims and
// only one of them is currently available. A caller that cannot tell them apart
// will eventually assert the second on the strength of the first — which is the
// specific failure a security stage cannot be allowed to have, since the content
// being gated is content that arrived from outside.
type Coverage struct {
	// Ran names the stages performed, in §9.3 order.
	Ran []string `json:"ran"`

	// Missing names the stages specified and not performed, in §9.3 order. Empty
	// only when the scan was complete.
	Missing []string `json:"missing,omitempty"`
}

// Complete reports whether every specified stage ran.
//
// Requires: nothing; the zero Coverage is not complete, which is the safe
// reading of a value nobody populated.
// Ensures: true only when Ran covers §9.3 and Missing is empty.
func (c Coverage) Complete() bool { return len(c.Missing) == 0 && len(c.Ran) > 0 }

// TextCoverage is what Hidden covers when scanning a string in memory.
//
// Requires: nothing.
// Ensures: names the stages honestly rather than the stages specified.
//
// Stage 4 is listed as missing here and that deserves a note, because it is the
// one stage that is arguably already done. `archive.Gates` enforces `per_file_cap`
// and `embedded_payload_cap` when a fetched source is admitted to tier 0, which is
// §9.3's oversize bound in the place §9.3 asks for it. What that does not cover is
// a *candidate document* — the archive gate bounds sources arriving from upstream,
// and a document a model wrote is neither fetched nor archived. So the bound
// exists for one input and not the other, and reporting stage 4 as run would be
// true of the tier-0 path and false of the promote path from one function that
// cannot tell which it is serving.
//
// Inventing a second bound here to close the gap was considered and rejected: §6.5
// exists to stop exactly that, a threshold with no basis chosen so a report can
// read clean.
func TextCoverage() Coverage {
	return Coverage{
		Ran:     []string{StageHidden},
		Missing: []string{StageInjection, StageSecrets, StageOversize},
	}
}
