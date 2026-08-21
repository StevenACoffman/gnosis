package command

// The three effects of SPEC §4.6.2.
//
// EffectUnset is the zero value and is **rejected**, never treated as a preview.
// A `DryRun bool` has this backwards — `false` means *really do it*, so a caller
// that forgot the field performs a live write. That inverts the rule applied
// everywhere else in this codebase: quotecheck's Unchecked is the zero value,
// finding.Action has no ActionUnknown, claims.pos is nullable because 0 is a real
// position, and archive.DispositionUnset asserts no durability.
//
// Substituting a preview would be almost as bad as substituting an apply: a
// preview is a deliberate request for a report, and quietly giving one to a
// caller that meant to write would let it believe the write happened.
const (
	EffectUnset   Effect = iota // rejected, never assumed
	EffectPreview               // run every gate, write nothing
	EffectApply                 // run every gate, then write what they approved
)

// Effect decides whether a command's final write happens.
//
// It is a field on the command rather than a separate verb, and that is what
// makes §9.4's guarantee constructible rather than promised: preview and apply
// are the same handler over the same input, so they cannot compute different
// diffs. Two code paths could agree by inspection and drift by edit; one path
// cannot disagree with itself.
type Effect int

// Valid reports whether e is one of the two effects a caller may ask for.
//
// Requires: nothing.
// Ensures: false for EffectUnset and for any value outside the enumeration, so
// the check fails closed on a number that arrived from a wire format.
func (e Effect) Valid() bool {
	return e == EffectPreview || e == EffectApply
}

// Writes reports whether this effect reaches the disk.
//
// Requires: nothing.
// Ensures: true only for EffectApply. Every other value, including EffectUnset
// and an out-of-range one, reads as "does not write" — the direction in which a
// mistake is recoverable.
func (e Effect) Writes() bool { return e == EffectApply }

// String renders the effect for a message a person reads.
func (e Effect) String() string {
	switch e {
	case EffectPreview:
		return "preview"
	case EffectApply:
		return "apply"
	case EffectUnset:
		return "unset"
	default:
		return "invalid"
	}
}
